package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlGuildBossItem struct {
	Id                 int32  `xml:"BossIndex,attr"`
	StageId            int32  `xml:"StageID,attr"`
	BattleRewardStr    string `xml:"BattleReward,attr"`
	RankReward1CondStr string `xml:"RankReward1Cond,attr"`
	RankReward1Str     string `xml:"RankReward1,attr"`
	RankReward2CondStr string `xml:"RankReward2Cond,attr"`
	RankReward2Str     string `xml:"RankReward2,attr"`
	RankReward3CondStr string `xml:"RankReward3Cond,attr"`
	RankReward3Str     string `xml:"RankReward3,attr"`
	RankReward4CondStr string `xml:"RankReward4Cond,attr"`
	RankReward4Str     string `xml:"RankReward4,attr"`
	RankReward5CondStr string `xml:"RankReward5Cond,attr"`
	RankReward5Str     string `xml:"RankReward5,attr"`
	BattleReward       []int32
	RankReward1Cond    []int32
	RankReward1        []int32
	RankReward2Cond    []int32
	RankReward2        []int32
	RankReward3Cond    []int32
	RankReward3        []int32
	RankReward4Cond    []int32
	RankReward4        []int32
	RankReward5Cond    []int32
	RankReward5        []int32
}

type XmlGuildBossConfig struct {
	Items []XmlGuildBossItem `xml:"item"`
}

type GuildBossTableMgr struct {
	Map   map[int32]*XmlGuildBossItem
	Array []*XmlGuildBossItem
}

func (this *GuildBossTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("GuildBossTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *GuildBossTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "GuildBoss.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("GuildBossTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlGuildBossConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("GuildBossTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlGuildBossItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlGuildBossItem, 0)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlGuildBossItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		tmp_item.BattleReward = parse_xml_str_arr2(tmp_item.BattleRewardStr, ",")
		if tmp_item.BattleReward == nil {
			log.Error("parse GuildBossTable field[BattleReward] with column %v failed", idx)
			return false
		}

		tmp_item.RankReward1 = parse_xml_str_arr2(tmp_item.RankReward1Str, ",")
		if tmp_item.RankReward1 == nil {
			log.Error("parse GuildBossTable field[RankReward1] with column %v failed", idx)
			return false
		}

		tmp_item.RankReward1Cond = parse_xml_str_arr2(tmp_item.RankReward1CondStr, ",")
		if tmp_item.RankReward1Cond == nil {
			log.Error("parse GuildBossTable field[RankReward1Cond] with column %v failed", idx)
			return false
		}

		tmp_item.RankReward2 = parse_xml_str_arr2(tmp_item.RankReward2Str, ",")
		if tmp_item.RankReward2 == nil {
			log.Error("parse GuildBossTable field[RankReward2] with column %v failed", idx)
			return false
		}

		tmp_item.RankReward2Cond = parse_xml_str_arr2(tmp_item.RankReward2CondStr, ",")
		if tmp_item.RankReward2Cond == nil {
			log.Error("parse GuildBossTable field[RankReward2Cond] with column %v failed", idx)
			return false
		}

		tmp_item.RankReward3 = parse_xml_str_arr2(tmp_item.RankReward3Str, ",")
		if tmp_item.RankReward3 == nil {
			log.Error("parse GuildBossTable field[RankReward3] with column %v failed", idx)
			return false
		}

		tmp_item.RankReward3Cond = parse_xml_str_arr2(tmp_item.RankReward3CondStr, ",")
		if tmp_item.RankReward3Cond == nil {
			log.Error("parse GuildBossTable field[RankReward3Cond] with column %v failed", idx)
			return false
		}

		tmp_item.RankReward4 = parse_xml_str_arr2(tmp_item.RankReward4Str, ",")
		if tmp_item.RankReward4 == nil {
			log.Error("parse GuildBossTable field[RankReward4] with column %v failed", idx)
			return false
		}

		tmp_item.RankReward4Cond = parse_xml_str_arr2(tmp_item.RankReward4CondStr, ",")
		if tmp_item.RankReward4Cond == nil {
			log.Error("parse GuildBossTable field[RankReward4Cond] with column %v failed", idx)
			return false
		}

		tmp_item.RankReward5 = parse_xml_str_arr2(tmp_item.RankReward5Str, ",")
		if tmp_item.RankReward5 == nil {
			log.Error("parse GuildBossTable field[RankReward5] with column %v failed", idx)
			return false
		}

		tmp_item.RankReward5Cond = parse_xml_str_arr2(tmp_item.RankReward5CondStr, ",")
		if tmp_item.RankReward5Cond == nil {
			log.Error("parse GuildBossTable field[RankReward5Cond] with column %v failed", idx)
			return false
		}

		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *GuildBossTableMgr) Get(id int32) *XmlGuildBossItem {
	return this.Map[id]
}

func (this *GuildBossTableMgr) GetNext(id int32) (next_item *XmlGuildBossItem) {
	for i, item := range this.Array {
		if item.Id == id {
			if i+1 >= len(this.Array) {
				return
			}
			next_item = this.Array[i+1]
			break
		}
	}
	return
}
