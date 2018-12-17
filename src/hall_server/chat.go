package main

import (
	"ih_server/libs/log"
	"ih_server/libs/utils"
	"sync"
	"time"

	"ih_server/proto/gen_go/client_message"
)

const MAX_CHAT_ONCE_GET int32 = 50      // 默认每次拉取消息条数
const MAX_CHAT_MSG_NUM int32 = 150      // 默认消息总数
const PULL_MSG_COOLDOWN int32 = 30      // 默认拉取消息冷却时间
const PULL_MAX_MSG_NUM int32 = 10       // 默认拉取最大消息条数
const CHAT_MSG_MAX_BYTES int32 = 200    // 默认消息最大字节数
const CHAT_MSG_EXIST_MINUTES int32 = 60 // 默认消息存在最长分钟数
const CHAT_SEND_MSG_COOLDOWN int32 = 5  // 默认发送消息间隔

type ChatItem struct {
	send_player_id    int32
	send_player_name  string
	send_player_level int32
	send_player_head  int32
	content           []byte
	extra_value       int32
	send_time         int32
	prev              *ChatItem
	next              *ChatItem
}

type ChatItemFactory struct {
}

func (this *ChatItemFactory) New() interface{} {
	return &ChatItem{}
}

type ChatMgr struct {
	channel       int32                 // 频道
	msg_num       int32                 // 消息数
	chat_msg_head *ChatItem             // 最早的结点
	chat_msg_tail *ChatItem             // 最新的节点
	items_pool    *utils.SimpleItemPool // 消息池
	items_factory *ChatItemFactory      // 对象工厂
	locker        *sync.RWMutex         // 锁
}

func get_chat_max_msg_num(channel int32) int32 {
	var max_num int32
	chat_config := get_chat_config_data(channel)
	if chat_config == nil {
		max_num = MAX_CHAT_MSG_NUM
	} else {
		max_num = chat_config.MaxMsgNum
	}
	return max_num
}

func get_chat_pull_msg_cooldown(channel int32) int32 {
	var pull_msg_cooldown int32
	chat_config := get_chat_config_data(channel)
	if chat_config == nil {
		pull_msg_cooldown = PULL_MSG_COOLDOWN
	} else {
		pull_msg_cooldown = chat_config.PullMsgCooldown
	}
	return pull_msg_cooldown
}

func get_chat_pull_max_msg_num(channel int32) int32 {
	var pull_max_msg_num int32
	chat_config := get_chat_config_data(channel)
	if chat_config == nil {
		pull_max_msg_num = PULL_MAX_MSG_NUM
	} else {
		pull_max_msg_num = chat_config.PullMaxMsgNum
	}
	return pull_max_msg_num
}

func get_chat_msg_max_bytes(channel int32) int32 {
	var msg_bytes int32
	chat_config := get_chat_config_data(channel)
	if chat_config == nil {
		msg_bytes = CHAT_MSG_MAX_BYTES
	} else {
		msg_bytes = chat_config.MsgMaxBytes
	}
	return msg_bytes
}

func get_chat_msg_exist_minutes(channel int32) int32 {
	var exist_minutes int32
	chat_config := get_chat_config_data(channel)
	if chat_config == nil {
		exist_minutes = CHAT_MSG_EXIST_MINUTES
	} else {
		exist_minutes = chat_config.MsgExistTime
	}
	return exist_minutes
}

func get_chat_send_msg_cooldown(channel int32) int32 {
	var send_msg_cooldown int32
	chat_config := get_chat_config_data(channel)
	if chat_config == nil {
		send_msg_cooldown = CHAT_SEND_MSG_COOLDOWN
	} else {
		send_msg_cooldown = chat_config.SendMsgCooldown
	}
	return send_msg_cooldown
}

func (this *ChatMgr) Init(channel int32) {
	this.channel = channel
	this.items_pool = &utils.SimpleItemPool{}
	this.items_factory = &ChatItemFactory{}
	this.items_pool.Init(get_chat_max_msg_num(channel), this.items_factory)
	this.locker = &sync.RWMutex{}
	this.chat_msg_head = nil
	this.chat_msg_tail = nil
}

