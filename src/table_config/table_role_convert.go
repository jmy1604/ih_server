package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlHeroConvertItem struct {
	ConvertGroupId int32 `xml:"ConvertGroupID,attr"`
	HeroId         int32 `xml:"HeroID,attr"`
	Weight         int32 `xml:"Weight,attr"`
}

type XmlHeroConvertConfig struct {
	Items []XmlHeroConvertItem `xml:"item"`
}

type XmlHeroConvertGroup struct {
	HeroItems   []*XmlHeroConvertItem
	TotalWeight int32
}

type HeroConvertTableMgr struct {
	//Map    map[int32]*XmlHeroConvertItem
	//Array  []*XmlHeroConvertItem
	Groups map[int32]*XmlHeroConvertGroup
}

func (this *HeroConvertTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("HeroConvertTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *HeroConvertTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "HeroConvert.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("HeroConvertTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlHeroConvertConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("HeroConvertTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Groups == nil {
		this.Groups = make(map[int32]*XmlHeroConvertGroup)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlHeroConvertItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]
		group := this.Groups[tmp_item.ConvertGroupId]
		if group == nil {
			group = &XmlHeroConvertGroup{
				HeroItems:   []*XmlHeroConvertItem{tmp_item},
				TotalWeight: tmp_item.Weight,
			}
			this.Groups[tmp_item.ConvertGroupId] = group
		} else {
			group.HeroItems = append(group.HeroItems, tmp_item)
			group.TotalWeight += tmp_item.Weight
		}
	}

	return true
}

func (this *HeroConvertTableMgr) GetGroup(group_id int32) *XmlHeroConvertGroup {
	return this.Groups[group_id]
}
