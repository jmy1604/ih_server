package main

import (
	"ih_server/libs/log"
	"ih_server/libs/utils"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"ih_server/src/table_config"
	_ "math/rand"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
)

const (
	ARENA_RANK_MAX = 100000
)

var arena_serial_id int32

type ArenaRankItem struct {
	SerialId    int32
	PlayerScore int32
	PlayerId    int32
}

func (this *ArenaRankItem) Less(value interface{}) bool {
	item := value.(*ArenaRankItem)
	if item == nil {
		return false
	}
	if this.PlayerScore < item.PlayerScore {
		return true
	}
	if this.PlayerScore == item.PlayerScore {
		if this.SerialId > item.SerialId {
			return true
		}
	}
	return false
}

func (this *ArenaRankItem) Greater(value interface{}) bool {
	item := value.(*ArenaRankItem)
	if item == nil {
		return false
	}
	if this.PlayerScore > item.PlayerScore {
		return true
	}
	if this.PlayerScore == item.PlayerScore {
		if this.SerialId < item.SerialId {
			return true
		}
	}
	return false
}

func (this *ArenaRankItem) KeyEqual(value interface{}) bool {
	item := value.(*ArenaRankItem)
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

func (this *ArenaRankItem) GetKey() interface{} {
	return this.PlayerId
}

func (this *ArenaRankItem) GetValue() interface{} {
	return this.PlayerScore
}

func (this *ArenaRankItem) SetValue(value interface{}) {
	this.PlayerScore = value.(int32)
	this.SerialId = atomic.AddInt32(&arena_serial_id, 1)
}

func (this *ArenaRankItem) New() utils.SkiplistNode {
	return &ArenaRankItem{}
}

func (this *ArenaRankItem) Assign(node utils.SkiplistNode) {
	n := node.(*ArenaRankItem)
	if n == nil {
		return
	}
	this.PlayerId = n.PlayerId
	this.PlayerScore = n.PlayerScore
	this.SerialId = n.SerialId
}

func (this *ArenaRankItem) CopyDataTo(node interface{}) {
	n := node.(*ArenaRankItem)
	if n == nil {
		return
	}
	n.PlayerId = this.PlayerId
	n.PlayerScore = this.PlayerScore
	n.SerialId = this.SerialId
}

type ArenaRobot struct {
	robot_data *table_config.XmlArenaRobotItem
	//defense_team *BattleTeam
	power int32
}

func (this *ArenaRobot) Init(robot *table_config.XmlArenaRobotItem) {
	this.robot_data = robot
	//this.defense_team = &BattleTeam{}
	this._calculate_power()
}

func (this *ArenaRobot) _calculate_power() {
	card_list := this.robot_data.RobotCardList
	if card_list == nil {
		return
	}

	for i := 0; i < len(card_list); i++ {
		for j := 0; j < len(card_list[i].EquipID); j++ {
			equip_item := item_table_mgr.Get(card_list[i].EquipID[j])
			if equip_item == nil {
				continue
			}
			this.power += equip_item.BattlePower
		}
		card := card_table_mgr.GetRankCard(card_list[i].MonsterID, card_list[i].Rank)
		if card != nil {
			this.power += calc_power_by_card(card, card_list[i].Level)
		}
	}
}

type ArenaRobotManager struct {
	robots map[int32]*ArenaRobot
}

var arena_robot_mgr ArenaRobotManager

func (this *ArenaRobotManager) Init() {
	array := arena_robot_table_mgr.Array
	if array == nil {
		return
	}

	this.robots = make(map[int32]*ArenaRobot)
	for _, r := range array {
		robot := &ArenaRobot{}
		robot.Init(r)
		this.robots[r.Id] = robot
		// 加入排行榜
		var d = ArenaRankItem{
			SerialId:    atomic.AddInt32(&arena_serial_id, 1),
			PlayerScore: r.RobotScore,
			PlayerId:    r.Id,
		}
		rank_list_mgr.UpdateItem(RANK_LIST_TYPE_ARENA, &d)
	}
}

func (this *ArenaRobotManager) Get(robot_id int32) *ArenaRobot {
	return this.robots[robot_id]
}

func (this *Player) check_arena_tickets_refresh() (remain_seconds int32) {
	last_refresh := this.db.Arena.GetLastTicketsRefreshTime()
	remain_seconds = utils.GetRemainSeconds2NextDayTime(last_refresh, global_config.ArenaTicketRefreshTime)
	if remain_seconds > 0 {
		return
	}
	this.set_resource(global_config.ArenaTicketItemId, global_config.ArenaTicketsDay)
	this.db.Arena.SetLastTicketsRefreshTime(int32(time.Now().Unix()))
	return
}

func (this *Player) _update_arena_score(data *ArenaRankItem) {
	rank_list_mgr.UpdateItem(RANK_LIST_TYPE_ARENA, data)
}

func (this *Player) LoadArenaScore() {
	score := this.db.Arena.GetScore()
	if score <= 0 {
		return
	}
	sid := this.db.Arena.GetSerialId()
	if sid == 0 {
		arena_serial_id += 1
		sid = arena_serial_id
	}
	if arena_serial_id < sid {
		arena_serial_id = sid
	}
	var data = ArenaRankItem{
		SerialId:    sid,
		PlayerScore: score,
		PlayerId:    this.Id,
	}

	this._update_arena_score(&data)
}

func (this *Player) UpdateArenaScore(is_win bool) (score, add_score int32) {
	now_score := this.db.Arena.GetScore()
	division := arena_division_table_mgr.GetByScore(now_score)
	if division == nil {
		log.Error("Arena division table data with score[%v] is not found", now_score)
		return
	}

	if is_win {
		add_score = division.WinScore
		if this.db.Arena.GetRepeatedWinNum() >= global_config.ArenaRepeatedWinNum {
			add_score += division.WinningStreakScoreBonus
		}
		if this.db.Arena.GetRepeatedLoseNum() > 0 {
			this.db.Arena.SetRepeatedLoseNum(0)
		}
		this.db.Arena.IncbyRepeatedWinNum(1)
	} else {
		add_score = division.LoseScore
	}

	if add_score > 0 {
		now_time := int32(time.Now().Unix())
		score = this.db.Arena.IncbyScore(add_score)
		this.db.Arena.SetUpdateScoreTime(now_time)

		var data = ArenaRankItem{
			SerialId:    atomic.AddInt32(&arena_serial_id, 1),
			PlayerScore: score,
			PlayerId:    this.Id,
		}
		this._update_arena_score(&data)

		top_rank := this.db.Arena.GetHistoryTopRank()
		rank := rank_list_mgr.GetRankByKey(RANK_LIST_TYPE_ARENA, this.Id)
		if rank < top_rank {
			this.db.Arena.SetHistoryTopRank(rank)
		}

		// 段位奖励
		new_division := arena_division_table_mgr.GetByScore(score)
		if new_division != nil && new_division.Id > division.Id {
			SendMail2(nil, this.Id, MAIL_TYPE_SYSTEM, "Arena Grade Reward", "", new_division.RewardList)
			notify := &msg_client_message.S2CArenaGradeRewardNotify{
				Grade: new_division.Id,
			}
			this.Send(uint16(msg_client_message_id.MSGID_S2C_ARENA_GRADE_REWARD_NOTIFY), notify)
		}

		// 更新任务
		this.TaskUpdate(table_config.TASK_COMPLETE_TYPE_ARENA_REACH_SCORE, true, score, 1)
	}

	return
}

func (this *Player) OutputArenaRankItems(rank_start, rank_num int32) {
	rank_items, self_rank, self_value := rank_list_mgr.GetItemsByRange(RANK_LIST_TYPE_ARENA, this.Id, rank_start, rank_num)
	if rank_items == nil {
		log.Warn("Player[%v] get rank list with range[%v,%v] failed", this.Id, rank_start, rank_num)
		return
	}

	l := int32(len(rank_items))
	for rank := rank_start; rank < l; rank++ {
		item := (rank_items[rank-rank_start]).(*ArenaRankItem)
		if item == nil {
			log.Error("Player[%v] get arena rank list by rank[%v] item failed")
			continue
		}
		log.Debug("Rank: %v   Player[%v] Score[%v]", rank, item.PlayerId, item.PlayerScore)
	}

	if self_value != nil && self_rank > 0 {
		log.Debug("Player[%v] score[%v] rank[%v]", this.Id, self_value.(int32), self_rank)
	}
}

// 匹配对手
func (this *Player) MatchArenaPlayer() (player_id int32) {
	rank := rank_list_mgr.GetRankByKey(RANK_LIST_TYPE_ARENA, this.Id)
	if rank < 0 {
		log.Error("Player[%v] get arena rank list rank failed", this.Id)
		return
	}

	var start_rank, rank_num, last_rank int32
	match_num := global_config.ArenaMatchPlayerNum
	if rank == 0 {
		start_rank, rank_num = rank_list_mgr.GetLastRankRange(RANK_LIST_TYPE_ARENA, match_num)
		if start_rank < 0 {
			log.Error("Player[%v] match arena player failed", this.Id)
			return
		}
	} else {
		last_rank, _ = rank_list_mgr.GetLastRankRange(RANK_LIST_TYPE_ARENA, 1)
		half_num := match_num / 2
		var left, right int32
		if this.db.Arena.GetRepeatedWinNum() >= global_config.ArenaRepeatedWinNum {
			right = rank - 1
			left = rank - match_num
		} else if this.db.Arena.GetRepeatedLoseNum() >= global_config.ArenaLoseRepeatedNum {
			right = rank + match_num
			left = rank + 1
		} else {
			right = rank + half_num
			left = rank - half_num
		}

		if left < 1 {
			left = 1
		}

		if right > last_rank {
			right = last_rank
		}

		if right-left < match_num {
			if left > 1 {
				left -= (match_num - (right - left))
				if left < 1 {
					left = 1
				}
			} else {
				if right < last_rank {
					right += (match_num - (right - left))
				}
				if right > last_rank {
					right = last_rank
				}
			}
		}

		start_rank = left
		rank_num = right - start_rank + 1
	}

	_, r := rand31n_from_range(start_rank, start_rank+rank_num)
	// 如果是自己
	if rank == r {
		r += 1
		if r > last_rank {
			r -= 2
		}
	}
	item := rank_list_mgr.GetItemByRank(RANK_LIST_TYPE_ARENA, r)
	if item == nil {
		log.Error("Player[%v] match arena player by random rank[%v] get empty item", this.Id, r)
		return
	}

	player_id = item.(*ArenaRankItem).PlayerId

	log.Debug("Player[%v] match arena players rank range [start:%v, num:%v], rand the rank %v, match player[%v]", this.Id, start_rank, rank_num, r, player_id)

	return
}

func (this *Player) send_arena_data() int32 {
	tickets_remain := this.check_arena_tickets_refresh()
	if tickets_remain > 0 {

	}

	score := this.db.Arena.GetScore()
	top_rank := this.db.Arena.GetHistoryTopRank()
	rank := rank_list_mgr.GetRankByKey(RANK_LIST_TYPE_ARENA, this.Id)
	day_remain, season_remain := arena_season_mgr.GetRemainSeconds()
	response := &msg_client_message.S2CArenaDataResponse{
		Score:                       score,
		Grade:                       arena_division_table_mgr.GetGradeByScore(score),
		Rank:                        rank,
		TopRank:                     top_rank,
		DayRemainSeconds:            day_remain,
		SeasonRemainSeconds:         season_remain,
		TicketsRefreshRemainSeconds: tickets_remain,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_ARENA_DATA_RESPONSE), response)
	log.Debug("Player[%v] arena data: %v", this.Id, response)
	return 1
}

