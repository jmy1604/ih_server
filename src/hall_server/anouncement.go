package main

import (
	"ih_server/libs/log"
	"ih_server/libs/utils"
	_ "ih_server/proto/gen_go/client_message"
	"sync"
	"time"
)

const (
	ANOUNCEMENT_TYPE_NONE                    = iota
	ANOUNCEMENT_TYPE_GET_FOSTER_CARD         = 1
	ANOUNCEMENT_TYPE_GET_BUILDING            = 2
	ANOUNCEMENT_TYPE_GET_FORMULA             = 3
	ANOUNCEMENT_TYPE_GET_SSR_CAT             = 4
	ANOUNCEMENT_TYPE_RANKING_LIST_FIRST_RANK = 5
	ANOUNCEMENT_TYPE_CAT_FULL_LEVEL          = 6
	ANOUNCEMENT_TYPE_TEXT                    = 7
)

type AnouncementItem struct {
	msg_type     int32
	player_id    int32
	player_name  string
	player_level int32
	send_time    int32
	param1       int32
	param2       int32
	param3       int32
	text         string
	prev         *AnouncementItem
	next         *AnouncementItem
}

type AnouncementItemFactory struct {
}

func (this *AnouncementItemFactory) New() interface{} {
	return &AnouncementItem{}
}

type PlayerAnouncementData struct {
	curr_msg       *AnouncementItem
	curr_send_time int32
}

type AnouncementMgr struct {
	msg_num       int32                   // 消息数
	chat_msg_head *AnouncementItem        // 最早的结点
	chat_msg_tail *AnouncementItem        // 最新的节点
	items_pool    *utils.SimpleItemPool   // 消息池
	items_factory *AnouncementItemFactory // 对象工厂
	locker        *sync.RWMutex           // 锁
}

var anouncement_mgr AnouncementMgr

func (this *AnouncementMgr) Init() {
	this.items_pool = &utils.SimpleItemPool{}
	this.items_factory = &AnouncementItemFactory{}
	this.items_pool.Init(global_config.AnouncementMaxNum, this.items_factory)
	this.locker = &sync.RWMutex{}
	this.chat_msg_head = nil
	this.chat_msg_tail = nil
}

// 删除超时的公告
func (this *AnouncementMgr) recycle_old() {
	now_time := int32(time.Now().Unix())
	msg := this.chat_msg_head
	for msg != nil {
		if now_time-msg.send_time >= global_config.AnouncementExistTime*60 {
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

func (this *AnouncementMgr) PushNew(msg_type int32, is_broadcast bool, player_id int32 /*player_name string, player_level int32, */, param1, param2, param3 int32, text string) bool {
	this.locker.Lock()
	defer this.locker.Unlock()

	this.recycle_old()

	if !this.items_pool.HasFree() {
		// 回收最早的节点
		if !this.items_pool.Recycle(this.chat_msg_head) {
			log.Error("###[AnouncementMgr]### Recycle failed")
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
		log.Error("###[AnouncementMgr]### No free item")
		return false
	}

	item := it.(*AnouncementItem)
	item.msg_type = msg_type
	item.player_id = player_id
	item.player_name, item.player_level, _ = GetPlayerBaseInfo(player_id)
	item.send_time = int32(time.Now().Unix())
	item.param1 = param1
	item.param2 = param2
	item.param3 = param3
	item.text = text

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

	if is_broadcast {
		player := player_mgr.GetPlayerById(player_id)
		if player != nil {
			player.rpc_anouncement(msg_type, param1, text)
		}
	}

	log.Debug("Pushed Anouncement all_anounce[%v] msg_type[%v] player_id[%v] param1[%v] param2[%v]", this.msg_num, msg_type, player_id /*player_name, player_level,*/, param1, param2)

	return true
}

/*
func (this *AnouncementMgr) GetSome(player *Player, num int32) (items []*msg_client_message.AnouncementItem) {
	this.locker.RLock()
	defer this.locker.RUnlock()

	if num <= 0 || this.msg_num <= 0 {
		return make([]*msg_client_message.AnouncementItem, 0)
	}

	if num > this.msg_num {
		num = this.msg_num
	}

	msg := player.anouncement_data.curr_msg
	if msg == nil {
		msg = this.chat_msg_head
	} else {
		if msg.send_time != player.anouncement_data.curr_send_time {
			msg = this.chat_msg_head
		} else {
			msg = msg.next
		}
	}

	if msg == nil {
		return
	}

	now_time := int32(time.Now().Unix())
	for n := int32(0); n < num; n++ {
		if msg == nil {
			break
		}

		if now_time-msg.send_time >= global_config_mgr.GetGlobalConfig().AnouncementExistTime*60 {
			msg = msg.next
			continue
		}

		item := &msg_client_message.AnouncementItem{
			MsgType:     proto.Int32(msg.msg_type),
			PlayerId:    proto.Int32(msg.player_id),
			PlayerName:  proto.String(msg.player_name),
			PlayerLevel: proto.Int32(msg.player_level),
			SendTime:    proto.Int32(msg.send_time),
		}
		if msg.msg_type == ANOUNCEMENT_TYPE_GET_FOSTER_CARD {
			item.FosterCardTableId = proto.Int32(msg.param1)
		} else if msg.msg_type == ANOUNCEMENT_TYPE_GET_BUILDING {
			item.BuildingTableId = proto.Int32(msg.param1)
		} else if msg.msg_type == ANOUNCEMENT_TYPE_GET_FORMULA {
			item.FormulaTableId = proto.Int32(msg.param1)
		} else if msg.msg_type == ANOUNCEMENT_TYPE_GET_SSR_CAT {
			item.SSRCatTableId = proto.Int32(msg.param1)
		} else if msg.msg_type == ANOUNCEMENT_TYPE_RANKING_LIST_FIRST_RANK {
			item.RankType = proto.Int32(msg.param1)
			item.StageId = proto.Int32(msg.param2)
		} else if msg.msg_type == ANOUNCEMENT_TYPE_CAT_FULL_LEVEL {
			item.CatFullLevelTableId = proto.Int32(msg.param1)
		} else if msg.msg_type == ANOUNCEMENT_TYPE_TEXT {
			item.Content = proto.String(msg.text)
		} else {
			log.Error("Invalid anouncement msg type[%v]", msg.msg_type)
		}
		items = append(items, item)

		player.anouncement_data.curr_msg = msg
		player.anouncement_data.curr_send_time = msg.send_time
		msg = msg.next
	}

	return
}*/

func (this *Player) CheckAndAnouncement() int32 {
	/*now_time := int32(time.Now().Unix())
	if now_time-this.db.Anouncement.GetLastSendTime() < global_config_mgr.GetGlobalConfig().AnouncementSendCooldown {
		return -1
	}
	items := anouncement_mgr.GetSome(this, global_config_mgr.GetGlobalConfig().AnouncementSendMaxNum)
	if items != nil && len(items) > 0 {
		this.db.Anouncement.SetLastSendTime(now_time)
		notify := &msg_client_message.S2CAnouncementNotify{
			Items: items,
		}
		this.Send(notify)
	}*/
	return 1
}
