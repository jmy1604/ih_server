package table_config

import (
	"encoding/xml"
	"ih_server/libs/ipmop"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

const (
	POSITION_GLOBAL = 0 // 全球位置

	POSITION_INNET_NAME = "局域网"
)

type XmlPositionItem struct {
	Pos  int32  `xml:"Position,attr"`
	Name string `xml:"Name,attr"`
}

type XmlPositionConfig struct {
	Items []XmlPositionItem `xml:"item"`
}

type PositionTable struct {
	Pos2Item  map[int32]*XmlPositionItem
	Name2Item map[string]*XmlPositionItem
}

func (this *PositionTable) Init(table_file string) bool {
	if table_file == "" {
		table_file = "position.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("PositionTable Init failed to read file(%s)", err.Error())
		return false
	}

	tmp_cfg := &XmlPositionConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("PositionTable init unmarshal failed [%s]", err.Error())
		return false
	}

	this.Pos2Item = make(map[int32]*XmlPositionItem)
	this.Name2Item = make(map[string]*XmlPositionItem)

	tmp_len := int32(len(tmp_cfg.Items))
	var tmp_item *XmlPositionItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]
		this.Pos2Item[tmp_item.Pos] = tmp_item
		this.Name2Item[tmp_item.Name] = tmp_item
	}
	this.Name2Item[POSITION_INNET_NAME] = &XmlPositionItem{Pos: 0, Name: POSITION_INNET_NAME}

	if !this.InitIpMop() {
		return false
	}

	return true
}

func (this *PositionTable) InitIpMop() bool {
	err := ip17mon.Init(server_config.GetGameDataPathFile("17monipdb.dat"))
	if nil != err {
		log.Error("PositionTable InitIpMop failed [%s]", err.Error())
		return false
	}

	return true
}

func (this *PositionTable) GetPosByIP(s string) int32 {
	posinfo, err := ip17mon.Find(s)
	if nil != err {
		log.Error("PositionTable GetPosByIP failed [%s]", err.Error())
		return POSITION_GLOBAL
	}

	log.Info("PositionTable  GetPosByIP %s:%s", posinfo.Country, posinfo.City)
	cur_pos := this.Name2Item[posinfo.Country]
	if nil == cur_pos {
		return POSITION_GLOBAL
	}

	return cur_pos.Pos
}
