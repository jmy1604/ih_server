package main

import (
	"ih_server/libs/log"
	"ih_server/libs/utils"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	_ "math/rand"
	"net/http"
	_ "sync"
	"time"

	"github.com/golang/protobuf/proto"
)

type GuildStageDamageItem struct {
	AttackerId int32
	Damage     int32
}

func (this *GuildStageDamageItem) Less(item utils.ShortRankItem) bool {
	it := item.(*GuildStageDamageItem)
	if it == nil {
		return false
	}
	if this.Damage < it.Damage {
		return true
	}
	return false
}

func (this *GuildStageDamageItem) Greater(item utils.ShortRankItem) bool {
	it := item.(*GuildStageDamageItem)
	if it == nil {
		return false
	}
	if this.Damage > it.Damage {
		return true
	}
	return false
}

func (this *GuildStageDamageItem) GetKey() interface{} {
	return this.AttackerId
}

func (this *GuildStageDamageItem) GetValue() interface{} {
	return this.Damage
}

func (this *GuildStageDamageItem) Assign(item utils.ShortRankItem) {
	it := item.(*GuildStageDamageItem)
	if it == nil {
		return
	}
	this.AttackerId = it.AttackerId
	this.Damage = it.Damage
}

func (this *GuildStageDamageItem) Add(item utils.ShortRankItem) {
	it := item.(*GuildStageDamageItem)
	if it == nil {
		return
	}
	if this.AttackerId == it.AttackerId {
		this.Damage += it.Damage
	}
}

func (this *GuildStageDamageItem) New() utils.ShortRankItem {
	return &GuildStageDamageItem{}
}

type GuildStageManager struct {
	stages *dbGuildStageTable
}

var guild_stage_manager GuildStageManager

func (this *GuildStageManager) Init() {
	this.stages = dbc.GuildStages
}

func (this *GuildStageManager) Get(guild_id, boss_id int32) *dbGuildStageRow {
	id := utils.Int64From2Int32(guild_id, boss_id)
	row := this.stages.GetRow(id)
	if row == nil {
		row = this.stages.AddRow(id)
	}
	return row
}

func (this *GuildStageManager) SaveDamageLog(guild_id, boss_id, attacker_id, damage int32) {
	row := this.Get(guild_id, boss_id)
	if !row.DamageLogs.HasIndex(attacker_id) {
		row.DamageLogs.Add(&dbGuildStageDamageLogData{
			AttackerId: attacker_id,
			Damage:     damage,
		})
	} else {
		row.DamageLogs.SetDamage(attacker_id, damage)
	}

	log.Debug("Saved guild %v stage %v attacker %v damage %v", guild_id, boss_id, attacker_id, damage)
}

func (this *GuildStageManager) LoadDB2RankList(guild_id, boss_id int32, rank_list *utils.ShortRankList) {
	if rank_list == nil {
		rank_list = guild_manager.GetStageDamageList(guild_id, boss_id)
	}
	row := this.Get(guild_id, boss_id)
	ids := row.DamageLogs.GetAllIndex()
	if ids != nil {
		var item GuildStageDamageItem
		for _, id := range ids {
			item.AttackerId = id
			item.Damage, _ = row.DamageLogs.GetDamage(id)
			rank_list.Update(&item, false)
		}
	}
}

func (this *GuildStageManager) RankListReward(guild_id, boss_id int32) {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	guild_stage := guild_boss_table_mgr.Get(boss_id)
	if guild_stage == nil {
		return
	}

	rank_list := guild_manager.GetStageDamageList(guild_id, boss_id)
	if rank_list == nil {
		return
	}

	var ranks [][]int32 = [][]int32{
		guild_stage.RankReward1Cond,
		guild_stage.RankReward2Cond,
		guild_stage.RankReward3Cond,
		guild_stage.RankReward4Cond,
		guild_stage.RankReward5Cond,
	}
	var rewards [][]int32 = [][]int32{
		guild_stage.RankReward1,
		guild_stage.RankReward2,
		guild_stage.RankReward3,
		guild_stage.RankReward4,
		guild_stage.RankReward5,
	}

	for i := 0; i < len(ranks); i++ {
		rank_range := ranks[i]
		if rank_range == nil {
			continue
		}

		b := false
		for r := rank_range[0]; r <= rank_range[1]; r++ {
			if r > rank_list.GetLength() {
				b = true
				break
			}
			if rewards[i] != nil {
				key, _ := rank_list.GetByRank(r)
				pid := key.(int32)
				if pid <= 0 {
					continue
				}
				RealSendMail(nil, pid, MAIL_TYPE_SYSTEM, 1107, "", "", rewards[i], 0)
			}
		}
		if b {
			break
		}
	}
}

