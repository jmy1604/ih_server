package main

import (
	"ih_server/libs/log"
	"ih_server/libs/utils"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"ih_server/src/table_config"
	"time"

	"github.com/golang/protobuf/proto"
)

// 任务状态
const (
	TASK_STATE_DOING    = 0 // 正在进行
	TASK_STATE_COMPLETE = 1 // 完成
	TASK_STATE_REWARD   = 2 // 已领奖
)

func (this *dbPlayerTaskColumn) has_reward(task_type int32) bool {
	this.m_row.m_lock.UnSafeLock("dbPlayerTaskColumn.has_reward")
	defer this.m_row.m_lock.UnSafeUnlock()

	for _, d := range this.m_data {
		task := task_table_mgr.GetTask(d.Id)
		if task == nil {
			continue
		}

		if d.State == TASK_STATE_DOING {
			if task.CompleteNum <= d.Value {
				d.State = TASK_STATE_COMPLETE
				this.m_changed = true
			}
		} else if d.State == TASK_STATE_COMPLETE {
			if task.CompleteNum > d.Value {
				d.State = TASK_STATE_DOING
				this.m_changed = true
			}
		}

		if task_type > 0 && task.Type != task_type {
			continue
		}

		if task.Type == table_config.TASK_TYPE_ACHIVE && task.Prev > 0 && this.m_data[task.Prev] != nil {
			continue
		}
		if d.State == TASK_STATE_COMPLETE {
			if task_type == 0 {
				return true
			} else if task_type == table_config.TASK_TYPE_DAILY {
				return true
			} else if task_type == table_config.TASK_TYPE_ACHIVE {
				return true
			}
		}
	}

	return false
}

func (this *dbPlayerTaskColumn) ResetDailyTask() {
	this.m_row.m_lock.UnSafeLock("dbPlayerTaskColumn.ChkResetDailyTask")
	defer this.m_row.m_lock.UnSafeUnlock()

	daily_tasks := task_table_mgr.GetDailyTasks()
	if daily_tasks == nil {
		return
	}

	for id, task := range daily_tasks {
		d := this.m_data[id]
		if d == nil {
			data := &dbPlayerTaskData{}
			data.Id = task.Id
			data.Value = 0
			this.m_data[id] = data
		} else {
			d.Value = 0
			d.State = 0
		}
	}

	this.m_changed = true

	return
}

func (this *Player) fill_task_msg(task_type int32) (task_list []*msg_client_message.TaskData) {
	tasks := task_table_mgr.GetTasks(task_type)
	if tasks == nil {
		return
	}

	for i := 0; i < len(tasks); i++ {
		t := tasks[i]
		if !this.db.Tasks.HasIndex(t.Id) {
			if this.db.FinishedTasks.HasIndex(t.Id) {
				continue
			}
			this.db.Tasks.Add(&dbPlayerTaskData{
				Id: t.Id,
			})
		}

		v, _ := this.db.Tasks.GetValue(t.Id)
		s, _ := this.db.Tasks.GetState(t.Id)
		if this.db.Tasks.HasIndex(t.Id) {
			if s == TASK_STATE_DOING && v >= t.CompleteNum {
				s = TASK_STATE_COMPLETE
				//this.db.Tasks.SetState(t.Id, s)
			} else if s == TASK_STATE_COMPLETE && v < t.CompleteNum {
				s = TASK_STATE_DOING
				this.db.Tasks.SetState(t.Id, s)
			} else {
				// 特殊判断等级和VIP等级，解决因为丢失任务数据不能完成的问题
				if s == TASK_STATE_DOING {
					var complete bool
					if t.EventId == table_config.TASK_COMPLETE_TYPE_REACH_LEVEL {
						if this.db.GetLevel() >= t.EventParam {
							complete = true
						}
					} else if t.EventId == table_config.TASK_COMPLETE_TYPE_REACH_VIP_N_LEVEL {
						if this.db.Info.GetVipLvl() >= t.EventParam {
							complete = true
						}
					}
					if complete {
						s = TASK_STATE_COMPLETE
						v = t.CompleteNum
						this.db.Tasks.SetValue(t.Id, t.CompleteNum)
					}
				}
			}
		}

		if t.Type != task_type {
			continue
		}

		if t.Prev > 0 && this.db.Tasks.HasIndex(t.Prev) {
			continue
		}

		task_list = append(task_list, &msg_client_message.TaskData{
			Id:    t.Id,
			Value: v,
			State: s,
		})
	}

	return
}

func (this *Player) ChkPlayerDailyTask() int32 {
	remain_seconds := utils.GetRemainSeconds2NextDayTime(this.db.TaskCommon.GetLastRefreshTime(), global_config.DailyTaskRefreshTime)
	if remain_seconds <= 0 {
		this.db.Tasks.ResetDailyTask()
		now_time := int32(time.Now().Unix())
		this.db.TaskCommon.SetLastRefreshTime(now_time)
		remain_seconds = utils.GetRemainSeconds2NextDayTime(now_time, global_config.DailyTaskRefreshTime)
	}
	return remain_seconds
}

