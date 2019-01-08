package main

import (
	"ih_server/libs/log"
	"ih_server/libs/utils"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	_ "ih_server/src/table_config"
	"time"

	"github.com/golang/protobuf/proto"
)

const (
	EXPEDITION_MATCH_LEVELS_NUM = 10
)

func (this *Player) get_expedition_db_role_list() []*dbPlayerExpeditionLevelRoleColumn {
	return []*dbPlayerExpeditionLevelRoleColumn{
		&this.db.ExpeditionLevelRole0s,
		&this.db.ExpeditionLevelRole1s,
		&this.db.ExpeditionLevelRole2s,
		&this.db.ExpeditionLevelRole3s,
		&this.db.ExpeditionLevelRole4s,
		&this.db.ExpeditionLevelRole5s,
		&this.db.ExpeditionLevelRole6s,
		&this.db.ExpeditionLevelRole7s,
		&this.db.ExpeditionLevelRole8s,
		&this.db.ExpeditionLevelRole9s,
	}
}

func (this *Player) get_curr_expedition_db_roles() *dbPlayerExpeditionLevelRoleColumn {
	curr_level := this.db.ExpeditionData.GetCurrLevel()
	role_list := this.get_expedition_db_role_list()
	return role_list[curr_level]
}

func (this *Player) get_curr_expedition_max_role_num() int32 {
	if this.db.ExpeditionData.GetRefreshTime() == 0 {
		return 0
	}
	curr_level := this.db.ExpeditionData.GetCurrLevel()
	return expedition_table_mgr.Array[curr_level].PlayerCardMax
}

func (this *Player) MatchExpeditionPlayer() int32 {
	arr := expedition_table_mgr.Array
	if len(arr) < int(EXPEDITION_MATCH_LEVELS_NUM) {
		log.Error("Expedition level %v not enough", len(arr))
		return -1
	}

	db_expe_list := this.get_expedition_db_role_list()
	if len(db_expe_list) < len(arr) {
		log.Error("Player %v not enough expedition level role db column", this.Id)
		return -1
	}

	self_node := rank_list_mgr.GetItemByKey(RANK_LIST_TYPE_ROLE_POWER, this.Id)
	n := self_node.(*RolesPowerRankItem)
	if n == nil {
		log.Error("Player[%v] no data in Role power rank list", this.Id)
		return -1
	}

	log.Debug("@@@@@@@@@@@ Player %v roles power %v", this.Id, n.Power)

	var player_ids []int32
	var player_powers []int32
	for i := 0; i < len(arr); i++ {
		power := int32(float32(n.Power) * (float32(arr[i].EnemyBattlePower) / 10000))
		pid := top_power_ranklist.GetNearestRandPlayer(power)
		player := player_mgr.GetPlayerById(pid)
		if player == nil {
			log.Error("Not found player %v by match expedition with level %v power %v for player %v", pid, i+1, power, this.Id)
			continue
		}

		// 防守阵型
		dm := player.db.BattleTeam.GetDefenseMembers()
		if dm == nil || len(dm) == 0 {
			log.Error("Player %v matched expedition player %v defense team is empty", this.Id, pid)
			continue
		}

		if db_expe_list[i].NumAll() > 0 {
			db_expe_list[i].Clear()
		}

		for pos, id := range dm {
			if id <= 0 {
				continue
			}
			if player.db.Roles.HasIndex(id) {
				table_id, _ := player.db.Roles.GetTableId(id)
				level, _ := player.db.Roles.GetLevel(id)
				rank, _ := player.db.Roles.GetRank(id)
				equip, _ := player.db.Roles.GetEquip(id)
				db_expe_list[i].Add(&dbPlayerExpeditionLevelRoleData{
					Pos:       int32(pos),
					TableId:   table_id,
					Rank:      rank,
					Level:     level,
					Equip:     equip,
					HP:        -1,
					HpPercent: 100,
				})
			}
		}
		player_ids = append(player_ids, pid)
		player_powers = append(player_powers, player.get_defense_team_power())
	}

	this.db.ExpeditionData.SetCurrLevel(0)
	this.db.ExpeditionData.SetRefreshTime(int32(time.Now().Unix()))
	this.db.ExpeditionData.SetPlayerIds(player_ids)
	this.db.ExpeditionData.SetPlayerPowers(player_powers)

	if this.db.ExpeditionRoles.NumAll() > 0 {
		this.db.ExpeditionRoles.Clear()
	}

	return 1
}

