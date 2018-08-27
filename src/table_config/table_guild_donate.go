package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlGuildDonateItem struct {
	ItemId              int32  `xml:"ItemID,attr"`
	RequestNum          int32  `xml:"RequestNum,attr"`
	DonateRewardItemStr string `xml:"DonateRewardItem,attr"`
	DonateRewardItem    []int32
	LimitScore          int32 `xml:"LimitScore,attr"`
}

type XmlGuildDonateConfig struct {
	Items []XmlGuildDonateItem `xml:"item"`
}

type GuildDonateTableMgr struct {
	Map   map[int32]*XmlGuildDonateItem
	Array []*XmlGuildDonateItem
}

func (this *GuildDonateTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("GuildDonateTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *GuildDonateTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "GuildDonate.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("GuildDonateTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlGuildDonateConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("GuildDonateTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlGuildDonateItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlGuildDonateItem, 0)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlGuildDonateItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		tmp_item.DonateRewardItem = parse_xml_str_arr2(tmp_item.DonateRewardItemStr, ",")
		if tmp_item.DonateRewardItem == nil {
			log.Error("parse GuildDonateTable field[DonateRewardItem] with column %v failed", idx)
			return false
		}

		this.Map[tmp_item.ItemId] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *GuildDonateTableMgr) Get(item_id int32) *XmlGuildDonateItem {
	return this.Map[item_id]
}
