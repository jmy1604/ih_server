package main

import (
	"ih_server/libs/log"
	"ih_server/libs/utils"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"ih_server/src/table_config"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
)

// -------------------------------- 关卡排行榜 --------------------------------
var campaign_rank_serial_id int32

type CampaignRankItem struct {
	SerialId   int32
	CampaignId int32
	PlayerId   int32
}

func (this *CampaignRankItem) Less(value interface{}) bool {
	item := value.(*CampaignRankItem)
	if item == nil {
		return false
	}
	if this.CampaignId < item.CampaignId {
		return true
	}
	if this.CampaignId == item.CampaignId {
		if this.SerialId > item.SerialId {
			return true
		}
	}
	return false
}

func (this *CampaignRankItem) Greater(value interface{}) bool {
	item := value.(*CampaignRankItem)
	if item == nil {
		return false
	}
	if this.CampaignId > item.CampaignId {
		return true
	}
	if this.CampaignId == item.CampaignId {
		if this.SerialId < item.SerialId {
			return true
		}
	}
	return false
}

func (this *CampaignRankItem) KeyEqual(value interface{}) bool {
	item := value.(*CampaignRankItem)
	if item == nil {
		return false
	}
	if item == nil {
		return false
	}
	if this.PlayerId == item.PlayerId {
		return true
	}
	return false
}

func (this *CampaignRankItem) GetKey() interface{} {
	return this.PlayerId
}

func (this *CampaignRankItem) GetValue() interface{} {
	return this.CampaignId
}

func (this *CampaignRankItem) SetValue(value interface{}) {
	this.CampaignId = value.(int32)
	this.SerialId = atomic.AddInt32(&campaign_rank_serial_id, 1)
}

func (this *CampaignRankItem) New() utils.SkiplistNode {
	return &CampaignRankItem{}
}

func (this *CampaignRankItem) Assign(node utils.SkiplistNode) {
	n := node.(*CampaignRankItem)
	if n == nil {
		return
	}
	this.PlayerId = n.PlayerId
	this.CampaignId = n.CampaignId
	this.SerialId = n.SerialId
}

func (this *CampaignRankItem) CopyDataTo(node interface{}) {
	n := node.(*CampaignRankItem)
	if n == nil {
		return
	}
	n.PlayerId = this.PlayerId
	n.CampaignId = this.CampaignId
	n.SerialId = this.SerialId
}

// ----------------------------------------------------------------------------

// 下一关
func get_next_campaign_id(campaign_id int32) int32 {
	campaign := campaign_table_mgr.Get(campaign_id)
	if campaign == nil {
		return 0
	}
	return campaign.UnlockMap
}

// 获得关卡章节和难度
func get_campaign_chapter_and_difficulty(campaign_id int32) (int32, int32) {
	campaign := campaign_table_mgr.Get(campaign_id)
	if campaign == nil {
		return 0, 0
	}
	return campaign.ChapterMap, campaign.Difficulty
}

// 获取stage_id
func get_stage_by_campaign(campaign_id int32) *table_config.XmlPassItem {
	campaign := campaign_table_mgr.Get(campaign_id)
	if campaign == nil {
		log.Error("战役[%v]找不到", campaign_id)
		return nil
	}
	return stage_table_mgr.Get(campaign.StageId)
}

// 是否解锁下一难度
func (this *Player) is_unlock_next_difficulty(curr_campaign_id int32) (bool, int32) {
	campaign := campaign_table_mgr.Get(curr_campaign_id)
	if campaign == nil {
		return false, 0
	}

	campaign_ids := campaign_table_mgr.GetDifficultyCampaign(campaign.Difficulty)
	if campaign_ids == nil || len(campaign_ids) == 0 {
		return false, 0
	}

	for i := 0; i < len(campaign_ids); i++ {
		if !this.db.Campaigns.HasIndex(campaign_ids[i]) {
			return false, 0
		}
	}

	if curr_campaign_id != campaign_ids[len(campaign_ids)-1] {
		return false, 0
	}

	next_campaign := campaign_table_mgr.Get(campaign.UnlockMap)
	if next_campaign == nil {
		return false, 0
	}

	return true, next_campaign.Difficulty
}

