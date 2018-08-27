package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlGuildLevelUpItem struct {
	Level     int32 `xml:"Level,attr"`
	Exp       int32 `xml:"Exp,attr"`
	MemberNum int32 `xml:"Members,attr"`
}

type XmlGuildLevelUpConfig struct {
	Items []XmlGuildLevelUpItem `xml:"item"`
}

type GuildLevelUpTableMgr struct {
	Map          map[int32]*XmlGuildLevelUpItem
	Array        []*XmlGuildLevelUpItem
	MaxLevel     int32
	MaxMemberNum int32
}

func (this *GuildLevelUpTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("GuildLevelUpTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *GuildLevelUpTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "GuildLevelUp.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("GuildLevelUpTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlGuildLevelUpConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("GuildLevelUpTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlGuildLevelUpItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlGuildLevelUpItem, 0)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlGuildLevelUpItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		this.Map[tmp_item.Level] = tmp_item
		this.Array = append(this.Array, tmp_item)

		if this.MaxLevel == 0 || this.MaxLevel < tmp_item.Level {
			this.MaxLevel = tmp_item.Level
		}
		if this.MaxMemberNum == 0 || this.MaxMemberNum < tmp_item.MemberNum {
			this.MaxMemberNum = tmp_item.MemberNum
		}
	}

	return true
}

func (this *GuildLevelUpTableMgr) Get(level int32) *XmlGuildLevelUpItem {
	return this.Map[level]
}

func (this *GuildLevelUpTableMgr) GetMemberNumLimit(level int32) int32 {
	m := this.Get(level)
	if m == nil {
		return 0
	}
	return m.MemberNum
}

func (this *GuildLevelUpTableMgr) GetMaxLevel() int32 {
	return this.MaxLevel
}

func (this *GuildLevelUpTableMgr) GetMaxMemberNum() int32 {
	return this.MaxMemberNum
}
