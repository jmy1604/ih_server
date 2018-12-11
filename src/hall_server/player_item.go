package main

import (
	"ih_server/libs/log"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"ih_server/src/share_data"
	"ih_server/src/table_config"
	"math"
	"net/http"

	"github.com/golang/protobuf/proto"
)

// 物品类型
const (
	ITEM_TYPE_NONE     = iota
	ITEM_TYPE_RESOURCE = 1 // 资源类
	ITEM_TYPE_EQUIP    = 2 // 装备
	ITEM_TYPE_COST     = 3 // 消耗品
	ITEM_TYPE_PIECE    = 4 // 碎片
	ITEM_TYPE_HEAD     = 5 // 头像
)

// 其他属性
const (
	ITEM_RESOURCE_ID_GOLD        = 1  // 金币
	ITEM_RESOURCE_ID_SOUL        = 2  // 绿魂
	ITEM_RESOURCE_ID_DIAMOND     = 3  // 钻石
	ITEM_RESOURCE_ID_EXP         = 7  // 经验值
	ITEM_RESOURCE_ID_STAMINA     = 8  // 体力
	ITEM_RESOURCE_ID_FRIENDPOINT = 9  // 友情点
	ITEM_RESOURCE_ID_HEROCOIN    = 10 // 英雄币
	ITEM_RESOURCE_ID_HONOR       = 17 // 荣誉点
	ITEM_RESOURCE_ID_VIP_EXP     = 21 // vip经验
)

// 装备类型
const (
	EQUIP_TYPE_HEAD      = 1 // 头
	EQUIP_TYPE_WEAPON    = 2 // 武器
	EQUIP_TYPE_CHEST     = 3 // 胸
	EQUIP_TYPE_BOOT      = 4 // 鞋
	EQUIP_TYPE_LEFT_SLOT = 5 // 左槽
	EQUIP_TYPE_RELIC     = 6 // 神器
	EQUIP_TYPE_MAX       = 7 //
)

func (this *dbPlayerItemColumn) BuildMsg() (items []*msg_client_message.ItemInfo) {
	this.m_row.m_lock.UnSafeRLock("dbPlayerItemColumn.BuildMsg")
	defer this.m_row.m_lock.UnSafeRUnlock()

	for _, v := range this.m_data {
		item := &msg_client_message.ItemInfo{
			Id:    v.Id,
			Value: v.Count,
		}
		items = append(items, item)
	}

	return
}

func (this *Player) add_item(id int32, count int32) bool {
	item := item_table_mgr.Get(id)
	if item == nil {
		log.Error("item %v not found in table", id)
		return false
	}

	if count < 0 {
		return false
	}

	if id == ITEM_RESOURCE_ID_HONOR {
		var vip_count int32
		vip_info := vip_table_mgr.Get(this.db.Info.GetVipLvl())
		if vip_info != nil {
			vip_count = int32(float32(count) * (float32(vip_info.HonorPointBonus) / 10000))
		}
		count += vip_count
	}

	if !this.db.Items.HasIndex(id) {
		if item.Type == ITEM_TYPE_HEAD {
			count = 1
		}
		this.db.Items.Add(&dbPlayerItemData{
			Id:    id,
			Count: count,
		})
	} else {
		if item.Type != ITEM_TYPE_HEAD {
			old_count, _ := this.db.Items.GetCount(id)
			if old_count+count < 0 {
				this.db.Items.SetCount(id, math.MaxInt32)
				count = math.MaxInt32 - old_count
			} else {
				this.db.Items.IncbyCount(id, count)
			}
		} else {
			count = 0
		}
	}

	if this.items_changed_info == nil {
		this.items_changed_info = make(map[int32]int32)
	}
	if count > 0 {
		d, o := this.items_changed_info[id]
		if !o {
			this.items_changed_info[id] = count
		} else {
			d += count
			if d != 0 {
				this.items_changed_info[id] = d
			} else {
				delete(this.items_changed_info, id)
			}
		}
	}

	// 更新任务
	if item.EquipType > 0 {
		this.TaskUpdate(table_config.TASK_COMPLETE_TYPE_GET_QUALITY_EQUIPS_NUM, false, item.Quality, count)
	}

	return true
}

