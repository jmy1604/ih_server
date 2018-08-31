package main

import (
	"ih_server/libs/log"
	"ih_server/libs/utils"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"ih_server/src/table_config"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
)

const (
	INIT_PLAYER_MSG_NUM = 10
	MSG_ITEM_HEAD_LEN   = 4
)

type PlayerMsgItem struct {
	data          []byte
	data_len      int32
	data_head_len int32
	msg_code      uint16
}

type IdChangeInfo struct {
	add    []int32
	remove []int32
	update []int32
}

func (this *IdChangeInfo) is_changed() bool {
	if this.add != nil || this.remove != nil || this.update != nil {
		return true
	}
	return false
}

func (this *IdChangeInfo) id_add(id int32) {
	if this.add == nil {
		this.add = []int32{id}
	} else {
		this.add = append(this.add, id)
	}
}

func (this *IdChangeInfo) id_remove(id int32) {
	if this.remove == nil {
		this.remove = []int32{id}
	} else {
		this.remove = append(this.remove, id)
	}
}

func (this *IdChangeInfo) id_update(id int32) {
	if this.update == nil {
		this.update = []int32{id}
	} else {
		this.update = append(this.update, id)
	}
}

func (this *IdChangeInfo) reset() {
	this.add = nil
	this.remove = nil
	this.update = nil
}

type Player struct {
	Id            int32
	Account       string
	Token         string
	ol_array_idx  int32
	all_array_idx int32
	db            *dbPlayerRow
	pos           int32

	is_lock int32
	//bhandling          bool
	msg_items          []*PlayerMsgItem
	msg_items_lock     *sync.Mutex
	cur_msg_items_len  int32
	max_msg_items_len  int32
	total_msg_data_len int32
	b_base_prop_chg    bool

	used_drop_ids          map[int32]int32                       // 抽卡掉落ID统计
	team_member_mgr        map[int32]*TeamMember                 // 成员map
	tmp_teams              map[int32][]int32                     // 临时阵容，缓存爬塔活动等进攻阵容ID
	attack_team            *BattleTeam                           // PVP进攻阵型
	campaign_team          *BattleTeam                           // PVE战役进攻阵容
	tower_team             *BattleTeam                           // PVE爬塔进攻阵容
	active_stage_team      *BattleTeam                           // PVE活动进攻阵容
	friend_boss_team       *BattleTeam                           // PVE好友BOSS进攻阵容
	explore_team           *BattleTeam                           // PVE探索任务进攻阵容
	guild_stage_team       *BattleTeam                           // PVE公会副本进攻阵容
	fighing_friend_boss    int32                                 // 是否好友BOSS正在被挑战
	defense_team           *BattleTeam                           // PVP防守阵型
	use_defense            int32                                 // 是否正在使用防守阵型
	target_stage_team      *BattleTeam                           // PVE关卡防守阵型
	stage_id               int32                                 // 关卡ID
	stage_wave             int32                                 // 当前关卡怪物第几波
	roles_power            map[int32]int32                       // 角色战力
	roles_power_max_data   map[int32][]*table_config.XmlItemItem // 角色战力最高的装备
	battle_record_list     []int32                               // 战斗录像，按时间排序
	battle_record_count    int32                                 // 录像数
	roles_id_change_info   IdChangeInfo                          // 角色增删更新
	items_changed_info     map[int32]int32                       // 物品增删更新
	tmp_cache_items        map[int32]int32                       // 用于临时缓存物品
	is_handbook_adds       bool                                  // 是否新增角色图鉴
	states_changed         map[int32]int32                       // 提示状态变化
	receive_mail_locker    *sync.Mutex                           // 接收邮件锁
	new_mail_list_locker   *sync.Mutex                           // 新邮件列表锁
	new_mail_ids           []int32                               // 新邮件ID列表
	tmp_left_slot_equip_id int32                                 // 左槽升级临时保存
	already_upgrade        bool                                  // 一键合成
	friend_ask_add         []int32                               // 增加的好友申请
	friend_ask_add_locker  *sync.Mutex                           // 好友申请锁
	friend_add             []int32                               // 增加的好友
	friend_add_locker      *sync.Mutex                           // 好友锁
	assist_role_id         int32                                 // 助战好友角色ID
	assist_role_pos        int32                                 // 助战角色位置
	assist_friend          *Player                               // 助战好友
	assist_member          *TeamMember                           // 助战成员
	world_chat_data        PlayerChatData                        // 世界聊天缓存数据
	guild_chat_data        PlayerChatData                        // 公会聊天缓存
	recruit_chat_data      PlayerChatData                        // 招募聊天缓存
	anouncement_data       PlayerAnouncementData                 // 公告缓存数据
	inited                 bool                                  // 是否已初始化
	is_login               bool                                  // 是否在线
	sweep_num              int32                                 // 扫荡次数
	curr_sweep             int32                                 // 已扫荡次数
	role_power_ranklist    *utils.ShortRankList                  // 角色战力排行
}