func (this *Player) _update_campaign_rank_data(campaign_id, sid int32) {
	if sid == 0 {
		campaign_rank_serial_id += 1
		sid = campaign_rank_serial_id
	}
	var data = CampaignRankItem{
		SerialId:   sid,
		CampaignId: campaign_id,
		PlayerId:   this.Id,
	}
	rank_list_mgr.UpdateItem(RANK_LIST_TYPE_CAMPAIGN, &data)
}

func (this *Player) LoadCampaignRankData() {
	campaign_id := this.db.CampaignCommon.GetLastestPassedCampaignId()
	if campaign_id <= 0 {
		return
	}
	sid := this.db.CampaignCommon.GetRankSerialId()
	if campaign_rank_serial_id < sid {
		campaign_rank_serial_id = sid
	}
	this._update_campaign_rank_data(campaign_id, sid)
}

func (this *Player) FightInStage(stage_type int32, stage *table_config.XmlPassItem, friend *Player, guild *dbGuildRow) (err int32, is_win bool, my_team, target_team []*msg_client_message.BattleMemberItem, enter_reports []*msg_client_message.BattleReportItem, rounds []*msg_client_message.BattleRoundReports, has_next_wave bool) {
	var attack_team *BattleTeam
	var team_type int32
	if stage_type == 1 {
		// PVP竞技场
		if this.attack_team == nil {
			this.attack_team = &BattleTeam{}
		}
		attack_team = this.attack_team
		team_type = BATTLE_TEAM_ATTACK
	} else if stage_type == 2 {
		// PVE战役
		if this.campaign_team == nil {
			this.campaign_team = &BattleTeam{}
		}
		attack_team = this.campaign_team
		team_type = BATTLE_TEAM_CAMPAIN
	} else if stage_type == 3 {
		// 爬塔
		if this.tower_team == nil {
			this.tower_team = &BattleTeam{}
		}
		attack_team = this.tower_team
		team_type = BATTLE_TEAM_TOWER
	} else if stage_type == 4 {
		// 活动副本，助战角色
		if this.active_stage_team == nil {
			this.active_stage_team = &BattleTeam{}
		}
		attack_team = this.active_stage_team
		team_type = BATTLE_TEAM_ACTIVE_STAGE
	} else if stage_type == 5 {
		// 好友BOSS
		if this.friend_boss_team == nil {
			this.friend_boss_team = &BattleTeam{}
		}
		attack_team = this.friend_boss_team
		team_type = BATTLE_TEAM_FRIEND_BOSS
	} else if stage_type == 6 || stage_type == 7 {
		// 探索副本
		if this.explore_team == nil {
			this.explore_team = &BattleTeam{}
		}
		attack_team = this.explore_team
		team_type = BATTLE_TEAM_EXPLORE
	} else if stage_type == 9 {
		// 公会副本
		if this.guild_stage_team == nil {
			this.guild_stage_team = &BattleTeam{}
		}
		attack_team = this.guild_stage_team
		team_type = BATTLE_TEAM_GUILD_STAGE
	} else {
		err = int32(msg_client_message.E_ERR_PLAYER_TEAM_TYPE_INVALID)
		log.Error("Stage type %v invalid", stage_type)
		return
	}

	if this.target_stage_team == nil {
		this.target_stage_team = &BattleTeam{}
	}

	// 新的关卡初始化
	if stage.Id != this.stage_id {
		this.stage_wave = 0
		err = attack_team.Init(this, team_type, 0)
		if err < 0 {
			log.Error("Player[%v] init attack team failed", this.Id)
			return
		}
	} else {
		if this.stage_wave == 0 {
			err = attack_team.Init(this, team_type, 0)
			if err < 0 {
				log.Error("Player[%v] init attack team failed", this.Id)
				return
			}
		}
	}

	self_member_num := attack_team.MembersNum()
	if this.assist_role_id > 0 && this.assist_role_id >= 0 {
		self_member_num = self_member_num - 1
	}

	if stage.PlayerCardMax > 0 && self_member_num > stage.PlayerCardMax {
		log.Error("Player[%v] fight stage %v is limited with member num", this.Id, stage.Id)
		err = int32(msg_client_message.E_ERR_PLAYER_STAGE_ROLE_NUM_LIMITED)
		return
	}

	if !this.target_stage_team.InitWithStage(1, stage.Id, this.stage_wave, friend, guild) {
		err = -1
		log.Error("Player[%v] init stage[%v] wave[%v] team failed", this.Id, stage.Id, this.stage_wave)
		return
	}

	my_team = attack_team._format_members_for_msg()
	target_team = this.target_stage_team._format_members_for_msg()

	// 扫荡状态
	if this.sweep_num > 0 {
		attack_team.is_sweeping = true
		this.target_stage_team.is_sweeping = true
	}

	is_win, enter_reports, rounds = attack_team.Fight(this.target_stage_team, BATTLE_END_BY_ROUND_OVER, stage.MaxRound)

	// 清除扫荡状态
	if attack_team.is_sweeping {
		attack_team.is_sweeping = false
	}
	if this.target_stage_team.is_sweeping {
		this.target_stage_team.is_sweeping = false
	}

	this.stage_id = stage.Id
	this.stage_wave += 1
	if this.stage_wave >= stage.MaxWaves {
		this.stage_wave = 0
	} else {
		has_next_wave = true
	}

	err = 1

	return
}

