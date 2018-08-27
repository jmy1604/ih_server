package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
	"math/rand"
	"sync"
)

const (
	SHOP_TYPE_NONE = iota
	SHOP_TYPE_SPECIAL
	SHOP_TYPE_FRIEND_POINTS
	SHOP_TYPE_CHARM_MEDAL
	SHOP_TYPE_RMB
	SHOP_TYPE_SOUL_STONE
)

type XmlShopItemItem struct {
	Id           int32  `xml:"GoodID,attr"`
	ShopId       int32  `xml:"ShopID,attr"`
	ItemStr      string `xml:"ItemList,attr"`
	Item         []int32
	BuyCostStr   string `xml:"BuyCost,attr"`
	BuyCost      []int32
	StockNum     int32 `xml:"StockNum,attr"`
	RandomWeight int32 `xml:"RandomWeight,attr"`
	LevelMin     int32 `xml:"LevelMin,attr"`
	LevelMax     int32 `xml:"LevelMax,attr"`
}

type XmlShopItemConfig struct {
	Items []*XmlShopItemItem `xml:"item"`
}

type LevelShopItems struct {
	Items        []*XmlShopItemItem
	total_weight int32
}

type ItemsShop struct {
	items        []*XmlShopItemItem
	total_weight int32
	level2items  map[int32]*LevelShopItems
	locker       *sync.RWMutex
}

type ShopItemTableManager struct {
	items_map   map[int32]*XmlShopItemItem
	items_array []*XmlShopItemItem
	shops_map   map[int32]*ItemsShop
}

func (this *ShopItemTableManager) Init(table_file string) bool {
	if table_file == "" {
		table_file = "ShopItem.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("ShopItemTableManager Load read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlShopItemConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("ShopItemTableManager Load xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	tmp_len := int32(len(tmp_cfg.Items))

	this.items_map = make(map[int32]*XmlShopItemItem)
	this.items_array = []*XmlShopItemItem{}
	this.shops_map = make(map[int32]*ItemsShop)
	for i := int32(0); i < tmp_len; i++ {
		c := tmp_cfg.Items[i]
		c.Item = parse_xml_str_arr2(c.ItemStr, ",")
		if c.Item == nil || len(c.Item)%2 != 0 {
			log.Error("ShopItemTableManager parse index[%v] column with value[%v] for field[ItemList] invalid", i, c.ItemStr)
			return false
		}
		c.BuyCost = parse_xml_str_arr2(c.BuyCostStr, ",")
		if c.BuyCost == nil || len(c.BuyCost)%2 != 0 {
			log.Error("ShopItemTableManager parse index[%v] column with value[%v] for field[BuyCost] invalid", i, c.BuyCostStr)
			return false
		}
		shop := this.shops_map[c.ShopId]
		if shop == nil {
			shop = &ItemsShop{}
			shop.level2items = make(map[int32]*LevelShopItems)
			shop.locker = &sync.RWMutex{}
			this.shops_map[c.ShopId] = shop
		}
		shop.items = append(shop.items, c)
		shop.total_weight += c.RandomWeight
		this.items_map[c.Id] = c
		this.items_array = append(this.items_array, c)
	}

	log.Info("Shop table load items count(%v)", tmp_len)

	return true
}

func (this *ShopItemTableManager) GetItem(item_id int32) *XmlShopItemItem {
	return this.items_map[item_id]
}

func (this *ShopItemTableManager) GetItems() map[int32]*XmlShopItemItem {
	return this.items_map
}

func (this *ShopItemTableManager) RandomShopItem(shop_id int32) *XmlShopItemItem {
	shop := this.shops_map[shop_id]
	if shop == nil {
		return nil
	}

	if shop.total_weight <= 0 {
		return nil
	}

	r := rand.Int31n(shop.total_weight)
	for _, item := range shop.items {
		if r < item.RandomWeight {
			return item
		}
		r -= item.RandomWeight
	}
	return nil
}

func (this *ShopItemTableManager) GetItemsShop(shop_id int32) []*XmlShopItemItem {
	shop := this.shops_map[shop_id]
	if shop == nil {
		return nil
	}
	return shop.items
}

func (this *ShopItemTableManager) RandomShopItemByPlayerLevel(shop_id, level int32) (item *XmlShopItemItem) {
	shop := this.shops_map[shop_id]
	if shop == nil {
		return nil
	}
	if shop.total_weight <= 0 {
		return nil
	}

	shop.locker.RLock()
	level_items := shop.level2items[level]
	if level_items != nil {
		item = level_items.RandomItem()
		shop.locker.RUnlock()
	} else {
		shop.locker.RUnlock()
		shop.locker.Lock()
		// double check
		level_items = shop.level2items[level]
		if level_items == nil {
			level_items = &LevelShopItems{}
			for _, it := range shop.items {
				if level >= it.LevelMin && level <= it.LevelMax {
					level_items.Items = append(level_items.Items, it)
					level_items.total_weight += it.RandomWeight
				}
			}
			shop.level2items[level] = level_items
		}
		item = level_items.RandomItem()
		shop.locker.Unlock()
	}
	return item
}

func (this *LevelShopItems) RandomItem() *XmlShopItemItem {
	r := rand.Int31n(this.total_weight)
	for _, item := range this.Items {
		if r < item.RandomWeight {
			return item
		}
		r -= item.RandomWeight
	}
	return nil
}