func (this *Player) _init() {
	this.max_msg_items_len = INIT_PLAYER_MSG_NUM
	this.msg_items_lock = &sync.Mutex{}
	this.msg_items = make([]*PlayerMsgItem, this.max_msg_items_len)

	this.receive_mail_locker = &sync.Mutex{}
	this.new_mail_list_locker = &sync.Mutex{}
	this.friend_ask_add_locker = &sync.Mutex{}
	this.friend_add_locker = &sync.Mutex{}
	this.role_power_ranklist = &utils.ShortRankList{}
	this.role_power_ranklist.Init(ROLE_MAX_COUNT)
}

func new_player(id int32, account, token string, db *dbPlayerRow) *Player {
	ret_p := &Player{}
	ret_p.Id = id
	ret_p.Account = account
	ret_p.Token = token
	ret_p.db = db
	ret_p.ol_array_idx = -1
	ret_p.all_array_idx = -1

	ret_p._init()

	return ret_p
}

func new_player_with_db(id int32, db *dbPlayerRow) *Player {
	if id <= 0 || nil == db {
		log.Error("new_player_with_db param error !", id, nil == db)
		return nil
	}

	ret_p := &Player{}
	ret_p.Id = id
	ret_p.db = db
	ret_p.ol_array_idx = -1
	ret_p.all_array_idx = -1
	ret_p.Account = db.GetAccount()

	ret_p._init()

	// 载入竞技场排名
	ret_p.LoadArenaScore()

	// 载入关卡排名
	ret_p.LoadCampaignRankData()

	// 载入所有角色战力排名
	ret_p.LoadRolesPowerRankData()

	return ret_p
}

func (this *Player) check_and_send_items_change() {
	if this.items_changed_info != nil {
		var msg msg_client_message.S2CItemsUpdate
		for k, v := range this.items_changed_info {
			msg.ItemsAdd = append(msg.ItemsAdd, &msg_client_message.ItemInfo{
				ItemCfgId: k,
				ItemNum:   v,
			})
		}
		this.Send(uint16(msg_client_message_id.MSGID_S2C_ITEMS_UPDATE), &msg)
		this.items_changed_info = nil
	}
}

func (this *Player) add_msg_data(msg_code uint16, data []byte) {
	if nil == data {
		log.Error("Player add_msg_data !")
		return
	}

	this.msg_items_lock.Lock()
	defer this.msg_items_lock.Unlock()

	if this.cur_msg_items_len >= this.max_msg_items_len {
		new_max := this.max_msg_items_len + 5
		new_msg_items := make([]*PlayerMsgItem, new_max)
		for idx := int32(0); idx < this.max_msg_items_len; idx++ {
			new_msg_items[idx] = this.msg_items[idx]
		}

		this.msg_items = new_msg_items
		this.max_msg_items_len = new_max
	}

	new_item := &PlayerMsgItem{}
	new_item.msg_code = msg_code
	new_item.data = data
	new_item.data_len = int32(len(data))
	this.total_msg_data_len += new_item.data_len + MSG_ITEM_HEAD_LEN
	this.msg_items[this.cur_msg_items_len] = new_item

	this.cur_msg_items_len++

	return
}