func (this *Player) send_stage_reward(rewards []int32, reward_type int32, income_remain_seconds int32) {
	if rewards == nil || len(rewards) == 0 {
		return
	}

	var item_rewards []*msg_client_message.ItemInfo
	// 奖励
	for i := 0; i < len(rewards)/2; i++ {
		item_id := rewards[2*i]
		item_num := rewards[2*i+1]
		this.add_resource(item_id, item_num)
		item_rewards = append(item_rewards, &msg_client_message.ItemInfo{
			Id:    item_id,
			Value: item_num,
		})
	}
	this._send_stage_reward(item_rewards, reward_type, income_remain_seconds)
}

func (this *Player) _send_stage_reward(item_rewards []*msg_client_message.ItemInfo, reward_type int32, income_remain_seconds int32) {
	response := &msg_client_message.S2CCampaignHangupIncomeResponse{
		Rewards:                   item_rewards,
		IncomeType:                reward_type,
		HangupIncomeRemainSeconds: income_remain_seconds,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_CAMPAIGN_HANGUP_INCOME_RESPONSE), response)
}

func (this *Player) FightInCampaign(campaign_id int32) int32 {
	stage := get_stage_by_campaign(campaign_id)
	if stage == nil {
		log.Error("Cant found stage by campaign[%v]", campaign_id)
		return int32(msg_client_message.E_ERR_PLAYER_NOT_FOUND_CAMPAIGN_TABLE_DATA)
	}

	if this.db.Campaigns.HasIndex(campaign_id) {
		log.Error("Player[%v] already fight campaign[%v]", this.Id, campaign_id)
		return int32(msg_client_message.E_ERR_PLAYER_ALREADY_FIGHT_CAMPAIGN)
	}

	current_campaign_id := this.db.CampaignCommon.GetCurrentCampaignId()
	if current_campaign_id == 0 {
		if campaign_id != campaign_table_mgr.Array[0].Id {
			log.Error("Player[%v] fight first campaign[%v] invalid", this.Id, campaign_id)
			return -1
		}
	} else if current_campaign_id != campaign_id {
		log.Error("Player[%v] fight campaign[%v] cant allow", this.Id, campaign_id)
		return int32(msg_client_message.E_ERR_PLAYER_CAMPAIGN_MUST_PlAY_NEXT)
	}

	err, is_win, my_team, target_team, enter_reports, rounds, has_next_wave := this.FightInStage(2, stage, nil, nil)
	if err < 0 {
		log.Error("Player[%v] fight campaign %v failed, err %v", this.Id, campaign_id, err)
		return err
	}

	next_campaign_id := int32(0)
	if is_win && !has_next_wave {
		this.db.Campaigns.Add(&dbPlayerCampaignData{
			CampaignId: campaign_id,
		})
		next_campaign_id = get_next_campaign_id(campaign_id)
		this.db.CampaignCommon.SetCurrentCampaignId(next_campaign_id)
		// 产生剧情探索任务
		campaign := campaign_table_mgr.Get(campaign_id)
		if campaign != nil && campaign.CampaignTask > 0 {
			this.explore_gen_story_task(campaign.CampaignTask)
		}
	} else {
		this.db.CampaignCommon.SetCurrentCampaignId(campaign_id)
	}

	member_damages := this.campaign_team.common_data.members_damage
	member_cures := this.campaign_team.common_data.members_cure
	response := &msg_client_message.S2CBattleResultResponse{
		IsWin:               is_win,
		EnterReports:        enter_reports,
		Rounds:              rounds,
		MyTeam:              my_team,
		TargetTeam:          target_team,
		MyMemberDamages:     member_damages[this.campaign_team.side],
		TargetMemberDamages: member_damages[this.target_stage_team.side],
		MyMemberCures:       member_cures[this.campaign_team.side],
		TargetMemberCures:   member_cures[this.target_stage_team.side],
		HasNextWave:         has_next_wave,
		NextCampaignId:      next_campaign_id,
		BattleType:          2,
		BattleParam:         campaign_id,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_BATTLE_RESULT_RESPONSE), response)

	if is_win && !has_next_wave {
		this.db.CampaignCommon.SetLastestPassedCampaignId(campaign_id)
		this.send_stage_reward(stage.RewardList, 2, 0)
		// 更新排名
		this._update_campaign_rank_data(campaign_id, atomic.AddInt32(&campaign_rank_serial_id, 1))
		// 更新任务 通过章节
		this.TaskUpdate(table_config.TASK_COMPLETE_TYPE_PASS_CAMPAIGN, false, campaign_id, 1)
	}

	Output_S2CBattleResult(this, response)
	return 1
}

