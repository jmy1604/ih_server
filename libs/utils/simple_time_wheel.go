package utils

import (
	"ih_server/libs/log"
	"sync/atomic"
	"time"
)

const (
	SIMPLE_TIMER_CHAN_LENGTH        = 4096      // 计时器插入缓冲队列长度
	DEFAULT_TIMER_INTERVAL_MSECONDS = 10        // 默认计时器间隔作用时间(毫秒)
	DEFAULT_TIME_PERIOD_SECONDS     = 24 * 3600 // 默认计时器总时间
)

type SimpleTimerFunc func(interface{}) int32

type SimpleTimerData struct {
	timer_id       int32           // 计时器ID
	timer_func     SimpleTimerFunc // 处理函数
	param          interface{}     // 参数
	begin_msecs    int32           // 是否马上开始
	interval_msecs int32           // 作用时间间隔(毫秒)
	total_num      int32           // 总作用次数
	curr_num       int32           // 当前已作用次数
}

type SimpleTimerOpData struct {
	op   int32            // 1 插入  2 删除
	data *SimpleTimerData // 数据
}

type SimpleTimer struct {
	data        *SimpleTimerData // 数据
	next        *SimpleTimer     // 下一个
	prev        *SimpleTimer     // 上一个
	parent_list *SimpleTimerList // 计时器链表
}

type SimpleTimerList struct {
	head *SimpleTimer
	tail *SimpleTimer
}

func (this *SimpleTimerList) add(data *SimpleTimerData) *SimpleTimer {
	node := &SimpleTimer{
		data: data,
	}
	if this.head == nil {
		this.head = node
		this.tail = node
	} else {
		node.prev = this.tail
		this.tail.next = node
		this.tail = node
	}
	return node
}

func (this *SimpleTimerList) remove(timer *SimpleTimer) {
	if timer.prev != nil {
		timer.prev.next = timer.next
	}
	if timer.next != nil {
		timer.next.prev = timer.prev
	}
	if timer == this.head {
		this.head = timer.next
	}
	if timer == this.tail {
		this.tail = timer.prev
	}
}

type SimpleTimeWheel struct {
	timer_lists             []*SimpleTimerList
	curr_timer_index        int32
	last_check_time         int64
	timer_interval_mseconds int32
	op_chan                 chan *SimpleTimerOpData
	curr_timer_id           int32
	id2timer                map[int32]*SimpleTimer
}

func NewSimpleTimeWheel() *SimpleTimeWheel {
	stw := &SimpleTimeWheel{}
	if !stw.Init(0, 0) {
		return nil
	}
	return stw
}

func (this *SimpleTimeWheel) Init(timer_interval_mseconds, time_period_seconds int32) bool {
	if timer_interval_mseconds == 0 {
		timer_interval_mseconds = DEFAULT_TIMER_INTERVAL_MSECONDS
	}
	if time_period_seconds == 0 {
		time_period_seconds = DEFAULT_TIME_PERIOD_SECONDS
	}
	if (time_period_seconds*1000)%timer_interval_mseconds != 0 {
		return false
	}
	this.timer_lists = make([]*SimpleTimerList, time_period_seconds*1000/timer_interval_mseconds)
	this.curr_timer_index = -1
	this.timer_interval_mseconds = timer_interval_mseconds
	this.op_chan = make(chan *SimpleTimerOpData, SIMPLE_TIMER_CHAN_LENGTH)
	this.id2timer = make(map[int32]*SimpleTimer)
	return true
}

func (this *SimpleTimeWheel) Insert(timer_func SimpleTimerFunc, param interface{}, begin_msecs, interval_msecs, total_effect_msecs int32) int32 {
	if interval_msecs == 0 || total_effect_msecs == 0 {
		log.Error("Insert new simple timer failed, interval_seconds or total_effect_mseconds cant to be set zero")
		return -1
	}
	new_timer_id := atomic.AddInt32(&this.curr_timer_id, 1)
	data := &SimpleTimerData{
		timer_id:       new_timer_id,
		timer_func:     timer_func,
		param:          param,
		begin_msecs:    begin_msecs,
		interval_msecs: interval_msecs,
		total_num:      total_effect_msecs / interval_msecs,
	}
	this.op_chan <- &SimpleTimerOpData{
		op:   1,
		data: data,
	}
	return new_timer_id
}