func (this *Player) PopCurMsgData() []byte {
	if this.b_base_prop_chg {
		this.send_info()
	}

	this.check_and_send_roles_change()
	this.check_and_send_items_change()
	this.CheckAndAnouncement()
	if this.is_handbook_adds {
		this.get_role_handbook()
		this.is_handbook_adds = false
	}
	this.CheckNewMail()

	this.msg_items_lock.Lock()
	defer this.msg_items_lock.Unlock()

	//this.bhandling = false

	out_bytes := make([]byte, this.total_msg_data_len)
	tmp_len := int32(0)
	var tmp_item *PlayerMsgItem
	for idx := int32(0); idx < this.cur_msg_items_len; idx++ {
		tmp_item = this.msg_items[idx]
		if nil == tmp_item {
			continue
		}

		out_bytes[tmp_len] = byte(tmp_item.msg_code >> 8)
		out_bytes[tmp_len+1] = byte(tmp_item.msg_code & 0xFF)
		out_bytes[tmp_len+2] = byte(tmp_item.data_len >> 8)
		out_bytes[tmp_len+3] = byte(tmp_item.data_len & 0xFF)
		tmp_len += 4
		copy(out_bytes[tmp_len:], tmp_item.data)
		tmp_len += tmp_item.data_len
	}

	this.cur_msg_items_len = 0
	this.total_msg_data_len = 0
	return out_bytes
}

func (this *Player) Send(msg_id uint16, msg proto.Message) (msg_data []byte) {
	/*if !this.bhandling {
		log.Error("Player [%d] send msg[%d] no bhandling !", this.Id, msg_id)
		return
	}*/

	//log.Debug("[发送] [玩家%d:%v] [%s] !", this.Id, msg_id, msg.String())

	var err error
	msg_data, err = proto.Marshal(msg)
	if nil != err {
		log.Error("Player Marshal msg failed err[%s] !", err.Error())
		return
	}

	this.add_msg_data(msg_id, msg_data)
	return
}

func (this *Player) OnCreate() {
	// 随机初始名称
	tmp_acc := this.Account
	if len(tmp_acc) > 6 {
		tmp_acc = string([]byte(tmp_acc)[0:6])
	}

	// 初始成就任务
	this.first_gen_achieve_tasks()

	this.db.Info.SetLvl(1)
	this.db.Info.SetCreateUnix(int32(time.Now().Unix()))
	this.add_init_roles()
	this.db.Info.IncbyDiamond(global_config.InitDiamond)
	this.db.Info.IncbyGold(global_config.InitCoin)
	this.db.SetName(this.Account) // 昵称用默认账号名

	return
}

func (this *Player) OnInit() {
	this.is_login = true
	if this.inited {
		return
	}
	this.team_member_mgr = make(map[int32]*TeamMember)
	this.roles_power = make(map[int32]int32)
	this.roles_power_max_data = make(map[int32][]*table_config.XmlItemItem)
	this.init_battle_record_list()
	this.inited = true
}

func (this *Player) OnLogin() {
	this.OnInit()
	/*if USE_CONN_TIMER_WHEEL == 0 {
		conn_timer_mgr.Insert(this.Id)
	} else {
		conn_timer_wheel.Insert(this.Id)
	}*/

	this.ChkPlayerDailyTask()
	this.db.Info.SetLastLogin(int32(time.Now().Unix()))
	friend_recommend_mgr.AddPlayer(this.Id)
	atomic.StoreInt32(&this.is_lock, 0)
	log.Info("Player[%v] login", this.Id)
}

func (this *Player) OnLogout() {
	if USE_CONN_TIMER_WHEEL == 0 {
		conn_timer_mgr.Remove(this.Id)
	} else {
		conn_timer_wheel.Remove(this.Id)
	}

	// 离线收益时间开始
	this.db.Info.SetLastLogout(int32(time.Now().Unix()))
	// 离线时结算挂机收益
	this.hangup_income_get(0, true)
	this.hangup_income_get(1, true)
	this.is_login = false
	log.Info("玩家[%d] 登出 ！！", this.Id)
}

func (this *Player) IsOffline() bool {
	return !this.is_login
}

func (this *Player) send_enter_game(acc string, id int32) {
	res := &msg_client_message.S2CEnterGameResponse{}
	res.Acc = acc
	res.PlayerId = id
	this.Send(uint16(msg_client_message_id.MSGID_S2C_ENTER_GAME_RESPONSE), res)
}

