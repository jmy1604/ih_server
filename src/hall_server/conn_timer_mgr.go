package main

import (
	"ih_server/libs/log"
	"sync"
	"time"
)

type ConnTimer struct {
	player_id int32
	next      *ConnTimer
	prev      *ConnTimer
}

type ConnTimerList struct {
	head *ConnTimer
	tail *ConnTimer
}

func (this *ConnTimerList) add(player_id int32) *ConnTimer {
	node := &ConnTimer{
		player_id: player_id,
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

func (this *ConnTimerList) remove(timer *ConnTimer) {
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

type ConnTimerPlayer struct {
	player_id  int32
	timer_list *ConnTimerList
	timer      *ConnTimer
}

type ConnTimerMgr struct {
	timer_lists      []*ConnTimerList
	curr_timer_index int32
	last_check_time  int32
	players          map[int32]*ConnTimerPlayer
	locker           *sync.Mutex
}

func (this *ConnTimerMgr) Init() {
	this.timer_lists = make([]*ConnTimerList, global_config.HeartbeatInterval*3)
	this.curr_timer_index = -1
	this.players = make(map[int32]*ConnTimerPlayer)
	this.locker = &sync.Mutex{}
}

func (this *ConnTimerMgr) Insert(player_id int32) bool {
	this.locker.Lock()
	defer this.locker.Unlock()

	p := this.players[player_id]
	if p != nil {
		this.remove(p)
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
	log.Debug("Player[%v] conn insert in index[%v] list", player_id, insert_list_index)
	return true
}

func (this *ConnTimerMgr) remove(p *ConnTimerPlayer) bool {
	p.timer_list.remove(p.timer)
	delete(this.players, p.player_id)
	return true
}

func (this *ConnTimerMgr) Remove(player_id int32) bool {
	this.locker.Lock()
	defer this.locker.Unlock()
	p := this.players[player_id]
	if p == nil {
		return false
	}
	return this.remove(p)
}

func (this *ConnTimerMgr) Run() {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	for {
		this.locker.Lock()

		now_time := int32(time.Now().Unix())
		if this.last_check_time == 0 {
			this.last_check_time = now_time
		}

		var players []*Player
		lists_len := int32(len(this.timer_lists))
		diff_secs := now_time - this.last_check_time
		if diff_secs > 0 {
			var idx int32
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
							players = append(players, p)
							log.Debug("############### to offline player[%v]", t.player_id)
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

		this.locker.Unlock()

		if players != nil {
			for i := 0; i < len(players); i++ {
				if players[i] != nil {
					players[i].OnLogout(true)
				}
			}
		}

		time.Sleep(time.Second * 1)
	}
}

var conn_timer_mgr ConnTimerMgr