func (this *Player) del_item(id int32, count int32) bool {
	c, o := this.db.Items.GetCount(id)
	if !o {
		return false
	}

	if count <= 0 || c < count {
		return false
	}

	if c == count {
		this.db.Items.Remove(id)
	} else {
		this.db.Items.IncbyCount(id, -count)
	}

	if this.items_changed_info == nil {
		this.items_changed_info = make(map[int32]int32)
	}
	if d, o := this.items_changed_info[id]; !o {
		this.items_changed_info[id] = -count
	} else {
		d -= count
		if d != 0 {
			this.items_changed_info[id] = d
		} else {
			delete(this.items_changed_info, id)
		}
	}
	return true
}

func (this *Player) get_item(id int32) int32 {
	c, _ := this.db.Items.GetCount(id)
	return c
}

func (this *Player) add_all_items() {
}

func (this *Player) send_items() {
	msg := &msg_client_message.S2CItemsSync{}
	msg.Items = this.db.Items.BuildMsg()
	this.Send(uint16(msg_client_message_id.MSGID_S2C_ITEMS_SYNC), msg)
}

func (this *Player) add_gold(add int32) int32 {
	result := int32(0)
	gold := add + this.db.Info.GetGold()
	if gold >= 0 {
		result = this.db.Info.IncbyGold(add)
	} else {
		if add > 0 {
			result = math.MaxInt32
		} else {
			result = 0
		}
		this.db.Info.SetGold(result)
	}
	if add != 0 {
		this.b_base_prop_chg = true
	}
	return result
}

func (this *Player) get_gold() int32 {
	return this.db.Info.GetGold()
}

func (this *Player) add_diamond(add int32) int32 {
	result := int32(0)
	diamond := add + this.db.Info.GetDiamond()
	if diamond >= 0 {
		result = this.db.Info.IncbyDiamond(add)
	} else {
		if add > 0 {
			result = math.MaxInt32
		} else {
			result = 0
		}
		this.db.Info.SetDiamond(result)
	}

	if add < 0 {
		this.activity_update(ACTIVITY_EVENT_DIAMOND_COST, -add, 0, 0, 0)
	}

	if add != 0 {
		this.b_base_prop_chg = true
	}
	return result
}

func (this *Player) get_diamond() int32 {
	return this.db.Info.GetDiamond()
}

func (this *Player) add_exp(add int32) (level, exp int32) {
	if add < 0 {
		return
	}

	exp = add + this.db.Info.GetExp()
	if exp < add || exp < this.db.Info.GetExp() {
		exp = math.MaxInt32
	}

	level = this.db.Info.GetLvl()
	for {
		lvl_data := levelup_table_mgr.Get(level)
		if lvl_data == nil {
			break
		}
		if lvl_data.PlayerLevelUpExp <= 0 {
			break
		}
		if lvl_data.PlayerLevelUpExp > exp {
			break
		}
		exp -= lvl_data.PlayerLevelUpExp
		level += 1
	}

	if exp != this.db.Info.GetExp() {
		this.db.Info.SetExp(exp)
		this.b_base_prop_chg = true
	}

	if level != this.db.Info.GetLvl() {
		this.db.Info.SetLvl(level)
		this.b_base_prop_chg = true
		// 更新任务
		this.TaskUpdate(table_config.TASK_COMPLETE_TYPE_REACH_LEVEL, true, level, 1)
		share_data.SaveAccountPlayerInfo(hall_server.redis_conn, this.Account, &msg_client_message.AccountPlayerInfo{
			ServerId:    config.ServerId,
			PlayerName:  this.db.GetName(),
			PlayerLevel: level,
			PlayerHead:  this.db.Info.GetHead(),
		})
	}

	return
}

func (this *Player) get_exp() int32 {
	return this.db.Info.GetExp()
}

