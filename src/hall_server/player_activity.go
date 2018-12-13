package main

import (
	"ih_server/libs/log"
	"ih_server/src/table_config"
	"net/http"
	"sync"
	"time"

	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"

	"github.com/golang/protobuf/proto"
)

const (
	ACTIVITY_TYPE_NONE                = iota
	ACTIVITY_TYPE_WEEK_FAVOR          = 1  // 周特惠
	ACTIVITY_TYPE_MONTH_FAVOR         = 2  // 月度特惠
	ACTIVITY_TYPE_FESTIVAL_FAVOR      = 3  // 节日特惠
	ACTIVITY_TYPE_CUMULATIVE_RECHARGE = 4  // 累计充值
	ACTIVITY_TYPE_GET_HERO            = 5  // 获得英雄
	ACTIVITY_TYPE_COST_DIAMOND        = 6  // 累计消费钻石
	ACTIVITY_TYPE_EXPLORE             = 7  // 探索任务
	ACTIVITY_TYPE_DRAW_CARD           = 8  // 抽卡任务
	ACTIVITY_TYPE_ARENA               = 9  // 竞技场任务
	ACTIVITY_TYPE_DROP                = 10 // 掉落道具
	ACTIVITY_TYPE_EXCHANGE            = 11 // 限时兑换
)

const (
	ACTIVITY_EVENT_CHARGE        = 301 // 充值购买
	ACTIVITY_EVENT_EXCHAGE_ITEM  = 302 // 兑换物品
	ACTIVITY_EVENT_CHARGE_RETURN = 303 // 充值返利
	ACTIVITY_EVENT_GET_HERO      = 304 // 获得英雄
	ACTIVITY_EVENT_DIAMOND_COST  = 305 // 钻石消耗
	ACTIVITY_EVENT_EXPLORE       = 306 // 探索任务
	ACTIVITY_EVENT_DRAW_SCORE    = 307 // 高级抽卡
	ACTIVITY_EVENT_ARENA_SCORE   = 308 // 竞技场积分
)

type ActivityManager struct {
	data_map map[int32]*table_config.XmlActivityItem
	locker   sync.RWMutex
}

var activity_mgr ActivityManager

func (this *ActivityManager) Run() {
	if activity_table_mgr.Array == nil {
		return
	}

	if this.data_map == nil {
		this.data_map = make(map[int32]*table_config.XmlActivityItem)
	}

	for {
		now_time := time.Now()
		for _, d := range activity_table_mgr.Array {
			if d.StartTime <= int32(now_time.Unix()) && d.EndTime >= int32(now_time.Unix()) {
				this.locker.RLock()
				if this.data_map[d.Id] != nil {
					this.locker.RLock()
					continue
				}
				this.locker.RUnlock()

				this.locker.Lock()
				if this.data_map[d.Id] == nil {
					this.data_map[d.Id] = d
				}
				this.locker.Unlock()
			} else if d.EndTime < int32(now_time.Unix()) {
				this.locker.RLock()
				if this.data_map[d.Id] == nil {
					this.locker.RUnlock()
					continue
				}
				this.locker.RUnlock()

				this.locker.Lock()
				if this.data_map[d.Id] != nil {
					delete(this.data_map, d.Id)
					row := dbc.ActivityToDeletes.AddRow(d.Id)
					if row != nil {
						row.SetStartTime(d.StartTime)
						row.SetEndTime(d.StartTime)
					}
				}
				this.locker.Unlock()
			}
		}
		time.Sleep(time.Second)
	}
}

func (this *ActivityManager) GetData() (data []*msg_client_message.ActivityData) {
	this.locker.RLock()
	defer this.locker.RUnlock()

	for _, v := range this.data_map {
		remain_seconds := GetRemainSeconds(v.StartTime, v.EndTime-v.StartTime)
		if remain_seconds > 0 {
			data = append(data, &msg_client_message.ActivityData{
				Id:            v.Id,
				RemainSeconds: remain_seconds,
			})
		}
	}

	return
}

func (this *ActivityManager) GetActivitysByEvent(event_type int32) (items []*table_config.XmlActivityItem) {
	this.locker.RLock()
	defer this.locker.RUnlock()

	for _, v := range this.data_map {
		if v.EventId != event_type {
			continue
		}
		if GetRemainSeconds(v.StartTime, v.EndTime-v.StartTime) > 0 {
			items = append(items, v)
		}
	}

	return
}

