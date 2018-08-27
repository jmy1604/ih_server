package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlHandbookItem struct {
	Id int32 `xml:"Id,attr"`
	//Type int32 `xml:"Tage,attr"`
}

type XmlHandbookConfig struct {
	Items []XmlHandbookItem `xml:"item"`
}

type HandbookTableMgr struct {
	Map   map[int32]*XmlHandbookItem
	Array []*XmlHandbookItem
}

func (this *HandbookTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("HandbookTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *HandbookTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "Fieldguide.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("HandbookTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlHandbookConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("HandbookTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlHandbookItem)
	}

	if this.Array == nil {
		this.Array = make([]*XmlHandbookItem, 0)
	}

	tmp_len := int32(len(tmp_cfg.Items))
	var tmp_item *XmlHandbookItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *HandbookTableMgr) Has(id int32) bool {
	if d := this.Map[id]; d == nil {
		return false
	}
	return true
}

func (this *HandbookTableMgr) Get(id int32) *XmlHandbookItem {
	return this.Map[id]
}