func (this *Player) add_vip_exp(add_exp int32) (level, exp int32) {
	if add_exp <= 0 {
		return
	}

	curr_exp := this.get_resource(ITEM_RESOURCE_ID_VIP_EXP)
	exp = add_exp + curr_exp
	if exp < add_exp || exp < curr_exp {
		exp = math.MaxInt32
	}

	level = this.db.Info.GetVipLvl()
	if int(level) >= len(vip_table_mgr.Array) {
		log.Warn("Player[%v] Vip level[%v] is max", this.Id, level)
		return
	}

	for {
		vip_data := vip_table_mgr.Get(level + 1)
		if vip_data == nil {
			break
		}
		if vip_data.Exp <= 0 || vip_data.Exp > exp {
			break
		}
		level += 1
	}

	if level != this.db.Info.GetVipLvl() {
		this.db.Info.SetVipLvl(level)
		this.b_base_prop_chg = true
		// 更新任务
		this.TaskUpdate(table_config.TASK_COMPLETE_TYPE_REACH_VIP_N_LEVEL, true, level, 1)
	}

	if exp != this.get_resource(ITEM_RESOURCE_ID_VIP_EXP) {
		this.set_resource(ITEM_RESOURCE_ID_VIP_EXP, exp)
	}

	return
}

func (this *Player) get_vip_exp() int32 {
	return this.get_resource(ITEM_RESOURCE_ID_VIP_EXP)
}

func (this *Player) add_resource(id, count int32) bool {
	res := true
	if id == ITEM_RESOURCE_ID_GOLD {
		this.add_gold(count)
	} else if id == ITEM_RESOURCE_ID_DIAMOND {
		this.add_diamond(count)
	} else if id == ITEM_RESOURCE_ID_EXP {
		this.add_exp(count)
	} else if id == ITEM_RESOURCE_ID_VIP_EXP {
		this.add_vip_exp(count)
	} else {
		if count > 0 {
			if !this.add_item(id, count) {
				res = false
			}
		} else {
			if !this.del_item(id, -count) {
				res = false
			}
		}
	}
	return res
}

func (this *Player) get_resource(id int32) int32 {
	if id == ITEM_RESOURCE_ID_GOLD {
		return this.get_gold()
	} else if id == ITEM_RESOURCE_ID_DIAMOND {
		return this.get_diamond()
	} else if id == ITEM_RESOURCE_ID_EXP {
		return this.get_exp()
	} else {
		return this.get_item(id)
	}
}

func (this *Player) set_resource(id int32, num int32) int32 {
	if num < 0 {
		return -1
	}
	if id == ITEM_RESOURCE_ID_GOLD {
		this.db.Info.SetGold(num)
		this.b_base_prop_chg = true
	} else if id == ITEM_RESOURCE_ID_DIAMOND {
		this.db.Info.SetDiamond(num)
		this.b_base_prop_chg = true
	} else if id == ITEM_RESOURCE_ID_EXP {

	} else {
		n := this.get_item(id)
		if n >= 0 {
			if -n+num > 0 {
				this.add_item(id, -n+num)
			} else {
				this.del_item(id, n-num)
			}
		}
	}
	return num
}

func (this *Player) add_resources(items []int32) {
	if items == nil {
		return
	}

	for i := 0; i < len(items)/2; i++ {
		item_id := items[2*i]
		item_num := items[2*i+1]
		this.add_resource(item_id, item_num)
	}
}

func (this *Player) check_resources(items []int32) bool {
	if items == nil {
		return false
	}

	for i := 0; i < len(items)/2; i++ {
		if this.get_resource(items[2*i]) < items[2*i+1] {
			return false
		}
	}

	return true
}

func (this *Player) cost_resources(items []int32) bool {
	if items == nil {
		return false
	}

	if !this.check_resources(items) {
		return false
	}

	for i := 0; i < len(items)/2; i++ {
		this.add_resource(items[2*i], -items[2*i+1])
	}
	return true
}

