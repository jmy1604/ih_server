package main

import (
	"ih_server/libs/log"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"ih_server/src/table_config"
	"math/rand"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
)

type DelaySkillList struct {
	head *DelaySkill
	tail *DelaySkill
}

type BattleCommonData struct {
	reports                []*msg_client_message.BattleReportItem
	remove_buffs           []*msg_client_message.BattleMemberBuff
	changed_fighters       []*msg_client_message.BattleFighter
	round_num              int32
	delay_skill_list       *DelaySkillList
	members_damage         [][]int32
	members_cure           [][]int32
	my_artifact_energy     int32
	target_artifact_energy int32
}

func (this *BattleCommonData) init_damage_data() {
	if this.members_damage == nil {
		this.members_damage = make([][]int32, 2)

	}
	if this.members_cure == nil {
		this.members_cure = make([][]int32, 2)
	}
}

func (this *BattleCommonData) reset_damage_data() {
	if this.members_damage != nil {
		for i := 0; i < len(this.members_damage); i++ {
			this.members_damage[i] = make([]int32, BATTLE_TEAM_MEMBER_MAX_NUM)
		}
	}
	if this.members_cure != nil {
		for i := 0; i < len(this.members_cure); i++ {
			this.members_cure[i] = make([]int32, BATTLE_TEAM_MEMBER_MAX_NUM)
		}
	}
}

func (this *BattleCommonData) Init() {
	this.init_damage_data()
}

func (this *BattleCommonData) Reset() {
	this.reports = make([]*msg_client_message.BattleReportItem, 0)
	this.remove_buffs = make([]*msg_client_message.BattleMemberBuff, 0)
	this.changed_fighters = make([]*msg_client_message.BattleFighter, 0)
	if this.delay_skill_list != nil {
		d := this.delay_skill_list.head
		for d != nil {
			n := d.next
			delay_skill_pool.Put(d)
			d = n
		}
	}
	this.my_artifact_energy = 0
	this.target_artifact_energy = 0
}

func (this *BattleCommonData) Recycle() {
	if this.reports != nil {
		for i := 0; i < len(this.reports); i++ {
			r := this.reports[i]
			if r == nil {
				continue
			}
			// user
			if r.User != nil {
				msg_battle_fighter_pool.Put(r.User)
				r.User = nil
			}
			// behiters
			if r.BeHiters != nil {
				for j := 0; j < len(r.BeHiters); j++ {
					if r.BeHiters[j] != nil {
						msg_battle_fighter_pool.Put(r.BeHiters[j])
					}
				}
				r.BeHiters = nil
			}
			// add buffs
			if r.AddBuffs != nil {
				for j := 0; j < len(r.AddBuffs); j++ {
					if r.AddBuffs[j] != nil {
						msg_battle_buff_item_pool.Put(r.AddBuffs[j])
						r.AddBuffs[j] = nil
					}
				}
				r.AddBuffs = nil
			}
			// remove buffs
			if r.RemoveBuffs != nil {
				for j := 0; j < len(r.RemoveBuffs); j++ {
					if r.RemoveBuffs[j] != nil {
						msg_battle_buff_item_pool.Put(r.RemoveBuffs[j])
						r.RemoveBuffs[j] = nil
					}
				}
				r.RemoveBuffs = nil
			}
			msg_battle_reports_item_pool.Put(r)
			this.reports[i] = nil
		}
		this.reports = nil
	}

	if this.remove_buffs != nil {
		for i := 0; i < len(this.remove_buffs); i++ {
			b := this.remove_buffs[i]
			if b == nil {
				continue
			}
			msg_battle_buff_item_pool.Put(b)
			this.remove_buffs[i] = nil
		}
		this.remove_buffs = nil
	}

	if this.changed_fighters != nil {
		for i := 0; i < len(this.changed_fighters); i++ {
			m := this.changed_fighters[i]
			if m == nil {
				continue
			}
			msg_battle_fighter_pool.Put(m)
			this.changed_fighters[i] = nil
		}
		this.changed_fighters = nil
	}
}

type BattleTeam struct {
	player            *Player
	team_type         int32
	curr_attack       int32             // 当前进攻的索引
	side              int32             // 0 左边 1 右边
	temp_curr_id      int32             // 临时ID，用于标识召唤的角色
	members           []*TeamMember     // 成员
	artifact          *TeamMember       // 神器
	common_data       *BattleCommonData // 每回合战报
	friend            *Player           // 用于好友BOSS
	guild             *dbGuildRow       // 用于公会副本
	first_hand        int32             // 先手值
	first_hand_locker sync.RWMutex      // 先手值锁
	is_sweeping       bool              // 是否正在扫荡
}

// 先手值
func (this *BattleTeam) get_first_hand() int32 {
	this.first_hand_locker.RLock()
	defer this.first_hand_locker.RUnlock()
	return this.first_hand
}

func (this *BattleTeam) clear_first_hand() {
	this.first_hand_locker.Lock()
	defer this.first_hand_locker.Unlock()
	this.first_hand = 0
}

func (this *BattleTeam) calc_first_hand(p *Player) {
	if p == nil {
		return
	}

	this.first_hand_locker.Lock()
	defer this.first_hand_locker.Unlock()

	this.first_hand = 0
	ids := p.db.Talents.GetAllIndex()
	if ids == nil {
		return
	}

	for i := 0; i < len(ids); i++ {
		id := ids[i]
		lvl, _ := p.db.Talents.GetLevel(id)
		t := talent_table_mgr.GetByIdLevel(id, lvl)
		if t == nil {
			log.Error("Player[%v] talent[%v] level[%v] data not found", p.Id, id, lvl)
			continue
		}
		this.first_hand += t.TeamSpeedBonus
		log.Debug("@@@@@ team[%v] add talent[%v] level[%v] first hand %v, total first hand %v", this.side, id, lvl, t.TeamSpeedBonus, this.first_hand)
	}
}