// 设置挂机战役关卡
func (this *Player) set_hangup_campaign_id(campaign_id int32) int32 {
	hangup_id := this.db.CampaignCommon.GetHangupCampaignId()
	if hangup_id == 0 {
		if campaign_id != campaign_table_mgr.Array[0].Id {
			return int32(msg_client_message.E_ERR_PLAYER_CANT_FIGHT_THE_CAMPAIGN)
		}
	} else if campaign_id != hangup_id {
		if !this.db.Campaigns.HasIndex(campaign_id) {
			current_campaign_id := this.db.CampaignCommon.GetCurrentCampaignId()
			//next_campaign_id := get_next_campaign_id(current_campaign_id)
			if current_campaign_id != campaign_id {
				return int32(msg_client_message.E_ERR_PLAYER_CAMPAIGN_MUST_PlAY_NEXT)
			}

			// 关卡完成就结算一次挂机收益
			if hangup_id != 0 {
				this.campaign_hangup_income_get(0, true)
				this.campaign_hangup_income_get(1, true)
			}
		}
	}

	// 设置挂机开始时间
	now_time := int32(time.Now().Unix())
	if hangup_id == 0 {
		this.db.CampaignCommon.SetHangupLastDropStaticIncomeTime(now_time)
		this.db.CampaignCommon.SetHangupLastDropRandomIncomeTime(now_time)
	}
	this.db.CampaignCommon.SetHangupCampaignId(campaign_id)

	return 1
}

func (this *Player) campaign_cache_static_income(item_id, item_num int32) *msg_client_message.ItemInfo {
	if !this.db.CampaignStaticIncomes.HasIndex(item_id) {
		this.db.CampaignStaticIncomes.Add(&dbPlayerCampaignStaticIncomeData{
			ItemId:  item_id,
			ItemNum: item_num,
		})
	} else {
		this.db.CampaignStaticIncomes.IncbyItemNum(item_id, item_num)
	}

	item_num, _ = this.db.CampaignStaticIncomes.GetItemNum(item_id)
	return &msg_client_message.ItemInfo{
		Id:    item_id,
		Value: item_num,
	}
}

func (this *Player) campaign_get_static_income(campaign *table_config.XmlCampaignItem, last_time, now_time int32, is_cache bool) (incomes []*msg_client_message.ItemInfo, correct_secs int32) {
	st := now_time - last_time
	correct_secs = (st % campaign.StaticRewardSec)
	var tmp_cache_items map[int32]int32

	// 固定掉落
	n := st / campaign.StaticRewardSec
	for i := 0; i < len(campaign.StaticRewardItem)/2; i++ {
		item_id := campaign.StaticRewardItem[2*i]
		item_num := n * campaign.StaticRewardItem[2*i+1]
		if is_cache {
			income := this.campaign_cache_static_income(item_id, item_num)
			incomes = append(incomes, income)
		} else {
			if tmp_cache_items == nil {
				tmp_cache_items = make(map[int32]int32)
			}
			d := tmp_cache_items[item_id]
			tmp_cache_items[item_id] = d + item_num
		}
	}

	if !is_cache {
		cache := this.db.CampaignStaticIncomes.GetAllIndex()
		if cache != nil {
			for i := 0; i < len(cache); i++ {
				n, _ := this.db.CampaignStaticIncomes.GetItemNum(cache[i])
				d := tmp_cache_items[cache[i]]
				tmp_cache_items[cache[i]] = d + n
			}
			this.db.CampaignStaticIncomes.Clear()
		}
		if tmp_cache_items != nil {
			for k, v := range tmp_cache_items {
				if this.add_resource(k, v) {
					incomes = append(incomes, &msg_client_message.ItemInfo{
						Id:    k,
						Value: v,
					})
				}
			}
		}
	}

	return
}

