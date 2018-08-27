package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
	"math/rand"
)

type XmlSearchTaskBossItem struct {
	SearchTaskBossGroup int32 `xml:"SearchTaskBossGroup,attr"`
	Weight              int32 `xml:"Weight,attr"`
	StageId             int32 `xml:"stageid,attr"`
}

type XmlSearchTaskBossConfig struct {
	Items []XmlSearchTaskBossItem `xml:"item"`
}

type SearchTaskBossGroup struct {
	items        []*XmlSearchTaskBossItem
	total_weight int32
}

type SearchTaskBossTableMgr struct {
	Groups map[int32]*SearchTaskBossGroup
	Array  []*XmlSearchTaskBossItem
}

func (this *SearchTaskBossTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("SearchTaskBossTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *SearchTaskBossTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "SearchTaskBoss.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("SearchTaskBossTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlSearchTaskBossConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("SearchTaskBossTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Groups == nil {
		this.Groups = make(map[int32]*SearchTaskBossGroup)
	}
	tmp_len := int32(len(tmp_cfg.Items))
	var tmp_item *XmlSearchTaskBossItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]
		m := this.Groups[tmp_item.SearchTaskBossGroup]
		if m == nil {
			m = &SearchTaskBossGroup{
				items:        []*XmlSearchTaskBossItem{tmp_item},
				total_weight: tmp_item.Weight,
			}
			this.Groups[tmp_item.SearchTaskBossGroup] = m
		} else {
			m.items = append(m.items, tmp_item)
			m.total_weight += tmp_item.Weight
		}
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *SearchTaskBossTableMgr) Random(group_id int32) (item *XmlSearchTaskBossItem) {
	d := this.Groups[group_id]
	if d == nil {
		return
	}

	w := rand.Int31n(d.total_weight)
	for i := 0; i < len(d.items); i++ {
		if w-d.items[i].Weight < 0 {
			item = d.items[i]
			break
		}
		w -= d.items[i].Weight
	}
	return
}
