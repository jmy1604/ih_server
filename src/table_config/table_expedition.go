package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlExpeditionItem struct {
	Level            int32 `xml:"ID,attr"`
	StageType        int32 `xml:"StageType,attr"`
	PlayerCardMax    int32 `xml:"PlayerCardMax,attr"`
	EnemyBattlePower int32 `xml:"EnemyBattlePower,attr"`
	GoldBase         int32 `xml:"GoldBase,attr"`
	GoldRate         int32 `xml:"GoldRate,attr"`
	TokenBase        int32 `xml:"TokenBase,attr"`
	TokenRate        int32 `xml:"TokenRate,attr"`
	PurifyPoint      int32 `xml:"PurifyPoint,attr"`
}

type XmlExpeditionConfig struct {
	Items []XmlExpeditionItem `xml:"item"`
}

type ExpeditionTableMgr struct {
	Map   map[int32]*XmlExpeditionItem
	Array []*XmlExpeditionItem
}

func (this *ExpeditionTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("ExpeditionTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *ExpeditionTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "Expedition.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("ExpeditionTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlExpeditionConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("ExpeditionTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlExpeditionItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlExpeditionItem, 0)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlExpeditionItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]
		this.Map[tmp_item.Level] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *ExpeditionTableMgr) Get(level int32) *XmlExpeditionItem {
	return this.Map[level]
}
