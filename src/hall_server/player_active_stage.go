package main

import (
	"ih_server/libs/log"
	"ih_server/libs/utils"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"ih_server/src/table_config"
	_ "math/rand"
	_ "sync"
	"time"

	"github.com/golang/protobuf/proto"
)

const (
	ACTIVE_STAGE_PURCHASE_NUM = 10
)

const (
	ACTIVE_STAGE_TYPE_GOLD_CHALLENGE    = 1
	ACTIVE_STAGE_TYPE_WARRIOR_CHALLENGE = 2
	ACTIVE_STAGE_TYPE_HERO_CHALLENGE    = 3
)

var active_stage_types []int32 = []int32{
	ACTIVE_STAGE_TYPE_GOLD_CHALLENGE,
	ACTIVE_STAGE_TYPE_WARRIOR_CHALLENGE,
	ACTIVE_STAGE_TYPE_HERO_CHALLENGE,
}

func (this *Player) _get_active_stage_purchase_num() int32 {
	var purchase_num int32
	vip_info := vip_table_mgr.Get(this.db.Info.GetVipLvl())
	if vip_info != nil {
		purchase_num = vip_info.ActiveStageBuyTimes
	}
	return purchase_num
}

func (this *Player) _active_stage_get_data(t int32) *msg_client_message.ActiveStageData {
	purchase_num := this._get_active_stage_purchase_num()
	remain_num, _ := this.db.ActiveStages.GetCanChallengeNum(t)
	purchased_num, _ := this.db.ActiveStages.GetPurchasedNum(t)
	return &msg_client_message.ActiveStageData{
		StageType:             t,
		RemainChallengeNum:    remain_num,
		RemainBuyChallengeNum: purchase_num - purchased_num,
	}
}

func (this *Player) _send_active_stage_data(typ int32) {
	var datas []*msg_client_message.ActiveStageData
	if typ == 0 {
		for _, t := range active_stage_types {
			if this.db.ActiveStages.HasIndex(t) {
				datas = append(datas, this._active_stage_get_data(t))
			}
		}
	} else {
		datas = []*msg_client_message.ActiveStageData{this._active_stage_get_data(typ)}
	}

	last_refresh := this.db.ActiveStageCommon.GetLastRefreshTime()
	response := &msg_client_message.S2CActiveStageDataResponse{
		StageDatas:            datas,
		MaxChallengeNum:       global_config.ActiveStageChallengeNumOfDay,
		RemainSeconds4Refresh: utils.GetRemainSeconds2NextDayTime(last_refresh, global_config.ActiveStageRefreshTime),
		ChallengeNumPrice:     global_config.ActiveStageChallengeNumPrice,
		GetPointsDay:          this.db.FriendCommon.GetGetPointsDay(),
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_ACTIVE_STAGE_DATA_RESPONSE), response)
	log.Trace("Player[%v] active stage data: %v", this.Id, response)
}

func (this *Player) check_active_stage_refresh(send bool) bool {
	// 固定时间点自动刷新
	if global_config.ActiveStageRefreshTime == "" {
		return false
	}

	now_time := int32(time.Now().Unix())
	last_refresh := this.db.ActiveStageCommon.GetLastRefreshTime()

	if !utils.CheckDayTimeArrival(last_refresh, global_config.ActiveStageRefreshTime) {
		return false
	}

	if this.db.ActiveStages.NumAll() == 0 {
		for _, t := range active_stage_types {
			this.db.ActiveStages.Add(&dbPlayerActiveStageData{
				Type:            t,
				CanChallengeNum: global_config.ActiveStageChallengeNumOfDay,
				PurchasedNum:    0,
			})
		}
	} else {
		for _, t := range active_stage_types {
			this.db.ActiveStages.SetCanChallengeNum(t, global_config.ActiveStageChallengeNumOfDay)
			this.db.ActiveStages.SetPurchasedNum(t, 0)
		}
	}

	this.db.ActiveStageCommon.SetGetPointsDay(0)
	this.db.ActiveStageCommon.SetWithdrawPoints(0)
	this.db.ActiveStageCommon.SetLastRefreshTime(now_time)

	if send {
		this._send_active_stage_data(0)

		notify := &msg_client_message.S2CActiveStageRefreshNotify{}
		this.Send(uint16(msg_client_message_id.MSGID_S2C_ACTIVE_STAGE_REFRESH_NOTIFY), notify)
	}

	log.Trace("Player[%v] active stage refreshed", this.Id)
	return true
}

