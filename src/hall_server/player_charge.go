package main

import (
	"ih_server/libs/log"
	"ih_server/libs/utils"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"ih_server/src/table_config"
	_ "math"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
)

type ChargeMonthCardManager struct {
	players_map     map[int32]*Player
	player_ids_chan chan int32
	to_end          int32
}

var charge_month_card_manager ChargeMonthCardManager

func (this *ChargeMonthCardManager) Init() {
	this.players_map = make(map[int32]*Player)
	this.player_ids_chan = make(chan int32, 10000)
}

func (this *ChargeMonthCardManager) InsertPlayer(player_id int32) {
	this.player_ids_chan <- player_id
}

func (this *ChargeMonthCardManager) _process_send_mails() {
	month_cards := pay_table_mgr.GetMonthCards()
	if month_cards == nil {
		return
	}

	now_time := time.Now()
	var to_delete_players map[int32]int32
	for pid, p := range this.players_map {
		d := false
		if p == nil {
			d = true
			continue
		}
		if !p.charge_month_card_award(month_cards, now_time) {
			d = true
			continue
		}
		if d {
			if to_delete_players == nil {
				to_delete_players = make(map[int32]int32)
			}
			to_delete_players[pid] = 1
		}
	}

	if to_delete_players != nil {
		for pid, _ := range to_delete_players {
			delete(this.players_map, pid)
		}
	}
}

func (this *ChargeMonthCardManager) Run() {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	for {
		if atomic.LoadInt32(&this.to_end) > 0 {
			break
		}

		var is_break bool
		for !is_break {
			select {
			case player_id, o := <-this.player_ids_chan:
				{
					if !o {
						log.Error("conn timer wheel op chan receive invalid !!!!!")
						return
					}
					if this.players_map[player_id] == nil {
						this.players_map[player_id] = player_mgr.GetPlayerById(player_id)
					}
				}
			default:
				{
					is_break = true
				}
			}
		}

		this._process_send_mails()

		time.Sleep(time.Minute)
	}
}

func (this *ChargeMonthCardManager) End() {
	atomic.StoreInt32(&this.to_end, 1)
}

func (this *Player) charge_month_card_award(month_cards []*table_config.XmlPayItem, now_time time.Time) bool {
	if !this.charge_has_month_card() {
		return false
	}
	for _, m := range month_cards {
		last_award_time, o := this.db.Pays.GetLastAwardTime(m.BundleId)
		if !o {
			continue
		}
		pay_item := pay_table_mgr.GetByBundle(m.BundleId)
		if pay_item == nil {
			continue
		}
		if utils.GetRemainSeconds2NextDayTime(last_award_time, global_config.MonthCardSendRewardTime) <= 0 {
			SendMail2(nil, this.Id, MAIL_TYPE_SYSTEM, "Month Card Award", "Month Card Award", []int32{ITEM_RESOURCE_ID_DIAMOND, pay_item.MonthCardReward})
			this.db.Pays.IncbySendMailNum(m.BundleId, 1)
			this.db.Pays.SetLastAwardTime(m.BundleId, int32(now_time.Unix()))
		}
	}
	return true
}

func (this *Player) charge_has_month_card() bool {
	arr := pay_table_mgr.GetMonthCards()
	if arr == nil {
		return false
	}

	for i := 0; i < len(arr); i++ {
		bundle_id := arr[i].BundleId
		send_num, o := this.db.Pays.GetSendMailNum(bundle_id)
		payed_time, _ := this.db.Pays.GetLastPayedTime(bundle_id)
		if o && payed_time > 0 && send_num < 30 {
			return true
		}
	}

	return false
}

func (this *Player) charge_data() int32 {
	var datas []*msg_client_message.MonthCardData
	all_index := this.db.Pays.GetAllIndex()
	for _, idx := range all_index {
		pay_item := pay_table_mgr.GetByBundle(idx)
		if pay_item == nil {
			continue
		}
		if pay_item.PayType == table_config.PAY_TYPE_MONTH_CARD {
			payed_time, _ := this.db.Pays.GetLastPayedTime(idx)
			send_mail_num, _ := this.db.Pays.GetSendMailNum(idx)
			datas = append(datas, &msg_client_message.MonthCardData{
				BundleId:    idx,
				EndTime:     payed_time + 30*24*3600,
				SendMailNum: send_mail_num,
			})
		}
	}

	response := &msg_client_message.S2CChargeDataResponse{
		FirstChargeState: this.db.PayCommon.GetFirstPayState(),
		Datas:            datas,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_CHARGE_DATA_RESPONSE), response)

	log.Debug("Player[%v] charge data %v", this.Id, response)

	return 1
}