func (this *Player) send_expedition_data() int32 {
	refresh_time := this.db.ExpeditionData.GetRefreshTime()
	remain_seconds := utils.GetRemainSeconds2NextDayTime(refresh_time, global_config.ExpeditionRefreshTime)
	if remain_seconds <= 0 {
		res := this.MatchExpeditionPlayer()
		if res < 0 {
			return res
		}
		remain_seconds = utils.GetRemainSeconds2NextDayTime(int32(time.Now().Unix()), global_config.ExpeditionRefreshTime)
	}

	curr_level := this.db.ExpeditionData.GetCurrLevel()
	used_ids := this.db.ExpeditionRoles.GetAllIndex()
	var roles []*msg_client_message.ExpeditionSelfRole
	if used_ids != nil {
		for _, id := range used_ids {
			hp, _ := this.db.ExpeditionRoles.GetHP(id)
			roles = append(roles, &msg_client_message.ExpeditionSelfRole{
				Id: id,
				HP: hp,
			})
		}
	}

	response := &msg_client_message.S2CExpeditionDataResponse{
		CurrLevel:            curr_level,
		RemainRefreshSeconds: remain_seconds,
		Roles:                roles,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_EXPEDITION_DATA_RESPONSE), response)

	log.Trace("Player %v expedition data %v", this.Id, response)

	return 1
}

func (this *Player) get_expedition_level_data_with_level(curr_level int32) int32 {
	player_ids := this.db.ExpeditionData.GetPlayerIds()
	player_powers := this.db.ExpeditionData.GetPlayerPowers()
	if player_ids == nil || len(player_ids) <= int(curr_level) || player_powers == nil || len(player_powers) <= int(curr_level) {
		log.Error("Player %v expedition enemy list length %v not enough", this.Id, len(player_ids))
		return -1
	}
	player := player_mgr.GetPlayerById(player_ids[curr_level])
	if player == nil {
		log.Error("Player %v not found expedition player %v data", this.Id, player_ids[curr_level])
		return -1
	}

	db_expe_list := this.get_expedition_db_role_list()
	all_pos := db_expe_list[curr_level].GetAllIndex()
	if all_pos == nil || len(all_pos) == 0 {
		log.Error("Player %v expedition level %v enemy role list is empty", this.Id, curr_level)
		return -1
	}

	var role_list []*msg_client_message.ExpeditionEnemyRole
	for _, pos := range all_pos {
		table_id, _ := db_expe_list[curr_level].GetTableId(pos)
		rank, _ := db_expe_list[curr_level].GetRank(pos)
		level, _ := db_expe_list[curr_level].GetLevel(pos)
		hp_percent, _ := db_expe_list[curr_level].GetHpPercent(pos)
		role_list = append(role_list, &msg_client_message.ExpeditionEnemyRole{
			Position:  pos,
			TableId:   table_id,
			Rank:      rank,
			Level:     level,
			HpPercent: hp_percent,
		})
	}

	response := &msg_client_message.S2CExpeditionLevelDataResponse{
		PlayerId:       player_ids[curr_level],
		PlayerName:     player.db.GetName(),
		PlayerLevel:    player.db.GetLevel(),
		PlayerVipLevel: player.db.Info.GetVipLvl(),
		PlayerHead:     player.db.Info.GetHead(),
		PlayerPower:    player_powers[curr_level],
		RoleList:       role_list,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_EXPEDITION_LEVEL_DATA_RESPONSE), response)

	log.Trace("Player %v get expedition level %v data %v", this.Id, curr_level, response)

	return 1
}

func (this *Player) get_expedition_level_data() int32 {
	if this.db.ExpeditionData.GetRefreshTime() == 0 {
		return -1
	}
	curr_level := this.db.ExpeditionData.GetCurrLevel()
	if int(curr_level) >= len(expedition_table_mgr.Array) {
		log.Error("Player %v curr expedition level %v invalid", this.Id)
		return -1
	}
	return this.get_expedition_level_data_with_level(curr_level)
}

