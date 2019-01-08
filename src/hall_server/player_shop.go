package main

import (
	"ih_server/libs/log"
	"ih_server/libs/utils"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"ih_server/src/table_config"
	"time"

	"github.com/golang/protobuf/proto"
)

const (
	SHOP_TYPE_NORMAL     = 1
	SHOP_TYPE_HERO       = 2
	SHOP_TYPE_TOWER      = 3
	SHOP_TYPE_ARENA      = 4
	SHOP_TYPE_GUILD      = 5
	SHOP_TYPE_EXPEDITOIN = 6
)

func (this *Player) _refresh_shop(shop *table_config.XmlShopItem) int32 {
	if !this.db.Shops.HasIndex(shop.Id) {
		this.db.Shops.Add(&dbPlayerShopData{
			Id: shop.Id,
		})
	}
	this.db.Shops.SetCurrAutoId(shop.Id, shop.Id*10000)

	if shop.ShopMaxSlot > 0 {
		for i := int32(0); i < shop.ShopMaxSlot; i++ {
			shop_item := shopitem_table_mgr.RandomShopItemByPlayerLevel(shop.Id, this.db.Info.GetLvl())
			if shop_item == nil {
				log.Error("Player[%v] random shop[%v] item failed", this.Id, shop.Id)
				return int32(msg_client_message.E_ERR_PLAYER_SHOP_ITEM_RANDOM_DATA_INVALID)
			}
			curr_id := this.db.Shops.IncbyCurrAutoId(shop.Id, 1)
			if this.db.ShopItems.HasIndex(curr_id) {
				this.db.ShopItems.SetShopItemId(curr_id, shop_item.Id)
				this.db.ShopItems.SetLeftNum(curr_id, shop_item.StockNum)
			} else {
				this.db.ShopItems.Add(&dbPlayerShopItemData{
					Id:         curr_id,
					ShopItemId: shop_item.Id,
					LeftNum:    shop_item.StockNum,
				})
			}
		}
	} else {
		// 商店所有物品都刷
		items_shop := shopitem_table_mgr.GetItemsShop(shop.Id)
		if items_shop == nil {
			log.Error("Shop[%v] cant found items", shop.Id)
			return int32(msg_client_message.E_ERR_PLAYER_SHOP_ITEM_TABLE_DATA_NOT_FOUND)
		}
		for _, item := range items_shop {
			curr_id := this.db.Shops.IncbyCurrAutoId(shop.Id, 1)
			if this.db.ShopItems.HasIndex(curr_id) {
				this.db.ShopItems.SetShopItemId(curr_id, item.Id)
				this.db.ShopItems.SetLeftNum(curr_id, item.StockNum)
			} else {
				this.db.ShopItems.Add(&dbPlayerShopItemData{
					Id:         curr_id,
					ShopItemId: item.Id,
					LeftNum:    item.StockNum,
				})
			}
		}
	}

	return 1
}

func (this *Player) get_shop_free_refresh_info(shop *table_config.XmlShopItem) (remain_secs int32, cost_res []int32) {
	cost_res = shop.RefreshRes
	if shop.FreeRefreshTime <= 0 {
		remain_secs = -1
		return
	}

	now_time := int32(time.Now().Unix())
	last_refresh, _ := this.db.Shops.GetLastFreeRefreshTime(shop.Id)
	if last_refresh == 0 {
		this._refresh_shop(shop)
		// 确保每次进商店只刷一次
		this.db.Shops.SetLastFreeRefreshTime(shop.Id, 1)
	}

	remain_secs = shop.FreeRefreshTime - (now_time - last_refresh)
	if remain_secs < 0 {
		remain_secs = 0
	}

	return
}

func (this *Player) _send_shop(shop *table_config.XmlShopItem, free_remain_secs int32) int32 {
	var shop_items []*msg_client_message.ShopItem
	item_ids := this.db.ShopItems.GetAllIndex()

	var has_item bool
	for _, id := range item_ids {
		if id/10000 != shop.Id {
			continue
		}
		has_item = true
		break
	}

	if !has_item && shop.AutoRefreshTime == "" && shop.FreeRefreshTime == 0 {
		this._refresh_shop(shop)
		item_ids = this.db.ShopItems.GetAllIndex()
	}

	for _, id := range item_ids {
		if id/10000 != shop.Id {
			continue
		}
		item_id, _ := this.db.ShopItems.GetShopItemId(id)
		shop_item_tdata := shopitem_table_mgr.GetItem(item_id)
		if shop_item_tdata == nil {
			log.Warn("Player[%v] shop[%v] item[%v] table data not found", this.Id, shop.Id, item_id)
			continue
		}
		num, o := this.db.ShopItems.GetLeftNum(id)
		if !o {
			continue
		}

		shop_item := &msg_client_message.ShopItem{
			Id:     id,
			ItemId: item_id,
			CostResource: &msg_client_message.ItemInfo{
				Id:    shop_item_tdata.BuyCost[0],
				Value: shop_item_tdata.BuyCost[1],
			},
			BuyNum: num,
		}
		shop_items = append(shop_items, shop_item)
	}

	auto_remain_secs := int32(-1)
	if shop.AutoRefreshTime != "" {
		if !this.db.Shops.HasIndex(shop.Id) {
			this.db.Shops.Add(&dbPlayerShopData{
				Id: shop.Id,
			})
		}
		last_refresh, _ := this.db.Shops.GetLastAutoRefreshTime(shop.Id)
		auto_remain_secs = utils.GetRemainSeconds2NextDayTime(last_refresh, shop.AutoRefreshTime)
	}

	response := &msg_client_message.S2CShopDataResponse{
		ShopId: shop.Id,
		Items:  shop_items,
		NextFreeRefreshRemainSeconds: free_remain_secs,
		NextAutoRefreshRemainSeconds: auto_remain_secs,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_SHOP_DATA_RESPONSE), response)

	log.Debug("Player[%v] send shop data: %v", this.Id, response)
	return 1
}