func (this *ActivityManager) IsDoing(id int32) bool {
	this.locker.RLock()
	defer this.locker.RUnlock()

	if this.data_map[id] == nil {
		return false
	}

	d := activity_table_mgr.Get(id)
	if d == nil {
		return false
	}

	if GetRemainSeconds(d.StartTime, d.EndTime-d.StartTime) <= 0 {
		return false
	}

	return true
}

func (this *Player) activity_data() int32 {
	ids := this.db.Activitys.GetAllIndex()
	datas := activity_mgr.GetData()
	if ids != nil {
		for _, id := range ids {
			if activity_table_mgr.Get(id) == nil {
				this.db.Activitys.Remove(id)
				continue
			}
			if !activity_mgr.IsDoing(id) {
				this.db.Activitys.Remove(id)
				continue
			}
			var found bool
			if ids != nil {
				for _, d := range datas {
					if d.GetId() == id {
						found = true
						break
					}
				}
			}
			if !found {
				datas = append(datas, &msg_client_message.ActivityData{Id: id})
			}
		}
	}

	if datas != nil {
		for _, d := range datas {
			id := int32(d.GetId())
			sub_ids, o := this.db.Activitys.GetSubIds(id)
			if dbc.ActivityToDeletes.GetRow(id) != nil {
				if this.db.Activitys.HasIndex(id) {
					this.db.Activitys.Remove(id)
					if !o {
						continue
					}
					for _, sid := range sub_ids {
						this.db.SubActivitys.Remove(sid)
					}
					continue
				}
			}
			if sub_ids != nil {
				for _, sid := range sub_ids {
					if this.db.SubActivitys.HasIndex(sid) {
						var sub_data msg_client_message.SubActivityData
						sub_data.SubId = sid
						sub_data.PurchasedNum, _ = this.db.SubActivitys.GetPurchasedNum(sid)
						sub_data.RechargeCumulative, _ = this.db.SubActivitys.GetRechargeCumulative(sid)
						sub_data.HeroNum, _ = this.db.SubActivitys.GetHeroNum(sid)
						sub_data.CostDiamond, _ = this.db.SubActivitys.GetCostDiamond(sid)
						sub_data.ExploreNum, _ = this.db.SubActivitys.GetExploreNum(sid)
						sub_data.DrawNum, _ = this.db.SubActivitys.GetDrawNum(sid)
						sub_data.ArenaScore, _ = this.db.SubActivitys.GetArenaScore(sid)
						d.SubDatas = append(d.SubDatas, &sub_data)
					}
				}
			}
		}
	}

	this.Send(uint16(msg_client_message_id.MSGID_S2C_ACTIVITY_DATA_RESPONSE), &msg_client_message.S2CActivityDataResponse{
		Data: datas,
	})
	log.Trace("Player[%v] activity data %v", this.Id, datas)
	return 1
}

func (this *Player) activity_check_and_add_sub(id, sub_id int32) bool {
	sub_ids, o := this.db.Activitys.GetSubIds(id)
	if !o {
		this.db.Activitys.Add(&dbPlayerActivityData{
			Id: id,
		})
	}

	var found bool
	for i := 0; i < len(sub_ids); i++ {
		if sub_id == sub_ids[i] {
			found = true
			break
		}
	}

	if !found {
		sub_ids = append(sub_ids, sub_id)
		this.db.Activitys.SetSubIds(id, sub_ids)
	}

	return found
}

func (this *Player) activity_has_sub(id, sub_id int32) bool {
	sub_ids, o := this.db.Activitys.GetSubIds(id)
	if !o {
		return false
	}
	for i := 0; i < len(sub_ids); i++ {
		if sub_id == sub_ids[i] {
			return true
		}
	}
	return false
}

func (this *Player) activity_get(id, event_type int32) (int32, *table_config.XmlActivityItem) {
	if !activity_mgr.IsDoing(id) {
		return -1, nil
	}

	a := activity_table_mgr.Get(id)
	if a == nil {
		return -1, nil
	}

	if a.SubActiveList == nil {
		return -1, nil
	}

	if event_type > 0 && a.EventId != event_type {
		log.Error("Player[%v] activity %v event[%v] no match", this.Id, id, a.EventId)
		return -1, nil
	}

	return 1, a
}

