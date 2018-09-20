package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlPayItem struct {
	Id       int32  `xml:"ID,attr"`
	BundleId string `xml:"BundleID,attr"`
}

type XmlPayConfig struct {
	Items []XmlPayItem `xml:"item"`
}

type PayTableMgr struct {
	Map   map[int32]*XmlPayItem
	Array []*XmlPayItem
}

func (this *PayTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("PayTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *PayTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "Pay.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("PayTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlPayConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("PayTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlPayItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlPayItem, 0)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlPayItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *PayTableMgr) Get(id int32) *XmlPayItem {
	return this.Map[id]
}