func (this *Player) equip(role_id, equip_id int32) int32 {
	var n int32
	var o bool
	if n, o = this.db.Items.GetCount(equip_id); !o {
		return int32(msg_client_message.E_ERR_PLAYER_ITEM_NOT_FOUND)
	}

	if n <= 0 {
		return int32(msg_client_message.E_ERR_PLAYER_ITEM_NUM_NOT_ENOUGH)
	}

	item_tdata := item_table_mgr.Get(equip_id)
	if item_tdata == nil {
		return int32(msg_client_message.E_ERR_PLAYER_ITEM_TABLE_ID_NOT_FOUND)
	}

	if item_tdata.EquipType < 1 || item_tdata.EquipType >= EQUIP_TYPE_MAX {
		return int32(msg_client_message.E_ERR_PLAYER_ITEM_TYPE_NOT_MATCH)
	}

	var equips []int32
	equips, o = this.db.Roles.GetEquip(role_id)
	if !o {
		return int32(msg_client_message.E_ERR_PLAYER_ROLE_NOT_FOUND)
	}

	if equips == nil || len(equips) == 0 {
		equips = make([]int32, EQUIP_TYPE_MAX)
		//this.db.Roles.SetEquip(role_id, equips)
	}

	if equips[item_tdata.EquipType] > 0 {
		this.add_item(equips[item_tdata.EquipType], 1)
	}
	equips[item_tdata.EquipType] = equip_id
	this.db.Roles.SetEquip(role_id, equips)
	this.del_item(equip_id, 1)
	this.roles_id_change_info.id_update(role_id)

	response := &msg_client_message.S2CItemEquipResponse{
		RoleId:    role_id,
		ItemId:    equip_id,
		EquipSlot: item_tdata.EquipType,
	}

	this.Send(uint16(msg_client_message_id.MSGID_S2C_ITEM_EQUIP_RESPONSE), response)

	// 更新排行榜
	this.UpdateRolePowerRank(role_id)

	log.Debug("Player[%v] equip role[%v] item[%v] on equip type[%v]", this.Id, role_id, equip_id, item_tdata.EquipType)

	return 1
}

func (this *Player) unequip(role_id, equip_type int32) int32 {
	equips, o := this.db.Roles.GetEquip(role_id)
	if !o {
		return int32(msg_client_message.E_ERR_PLAYER_ROLE_NOT_FOUND)
	}
	if equips == nil || len(equips) == 0 {
		return int32(msg_client_message.E_ERR_PLAYER_EQUIP_SLOT_EMPTY)
	}

	if equip_type < 1 || equip_type >= EQUIP_TYPE_MAX {
		return int32(msg_client_message.E_ERR_PLAYER_EQUIP_TYPE_INVALID)
	}

	if equips[equip_type] <= 0 {
		return int32(msg_client_message.E_ERR_PLAYER_EQUIP_SLOT_EMPTY)
	}

	this.add_item(equips[equip_type], 1)
	equips[equip_type] = 0
	this.db.Roles.SetEquip(role_id, equips)
	this.roles_id_change_info.id_update(role_id)

	response := &msg_client_message.S2CItemUnequipResponse{
		RoleId:    role_id,
		EquipSlot: equip_type,
	}

	this.Send(uint16(msg_client_message_id.MSGID_S2C_ITEM_UNEQUIP_RESPONSE), response)

	// 更新排行榜
	this.UpdateRolePowerRank(role_id)

	log.Debug("Player[%v] unequip role[%v] equip type[%v]", this.Id, role_id, equip_type)

	return 1
}