// ----------------------------------------------------------------------------

// ----------------------------------------------------------------------------

// 获得公会副本伤害排名
func guild_stage_damage_list(guild_id, boss_id int32) (damage_list_msg []*msg_client_message.GuildStageDamageItem) {
	damage_list := guild_manager.GetStageDamageList(guild_id, boss_id)
	if damage_list == nil {
		return
	}

	length := damage_list.GetLength()
	if length > 0 {
		for r := int32(1); r <= length; r++ {
			k, v := damage_list.GetByRank(r)
			attacker_id := k.(int32)
			if attacker_id <= 0 {
				continue
			}
			name, level, head := GetPlayerBaseInfo(attacker_id)
			damage := v.(int32)
			damage_list_msg = append(damage_list_msg, &msg_client_message.GuildStageDamageItem{
				Rank:       r,
				MemberId:   attacker_id,
				MemberName: name,
				Level:      level,
				Head:       head,
				Damage:     damage,
			})
		}
	}
	return
}

// 初始化公会副本
func guild_stage_data_init(guild *dbGuildRow, boss_id int32) int32 {
	guild_stage := guild_boss_table_mgr.Get(boss_id)
	if guild_stage == nil {
		log.Error("guild stage %v not found", boss_id)
		return int32(msg_client_message.E_ERR_PLAYER_GUILD_STAGE_TABLE_DATA_NOT_FOUND)
	}
	stage_id := guild_boss_table_mgr.Array[0].StageId
	stage := stage_table_mgr.Get(stage_id)
	if stage == nil {
		log.Error("Stage %v table data not found", stage_id)
		return int32(msg_client_message.E_ERR_PLAYER_STAGE_TABLE_DATA_NOT_FOUND)
	}
	if stage.Monsters == nil || len(stage.Monsters) == 0 {
		log.Error("Stage[%v] monster list is empty", stage_id)
		return int32(msg_client_message.E_ERR_PLAYER_STAGE_TABLE_DATA_INVALID)
	}
	monster := stage.Monsters[0]
	if monster.Slot < 1 || monster.Slot > BATTLE_TEAM_MEMBER_MAX_NUM {
		log.Error("Stage[%v] monster[%v] pos %v invalid", stage_id, monster.MonsterID, monster.Slot)
		return int32(msg_client_message.E_ERR_PLAYER_STAGE_TABLE_DATA_INVALID)
	}
	guild.Stage.SetBossId(boss_id)
	guild.Stage.SetBossPos(monster.Slot - 1)
	guild.Stage.SetHpPercent(100)
	guild.Stage.SetBossHP(0)
	return 1
}

// 公会副本数据
func (this *Player) send_guild_stage_data(check_refresh bool) int32 {
	guild := guild_manager._get_guild(this.Id, false)
	if guild == nil {
		log.Error("Player[%v] get guild failed or guild not found", this.Id)
		return int32(msg_client_message.E_ERR_PLAYER_GUILD_DATA_NOT_FOUND)
	}

	if check_refresh {
		this.guild_stage_check_refresh(false)
	}

	boss_id := guild.Stage.GetBossId()
	if boss_id == 0 {
		boss_id = guild_boss_table_mgr.Array[0].Id
		res := guild_stage_data_init(guild, boss_id)
		if res < 0 {
			return res
		}
	}

	response := &msg_client_message.S2CGuildStageDataResponse{
		CurrBossId:            boss_id,
		HpPercent:             guild.Stage.GetHpPercent(),
		RespawnNum:            this.db.GuildStage.GetRespawnNum(),
		TotalRespawnNum:       _get_total_guild_stage_respawn_num(),
		RefreshRemainSeconds:  utils.GetRemainSeconds2NextDayTime(this.db.GuildStage.GetLastRefreshTime(), global_config.GuildStageRefreshTime),
		StageState:            this.db.GuildStage.GetRespawnState(),
		RespawnNeedCost:       global_config.GuildStageResurrectionGem,
		CanResetRemainSeconds: GetRemainSeconds(guild.GetLastStageResetTime(), global_config.GuildStageResetCDSecs),
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_GUILD_STAGE_DATA_RESPONSE), response)
	log.Debug("Player[%v] send guild data %v", this.Id, response)
	return 1
}

