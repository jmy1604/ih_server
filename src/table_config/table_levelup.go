package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlLevelUpItem struct {
	Level               int32  `xml:"Level,attr"`
	PlayerLevelUpExp    int32  `xml:"PlayerLevelUpExp,attr"`
	CardLevelUpResStr   string `xml:"CardLevelUpRes,attr"`
	CardLevelUpRes      []int32
	CardDecomposeResStr string `xml:"CardDecomposeRes,attr"`
	CardDecomposeRes    []int32
}

type XmlLevelUpConfig struct {
	Items []XmlLevelUpItem `xml:"item"`
}

type LevelUpTableMgr struct {
	Map   map[int32]*XmlLevelUpItem
	Array []*XmlLevelUpItem
}

func (this *LevelUpTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("LevelUpTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *LevelUpTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "LevelUp.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("LevelUpTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlLevelUpConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("LevelUpTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlLevelUpItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlLevelUpItem, 0)
	}

	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlLevelUpItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		if tmp_item.CardLevelUpResStr != "" {
			a := parse_xml_str_arr2(tmp_item.CardLevelUpResStr, ",")
			if a == nil {
				log.Error("ItemTableMgr parse CardLevelUpResStr with [%v] failed", tmp_item.CardLevelUpResStr)
				return false
			} else {
				tmp_item.CardLevelUpRes = a
			}
		}

		if tmp_item.CardDecomposeResStr != "" {
			a := parse_xml_str_arr2(tmp_item.CardDecomposeResStr, ",")
			if a == nil {
				log.Error("ItemTableMgr parse CardDecomposeResStr with [%v] failed", tmp_item.CardDecomposeResStr)
				return false
			} else {
				tmp_item.CardDecomposeRes = a
			}
		}

		this.Map[tmp_item.Level] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *LevelUpTableMgr) Get(level int32) *XmlLevelUpItem {
	return this.Map[level]
}
