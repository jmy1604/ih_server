package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

const (
	CARNIVAL_EVENT_NONE              = iota
	CARNIVAL_EVENT_COMMENT           = 401 // 限时评论
	CARNIVAL_EVENT_FOCUS_COMMUNITY   = 402 // 关注社区
	CARNIVAL_EVENT_SHARE             = 403 // 分享
	CARNIVAL_EVENT_INVITE            = 404 // 邀请好友
	CARNIVAL_EVENT_ITEM_EXCHANGE     = 405 // 物品兑换
	CARNIVAL_EVENT_BE_INVITED_REWARD = 406 // 被邀请奖励
)

type XmlCarnivalTaskItem struct {
	Id            int32  `xml:"ID,attr"`
	ResetTimeType int32  `xml:"ResetTimeType,attr"`
	EventType     int32  `xml:"ActiveType,attr"`
	Param1        int32  `xml:"Param1,attr"`
	Param2        int32  `xml:"Param2,attr"`
	Param3        int32  `xml:"Param3,attr"`
	Param4        int32  `xml:"Param4,attr"`
	EventCount    int32  `xml:"EventCount,attr"`
	RewardStr     string `xml:"Reward,attr"`
	Reward        []int32
	RewardMailId  int32 `xml:"RewardMailID,attr"`
}

type XmlCarnivalTaskConfig struct {
	Items []XmlCarnivalTaskItem `xml:"item"`
}

type CarnivalTaskTableMgr struct {
	Map             map[int32]*XmlCarnivalTaskItem
	Array           []*XmlCarnivalTaskItem
	InviteTasks     []*XmlCarnivalTaskItem
	RoundResetTasks []*XmlCarnivalTaskItem
	DayResetTasks   []*XmlCarnivalTaskItem
}

func (this *CarnivalTaskTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("CarnivalTaskTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *CarnivalTaskTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "CarnivalSub.xml"
	}

	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("CarnivalTaskTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlCarnivalTaskConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("CarnivalTaskTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlCarnivalTaskItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlCarnivalTaskItem, 0)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlCarnivalTaskItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		tmp_item.Reward = parse_xml_str_arr2(tmp_item.RewardStr, ",")
		if tmp_item.Reward == nil {
			log.Error("CarnivalTaskTableMgr parse column[Reward] with value[%v] on line %v", tmp_item.RewardStr, idx)
			return false
		}

		if tmp_item.EventCount <= 0 {
			tmp_item.EventCount = 1
		}

		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
		if tmp_item.EventType == CARNIVAL_EVENT_INVITE {
			this.InviteTasks = append(this.InviteTasks, tmp_item)
		}
		if tmp_item.ResetTimeType == 1 {
			this.RoundResetTasks = append(this.RoundResetTasks, tmp_item)
		} else if tmp_item.ResetTimeType == 2 {
			this.DayResetTasks = append(this.DayResetTasks, tmp_item)
		}
	}

	return true
}

func (this *CarnivalTaskTableMgr) Get(id int32) *XmlCarnivalTaskItem {
	return this.Map[id]
}

func (this *CarnivalTaskTableMgr) GetInviteTasks() []*XmlCarnivalTaskItem {
	return this.InviteTasks
}

func (this *CarnivalTaskTableMgr) GetRoundResetTasks() []*XmlCarnivalTaskItem {
	return this.RoundResetTasks
}

func (this *CarnivalTaskTableMgr) GetDayResetTasks() []*XmlCarnivalTaskItem {
	return this.DayResetTasks
}