func (this *Player) arena_player_defense_team(player_id int32) int32 {
	if this.check_arena_tickets_refresh() > 0 {

	}

	var level, head, guild_id int32
	var defense_team []int32
	var robot *ArenaRobot
	p := player_mgr.GetPlayerById(player_id)
	if p == nil {
		robot = arena_robot_mgr.Get(player_id)
		if robot == nil {
			log.Error("Player[%v] not found", player_id)
			return int32(msg_client_message.E_ERR_PLAYER_NOT_EXIST)
		}
		level = 0
		head = robot.robot_data.RobotHead
		guild_id = 0
	} else {
		defense_team = p.db.BattleTeam.GetDefenseMembers()
		level = p.db.Info.GetLvl()
		head = p.db.Info.GetHead()
		guild_id = 0
	}

	var power int32
	var team []*msg_client_message.PlayerTeamRole
	if defense_team != nil {
		for i := 0; i < len(defense_team); i++ {
			m := defense_team[i]
			if m <= 0 {
				continue
			}
			table_id, _ := p.db.Roles.GetTableId(m)
			level, _ := p.db.Roles.GetLevel(m)
			rank, _ := p.db.Roles.GetRank(m)
			team = append(team, &msg_client_message.PlayerTeamRole{
				TableId: table_id,
				Pos:     int32(i),
				Level:   level,
				Rank:    rank,
			})
		}
		power = p.get_defense_team_power()
	} else {
		for i := 0; i < len(robot.robot_data.RobotCardList); i++ {
			m := robot.robot_data.RobotCardList[i]
			if m == nil {
				continue
			}
			team = append(team, &msg_client_message.PlayerTeamRole{
				TableId: m.MonsterID,
				Pos:     m.Slot - 1,
				Level:   m.Level,
				Rank:    m.Rank,
			})
		}
		power = robot.power
	}

	response := &msg_client_message.S2CArenaPlayerDefenseTeamResponse{
		PlayerId:    player_id,
		PlayerLevel: level,
		PlayerHead:  head,
		DefenseTeam: team,
		Power:       power,
		GuildId:     guild_id,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_ARENA_PLAYER_DEFENSE_TEAM_RESPONSE), response)
	log.Debug("Player[%v] get arena player[%v] defense team[%v]", this.Id, player_id, team)
	return 1
}