func (this *SimpleTimeWheel) Remove(timer_id int32) {
	this.op_chan <- &SimpleTimerOpData{
		op: 2,
		data: &SimpleTimerData{
			timer_id: timer_id,
		},
	}
}

func (this *SimpleTimeWheel) insert(data *SimpleTimerData) bool {
	if data.curr_num >= data.total_num {
		return false
	}
	lists_len := int32(len(this.timer_lists))
	var insert_list_index int32
	if data.curr_num == 0 {
		insert_list_index = (this.curr_timer_index + 1 + (data.begin_msecs+DEFAULT_TIMER_INTERVAL_MSECONDS-1)/DEFAULT_TIMER_INTERVAL_MSECONDS) % lists_len
	} else {
		insert_list_index = (this.curr_timer_index + 1 + (data.interval_msecs+DEFAULT_TIMER_INTERVAL_MSECONDS-1)/DEFAULT_TIMER_INTERVAL_MSECONDS) % lists_len
	}
	data.curr_num += 1
	list := this.timer_lists[insert_list_index]
	if list == nil {
		list = &SimpleTimerList{}
		this.timer_lists[insert_list_index] = list
	}
	timer := list.add(data)
	tmp_timer := this.id2timer[data.timer_id]
	if tmp_timer != nil {
		this.remove(tmp_timer)
		log.Warn("SimpleTimeWheel already exists timer[%v], remove it", data.timer_id)
	}
	this.id2timer[data.timer_id] = timer
	//log.Debug("Player[%v] conn insert in index[%v] list", player_id, insert_list_index)
	return true
}

func (this *SimpleTimeWheel) remove(timer *SimpleTimer) bool {
	timer.parent_list.remove(timer)
	delete(this.id2timer, timer.data.timer_id)
	return true
}

func (this *SimpleTimeWheel) remove_by_id(timer_id int32) bool {
	timer := this.id2timer[timer_id]
	if timer == nil {
		return false
	}
	this.remove(timer)
	return true
}

func (this *SimpleTimeWheel) Run() {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	for {
		// 处理操作队列
		is_break := false
		for !is_break {
			select {
			case d, ok := <-this.op_chan:
				{
					if !ok {
						log.Error("conn timer wheel op chan receive invalid !!!!!")
						return
					}

					if d.op == 1 {
						this.insert(d.data)
					} else if d.op == 2 {
						this.remove_by_id(d.data.timer_id)
					}
				}
			default:
				{
					is_break = true
				}
			}
		}

		now_time := int64(time.Now().Unix()*1000 + time.Now().UnixNano()/1000000)
		if this.last_check_time == 0 {
			this.last_check_time = now_time
		}
		// 跟上一次相差毫秒数
		diff_msecs := int32(now_time - this.last_check_time)
		y := diff_msecs / this.timer_interval_mseconds
		if y > 0 {
			var idx int32
			lists_len := int32(len(this.timer_lists))
			if y >= lists_len {
				if this.curr_timer_index > 0 {
					idx = this.curr_timer_index - 1
				} else {
					idx = lists_len - 1
				}
			} else {
				idx = (this.curr_timer_index + y) % lists_len
			}

			i := (this.curr_timer_index + 1) % lists_len
			for {
				list := this.timer_lists[i]
				if list != nil {
					t := list.head
					for t != nil {
						// execute timer function
						t.data.timer_func(t.data.param)
						this.remove(t)
						t = t.next
					}
					this.timer_lists[i] = nil
				}
				if i == idx {
					break
				}
				i = (i + 1) % lists_len
			}
			this.curr_timer_index = idx
			this.last_check_time = now_time
		}

		time.Sleep(time.Millisecond * 2)
	}
}
