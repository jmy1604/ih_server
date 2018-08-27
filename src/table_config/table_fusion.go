package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type FusionCostCond struct {
	CostId   int32
	CostCamp int32
	CostType int32
	CostStar int32
	CostNum  int32
}

type XmlFusionItem struct {
	Id                int32  `xml:"FormulaID,attr"`
	FusionType        int32  `xml:"FusionType,attr"`
	ResultDropID      int32  `xml:"ResultDropID,attr"`
	MainCardID        int32  `xml:"MainCardID,attr"`
	MainCardLevelCond int32  `xml:"MainCardLevelCond,attr"`
	ResConditionStr   string `xml:"ResCondtion,attr"`
	ResCondition      []int32
	Cost1IDCond       int32 `xml:"Cost1IDCond,attr"`
	Cost1CampCond     int32 `xml:"Cost1CampCond,attr"`
	Cost1TypeCond     int32 `xml:"Cost1TypeCond,attr"`
	Cost1StarCond     int32 `xml:"Cost1StarCond,attr"`
	Cost1NumCond      int32 `xml:"Cost1NumCond,attr"`
	Cost2IDCond       int32 `xml:"Cost2IDCond,attr"`
	Cost2CampCond     int32 `xml:"Cost2CampCond,attr"`
	Cost2TypeCond     int32 `xml:"Cost2TypeCond,attr"`
	Cost2StarCond     int32 `xml:"Cost2StarCond,attr"`
	Cost2NumCond      int32 `xml:"Cost2NumCond,attr"`
	Cost3IDCond       int32 `xml:"Cost3IDCond,attr"`
	Cost3CampCond     int32 `xml:"Cost3CampCond,attr"`
	Cost3TypeCond     int32 `xml:"Cost3TypeCond,attr"`
	Cost3StarCond     int32 `xml:"Cost3StarCond,attr"`
	Cost3NumCond      int32 `xml:"Cost3NumCond,attr"`
	CostConds         []*FusionCostCond
}

type XmlFusionConfig struct {
	Items []XmlFusionItem `xml:"item"`
}

type FusionTableMgr struct {
	Map   map[int32]*XmlFusionItem
	Array []*XmlFusionItem
}

func (this *FusionTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("FusionTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *FusionTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "Fusion.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("FusionTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlFusionConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("FusionTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlFusionItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlFusionItem, 0)
	}

	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlFusionItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		tmp_item.ResCondition = parse_xml_str_arr2(tmp_item.ResConditionStr, ",")
		if tmp_item.ResCondition == nil || len(tmp_item.ResCondition)%2 != 0 {
			log.Error("FusionTableMgr parse ResCondition with %v failed", tmp_item.ResConditionStr)
			return false
		}

		tmp_item.CostConds = make([]*FusionCostCond, 3)
		tmp_item.CostConds[0] = &FusionCostCond{}
		tmp_item.CostConds[0].CostId = tmp_item.Cost1IDCond
		tmp_item.CostConds[0].CostCamp = tmp_item.Cost1CampCond
		tmp_item.CostConds[0].CostType = tmp_item.Cost1TypeCond
		tmp_item.CostConds[0].CostStar = tmp_item.Cost1StarCond
		tmp_item.CostConds[0].CostNum = tmp_item.Cost1NumCond
		tmp_item.CostConds[1] = &FusionCostCond{}
		tmp_item.CostConds[1].CostId = tmp_item.Cost2IDCond
		tmp_item.CostConds[1].CostCamp = tmp_item.Cost2CampCond
		tmp_item.CostConds[1].CostType = tmp_item.Cost2TypeCond
		tmp_item.CostConds[1].CostStar = tmp_item.Cost2StarCond
		tmp_item.CostConds[1].CostNum = tmp_item.Cost2NumCond
		tmp_item.CostConds[2] = &FusionCostCond{}
		tmp_item.CostConds[2].CostId = tmp_item.Cost3IDCond
		tmp_item.CostConds[2].CostCamp = tmp_item.Cost3CampCond
		tmp_item.CostConds[2].CostType = tmp_item.Cost3TypeCond
		tmp_item.CostConds[2].CostStar = tmp_item.Cost3StarCond
		tmp_item.CostConds[2].CostNum = tmp_item.Cost3NumCond

		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *FusionTableMgr) Get(id int32) *XmlFusionItem {
	return this.Map[id]
}