func (this *Player) arena_match() int32 {
	if this.check_arena_tickets_refresh() > 0 {

	}
	if this.get_resource(global_config.ArenaTicketItemId) < 1 {
		log.Error("Player[%v] match arena player failed, ticket not enough", this.Id)
		return int32(msg_client_message.E_ERR_PLAYER_ARENA_TICKETS_NOT_ENOUGH)
	}

	pid := this.MatchArenaPlayer()

	var robot *ArenaRobot
	p := player_mgr.GetPlayerById(pid)
	if p == nil {
		robot = arena_robot_mgr.Get(pid)
		if robot == nil {
			log.Error("Player[%v] matched id[%v] is not player and not robot", this.Id, pid)
			return int32(msg_client_message.E_ERR_PLAYER_ARENA_MATCH_PLAYER_FAILED)
		}
	}

	// 当前匹配到的玩家
	this.db.Arena.SetMatchedPlayerId(pid)
	this.add_resource(global_config.ArenaTicketItemId, -1)

	name, level, head, score, grade, power := GetFighterInfo(pid)
	response := &msg_client_message.S2CArenaMatchPlayerResponse{
		PlayerId:    pid,
		PlayerName:  name,
		PlayerLevel: level,
		PlayerHead:  head,
		PlayerScore: score,
		PlayerGrade: grade,
		PlayerPower: power,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_ARENA_MATCH_PLAYER_RESPONSE), response)
	log.Debug("Player[%v] matched arena player[%v]", this.Id, response)
	return 1
}

