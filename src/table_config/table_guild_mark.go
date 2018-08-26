package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"io/ioutil"
)

type XmlGuildMarkItem struct {
	Id int32 `xml:"ID,attr"`
}

type XmlGuildMarkConfig struct {
	Items []XmlGuildMarkItem `xml:"item"`
}

type GuildMarkTableMgr struct {
	Map   map[int32]*XmlGuildMarkItem
	Array []*XmlGuildMarkItem
}

func (this *GuildMarkTableMgr) Init() bool {
	if !this.Load() {
		log.Error("GuildMarkTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *GuildMarkTableMgr) Load() bool {
	data, err := ioutil.ReadFile("../src/ih_server/game_data/GuildMark.xml")
	if nil != err {
		log.Error("GuildMarkTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlGuildMarkConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("GuildMarkTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlGuildMarkItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlGuildMarkItem, 0)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlGuildMarkItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *GuildMarkTableMgr) Get(id int32) *XmlGuildMarkItem {
	return this.Map[id]
}
