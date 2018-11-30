package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

const (
	CARD_ROLE_TYPE_ATTACK  = 1
	CARD_ROLE_TYPE_DEFENSE = 2
	CARD_ROLE_TYPE_SKILL   = 3
)

type XmlCardItem struct {
	Id                   int32  `xml:"ID,attr"`
	Rank                 int32  `xml:"Rank,attr"`
	ClientId             int32  `xml:"ClientID,attr"`
	MaxLevel             int32  `xml:"MaxLevel,attr"`
	MaxRank              int32  `xml:"MaxRank,attr"`
	Rarity               int32  `xml:"Rarity,attr"`
	Type                 int32  `xml:"Type,attr"`
	Camp                 int32  `xml:"Camp,attr"`
	LabelStr             string `xml:"Label,attr"`
	Label                []int32
	BaseHP               int32  `xml:"BaseHP,attr"`
	BaseAttack           int32  `xml:"BaseAttack,attr"`
	BaseDefence          int32  `xml:"BaseDefence,attr"`
	GrowthHP             int32  `xml:"GrowthHP,attr"`
	GrowthAttack         int32  `xml:"GrowthAttack,attr"`
	GrowthDefence        int32  `xml:"GrowthDefence,attr"`
	NormalSkillID        int32  `xml:"NormalSkillID,attr"`
	SuperSkillID         int32  `xml:"SuperSkillID,attr"`
	PassiveSkillIDStr    string `xml:"PassiveSkillID,attr"`
	PassiveSkillIds      []int32
	DecomposeResStr      string `xml:"DecomposeRes,attr"`
	DecomposeRes         []int32
	BattlePower          int32  `xml:"BattlePower,attr"`
	BattlePowerGrowth    int32  `xml:"BattlePowerGrowth,attr"`
	HeadItem             int32  `xml:"HeadItem,attr"`
	BagFullChangeItemStr string `xml:"BagFullChangeItem,attr"`
	BagFullChangeItem    []int32
	ConvertId1           int32  `xml:"ConvertID1,attr"`
	ConvertId2           int32  `xml:"ConvertID2,attr"`
	ConvertItemStr       string `xml:"ConvertItem,attr"`
	ConvertItem          []int32
}

type XmlCardConfig struct {
	Items []XmlCardItem `xml:"item"`
}

type CardTableMgr struct {
	Map          map[int32]*XmlCardItem
	Array        []*XmlCardItem
	Id2RankArray map[int32][]*XmlCardItem
}

func (this *CardTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("CardTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *CardTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "Card.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("CardTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlCardConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("CardTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlCardItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlCardItem, 0)
	}
	if this.Id2RankArray == nil {
		this.Id2RankArray = make(map[int32][]*XmlCardItem)
	}

	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlCardItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		pids := parse_xml_str_arr2(tmp_item.PassiveSkillIDStr, ",")
		if pids == nil {
			log.Error("CardTableMgr parse PassiveSkillIDStr with [%v] failed", tmp_item.PassiveSkillIDStr)
			return false
		} else {
			tmp_item.PassiveSkillIds = pids
		}
		tmp_item.DecomposeRes = parse_xml_str_arr2(tmp_item.DecomposeResStr, ",")
		if tmp_item.DecomposeRes == nil {
			log.Error("CardTableMgr parse DecomposeResStr with [%v] failed", tmp_item.DecomposeResStr)
			return false
		}
		tmp_item.Label = parse_xml_str_arr2(tmp_item.LabelStr, ",")
		if tmp_item.Label == nil {
			log.Error("CardTableMgr parse LabelStr with [%v] failed", tmp_item.LabelStr)
			return false
		}
		tmp_item.BagFullChangeItem = parse_xml_str_arr2(tmp_item.BagFullChangeItemStr, ",")
		if tmp_item.BagFullChangeItem == nil {
			log.Error("CardTableMgr parse BagFullChangeItem with [%v] failed", tmp_item.BagFullChangeItemStr)
			return false
		}
		tmp_item.ConvertItem = parse_xml_str_arr2(tmp_item.ConvertItemStr, ",")
		if tmp_item.ConvertItem == nil {
			log.Error("CardTableMgr parse ConvertItem with [%v] failed", tmp_item.ConvertItemStr)
			return false
		}

		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)

		a := this.Id2RankArray[tmp_item.Id]
		if a == nil {
			a = make([]*XmlCardItem, 0)
		}
		a = append(a, tmp_item)
		this.Id2RankArray[tmp_item.Id] = a
	}

	return true
}

func (this *CardTableMgr) GetByClientID(client_id int32) *XmlCardItem {
	return this.Map[client_id]
}

func (this *CardTableMgr) GetCards(id int32) []*XmlCardItem {
	return this.Id2RankArray[id]
}

func (this *CardTableMgr) GetRankCard(id int32, rank int32) *XmlCardItem {
	cards := this.GetCards(id)
	if cards == nil {
		return nil
	}
	if rank < 1 || int(rank) > len(cards) {
		return nil
	}
	return cards[rank-1]
}
