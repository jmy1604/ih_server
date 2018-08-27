package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlItemUpgradeItem struct {
	Id             int32  `xml:"UpgradeID,attr"`
	ItemId         int32  `xml:"ItemID,attr"`
	UpgradeType    int32  `xml:"UpgradeType,attr"`
	ResultDropId   int32  `xml:"ResultDropID,attr"`
	ResCondtionStr string `xml:"ResCondtion,attr"`
	ResCondition   []int32
	Next           *XmlItemUpgradeItem
}

type XmlItemUpgradeConfig struct {
	Items []XmlItemUpgradeItem `xml:"item"`
}

type ItemUpgradeTableMgr struct {
	Map   map[int32]*XmlItemUpgradeItem
	Array []*XmlItemUpgradeItem
	Items map[int32]*XmlItemUpgradeItem
}

func (this *ItemUpgradeTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("ItemUpgradeTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *ItemUpgradeTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "ItemUpgrade.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("ItemUpgradeTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlItemUpgradeConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("ItemUpgradeTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlItemUpgradeItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlItemUpgradeItem, 0)
	}
	if this.Items == nil {
		this.Items = make(map[int32]*XmlItemUpgradeItem)
	}

	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlItemUpgradeItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		if tmp_item.ResCondtionStr != "" {
			a := parse_xml_str_arr2(tmp_item.ResCondtionStr, ",")
			if a == nil {
				log.Error("ItemUpgradeTableMgr parse ResCondtion with [%v] failed", tmp_item.ResCondtionStr)
				return false
			} else {
				tmp_item.ResCondition = a
			}
		}

		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
		item := this.Items[tmp_item.ItemId]
		if item == nil {
			this.Items[tmp_item.ItemId] = tmp_item
		} else {
			i := item
			for ; i.Next != nil; i = i.Next {
			}
			i.Next = tmp_item
		}
	}

	return true
}

func (this *ItemUpgradeTableMgr) Get(id int32) *XmlItemUpgradeItem {
	return this.Map[id]
}

func (this *ItemUpgradeTableMgr) GetByItemId(item_id int32) *XmlItemUpgradeItem {
	return this.Items[item_id]
}