// 利用玩家初始化
func (this *BattleTeam) Init(p *Player, team_id int32, side int32) int32 {
	var members []int32
	var artifact *table_config.XmlArtifactItem
	if team_id == BATTLE_TEAM_DEFENSE {
		members = p.db.BattleTeam.GetDefenseMembers()
		aid := p.db.BattleTeam.GetDefenseArtifactId()
		ar, _ := p.db.Artifacts.GetRank(aid)
		al, _ := p.db.Artifacts.GetLevel(aid)
		artifact = artifact_table_mgr.Get(aid, ar, al)
	} else if team_id == BATTLE_TEAM_CAMPAIN {
		members = p.db.BattleTeam.GetCampaignMembers()
		aid := p.db.BattleTeam.GetCampaignArtifactId()
		ar, _ := p.db.Artifacts.GetRank(aid)
		al, _ := p.db.Artifacts.GetLevel(aid)
		artifact = artifact_table_mgr.Get(aid, ar, al)
	} else if team_id < BATTLE_TEAM_MAX {
		if p.tmp_teams == nil {
			p.tmp_teams = make(map[int32]*TmpTeam)
		}
		tmp_team := p.tmp_teams[team_id]
		if tmp_team == nil {
			tmp_team = &TmpTeam{
				// 没有设置阵型就用战役阵型
				members: p.db.BattleTeam.GetCampaignMembers(),
			}
			p.tmp_teams[team_id] = tmp_team
		}
		members = tmp_team.members
		artifact = tmp_team.artifact
	} else {
		log.Warn("Unknown team id %v", team_id)
		return int32(msg_client_message.E_ERR_PLAYER_TEAM_TYPE_INVALID)
	}

	if members == nil {
		return int32(msg_client_message.E_ERR_PLAYER_TEAM_MEMBERS_IS_EMPTY)
	}

	is_empty := true
	// 检测是否为空
	for i := 0; i < len(members); i++ {
		if members[i] > 0 {
			is_empty = false
			break
		}
	}
	if is_empty {
		return int32(msg_client_message.E_ERR_PLAYER_TEAM_MEMBERS_IS_EMPTY)
	}

	this.player = p
	this.team_type = team_id
	this.clear_first_hand()

	// 成员
	if this.members == nil {
		this.members = make([]*TeamMember, BATTLE_TEAM_MEMBER_MAX_NUM)
	}
	for i := 0; i < len(this.members); i++ {
		if this.members[i] != nil {
			team_member_pool.Put(this.members[i])
			this.members[i] = nil
		}
		if (i < len(members) && members[i] <= 0) || i >= len(members) {
			continue
		}
		m := p.get_team_member_by_role(members[i], this, int32(i))
		if m == nil {
			log.Error("Player[%v] init battle team get member with role_id[%v] error", p.Id, members[i])
			continue
		}
		this.members[i] = m
	}

	// 神器
	this._init4artifact(artifact)

	this.calc_first_hand(p)
	this.curr_attack = 0
	this.side = side
	this.temp_curr_id = p.db.Global.GetCurrentRoleId() + 1

	// 远征
	if team_id == BATTLE_TEAM_EXPEDITION {
		res := p.expedition_team_init(this.members)
		if res < 0 {
			return res
		}
	}

	return 1
}

// init for artifact
func (this *BattleTeam) _init4artifact(artifact *table_config.XmlArtifactItem) {
	if this.artifact != nil {
		team_member_pool.Put(this.artifact)
		this.artifact = nil
	}
	if artifact != nil {
		this.artifact = team_member_pool.Get()
		this.artifact.attrs = make([]int32, ATTR_COUNT_MAX)
		this.artifact.energy = 0
		this.artifact.pos = -1
		this.artifact.artifact = artifact
		this.artifact.team = this
	}
}

// init with stage
func (this *BattleTeam) InitWithStage(side int32, stage_id int32, monster_wave int32, friend *Player, guild *dbGuildRow) bool {
	this.player = nil
	stage := stage_table_mgr.Get(stage_id)
	if stage == nil {
		log.Warn("Cant found stage %v", stage_id)
		return false
	}

	if stage.Monsters == nil || len(stage.Monsters) == 0 {
		return false
	}

	if this.members == nil {
		this.members = make([]*TeamMember, BATTLE_TEAM_MEMBER_MAX_NUM)
	}

	for i := 0; i < len(this.members); i++ {
		if this.members[i] != nil {
			team_member_pool.Put(this.members[i])
			this.members[i] = nil
		}
	}

	this.side = side
	this.curr_attack = 0

	for i := 0; i < len(stage.Monsters); i++ {
		monster := stage.Monsters[i]
		if monster.Wave-1 == monster_wave {
			pos := monster.Slot - 1
			if pos < 0 || pos >= BATTLE_ROUND_MAX_NUM {
				log.Error("Stage[%v] monster wave[%v] slot[%v] invalid", stage_id, monster_wave, monster.Slot)
				return false
			}

			if friend != nil && !friend.db.FriendBosss.HasIndex(pos) {
				// 好友BOSS
				continue
			} else if guild != nil && guild.Stage.GetBossPos() != pos {
				// 公会副本
				continue
			}

			role_card := card_table_mgr.GetRankCard(monster.MonsterID, monster.Rank)
			if role_card == nil {
				log.Error("Cant get card by role_id[%v] and rank[%v]", monster.MonsterID, monster.Rank)
				return false
			}

			m := team_member_pool.Get()

			m.init_all(this, 0, monster.Level, role_card, pos, nil, monster.EquipID)

			// 好友BOSS
			if friend != nil {
				hp, _ := friend.db.FriendBosss.GetMonsterHp(pos)
				max_hp, _ := friend.db.FriendBosss.GetMonsterMaxHp(pos)

				// 新BOSS
				if hp == 0 {
					hp = m.attrs[ATTR_HP_MAX]
				}

				var hp_adjust bool
				if max_hp != m.attrs[ATTR_HP_MAX] {
					if max_hp > 0 {
						hp_adjust = true
					}
					friend.db.FriendBosss.SetMonsterMaxHp(pos, m.attrs[ATTR_HP_MAX])
				}
				if hp_adjust {
					if hp > max_hp {
						hp = max_hp
					}
					hp = int32(int64(hp) * int64(m.attrs[ATTR_HP_MAX]) / int64(max_hp))
				} else if hp > m.attrs[ATTR_HP_MAX] {
					hp = m.attrs[ATTR_HP_MAX]
				}
				friend.db.FriendBosss.SetMonsterHp(pos, hp)
				m.attrs[ATTR_HP] = hp
				m.hp = m.attrs[ATTR_HP]
			} else if guild != nil {
				// 公会副本
				hp_percent := guild.Stage.GetHpPercent()
				if hp_percent == 100 {
					m.attrs[ATTR_HP] = int32(int64(m.attrs[ATTR_HP_MAX]) * int64(hp_percent) / 100)
					m.hp = m.attrs[ATTR_HP]
				} else {
					boss_hp := guild.Stage.GetBossHP()
					if boss_hp > 0 {
						m.attrs[ATTR_HP] = boss_hp
						m.hp = boss_hp
					} else {
						if hp_percent == 0 {
							boss_hp = -1
							m.attrs[ATTR_HP] = boss_hp
							m.hp = boss_hp
						} else {
							boss_hp = int32(int64(m.attrs[ATTR_HP_MAX]) * int64(hp_percent) / 100)
							m.attrs[ATTR_HP] = boss_hp
							m.hp = boss_hp
						}
					}
				}
			}

			this.members[pos] = m
		}
	}

	this.friend = friend
	this.guild = guild

	return true
}

