package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlItemItem struct {
	Id            int32  `xml:"ID,attr"`
	Type          int32  `xml:"ItemType,attr"`
	MaxCount      string `xml:"MaxCount,attr"`
	EquipType     int32  `xml:"EquipType,attr"`
	Quality       int32  `xml:"Quality,attr"`
	EquipAttrStr  string `xml:"EquipAttr,attr"`
	EquipSkillStr string `xml:"EquipSkill,attr"`
	EquipAttr     []int32
	EquipSkill    []int32
	ComposeNum    int32  `xml:"ComposeNum,attr"`
	ComposeType   int32  `xml:"ComposeType,attr"`
	ComposeDropID int32  `xml:"ComposeDropID,attr"`
	SellRewardStr string `xml:"SellReward,attr"`
	SellReward    []int32
	BattlePower   int32 `xml:"BattlePower,attr"`
	SuitId        int32 `xml:"SuitID,attr"`
}

type XmlItemConfig struct {
	Items []XmlItemItem `xml:"item"`
}

type ItemTableMgr struct {
	Map   map[int32]*XmlItemItem
	Array []*XmlItemItem
}

func (this *ItemTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("ItemTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *ItemTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "Item.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("ItemTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlItemConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("ItemTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlItemItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlItemItem, 0)
	}

	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlItemItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		if tmp_item.EquipAttrStr != "" {
			a := parse_xml_str_arr2(tmp_item.EquipAttrStr, ",")
			if a == nil {
				log.Error("ItemTableMgr parse EquipAttrStr with [%v] failed", tmp_item.EquipAttrStr)
				return false
			} else {
				tmp_item.EquipAttr = a
			}
		}

		if tmp_item.EquipSkillStr != "" {
			a := parse_xml_str_arr2(tmp_item.EquipSkillStr, ",")
			if a == nil {
				log.Error("ItemTableMgr parse EquipSkillStr with [%v] failed", tmp_item.EquipSkillStr)
				return false
			} else {
				tmp_item.EquipSkill = a
			}
		}

		if tmp_item.SellRewardStr != "" {
			a := parse_xml_str_arr2(tmp_item.SellRewardStr, ",")
			if a == nil {
				log.Error("ItemTableMgr parse SellRewardStr with [%v] failed", tmp_item.SellRewardStr)
				return false
			} else {
				tmp_item.SellReward = a
			}
		}

		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *ItemTableMgr) Get(id int32) *XmlItemItem {
	return this.Map[id]
}

func (this *ItemTableMgr) GetBattlePower(id int32) int32 {
	item := this.Map[id]
	if item == nil {
		return -1
	}
	return item.BattlePower
}
