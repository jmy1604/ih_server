package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

// 任务类型
const (
	TASK_TYPE_DAILY  = 1 // 日常
	TASK_TYPE_ACHIVE = 2 // 成就
)

// 任务完成类型
const (
	TASK_COMPLETE_TYPE_ALL_DAILY              = 101 // 完成所有日常任务
	TASK_COMPLETE_TYPE_ARENA_FIGHT_NUM        = 102 // 完成N次竞技场战斗
	TASK_COMPLETE_TYPE_ACTIVE_STAGE_WIN_NUM   = 103 // 获得N次活动副本胜利
	TASK_COMPLETE_TYPE_EXPLORE_NUM            = 104 // 完成N次日常探索
	TASK_COMPLETE_TYPE_DRAW_NUM               = 105 // 完成N次抽取英雄                    参数   1 高级  2 普通
	TASK_COMPLETE_TYPE_HUANG_UP_NUM           = 106 // 领取N次挂机收益
	TASK_COMPLETE_TYPE_BUY_ITEM_NUM_ON_SHOP   = 107 // 神秘商店购买商品
	TASK_COMPLETE_TYPE_GIVE_POINTS_NUM        = 108 // 赠送N颗爱心
	TASK_COMPLETE_TYPE_GOLD_HAND_NUM          = 109 // 完成N次点金
	TASK_COMPLETE_TYPE_FORGE_EQUIP_NUM        = 110 // 合成N件装备
	TASK_COMPLETE_TYPE_PASS_CAMPAIGN          = 201 // 通过战役                           参数   指定战役ID
	TASK_COMPLETE_TYPE_REACH_LEVEL            = 202 // 等级提升到N级
	TASK_COMPLETE_TYPE_GET_STAR_ROLES         = 203 // 获得N个M星英雄(历史总数)           参数   英雄等级
	TASK_COMPLETE_TYPE_DECOMPOSE_ROLES        = 204 // 分解N个英雄(历史总数)
	TASK_COMPLETE_TYPE_ARENA_WIN_NUM          = 205 // 冠军争夺战进攻获胜N场次(历史总数)
	TASK_COMPLETE_TYPE_ARENA_REACH_SCORE      = 206 // 冠军争夺战积分达到N分(历史最高分)
	TASK_COMPLETE_TYPE_PASS_STAR_EXPLORE      = 207 // 完成N个M星探索任务                 参数   星级
	TASK_COMPLETE_TYPE_LEVELUP_ROLE_WITH_CAMP = 208 // 将一名某阵营角色升到指定等级       参数   角色阵营
	TASK_COMPLETE_TYPE_REACH_VIP_N_LEVEL      = 209 // 成为VIP N
	TASK_COMPLETE_TYPE_GET_QUALITY_EQUIPS_NUM = 210 // 获得N件某品质装备                  参数   装备品质
)

type TaskReward struct {
	ItemId int32
	Num    int32
}

type XmlTaskItem struct {
	Id          int32  `xml:"Id,attr"`
	Type        int32  `xml:"Type,attr"`
	EventId     int32  `xml:"EventId,attr"`
	EventParam  int32  `xml:"EventParam,attr"`
	CompleteNum int32  `xml:"CompleteNum,attr"`
	Prev        int32  `xml:"Prev,attr"`
	Next        int32  `xml:"Next,attr"`
	RewardStr   string `xml:"Reward,attr"`
	Rewards     []int32
}

type XmlTaskTable struct {
	Items []XmlTaskItem `xml:"item"`
}

type FinishTypeTasks struct {
	count int32
	array []*XmlTaskItem
}

func (this *FinishTypeTasks) GetCount() int32 {
	return this.count
}

func (this *FinishTypeTasks) GetArray() []*XmlTaskItem {
	return this.array
}

type TaskTableMgr struct {
	task_map          map[int32]*XmlTaskItem     // 任务map
	task_array        []*XmlTaskItem             // 任务数组
	task_array_len    int32                      // 数组长度
	finish_tasks      map[int32]*FinishTypeTasks // 按完成条件组织任务数据
	daily_task_map    map[int32]*XmlTaskItem     // 日常任务MAP
	daily_task_array  []*XmlTaskItem             // 日常任务数组
	all_daily_task    *XmlTaskItem               // 所有日常任务
	achieve_tasks_map map[int32]*XmlTaskItem     // 成就任务MAP
	achieve_tasks     []*XmlTaskItem             // 初始成就任务
}

func (this *TaskTableMgr) Init(table_file string) bool {
	if !this.LoadTask(table_file) {
		return false
	}
	return true
}