// init with arena robot
func (this *BattleTeam) InitWithArenaRobot(robot *table_config.XmlArenaRobotItem, side int32) bool {
	if this.members == nil {
		this.members = make([]*TeamMember, BATTLE_TEAM_MEMBER_MAX_NUM)
	}

	for i := 0; i < len(this.members); i++ {
		if this.members[i] != nil {
			team_member_pool.Put(this.members[i])
			this.members[i] = nil
		}
	}

	this.side = side
	this.curr_attack = 0

	for i := 0; i < len(robot.RobotCardList); i++ {
		monster := robot.RobotCardList[i]
		pos := monster.Slot - 1
		if pos < 0 || pos >= BATTLE_ROUND_MAX_NUM {
			log.Error("Arena Robot[%v] monster slot[%v] invalid", robot.Id, monster.Slot)
			return false
		}

		role_card := card_table_mgr.GetRankCard(monster.MonsterID, monster.Rank)
		if role_card == nil {
			log.Error("Cant get card by role_id[%v] and rank[%v]", monster.MonsterID, monster.Rank)
			return false
		}

		m := team_member_pool.Get()

		m.init_all(this, 0, monster.Level, role_card, pos, nil, monster.EquipID)
		this.members[pos] = m
	}

	return true
}

func (this *BattleTeam) InitExpeditionEnemy(p *Player) bool {
	db_expe := p.get_curr_expedition_db_roles()
	if db_expe == nil {
		return false
	}

	all_pos := db_expe.GetAllIndex()
	if all_pos == nil || len(all_pos) == 0 {
		return false
	}

	if this.members == nil {
		this.members = make([]*TeamMember, BATTLE_TEAM_MEMBER_MAX_NUM)
	}

	for i := 0; i < len(this.members); i++ {
		if this.members[i] != nil {
			team_member_pool.Put(this.members[i])
			this.members[i] = nil
		}
	}

	this.side = 1
	this.curr_attack = 0

	for i := 0; i < len(all_pos); i++ {
		pos := all_pos[i]
		if pos < 0 || pos >= BATTLE_ROUND_MAX_NUM {
			log.Error("Player %v Expedition enemy pos [%v] invalid", p.Id, pos)
			return false
		}

		table_id, _ := db_expe.GetTableId(pos)
		rank, _ := db_expe.GetRank(pos)
		role_card := card_table_mgr.GetRankCard(table_id, rank)
		if role_card == nil {
			log.Error("Cant get card by role_id[%v] and rank[%v]", table_id, rank)
			return false
		}

		m := team_member_pool.Get()
		level, _ := db_expe.GetLevel(pos)
		equips, _ := db_expe.GetEquip(pos)
		m.init_all(this, 0, level, role_card, pos, equips, nil)
		hp, _ := db_expe.GetHP(pos)
		if hp >= 0 {
			m.hp = hp
			m.attrs[ATTR_HP] = hp
		}
		this.members[pos] = m
	}

	this.team_type = BATTLE_TEAM_EXPEDITION_ENEMY

	return true
}

// 神器不能使用技能就增加能量
func (this *BattleTeam) check_artifact_energy_add() bool {
	if this.artifact != nil {
		if this.artifact.energy < BATTLE_TEAM_ARTIFACT_MAX_ENERGY {
			this.artifact.energy += BATTLE_TEAM_ARTIFACT_ADD_ENERGY
			return true
		}
	}
	return false
}

// round start
func (this *BattleTeam) RoundStart() {
	for i := 0; i < BATTLE_TEAM_MEMBER_MAX_NUM; i++ {
		if this.members[i] != nil {
			this.members[i].round_start()
		}
	}
	this.curr_attack = 0
	this.check_artifact_energy_add()
}

// round end
func (this *BattleTeam) RoundEnd() {
	for i := 0; i < BATTLE_TEAM_MEMBER_MAX_NUM; i++ {
		if this.members[i] != nil && !this.members[i].is_dead() {
			this.members[i].round_end()
		}
	}
}

