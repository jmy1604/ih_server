package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlActiveStageItem struct {
	Id                    int32 `xml:"ID,attr"`
	Type                  int32 `xml:"Type,attr"`
	StageId               int32 `xml:"StageID,attr"`
	PlayerLevel           int32 `xml:"PlayerLevelCond,attr"`
	PlayerLevelSuggestion int32 `xml:"PlayerLevelSuggestion,attr"`
}

type XmlActiveStageConfig struct {
	Items []XmlActiveStageItem `xml:"item"`
}

type ActiveStageTableMgr struct {
	Map   map[int32]*XmlActiveStageItem
	Array []*XmlActiveStageItem
}

func (this *ActiveStageTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("ActiveStageTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *ActiveStageTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "ActiveStage.xml"
	}
	config_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(config_path)
	if nil != err {
		log.Error("ActiveStageTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlActiveStageConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("ActiveStageTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlActiveStageItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlActiveStageItem, 0)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlActiveStageItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *ActiveStageTableMgr) Get(active_stage_id int32) *XmlActiveStageItem {
	return this.Map[active_stage_id]
}