func (this *Player) send_teams() {
	msg := &msg_client_message.S2CTeamsResponse{}
	/*attack_team := &msg_client_message.TeamData{
		TeamType:    BATTLE_ATTACK_TEAM,
		TeamMembers: this.db.BattleTeam.GetAttackMembers(),
	}*/
	defense_team := &msg_client_message.TeamData{
		TeamType:    BATTLE_DEFENSE_TEAM,
		TeamMembers: this.db.BattleTeam.GetDefenseMembers(),
	}
	campaign_team := &msg_client_message.TeamData{
		TeamType:    BATTLE_CAMPAIN_TEAM,
		TeamMembers: this.db.BattleTeam.GetCampaignMembers(),
	}
	msg.Teams = []*msg_client_message.TeamData{ /*attack_team, */ defense_team, campaign_team}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_TEAMS_RESPONSE), msg)
}

func (this *Player) send_info() {
	response := &msg_client_message.S2CPlayerInfoResponse{
		Level:    this.db.Info.GetLvl(),
		Exp:      this.db.Info.GetExp(),
		Gold:     this.db.Info.GetGold(),
		Diamond:  this.db.Info.GetDiamond(),
		Icon:     this.db.Info.GetIcon(),
		VipLevel: this.db.Info.GetVipLvl(),
		Name:     this.db.GetName(),
		SysTime:  int32(time.Now().Unix()),
		GuildId:  this.db.Guild.GetId(),
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_PLAYER_INFO_RESPONSE), response)
	log.Debug("Player[%v] info: %v", this.Id, response)
}

func (this *Player) notify_enter_complete() {
	msg := &msg_client_message.S2CEnterGameCompleteNotify{}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_ENTER_GAME_COMPLETE_NOTIFY), msg)
}

func (this *Player) send_notify_state() {
	var response *msg_client_message.S2CStateNotify

	// 挂机收益
	s := this.check_income_state()
	if s != 0 {
		if response == nil {
			response = &msg_client_message.S2CStateNotify{}
		}
	}
	if s > 0 {
		response.States = append(response.States, int32(msg_client_message.MODULE_STATE_HANGUP_RANDOM_INCOME))
	} else if s < 0 {
		response.CancelStates = append(response.CancelStates, int32(msg_client_message.MODULE_STATE_HANGUP_RANDOM_INCOME))
	}

	// 其他
	if this.states_changed != nil {
		if response == nil {
			response = &msg_client_message.S2CStateNotify{}
		}
		for k, v := range this.states_changed {
			if v == 1 {
				response.States = append(response.States, k)
			} else if v == 2 {
				response.CancelStates = append(response.CancelStates, k)
			}
		}
		this.states_changed = nil
	}

	if response != nil {
		this.Send(uint16(msg_client_message_id.MSGID_S2C_STATE_NOTIFY), response)
	}
}

func (this *Player) notify_state_changed(state int32, change_type int32) {
	if this.states_changed == nil {
		this.states_changed = make(map[int32]int32)
	}
	this.states_changed[state] = change_type
}

func (this *Player) SetTeam(team_type int32, team []int32) int32 {
	if team == nil {
		return -1
	}

	// 判断是否有重复
	var member_num int32
	used_id := make(map[int32]bool)
	for i := 0; i < len(team); i++ {
		if team[i] > 0 {
			member_num += 1
		}

		if this.assist_friend != nil {
			if this.assist_role_pos == int32(i) {
				continue
			}
		} else if team[i] <= 0 {
			continue
		}

		if _, o := used_id[team[i]]; o {
			return int32(msg_client_message.E_ERR_PLAYER_SET_ATTACK_MEMBERS_FAILED)
		}
		used_id[team[i]] = true
	}

	if team_type == BATTLE_ATTACK_TEAM || team_type == BATTLE_DEFENSE_TEAM {
		if member_num > PVP_TEAM_MAX_MEMBER_NUM {
			return int32(msg_client_message.E_ERR_PLAYER_PVP_TEAM_MEMBERS_TOO_MORE)
		}
	}

	for i := 0; i < len(team); i++ {
		if i >= BATTLE_TEAM_MEMBER_MAX_NUM {
			break
		}
		if this.assist_friend != nil && this.assist_role_pos == int32(i) {
			this.assist_friend.db.Roles.HasIndex(this.assist_role_id)
			continue
		} else if team[i] <= 0 {
			continue
		}
		if !this.db.Roles.HasIndex(team[i]) {
			log.Warn("Player[%v] not has role[%v] for set attack team", this.Id, team[i])
			return int32(msg_client_message.E_ERR_PLAYER_SET_ATTACK_MEMBERS_FAILED)
		}
		//this.db.Roles.SetIsLock(team[i], 1)
	}

	if team_type == BATTLE_CAMPAIN_TEAM {
		this.db.BattleTeam.SetCampaignMembers(team)
	} else if team_type == BATTLE_DEFENSE_TEAM {
		this.db.BattleTeam.SetDefenseMembers(team)
	} else {
		if this.tmp_teams == nil {
			this.tmp_teams = make(map[int32][]int32)
		}
		this.tmp_teams[team_type] = team
	}
	return 1
}

