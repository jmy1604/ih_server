package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlShopItem struct {
	Id              int32  `xml:"ID,attr"`
	Type            int32  `xml:"ShopType,attr"`
	ShopMaxSlot     int32  `xml:"ShopMaxSlot,attr"`
	AutoRefreshTime string `xml:"AutoRefreshTime,attr"`
	FreeRefreshTime int32  `xml:"FreeRefreshTime,attr"`
	RefreshResStr   string `xml:"RefreshRes,attr"`
	RefreshRes      []int32
}

func (this *XmlShopItem) NoRefresh() bool {
	return this.AutoRefreshTime == "" && this.FreeRefreshTime == 0
}

type XmlShopConfig struct {
	Items []*XmlShopItem `xml:"item"`
}

type ShopTableManager struct {
	shops_map   map[int32]*XmlShopItem
	shops_array []*XmlShopItem
}

func (this *ShopTableManager) Init(table_file string) bool {
	if table_file == "" {
		table_file = "Shop.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("ShopTableManager Load read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlShopConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("ShopTableManager Load xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	this.shops_map = make(map[int32]*XmlShopItem)
	this.shops_array = []*XmlShopItem{}
	for i := 0; i < len(tmp_cfg.Items); i++ {
		c := tmp_cfg.Items[i]
		c.RefreshRes = parse_xml_str_arr2(c.RefreshResStr, ",")
		if c.RefreshRes == nil || len(c.RefreshRes)%2 != 0 {
			return false
		}
		this.shops_map[c.Id] = c
		this.shops_array = append(this.shops_array, c)
	}

	log.Info("Shop table load items count(%v)", len(tmp_cfg.Items))

	return true
}

func (this *ShopTableManager) Get(shop_id int32) *XmlShopItem {
	return this.shops_map[shop_id]
}
