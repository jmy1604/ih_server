package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlSubActivityItem struct {
	Id         int32  `xml:"SubActiveID,attr"`
	EventId    int32  `xml:"EventId,attr"`
	BundleID   string `xml:"BundleID,attr"`
	Param1     int32  `xml:"Param1,attr"`
	Param2     int32  `xml:"Param2,attr"`
	Param3     int32  `xml:"Param3,attr"`
	Param4     int32  `xml:"Param4,attr"`
	EventCount int32  `xml:"EventCount,attr"`
	RewardStr  string `xml:"Reward,attr"`
	Reward     []int32
}

type XmlSubActivityConfig struct {
	Items []XmlSubActivityItem `xml:"item"`
}

type SubActivityTableMgr struct {
	Map   map[int32]*XmlSubActivityItem
	Array []*XmlSubActivityItem
}

func (this *SubActivityTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("SubActivityTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *SubActivityTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "SubActive.xml"
	}

	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("SubActivityTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlSubActivityConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("SubActivityTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlSubActivityItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlSubActivityItem, 0)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlSubActivityItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		tmp_item.Reward = parse_xml_str_arr2(tmp_item.RewardStr, ",")
		if tmp_item.Reward == nil {
			log.Error("SubActivityTableMgr parse column[Reward] with value[%v] on line %v", tmp_item.RewardStr, idx)
			return false
		}

		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *SubActivityTableMgr) Get(id int32) *XmlSubActivityItem {
	return this.Map[id]
}