func (this *Player) check_shop_auto_refresh(shop *table_config.XmlShopItem, send_notify bool) bool {
	// 固定时间点自动刷新
	if shop.AutoRefreshTime == "" {
		return false
	}

	now_time := int32(time.Now().Unix())
	last_refresh, o := this.db.Shops.GetLastAutoRefreshTime(shop.Id)
	if !o {
		this.db.Shops.Add(&dbPlayerShopData{
			Id: shop.Id,
		})
	} else {
		if !utils.CheckDayTimeArrival(last_refresh, shop.AutoRefreshTime) {
			return false
		}
	}

	this._refresh_shop(shop)

	this.db.Shops.SetLastAutoRefreshTime(shop.Id, now_time)

	if send_notify {
		this.send_shop(shop.Id)
		notify := &msg_client_message.S2CShopAutoRefreshNotify{
			ShopId: shop.Id,
		}
		this.Send(uint16(msg_client_message_id.MSGID_S2C_SHOP_AUTO_REFRESH_NOTIFY), notify)
	}

	log.Debug("Player[%v] shop[%v] auto refreshed", this.Id, shop.Id)

	return true
}

// 商店数据
func (this *Player) send_shop(shop_id int32) int32 {
	if shop_id == SHOP_TYPE_GUILD && this.db.Guild.GetId() <= 0 {
		return int32(msg_client_message.E_ERR_PLAYER_SHOP_GUILD_NOT_JOIN)
	}
	shop_tdata := shop_table_mgr.Get(shop_id)
	if shop_tdata == nil {
		log.Error("Shop[%v] table data not found", shop_id)
		return int32(msg_client_message.E_ERR_PLAYER_SHOP_TABLE_DATA_NOT_FOUND)
	}

	if this.check_shop_auto_refresh(shop_tdata, false) {
		log.Debug("!!!!!!!!!!!!!!!!!! Player[%v] shop[%v] refreshed", this.Id, shop_id)
	}

	free_remain_secs, _ := this.get_shop_free_refresh_info(shop_tdata)
	/*if shop_tdata.FreeRefreshTime > 0 && free_remain_secs <= 0 {
		free_remain_secs = shop_tdata.FreeRefreshTime
	}*/
	res := this._send_shop(shop_tdata, free_remain_secs)
	if res < 0 {
		return res
	}
	return 1
}