func (this *TaskTableMgr) LoadTask(table_file string) bool {
	if table_file == "" {
		table_file = "Mission.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	content, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("TaskTableMgr LoadTask read file error !")
		return false
	}

	tmp_cfg := &XmlTaskTable{}
	err = xml.Unmarshal(content, tmp_cfg)
	if nil != err {
		log.Error("TaskTableMgr LoadTask unmarshal failed(%s)", err.Error())
		return false
	}

	tmp_len := int32(len(tmp_cfg.Items))

	this.task_array = make([]*XmlTaskItem, 0, tmp_len)
	this.task_map = make(map[int32]*XmlTaskItem)
	this.finish_tasks = make(map[int32]*FinishTypeTasks)
	this.daily_task_map = make(map[int32]*XmlTaskItem)
	this.achieve_tasks_map = make(map[int32]*XmlTaskItem)

	var tmp_item *XmlTaskItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		rewards := parse_xml_str_arr2(tmp_item.RewardStr, ",")
		if rewards == nil || len(rewards)%2 != 0 {
			log.Error("@@@@@@ Task[%v] Reward[%v] invalid", tmp_item.Id, tmp_item.RewardStr)
			return false
		}

		tmp_item.Rewards = rewards
		if tmp_item.EventId != TASK_COMPLETE_TYPE_ALL_DAILY && tmp_item.CompleteNum <= 0 {
			tmp_item.CompleteNum = 1
		}

		this.task_map[tmp_item.Id] = tmp_item
		this.task_array = append(this.task_array, tmp_item)
		if nil == this.finish_tasks[tmp_item.EventId] {
			this.finish_tasks[tmp_item.EventId] = &FinishTypeTasks{}
		}
		this.finish_tasks[tmp_item.EventId].count++
		if tmp_item.Type == TASK_TYPE_DAILY {
			this.daily_task_map[tmp_item.Id] = tmp_item
			this.daily_task_array = append(this.daily_task_array, tmp_item)
			if tmp_item.EventId == TASK_COMPLETE_TYPE_ALL_DAILY {
				this.all_daily_task = tmp_item
			}
		} else if tmp_item.Type == TASK_TYPE_ACHIVE {
			this.achieve_tasks = append(this.achieve_tasks, tmp_item)
			this.achieve_tasks_map[tmp_item.Id] = tmp_item
		}
	}
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]
		if nil == this.finish_tasks[tmp_item.EventId].array {
			this.finish_tasks[tmp_item.EventId].array = make([]*XmlTaskItem, 0, this.finish_tasks[tmp_item.EventId].count)
		}
		this.finish_tasks[tmp_item.EventId].array = append(this.finish_tasks[tmp_item.EventId].array, tmp_item)
	}

	this.task_array_len = int32(len(this.task_array))

	// 所有日常任务CompleteNum处理
	if this.all_daily_task != nil {
		for _, d := range this.daily_task_map {
			if d.EventId != TASK_COMPLETE_TYPE_ALL_DAILY {
				this.all_daily_task.CompleteNum += 1
			}
		}
	}

	log.Info("TaskTableMgr Loaded Task table")

	return true
}

func (this *TaskTableMgr) GetTaskMap() map[int32]*XmlTaskItem {
	return this.task_map
}

func (this *TaskTableMgr) GetTask(task_id int32) *XmlTaskItem {
	if this.task_map == nil {
		return nil
	}
	return this.task_map[task_id]
}

func (this *TaskTableMgr) GetWholeDailyTask() *XmlTaskItem {
	return this.all_daily_task
}

func (this *TaskTableMgr) GetFinishTasks() map[int32]*FinishTypeTasks {
	return this.finish_tasks
}

func (this *TaskTableMgr) GetDailyTasks() map[int32]*XmlTaskItem {
	return this.daily_task_map
}

func (this *TaskTableMgr) GetAchieveTasks() []*XmlTaskItem {
	return this.achieve_tasks
}

func (this *TaskTableMgr) GetTasks(task_type int32) []*XmlTaskItem {
	if task_type == TASK_TYPE_DAILY {
		return this.daily_task_array
	} else if task_type == TASK_TYPE_ACHIVE {
		return this.achieve_tasks
	}
	return nil
}

func (this *TaskTableMgr) IsDaily(task_id int32) bool {
	if this.daily_task_map != nil {
		if this.daily_task_map[task_id] != nil {
			return true
		}
	}
	return false
}

func (this *TaskTableMgr) IsAchieve(task_id int32) bool {
	if this.achieve_tasks_map != nil {
		if this.achieve_tasks_map[task_id] != nil {
			return true
		}
	}
	return false
}
