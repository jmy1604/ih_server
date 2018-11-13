package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlAccelCostItem struct {
	Times int32 `xml:"AccelTimes,attr"`
	Cost  int32 `xml:"Cost,attr"`
}

type XmlAccelCostConfig struct {
	Items []XmlAccelCostItem `xml:"item"`
}

type AccelCostTableMgr struct {
	Map   map[int32]*XmlAccelCostItem
	Array []*XmlAccelCostItem
}

func (this *AccelCostTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("AccelCostTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *AccelCostTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "AccelCost.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("AccelCostTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlAccelCostConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("AccelCostTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlAccelCostItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlAccelCostItem, 0)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlAccelCostItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		this.Map[tmp_item.Times] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *AccelCostTableMgr) Get(times int32) *XmlAccelCostItem {
	return this.Map[times]
}