func C2SArenaDataHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SArenaDataRequest
	err := proto.Unmarshal(msg_data, &req)
	if nil != err {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.send_arena_data()
}

func C2SArenaPlayerDefenseTeamHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SArenaPlayerDefenseTeamRequest
	err := proto.Unmarshal(msg_data, &req)
	if nil != err {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.arena_player_defense_team(req.GetPlayerId())
}

func C2SArenaMatchPlayerHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SArenaMatchPlayerRequest
	err := proto.Unmarshal(msg_data, &req)
	if nil != err {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.arena_match()
}

const (
	ARENA_STATE_IDLE          = iota
	ARENA_STATE_DOING         = 1
	ARENA_STATE_DAY_REWARD    = 2
	ARENA_STATE_SEASON_REWARD = 3
)

// 竞技场赛季管理
type ArenaSeasonMgr struct {
	state          int32 // 0 结束  1 开始
	start_time     int32
	day_checker    *utils.DaysTimeChecker
	season_checker *utils.DaysTimeChecker
	to_exit        int32
}

var arena_season_mgr ArenaSeasonMgr

func (this *ArenaSeasonMgr) Init() bool {
	this.day_checker = &utils.DaysTimeChecker{}
	if !this.day_checker.Init("15:04:05", global_config.ArenaDayResetTime, 1) {
		log.Error("ArenaSeasonMgr day checker init failed")
		return false
	}
	this.season_checker = &utils.DaysTimeChecker{}
	if !this.season_checker.Init("15:04:05", global_config.ArenaSeasonResetTime, global_config.ArenaSeasonDays) {
		log.Error("ArenaSeasonMgr season checker init failed")
		return false
	}
	return true
}