func (this *Player) fusion_item(piece_id int32, fusion_num int32) int32 {
	if fusion_num <= 0 || fusion_num >= 1000 {
		log.Error("!!!!!!! Player[%v] fusion num %v too small or big", this.Id, fusion_num)
		return -1
	}

	piece_num := this.get_item(piece_id)
	if piece_num <= 0 {
		log.Error("Player[%v] no piece[%v], cant fusion", this.Id, piece_id)
		return int32(msg_client_message.E_ERR_PLAYER_ITEM_NOT_FOUND)
	}

	piece := item_table_mgr.Get(piece_id)
	if piece == nil {
		log.Error("Cant found item[%v] table data", piece_id)
		return int32(msg_client_message.E_ERR_PLAYER_ITEM_TABLE_ID_NOT_FOUND)
	}

	if piece.ComposeType != 2 && piece.ComposeType != 1 {
		log.Error("Cant fusion item with piece[%v]", piece_id)
		return int32(msg_client_message.E_ERR_PLAYER_ITEM_FUSION_FAILED)
	}

	if piece.ComposeType == 1 && this.db.Roles.NumAll() >= global_config.MaxRoleCount {
		log.Error("Player[%v] role bag is full, cant fusion", this.Id)
		return int32(msg_client_message.E_ERR_PLAYER_ROLE_INVENTORY_NOT_ENOUGH_SPACE)
	}

	if piece.ComposeNum*fusion_num > piece_num {
		log.Error("Player[%v] piece[%v] not enough to fusion", this.Id, piece_id)
		return int32(msg_client_message.E_ERR_PLAYER_ITEM_COUNT_NOT_ENOUGH_TO_FUSION)
	}

	items := make(map[int32]int32)
	i := int32(0)
	for i < fusion_num {
		o, item := this.drop_item_by_id(piece.ComposeDropID, true, nil)
		if !o || item == nil {
			log.Error("Player[%v] fusion item with piece[%v] and drop_id[%v] failed", this.Id, piece_id, piece.ComposeDropID)
			return int32(msg_client_message.E_ERR_PLAYER_ITEM_FUSION_FAILED)
		}
		items[item.Id] += item.Value
		i++
		if piece.ComposeType == 1 && this.db.Roles.NumAll() >= global_config.MaxRoleCount {
			break
		}
	}

	this.del_item(piece_id, i*piece.ComposeNum)

	response := &msg_client_message.S2CItemFusionResponse{
		Items: Map2ItemInfos(items),
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_ITEM_FUSION_RESPONSE), response)

	log.Debug("Player[%v] fusioned items[%v] with piece[%v,%v]", this.Id, items, piece_id, fusion_num)

	return 1
}

func (this *Player) item_to_resource(item_id int32) (items_map map[int32]int32) {
	item := item_table_mgr.Get(item_id)
	if item == nil {
		log.Error("Cant found item[%v] table data", item_id)
		return nil
	}

	if item.SellReward != nil {
		for i := 0; i < len(item.SellReward)/2; i++ {
			this.add_resource(item.SellReward[2*i], item.SellReward[2*i+1])
			if items_map == nil {
				items_map = make(map[int32]int32)
			}
			items_map[item.SellReward[2*i]] += item.SellReward[2*i+1]
		}
	}
	return
}

func (this *Player) sell_item(item_id, item_num int32, send_msg bool) int32 {
	if item_num <= 0 {
		log.Error("Player[%v] cant sell item with num[%v]", this.Id, item_num)
		return -1
	}

	item := item_table_mgr.Get(item_id)
	if item == nil {
		log.Error("Cant found item[%v] table data", item_id)
		return int32(msg_client_message.E_ERR_PLAYER_ITEM_TABLE_ID_NOT_FOUND)
	}

	if this.get_item(item_id) < item_num {
		log.Error("Player[%v] item[%v] not enough", this.Id, item_id)
		return int32(msg_client_message.E_ERR_PLAYER_ITEM_NUM_NOT_ENOUGH)
	}

	this.del_item(item_id, item_num)
	if item.SellReward != nil {
		for i := 0; i < len(item.SellReward)/2; i++ {
			this.add_resource(item.SellReward[2*i], item_num*item.SellReward[2*i+1])
		}
	}

	if send_msg {
		response := &msg_client_message.S2CItemSellResponse{
			ItemId:  item_id,
			ItemNum: item_num,
		}
		this.Send(uint16(msg_client_message_id.MSGID_S2C_ITEM_SELL_RESPONSE), response)
	}

	log.Debug("Player[%v] sell item[%v,%v], get items[%v]", this.Id, item_id, item_num, item.SellReward)

	return 1
}

