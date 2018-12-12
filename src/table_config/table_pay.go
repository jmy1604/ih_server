package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

const (
	PAY_TYPE_NORMAL     = 0
	PAY_TYPE_MONTH_CARD = 1
)

type XmlPayItem struct {
	Id              int32  `xml:"ID,attr"`
	ActivePay       int32  `xml:"ActivePay,attr"`
	BundleId        string `xml:"BundleID,attr"`
	GemRewardFirst  int32  `xml:"GemRewardFirst,attr"`
	GemReward       int32  `xml:"GemReward,attr"`
	ItemRewardStr   string `xml:"ItemReward,attr"`
	ItemReward      []int32
	MonthCardDay    int32   `xml:"MonthCardDay,attr"`
	MonthCardReward int32   `xml:"MonthCardReward,attr"`
	RecordGold      float64 `xml:"RecordGold,attr"`
	PayType         int32
}

type XmlPayConfig struct {
	Items []XmlPayItem `xml:"item"`
}

type PayTableMgr struct {
	Map            map[int32]*XmlPayItem
	Array          []*XmlPayItem
	Map2           map[string]*XmlPayItem
	MonthCardArray []*XmlPayItem
}

func (this *PayTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("PayTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *PayTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "Pay.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("PayTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlPayConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("PayTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlPayItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlPayItem, 0)
	}
	if this.Map2 == nil {
		this.Map2 = make(map[string]*XmlPayItem)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlPayItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		if tmp_item.MonthCardDay == 30 {
			tmp_item.PayType = PAY_TYPE_MONTH_CARD
		}

		if tmp_item.ItemRewardStr != "" {
			tmp_item.ItemReward = parse_xml_str_arr2(tmp_item.ItemRewardStr, ",")
			if tmp_item.ItemReward == nil || len(tmp_item.ItemReward) == 0 {
				log.Error("Pay Table idx[%v] column[ItemReward] %v parse invalid", idx, tmp_item.ItemReward)
				return false
			}
		}

		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
		this.Map2[tmp_item.BundleId] = tmp_item
		if tmp_item.PayType == PAY_TYPE_MONTH_CARD {
			this.MonthCardArray = append(this.MonthCardArray, tmp_item)
		}
	}

	return true
}

func (this *PayTableMgr) Get(id int32) *XmlPayItem {
	return this.Map[id]
}

func (this *PayTableMgr) GetByBundle(bundle_id string) *XmlPayItem {
	return this.Map2[bundle_id]
}

func (this *PayTableMgr) GetMonthCards() []*XmlPayItem {
	return this.MonthCardArray
}
