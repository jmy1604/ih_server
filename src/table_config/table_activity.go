package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
	"time"
)

type XmlActivityItem struct {
	Id               int32  `xml:"MainActiveID,attr"`
	Type             int32  `xml:"ActiveType,attr"`
	EventId          int32  `xml:"EventID,attr"`
	StartTimeStr     string `xml:"StartTime,attr"`
	EndTimeStr       string `xml:"EndTime,attr"`
	SubActiveListStr string `xml:"SubActiveList,attr"`
	RewardMailId     int32  `xml:"RewardMailID,attr"`
	StartTime        int32
	EndTime          int32
	SubActiveList    []int32
}

type XmlActivityConfig struct {
	Items []XmlActivityItem `xml:"item"`
}

type ActivityTableMgr struct {
	Map   map[int32]*XmlActivityItem
	Array []*XmlActivityItem
}

func (this *ActivityTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("ActivityTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *ActivityTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "MainActive.xml"
	}

	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("ActivityTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlActivityConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("ActivityTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlActivityItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlActivityItem, 0)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var t time.Time
	var tmp_item *XmlActivityItem

	var loc *time.Location
	loc, err = time.LoadLocation("Local")
	if err != nil {
		log.Error("!!!!!!! Load Location Local error[%v]", err.Error())
		return false
	}

	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		t, err = time.ParseInLocation("2006-01-02 15:04:05", tmp_item.StartTimeStr, loc)
		if err != nil {
			log.Error("ActivityTableMgr parse column[StartTime] with line %v error %v", idx, err.Error())
			return false
		}
		tmp_item.StartTime = int32(t.Unix())

		t, err = time.ParseInLocation("2006-01-02 15:04:05", tmp_item.EndTimeStr, loc)
		if err != nil {
			log.Error("ActivityTableMgr parse column[EndTime] with line %v error %v", idx, err.Error())
			return false
		}
		tmp_item.EndTime = int32(t.Unix())

		tmp_item.SubActiveList = parse_xml_str_arr2(tmp_item.SubActiveListStr, ",")
		if tmp_item.SubActiveList == nil {
			tmp_item.SubActiveList = []int32{}
		}

		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *ActivityTableMgr) Get(id int32) *XmlActivityItem {
	return this.Map[id]
}
