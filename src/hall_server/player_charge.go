package main

import (
	"ih_server/libs/log"
	"ih_server/libs/utils"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"ih_server/src/table_config"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
)

const (
	MONTH_CARD_DAYS_NUM = 30
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
		if p == nil || !p.charge_month_card_award(month_cards, now_time) {
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

func (this *Player) _charge_month_card_award(month_card *table_config.XmlPayItem, now_time time.Time) (send_num int32) {
	var bonus []int32 = []int32{ITEM_RESOURCE_ID_DIAMOND, month_card.MonthCardReward}
	// 大月卡
	if month_card.Id == 2 {
		vip_info := vip_table_mgr.Get(this.db.Info.GetVipLvl())
		if vip_info != nil && vip_info.MonthCardItemBonus != nil && len(vip_info.MonthCardItemBonus) > 0 {
			bonus = append(bonus, vip_info.MonthCardItemBonus...)
		}
	}
	RealSendMail(nil, this.Id, MAIL_TYPE_SYSTEM, 1105, "", "", bonus, 0)
	send_num = this.db.Pays.IncbySendMailNum(month_card.BundleId, 1)
	log.Trace("Player[%v] charge month card %v get reward, send_num %v", this.Id, month_card.BundleId, send_num)
	return
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
		send_num, _ := this.db.Pays.GetSendMailNum(m.BundleId)
		if send_num >= MONTH_CARD_DAYS_NUM {
			continue
		}
		num := utils.GetDaysNumToLastSaveTime(last_award_time, global_config.MonthCardSendRewardTime, now_time)
		for i := int32(0); i < num; i++ {
			send_num = this._charge_month_card_award(m, now_time)
			if send_num >= MONTH_CARD_DAYS_NUM {
				break
			}
		}
		if num > 0 {
			this.db.Pays.SetLastAwardTime(m.BundleId, int32(now_time.Unix()))
		}
	}
	return true
}

func (this *Player) charge_has_month_card() bool {
	// 获得月卡配置
	arr := pay_table_mgr.GetMonthCards()
	if arr == nil {
		return false
	}

	for i := 0; i < len(arr); i++ {
		bundle_id := arr[i].BundleId
		send_num, o := this.db.Pays.GetSendMailNum(bundle_id)
		if o && send_num < MONTH_CARD_DAYS_NUM {
			return true
		}
	}

	return false
}

func (this *Player) charge_data() int32 {
	var charged_ids []string
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
				EndTime:     payed_time + MONTH_CARD_DAYS_NUM*24*3600,
				SendMailNum: send_mail_num,
			})
		}
		charged_ids = append(charged_ids, idx)
	}

	response := &msg_client_message.S2CChargeDataResponse{
		FirstChargeState: this.db.PayCommon.GetFirstPayState(),
		Datas:            datas,
		ChargedBundleIds: charged_ids,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_CHARGE_DATA_RESPONSE), response)

	log.Trace("Player[%v] charge data %v", this.Id, response)

	return 1
}

func (this *Player) charge(channel, id int32) int32 {
	pay_item := pay_table_mgr.Get(id)
	if pay_item == nil {
		return -1
	}
	res, _ := this._charge_with_bundle_id(channel, pay_item.BundleId, nil, nil, 0)
	return res
}