func (this *Player) item_upgrade(role_id, item_id, item_num, upgrade_type int32) int32 {
	item := item_table_mgr.Get(item_id)
	if item == nil {
		log.Error("Player[%v] upgrade role[%v] item[%v] with upgrade_type[%v] failed, because item[%v] table data not found", this.Id, role_id, item_id, upgrade_type, item_id)
		return int32(msg_client_message.E_ERR_PLAYER_ITEM_TABLE_ID_NOT_FOUND)
	}
	if item.Type != ITEM_TYPE_EQUIP {
		log.Error("Player[%v] upgrade item[%v] invalid", this.Id, item_id)
		return int32(msg_client_message.E_ERR_PLAYER_ITEM_UPGRADE_TYPE_INVALID)
	}

	if item_num <= 0 {
		item_num = 1
	}

	// 左槽 右槽
	var equips []int32
	if item.EquipType == EQUIP_TYPE_LEFT_SLOT || item.EquipType == EQUIP_TYPE_RELIC {
		if !this.db.Roles.HasIndex(role_id) {
			log.Error("Player[%v] upgrade left slot equip[%v] failed, role[%v] not found", this.Id, item_id, role_id)
			return int32(msg_client_message.E_ERR_PLAYER_ROLE_NOT_FOUND)
		}
		equips, _ = this.db.Roles.GetEquip(role_id)
		if equips == nil || len(equips) < EQUIP_TYPE_MAX {
			log.Error("Player[%v] role[%v] no equips", this.Id, role_id)
			return -1
		}
		if equips[item.EquipType] != item_id {
			log.Error("Player[%v] equip pos[%v] no item[%v]", this.Id, item.EquipType, item_id)
			return int32(msg_client_message.E_ERR_PLAYER_ITEM_UPGRADE_TYPE_INVALID)
		}
	} else {
		if this.get_resource(item_id) < item_num {
			log.Error("Player[%v] upgrade item[%v] failed, item[%] not enough", this.Id, item_id, item_id)
			return int32(msg_client_message.E_ERR_PLAYER_ITEM_NOT_FOUND)
		}
	}

	item_upgrade := item_upgrade_table_mgr.GetByItemId(item_id)
	if item_upgrade == nil {
		return int32(msg_client_message.E_ERR_PLAYER_ITEM_UPGRADE_DATA_NOT_FOUND)
	}

	if item.EquipType == EQUIP_TYPE_LEFT_SLOT {
		for {
			if item_upgrade.UpgradeType == upgrade_type {
				break
			}
			item_upgrade = item_upgrade.Next
			if item_upgrade == nil {
				break
			}
		}
		if item_upgrade == nil {
			log.Error("Player[%v] no upgrade table data for role[%v] item[%v] upgrade_type[%v]", this.Id, role_id, item_id, upgrade_type)
			return int32(msg_client_message.E_ERR_PLAYER_ITEM_UPGRADE_FAILED)
		}
	}

	// 检测消耗物品
	for i := 0; i < len(item_upgrade.ResCondition)/2; i++ {
		res_id := item_upgrade.ResCondition[2*i]
		res_num := item_upgrade.ResCondition[2*i+1] * item_num
		if this.get_resource(res_id) < res_num {
			log.Error("Player[%v] upgrade item[%v] failed, res[%v] not enough", this.Id, item_id, res_id)
			return int32(msg_client_message.E_ERR_PLAYER_ITEM_UPGRADE_RES_NOT_ENOUGH)
		}
	}

	var new_items map[int32]int32
	if item.EquipType == EQUIP_TYPE_LEFT_SLOT || item.EquipType == EQUIP_TYPE_RELIC {
		o, new_item := this.drop_item_by_id(item_upgrade.ResultDropId, false, nil)
		if !o {
			log.Error("Player[%v] upgrade item[%v] failed, drop error", this.Id, item_id)
			return int32(msg_client_message.E_ERR_PLAYER_ITEM_UPGRADE_FAILED)
		}
		if item.EquipType == EQUIP_TYPE_LEFT_SLOT && upgrade_type == 3 {
			this.db.Equip.SetTmpSaveLeftSlotRoleId(role_id)
			this.db.Equip.SetTmpLeftSlotItemId(new_item.Id)
		} else {
			equips[item.EquipType] = new_item.Id
			this.db.Roles.SetEquip(role_id, equips)
			this.roles_id_change_info.id_update(role_id)
		}
		if new_items == nil {
			new_items = make(map[int32]int32)
		}
		new_items[new_item.GetId()] += new_item.GetValue()
	} else {
		for i := int32(0); i < item_num; i++ {
			o, new_item := this.drop_item_by_id(item_upgrade.ResultDropId, true, nil)
			if !o {
				log.Error("Player[%v] upgrade item[%v] failed, drop error", this.Id, item_id)
				return int32(msg_client_message.E_ERR_PLAYER_ITEM_UPGRADE_FAILED)
			}
			this.add_resource(item_id, -1)
			if new_items == nil {
				new_items = make(map[int32]int32)
			}
			new_items[new_item.GetId()] += new_item.GetValue()
		}
	}

	// 消耗物品
	for i := 0; i < len(item_upgrade.ResCondition)/2; i++ {
		res_id := item_upgrade.ResCondition[2*i]
		res_num := item_upgrade.ResCondition[2*i+1] * item_num
		if res_num > 0 && res_id == item_id {
			res_num -= item_num
		}
		this.add_resource(res_id, -res_num)
	}

	response := &msg_client_message.S2CItemUpgradeResponse{
		RoleId:   role_id,
		NewItems: Map2ItemInfos(new_items),
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_ITEM_UPGRADE_RESPONSE), response)

	// 更新任务
	this.TaskUpdate(table_config.TASK_COMPLETE_TYPE_FORGE_EQUIP_NUM, false, 0, item_num)

	// 更新排行榜
	this.UpdateRolePowerRank(role_id)

	log.Debug("Player[%v] upgraded item[%v] to new item[%v]", this.Id, item_id, new_items)

	return 1
}

