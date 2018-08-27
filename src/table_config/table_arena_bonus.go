package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlArenaBonusItem struct {
	Id                  int32  `xml:"Index,attr"`
	RankingMin          int32  `xml:"RankingMin,attr"`
	RankingMax          int32  `xml:"RankingMax,attr"`
	DayRewardListStr    string `xml:"DayRewardList,attr"`
	SeasonRewardListStr string `xml:"SeasonRewardList,attr"`
	DayRewardList       []int32
	SeasonRewardList    []int32
}

type XmlArenaBonusConfig struct {
	Items []XmlArenaBonusItem `xml:"item"`
}

type ArenaBonusTableMgr struct {
	Map   map[int32]*XmlArenaBonusItem
	Array []*XmlArenaBonusItem
}

func (this *ArenaBonusTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("ArenaBonusTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *ArenaBonusTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "ArenaRankingBonus.xml"
	}
	config_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(config_path)
	if nil != err {
		log.Error("ArenaBonusTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlArenaBonusConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("ArenaBonusTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlArenaBonusItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlArenaBonusItem, 0)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlArenaBonusItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]
		tmp_item.DayRewardList = parse_xml_str_arr2(tmp_item.DayRewardListStr, ",")
		if tmp_item.DayRewardList == nil || len(tmp_item.DayRewardList)%2 != 0 {
			log.Error("ArenaDivisionTableMgr parse index[%v] column error on field[DayRewardList] with value[%v]", idx, tmp_item.DayRewardListStr)
			return false
		}
		tmp_item.SeasonRewardList = parse_xml_str_arr2(tmp_item.SeasonRewardListStr, ",")
		if tmp_item.SeasonRewardList == nil || len(tmp_item.SeasonRewardList)%2 != 0 {
			log.Error("ArenaDivisionTableMgr parse index[%v] column error on field[SeasonRewardList] with value[%v]", idx, tmp_item.SeasonRewardListStr)
			return false
		}
		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *ArenaBonusTableMgr) GetByGrade(id int32) *XmlArenaBonusItem {
	return this.Map[id]
}

func (this *ArenaBonusTableMgr) GetByRank(rank int32) *XmlArenaBonusItem {
	for i := 0; i < len(this.Array); i++ {
		d := this.Array[i]
		if d.RankingMin <= rank && d.RankingMax >= rank {
			return d
		}
	}
	return nil
}