func (this *Player) _charge_with_bundle_id(channel int32, bundle_id string, purchase_data []byte, extra_data []byte, index int32) (int32, bool) {
	pay_item := pay_table_mgr.GetByBundle(bundle_id)
	if pay_item == nil {
		log.Error("pay %v table data not found", bundle_id)
		return int32(msg_client_message.E_ERR_CHARGE_TABLE_DATA_NOT_FOUND), false
	}

	var aid int32
	var sa *table_config.XmlSubActivityItem
	if pay_item.ActivePay > 0 {
		aid, sa = this.activity_get_one_charge(bundle_id)
		if aid <= 0 || sa == nil {
			log.Error("Player[%v] no activity charge %v with channel %v", this.Id, bundle_id, channel)
			return int32(msg_client_message.E_ERR_CHARGE_NO_ACTIVITY_ON_THIS_TIME), false
		}
	}

	var has bool
	has = this.db.Pays.HasIndex(bundle_id)
	if has {
		if pay_item.PayType == table_config.PAY_TYPE_MONTH_CARD {
			mail_num, o := this.db.Pays.GetSendMailNum(bundle_id)
			if o && mail_num < MONTH_CARD_DAYS_NUM {
				log.Error("Player[%v] payed month card %v is using, not outdate", this.Id, bundle_id)
				return int32(msg_client_message.E_ERR_CHARGE_MONTH_CARD_ALREADY_PAYED), false
			}
		}
	}

	if channel == 1 {
		err_code := verify_google_purchase_data(this, bundle_id, purchase_data, extra_data)
		if err_code < 0 {
			return err_code, false
		}
	} else if channel == 2 {
		err_code := verify_apple_purchase_data(this, bundle_id, purchase_data)
		if err_code < 0 {
			return err_code, false
		}
	} else if channel == 0 {
		// 测试用
	} else {
		log.Error("Player[%v] charge channel[%v] invalid", this.Id, channel)
		return int32(msg_client_message.E_ERR_CHARGE_CHANNEL_INVALID), false
	}

	if has {
		this.db.Pays.IncbyChargeNum(bundle_id, 1)
		this.add_diamond(pay_item.GemReward) // 充值获得钻石
		if pay_item.PayType == table_config.PAY_TYPE_MONTH_CARD {
			this.db.Pays.SetSendMailNum(bundle_id, 0)
			this.db.Pays.SetLastAwardTime(bundle_id, 0)
		}
	} else {
		this.db.Pays.Add(&dbPlayerPayData{
			BundleId: bundle_id,
		})
		this.add_diamond(pay_item.GemRewardFirst) // 首次充值奖励
	}

	this.add_resources(pay_item.ItemReward)

	if pay_item.PayType == table_config.PAY_TYPE_MONTH_CARD {
		charge_month_card_manager.InsertPlayer(this.Id)
	}

	now_time := time.Now()
	this.db.Pays.SetLastPayedTime(bundle_id, int32(now_time.Unix()))

	if aid > 0 && sa != nil {
		this.activity_update_one_charge(aid, sa)
	}

	if this.db.PayCommon.GetFirstPayState() == 0 {
		this.db.PayCommon.SetFirstPayState(1)
		// 首充通知
		notify := &msg_client_message.S2CChargeFirstRewardNotify{}
		this.Send(uint16(msg_client_message_id.MSGID_S2C_CHARGE_FIRST_REWARD_NOTIFY), notify)
	}

	return 1, !has
}

func (this *Player) charge_with_bundle_id(channel int32, bundle_id string, purchase_data []byte, extra_data []byte, index int32) int32 {
	if channel <= 0 {
		log.Error("Player %v charge bundle id %v with channel %v invalid", this.Id, bundle_id, channel)
		return -1
	}

	res, is_first := this._charge_with_bundle_id(channel, bundle_id, purchase_data, extra_data, index)
	if res < 0 {
		return res
	}

	response := &msg_client_message.S2CChargeResponse{
		BundleId:    bundle_id,
		IsFirst:     is_first,
		ClientIndex: index,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_CHARGE_RESPONSE), response)

	log.Trace("Player[%v] charged bundle %v with channel %v", this.Id, response, channel)

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
	} else if pay_state != 1 {
		log.Error("Player[%v] first charge state %v invalid", this.Id, pay_state)
		return -1
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

	this.db.PayCommon.SetFirstPayState(2)

	response := &msg_client_message.S2CChargeFirstAwardResponse{
		Rewards: Map2ItemInfos(rewards),
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_CHARGE_FIRST_AWARD_RESPONSE), response)

	log.Trace("Player[%v] first charge award %v", this.Id, response)

	return 1
}

func C2SChargeDataHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SChargeDataRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%v)", err.Error())
		return -1
	}
	return p.charge_data()
}

func C2SChargeHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SChargeRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%v)", err.Error())
		return -1
	}
	return p.charge_with_bundle_id(req.GetChannel(), req.GetBundleId(), req.GetPurchareData(), req.GetExtraData(), req.GetClientIndex())
}

func C2SChargeFirstAwardHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SChargeFirstAwardRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%v)", err.Error())
		return -1
	}
	return p.charge_first_award()
}
