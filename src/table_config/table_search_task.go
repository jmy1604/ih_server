package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
	"math/rand"
)

const (
	EXPLORE_TASK_TYPE_RANDOM = 1
	EXPLORE_TASK_TYPE_FIXED  = 2
)

type XmlSearchTaskItem struct {
	Id                  int32  `xml:"Id,attr"`
	Type                int32  `xml:"Type,attr"`
	TaskWeight          int32  `xml:"TaskWeight,attr"`
	TaskStar            int32  `xml:"TaskStar,attr"`
	CardStarCond        int32  `xml:"CardStarCond,attr"`
	CardCampNumCond     int32  `xml:"CardCampNumCond,attr"`
	CardCampCondStr     string `xml:"CardCampCond,attr"`
	CardCampCond        []int32
	CardTypeNumCond     int32  `xml:"CardTypeNumCond,attr"`
	CardTypeCondStr     string `xml:"CardTypeCond,attr"`
	CardTypeCond        []int32
	CardNum             int32  `xml:"CardNum,attr"`
	SearchTime          int32  `xml:"SearchTime,attr"`
	AccelCost           int32  `xml:"AccelCost,attr"`
	ConstRewardStr      string `xml:"ConstReward,attr"`
	ConstReward         []int32
	RandomReward        int32  `xml:"RandomReward,attr"`
	BonusStageLevelCond int32  `xml:"BonusStageLevelCond,attr"`
	BonusStageChance    int32  `xml:"BonusStageChance,attr"`
	BonusStageListID    int32  `xml:"BonusStageListID,attr"`
	TaskHeroNameListStr string `xml:"TaskHeroNameList,attr"`
	TaskHeroNameList    []int32
	TaskNameListStr     string `xml:"TaskNameList,attr"`
	TaskNameList        []int32
}

type XmlSearchTaskConfig struct {
	Items []XmlSearchTaskItem `xml:"item"`
}

type RandomSearchTasks struct {
	tasks        []*XmlSearchTaskItem
	total_weight int32
}

type SearchTaskTableMgr struct {
	Map         map[int32]*XmlSearchTaskItem
	Array       []*XmlSearchTaskItem
	RandomTasks *RandomSearchTasks
}

func (this *SearchTaskTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("SearchTaskTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *SearchTaskTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "SearchTask.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("SearchTaskTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlSearchTaskConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("SearchTaskTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlSearchTaskItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlSearchTaskItem, 0)
	}
	if this.RandomTasks == nil {
		this.RandomTasks = &RandomSearchTasks{}
	}

	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlSearchTaskItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		tmp_item.CardCampCond = parse_xml_str_arr2(tmp_item.CardCampCondStr, ",")
		if tmp_item.CardCampCond == nil {
			log.Error("SearchTaskTableMgr parse field[CardCampCond] failed with column %v", idx)
			return false
		}

		tmp_item.CardTypeCond = parse_xml_str_arr2(tmp_item.CardTypeCondStr, ",")
		if tmp_item.CardTypeCond == nil {
			log.Error("SearchTaskTableMgr parse field[CardTypeCond] failed with column %v", idx)
			return false
		}

		tmp_item.ConstReward = parse_xml_str_arr2(tmp_item.ConstRewardStr, ",")
		if tmp_item.ConstReward == nil {
			log.Error("SearchTaskTableMgr parse field[ConstReward] failed with column %v", idx)
			return false
		}

		tmp_item.TaskHeroNameList = parse_xml_str_arr2(tmp_item.TaskHeroNameListStr, ",")
		if tmp_item.TaskHeroNameList == nil {
			log.Error("SearchTaskTableMgr parse field[TaskHeroNameList] failed with column %v", idx)
			return false
		}

		tmp_item.TaskNameList = parse_xml_str_arr2(tmp_item.TaskNameListStr, ",")
		if tmp_item.TaskNameList == nil {
			log.Error("SearchTaskTableMgr parse field[TaskNameList] failed with column %v", idx)
			return false
		}

		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)

		if tmp_item.Type == EXPLORE_TASK_TYPE_RANDOM {
			this.RandomTasks.tasks = append(this.RandomTasks.tasks, tmp_item)
			this.RandomTasks.total_weight += tmp_item.TaskWeight
		}
	}

	return true
}

func (this *SearchTaskTableMgr) Get(task_id int32) *XmlSearchTaskItem {
	return this.Map[task_id]
}

func (this *SearchTaskTableMgr) RandomTask() *XmlSearchTaskItem {
	r := rand.Int31n(this.RandomTasks.total_weight)
	for i := 0; i < len(this.RandomTasks.tasks); i++ {
		item := this.RandomTasks.tasks[i]
		if r < item.TaskWeight {
			return item
		}
		r -= item.TaskWeight
	}
	return nil
}
