package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"io/ioutil"
)

type XmlExtractItem struct {
	Id             int32  `xml:"Id,attr"`
	DropIdStr      string `xml:"dropID,attr"`
	DropItems      []*ItemInfo
	CostStr        string `xml:"cost,attr"`
	CostId         int32
	CostNum        int32
	FirstDropIdStr string `xml:"FirstDropID,attr"`
	FirstDropIds   []int32
}

type XmlExtractConfig struct {
	Items []*XmlExtractItem `xml:"item"`
}

type ExtractTableManager struct {
	Map map[int32]*XmlExtractItem
}

func (this *ExtractTableManager) Init() bool {
	if !this.Load() {
		return false
	}
	return true
}

func (this *ExtractTableManager) Load() bool {
	data, err := ioutil.ReadFile("../src/ih_server/game_data/extract.xml")
	if nil != err {
		log.Error("ExtractTableManager load read file failed[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlExtractConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("ExtractTableManager load xml unmarshal failed [%s]!", err.Error())
		return false
	}

	this.Map = make(map[int32]*XmlExtractItem)

	tmp_len := len(tmp_cfg.Items)
	var tmp_item *XmlExtractItem
	for i := 0; i < tmp_len; i++ {
		tmp_item = tmp_cfg.Items[i]
		if nil == tmp_item {
			continue
		}

		drop_ids := parse_xml_str_arr(tmp_item.DropIdStr, ",")
		if drop_ids == nil || len(drop_ids) < 2 || len(drop_ids)%2 != 0 {
			log.Error("ExtractTableManager Column[%v] Field[%v] parse failed", i, tmp_item.DropIdStr)
			return false
		}
		for n := 0; n < (len(drop_ids) / 2); n++ {
			if tmp_item.DropItems == nil {
				tmp_item.DropItems = make([]*ItemInfo, len(drop_ids)/2)
			}
			tmp_item.DropItems[n] = &ItemInfo{}
			tmp_item.DropItems[n].Id = drop_ids[2*n]
			tmp_item.DropItems[n].Num = drop_ids[2*n+1]
		}

		cost_info := parse_xml_str_arr(tmp_item.CostStr, ",")
		if cost_info == nil || len(cost_info) < 2 {
			log.Error("ExtractTableManager Column[%v] Field[%v] parse failed", i, tmp_item.CostStr)
			return false
		}
		tmp_item.CostId = cost_info[0]
		tmp_item.CostNum = cost_info[1]
		first_drop_ids := parse_xml_str_arr(tmp_item.FirstDropIdStr, ",")
		if first_drop_ids == nil || len(first_drop_ids)%2 != 0 {
			log.Error("ExtractTableManager Column[%v] Field[%v] parse failed", i, tmp_item.FirstDropIdStr)
			return false
		}
		tmp_item.FirstDropIds = first_drop_ids
		this.Map[tmp_item.Id] = tmp_item
	}

	return true
}

func (this *ExtractTableManager) Get(id int32) *XmlExtractItem {
	return this.Map[id]
}