func (this *Player) activity_sub_get(id, sub_id, event_type int32) (int32, *table_config.XmlSubActivityItem) {
	res, a := this.activity_get(id, event_type)
	if res < 0 {
		return -1, nil
	}

	var d *table_config.XmlSubActivityItem
	for i := 0; i < len(a.SubActiveList); i++ {
		if sub_id == a.SubActiveList[i] {
			sd := sub_activity_table_mgr.Get(sub_id)
			if sd == nil {
				return -1, nil
			}
			d = sd
			break
		}
	}

	if d == nil {
		return -1, nil
	}

	return 1, d
}

func (this *Player) activity_get_one_charge(bundle_id string) (int32, *table_config.XmlSubActivityItem) {
	as := activity_mgr.GetActivitysByEvent(ACTIVITY_EVENT_CHARGE)
	if as == nil {
		return -1, nil
	}

	for _, a := range as {
		if a.SubActiveList == nil || len(a.SubActiveList) == 0 {
			continue
		}
		for _, sa_id := range a.SubActiveList {
			sa := sub_activity_table_mgr.Get(sa_id)
			if sa == nil || sa.EventCount <= 0 {
				continue
			}
			if sa.BundleID != bundle_id {
				continue
			}

			n, _ := this.db.SubActivitys.GetPurchasedNum(sa_id)
			if n < sa.EventCount {
				return a.Id, sa
			}
		}
	}

	return -1, nil
}

// 充值
func (this *Player) activity_update_one_charge(id int32, sa *table_config.XmlSubActivityItem) {
	num, _ := this.db.SubActivitys.GetPurchasedNum(sa.Id)
	if !this.db.SubActivitys.HasIndex(sa.Id) {
		this.db.SubActivitys.Add(&dbPlayerSubActivityData{
			SubId:        sa.Id,
			PurchasedNum: 1,
		})
		num = 1
	} else {
		num = this.db.SubActivitys.IncbyPurchasedNum(sa.Id, 1)
	}

	this.add_resources(sa.Reward)

	this.activity_check_and_add_sub(id, sa.Id)

	this.Send(uint16(msg_client_message_id.MSGID_S2C_ACTIVITY_DATA_NOTIFY), &msg_client_message.S2CActivityDataNotify{
		Id:    id,
		SubId: sa.Id,
		Value: num,
	})

	log.Trace("Player[%v] activity[%v,%v] update progress %v/%v", this.Id, id, sa.Id, num, sa.EventCount)
}

// 英雄活动更新
func (this *Player) activity_update_get_hero(a *table_config.XmlActivityItem, hero_star, hero_num, hero_camp, hero_type int32) {
	if a.SubActiveList == nil {
		return
	}

	for _, sa_id := range a.SubActiveList {
		sa := sub_activity_table_mgr.Get(sa_id)
		if sa == nil {
			continue
		}
		if sa.Param1 != hero_star {
			continue
		}
		if sa.Param3 > 0 && sa.Param3 != hero_camp {
			continue
		}
		if sa.Param4 > 0 && sa.Param4 != hero_type {
			continue
		}

		num, _ := this.db.SubActivitys.GetHeroNum(sa_id)
		if num >= sa.Param2 {
			continue
		}
		if !this.db.SubActivitys.HasIndex(sa_id) {
			this.db.SubActivitys.Add(&dbPlayerSubActivityData{
				SubId:   sa_id,
				HeroNum: hero_num,
			})
			num = hero_num
		} else {
			num = this.db.SubActivitys.IncbyHeroNum(sa_id, hero_num)
		}

		this.activity_check_and_add_sub(a.Id, sa_id)

		this.Send(uint16(msg_client_message_id.MSGID_S2C_ACTIVITY_DATA_NOTIFY), &msg_client_message.S2CActivityDataNotify{
			Id:    a.Id,
			SubId: sa_id,
			Value: num,
		})

		if num >= sa.Param2 && a.RewardMailId > 0 {
			RealSendMail(nil, this.Id, MAIL_TYPE_SYSTEM, a.RewardMailId, "", "", sa.Reward, sa.Param2)
		}

		log.Trace("Player[%v] activity[%v,%v] update progress %v/%v", this.Id, a.Id, sa_id, num, sa.Param2)
	}
}

