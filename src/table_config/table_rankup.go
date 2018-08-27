package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlRankUpItem struct {
	Rank                 int32  `xml:"Rank,attr"`
	Type1RankUpResStr    string `xml:"Type1RankUpRes,attr"`
	Type1RankUpRes       []int32
	Type2RankUpResStr    string `xml:"Type2RankUpRes,attr"`
	Type2RankUpRes       []int32
	Type3RankUpResStr    string `xml:"Type3RankUpRes,attr"`
	Type3RankUpRes       []int32
	Type1DecomposeResStr string `xml:"Type1DecomposeRes,attr"`
	Type1DecomposeRes    []int32
	Type2DecomposeResStr string `xml:"Type2DecomposeRes,attr"`
	Type2DecomposeRes    []int32
	Type3DecomposeResStr string `xml:"Type3DecomposeRes,attr"`
	Type3DecomposeRes    []int32
}

type XmlRankUpConfig struct {
	Items []XmlRankUpItem `xml:"item"`
}

type RankUpTableMgr struct {
	Map   map[int32]*XmlRankUpItem
	Array []*XmlRankUpItem
}

func (this *RankUpTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("RankUpTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *RankUpTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "RankUp.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("RankUpTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlRankUpConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("RankUpTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlRankUpItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlRankUpItem, 0)
	}

	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlRankUpItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		if tmp_item.Type1RankUpResStr != "" {
			a := parse_xml_str_arr2(tmp_item.Type1RankUpResStr, ",")
			if a == nil {
				log.Error("RankUpTableMgr parse Type1RankUpResStr with [%v] failed", tmp_item.Type1RankUpResStr)
				return false
			} else {
				tmp_item.Type1RankUpRes = a
			}
		}
		if tmp_item.Type2RankUpResStr != "" {
			a := parse_xml_str_arr2(tmp_item.Type2RankUpResStr, ",")
			if a == nil {
				log.Error("RankUpTableMgr parse Type2RankUpResStr with [%v] failed", tmp_item.Type2RankUpResStr)
				return false
			} else {
				tmp_item.Type2RankUpRes = a
			}
		}
		if tmp_item.Type3RankUpResStr != "" {
			a := parse_xml_str_arr2(tmp_item.Type3RankUpResStr, ",")
			if a == nil {
				log.Error("RankUpTableMgr parse Type3RankUpResStr with [%v] failed", tmp_item.Type3RankUpResStr)
				return false
			} else {
				tmp_item.Type3RankUpRes = a
			}
		}

		if tmp_item.Type1DecomposeResStr != "" {
			a := parse_xml_str_arr2(tmp_item.Type1DecomposeResStr, ",")
			if a == nil {
				log.Error("RankUpTableMgr parse Type1DecomposeResStr with [%v] failed", tmp_item.Type1DecomposeResStr)
				return false
			} else {
				tmp_item.Type1DecomposeRes = a
			}
		}
		if tmp_item.Type2DecomposeResStr != "" {
			a := parse_xml_str_arr2(tmp_item.Type2DecomposeResStr, ",")
			if a == nil {
				log.Error("RankUpTableMgr parse Type2DecomposeResStr with [%v] failed", tmp_item.Type2DecomposeResStr)
				return false
			} else {
				tmp_item.Type2DecomposeRes = a
			}
		}
		if tmp_item.Type3DecomposeResStr != "" {
			a := parse_xml_str_arr2(tmp_item.Type3DecomposeResStr, ",")
			if a == nil {
				log.Error("RankUpTableMgr parse Type3DecomposeResStr with [%v] failed", tmp_item.Type3DecomposeResStr)
				return false
			} else {
				tmp_item.Type3DecomposeRes = a
			}
		}

		this.Map[tmp_item.Rank] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *RankUpTableMgr) Get(rank int32) *XmlRankUpItem {
	return this.Map[rank]
}