func (this *Player) SetCampaignTeam(team []int32) int32 {
	if team == nil {
		return -1
	}

	used_id := make(map[int32]bool)
	for i := 0; i < len(team); i++ {
		if team[i] <= 0 {
			continue
		}
		if _, o := used_id[team[i]]; o {
			return int32(msg_client_message.E_ERR_PLAYER_SET_ATTACK_MEMBERS_FAILED)
		}
		used_id[team[i]] = true
	}
	for i := 0; i < len(team); i++ {
		if i >= BATTLE_TEAM_MEMBER_MAX_NUM {
			break
		}
		if team[i] <= 0 {
			continue
		}
		if !this.db.Roles.HasIndex(team[i]) {
			log.Warn("Player[%v] not has role[%v] for set campaign team", this.Id, team[i])
			return int32(msg_client_message.E_ERR_PLAYER_SET_ATTACK_MEMBERS_FAILED)
		}
		//this.db.Roles.SetIsLock(team[i], 1)
	}
	this.db.BattleTeam.SetCampaignMembers(team)
	return 1
}

func (this *Player) SetDefenseTeam(team []int32) int32 {
	if team == nil {
		return -1
	}

	used_id := make(map[int32]bool)
	for i := 0; i < len(team); i++ {
		if team[i] <= 0 {
			continue
		}
		if _, o := used_id[team[i]]; o {
			return int32(msg_client_message.E_ERR_PLAYER_SET_DEFENSE_MEMBERS_FAILED)
		}
		used_id[team[i]] = true
	}

	for i := 0; i < len(team); i++ {
		if i >= BATTLE_TEAM_MEMBER_MAX_NUM {
			break
		}
		if team[i] <= 0 {
			continue
		}
		if !this.db.Roles.HasIndex(team[i]) {
			log.Warn("Player[%v] not has role[%v] for set defense team", this.Id, team[i])
			return int32(msg_client_message.E_ERR_PLAYER_SET_DEFENSE_MEMBERS_FAILED)
		}
		//this.db.Roles.SetIsLock(team[i], 1)
	}
	this.db.BattleTeam.SetDefenseMembers(team)
	return 1
}

func (this *Player) SetDefensing() bool {
	return atomic.CompareAndSwapInt32(&this.use_defense, 0, 1)
}

func (this *Player) CancelDefensing() bool {
	return atomic.CompareAndSwapInt32(&this.use_defense, 1, 0)
}