func (this *Player) check_and_send_daily_task() {
	if this.ChkPlayerDailyTask() <= 0 {
		this.send_task(table_config.TASK_TYPE_DAILY)
	}
}

func (this *Player) first_gen_achieve_tasks() {
	if this.db.Tasks.NumAll() > 0 {
		this.db.Tasks.Clear()
	}
	achieves := task_table_mgr.GetAchieveTasks()
	if achieves != nil {
		for i := 0; i < len(achieves); i++ {
			this.db.Tasks.Add(&dbPlayerTaskData{
				Id: achieves[i].Id,
			})
		}
	}
}

func (this *Player) send_task(task_type int32) int32 {
	var response msg_client_message.S2CTaskDataResponse
	if task_type == 0 || task_type == table_config.TASK_TYPE_DAILY {
		remain_seconds := this.ChkPlayerDailyTask()
		response.TaskType = table_config.TASK_TYPE_DAILY
		response.TaskList = this.fill_task_msg(table_config.TASK_TYPE_DAILY)
		response.DailyTaskRefreshRemainSeconds = remain_seconds
		this.Send(uint16(msg_client_message_id.MSGID_S2C_TASK_DATA_RESPONSE), &response)
		log.Trace("Player[%v] daily tasks %v", this.Id, response)
	}

	if task_type == 0 || task_type == table_config.TASK_TYPE_ACHIVE {
		response.TaskType = table_config.TASK_TYPE_ACHIVE
		response.TaskList = this.fill_task_msg(table_config.TASK_TYPE_ACHIVE)
		this.Send(uint16(msg_client_message_id.MSGID_S2C_TASK_DATA_RESPONSE), &response)
		log.Trace("Player[%v] achive tasks %v", this.Id, response)
	}

	return 1
}

// ============================================================================

func (this *Player) NotifyTaskValue(notify_task *msg_client_message.S2CTaskValueNotify, task_id, value, state int32) {
	notify_task.Data = &msg_client_message.TaskData{}
	notify_task.Data.Id = task_id
	notify_task.Data.Value = value
	notify_task.Data.State = state
	this.Send(uint16(msg_client_message_id.MSGID_S2C_TASK_VALUE_NOTIFY), notify_task)
}

// 任务是否完成
func (this *Player) IsTaskComplete(task *table_config.XmlTaskItem) bool {
	if task.Type == table_config.TASK_TYPE_DAILY || task.Type == table_config.TASK_TYPE_ACHIVE {
		task_data := this.db.Tasks.Get(task.Id)
		if task_data == nil {
			return false
		}
		if task_data.Value < task.CompleteNum {
			return false
		}
	} else {
		return false
	}
	return true
}

// 单个日常任务更新
func (this *Player) SingleTaskUpdate(task *table_config.XmlTaskItem, add_val int32) (updated bool, cur_val int32, cur_state int32) {
	if this.db.Tasks.HasIndex(task.Id) {
		// 已领奖
		state, _ := this.db.Tasks.GetState(task.Id)
		if state == TASK_STATE_REWARD {
			return
		}

		value, _ := this.db.Tasks.GetValue(task.Id)
		if task.CompleteNum > value {
			cur_val = this.db.Tasks.IncbyValue(task.Id, add_val)
			updated = true
		}
	} else {
		this.db.Tasks.Add(&dbPlayerTaskData{
			Id:    task.Id,
			Value: add_val,
		})
		cur_val = add_val
		updated = true
	}

	if cur_val >= task.CompleteNum {
		cur_state = TASK_STATE_COMPLETE
		//this.db.Tasks.SetState(task.Id, TASK_STATE_COMPLETE)
	} else {
		cur_state = TASK_STATE_DOING
	}
	return
}

// 完成所有日常任务更新
func (this *Player) WholeDailyTaskUpdate(daily_task *table_config.XmlTaskItem, notify_task *msg_client_message.S2CTaskValueNotify) {
	whole_daily_task := task_table_mgr.GetWholeDailyTask()
	if whole_daily_task == nil || this.IsTaskComplete(whole_daily_task) {
		return
	}

	if daily_task.EventId != table_config.TASK_COMPLETE_TYPE_ALL_DAILY {
		to_send, cur_val, cur_state := this.SingleTaskUpdate(whole_daily_task, 1)
		if to_send {
			this.NotifyTaskValue(notify_task, whole_daily_task.Id, cur_val, cur_state)
			log.Trace("Player(%v) WholeDailyTask(%v) Update, Progress(%v/%v), Complete(%v)", this.Id, whole_daily_task.Id, cur_val, whole_daily_task.CompleteNum, cur_state)
		}
	}
}

