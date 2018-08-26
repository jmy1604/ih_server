package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"io/ioutil"
)

type XmlDrawItem struct {
	Id               int32  `xml:"Id,attr"`
	DropIdStr        string `xml:"DropID,attr"`
	DropId           []int32
	ResCondition1Str string `xml:"ResCondition1,attr"`
	ResCondition1    []int32
	ResCondition2Str string `xml:"ResCondition2,attr"`
	ResCondition2    []int32
	FreeExtractTime  int32 `xml:"FreeExtractTime,attr"`
}

type XmlDrawConfig struct {
	Items []XmlDrawItem `xml:"item"`
}

type DrawTableMgr struct {
	Map   map[int32]*XmlDrawItem
	Array []*XmlDrawItem
}

func (this *DrawTableMgr) Init() bool {
	if !this.Load() {
		log.Error("DrawTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *DrawTableMgr) Load() bool {
	data, err := ioutil.ReadFile("../game_data/Extract.xml")
	if nil != err {
		log.Error("DrawTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlDrawConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("DrawTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlDrawItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlDrawItem, 0)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlDrawItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]
		tmp_item.DropId = parse_xml_str_arr2(tmp_item.DropIdStr, ",")
		if tmp_item.DropId == nil || len(tmp_item.DropId)%2 != 0 {
			log.Error("DrawTableMgr field[DropId] value[%v] with index[%v] invalid", tmp_item.DropIdStr, idx)
			return false
		}
		tmp_item.ResCondition1 = parse_xml_str_arr2(tmp_item.ResCondition1Str, ",")
		if tmp_item.ResCondition1 == nil || len(tmp_item.ResCondition1)%2 != 0 {
			log.Error("DrawTableMgr field[ResCondition1] value[%v] with index[%v] invalid", tmp_item.ResCondition1Str, idx)
			return false
		}
		tmp_item.ResCondition2 = parse_xml_str_arr2(tmp_item.ResCondition2Str, ",")
		if tmp_item.ResCondition2 == nil || len(tmp_item.ResCondition2)%2 != 0 {
			log.Error("DrawTableMgr field[ResCondition2] value[%v] with index[%v] invalid", tmp_item.ResCondition2Str, idx)
			return false
		}
		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *DrawTableMgr) Get(tower_id int32) *XmlDrawItem {
	return this.Map[tower_id]
}