func (this *Player) send_active_stage_data(typ int32) int32 {
	if this.check_active_stage_refresh(false) {
		return 1
	}
	this._send_active_stage_data(typ)
	return 1
}

func (this *Player) active_stage_challenge_num_purchase(typ, num int32) int32 {
	if num <= 0 {
		return -1
	}

	purchased_num, o := this.db.ActiveStages.GetPurchasedNum(typ)
	if !o {
		log.Error("Player[%v] purchase active stage challenge num with type %v invalid", this.Id, typ)
		return -1
	}

	purchase_num := this._get_active_stage_purchase_num()
	if purchase_num-purchased_num < num {
		log.Error("Player[%v] left purchase num %v for active stage type %v not enough", this.Id, purchased_num-purchase_num, typ)
		return int32(msg_client_message.E_ERR_PLAYER_ACTIVE_STAGE_PURCHASE_NUM_OUT)
	}

	diamond := this.get_resource(ITEM_RESOURCE_ID_DIAMOND)
	if diamond < global_config.ActiveStageChallengeNumPrice*num {
		log.Error("Player[%v] buy active stage challenge num failed, diamond %v not enough, need %v", this.Id, diamond, global_config.ActiveStageChallengeNumPrice)
		return int32(msg_client_message.E_ERR_PLAYER_DIAMOND_NOT_ENOUGH)
	}

	if num == 0 {
		log.Error("Player[%v] active stage challenge num cant buy with 0", this.Id)
		return -1
	}

	this.db.ActiveStages.IncbyCanChallengeNum(typ, num)
	purchased_num = this.db.ActiveStages.IncbyPurchasedNum(typ, num)
	this.add_resource(ITEM_RESOURCE_ID_DIAMOND, -global_config.ActiveStageChallengeNumPrice*num)

	response := &msg_client_message.S2CActiveStageBuyChallengeNumResponse{
		StageType:    typ,
		RemainBuyNum: purchase_num - purchased_num,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_ACTIVE_STAGE_BUY_CHALLENGE_NUM_RESPONSE), response)

	this._send_active_stage_data(typ)

	log.Trace("Player[%v] active stage purchased challenge num response %v", this.Id, response)

	return 1
}

// 助战友情点
func (this *Player) get_assist_points() int32 {
	curr_points := this.db.ActiveStageCommon.GetGetPointsDay()
	withdraw_points := this.db.ActiveStageCommon.GetWithdrawPoints()
	get_points := curr_points - withdraw_points
	if get_points < 0 {
		get_points = 0
	} else if get_points > 0 {
		if get_points+withdraw_points > global_config.FriendAssistPointsGetLimitDay {
			get_points = global_config.FriendAssistPointsGetLimitDay - withdraw_points
		}
	}
	log.Trace("Player[%v] assist points %v", this.Id, get_points)
	return get_points
}

// 提现助战友情点
func (this *Player) active_stage_withdraw_assist_points() int32 {
	get_points := this.get_assist_points()
	if get_points > 0 {
		this.db.ActiveStageCommon.IncbyWithdrawPoints(get_points)
		this.add_resource(global_config.FriendPointItemId, get_points)
	}
	response := &msg_client_message.S2CFriendGetAssistPointsResponse{
		GetPoints:      get_points,
		TotalGetPoints: this.db.ActiveStageCommon.GetWithdrawPoints(),
		CanGetPoints:   0,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_FRIEND_GET_ASSIST_POINTS_RESPONSE), response)
	return 1
}

func (this *Player) active_stage_get_friends_assist_role_list() int32 {
	var roles []*msg_client_message.Role
	friend_ids := this.db.Friends.GetAllIndex()
	if friend_ids != nil && len(friend_ids) > 0 {
		for i := 0; i < len(friend_ids); i++ {
			friend := player_mgr.GetPlayerById(friend_ids[i])
			if friend == nil {
				continue
			}
			role_id := friend.db.FriendCommon.GetAssistRoleId()
			if role_id == 0 || !friend.db.Roles.HasIndex(role_id) {
				continue
			}
			table_id, _ := friend.db.Roles.GetTableId(role_id)
			level, _ := friend.db.Roles.GetLevel(role_id)
			rank, _ := friend.db.Roles.GetRank(role_id)
			equips, _ := friend.db.Roles.GetEquip(role_id)
			roles = append(roles, &msg_client_message.Role{
				Id:       role_id,
				TableId:  table_id,
				Level:    level,
				Rank:     rank,
				Equips:   equips,
				PlayerId: friend_ids[i],
			})
		}
	}
	response := &msg_client_message.S2CActiveStageAssistRoleListResponse{
		Roles: roles,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_ACTIVE_STAGE_ASSIST_ROLE_LIST_RESPONSE), response)

	log.Trace("Player[%v] active stage get assist role list %v", this.Id, response)

	return 1
}