func (this *Player) item_one_key_upgrade(item_id int32, cost_items map[int32]int32, result_items map[int32]int32) int32 {
	var next_item_id, result_item_id, res int32
	next_item_id = item_id
	res = 1
	for {
		item := item_table_mgr.Get(next_item_id)
		if item == nil {
			log.Error("item[%v] table data not found on item one key upgrade", next_item_id)
			return int32(msg_client_message.E_ERR_PLAYER_ITEM_TABLE_ID_NOT_FOUND)
		}

		if item.EquipType < 1 && item.EquipType >= EQUIP_TYPE_LEFT_SLOT {
			res = 0
			break
		}

		item_upgrade := item_upgrade_table_mgr.GetByItemId(next_item_id)
		if item_upgrade == nil {
			if next_item_id == item_id {
				res = 0
			}
			break
		}

		if item_upgrade.ResCondition == nil {
			if next_item_id == item_id {
				res = 0
			}
			break
		}

		if this.get_item(next_item_id) < 1 {
			if next_item_id == item_id {
				res = 0
			}
			break
		}

		has_gold := false
		for n := 0; n < len(item_upgrade.ResCondition)/2; n++ {
			if item_upgrade.ResCondition[2*n] == ITEM_RESOURCE_ID_GOLD {
				has_gold = true
				break
			}
		}

		for n := 0; n < len(item_upgrade.ResCondition)/2; n++ {
			res_id := item_upgrade.ResCondition[2*n]
			res_num := item_upgrade.ResCondition[2*n+1]
			num := this.get_resource(res_id)
			if num < res_num {
				if !this.already_upgrade {
					if !has_gold && res_id == ITEM_RESOURCE_ID_GOLD {
						log.Error("Player[%v] item one key upgrade[%v] failed, not enough gold, need[%v] now[%v]", this.Id, item_id, res_num, num)
						return int32(msg_client_message.E_ERR_PLAYER_GOLD_NOT_ENOUGH)
					} else {
						log.Error("Player[%v] item one key upgrade[%v] failed, material[%v] num[%v] not enough, need[%v]", this.Id, item_id, res_id, num, res_num)
						return int32(msg_client_message.E_ERR_PLAYER_ITEM_ONE_KEY_UPGRADE_NOT_ENOUGH_MATERIAL)
					}
				}
				res = 0
				break
			}
		}

		if res == 0 {
			if next_item_id != item_id {
				res = 1
			}
			break
		}

		drop_data := drop_table_mgr.Map[item_upgrade.ResultDropId]
		if drop_data == nil {
			res = 0
			log.Error("Drop id[%v] data not found on player[%v] one key upgrade item", item_upgrade.ResultDropId, this.Id)
			break
		}

		if drop_data.DropItems == nil || len(drop_data.DropItems) == 0 {
			res = 0
			break
		}

		item = item_table_mgr.Get(drop_data.DropItems[0].DropItemID)
		if item == nil {
			res = 0
			break
		}

		// 新生成装备
		result_item_id = drop_data.DropItems[0].DropItemID
		this.add_item(result_item_id, 1)
		result_items[result_item_id] += 1

		// 消耗资源
		for n := 0; n < len(item_upgrade.ResCondition)/2; n++ {
			res_id := item_upgrade.ResCondition[2*n]
			res_num := item_upgrade.ResCondition[2*n+1]
			if res_num > 0 && res_id == next_item_id {
				res_num -= 1
			}
			this.add_resource(res_id, -res_num)
			cost_items[res_id] += res_num
		}

		// 删除老装备
		this.del_item(next_item_id, 1)
		cost_items[next_item_id] += 1

		next_item_id = result_item_id
		this.already_upgrade = true
	}
	return res
}

