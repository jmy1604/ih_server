package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlArenaDivisionItem struct {
	Id                      int32  `xml:"Division,attr"`
	DivisionScoreMin        int32  `xml:"DivisionScoreMin,attr"`
	DivisionScoreMax        int32  `xml:"DivisionScoreMax,attr"`
	WinScore                int32  `xml:"WinScore,attr"`
	WinningStreakScoreBonus int32  `xml:"WinningStreakScoreBonus,attr"`
	LoseScore               int32  `xml:"LoseScore,attr"`
	NewSeasonScore          int32  `xml:"NewSeasonScore,attr"`
	RewardListStr           string `xml:"RewardList,attr"`
	RewardList              []int32
}

type XmlArenaDivisionConfig struct {
	Items []XmlArenaDivisionItem `xml:"item"`
}

type ArenaDivisionTableMgr struct {
	Map   map[int32]*XmlArenaDivisionItem
	Array []*XmlArenaDivisionItem
}

func (this *ArenaDivisionTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("ArenaDivisionTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *ArenaDivisionTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "ArenaDivision.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("ArenaDivisionTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlArenaDivisionConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("ArenaDivisionTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlArenaDivisionItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlArenaDivisionItem, 0)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlArenaDivisionItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]
		tmp_item.RewardList = parse_xml_str_arr2(tmp_item.RewardListStr, ",")
		if tmp_item.RewardList == nil || len(tmp_item.RewardList)%2 != 0 {
			log.Error("ArenaDivisionTableMgr parse index[%v] column error on field[RewardList] with value[%v]", idx, tmp_item.RewardListStr)
			return false
		}
		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *ArenaDivisionTableMgr) GetByGrade(id int32) *XmlArenaDivisionItem {
	return this.Map[id]
}

func (this *ArenaDivisionTableMgr) GetByScore(score int32) *XmlArenaDivisionItem {
	for i := 0; i < len(this.Array); i++ {
		d := this.Array[i]
		if d.DivisionScoreMin <= score && d.DivisionScoreMax >= score {
			return d
		}
	}
	return nil
}

func (this *ArenaDivisionTableMgr) GetGradeByScore(score int32) int32 {
	d := this.GetByScore(score)
	if d == nil {
		return 0
	}
	return d.Id
}