// 增加友情点
func (this *Player) friend_assist_add_points(points int32) bool {
	this.db.ActiveStageCommon.IncbyGetPointsDay(points)
	log.Debug("Add assist friend points %v to player %v", points, this.Id)
	return true
}

func (this *Player) fight_active_stage(active_stage_id int32) int32 {
	var active_stage *table_config.XmlActiveStageItem
	active_stage = active_stage_table_mgr.Get(active_stage_id)
	if active_stage == nil {
		log.Error("Active stage %v table data not found", active_stage_id)
		return -1
	}

	if active_stage.PlayerLevel > this.db.Info.GetLvl() {
		log.Error("Player[%v] fight active stage %v level %v not enough, need %v", this.Id, active_stage_id, this.db.Info.GetLvl(), active_stage.PlayerLevel)
		return int32(msg_client_message.E_ERR_PLAYER_ACTIVE_STAGE_LEVEL_NOT_ENOUGH)
	}

	stage_id := active_stage.StageId
	stage := stage_table_mgr.Get(stage_id)
	if stage == nil {
		log.Error("Active stage[%v] stage[%v] not found", active_stage_id, stage_id)
		return int32(msg_client_message.E_ERR_PLAYER_STAGE_TABLE_DATA_NOT_FOUND)
	}

	this.check_active_stage_refresh(false)

	can_num, _ := this.db.ActiveStages.GetCanChallengeNum(active_stage.Type)
	if can_num <= 0 {
		log.Error("Player[%v] active stage challenge num used out", this.Id)
		return -1
	}

	err, is_win, my_team, target_team, my_artifact_id, target_artifact_id, enter_reports, rounds, _ := this.FightInStage(4, stage, nil, nil)
	if err < 0 {
		log.Error("Player[%v] fight active stage %v failed", this.Id, active_stage_id)
		return err
	}

	if is_win {
		this.db.ActiveStages.IncbyCanChallengeNum(active_stage.Type, -1)
		this.send_stage_reward(stage.RewardList, 4, 0)
	}

	member_damages := this.active_stage_team.common_data.members_damage
	member_cures := this.active_stage_team.common_data.members_cure
	var assist_friend_id int32
	if this.assist_friend != nil {
		if is_win {
			this.assist_friend.friend_assist_add_points(global_config.FriendAssistPointsGet)
		}
		assist_friend_id = this.assist_friend.Id
	}
	response := &msg_client_message.S2CBattleResultResponse{
		IsWin:               is_win,
		MyTeam:              my_team,
		TargetTeam:          target_team,
		EnterReports:        enter_reports,
		Rounds:              rounds,
		MyMemberDamages:     member_damages[this.active_stage_team.side],
		TargetMemberDamages: member_damages[this.target_stage_team.side],
		MyMemberCures:       member_cures[this.active_stage_team.side],
		TargetMemberCures:   member_cures[this.target_stage_team.side],
		BattleType:          4,
		BattleParam:         active_stage_id,
		AssistFriendId:      assist_friend_id,
		AssistRoleId:        this.assist_role_id,
		AssistPos:           this.assist_role_pos,
		MyArtifactId:        my_artifact_id,
		TargetArtifactId:    target_artifact_id,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_BATTLE_RESULT_RESPONSE), response)

	if is_win {
		// 更新任务
		this.TaskUpdate(table_config.TASK_COMPLETE_TYPE_ACTIVE_STAGE_WIN_NUM, false, 0, 1)
	}

	//Output_S2CBattleResult(this, response)

	log.Trace("Player %v fight active stage %v", this.Id, active_stage_id)

	return 1
}

func C2SActiveStageDataHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SActiveStageDataRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}
	return p.send_active_stage_data(req.GetStageType())
}

func C2SActiveStageBuyChallengeNumHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SActiveStageBuyChallengeNumRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}
	return p.active_stage_challenge_num_purchase(req.GetStageType(), req.GetNum())
}

func C2SActiveStageGetAssistRoleListHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SActiveStageAssistRoleListRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}
	return p.active_stage_get_friends_assist_role_list()
}