func (this *Player) items_one_key_upgrade(item_ids []int32) int32 {
	this.already_upgrade = false
	cost_items := make(map[int32]int32)
	result_items := make(map[int32]int32)
	for i := 0; i < len(item_ids); i++ {
		for {
			res := this.item_one_key_upgrade(item_ids[i], cost_items, result_items)
			if res < 0 {
				return res
			}
			if res == 0 {
				break
			}
		}
	}

	response := &msg_client_message.S2CItemOneKeyUpgradeResponse{
		ItemIds:     item_ids,
		CostItems:   Map2ItemInfos(cost_items),
		ResultItems: Map2ItemInfos(result_items),
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_ITEM_ONEKEY_UPGRADE_RESPONSE), response)

	// 更新任务
	this.TaskUpdate(table_config.TASK_COMPLETE_TYPE_FORGE_EQUIP_NUM, false, 0, 1)

	log.Debug("Player[%v] item one key upgrade result: %v", this.Id, response)

	return 1
}

func C2SItemFusionHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SItemFusionRequest
	err := proto.Unmarshal(msg_data, &req)
	if nil != err {
		log.Error("Unmarshal msg failed err(%s) !", err.Error())
		return -1
	}
	return p.fusion_item(req.GetPieceId(), req.GetFusionNum())
}

func C2SItemSellHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SItemSellRequest
	err := proto.Unmarshal(msg_data, &req)
	if nil != err {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}
	return p.sell_item(req.GetItemId(), req.GetItemNum(), true)
}

func C2SItemEquipHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SItemEquipRequest
	err := proto.Unmarshal(msg_data, &req)
	if nil != err {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}
	return p.equip(req.GetRoleId(), req.GetItemId())
}

func C2SItemUnequipHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SItemUnequipRequest
	err := proto.Unmarshal(msg_data, &req)
	if nil != err {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.unequip(req.GetRoleId(), req.GetEquipSlot())
}

func C2SItemUpgradeHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SItemUpgradeRequest
	err := proto.Unmarshal(msg_data, &req)
	if nil != err {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.item_upgrade(req.GetRoleId(), req.GetItemId(), req.GetItemNum(), req.GetUpgradeType())
}

func C2SItemOneKeyUpgradeHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SItemOneKeyUpgradeRequest
	err := proto.Unmarshal(msg_data, &req)
	if nil != err {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.items_one_key_upgrade(req.GetItemIds())
}
