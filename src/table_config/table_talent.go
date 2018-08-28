package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlTalentItem struct {
	Id                  int32  `xml:"TalentID,attr"`
	LevelId             int32  `xml:"TalentBaseID,attr"`
	Level               int32  `xml:"Level,attr"`
	CanLearn            int32  `xml:"CanLearn,attr"`
	UpgradeCostStr      string `xml:"UpgradeCost,attr"`
	UpgradeCost         []int32
	PrevSkillCond       int32  `xml:"PreSkillCond,attr"`
	PreSkillLevCond     int32  `xml:"PreSkillLevCond,attr"`
	TeamSpeedBonus      int32  `xml:"TeamSpeedBonus,attr"`
	TalentEffectCondStr string `xml:"TalentEffectCond,attr"`
	TalentEffectCond    []int32
	TalentAttrStr       string `xml:"TalentAttr,attr"`
	TalentAttr          []int32
	TalentSkillListStr  string `xml:"TalentSkillList,attr"`
	TalentSkillList     []int32
	Tag                 int32 `xml:"PageLabel,attr"`
	Prev                *XmlTalentItem
	Next                *XmlTalentItem
}

type XmlTalentConfig struct {
	Items []XmlTalentItem `xml:"item"`
}

type TalentTableMgr struct {
	Map        map[int32]*XmlTalentItem
	Array      []*XmlTalentItem
	IdLevelMap map[int32][]*XmlTalentItem
}

func (this *TalentTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("TalentTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *TalentTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "Talent.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("TalentTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlTalentConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("TalentTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlTalentItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlTalentItem, 0)
	}
	if this.IdLevelMap == nil {
		this.IdLevelMap = make(map[int32][]*XmlTalentItem)
	}

	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item, prev *XmlTalentItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]
		tmp_item.UpgradeCost = parse_xml_str_arr2(tmp_item.UpgradeCostStr, ",")
		if tmp_item.UpgradeCost == nil {
			log.Error("TalentTableMgr parse UpgradeCostStr with [%v] failed", tmp_item.UpgradeCostStr)
			return false
		}

		tmp_item.TalentEffectCond = parse_xml_str_arr2(tmp_item.TalentEffectCondStr, "|")
		if tmp_item.TalentEffectCond == nil {
			log.Error("TalentTableMgr parse TalentEffectCondStr with [%v] failed", tmp_item.TalentEffectCondStr)
			return false
		}

		tmp_item.TalentAttr = parse_xml_str_arr2(tmp_item.TalentAttrStr, ",")
		if tmp_item.TalentAttr == nil {
			log.Error("TalentTableMgr parse TalentAttrStr with [%v] failed", tmp_item.TalentAttrStr)
			return false
		}

		tmp_item.TalentSkillList = parse_xml_str_arr2(tmp_item.TalentSkillListStr, ",")
		if tmp_item.TalentSkillList == nil {
			log.Error("TalentTableMgr parse TalentSkillListStr with [%v] failed", tmp_item.TalentSkillListStr)
			return false
		}

		if prev != nil && prev.LevelId == tmp_item.LevelId && prev.Level+1 == tmp_item.Level {
			prev.Next = tmp_item
			tmp_item.Prev = prev
		}

		prev = tmp_item

		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
		arr := this.IdLevelMap[tmp_item.LevelId]
		if arr == nil {
			arr = []*XmlTalentItem{tmp_item}
		} else {
			arr = append(arr, tmp_item)
		}
		this.IdLevelMap[tmp_item.LevelId] = arr
	}

	return true
}

func (this *TalentTableMgr) Get(talent_id int32) *XmlTalentItem {
	return this.Map[talent_id]
}

func (this *TalentTableMgr) GetNext(talent_id int32) *XmlTalentItem {
	t := this.Map[talent_id]
	if t == nil {
		return nil
	}
	return t.Next
}

func (this *TalentTableMgr) GetByIdLevel(talent_id, level int32) *XmlTalentItem {
	arr := this.IdLevelMap[talent_id]
	if arr == nil {
		return nil
	}
	if level < int32(1) || level > int32(len(arr)) {
		return nil
	}
	return arr[level-1]
}