// 公会副本排行榜
func (this *Player) guild_stage_rank_list(boss_id int32) int32 {
	guild_id := this.db.Guild.GetId()
	if guild_id <= 0 {
		log.Error("Player[%v] no joined one guild")
		return int32(msg_client_message.E_ERR_PLAYER_GUILD_NOT_JOINED)
	}
	guild := guild_manager._get_guild(this.Id, false)
	if guild == nil {
		log.Error("Player[%v] cant get guild data", this.Id)
		return int32(msg_client_message.E_ERR_PLAYER_GUILD_DATA_NOT_FOUND)
	}

	if guild.Stage.GetBossId() > 0 && guild.Stage.GetBossId() < boss_id {
		log.Error("Player[%v] cant get guild stage %v rank list", this.Id, boss_id)
		return int32(msg_client_message.E_ERR_PLAYER_GUILD_STAGE_CANT_GET_DMG_RANKLIST)
	}
	damage_list := guild_stage_damage_list(guild_id, boss_id)
	response := &msg_client_message.S2CGuildStageRankListResponse{
		BossId:  boss_id,
		DmgList: damage_list,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_GUILD_STAGE_RANK_LIST_RESPONSE), response)
	log.Debug("Player[%v] guild stage %v rank list %v", this.Id, boss_id, response)
	return 1
}

const (
	GUILD_STAGE_STATE_CAN_FIGHT = iota
	GUILD_STAGE_STATE_DEAD      = 1
)

// 公会副本挑战
func (this *Player) guild_stage_fight(boss_id int32) int32 {
	if this.db.GuildStage.GetRespawnState() == GUILD_STAGE_STATE_DEAD {
		res := this.guild_stage_player_respawn()
		if res < 0 {
			return res
		}
	}

	guild_stage := guild_boss_table_mgr.Get(boss_id)
	if guild_stage == nil {
		log.Error("guild stage %v table data not found", boss_id)
		return int32(msg_client_message.E_ERR_PLAYER_GUILD_STAGE_TABLE_DATA_NOT_FOUND)
	}
	stage := stage_table_mgr.Get(guild_stage.StageId)
	if stage == nil {
		log.Error("stage %v table data not found", guild_stage.StageId)
		return int32(msg_client_message.E_ERR_PLAYER_STAGE_TABLE_DATA_INVALID)
	}

	stage_state := this.db.GuildStage.GetRespawnState()
	if stage_state == GUILD_STAGE_STATE_DEAD {
		log.Error("Player[%v] waiting to respawn for guild stage", this.Id)
		return int32(msg_client_message.E_ERR_PLAYER_GUILD_STAGE_STATE_IS_DEAD)
	} else if stage_state != GUILD_STAGE_STATE_CAN_FIGHT {
		log.Error("Player[%v] guild stage state %v invalid", stage_state)
		return -1
	}

	guild := guild_manager._get_guild(this.Id, false)
	if guild == nil {
		log.Error("Player[%v] get guild failed or guild not found", this.Id)
		return int32(msg_client_message.E_ERR_PLAYER_GUILD_DATA_NOT_FOUND)
	}

	this.guild_stage_check_refresh(false)

	guild_ex := guild_manager.GetGuildEx(guild.GetId())
	if guild_ex == nil {
		log.Error("Cant get guild ex by id %v", guild.GetId())
		return int32(msg_client_message.E_ERR_PLAYER_GUILD_STAGE_EX_DATA_NOT_FOUND)
	}

	if !guild_ex.CanStageFight() {
		log.Error("Player[%v] cant fight guild %v stage %v, there is other player fighting", this.Id, guild.GetId(), boss_id)
		return int32(msg_client_message.E_ERR_PLAYER_GUILD_STAGE_IS_FIGHTING)
	}

	curr_boss_id := guild.Stage.GetBossId()
	if boss_id != curr_boss_id {
		guild_ex.CancelStageFight()
		if boss_id > curr_boss_id {
			log.Error("Player[%v] cant fight guild stage %v", this.Id, boss_id)
			return int32(msg_client_message.E_ERR_PLAYER_GUILD_STAGE_CANT_FIGHTING)
		}
		// 返回排行榜
		return this.guild_stage_rank_list(boss_id)
	}

	err, is_win, my_team, target_team, enter_reports, rounds, has_next_wave := this.FightInStage(9, stage, nil, guild)

	guild_ex.CancelStageFight()

	if err < 0 {
		log.Error("Player[%v] fight guild stage %v failed, team is empty", this.Id, boss_id)
		return err
	}

	if is_win {
		next_guild_stage := guild_boss_table_mgr.GetNext(boss_id)
		if next_guild_stage != nil {
			// 下一副本
			err := guild_stage_data_init(guild, next_guild_stage.Id)
			if err < 0 {
				log.Error("Player[%v] fight guild stage %v win, init next stage %v failed %v", this.Id, boss_id, next_guild_stage.Id, err)
				return err
			}
		} else {
			guild.Stage.SetBossId(-1)
		}
	} else {
		// 状态置成等待复活
		stage_state = GUILD_STAGE_STATE_DEAD
		this.db.GuildStage.SetRespawnState(stage_state)
	}

	member_damages := this.guild_stage_team.common_data.members_damage
	member_cures := this.guild_stage_team.common_data.members_cure
	response := &msg_client_message.S2CBattleResultResponse{
		IsWin:               is_win,
		EnterReports:        enter_reports,
		Rounds:              rounds,
		MyTeam:              my_team,
		TargetTeam:          target_team,
		MyMemberDamages:     member_damages[this.guild_stage_team.side],
		TargetMemberDamages: member_damages[this.target_stage_team.side],
		MyMemberCures:       member_cures[this.guild_stage_team.side],
		TargetMemberCures:   member_cures[this.target_stage_team.side],
		HasNextWave:         has_next_wave,
		BattleType:          9,
		BattleParam:         boss_id,
		ExtraValue:          guild.Stage.GetHpPercent(),
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_BATTLE_RESULT_RESPONSE), response)

	if is_win && !has_next_wave {
		// 关卡奖励
		rewards := append(stage.RewardList, guild_stage.BattleReward...)
		this.send_stage_reward(rewards, 7, 0)
	} else {
		this.send_stage_reward(guild_stage.BattleReward, 7, 0)
	}

	// 更新伤害排行榜
	damage_list := guild_manager.GetStageDamageList(guild.GetId(), boss_id)
	if damage_list != nil {
		var this_fight_damage int32
		for _, dmg := range member_damages[this.guild_stage_team.side] {
			this_fight_damage += dmg
		}

		var damage_item GuildStageDamageItem = GuildStageDamageItem{
			AttackerId: this.Id,
			Damage:     this_fight_damage,
		}
		damage_list.Update(&damage_item, true)

		// 保存
		guild_stage_manager.SaveDamageLog(guild.GetId(), boss_id, this.Id, this_fight_damage)
	}

	if is_win && !has_next_wave {
		// 排名奖励
		guild_stage_manager.RankListReward(guild.GetId(), boss_id)
	}

	Output_S2CBattleResult(this, response)

	return 1
}