// 获得使用的技能
func (this *BattleTeam) GetTheUseSkill(self *TeamMember, target_team *BattleTeam, trigger_skill int32) (skill *table_config.XmlSkillItem) {
	var skill_id int32

	if trigger_skill == 0 {
		if self.pos < 0 { // 神器
			if self.artifact != nil && self.energy >= BATTLE_TEAM_ARTIFACT_MAX_ENERGY {
				skill_id = self.artifact.SkillId
			}
		} else {
			use_normal := true
			// 能量满用绝杀
			if self.energy >= BATTLE_TEAM_MEMBER_MAX_ENERGY {
				if !self.is_disable_super_attack() {
					use_normal = false
				} else {
					log.Debug("@@@@@@@@@@@!!!!!!!!!!!!!!! Team[%v] member[%v] disable super attack", this.side, self.pos)
				}
			} else {
				if self.is_disable_normal_attack() {
					log.Debug("@@@############## Team[%v] member[%v] disable all attack", this.side, self.pos)
					return
				}
			}

			if use_normal {
				if self.temp_normal_skill > 0 {
					skill_id = self.temp_normal_skill
					self.use_temp_skill = true
				} else {
					if self.card.NormalSkillID == 0 {
						skill_id = self.card.SuperSkillID
					} else {
						skill_id = self.card.NormalSkillID
					}
				}
			} else {
				if self.temp_super_skill > 0 {
					skill_id = self.temp_super_skill
					self.use_temp_skill = true
				} else {
					if self.card.SuperSkillID == 0 {
						skill_id = self.card.NormalSkillID
					} else {
						skill_id = self.card.SuperSkillID
					}
				}
			}
		}
	} else {
		skill_id = trigger_skill
	}

	skill = skill_table_mgr.Get(skill_id)
	if skill == nil {
		log.Error("Self[%v] member[%v] Cant get skill by id[%v] to target[%v]", self.team.side, self.pos, skill_id, target_team.side)
		return
	}

	if self.pos >= 0 && trigger_skill > 0 && self.is_disable_attack() && skill.Type != SKILL_TYPE_PASSIVE {
		log.Debug("############# Team[%v] member[%v] disable combo skill[%v]", this.side, self.pos, trigger_skill)
		return nil
	}

	return
}

// find targets
func (this *BattleTeam) FindTargets(self *TeamMember, target_team *BattleTeam, skill *table_config.XmlSkillItem, passive_trigger_pos []int32) (pos []int32, is_enemy bool) {
	if skill.Type == SKILL_TYPE_NORMAL {

	} else if skill.Type == SKILL_TYPE_SUPER {

	} else if skill.Type == SKILL_TYPE_PASSIVE {
		// 被动触发
	} else if skill.Type == SKILL_TYPE_NEXT {

	} else if skill.Type == SKILL_TYPE_ARTIFACT {

	} else {
		log.Error("Invalid skill type[%v]", skill.Type)
		return
	}

	if skill.SkillEnemy == SKILL_ENEMY_TYPE_ENEMY {
		is_enemy = true
	} else if skill.SkillEnemy == SKILL_ENEMY_TYPE_OUR {
		target_team = this
	} else {
		log.Error("Invalid skill enemy type[%v]", skill.SkillEnemy)
		return
	}

	if skill.SkillTarget == SKILL_TARGET_TYPE_DEFAULT {
		pos = skill_get_default_targets(self.pos, target_team, skill)
	} else if skill.SkillTarget == SKILL_TARGET_TYPE_BACK {
		pos = skill_get_back_targets(self.pos, target_team, skill)
	} else if skill.SkillTarget == SKILL_TARGET_TYPE_HP_MIN {
		pos = skill_get_hp_min_targets(self.pos, target_team, skill)
	} else if skill.SkillTarget == SKILL_TARGET_TYPE_RANDOM {
		pos = skill_get_random_targets(self.pos, target_team, skill)
	} else if skill.SkillTarget == SKILL_TARGET_TYPE_SELF {
		pos = skill_get_force_self_targets(self.pos, target_team, skill)
	} else if skill.SkillTarget == SKILL_TARGET_TYPE_TRIGGER_OBJECT {
		pos = passive_trigger_pos
	} else if skill.SkillTarget == SKILL_TARGET_TYPE_CROPSE {

	} else if skill.SkillTarget == SKILL_TARGET_TYPE_EMPTY_POS {
		pos = skill_get_empty_pos(target_team, skill)
	} else {
		log.Error("Invalid skill target type: %v", skill.SkillTarget)
		return
	}

	return
}

func (this *BattleTeam) get_member(index int32) (mem *TeamMember) {
	if index < 0 {
		mem = this.artifact
	} else {
		mem = this.members[index]
		if mem != nil && mem.is_dead() {
			mem = nil
		}
	}
	return
}

func (this *BattleTeam) UseSkillOnce(self_index int32, target_team *BattleTeam, trigger_skill int32) (skill *table_config.XmlSkillItem) {
	self := this.get_member(self_index)
	if self == nil {
		return nil
	}

	skill = this.GetTheUseSkill(self, target_team, trigger_skill)
	if skill == nil {
		log.Warn("team[%v] member[%v] cant get the use skill", this.side, self_index)
		return
	}

	target_pos, is_enemy := this.FindTargets(self, target_team, skill, nil)
	if target_pos == nil {
		log.Warn("team[%v] member[%v] Cant find targets to attack", this.side, self_index)
		return nil
	}

	log.Debug("team[%v] member[%v] find is_enemy[%v] targets[%v] to use skill[%v]", this.side, self_index, is_enemy, target_pos, skill.Id)

	if !is_enemy {
		target_team = this
	}

	self.used_skill(skill)
	skill_effect(this, self_index, target_team, target_pos, skill)

	// 清除临时技能
	if self_index >= 0 && self.use_temp_skill {
		if self.temp_normal_skill > 0 {
			log.Debug("!!!!!!!!!!!!!!!!!!! Team[%v] mem[%v] clear temp normal skill[%v]", this.side, self_index, self.temp_normal_skill)
			self.temp_normal_skill = 0
		} else if self.temp_super_skill > 0 {
			log.Debug("!!!!!!!!!!!!!!!!!!! Team[%v] mem[%v] clear temp super skill[%v]", this.side, self_index, self.temp_normal_skill)
			self.temp_super_skill = 0
		}
		self.use_temp_skill = false
	}

	// 是否有combo技能
	if skill.ComboSkill > 0 {
		r := this.GetLastReport()
		if r != nil {
			r.HasCombo = true
			log.Debug("########################################### Team[%v] member[%v] 后面有组合技 %v", this.side, self_index, skill.ComboSkill)
		}
	}

	return skill
}