func (this *ArenaSeasonMgr) ToEnd() {
	atomic.StoreInt32(&this.to_exit, 1)
}

func (this *ArenaSeasonMgr) SeasonStart() {
	for {
		if !atomic.CompareAndSwapInt32(&this.state, ARENA_STATE_IDLE, ARENA_STATE_DOING) {
			time.Sleep(time.Second * 1)
			continue
		}
		break
	}
	this.start_time = int32(time.Now().Unix())
}

func (this *ArenaSeasonMgr) SeasonEnd() {
	atomic.StoreInt32(&this.state, ARENA_STATE_IDLE)
}

func (this *ArenaSeasonMgr) IsSeasonStart() bool {
	return atomic.LoadInt32(&this.state) == ARENA_STATE_DOING
}

func (this *ArenaSeasonMgr) IsSeasonEnd() bool {
	return atomic.LoadInt32(&this.state) == ARENA_STATE_IDLE
}

func (this *ArenaSeasonMgr) GetRemainSeconds() (day_remain int32, season_remain int32) {
	now_time := int32(time.Now().Unix())
	day_set := dbc.ArenaSeason.GetRow().Data.GetLastDayResetTime()
	if day_set == 0 {
		dbc.ArenaSeason.GetRow().Data.SetLastDayResetTime(now_time)
		day_set = now_time
	}
	season_set := dbc.ArenaSeason.GetRow().Data.GetLastSeasonResetTime()
	if season_set == 0 {
		dbc.ArenaSeason.GetRow().Data.SetLastSeasonResetTime(now_time)
		season_set = now_time
	}

	day_remain = this.day_checker.RemainSecondsToNextRefresh(day_set)
	season_remain = this.season_checker.RemainSecondsToNextRefresh(season_set)

	return
}

func (this *ArenaSeasonMgr) IsRewardArrive(now_time int32) (day_arrive, season_arrive bool) {
	day_set := dbc.ArenaSeason.GetRow().Data.GetLastDayResetTime()
	if day_set == 0 {
		dbc.ArenaSeason.GetRow().Data.SetLastDayResetTime(now_time)
		day_set = now_time
	}
	season_set := dbc.ArenaSeason.GetRow().Data.GetLastSeasonResetTime()
	if season_set == 0 {
		dbc.ArenaSeason.GetRow().Data.SetLastSeasonResetTime(now_time)
		season_set = now_time
	}
	day_arrive = this.day_checker.IsArrival(day_set)
	season_arrive = this.season_checker.IsArrival(season_set)
	return
}

