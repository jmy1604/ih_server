package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlSignItem struct {
	Id         int32  `xml:"TotalIndex,attr"`
	Group      int32  `xml:"Group,attr"`
	GroupIndex int32  `xml:"GroupIndex,attr"`
	RewardStr  string `xml:"Reward,attr"`
	Reward     []int32
}

type XmlSignConfig struct {
	Items []XmlSignItem `xml:"item"`
}

type SignTableMgr struct {
	Map        map[int32]*XmlSignItem
	Array      []*XmlSignItem
	GroupItems map[int32][]*XmlSignItem
}

func (this *SignTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("SignTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *SignTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "Sign.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("SignTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlSignConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("SignTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlSignItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlSignItem, 0)
	}
	if this.GroupItems == nil {
		this.GroupItems = make(map[int32][]*XmlSignItem)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlSignItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		tmp_item.Reward = parse_xml_str_arr2(tmp_item.RewardStr, ",")
		if tmp_item.Reward == nil {
			log.Error("SignTableMgr parse column[Reward] value[%v] with idx[%v] falied", tmp_item.RewardStr, idx)
			return false
		}

		group_items := this.GroupItems[tmp_item.Group]
		if group_items == nil {
			group_items = []*XmlSignItem{tmp_item}
		} else {
			group_items = append(group_items, tmp_item)
		}
		this.GroupItems[tmp_item.Group] = group_items

		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *SignTableMgr) Get(id int32) *XmlSignItem {
	return this.Map[id]
}

func (this *SignTableMgr) GetGroup(group int32) []*XmlSignItem {
	return this.GroupItems[group]
}
