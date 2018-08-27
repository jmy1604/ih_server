package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlStatusItem struct {
	Id               int32  `xml:"BuffID,attr"`
	Type             int32  `xml:"Type,attr"`
	EffectStr        string `xml:"Effect,attr"`
	Effect           []int32
	ResistCountMax   int32  `xml:"ResistCountMax,attr"`
	MutexType        int32  `xml:"MutexType,attr"`
	ResistMutexType  string `xml:"ResistMutexType,attr"`
	CancelMutexType  string `xml:"CancelMutexType,attr"`
	ResistMutexID    string `xml:"ResistMutexID,attr"`
	CancelMutexID    string `xml:"CancelMutexID,attr"`
	ResistMutexTypes []int32
	CancelMutexTypes []int32
	ResistMutexIDs   []int32
	CancelMutexIDs   []int32
}

type XmlStatusConfig struct {
	Items []XmlStatusItem `xml:"item"`
}

type StatusTableMgr struct {
	Map   map[int32]*XmlStatusItem
	Array []*XmlStatusItem
}

func (this *StatusTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("StatusTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *StatusTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "Status.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("StatusTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlStatusConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("StatusTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlStatusItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlStatusItem, 0)
	}

	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlStatusItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		tmp_item.Effect = parse_xml_str_arr2(tmp_item.EffectStr, "|")
		if tmp_item.Effect == nil {
			log.Error("StatusTableMgr parse EffectStr with [%v] failed", tmp_item.EffectStr)
			return false
		}
		tmp_item.ResistMutexTypes = parse_xml_str_arr2(tmp_item.ResistMutexType, ",")
		if tmp_item.ResistMutexTypes == nil {
			log.Error("StatusTableMgr parse ResistMutexType with [%v] failed", tmp_item.ResistMutexType)
			return false
		}
		tmp_item.CancelMutexTypes = parse_xml_str_arr2(tmp_item.CancelMutexType, ",")
		if tmp_item.CancelMutexTypes == nil {
			log.Error("StatusTableMgr parse CancelMutexType with [%v] failed", tmp_item.CancelMutexType)
			return false
		}
		tmp_item.ResistMutexIDs = parse_xml_str_arr2(tmp_item.ResistMutexID, ",")
		if tmp_item.ResistMutexIDs == nil {
			log.Error("StatusTableMgr parse ResistMutexID with [%v] failed", tmp_item.ResistMutexID)
			return false
		}
		tmp_item.CancelMutexIDs = parse_xml_str_arr2(tmp_item.CancelMutexID, ",")
		if tmp_item.CancelMutexIDs == nil {
			log.Error("StatusTableMgr parse CancelMutexID with [%v] failed", tmp_item.CancelMutexID)
			return false
		}

		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *StatusTableMgr) Get(id int32) *XmlStatusItem {
	return this.Map[id]
}