// 商店购买
func (this *Player) shop_buy_item(shop_id, id, buy_num int32) int32 {
	if shop_id == SHOP_TYPE_GUILD && this.db.Guild.GetId() <= 0 {
		return int32(msg_client_message.E_ERR_PLAYER_SHOP_GUILD_NOT_JOIN)
	}

	if buy_num <= 0 {
		log.Error("Player[%v] buy shop item num[%v] must greater than 0", this.Id, buy_num)
		return -1
	}

	shop_tdata := shop_table_mgr.Get(shop_id)
	if shop_tdata == nil {
		log.Error("Shop[%v] table data not found", shop_id)
		return int32(msg_client_message.E_ERR_PLAYER_SHOP_TABLE_DATA_NOT_FOUND)
	}

	if this.check_shop_auto_refresh(shop_tdata, true) {
		return 1
	}

	item_id, o := this.db.ShopItems.GetShopItemId(id)
	if !o {
		log.Error("Player[%v] shop[%v] not found item id[%v]", this.Id, shop_id, id)
		return int32(msg_client_message.E_ERR_PLAYER_SHOP_ITEM_NOT_FOUND)
	}

	shopitem_tdata := shopitem_table_mgr.GetItem(item_id)
	if shopitem_tdata == nil {
		log.Error("Shop[%v] item[%v] table data not found", shop_id, item_id)
		return int32(msg_client_message.E_ERR_PLAYER_SHOP_ITEM_TABLE_DATA_NOT_FOUND)
	}

	left_num := int32(-1)
	if shopitem_tdata.StockNum > 0 {
		left_num, _ = this.db.ShopItems.GetLeftNum(id)
		if left_num < buy_num {
			log.Error("Player[%v] shop[%v] item[%v] num[%v] not enough to buy, need[%v]", this.Id, shop_id, id, left_num, buy_num)
			return int32(msg_client_message.E_ERR_PLAYER_SHOP_ITEM_NUM_NOT_ENOUGH)
		}
	}

	for i := 0; i < len(shopitem_tdata.BuyCost)/2; i++ {
		res_id := shopitem_tdata.BuyCost[2*i]
		res_cnt := shopitem_tdata.BuyCost[2*i+1] * buy_num
		now_cnt := this.get_resource(res_id)
		if now_cnt < res_cnt {
			log.Error("Player[%v] in shop[%v] buy item[%v] num[%v] not enough resource[%v], need[%v] now[%v]", this.Id, shop_id, item_id, buy_num, res_id, res_cnt, now_cnt)
			return int32(msg_client_message.E_ERR_PLAYER_SHOP_ITEM_BUY_RESOURCE_NOT_ENOUGH)
		}
	}

	for i := 0; i < len(shopitem_tdata.Item)/2; i++ {
		this.add_resource(shopitem_tdata.Item[2*i], shopitem_tdata.Item[2*i+1]*buy_num)
	}

	for i := 0; i < len(shopitem_tdata.BuyCost)/2; i++ {
		this.add_resource(shopitem_tdata.BuyCost[2*i], -shopitem_tdata.BuyCost[2*i+1]*buy_num)
	}

	//if shopitem_tdata.StockNum > 0 {
	this.db.ShopItems.IncbyLeftNum(id, -buy_num)
	//}

	if left_num > 0 {
		left_num -= buy_num
	}
	response := &msg_client_message.S2CShopBuyItemResponse{
		ShopId:       shop_id,
		Id:           id,
		BuyNum:       buy_num,
		RemainBuyNum: left_num,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_SHOP_BUY_ITEM_RESPONSE), response)

	// 更新任务
	this.TaskUpdate(table_config.TASK_COMPLETE_TYPE_BUY_ITEM_NUM_ON_SHOP, false, shop_id, buy_num)

	log.Debug("Player[%v] in shop[%v] buy item[%v] num[%v], cost resource %v  add item %v", this.Id, shop_id, id, buy_num, shopitem_tdata.BuyCost, shopitem_tdata.Item)

	return 1
}

// 商店刷新
func (this *Player) shop_refresh(shop_id int32) int32 {
	if shop_id == SHOP_TYPE_GUILD && this.db.Guild.GetId() <= 0 {
		return int32(msg_client_message.E_ERR_PLAYER_SHOP_GUILD_NOT_JOIN)
	}

	shop_tdata := shop_table_mgr.Get(shop_id)
	if shop_tdata == nil {
		log.Error("Shop[%v] table data not found", shop_id)
		return int32(msg_client_message.E_ERR_PLAYER_SHOP_TABLE_DATA_NOT_FOUND)
	}

	if this.check_shop_auto_refresh(shop_tdata, true) {
		return 1
	}

	free_remain_secs, cost_res := this.get_shop_free_refresh_info(shop_tdata)

	// 免费刷新
	is_free := false
	if shop_tdata.FreeRefreshTime > 0 && free_remain_secs <= 0 {
		free_remain_secs = shop_tdata.FreeRefreshTime
		is_free = true
	}

	// 手动刷新
	if !is_free {
		for i := 0; i < len(cost_res)/2; i++ {
			if this.get_resource(cost_res[2*i]) < cost_res[2*i+1] {
				log.Error("Player[%v] refresh shop[%v] failed, not enough resource%v", this.Id, shop_id, cost_res)
				return int32(msg_client_message.E_ERR_PLAYER_ITEM_NUM_NOT_ENOUGH)
			}
		}
	}

	this._refresh_shop(shop_tdata)

	if !is_free {
		for i := 0; i < len(cost_res)/2; i++ {
			this.add_resource(cost_res[2*i], -cost_res[2*i+1])
		}
	}

	this._send_shop(shop_tdata, free_remain_secs)

	if is_free {
		this.db.Shops.SetLastFreeRefreshTime(shop_id, int32(time.Now().Unix()))
	}

	response := &msg_client_message.S2CShopRefreshResponse{
		ShopId:        shop_id,
		IsFreeRefresh: is_free,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_SHOP_REFRESH_RESPONSE), response)
	log.Debug("Player[%v] refresh shop %v", this.Id, response)
	return 1
}

func C2SShopDataHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SShopDataRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}
	return p.send_shop(req.GetShopId())
}

func C2SShopBuyItemHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SShopBuyItemRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}
	return p.shop_buy_item(req.GetShopId(), req.GetItemId(), req.GetBuyNum())
}

func C2SShopRefreshHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SShopRefreshRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}
	return p.shop_refresh(req.GetShopId())
}
