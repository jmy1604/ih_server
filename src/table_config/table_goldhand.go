package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlGoldHandItem struct {
	Level       int32 `xml:"Level,attr"`
	GoldReward1 int32 `xml:"GoldReward1,attr"`
	GemCost1    int32 `xml:"GemCost1,attr"`
	GoldReward2 int32 `xml:"GoldReward2,attr"`
	GemCost2    int32 `xml:"GemCost2,attr"`
	GoldReward3 int32 `xml:"GoldReward3,attr"`
	GemCost3    int32 `xml:"GemCost3,attr"`
	RefreshCD   int32 `xml:"RefreshCD,attr"`
}

type XmlGoldHandConfig struct {
	Items []XmlGoldHandItem `xml:"item"`
}

type GoldHandTableMgr struct {
	Map   map[int32]*XmlGoldHandItem
	Array []*XmlGoldHandItem
}

func (this *GoldHandTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("GoldHandTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *GoldHandTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "GoldHand.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("GoldHandTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlGoldHandConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("GoldHandTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlGoldHandItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlGoldHandItem, 0)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlGoldHandItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		this.Map[tmp_item.Level] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *GoldHandTableMgr) Get(level int32) *XmlGoldHandItem {
	return this.Map[level]
}