func (this *BattleTeam) UseSkill(self_index int32, target_team *BattleTeam) int32 {
	mem := this.members[self_index]
	if mem == nil || mem.is_dead() || mem.is_will_dead() {
		return -1
	}

	for mem.get_use_skill() > 0 {
		if target_team.IsAllDead() {
			return 0
		}

		mem.act_done()

		if mem.is_disable_attack() {
			return 0
		}

		if mem.energy >= BATTLE_TEAM_MEMBER_MAX_ENERGY {
			// 被动技，怒气攻击前
			if mem.temp_super_skill == 0 {
				passive_skill_effect_with_self_pos(EVENT_BEFORE_RAGE_ATTACK, this, self_index, target_team, nil, true)
			}
		} else {
			// 被动技，普通攻击前
			if mem.temp_normal_skill == 0 {
				passive_skill_effect_with_self_pos(EVENT_BEFORE_NORMAL_ATTACK, this, self_index, target_team, nil, true)
			}
		}

		skill := this.UseSkillOnce(self_index, target_team, 0)
		if skill == nil {
			break
		}
		if skill.ComboSkill > 0 {
			log.Debug("@@@@@@!!!!!! Team[%v] member[%v] will use combo skill[%v]", this.side, self_index, skill.ComboSkill)
			this.UseSkillOnce(self_index, target_team, skill.ComboSkill)
		}
		this.DelaySkillEffect()
	}

	return 1
}

func (this *BattleTeam) CheckAndUseArtifactEveryRound(target_team *BattleTeam) bool {
	if this.artifact == nil || this.player == nil {
		return false
	}

	a := this.artifact.artifact
	if a == nil {
		return false
	}

	self_index := int32(-1)
	skill := this.UseSkillOnce(self_index, target_team, 0)
	if skill == nil {
		return false
	}

	if skill.ComboSkill > 0 {
		log.Debug("@@@@@@!!!!!! Team[%v] artifact %v will use combo skill[%v]", this.side, this.artifact.artifact.Id, skill.ComboSkill)
		this.UseSkillOnce(self_index, target_team, skill.ComboSkill)
	}

	if this.artifact.energy >= BATTLE_TEAM_ARTIFACT_MAX_ENERGY {
		this.artifact.energy -= BATTLE_TEAM_ARTIFACT_MAX_ENERGY
	}

	return true
}

func (this *BattleTeam) _is_slave(index int32) bool {
	if this.members == nil {
		return false
	}
	if int(index) >= len(this.members) {
		return false
	}
	m := this.members[index]
	if m == nil || !m.is_slave {
		return false
	}
	return true
}

func (this *BattleTeam) _fight_pair(self_index, target_index int32, target_team *BattleTeam) (int32, int32) {
	for ; self_index < BATTLE_TEAM_MEMBER_MAX_NUM; self_index++ {
		if this.UseSkill(self_index, target_team) >= 0 {
			if !this._is_slave(self_index) {
				self_index += 1
				break
			}
		}
	}
	for ; target_index < BATTLE_TEAM_MEMBER_MAX_NUM; target_index++ {
		if target_team.UseSkill(target_index, this) >= 0 {
			if !target_team._is_slave(target_index) {
				target_index += 1
				break
			}
		}
	}
	return self_index, target_index
}

// 回合
func (this *BattleTeam) DoRound(target_team *BattleTeam, round *msg_client_message.BattleRoundReports) {
	this.RoundStart()
	target_team.RoundStart()

	if round != nil {
		// 非扫荡
		if !this.IsSweep() {
			if this.artifact != nil {
				round.MyArtifactStartEnergy = this.artifact.energy
			}
			if target_team.artifact != nil {
				round.TargetArtifactStartEnergy = target_team.artifact.energy
			}
		}
	}

	// 被动技，回合行动前触发
	for i := int32(0); i < BATTLE_TEAM_MEMBER_MAX_NUM; i++ {
		passive_skill_effect_with_self_pos(EVENT_BEFORE_ROUND, this, i, target_team, nil, false)
		passive_skill_effect_with_self_pos(EVENT_BEFORE_ROUND, target_team, i, this, nil, false)
	}

	// 检测使用神器
	this.CheckAndUseArtifactEveryRound(target_team)

	var self_index, target_index int32
	for self_index < BATTLE_TEAM_MEMBER_MAX_NUM || target_index < BATTLE_TEAM_MEMBER_MAX_NUM {
		if this.get_first_hand() >= target_team.get_first_hand() {
			self_index, target_index = this._fight_pair(self_index, target_index, target_team)
		} else {
			target_index, self_index = target_team._fight_pair(target_index, self_index, this)
		}
	}

	this.RoundEnd()
	target_team.RoundEnd()

	if round != nil {
		// 非扫荡
		if !this.IsSweep() {
			round.MyMembersEnergy = this.GetMembersEnergy()
			round.TargetMembersEnergy = target_team.GetMembersEnergy()
			round.Reports = this.common_data.reports
			round.RemoveBuffs = this.common_data.remove_buffs
			round.ChangedFighters = this.common_data.changed_fighters
			if this.artifact != nil {
				round.MyArtifactEndEnergy = this.artifact.energy
			}
			if target_team.artifact != nil {
				round.TargetArtifactEndEnergy = target_team.artifact.energy
			}
		}
	}
}