func (this *ArenaSeasonMgr) Reward(typ int32) {
	rank_list := rank_list_mgr.GetRankList(RANK_LIST_TYPE_ARENA)
	if rank_list == nil {
		return
	}
	rank_num := rank_list.RankNum()
	for rank := int32(1); rank <= rank_num; rank++ {
		item := rank_list.GetItemByRank(rank)
		if item == nil {
			log.Warn("Cant found rank[%v] item in arena rank list with reset", rank)
			continue
		}
		arena_item := item.(*ArenaRankItem)
		if arena_item == nil {
			log.Warn("Arena rank[%v] item convert failed on DayReward", rank)
			continue
		}

		bonus := arena_bonus_table_mgr.GetByRank(rank)
		if bonus == nil {
			log.Warn("Arena rank[%v] item get bonus failed on DayReward", rank)
			continue
		}

		p := player_mgr.GetPlayerById(arena_item.PlayerId)
		if p == nil {
			continue
		}

		if typ == 1 {
			SendMail2(nil, arena_item.PlayerId, MAIL_TYPE_SYSTEM, "Arena Day Reward", "", bonus.DayRewardList)
		} else if typ == 2 {
			SendMail2(nil, arena_item.PlayerId, MAIL_TYPE_SYSTEM, "Arena Season Reward", "", bonus.SeasonRewardList)
		}
	}
}

func (this *ArenaSeasonMgr) Reset() {
	for {
		// 等待直到结束
		if atomic.LoadInt32(&this.state) == 1 {
			time.Sleep(time.Second * 1)
			continue
		}
		break
	}

	rank_list := rank_list_mgr.GetRankList(RANK_LIST_TYPE_ARENA)
	if rank_list == nil {
		return
	}

	atomic.StoreInt32(&arena_serial_id, 0)
	rank_num := rank_list.RankNum()
	for rank := int32(1); rank <= rank_num; rank++ {
		item := rank_list.GetItemByRank(rank)
		if item == nil {
			log.Warn("Cant found rank[%v] item in arena rank list with reset", rank)
			continue
		}
		arena_item := item.(*ArenaRankItem)
		if arena_item == nil {
			log.Warn("Arena rank[%v] item convert failed", rank)
			continue
		}
		division := arena_division_table_mgr.GetByScore(arena_item.PlayerScore)
		if division == nil {
			log.Error("arena division not found by player[%v] score[%v]", arena_item.PlayerId, arena_item.PlayerScore)
			continue
		}
		rank_list.SetValueByKey(arena_item.PlayerId, division.NewSeasonScore)
		p := player_mgr.GetPlayerById(arena_item.PlayerId)
		if p != nil {
			p.db.Arena.SetScore(division.NewSeasonScore)
		} else {
			robot := arena_robot_mgr.Get(arena_item.PlayerId)
			if robot != nil {
				robot.robot_data.RobotScore = division.NewSeasonScore
			}
		}
	}
}

func (this *ArenaSeasonMgr) Run() {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	this.SeasonStart()

	for {
		if atomic.LoadInt32(&this.to_exit) > 0 {
			break
		}
		// 检测时间
		now_time := int32(time.Now().Unix())
		day_arrive, season_arrive := this.IsRewardArrive(now_time)
		if day_arrive {
			atomic.StoreInt32(&this.state, ARENA_STATE_DAY_REWARD)
			this.Reward(1)
			dbc.ArenaSeason.GetRow().Data.SetLastDayResetTime(now_time)
			this.day_checker.ToNextTimePoint()
			atomic.StoreInt32(&this.state, ARENA_STATE_DOING)
			log.Info("Arena Day Reward")
		}

		if season_arrive {
			// 发奖
			atomic.StoreInt32(&this.state, ARENA_STATE_SEASON_REWARD)
			this.Reward(2)
			dbc.ArenaSeason.GetRow().Data.SetLastSeasonResetTime(now_time)
			this.season_checker.ToNextTimePoint()
			atomic.StoreInt32(&this.state, ARENA_STATE_IDLE)
			log.Info("Arena Season Reward")

			// 重置
			this.Reset()
			this.SeasonStart()
			time.Sleep(time.Second * 1)
			continue
		}

		time.Sleep(time.Second * 2)
	}
}