func (this *Player) Fight2Player(battle_type, player_id int32) int32 {
	if battle_type == 1 {
		matched_player_id := this.db.Arena.GetMatchedPlayerId()
		if matched_player_id > 0 && player_id != matched_player_id {
			log.Error("Player[%v] only fight to matched player[%v], not player[%v]", this.Id, matched_player_id, player_id)
			return int32(msg_client_message.E_ERR_PLAYER_ARENA_ONLY_FIGHT_MATCHED_PLAYER)
		}
	}

	var robot *ArenaRobot
	p := player_mgr.GetPlayerById(player_id)
	if p == nil {
		robot = arena_robot_mgr.Get(player_id)
		if robot == nil {
			return int32(msg_client_message.E_ERR_PLAYER_NOT_EXIST)
		}
	}

	// 赛季是否开始
	if battle_type == 1 {
		if !arena_season_mgr.IsSeasonStart() {
			log.Error("Arena Season is not start, wait a while")
			return int32(msg_client_message.E_ERR_PLAYER_ARENA_SEASON_IS_RESETING)
		}
	}

	// 设置正在防守
	if p != nil {
		if !p.SetDefensing() {
			log.Warn("Player[%v] is defensing, player[%v] fight failed", player_id, this.Id)
			return int32(msg_client_message.E_ERR_PLAYER_IS_DEFENSING)
		}
	}

	if this.attack_team == nil {
		this.attack_team = &BattleTeam{}
	}

	res := this.attack_team.Init(this, BATTLE_ATTACK_TEAM, 0)
	if res < 0 {
		if p != nil {
			p.CancelDefensing()
		}
		log.Error("Player[%v] init attack team failed, err %v", this.Id, res)
		return res
	}

	var target_team *BattleTeam
	var target_team_format []*msg_client_message.BattleMemberItem
	if p != nil {
		if p.defense_team == nil {
			p.defense_team = &BattleTeam{}
		}
		res = p.defense_team.Init(p, BATTLE_DEFENSE_TEAM, 1)
		if res < 0 {
			p.CancelDefensing()
			log.Error("Player[%v] init defense team failed", player_id)
			return res
		}

		target_team_format = p.defense_team._format_members_for_msg()
		target_team = p.defense_team
	} else {
		if this.target_stage_team == nil {
			this.target_stage_team = &BattleTeam{}
		}
		if !this.target_stage_team.InitWithArenaRobot(robot.robot_data, 1) {
			log.Error("Robot[%v] init defense team failed", player_id)
			return -1
		}

		target_team_format = this.target_stage_team._format_members_for_msg()
		target_team = this.target_stage_team
	}

	my_team_format := this.attack_team._format_members_for_msg()

	// To Fight
	is_win, enter_reports, rounds := this.attack_team.Fight(target_team, BATTLE_END_BY_ALL_DEAD, 0)

	if p != nil {
		// 对方防守结束
		p.CancelDefensing()
	}

	var add_score int32
	if battle_type == 1 {
		this.db.Arena.SetMatchedPlayerId(0)
		// 竞技场加分
		_, add_score = this.UpdateArenaScore(is_win)
	}

	members_damage := this.attack_team.common_data.members_damage
	members_cure := target_team.common_data.members_cure
	response := &msg_client_message.S2CBattleResultResponse{
		IsWin:               is_win,
		EnterReports:        enter_reports,
		Rounds:              rounds,
		MyTeam:              my_team_format,
		TargetTeam:          target_team_format,
		MyMemberDamages:     members_damage[this.attack_team.side],
		TargetMemberDamages: members_damage[target_team.side],
		MyMemberCures:       members_cure[this.attack_team.side],
		TargetMemberCures:   members_cure[target_team.side],
		BattleType:          battle_type,
		BattleParam:         player_id,
		MySpeedBonus:        this.attack_team.first_hand,
		TargetSpeedBonus:    target_team.first_hand,
	}
	d := this.Send(uint16(msg_client_message_id.MSGID_S2C_BATTLE_RESULT_RESPONSE), response)

	// 保存录像
	if battle_type == 1 && d != nil {
		var win int32
		if is_win {
			win = 1
		}
		battle_record_mgr.SaveNew(this.Id, player_id, d, win, add_score)
	}

	// 更新任务
	if battle_type == 1 {
		this.TaskUpdate(table_config.TASK_COMPLETE_TYPE_ARENA_FIGHT_NUM, false, 0, 1)
		if is_win {
			this.TaskUpdate(table_config.TASK_COMPLETE_TYPE_ARENA_WIN_NUM, false, 0, 1)
		}
	}

	Output_S2CBattleResult(this, response)
	return 1
}

