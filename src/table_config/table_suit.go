package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlSuitItem struct {
	Id           int32  `xml:"SuitID,attr"`
	AttrSuit2Str string `xml:"AttrSuit2,attr"`
	AttrSuit3Str string `xml:"AttrSuit3,attr"`
	AttrSuit4Str string `xml:"AttrSuit4,attr"`
	BpowerSuit2  int32  `xml:"BpowerSuit2,attr"`
	BpowerSuit3  int32  `xml:"BpowerSuit3,attr"`
	BpowerSuit4  int32  `xml:"BpowerSuit4,attr"`
	SuitAddAttrs map[int32][]int32
	SuitPowers   map[int32]int32
}

type XmlSuitConfig struct {
	Items []XmlSuitItem `xml:"item"`
}

type SuitTableMgr struct {
	Map   map[int32]*XmlSuitItem
	Array []*XmlSuitItem
}

func (this *SuitTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("SuitTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *SuitTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "Suit.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("SuitTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlSuitConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("SuitTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlSuitItem)
	}

	if this.Array == nil {
		this.Array = make([]*XmlSuitItem, 0)
	}

	tmp_len := int32(len(tmp_cfg.Items))
	var tmp_item *XmlSuitItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		suitattrs := parse_xml_str_arr2(tmp_item.AttrSuit2Str, ",")
		if suitattrs == nil || len(suitattrs)%2 != 0 {
			log.Error("Suit table parse field AttrSuit2[%v] error", tmp_item.AttrSuit2Str)
			return false
		}
		if tmp_item.SuitAddAttrs == nil {
			tmp_item.SuitAddAttrs = make(map[int32][]int32)
		}
		tmp_item.SuitAddAttrs[2] = suitattrs

		suitattrs = parse_xml_str_arr2(tmp_item.AttrSuit3Str, ",")
		if suitattrs == nil || len(suitattrs)%2 != 0 {
			log.Error("Suit table parse field AttrSuit3[%v] error", tmp_item.AttrSuit3Str)
			return false
		}
		tmp_item.SuitAddAttrs[3] = suitattrs

		suitattrs = parse_xml_str_arr2(tmp_item.AttrSuit4Str, ",")
		if suitattrs == nil || len(suitattrs)%2 != 0 {
			log.Error("Suit table parse field AttrSuit4[%v] error", tmp_item.AttrSuit4Str)
			return false
		}
		tmp_item.SuitAddAttrs[4] = suitattrs

		if tmp_item.SuitPowers == nil {
			tmp_item.SuitPowers = make(map[int32]int32)
		}
		tmp_item.SuitPowers[2] = tmp_item.BpowerSuit2
		tmp_item.SuitPowers[3] = tmp_item.BpowerSuit3
		tmp_item.SuitPowers[4] = tmp_item.BpowerSuit4

		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *SuitTableMgr) Has(id int32) bool {
	if d := this.Map[id]; d == nil {
		return false
	}
	return true
}

func (this *SuitTableMgr) Get(id int32) *XmlSuitItem {
	return this.Map[id]
}