// 任务更新
func (this *Player) TaskUpdate(complete_type int32, if_not_less bool, event_param int32, value int32) {
	//log.Info("complete_type[%d] event_param[%v] aval[%d]", complete_type, event_param, value)
	var idx int32
	var cur_val, cur_state int32

	var notify_task msg_client_message.S2CTaskValueNotify
	ftasks := task_table_mgr.GetFinishTasks()[complete_type]
	if nil == ftasks || ftasks.GetCount() == 0 {
		log.Warn("Task complete type %v no corresponding tasks", complete_type)
		return
	}

	var taskcfg *table_config.XmlTaskItem
	for idx = 0; idx < ftasks.GetCount(); idx++ {
		taskcfg = ftasks.GetArray()[idx]

		if !this.db.Tasks.HasIndex(taskcfg.Id) {
			continue
		}

		// 已完成
		if this.IsTaskComplete(taskcfg) {
			continue
		}

		// 事件参数
		if taskcfg.EventParam > 0 {
			if if_not_less {
				if event_param < taskcfg.EventParam {
					continue
				}
			} else {
				// 参数不一致
				if event_param != taskcfg.EventParam {
					continue
				}
			}
		}

		var updated bool
		updated, cur_val, cur_state = this.SingleTaskUpdate(taskcfg, value)

		if updated && !(taskcfg.Prev > 0 && this.db.Tasks.HasIndex(taskcfg.Prev)) {
			this.NotifyTaskValue(&notify_task, taskcfg.Id, cur_val, cur_state)
			log.Trace("Player[%v] Task[%v] EventParam[%v] Progress[%v/%v] FinishType(%v) Complete(%v)", this.Id, taskcfg.Id, event_param, cur_val, taskcfg.CompleteNum, complete_type, cur_state)
			if taskcfg.Type == table_config.TASK_TYPE_DAILY && cur_state == TASK_STATE_COMPLETE {
				// 所有日常任务更新
				this.WholeDailyTaskUpdate(taskcfg, &notify_task)
			}
		}
	}

	this.check_and_send_daily_task()
}

func (this *Player) check_notify_next_task(task *table_config.XmlTaskItem) {
	if task.Next <= 0 {
		return
	}
	next_task := task_table_mgr.GetTask(task.Next)
	if next_task == nil {
		return
	}

	if !this.db.Tasks.HasIndex(task.Next) {
		return
	}

	v, _ := this.db.Tasks.GetValue(task.Next)
	s, _ := this.db.Tasks.GetState(task.Next)

	if next_task.CompleteNum <= v {
		s = TASK_STATE_COMPLETE
	}

	notify := &msg_client_message.S2CTaskValueNotify{}
	this.NotifyTaskValue(notify, task.Next, v, s)
	log.Trace("Player[%v] notify new task %v value %v state %v", this.Id, task.Next, v, s)
}

func (p *Player) task_get_reward(task_id int32) int32 {
	state, _ := p.db.Tasks.GetState(task_id)
	if state == TASK_STATE_REWARD {
		log.Error("Player[%v] task[%v] already award", p.Id, task_id)
		return int32(msg_client_message.E_ERR_PLAYER_TASK_ALREADY_REWARDED)
	}

	task_cfg := task_table_mgr.GetTaskMap()[task_id]
	if nil == task_cfg {
		log.Error("task %v table data not found", task_id)
		return int32(msg_client_message.E_ERR_PLAYER_TASK_NOT_FOUND)
	}

	cur_val, _ := p.db.Tasks.GetValue(task_id)
	if cur_val < task_cfg.CompleteNum {
		log.Error("Player[%v] task %v not finished(%d < %d)", p.Id, task_id, cur_val, task_cfg.CompleteNum)
		return int32(msg_client_message.E_ERR_PLAYER_TASK_NOT_COMPLETE)
	}

	p.add_resources(task_cfg.Rewards)
	p.db.Tasks.SetState(task_id, TASK_STATE_REWARD)
	notify_task := &msg_client_message.S2CTaskValueNotify{}
	p.NotifyTaskValue(notify_task, task_id, cur_val, TASK_STATE_REWARD)

	response := &msg_client_message.S2CTaskRewardResponse{
		TaskId: task_id,
	}
	p.Send(uint16(msg_client_message_id.MSGID_S2C_TASK_REWARD_RESPONSE), response)

	log.Trace("Player[%v] get task %v reward", p.Id, task_id)

	if task_cfg.Type == table_config.TASK_TYPE_ACHIVE {
		if task_cfg.Next > 0 {
			p.db.Tasks.Remove(task_id)
			var data dbPlayerFinishedTaskData
			data.Id = task_id
			p.db.FinishedTasks.Add(&data)

			// 后置任务
			p.check_notify_next_task(task_cfg)
		}
	}

	p.check_and_send_daily_task()

	return 1
}

// ============================================================================

func C2STaskDataHanlder(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2STaskDataRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}

	return p.send_task(req.GetTaskType())
}

func C2SGetTaskRewardHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2STaskRewardRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}

	return p.task_get_reward(req.GetTaskId())
}