func (this *Player) campaign_has_random_income() bool {
	campaign := campaign_table_mgr.Get(this.db.CampaignCommon.GetHangupCampaignId())
	if campaign == nil {
		return false
	}

	random_income_time := this.db.CampaignCommon.GetHangupLastDropRandomIncomeTime()
	now_time := int32(time.Now().Unix())
	if now_time-random_income_time >= campaign.RandomDropSec {
		return true
	}

	if this.db.CampaignRandomIncomes.NumAll() > 0 {
		return true
	}

	return false
}

func (this *Player) campaign_cache_random_income(item_id, item_num int32) {
	if !this.db.CampaignRandomIncomes.HasIndex(item_id) {
		this.db.CampaignRandomIncomes.Add(&dbPlayerCampaignRandomIncomeData{
			ItemId:  item_id,
			ItemNum: item_num,
		})
	} else {
		this.db.CampaignRandomIncomes.IncbyItemNum(item_id, item_num)
	}
}

func (this *Player) campaign_get_random_income(campaign *table_config.XmlCampaignItem, last_time, now_time int32, is_cache bool) (has_income bool, incomes []*msg_client_message.ItemInfo, correct_secs int32) {
	rt := now_time - last_time
	correct_secs = rt % campaign.RandomDropSec
	// 随机掉落
	rand.Seed(time.Now().Unix())
	this.tmp_cache_items = make(map[int32]int32)
	n := rt / campaign.RandomDropSec
	for k := 0; k < int(n); k++ {
		for i := 0; i < len(campaign.RandomDropIDList)/2; i++ {
			group_id := campaign.RandomDropIDList[2*i]
			count := campaign.RandomDropIDList[2*i+1]
			for j := 0; j < int(count); j++ {
				if o, _ := this.drop_item_by_id(group_id, false, nil); !o {
					continue
				}
			}
		}
	}

	log.Debug("now_time: %v   last_time: %v   rt: %v   n: %v   tmp_cache_items: %v", now_time, last_time, rt, n, this.tmp_cache_items)

	if !is_cache {
		// 缓存的收益
		cache := this.db.CampaignRandomIncomes.GetAllIndex()
		if cache != nil {
			for i := 0; i < len(cache); i++ {
				n, _ := this.db.CampaignRandomIncomes.GetItemNum(cache[i])

				d := this.tmp_cache_items[cache[i]]
				this.tmp_cache_items[cache[i]] = d + n
			}
			this.db.CampaignRandomIncomes.Clear()
		}

		for k, v := range this.tmp_cache_items {
			if this.add_resource(k, v) {
				incomes = append(incomes, &msg_client_message.ItemInfo{
					Id:    k,
					Value: v,
				})
				has_income = true
			}
		}
	} else {
		for k, v := range this.tmp_cache_items {
			this.campaign_cache_random_income(k, v)
		}

		if this.db.CampaignRandomIncomes.NumAll() > 0 {
			has_income = true
		}
	}
	this.tmp_cache_items = nil
	return
}