// 结束
func (this *BattleTeam) OnFinish() {
	if this.members == nil {
		return
	}
	for i := 0; i < len(this.members); i++ {
		if this.members[i] != nil {
			this.members[i].on_battle_finish()
			team_member_pool.Put(this.members[i])
			this.members[i] = nil
		}
	}
	if this.player != nil && this.player.assist_member != nil {
		this.player.assist_member = nil
	}
}

func (this *BattleTeam) GetLastReport() (last_report *msg_client_message.BattleReportItem) {
	if this.common_data == nil {
		return
	}

	l := len(this.common_data.reports)
	if l > 0 {
		last_report = this.common_data.reports[l-1]
	}
	return
}

// 人数
func (this *BattleTeam) MembersNum() (num int32) {
	if this.members == nil {
		return
	}
	for i := 0; i < len(this.members); i++ {
		if this.members[i] != nil && !this.members[i].is_dead() {
			num += 1
		}
	}
	return
}

func (this *BattleTeam) GetMembersEnergy() (energys []int32) {
	energys = make([]int32, BATTLE_TEAM_MEMBER_MAX_NUM)
	for i := 0; i < len(energys); i++ {
		if this.members[i] != nil && !this.members[i].is_dead() {
			energys[i] = this.members[i].energy
		}
	}
	return
}

// 好友BOSS更新血量
func (this *BattleTeam) UpdateFriendBossHP() {
	if this.friend == nil {
		return
	}

	var percent int32
	var boss *TeamMember
	for i := int32(0); i < BATTLE_TEAM_MEMBER_MAX_NUM; i++ {
		m := this.members[i]
		if m == nil {
			continue
		}
		if !this.friend.db.FriendBosss.HasIndex(i) {
			continue
		}
		if m.is_dead() {
			this.friend.db.FriendBosss.Remove(i)
			continue
		}
		this.friend.db.FriendBosss.SetMonsterHp(i, m.hp)
		this.friend.db.FriendBosss.SetMonsterMaxHp(i, m.attrs[ATTR_HP_MAX])
		boss = m
	}
	if boss != nil {
		if boss.hp > 0 {
			percent = int32(100 * int64(boss.hp) / int64(boss.attrs[ATTR_HP_MAX]))
			if percent <= 0 {
				percent = 1
			}
		}
		this.friend.db.FriendCommon.SetFriendBossHpPercent(percent)
		log.Debug("!!!!!!!!!!!!!!!!!!!!!!!! Update player[%v] friend boss hp percent %v", this.friend.Id, percent)
	}
}

// 是否扫荡
func (this *BattleTeam) IsSweep() bool {
	if this.player != nil && this.player.sweep_num > 0 {
		return true
	}
	return false
}

// 公会副本BOSS血量更新
func (this *BattleTeam) UpdateGuildStageBossHP() {
	if this.guild == nil {
		return
	}

	pos := this.guild.Stage.GetBossPos()
	if pos < 0 || pos >= BATTLE_TEAM_MEMBER_MAX_NUM {
		return
	}
	boss := this.members[pos]
	if boss == nil {
		return
	}
	this.guild.Stage.SetBossHP(boss.hp)
	var percent int32
	if boss.hp > 0 {
		percent = int32(100 * int64(boss.hp) / int64(boss.attrs[ATTR_HP_MAX]))
		if percent <= 0 {
			percent = 1
		}
	}
	this.guild.Stage.SetHpPercent(percent)
	log.Debug("!!!!!!!!!!!!!!!!!!!!!!!! Update guild[%v] stage boss hp percent %v", this.guild.GetId(), percent)
}

// 开打
func (this *BattleTeam) Fight(target_team *BattleTeam, end_type int32, end_param int32) (is_win bool, enter_reports []*msg_client_message.BattleReportItem, rounds []*msg_client_message.BattleRoundReports) {
	round_max := end_param
	if end_type == BATTLE_END_BY_ALL_DEAD {
		round_max = BATTLE_ROUND_MAX_NUM
	} else if end_type == BATTLE_END_BY_ROUND_OVER {
		round_max = end_param
	}

	// 存放战报
	if this.common_data == nil {
		this.common_data = &BattleCommonData{}
		this.common_data.Init()
	}
	// 非扫荡或扫荡第一次
	if !(this.player != nil && this.player.curr_sweep > 0) {
		this.common_data.reset_damage_data()
	}
	target_team.common_data = this.common_data
	this.common_data.Reset()
	this.common_data.round_num = 0

	// 被动技，进场前触发
	for i := int32(0); i < BATTLE_TEAM_MEMBER_MAX_NUM; i++ {
		passive_skill_effect_with_self_pos(EVENT_ENTER_BATTLE, this, i, target_team, nil, false)
		passive_skill_effect_with_self_pos(EVENT_ENTER_BATTLE, target_team, i, this, nil, false)
	}

	// 非扫荡
	if !this.IsSweep() && this.common_data.reports != nil {
		enter_reports = this.common_data.reports
		this.common_data.reports = make([]*msg_client_message.BattleReportItem, 0)
	}

	rand.Seed(time.Now().Unix())
	for c := int32(0); c < round_max; c++ {
		log.Debug("----------------------------------------------- Round[%v] --------------------------------------------", c+1)

		round := msg_battle_round_reports_pool.Get()

		this.common_data.round_num += 1
		this.DoRound(target_team, round)

		if !this.IsSweep() {
			round.RoundNum = c + 1
			rounds = append(rounds, round)
		}

		if this.IsAllDead() {
			log.Debug("self all dead")
			break
		}
		if target_team.IsAllDead() {
			is_win = true
			log.Debug("target all dead")
			break
		}

		this.common_data.Reset()
	}

	// 好友BOSS血量更新
	if target_team.friend != nil {
		target_team.UpdateFriendBossHP()
	}
	// 公会副本BOSS血量更新
	if target_team.guild != nil {
		target_team.UpdateGuildStageBossHP()
	}

	// 远征
	if this.team_type == BATTLE_TEAM_EXPEDITION {
		if !is_win && target_team.IsAllDead() {
			is_win = true
		}
		this.player.expedition_update_self_roles(is_win, this.members)
	}
	if target_team.team_type == BATTLE_TEAM_EXPEDITION_ENEMY {
		this.player.expedition_update_enemy_roles(target_team.members)
	}

	// 扫荡
	if this.IsSweep() {
		this.player.curr_sweep += 1
	}

	this.OnFinish()
	target_team.OnFinish()

	return
}

