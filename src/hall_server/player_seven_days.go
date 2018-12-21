package main

import (
	"ih_server/libs/log"
	_ "ih_server/libs/utils"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	_ "ih_server/src/table_config"
	"time"

	"github.com/golang/protobuf/proto"
)

const (
	SEVEN_DAYS = 7
)

func _get_start_secs_for_seven_days(player_create_time int32) int32 {
	ct := time.Unix(int64(player_create_time), 0)
	sct := int32(ct.Unix()) - int32(ct.Hour()*3600+ct.Minute()*60+ct.Second())
	return sct
}

func (this *dbPlayerSevenDaysColumn) has_reward(player_create_time int32) bool {
	now_time := time.Now()
	start_secs := _get_start_secs_for_seven_days(player_create_time)
	diff_secs := int32(now_time.Unix()) - start_secs
	if diff_secs >= SEVEN_DAYS*24*3600 {
		return false
	}

	this.m_row.m_lock.UnSafeRLock("dbPlayerSevenDaysColumn.has_reward")
	defer this.m_row.m_lock.UnSafeRUnlock()

	if this.m_data.Awards == nil || len(this.m_data.Awards) == 0 {
		return true
	}

	if this.m_data.Awards[diff_secs/(24*3600)] > 0 {
		return false
	}

	return true
}

func (this *Player) check_seven_days(get_remain_seconds bool) (days, state, remain_seconds int32) {
	create_time := this.db.Info.GetCreateUnix()
	now_time := int32(time.Now().Unix())
	start_secs := _get_start_secs_for_seven_days(create_time)
	diff_secs := now_time - start_secs
	if diff_secs >= SEVEN_DAYS*24*3600 {
		return -1, 0, 0
	}
	diff_days := diff_secs / (24 * 3600)
	award_states := this.db.SevenDays.GetAwards()
	if award_states == nil || len(award_states) == 0 {
		award_states = make([]int32, SEVEN_DAYS)
		this.db.SevenDays.SetAwards(award_states)
	}
	days = diff_days + 1
	this.db.SevenDays.SetDays(days)
	state = award_states[diff_days]

	if get_remain_seconds {
		remain_seconds = start_secs + SEVEN_DAYS*24*3600 - now_time
		if remain_seconds < 0 {
			remain_seconds = 0
		}
	}

	return
}

func (this *Player) seven_days_data() int32 {
	days, _, remain_seconds := this.check_seven_days(true)
	award_states := this.db.SevenDays.GetAwards()
	response := &msg_client_message.S2CSevenDaysDataResponse{
		AwardStates: func() []int32 {
			if days < 0 {
				return make([]int32, 0)
			} else {
				return award_states[:days]
			}
		}(),
		StartTime:     this.db.Info.GetCreateUnix(),
		RemainSeconds: remain_seconds,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_SEVENDAYS_DATA_RESPONSE), response)
	log.Debug("Player[%v] seven days data %v", this.Id, response)
	return 1
}

func (this *Player) seven_days_award() int32 {
	days, state, _ := this.check_seven_days(false)
	if days < 0 {
		log.Error("Player[%v] seven days finished", this.Id)
		return int32(msg_client_message.E_ERR_SEVEN_DAYS_FINISHED)
	}
	if state > 0 {
		log.Error("Player[%v] seven day %v already awarded", this.Id, days)
		return int32(msg_client_message.E_ERR_SEVEN_DAYS_AWARDED)
	}

	awards := this.db.SevenDays.GetAwards()
	awards[days-1] = 1
	this.db.SevenDays.SetAwards(awards)
	this.db.SevenDays.SetDays(days - 1)

	item := seven_days_table_mgr.Get(days)
	if item == nil {
		log.Error("Player[%v] seven day %v table data not found", this.Id, days)
		return -1
	}
	rewards := make(map[int32]int32)
	if item.Reward != nil {
		this.add_resources(item.Reward)
		for i := 0; i < len(item.Reward)/2; i++ {
			rewards[item.Reward[2*i]] += item.Reward[2*i+1]
		}
	}

	response := &msg_client_message.S2CSevenDaysAwardResponse{
		Days:    days,
		Rewards: Map2ItemInfos(rewards),
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_SEVENDAYS_AWARD_RESPONSE), response)

	log.Debug("Player[%v] seven days awarded day %v", this.Id, response)
	return 1
}

func C2SSevenDaysDataHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SSevenDaysDataRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%v)", err.Error())
		return -1
	}
	return p.seven_days_data()
}

func C2SSevenDaysAwardHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SSevenDaysAwardRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%v)", err.Error())
		return -1
	}
	return p.seven_days_award()
}
