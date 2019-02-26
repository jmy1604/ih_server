package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlArtifactItem struct {
	ClientIndex       int32  `xml:"ClientIndex,attr"`
	Id                int32  `xml:"ArtifactID,attr"`
	Rank              int32  `xml:"Rank,attr"`
	Level             int32  `xml:"Level,attr"`
	MaxLevel          int32  `xml:"MaxLevel,attr"`
	SkillId           int32  `xml:"SkillID,attr"`
	ArtifactAttrStr   string `xml:"ArtifactAttr,attr"`
	ArtifactAttr      []int32
	LevelUpResCostStr string `xml:"LevelUpResCost,attr"`
	LevelUpResCost    []int32
	RankUpResCostStr  string `xml:"RankUpResCost,attr"`
	RankUpResCost     []int32
	DecomposeResStr   string `xml:"DecomposeRes,attr"`
	DecomposeRes      []int32
}

type XmlArtifactConfig struct {
	Items []XmlArtifactItem `xml:"item"`
}

type ArtifactRankItem struct {
	Level2Item map[int32]*XmlArtifactItem
	MaxLevel   int32
}

type ArtifactIdItem struct {
	Rank2Item map[int32]*ArtifactRankItem
}

type ArtifactTableMgr struct {
	Map   map[int32]*ArtifactIdItem
	Array []*XmlArtifactItem
}

func (this *ArtifactTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("ArtifactTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *ArtifactTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "ArtifactLevel.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("ArtifactTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlArtifactConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("ArtifactTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*ArtifactIdItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlArtifactItem, 0)
	}

	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlArtifactItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]
		tmp_item.ArtifactAttr = parse_xml_str_arr2(tmp_item.ArtifactAttrStr, ",")
		if tmp_item.ArtifactAttr != nil && len(tmp_item.ArtifactAttr)%2 > 0 {
			log.Error("ArtifactTableMgr parse column ArtifactAttr data %v with row %v failed", tmp_item.ArtifactAttrStr, idx+1)
			return false
		}
		tmp_item.LevelUpResCost = parse_xml_str_arr2(tmp_item.LevelUpResCostStr, ",")
		if tmp_item.LevelUpResCost != nil && len(tmp_item.LevelUpResCost)%2 > 0 {
			log.Error("ArtifactTableMgr parse column LevelUpResCost data %v with row %v failed", tmp_item.LevelUpResCostStr, idx+1)
			return false
		}
		tmp_item.RankUpResCost = parse_xml_str_arr2(tmp_item.RankUpResCostStr, ",")
		if tmp_item.RankUpResCost != nil && len(tmp_item.RankUpResCost)%2 > 0 {
			log.Error("ArtifactTableMgr parse column LevelUpResCost data %v with row %v failed", tmp_item.RankUpResCostStr, idx+1)
			return false
		}
		tmp_item.DecomposeRes = parse_xml_str_arr2(tmp_item.DecomposeResStr, ",")
		if tmp_item.DecomposeRes != nil && len(tmp_item.DecomposeRes)%2 > 0 {
			log.Error("ArtifactTableMgr parse column DecomposeRes Data %v with row %v failed", tmp_item.DecomposeResStr, idx+1)
			return false
		}

		id_item := this.Map[tmp_item.Id]
		if id_item == nil {
			id_item = &ArtifactIdItem{}
			id_item.Rank2Item = make(map[int32]*ArtifactRankItem)
			this.Map[tmp_item.Id] = id_item
		}
		rank_item := id_item.Rank2Item[tmp_item.Rank]
		if rank_item == nil {
			rank_item = &ArtifactRankItem{}
			rank_item.Level2Item = make(map[int32]*XmlArtifactItem)
			rank_item.MaxLevel = tmp_item.MaxLevel
			id_item.Rank2Item[tmp_item.Rank] = rank_item
		}
		rank_item.Level2Item[tmp_item.Level] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *ArtifactTableMgr) Get(id, rank, level int32) *XmlArtifactItem {
	id_item := this.Map[id]
	if id_item == nil {
		return nil
	}
	rank_item := id_item.Rank2Item[rank]
	if rank_item == nil {
		return nil
	}
	return rank_item.Level2Item[level]
}

func (this *ArtifactTableMgr) GetByIdAndRank(id, rank int32) *ArtifactRankItem {
	id_item := this.Map[id]
	if id_item == nil {
		return nil
	}
	return id_item.Rank2Item[rank]
}

func (this *ArtifactTableMgr) GetMaxRank(id int32) int32 {
	id_item := this.Map[id]
	if id_item == nil {
		return 0
	}
	return int32(len(id_item.Rank2Item))
}
