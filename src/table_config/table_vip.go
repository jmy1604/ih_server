package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlVipItem struct {
	Id                    int32  `xml:"VipLevel,attr"`
	Exp                   int32  `xml:"Money,attr"`
	AccelTimes            int32  `xml:"AccelTimes,attr"`
	ActiveStageBuyTimes   int32  `xml:"ActiveStageBuyTimes,attr"`
	GoldFingerBonus       int32  `xml:"GoldFingerBonus,attr"`
	HonorPointBonus       int32  `xml:"HonorPointBonus,attr"`
	MonthCardItemBonusStr string `xml:"MonthCardItemBonus,attr"`
	MonthCardItemBonus    []int32
	SearchTaskCount       int32 `xml:"SearchTaskCount,attr"`
}

type XmlVipConfig struct {
	Items []XmlVipItem `xml:"item"`
}

type VipTableMgr struct {
	Map   map[int32]*XmlVipItem
	Array []*XmlVipItem
}

func (this *VipTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("VipTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *VipTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "Vip.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("VipTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlVipConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("VipTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlVipItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlVipItem, 0)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlVipItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		tmp_item.MonthCardItemBonus = parse_xml_str_arr2(tmp_item.MonthCardItemBonusStr, ",")
		/*if tmp_item.MonthCardItemBonus == nil || len(tmp_item.MonthCardItemBonus) == 0 {
			log.Error("VipTableMgr parse column %v field[MonthCardItemBonusStr] invalid", idx)
			return false
		}*/

		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *VipTableMgr) Get(level int32) *XmlVipItem {
	return this.Map[level]
}