func (this *Player) charge(id int32) int32 {
	pay_item := pay_table_mgr.Get(id)
	if pay_item == nil {
		return -1
	}
	return this.charge_with_bundle_id(pay_item.BundleId)
}

func (this *Player) charge_with_bundle_id(bundle_id string) int32 {
	pay_item := pay_table_mgr.GetByBundle(bundle_id)
	if pay_item == nil {
		log.Error("pay %v table data not found", bundle_id)
		return int32(msg_client_message.E_ERR_CHARGE_TABLE_DATA_NOT_FOUND)
	}

	if pay_item.PayType != table_config.PAY_TYPE_MONTH_CARD {
		if this.db.PayCommon.GetFirstPayState() == 0 {
			this.db.PayCommon.SetFirstPayState(1)
			// 首充通知
			notify := &msg_client_message.S2CChargeFirstRewardNotify{}
			this.Send(uint16(msg_client_message_id.MSGID_S2C_CHARGE_FIRST_REWARD_NOTIFY), notify)
		}
	}

	now_time := time.Now()
	var has bool
	has = this.db.Pays.HasIndex(bundle_id)
	if has {
		if pay_item.PayType == table_config.PAY_TYPE_MONTH_CARD {
			payed_time, _ := this.db.Pays.GetLastPayedTime(bundle_id)
			if int32(now_time.Unix())-payed_time < 30*24*3600 {
				log.Error("Player[%v] payed month card %v is using, not outdate", this.Id, bundle_id)
				return int32(msg_client_message.E_ERR_CHARGE_MONTH_CARD_ALREADY_PAYED)
			}
			this.add_diamond(pay_item.MonthCardReward) // 月卡奖励
		}
	} else {
		this.db.Pays.Add(&dbPlayerPayData{
			BundleId: bundle_id,
		})
		this.add_diamond(pay_item.GemRewardFirst) // 首次充值奖励
	}

	this.db.Pays.SetLastPayedTime(bundle_id, int32(now_time.Unix()))
	// 充值获得钻石
	this.add_diamond(pay_item.GemReward)

	response := &msg_client_message.S2CChargeResponse{
		BundleId: bundle_id,
		IsFirst:  !has,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_CHARGE_RESPONSE), response)

	log.Debug("Player[%v] charged bundle %v", this.Id, response)

	return 1
}

func (this *Player) charge_first_award() int32 {
	pay_state := this.db.PayCommon.GetFirstPayState()
	if pay_state == 0 {
		log.Error("Player[%v] not first charge, cant award", this.Id)
		return int32(msg_client_message.E_ERR_CHARGE_FIRST_NO_DONE)
	} else if pay_state == 2 {
		log.Error("Player[%v] first charge cant award repeated", this.Id)
		return int32(msg_client_message.E_ERR_CHARGE_FIRST_ALREADY_AWARD)
	}

	var rewards map[int32]int32
	first_charge_rewards := global_config.FirstChargeRewards
	if first_charge_rewards != nil {
		this.add_resources(first_charge_rewards)

		for i := 0; i < len(first_charge_rewards)/2; i++ {
			rid := first_charge_rewards[2*i]
			rn := first_charge_rewards[2*i+1]
			if rewards == nil {
				rewards = make(map[int32]int32)
			}
			rewards[rid] += rn
		}
	}

	response := &msg_client_message.S2CChargeFirstAwardResponse{
		Rewards: Map2ItemInfos(rewards),
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_CHARGE_FIRST_AWARD_RESPONSE), response)

	log.Debug("Player[%v] first charge award %v", this.Id, response)

	return 1
}

func C2SChargeDataHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SChargeDataRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%v)", err.Error())
		return -1
	}
	return p.charge_data()
}

func C2SChargeHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SChargeRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%v)", err.Error())
		return -1
	}
	return p.charge_with_bundle_id(req.GetBundleId())
}

func C2SChargeFirstAwardHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SChargeFirstAwardRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%v)", err.Error())
		return -1
	}
	return p.charge_first_award()
}
