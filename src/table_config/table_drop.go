package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlDropItem struct {
	GroupId    int32 `xml:"DropGroupID,attr"`
	DropItemID int32 `xml:"DropItemID,attr"`
	Weight     int32 `xml:"Weight,attr"`
	Min        int32 `xml:"Min,attr"`
	Max        int32 `xml:"Max,attr"`
}

type XmlDropConfig struct {
	Items []*XmlDropItem `xml:"item"`
}

type DropTypeLib struct {
	DropLibType int32
	TotalCount  int32
	TotalWeight int32
	DropItems   []*XmlDropItem
}

type DropManager struct {
	Map map[int32]*DropTypeLib
}

func (this *DropManager) Init(table_file string) bool {
	if !this.Load(table_file) {
		return false
	}
	return true
}

func (this *DropManager) Load(table_file string) bool {
	if table_file == "" {
		table_file = "Drop.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("DropManager load read file failed[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlDropConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("DropManager load xml unmarshal failed [%s]!", err.Error())
		return false
	}

	this.Map = make(map[int32]*DropTypeLib)

	tmp_len := len(tmp_cfg.Items)
	var tmp_item *XmlDropItem
	var tmp_lib *DropTypeLib
	for i := 0; i < tmp_len; i++ {
		tmp_item = tmp_cfg.Items[i]
		if nil == tmp_item {
			continue
		}

		tmp_lib = this.Map[tmp_item.GroupId]
		if nil == tmp_lib {
			tmp_lib := &DropTypeLib{}
			tmp_lib.DropLibType = tmp_item.GroupId //tmp_item.DropType
			tmp_lib.TotalCount = 1
			tmp_lib.TotalWeight = tmp_item.Weight
			this.Map[tmp_item.GroupId] = tmp_lib
		} else {
			tmp_lib.TotalCount++
			tmp_lib.TotalWeight += tmp_item.Weight
		}
	}

	for i := 0; i < tmp_len; i++ {
		tmp_item = tmp_cfg.Items[i]
		if nil == tmp_item {
			continue
		}

		tmp_lib := this.Map[tmp_item.GroupId]
		if nil == tmp_lib {
			continue
		}

		if nil == tmp_lib.DropItems {
			//log.Info("类型%d的随机总权重%d", tmp_lib.DropLibType, tmp_lib.TotalWeight)
			tmp_lib.DropItems = make([]*XmlDropItem, 0, tmp_lib.TotalCount)
		}

		tmp_lib.DropItems = append(tmp_lib.DropItems, tmp_item)

	}

	return true
}