func (this *ChatMgr) recycle_old() {
	exist_time := get_chat_msg_exist_minutes(this.channel)
	now_time := int32(time.Now().Unix())
	msg := this.chat_msg_head
	for msg != nil {
		if now_time-msg.send_time >= exist_time*60 {
			if msg == this.chat_msg_head {
				this.chat_msg_head = msg.next
			}
			if msg == this.chat_msg_tail {
				this.chat_msg_tail = nil
			}
			this.items_pool.Recycle(msg)
			if msg.prev != nil {
				msg.prev.next = msg.next
			}
			if msg.next != nil {
				msg.next.prev = msg.prev
			}
		}
		msg = msg.next
	}
}

func (this *ChatMgr) push_chat_msg(content []byte, extra_value int32, player_id int32, player_level int32, player_name string, player_head int32) bool {
	this.locker.Lock()
	defer this.locker.Unlock()

	this.recycle_old()

	if !this.items_pool.HasFree() {
		// 回收最早的节点
		if !this.items_pool.Recycle(this.chat_msg_head) {
			log.Error("###[ChatMgr]### Recycle failed")
			return false
		}
		n := this.chat_msg_head.next
		this.chat_msg_head = n
		if n != nil {
			n.prev = nil
		}
	}

	it := this.items_pool.GetFree()
	if it == nil {
		log.Error("###[ChatMgr]### No free item")
		return false
	}

	item := it.(*ChatItem)
	item.content = content
	item.send_player_id = player_id
	item.send_player_name = player_name
	item.send_player_head = player_head
	item.send_player_level = player_level
	item.send_time = int32(time.Now().Unix())
	item.extra_value = extra_value

	item.prev = this.chat_msg_tail
	item.next = nil
	if this.chat_msg_head == nil {
		this.chat_msg_head = item
	}
	if this.chat_msg_tail != nil {
		this.chat_msg_tail.next = item
	}
	this.chat_msg_tail = item
	this.msg_num += 1

	return true
}

func (this *ChatMgr) get_curr_msg(player *Player, is_lock bool) *ChatItem {
	if is_lock {
		this.locker.RLock()
		defer this.locker.RUnlock()
	}

	chat_data := player.get_chat_data(this.channel)
	if chat_data == nil {
		log.Error("Player[%v] cant get chat data with channel %v", player.Id, this.channel)
		return nil
	}

	var msg *ChatItem = chat_data.curr_msg

	if msg == nil {
		msg = this.chat_msg_head
	} else {
		var curr_send_time int32 = chat_data.curr_send_time

		if msg.send_time != curr_send_time {
			msg = this.chat_msg_head
		} else {
			msg = msg.next
		}
	}

	return msg
}

func (this *ChatMgr) has_new_msg(player *Player) bool {
	this.locker.RLock()
	defer this.locker.RUnlock()

	msg := this.get_curr_msg(player, false)
	now_time := int32(time.Now().Unix())
	exist_minutes := get_chat_msg_exist_minutes(this.channel)
	for {
		if msg == nil {
			break
		}

		if now_time-msg.send_time < exist_minutes*60 {
			return true
		}

		msg = msg.next
	}

	return false
}

func (this *ChatMgr) pull_chat(player *Player) (chat_items []*msg_client_message.ChatItem) {
	this.locker.RLock()
	defer this.locker.RUnlock()

	chat_data := player.get_chat_data(this.channel)
	if chat_data == nil {
		log.Error("Player[%v] cant get chat data with channel %v", player.Id, this.channel)
		return
	}

	if this.msg_num <= 0 {
		chat_items = make([]*msg_client_message.ChatItem, 0)
		return
	}
	msg_num := MAX_CHAT_ONCE_GET
	if msg_num > this.msg_num {
		msg_num = this.msg_num
	}

	msg := this.get_curr_msg(player, false)
	now_time := int32(time.Now().Unix())
	exist_minutes := get_chat_msg_exist_minutes(this.channel)
	for n := int32(0); n < msg_num; n++ {
		if msg == nil {
			break
		}

		if now_time-msg.send_time >= exist_minutes*60 {
			msg = msg.next
			continue
		}
		item := &msg_client_message.ChatItem{
			Content:     msg.content,
			PlayerId:    msg.send_player_id,
			PlayerName:  msg.send_player_name,
			PlayerLevel: msg.send_player_level,
			PlayerHead:  msg.send_player_head,
			SendTime:    msg.send_time,
			ExtraValue:  msg.extra_value,
		}
		chat_items = append(chat_items, item)

		chat_data.curr_msg = msg
		chat_data.curr_send_time = msg.send_time
		msg = msg.next
	}

	return
}
