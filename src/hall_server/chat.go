package main

import (
	"ih_server/libs/log"
	"ih_server/libs/utils"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"ih_server/src/table_config"
	"net/http"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
)

const (
	CHAT_CHANNEL_NONE    = iota
	CHAT_CHANNEL_WORLD   = 1 // 世界
	CHAT_CHANNEL_GUILD   = 2 // 公会
	CHAT_CHANNEL_RECRUIT = 3 // 招募
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

type PlayerChatData struct {
	curr_msg       *ChatItem
	curr_send_time int32
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

var world_chat_mgr ChatMgr
var recruit_chat_mgr ChatMgr

func get_chat_config_data(channel int32) *table_config.ChatConfig {
	if channel == CHAT_CHANNEL_WORLD {
		return &global_config.WorldChatData
	} else if channel == CHAT_CHANNEL_GUILD {
		return &global_config.GuildChatData
	} else if channel == CHAT_CHANNEL_RECRUIT {
		return &global_config.RecruitChatData
	}
	return nil
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

	var msg *ChatItem
	if this.channel == CHAT_CHANNEL_WORLD {
		msg = player.world_chat_data.curr_msg
	} else if this.channel == CHAT_CHANNEL_GUILD {
		msg = player.guild_chat_data.curr_msg
	} else if this.channel == CHAT_CHANNEL_RECRUIT {
		msg = player.recruit_chat_data.curr_msg
	} else {
		return nil
	}

	if msg == nil {
		msg = this.chat_msg_head
	} else {
		var curr_send_time int32
		if this.channel == CHAT_CHANNEL_WORLD {
			curr_send_time = player.world_chat_data.curr_send_time
		} else if this.channel == CHAT_CHANNEL_GUILD {
			curr_send_time = player.guild_chat_data.curr_send_time
		} else {
			curr_send_time = player.recruit_chat_data.curr_send_time
		}

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

		if this.channel == CHAT_CHANNEL_WORLD {
			player.world_chat_data.curr_msg = msg
			player.world_chat_data.curr_send_time = msg.send_time
		} else if this.channel == CHAT_CHANNEL_GUILD {
			player.guild_chat_data.curr_msg = msg
			player.guild_chat_data.curr_send_time = msg.send_time
		} else {
			player.recruit_chat_data.curr_msg = msg
			player.recruit_chat_data.curr_send_time = msg.send_time
		}
		msg = msg.next
	}

	return
}

func (this *Player) chat(channel int32, content []byte) int32 {
	chat_mgr := this.get_chat_mgr(channel)
	if chat_mgr == nil {
		log.Error("Player[%v] get chat mgr by channel %v failed", this.Id, channel)
		return int32(msg_client_message.E_ERR_CHAT_CHANNEL_CANT_GET)
	}

	now_time := int32(time.Now().Unix())
	cooldown_seconds := get_chat_send_msg_cooldown(channel)
	max_bytes := get_chat_msg_max_bytes(channel)

	last_chat_time, _ := this.db.Chats.GetLastChatTime(channel)
	if now_time-last_chat_time < cooldown_seconds {
		log.Error("Player[%v] channel[%v] chat is cooling down !", channel, this.Id)
		return int32(msg_client_message.E_ERR_CHAT_SEND_MSG_COOLING_DOWN)
	}
	if int32(len(content)) > max_bytes {
		log.Error("Player[%v] channel[%v] chat content length is too long !", channel, this.Id)
		return int32(msg_client_message.E_ERR_CHAT_SEND_MSG_BYTES_TOO_LONG)
	}

	var lvl, extra_value int32
	var name string
	if channel == CHAT_CHANNEL_RECRUIT {
		guild := guild_manager._get_guild(this.Id, false)
		if guild != nil {
			lvl = guild.GetLevel()
			name = guild.GetName()
		}
		extra_value = this.db.Guild.GetId()
	} else {
		lvl = this.db.Info.GetLvl()
		name = this.db.GetName()
	}
	if !chat_mgr.push_chat_msg(content, extra_value, this.Id, lvl, name, this.db.Info.GetHead()) {
		return int32(msg_client_message.E_ERR_CHAT_CANT_SEND_WITH_NO_FREE)
	}

	if !this.db.Chats.HasIndex(channel) {
		this.db.Chats.Add(&dbPlayerChatData{
			Channel:      channel,
			LastChatTime: now_time,
		})
	} else {
		this.db.Chats.SetLastChatTime(channel, now_time)
	}

	response := &msg_client_message.S2CChatResponse{
		Channel: channel,
		Content: content,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_CHAT_RESPONSE), response)
	log.Debug("Player[%v] chat content[%v] in channel[%v]", this.Id, content, channel)

	return 1
}

func (this *Player) get_chat_mgr(channel int32) *ChatMgr {
	var chat_mgr *ChatMgr
	if channel == CHAT_CHANNEL_WORLD {
		chat_mgr = &world_chat_mgr
	} else if channel == CHAT_CHANNEL_GUILD {
		guild_id := this.db.Guild.GetId()
		chat_mgr = guild_manager.GetChatMgr(guild_id)
	} else if channel == CHAT_CHANNEL_RECRUIT {
		chat_mgr = &recruit_chat_mgr
	}
	return chat_mgr
}

func (this *Player) pull_chat(channel int32) int32 {
	chat_mgr := this.get_chat_mgr(channel)
	if chat_mgr == nil {
		log.Error("Player[%v] get chat mgr by channel %v failed", this.Id, channel)
		return int32(msg_client_message.E_ERR_CHAT_CHANNEL_CANT_GET)
	}
	pull_msg_cooldown := get_chat_pull_msg_cooldown(channel)
	if pull_msg_cooldown < 0 {
		log.Error("Player[%v] pull chat with unknown channel %v", this.Id, channel)
		return int32(msg_client_message.E_ERR_CHAT_CHANNEL_CANT_GET)
	}

	now_time := int32(time.Now().Unix())
	pull_time, _ := this.db.Chats.GetLastPullTime(channel)
	if now_time-pull_time < pull_msg_cooldown {
		log.Error("Player[%v] pull channel[%v] chat msg is cooling down", this.Id, channel)
		return int32(msg_client_message.E_ERR_CHAT_PULL_COOLING_DOWN)
	}

	msgs := chat_mgr.pull_chat(this)
	if !this.db.Chats.HasIndex(channel) {
		this.db.Chats.Add(&dbPlayerChatData{
			Channel:      channel,
			LastPullTime: now_time,
		})
	} else {
		this.db.Chats.SetLastPullTime(channel, now_time)
	}

	response := &msg_client_message.S2CChatMsgPullResponse{
		Channel: channel,
		Items:   msgs,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_CHAT_MSG_PULL_RESPONSE), response)

	log.Debug("Player[%v] pulled chat channel %v msgs %v", this.Id, channel, response)

	return 1
}

func (this *Player) has_new_chat_msg(channel int32) bool {
	chat_mgr := this.get_chat_mgr(channel)
	if chat_mgr == nil {
		//log.Error("Player[%v] get chat mgr by channel %v failed", this.Id, channel)
		return false
	}

	return chat_mgr.has_new_msg(this)
}

func C2SChatHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SChatRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}

	return p.chat(req.GetChannel(), req.GetContent())
}

func C2SChatPullMsgHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SChatMsgPullRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}
	return p.pull_chat(req.GetChannel())
}
