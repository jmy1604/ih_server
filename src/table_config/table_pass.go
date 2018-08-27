package table_config

import (
	"encoding/json"
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type JsonMonster struct {
	Wave      int32
	Slot      int32
	MonsterID int32
	Rank      int32
	Level     int32
	EquipID   []int32
}

type XmlPassItem struct {
	Id               int32  `xml:"StageID,attr"`
	MaxWaves         int32  `xml:"MaxWaves,attr"`
	MonsterList      string `xml:"MonsterList,attr"`
	MaxRound         int32  `xml:"MaxRound,attr"`
	TimeUpWin        int32  `xml:"TimeUpWin,attr"`
	PlayerCardMax    int32  `xml:"PlayerCardMax,attr"`
	FriendSupportMax int32  `xml:"FriendSupportMax,attr"`
	NpcSupportList   string `xml:"NpcSupportList,attr"`
	Monsters         []*JsonMonster
	RewardListStr    string `xml:"RewardList,attr"`
	RewardList       []int32
}

type XmlPassConfig struct {
	Items []XmlPassItem `xml:"item"`
}

type PassTableMgr struct {
	Map   map[int32]*XmlPassItem
	Array []*XmlPassItem
}

func (this *PassTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("PassTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *PassTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "Stage.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("PassTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlPassConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("PassTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlPassItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlPassItem, 0)
	}

	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlPassItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]
		if err = json.Unmarshal([]byte(tmp_item.MonsterList), &tmp_item.Monsters); err != nil {
			log.Error("Parse MonsterList[%v] error[%v]", tmp_item.MonsterList, err.Error())
			return false
		}
		tmp_item.RewardList = parse_xml_str_arr2(tmp_item.RewardListStr, ",")
		if tmp_item.RewardList == nil {
			log.Error("PassTableMgr parse RewardListStr with [%v] failed", tmp_item.RewardListStr)
			return false
		}
		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *PassTableMgr) Get(id int32) *XmlPassItem {
	return this.Map[id]
}