func (this *Player) expedition_team_init(members []*TeamMember) bool {
	if members == nil {
		return false
	}

	for _, m := range members {
		if m == nil {
			continue
		}
		if this.db.ExpeditionRoles.HasIndex(m.id) {
			hp, _ := this.db.ExpeditionRoles.GetHP(m.id)
			if hp <= 0 {
				log.Warn("Player %v expedition role %v no hp, cant use", this.Id, m.id)
				return false
			}
			if hp >= 0 {
				m.hp = hp
				m.attrs[hp] = hp
			}
		}
	}

	return true
}

func (this *Player) expedition_update_self_roles(members []*TeamMember) {
	for pos := 0; pos < len(members); pos++ {
		m := members[pos]
		if m == nil {
			continue
		}
		id := m.id
		hp := m.hp
		if m.is_dead() {
			hp = 0
		}
		if !this.db.ExpeditionRoles.HasIndex(id) {
			this.db.ExpeditionRoles.Add(&dbPlayerExpeditionRoleData{
				Id: id,
				HP: hp,
			})
		} else {
			this.db.ExpeditionRoles.SetHP(id, hp)
		}
	}
}

func (this *Player) expedition_update_enemy_roles(members []*TeamMember) {
	db_roles := this.get_curr_expedition_db_roles()
	for pos := 0; pos < len(members); pos++ {
		m := members[pos]
		if m == nil {
			continue
		}
		if !db_roles.HasIndex(int32(pos)) {
			continue
		}

		hp := m.hp
		if m.is_dead() {
			db_roles.Remove(int32(pos))
		}
		hp_percent := 100 * hp / m.attrs[ATTR_HP_MAX]
		db_roles.SetHpPercent(int32(pos), hp_percent)
		db_roles.SetHP(int32(pos), hp)
	}
}

func (this *Player) expedition_fight() int32 {
	if this.db.ExpeditionData.GetRefreshTime() == 0 {
		return -1
	}

	curr_level := this.db.ExpeditionData.GetCurrLevel()
	if int(curr_level) >= len(expedition_table_mgr.Array) {
		log.Error("Player %v already pass all level expedition", this.Id)
		return -1
	}

	if this.expedition_team == nil {
		this.expedition_team = &BattleTeam{}
	}

	res := this.expedition_team.Init(this, BATTLE_TEAM_EXPEDITION, 0)
	if res < 0 {
		log.Error("Player[%v] init expedition team failed, err %v", this.Id, res)
		return res
	}

	if this.expedition_enemy_team == nil {
		this.expedition_enemy_team = &BattleTeam{}
	}
	if !this.expedition_enemy_team.InitExpeditionEnemy(this) {
		log.Error("Player[%v] init expedition enemy team failed", this.Id)
		return res
	}

	team_format := this.expedition_team._format_members_for_msg()
	enemy_team_format := this.expedition_enemy_team._format_members_for_msg()

	// To Fight
	is_win, enter_reports, rounds := this.expedition_team.Fight(this.expedition_enemy_team, BATTLE_END_BY_ALL_DEAD, 0)

	if is_win {
		curr_level = this.db.ExpeditionData.IncbyCurrLevel(1)
	}

	members_damage := this.expedition_team.common_data.members_damage
	members_cure := this.expedition_team.common_data.members_cure
	response := &msg_client_message.S2CBattleResultResponse{
		IsWin:               is_win,
		EnterReports:        enter_reports,
		Rounds:              rounds,
		MyTeam:              team_format,
		TargetTeam:          enemy_team_format,
		MyMemberDamages:     members_damage[this.expedition_team.side],
		TargetMemberDamages: members_damage[this.expedition_enemy_team.side],
		MyMemberCures:       members_cure[this.expedition_team.side],
		TargetMemberCures:   members_cure[this.expedition_enemy_team.side],
		BattleType:          10,
		BattleParam:         0,
		MySpeedBonus:        this.expedition_team.first_hand,
		TargetSpeedBonus:    this.expedition_enemy_team.first_hand,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_BATTLE_RESULT_RESPONSE), response)

	if is_win {
		this.Send(uint16(msg_client_message_id.MSGID_S2C_EXPEDITION_CURR_LEVEL_SYNC), &msg_client_message.S2CExpeditionCurrLevelSync{
			CurrLevel: curr_level,
		})
	}

	log.Trace("Player %v expedition fight %v", this.Id, response)

	return 1
}

func C2SExpeditionDataHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SExpeditionDataRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}
	return p.send_expedition_data()
}

func C2SExpeditionLevelDataHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SExpeditionLevelDataRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}
	return p.get_expedition_level_data()
}