func (this *BattleTeam) _format_members_for_msg() (members []*msg_client_message.BattleMemberItem) {
	for i := 0; i < len(this.members); i++ {
		if this.members[i] == nil {
			continue
		}
		mem := this.members[i].build_battle_member()
		mem.Side = this.side
		members = append(members, mem)
	}
	return
}

// 是否全挂
func (this *BattleTeam) IsAllDead() bool {
	all_dead := true
	for i := 0; i < BATTLE_TEAM_MEMBER_MAX_NUM; i++ {
		if this.members[i] == nil {
			continue
		}
		if !this.members[i].is_dead() {
			all_dead = false
			break
		}
	}
	return all_dead
}

// 是否有某个角色
func (this *BattleTeam) HasRole(role_id int32) bool {
	for i := 0; i < BATTLE_TEAM_MEMBER_MAX_NUM; i++ {
		if this.members[i] == nil {
			continue
		}
		if this.members[i].card.Id == role_id {
			return true
		}
	}
	return false
}

// 延迟被动技
func (this *BattleTeam) PushDelaySkill(trigger_event int32, skill *table_config.XmlSkillItem, user *TeamMember, target_team *BattleTeam, trigger_pos []int32) {
	if this.common_data == nil {
		return
	}

	ds := delay_skill_pool.Get()
	ds.trigger_event = trigger_event
	ds.skill = skill
	ds.user = user
	ds.target_team = target_team
	ds.trigger_pos = trigger_pos
	ds.next = nil

	dl := this.common_data.delay_skill_list
	if dl == nil {
		dl = &DelaySkillList{}
		this.common_data.delay_skill_list = dl
	}
	if dl.head == nil {
		dl.head = ds
		dl.tail = ds
	} else {
		dl.tail.next = ds
		dl.tail = ds
	}

	log.Debug("############ Team[%v] member[%v] 推入了延迟被动技[%v]", user.team.side, user.pos, skill.Id)
}

// 处理延迟被动技
func (this *BattleTeam) DelaySkillEffect() {
	if this.common_data == nil {
		return
	}
	dl := this.common_data.delay_skill_list
	if dl == nil {
		return
	}

	d := dl.head
	for d != nil {
		one_passive_skill_effect(d.trigger_event, d.skill, d.user, d.target_team, d.trigger_pos, true)
		n := d.next
		delay_skill_pool.Put(d)
		d = n
	}
	dl.head = nil
	dl.tail = nil
}

// 是否有延迟技
func (this *BattleTeam) HasDelayTriggerEventSkill(trigger_event int32, behiter *TeamMember) bool {
	if this.common_data == nil {
		return false
	}
	dl := this.common_data.delay_skill_list
	if dl == nil {
		return false
	}
	d := dl.head
	for d != nil {
		if d.trigger_event == trigger_event && d.user == behiter {
			return true
		}
		d = d.next
	}
	return false
}

func (this *Player) send_battle_team(tt int32, team_members []int32) {
	response := &msg_client_message.S2CSetTeamResponse{}
	response.TeamType = tt
	response.TeamMembers = team_members
	this.Send(uint16(msg_client_message_id.MSGID_S2C_SET_TEAM_RESPONSE), response)
}

const (
	PVP_TEAM_MAX_MEMBER_NUM = 4
)