// 关卡挂机收益
func (this *Player) campaign_hangup_income_get(income_type int32, is_cache bool) (incomes []*msg_client_message.ItemInfo, income_remain_seconds int32) {
	hangup_id := this.db.CampaignCommon.GetHangupCampaignId()
	if hangup_id == 0 {
		return
	}

	campaign := campaign_table_mgr.Get(hangup_id)
	if campaign == nil {
		return
	}

	now_time := int32(time.Now().Unix())
	last_logout := this.db.Info.GetLastLogout()
	var has_income bool
	if income_type == 0 {
		static_income_time := this.db.CampaignCommon.GetHangupLastDropStaticIncomeTime()
		var cs int32
		if last_logout > 0 && last_logout >= static_income_time && now_time-last_logout >= 8*3600 {
			incomes, cs = this.campaign_get_static_income(campaign, static_income_time, last_logout+8*3600, is_cache)
		} else {
			incomes, cs = this.campaign_get_static_income(campaign, static_income_time, now_time, is_cache)
		}

		this.db.CampaignCommon.SetHangupLastDropStaticIncomeTime(now_time - cs)
		income_remain_seconds = campaign.RandomDropSec - cs
	} else {
		random_income_time := this.db.CampaignCommon.GetHangupLastDropRandomIncomeTime()
		var cr int32
		if last_logout > 0 && last_logout >= random_income_time && now_time-last_logout >= 8*3600 {
			has_income, incomes, cr = this.campaign_get_random_income(campaign, random_income_time, last_logout+8*3600, is_cache)
		} else {
			has_income, incomes, cr = this.campaign_get_random_income(campaign, random_income_time, now_time, is_cache)
		}

		this.db.CampaignCommon.SetHangupLastDropRandomIncomeTime(now_time - cr)
		income_remain_seconds = campaign.RandomDropSec - cr
	}

	if has_income || (incomes != nil && len(incomes) > 0) {
		income_remain_seconds = 0
	}

	if !is_cache {
		this._send_stage_reward(incomes, income_type, income_remain_seconds)
		if incomes != nil {
			// 更新任务
			this.TaskUpdate(table_config.TASK_COMPLETE_TYPE_HUANG_UP_NUM, false, 0, 1)
			log.Debug("Player[%v] hangup %v incomes: %v", this.Id, income_type, incomes)
		}
	}

	return
}

func (this *dbPlayerCampaignColumn) GetPassedCampaignIds() []int32 {
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignColumn.GetPassedCampaignIds")
	defer this.m_row.m_lock.UnSafeRUnlock()

	var ids []int32
	for id, _ := range this.m_data {
		ids = append(ids, id)
	}
	return ids
}

func (this *Player) send_campaigns() {
	incomes, _ := this.campaign_hangup_income_get(0, true)
	_, remain_seconds := this.campaign_hangup_income_get(1, true)
	passed_ids := this.db.Campaigns.GetPassedCampaignIds()
	response := &msg_client_message.S2CCampaignDataResponse{}
	response.PassedCampaignIds = passed_ids
	response.UnlockCampaignId = this.db.CampaignCommon.GetCurrentCampaignId()
	response.HangupCampaignId = this.db.CampaignCommon.GetHangupCampaignId()
	response.StaticIncomes = incomes
	response.IncomeRemainSeconds = remain_seconds
	response.AccelerateRefreshRemainSeconds, response.RemainAccelerateNum, response.AccelerateCostDiamond = this.campaign_check_accel_refresh()
	this.Send(uint16(msg_client_message_id.MSGID_S2C_CAMPAIGN_DATA_RESPONSE), response)

	log.Trace("Player[%v] campaign data %v", this.Id, response)
}

func (this *Player) campaign_check_accel_refresh() (remain_refresh_seconds, remain_accel_num, next_cost_diamond int32) {
	last_refresh := this.db.CampaignCommon.GetVipAccelRefreshTime()
	remain_refresh_seconds = utils.GetRemainSeconds2NextDayTime(last_refresh, "00:00:00")
	if remain_refresh_seconds <= 0 {
		this.db.CampaignCommon.SetVipAccelNum(0)
		now_time := time.Now()
		this.db.CampaignCommon.SetVipAccelRefreshTime(int32(now_time.Unix()))
		remain_refresh_seconds = utils.GetRemainSeconds2NextDayTime(int32(now_time.Unix()), "00:00:00")
	}
	accel_num := this.db.CampaignCommon.GetVipAccelNum()
	vip_info := vip_table_mgr.Get(this.db.Info.GetVipLvl())
	if vip_info != nil {
		remain_accel_num = vip_info.AccelTimes - accel_num
	}
	accel_info := accel_cost_table_mgr.Get(accel_num + 1)
	if accel_info != nil {
		next_cost_diamond = accel_info.Cost
	}
	return
}