func output_report(rr *msg_client_message.BattleReportItem) {
	log.Debug("		 	report: side[%v]", rr.Side)
	log.Debug("					 skill_id: %v", rr.SkillId)
	log.Debug("					 user: Side[%v], Pos[%v], HP[%v], MaxHP[%v], Energy[%v], Damage[%v]", rr.User.Side, rr.User.Pos, rr.User.HP, rr.User.MaxHP, rr.User.Energy, rr.User.Damage)
	if rr.IsSummon {
		if rr.SummonNpcs != nil {
			for n := 0; n < len(rr.SummonNpcs); n++ {
				rrs := rr.SummonNpcs[n]
				if rrs != nil {
					log.Debug("					 summon npc: Side[%v], Pos[%v], Id[%v], TableId[%v], HP[%v], MaxHP[%v], Energy[%v]", rrs.Side, rrs.Pos, rrs.Id, rrs.TableId, rrs.HP, rrs.MaxHP, rrs.Energy)
				}
			}
		}
	} else {
		if rr.BeHiters != nil {
			for n := 0; n < len(rr.BeHiters); n++ {
				rrb := rr.BeHiters[n]
				log.Debug("					 behiter: Side[%v], Pos[%v], HP[%v], MaxHP[%v], Energy[%v], Damage[%v], IsCritical[%v], IsBlock[%v]",
					rrb.Side, rrb.Pos, rrb.HP, rrb.MaxHP, rrb.Energy, rrb.Damage, rrb.IsCritical, rrb.IsBlock)
			}
		}
	}
	if rr.AddBuffs != nil {
		for n := 0; n < len(rr.AddBuffs); n++ {
			log.Debug("					 add buff: Side[%v], Pos[%v], BuffId[%v]", rr.AddBuffs[n].Side, rr.AddBuffs[n].Pos, rr.AddBuffs[n].BuffId)
		}
	}
	if rr.RemoveBuffs != nil {
		for n := 0; n < len(rr.RemoveBuffs); n++ {
			log.Debug("					 remove buff: Side[%v], Pos[%v], BuffId[%v]", rr.RemoveBuffs[n].Side, rr.RemoveBuffs[n].Pos, rr.RemoveBuffs[n].BuffId)
		}
	}

	log.Debug("					 has_combo: %v", rr.HasCombo)
}

func Output_S2CBattleResult(player *Player, m proto.Message) {
	response := m.(*msg_client_message.S2CBattleResultResponse)
	if response.IsWin {
		log.Debug("Player[%v] wins", player.Id)
	} else {
		log.Debug("Player[%v] lost", player.Id)
	}
	log.Debug("My Speed Bonus %v", response.GetMySpeedBonus())
	log.Debug("Target Speed Bonus %v", response.GetTargetSpeedBonus())
	if response.MyTeam != nil {
		log.Debug("My team:")
		for i := 0; i < len(response.MyTeam); i++ {
			m := response.MyTeam[i]
			if m == nil {
				continue
			}
			log.Debug("		 Side:%v Id:%v Pos:%v HP:%v MaxHP:%v Energy:%v TableId:%v", m.Side, m.Id, m.Pos, m.HP, m.MaxHP, m.Energy, m.TableId)
		}
	}
	if response.TargetTeam != nil {
		log.Debug("Target team:")
		for i := 0; i < len(response.TargetTeam); i++ {
			m := response.TargetTeam[i]
			if m == nil {
				continue
			}
			log.Debug("		 Side:%v Id:%v Pos:%v HP:%v MaxHP:%v Energy:%v TableId:%v", m.Side, m.Id, m.Pos, m.HP, m.MaxHP, m.Energy, m.TableId)
		}
	}

	if response.EnterReports != nil {
		log.Debug("   before enter:")
		for i := 0; i < len(response.EnterReports); i++ {
			r := response.EnterReports[i]
			output_report(r)
		}
	}

	if response.Rounds != nil {
		log.Debug("Round num: %v", len(response.Rounds))
		for i := 0; i < len(response.Rounds); i++ {
			r := response.Rounds[i]
			log.Debug("	  round[%v]", r.RoundNum)
			if r.Reports != nil {
				for j := 0; j < len(r.Reports); j++ {
					rr := r.Reports[j]
					output_report(rr)
				}
			}
			if r.RemoveBuffs != nil {
				for j := 0; j < len(r.RemoveBuffs); j++ {
					b := r.RemoveBuffs[j]
					log.Debug("		 	remove buffs: Side[%v], Pos[%v], BuffId[%v]", b.Side, b.Pos, b.BuffId)
				}
			}
			if r.ChangedFighters != nil {
				for j := 0; j < len(r.ChangedFighters); j++ {
					m := r.ChangedFighters[j]
					log.Debug("			changed member: Side[%v], Pos[%v], HP[%v], MaxHP[%v], Energy[%v], Damage[%v]", m.Side, m.Pos, m.HP, m.MaxHP, m.Energy, m.Damage)
				}
			}
		}
	}
}