func (this *Player) fight(team_members []int32, battle_type, battle_param, assist_friend_id, assist_role_id, assist_pos, artifact_id int32) int32 {
	if battle_type == 1 && this.Id == battle_param {
		log.Error("Cant fight with self")
		return -1
	}

	log.Debug("Player[%v] fight battle_type[%v], battle_param[%v], sweep_num[%v], team members: %v, assist_friend_id: %v  assist_role_id: %v, assist_pos: %v, artifact: %v", this.Id, battle_type, battle_param, this.sweep_num, team_members, assist_friend_id, assist_role_id, assist_pos, artifact_id)

	// 助战
	if assist_friend_id > 0 && this.db.Friends.HasIndex(assist_friend_id) {
		assist_friend := player_mgr.GetPlayerById(assist_friend_id)
		if assist_friend != nil {
			if assist_friend.db.Roles.HasIndex(assist_role_id) && assist_friend.db.FriendCommon.GetAssistRoleId() == assist_role_id {
				if assist_pos >= 0 && assist_pos < BATTLE_TEAM_MEMBER_MAX_NUM {
					this.assist_friend = assist_friend
					this.assist_role_id = assist_role_id
					this.assist_role_pos = assist_pos

					if team_members != nil && len(team_members) > int(this.assist_role_pos) {
						team_members[this.assist_role_pos] = 0
					}
				}
			}
		}
	}

	if team_members != nil && len(team_members) > 0 {
		if battle_type == 1 || battle_type == 8 {
			res := this.SetTeam(BATTLE_TEAM_ATTACK, team_members, artifact_id)
			if res < 0 {
				this.assist_friend = nil
				log.Error("Player[%v] set attack team failed", this.Id)
				return res
			}
		} else if battle_type == 2 {
			res := this.SetCampaignTeam(team_members, artifact_id)
			if res < 0 {
				this.assist_friend = nil
				log.Error("Player[%v] set campaign members[%v] failed", this.Id, team_members)
				return res
			}
			this.send_teams()
		} else {
			team_type := int32(-1)
			if battle_type == 3 {
				// 爬塔阵容
				team_type = BATTLE_TEAM_TOWER
			} else if battle_type == 4 {
				// 活动副本阵容
				team_type = BATTLE_TEAM_ACTIVE_STAGE
			} else if battle_type == 5 {
				// 好友BOSS
				team_type = BATTLE_TEAM_FRIEND_BOSS
			} else if battle_type == 6 || battle_type == 7 {
				// 探索任务
				team_type = BATTLE_TEAM_EXPLORE
			} else if battle_type == 9 {
				// 公会副本
				team_type = BATTLE_TEAM_GUILD_STAGE
			} else if battle_type == 10 {
				// 远征
				team_type = BATTLE_TEAM_EXPEDITION
			} else {
				this.assist_friend = nil
				log.Error("Player[%v] set battle_type[%v] team[%v] invalid", this.Id, battle_type, team_type)
				return -1
			}

			res := this.SetTeam(team_type, team_members, artifact_id)
			if res < 0 {
				this.assist_friend = nil
				log.Error("Player[%v] set team[%v:%v] failed", this.Id, team_type, team_members)
				return res
			}
		}
	}

	var res int32
	if battle_type == 1 || battle_type == 8 {
		res = this.Fight2Player(battle_type, battle_param)
	} else if battle_type == 2 {
		res = this.FightInCampaign(battle_param)
	} else if battle_type == 3 {
		res = this.fight_tower(battle_param)
	} else if battle_type == 4 {
		res = this.fight_active_stage(battle_param)
	} else if battle_type == 5 {
		res = this.friend_boss_challenge(battle_param)
	} else if battle_type == 6 {
		res = this.explore_fight(battle_param, false)
	} else if battle_type == 7 {
		res = this.explore_fight(battle_param, true)
	} else if battle_type == 9 {
		res = this.guild_stage_fight(battle_param)
	} else if battle_type == 10 {
		res = this.expedition_fight()
	} else {
		res = -1
	}

	if this.assist_friend != nil {
		this.assist_friend = nil
	}
	if this.assist_member != nil {
		this.assist_member = nil
	}

	if res > 0 {
		if battle_type == 1 {
			//this.send_battle_team(BATTLE_ATTACK_TEAM, team_members)
		} else if battle_type == 2 {
			this.send_battle_team(BATTLE_TEAM_CAMPAIN, team_members)
		} else if battle_type == 3 {
			this.send_battle_team(BATTLE_TEAM_TOWER, team_members)
		}
	}

	return res
}

const PLAYER_SWEEP_MAX_NUM int32 = 10

func C2SFightHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SBattleResultRequest
	err := proto.Unmarshal(msg_data, &req)
	if nil != err {
		log.Error("Unmarshal msg failed err(%s) !", err.Error())
		return -1
	}
	if req.GetFightPlayerId() > 0 {
		req.BattleType = 1
		req.BattleParam = req.GetFightPlayerId()
	} else if req.GetCampaignId() > 0 {
		req.BattleType = 2
		req.BattleParam = req.GetCampaignId()
	}

	if req.GetSweepNum() < 0 || req.GetSweepNum() > PLAYER_SWEEP_MAX_NUM {
		log.Error("Player[%v] sweep num %v invalid", p.Id, req.GetSweepNum())
		return -1
	}

	p.sweep_num = req.GetSweepNum()
	p.curr_sweep = 0
	return p.fight(req.GetAttackMembers(), req.GetBattleType(), req.GetBattleParam(), req.GetAssistFriendId(), req.GetAssistRoleId(), req.GetAssistPos(), req.GetAritfactId())
}

func C2SSetTeamHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SSetTeamRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s) !", err.Error())
		return -1
	}

	var res int32
	tt := req.GetTeamType()
	if tt == BATTLE_TEAM_ATTACK {
		//res = p.SetAttackTeam(req.TeamMembers)
	} else if tt == BATTLE_TEAM_DEFENSE {
		res = p.SetTeam(BATTLE_TEAM_DEFENSE, req.GetTeamMembers(), req.GetArtifactId())
		if res > 0 {
			top_power_match_manager.Update(p.Id, p.get_defense_team_power())
		}
	} else if tt == BATTLE_TEAM_CAMPAIN {
		res = p.SetTeam(BATTLE_TEAM_CAMPAIN, req.GetTeamMembers(), req.GetArtifactId())
	} else {
		log.Warn("Unknown team type[%v] to player[%v]", tt, p.Id)
	}

	p.send_battle_team(tt, req.TeamMembers)

	return res
}

func C2SSetHangupCampaignHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SBattleSetHangupCampaignRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s) !", err.Error())
		return -1
	}

	res := p.set_hangup_campaign_id(req.GetCampaignId())
	if res < 0 {
		log.Debug("Player[%v] set hangup campaign %v failed[%v]", p.Id, req.GetCampaignId(), res)
		return res
	}

	response := &msg_client_message.S2CBattleSetHangupCampaignResponse{}
	response.CampaignId = req.GetCampaignId()
	p.Send(uint16(msg_client_message_id.MSGID_S2C_BATTLE_SET_HANGUP_CAMPAIGN_RESPONSE), response)

	log.Debug("Player[%v] set hangup campaign %v success", p.Id, req.GetCampaignId())

	return 1
}

func C2SCampaignHangupIncomeHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SCampaignHangupIncomeRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s) !", err.Error())
		return -1
	}

	t := req.GetIncomeType()
	p.campaign_hangup_income_get(t, false)
	return 1
}

func C2SCampaignDataHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SCampaignDataRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s) !", err.Error())
		return -1
	}
	p.send_campaigns()
	return 1
}
