package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
	"time"
)

type XmlCarnivalItem struct {
	Round        int32  `xml:"Round,attr"`
	StartTimeStr string `xml:"StartTime,attr"`
	EndTimeStr   string `xml:"EndTime,attr"`
	StartTime    int32
	EndTime      int32
}

type XmlCarnivalConfig struct {
	Items []XmlCarnivalItem `xml:"item"`
}

type CarnivalTableMgr struct {
	Map   map[int32]*XmlCarnivalItem
	Array []*XmlCarnivalItem
}

func (this *CarnivalTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("CarnivalTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *CarnivalTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "Carnival.xml"
	}

	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("CarnivalTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlCarnivalConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("CarnivalTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlCarnivalItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlCarnivalItem, 0)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var t time.Time
	var tmp_item *XmlCarnivalItem

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
			log.Error("CarnivalTableMgr parse column[StartTime] with line %v error %v", idx, err.Error())
			return false
		}
		tmp_item.StartTime = int32(t.Unix())

		t, err = time.ParseInLocation("2006-01-02 15:04:05", tmp_item.EndTimeStr, loc)
		if err != nil {
			log.Error("CarnivalTableMgr parse column[EndTime] with line %v error %v", idx, err.Error())
			return false
		}
		tmp_item.EndTime = int32(t.Unix())

		this.Map[tmp_item.Round] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *CarnivalTableMgr) Get(round int32) *XmlCarnivalItem {
	return this.Map[round]
}
