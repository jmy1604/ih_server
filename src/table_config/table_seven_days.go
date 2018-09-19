package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlSevenDaysItem struct {
	Id        int32  `xml:"TotalIndex,attr"`
	RewardStr string `xml:"Reward,attr"`
	Reward    []int32
}

type XmlSevenDaysConfig struct {
	Items []XmlSevenDaysItem `xml:"item"`
}

type SevenDaysTableMgr struct {
	Map   map[int32]*XmlSevenDaysItem
	Array []*XmlSevenDaysItem
}

func (this *SevenDaysTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("SevenDaysTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *SevenDaysTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "Sevenday.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("SevenDaysTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlSevenDaysConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("SevenDaysTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlSevenDaysItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlSevenDaysItem, 0)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlSevenDaysItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		tmp_item.Reward = parse_xml_str_arr2(tmp_item.RewardStr, ",")
		if tmp_item.Reward == nil {
			log.Error("SevenDaysTableMgr parse column[Reward] value[%v] with idx[%v] falied", tmp_item.RewardStr, idx)
			return false
		}

		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *SevenDaysTableMgr) Get(id int32) *XmlSevenDaysItem {
	return this.Map[id]
}