func (this *Player) campaign_accel_get_income() int32 {
	this.campaign_check_accel_refresh()

	lvl := this.db.Info.GetVipLvl()
	vip_info := vip_table_mgr.Get(lvl)
	if vip_info == nil {
		log.Error("Player[%v] vip level %v not found in vip table", this.Id, lvl)
		return -1
	}

	accel_num := this.db.CampaignCommon.GetVipAccelNum()
	if accel_num >= vip_info.AccelTimes {
		log.Error("Player[%v] vip level %v accelerate campaign income num %v used out", this.Id, lvl, vip_info.AccelTimes)
		return -1
	}

	accel_info := accel_cost_table_mgr.Get(accel_num + 1)
	if accel_info == nil {
		log.Error("AccelCost table data with accel num %v not found", accel_num)
		return -1
	}

	if this.get_diamond() < accel_info.Cost {
		log.Error("Player[%v] accelerate get campagin income not enough diamond", this.Id)
		return int32(msg_client_message.E_ERR_PLAYER_DIAMOND_NOT_ENOUGH_FOR_ACCEL)
	}

	hungup_id := this.db.CampaignCommon.GetHangupCampaignId()
	campaign := campaign_table_mgr.Get(hungup_id)
	if campaign == nil {
		log.Error("Player %v hung up campagin %v table data not found", this.Id, hungup_id)
		return int32(msg_client_message.E_ERR_PLAYER_NOT_FOUND_CAMPAIGN_TABLE_DATA)
	}

	var incomes map[int32]int32 = make(map[int32]int32)
	// 固定掉落
	n := 2 * 3600 / campaign.StaticRewardSec
	for i := 0; i < len(campaign.StaticRewardItem)/2; i++ {
		item_id := campaign.StaticRewardItem[2*i]
		item_num := n * campaign.StaticRewardItem[2*i+1]
		this.add_resource(item_id, item_num)
		incomes[item_id] += item_num
	}
	// 随机掉落
	rand.Seed(time.Now().Unix() + time.Now().UnixNano())
	n = 2 * 3600 / campaign.RandomDropSec
	for k := 0; k < int(n); k++ {
		for i := 0; i < len(campaign.RandomDropIDList)/2; i++ {
			group_id := campaign.RandomDropIDList[2*i]
			count := campaign.RandomDropIDList[2*i+1]
			for j := 0; j < int(count); j++ {
				if o, item := this.drop_item_by_id(group_id, false, nil); o && item != nil {
					this.add_resource(item.GetId(), item.GetValue())
					incomes[item.GetId()] += item.GetValue()
				}
			}
		}
	}

	this.add_diamond(-accel_info.Cost)
	accel_num = this.db.CampaignCommon.IncbyVipAccelNum(1)

	var next_cost_diamond int32
	accel_info = accel_cost_table_mgr.Get(accel_num)
	if accel_info != nil {
		next_cost_diamond = accel_info.Cost
	}

	response := &msg_client_message.S2CCampaignAccelerateIncomeResponse{
		Incomes:         Map2ItemInfos(incomes),
		RemainNum:       vip_info.AccelTimes - accel_num,
		NextCostDiamond: next_cost_diamond,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_CAMPAIGN_ACCELERATE_INCOME_RESPONSE), response)

	log.Trace("Player[%v] accelerate campagin %v income response %v", this.Id, hungup_id, response)

	return 1
}

func (this *Player) campaign_accel_num_refresh() int32 {
	this.campaign_check_accel_refresh()

	vip_lvl := this.db.Info.GetVipLvl()
	vip_info := vip_table_mgr.Get(vip_lvl)
	if vip_info == nil {
		log.Error("Player[%v] vip level %v not found in vip table", this.Id, vip_lvl)
		return -1
	}

	if this.get_diamond() < global_config.AccelHungupRefreshCostDiamond {
		log.Error("Player[%v] not enough diamond to refresh accel hungup", this.Id)
		return -1
	}

	if this.db.CampaignCommon.GetVipAccelNum() == 0 {
		log.Error("Player[%v] no need to refresh campaign accel num", this.Id)
		return -1
	}

	this.db.CampaignCommon.SetVipAccelNum(0)
	this.add_diamond(-global_config.AccelHungupRefreshCostDiamond)

	response := &msg_client_message.S2CCampaignAccelerateRefreshResponse{
		RemainNum: vip_info.AccelTimes,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_CAMPAIGN_ACCELERATE_REFRESH_RESPONSE), response)

	log.Trace("Player[%v] refreshed hungup accel num", this.Id)

	return 1
}

func C2SCampaignAccelGetIncomeHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SCampaignAccelerateIncomeRequest
	err := proto.Unmarshal(msg_data, &req)
	if nil != err {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.campaign_accel_get_income()
}

func C2SCampaignAccelNumRefreshHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SCampaignAccelerateRefreshRequest
	err := proto.Unmarshal(msg_data, &req)
	if nil != err {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.campaign_accel_num_refresh()
}