// 消耗钻石活动更新
func (this *Player) activity_update_cost_diamond(a *table_config.XmlActivityItem, diamond int32) {
	if a.SubActiveList == nil {
		return
	}

	for _, sa_id := range a.SubActiveList {
		sa := sub_activity_table_mgr.Get(sa_id)
		if sa == nil {
			continue
		}

		cost_diamond, _ := this.db.SubActivitys.GetCostDiamond(sa_id)
		if cost_diamond >= sa.Param1 {
			continue
		}
		if !this.db.SubActivitys.HasIndex(sa_id) {
			this.db.SubActivitys.Add(&dbPlayerSubActivityData{
				SubId:       sa_id,
				CostDiamond: diamond,
			})
			cost_diamond = diamond
		} else {
			cost_diamond = this.db.SubActivitys.IncbyCostDiamond(sa_id, diamond)
		}

		this.activity_check_and_add_sub(a.Id, sa_id)

		this.Send(uint16(msg_client_message_id.MSGID_S2C_ACTIVITY_DATA_NOTIFY), &msg_client_message.S2CActivityDataNotify{
			Id:    a.Id,
			SubId: sa_id,
			Value: cost_diamond,
		})

		if cost_diamond >= sa.Param1 && a.RewardMailId > 0 {
			RealSendMail(nil, this.Id, MAIL_TYPE_SYSTEM, a.RewardMailId, "", "", sa.Reward, sa.Param1)
		}

		log.Trace("Player[%v] activity[%v,%v] update progress %v/%v", this.Id, a.Id, sa_id, cost_diamond, sa.Param1)
	}
}

// 探索任务活动更新
func (this *Player) activity_update_explore(a *table_config.XmlActivityItem, explore_star, explore_num int32) {
	if a.SubActiveList == nil {
		return
	}

	for _, sa_id := range a.SubActiveList {
		sa := sub_activity_table_mgr.Get(sa_id)
		if sa == nil {
			continue
		}

		if sa.Param1 != explore_star {
			continue
		}

		num, _ := this.db.SubActivitys.GetExploreNum(sa_id)
		if num >= sa.Param2 {
			continue
		}
		if !this.db.SubActivitys.HasIndex(sa_id) {
			this.db.SubActivitys.Add(&dbPlayerSubActivityData{
				SubId:      sa_id,
				ExploreNum: explore_num,
			})
			num = explore_num
		} else {
			num = this.db.SubActivitys.IncbyExploreNum(sa_id, explore_num)
		}

		this.activity_check_and_add_sub(a.Id, sa_id)

		this.Send(uint16(msg_client_message_id.MSGID_S2C_ACTIVITY_DATA_NOTIFY), &msg_client_message.S2CActivityDataNotify{
			Id:    a.Id,
			SubId: sa_id,
			Value: num,
		})

		if num >= sa.Param2 && a.RewardMailId > 0 {
			RealSendMail(nil, this.Id, MAIL_TYPE_SYSTEM, a.RewardMailId, "", "", sa.Reward, sa.Param2)
		}

		log.Trace("Player[%v] activity[%v,%v] update progress %v/%v", this.Id, a.Id, sa_id, num, sa.Param2)
	}
}

// 抽卡积分活动更新
func (this *Player) activity_update_draw_score(a *table_config.XmlActivityItem, draw_num int32) {
	if a.SubActiveList == nil {
		return
	}

	for _, sa_id := range a.SubActiveList {
		sa := sub_activity_table_mgr.Get(sa_id)
		if sa == nil {
			continue
		}
		num, _ := this.db.SubActivitys.GetDrawNum(sa_id)
		if num >= sa.Param1 {
			continue
		}
		if !this.db.SubActivitys.HasIndex(sa_id) {
			this.db.SubActivitys.Add(&dbPlayerSubActivityData{
				SubId:   sa_id,
				DrawNum: draw_num,
			})
			num = draw_num
		} else {
			num = this.db.SubActivitys.IncbyDrawNum(sa_id, draw_num)
		}

		this.activity_check_and_add_sub(a.Id, sa_id)

		this.Send(uint16(msg_client_message_id.MSGID_S2C_ACTIVITY_DATA_NOTIFY), &msg_client_message.S2CActivityDataNotify{
			Id:    a.Id,
			SubId: sa_id,
			Value: num,
		})

		if num >= sa.Param1 && a.RewardMailId > 0 {
			RealSendMail(nil, this.Id, MAIL_TYPE_SYSTEM, a.RewardMailId, "", "", sa.Reward, sa.Param1)
		}

		log.Trace("Player[%v] activity[%v,%v] update progress %v/%v", this.Id, a.Id, sa_id, num, sa.Param1)
	}
}

