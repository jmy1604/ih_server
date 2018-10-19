package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlSystemUnlockItem struct {
	ServerId string `xml:"ServerID,attr"`
	Level    int32  `xml:"Level,attr"`
}

type XmlSystemUnlockConfig struct {
	Items []XmlSystemUnlockItem `xml:"item"`
}

type SystemUnlockTableMgr struct {
	Map   map[string]*XmlSystemUnlockItem
	Array []*XmlSystemUnlockItem
}

func (this *SystemUnlockTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("SystemUnlockTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *SystemUnlockTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "SystemUnlock.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("SystemUnlockTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlSystemUnlockConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("SystemUnlockTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[string]*XmlSystemUnlockItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlSystemUnlockItem, 0)
	}

	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlSystemUnlockItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		this.Map[tmp_item.ServerId] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *SystemUnlockTableMgr) Get(id string) *XmlSystemUnlockItem {
	return this.Map[id]
}

func (this *SystemUnlockTableMgr) GetUnlockLevel(id string) int32 {
	item := this.Map[id]
	if item != nil {
		return item.Level
	}
	return 0
}