func _get_total_guild_stage_respawn_num() int32 {
	var total_respawn_num int32
	if global_config.GuildStageResurrectionGem != nil {
		total_respawn_num = int32(len(global_config.GuildStageResurrectionGem))
	}
	return total_respawn_num
}

// 公会副本自动刷新
func (this *Player) guild_stage_check_refresh(is_notify bool) bool {
	last_refresh := this.db.GuildStage.GetLastRefreshTime()
	if !utils.CheckDayTimeArrival(last_refresh, global_config.GuildStageRefreshTime) {
		return false
	}

	this.db.GuildStage.SetRespawnNum(0)
	this.db.GuildStage.SetLastRefreshTime(int32(time.Now().Unix()))
	this.db.GuildStage.SetRespawnState(GUILD_STAGE_STATE_CAN_FIGHT)

	if is_notify {
		this.send_guild_stage_data(false)

		var notify msg_client_message.S2CGuildStageAutoRefreshNotify
		notify.NextRefreshRemainSeconds = utils.GetRemainSeconds2NextDayTime(int32(time.Now().Unix()), global_config.GuildStageRefreshTime)
		this.Send(uint16(msg_client_message_id.MSGID_S2C_GUILD_STAGE_AUTO_REFRESH_NOTIFY), &notify)
	}

	log.Debug("Player[%v] guild stage auto refreshed", this.Id)

	return true
}

