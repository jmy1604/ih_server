package main

import (
	"ih_server/libs/log"
	"sync/atomic"
	"time"
)

const (
	USE_CONN_TIMER_WHEEL = 1
)

type conn_op_data struct {
	op        int32
	player_id int32
}

type ConnTimerWheel struct {
	timer_lists      []*ConnTimerList
	curr_timer_index int32
	last_check_time  int32
	last_players_num int32
	players          map[int32]*ConnTimerPlayer
	op_chan          chan *conn_op_data
}

func (this *ConnTimerWheel) Init() {
	this.timer_lists = make([]*ConnTimerList, global_config.HeartbeatInterval*3)
	this.curr_timer_index = -1
	this.players = make(map[int32]*ConnTimerPlayer)
	this.op_chan = make(chan *conn_op_data, config.MaxClientConnections)
}

func (this *ConnTimerWheel) Insert(player_id int32) {
	this.op_chan <- &conn_op_data{
		op:        1,
		player_id: player_id,
	}
}

func (this *ConnTimerWheel) Remove(player_id int32) {
	this.op_chan <- &conn_op_data{
		op:        2,
		player_id: player_id,
	}
}

func (this *ConnTimerWheel) insert(player_id int32) bool {
	p := this.players[player_id]
	if p != nil {
		this._remove(p)
	}
	lists_len := int32(len(this.timer_lists))
	insert_list_index := (this.curr_timer_index + global_config.HeartbeatInterval) % lists_len
	list := this.timer_lists[insert_list_index]
	if list == nil {
		list = &ConnTimerList{}
		this.timer_lists[insert_list_index] = list
	}
	timer := list.add(player_id)
	if p == nil {
		this.players[player_id] = &ConnTimerPlayer{
			player_id:  player_id,
			timer_list: list,
			timer:      timer,
		}
	} else {
		p.player_id = player_id
		p.timer_list = list
		p.timer = timer
		this.players[player_id] = p
	}
	//log.Debug("Player[%v] conn insert in index[%v] list", player_id, insert_list_index)
	return true
}

func (this *ConnTimerWheel) _remove(p *ConnTimerPlayer) {
	p.timer_list.remove(p.timer)
	delete(this.players, p.player_id)
}

func (this *ConnTimerWheel) remove(player_id int32) bool {
	p := this.players[player_id]
	if p == nil {
		return false
	}
	this._remove(p)
	return true
}

func (this *ConnTimerWheel) Run() {
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
						this.insert(d.player_id)
					} else if d.op == 2 {
						this.remove(d.player_id)
					}
				}
			default:
				{
					is_break = true
				}
			}
		}

		now_time := int32(time.Now().Unix())
		if this.last_check_time == 0 {
			this.last_check_time = now_time
		}
		// 跟上一次相差秒数
		diff_secs := now_time - this.last_check_time
		if diff_secs > 0 {
			var idx int32
			lists_len := int32(len(this.timer_lists))
			if diff_secs >= lists_len {
				if this.curr_timer_index > 0 {
					idx = this.curr_timer_index - 1
				} else {
					idx = lists_len - 1
				}
			} else {
				idx = (this.curr_timer_index + diff_secs) % lists_len
			}

			i := (this.curr_timer_index + 1) % lists_len
			for {
				list := this.timer_lists[i]
				if list != nil {
					t := list.head
					for t != nil {
						p := player_mgr.GetPlayerById(t.player_id)
						if p != nil {
							log.Trace("############### to offline player[%v]", t.player_id)
							this.remove(p.Id)
							p.OnLogout(false)
						}
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

		curr_num := int32(len(this.players))
		last_num := atomic.LoadInt32(&this.last_players_num)
		if curr_num != last_num {
			atomic.StoreInt32(&this.last_players_num, curr_num)
			log.Trace("{@} Server Players Num: %v", curr_num)
		}

		time.Sleep(time.Second * 1)
	}
}

func (this *ConnTimerWheel) GetCurrPlayerNum() int32 {
	return atomic.LoadInt32(&this.last_players_num)
}

var conn_timer_wheel ConnTimerWheel
