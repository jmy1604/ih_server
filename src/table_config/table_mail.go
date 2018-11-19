package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlMailItem struct {
	MailSubtype   int32 `xml:"MailID,attr"`
	MailType      int32 `xml:"MailType,attr"`
	MailTitleID   int32 `xml:"MailTitleID,attr"`
	MailContentID int32 `xml:"MailContentID,attr"`
}

type XmlMailConfig struct {
	Items []XmlMailItem `xml:"item"`
}

type MailTableMgr struct {
	Map   map[int32]*XmlMailItem
	Array []*XmlMailItem
}

func (this *MailTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("MailTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *MailTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "Mail.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("MailTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlMailConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("MailTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlMailItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlMailItem, 0)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlMailItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		this.Map[tmp_item.MailSubtype] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *MailTableMgr) Get(level int32) *XmlMailItem {
	return this.Map[level]
}