// 公会副本玩家复活
func (this *Player) guild_stage_player_respawn() int32 {
	guild := guild_manager._get_guild(this.Id, false)
	if guild == nil {
		log.Error("Player[%v] get guild failed or guild not found", this.Id)
		return int32(msg_client_message.E_ERR_PLAYER_GUILD_DATA_NOT_FOUND)
	}

	this.guild_stage_check_refresh(true)

	if this.db.GuildStage.GetRespawnState() != GUILD_STAGE_STATE_DEAD {
		log.Error("Player[%v] is no dead in guild stage, cant respawn", this.Id)
		return int32(msg_client_message.E_ERR_PLAYER_GUILD_STAGE_CANT_RESPAWN_NO_DEAD)
	}

	respawn_num := this.db.GuildStage.GetRespawnNum()

	total_respawn_num := _get_total_guild_stage_respawn_num()
	if respawn_num >= total_respawn_num {
		log.Error("Player[%v] respawn num %v is max", this.Id, respawn_num)
		return int32(msg_client_message.E_ERR_PLAYER_GUILD_STAGE_RESPAWN_NUM_USED_OUT)
	}

	need_diamond := global_config.GuildStageResurrectionGem[respawn_num]
	if this.get_diamond() < need_diamond {
		log.Error("Player[%v] respawn in guild stage not enough diamond", this.Id)
		return int32(msg_client_message.E_ERR_PLAYER_DIAMOND_NOT_ENOUGH)
	}

	this.db.GuildStage.SetRespawnState(GUILD_STAGE_STATE_CAN_FIGHT)
	respawn_num = this.db.GuildStage.IncbyRespawnNum(1)
	this.add_diamond(-need_diamond)

	var next_cost int32
	if respawn_num < total_respawn_num {
		next_cost = global_config.GuildStageResurrectionGem[respawn_num]
	}
	response := &msg_client_message.S2CGuildStagePlayerRespawnResponse{
		RemainRespawnNum: total_respawn_num - respawn_num,
		CostDiamond:      need_diamond,
		NextCost:         next_cost,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_GUILD_STAGE_PLAYER_RESPAWN_RESPONSE), response)
	log.Debug("Player[%v] respawn in guild stage %v", this.Id, response)

	return 1
}

// 公会副本重置
func (this *Player) guild_stage_reset() int32 {
	guild := guild_manager._get_guild(this.Id, true)
	if guild == nil {
		log.Error("Player[%v] cant get guild or no guild", this.Id)
		return int32(msg_client_message.E_ERR_PLAYER_GUILD_DATA_NOT_FOUND)
	}

	last_reset_time := guild.GetLastStageResetTime()
	now_time := int32(time.Now().Unix())
	if now_time-last_reset_time < global_config.GuildStageResetCDSecs {
		log.Error("Player[%v] guild stage reset is cooldown", this.Id)
		return int32(msg_client_message.E_ERR_PLAYER_GUILD_STAGE_RESET_IS_COOLDOWN)
	}

	guild.Stage.SetBossId(0)
	guild.Stage.SetBossHP(0)
	guild.Stage.SetBossPos(0)
	guild.Stage.SetHpPercent(0)
	guild.SetLastStageResetTime(now_time)

	// 清空副本排名数据
	stage_array := guild_boss_table_mgr.Array
	for i := 0; i < len(stage_array); i++ {
		damage_list := guild_manager.GetStageDamageList(guild.GetId(), stage_array[i].Id)
		if damage_list != nil {
			damage_list.Clear()
		}

		row := guild_stage_manager.Get(guild.GetId(), stage_array[i].Id)
		if row != nil {
			row.DamageLogs.Clear()
		}
	}

	response := &msg_client_message.S2CGuildStageResetResponse{
		NextResetRemainSeconds: global_config.GuildStageResetCDSecs,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_GUILD_STAGE_RESET_RESPONSE), response)

	// 重新初始化
	first_boss_id := guild_boss_table_mgr.Array[0].Id
	guild_stage_data_init(guild, first_boss_id)

	ids := guild.Members.GetAllIndex()
	if ids != nil {
		var notify msg_client_message.S2CGuildStageResetNotify
		notify.NextResetRemainSeconds = global_config.GuildStageResetCDSecs
		for _, id := range ids {
			if id == this.Id {
				continue
			}
			player := player_mgr.GetPlayerById(id)
			if player == nil {
				continue
			}
			player.Send(uint16(msg_client_message_id.MSGID_S2C_GUILD_STAGE_RESET_NOTIFY), &notify)
			log.Debug("Notify player[%v] guild stage reset", id)
		}
	}

	log.Debug("Player[%v] reset guild stage", this.Id)

	return 1
}

func C2SGuildStageDataHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SGuildStageDataRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.send_guild_stage_data(true)
}

func C2SGuildStageRankListHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SGuildStageRankListRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%v)", err.Error())
		return -1
	}
	return p.guild_stage_rank_list(req.GetBossId())
}

func C2SGuildStagePlayerRespawnHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SGuildStagePlayerRespawnRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%v)", err.Error())
		return -1
	}
	return p.guild_stage_player_respawn()
}

func C2SGuildStageResetHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SGuildStageResetRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%v)", err.Error())
		return -1
	}
	return p.guild_stage_reset()
}