// 竞技场积分更新
func (this *Player) activity_update_arena_score(a *table_config.XmlActivityItem, add_score int32) {
	if a.SubActiveList == nil {
		return
	}

	for _, sa_id := range a.SubActiveList {
		sa := sub_activity_table_mgr.Get(sa_id)
		if sa == nil {
			continue
		}

		score, _ := this.db.SubActivitys.GetArenaScore(sa_id)
		if score >= sa.Param1 {
			continue
		}
		if !this.db.SubActivitys.HasIndex(sa_id) {
			this.db.SubActivitys.Add(&dbPlayerSubActivityData{
				SubId:      sa_id,
				ArenaScore: add_score,
			})
			score = add_score
		} else {
			score = this.db.SubActivitys.IncbyArenaScore(sa_id, add_score)
		}

		this.activity_check_and_add_sub(a.Id, sa_id)

		this.Send(uint16(msg_client_message_id.MSGID_S2C_ACTIVITY_DATA_NOTIFY), &msg_client_message.S2CActivityDataNotify{
			Id:    a.Id,
			SubId: sa_id,
			Value: score,
		})
		if score >= sa.Param1 && a.RewardMailId > 0 {
			RealSendMail(nil, this.Id, MAIL_TYPE_SYSTEM, a.RewardMailId, "", "", sa.Reward, sa.Param1)
		}

		log.Trace("Player[%v] activity[%v,%v] update progress %v/%v", this.Id, a.Id, sa_id, score, sa.Param1)
	}
}

// 活动更新
func (this *Player) activity_update(event_type, param1, param2, param3, param4 int32, param_str string) {
	var as []*table_config.XmlActivityItem
	if event_type == ACTIVITY_EVENT_GET_HERO || event_type == ACTIVITY_EVENT_DIAMOND_COST || event_type == ACTIVITY_EVENT_EXPLORE || event_type == ACTIVITY_EVENT_DRAW_SCORE || event_type == ACTIVITY_EVENT_ARENA_SCORE { // 获得英雄
		as = activity_mgr.GetActivitysByEvent(event_type)
	} else {
		return
	}

	if as == nil {
		return
	}

	for _, a := range as {
		if event_type == ACTIVITY_EVENT_GET_HERO {
			this.activity_update_get_hero(a, param1, param2, param3, param4)
		} else if event_type == ACTIVITY_EVENT_DIAMOND_COST {
			this.activity_update_cost_diamond(a, param1)
		} else if event_type == ACTIVITY_EVENT_EXPLORE {
			this.activity_update_explore(a, param1, param2)
		} else if event_type == ACTIVITY_EVENT_DRAW_SCORE {
			this.activity_update_draw_score(a, param1)
		} else if event_type == ACTIVITY_EVENT_ARENA_SCORE {
			this.activity_update_arena_score(a, param1)
		}
	}
}

// 活动兑换
func (this *Player) activity_exchange(id, sub_id int32) int32 {
	res, sa := this.activity_sub_get(id, sub_id, ACTIVITY_EVENT_EXCHAGE_ITEM)
	if res < 0 {
		return res
	}

	if sa == nil || sa.EventCount <= 0 {
		log.Error("Activity %v no sub activity %v", id, sub_id)
		return -1
	}

	purchased_num, o := this.db.SubActivitys.GetPurchasedNum(sub_id)
	if purchased_num >= sa.EventCount {
		log.Error("Player[%v] use activity %v sub %v purchased num out", this.Id, id, sub_id)
		return -1
	}

	if !o {
		purchased_num = 1
		this.db.SubActivitys.Add(&dbPlayerSubActivityData{
			SubId:        id,
			PurchasedNum: purchased_num,
		})
	} else {
		purchased_num = this.db.SubActivitys.IncbyPurchasedNum(sub_id, 1)
	}

	this.activity_check_and_add_sub(id, sub_id)

	response := &msg_client_message.S2CActivityExchangeResponse{
		Id:    id,
		SubId: sub_id,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_ACTIVITY_EXCHANGE_RESPONSE), response)

	return 1
}

func C2SActivityDataHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SActivityDataRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}
	return p.activity_data()
}

func C2SActivityExchangeHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SActivityExchangeRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}
	return p.activity_exchange(req.GetId(), req.GetSubId())
}