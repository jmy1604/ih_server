package main

import (
	"ih_server/libs/log"
	"ih_server/src/table_config"
	"math/rand"
)

// 基础属性
const (
	ATTR_HP_MAX             = 1  // 最大血量
	ATTR_HP                 = 2  // 当前血量
	ATTR_MP                 = 3  // 气势
	ATTR_ATTACK             = 4  // 攻击
	ATTR_DEFENSE            = 5  // 防御
	ATTR_DODGE_COUNT        = 6  // 闪避次数
	ATTR_INJURED_MAX        = 7  // 受伤上限
	ATTR_SHIELD             = 8  // 护盾
	ATTR_CRITICAL           = 9  // 暴击率
	ATTR_CRITICAL_MULTI     = 10 // 暴击伤害倍率
	ATTR_ANTI_CRITICAL      = 11 // 抗暴率
	ATTR_BLOCK_RATE         = 12 // 格挡率
	ATTR_BLOCK_DEFENSE_RATE = 13 // 格挡减伤率
	ATTR_BREAK_BLOCK_RATE   = 14 // 破格率

	ATTR_TOTAL_DAMAGE_ADD      = 15 // 总增伤
	ATTR_CLOSE_DAMAGE_ADD      = 16 // 近战增伤
	ATTR_REMOTE_DAMAGE_ADD     = 17 // 远程增伤
	ATTR_NORMAL_DAMAGE_ADD     = 18 // 普攻增伤
	ATTR_RAGE_DAMAGE_ADD       = 19 // 怒气增伤
	ATTR_TOTAL_DAMAGE_SUB      = 20 // 总减伤
	ATTR_CLOSE_DAMAGE_SUB      = 21 // 近战减伤
	ATTR_REMOTE_DAMAGE_SUB     = 22 // 远程减伤
	ATTR_NORMAL_DAMAGE_SUB     = 23 // 普攻减伤
	ATTR_RAGE_DAMAGE_SUB       = 24 // 怒气减伤
	ATTR_CLOSE_VAMPIRE         = 25 // 近战吸血
	ATTR_REMOTE_VAMPIRE        = 26 // 远程吸血
	ATTR_CURE_RATE_CORRECT     = 27 // 治疗率修正
	ATTR_CURED_RATE_CORRECT    = 28 // 被治疗率修正
	ATTR_CLOSE_REFLECT         = 29 // 近战反击系数
	ATTR_REMOTE_REFLECT        = 30 // 远程反击系数
	ATTR_ARMOR_ADD             = 31 // 护甲增益
	ATTR_BREAK_ARMOR           = 32 // 破甲
	ATTR_POISON_INJURED_RESIST = 33 // 毒气受伤抗性
	ATTR_BURN_INJURED_RESIST   = 34 // 点燃受伤抗性
	ATTR_BLEED_INJURED_RESIST  = 35 // 流血受伤抗性
	ATTR_HP_PERCENT_BONUS      = 36 // 血量百分比
	ATTR_ATTACK_PERCENT_BONUS  = 37 // 攻击百分比
	ATTR_DEFENSE_PERCENT_BONUS = 38 // 防御百分比
	ATTR_DAMAGE_PERCENT_BONUS  = 39 // 伤害百分比
	ATTR_COUNT_MAX             = 40
)

// 战斗结束类型
const (
	BATTLE_END_BY_ALL_DEAD   = 1 // 一方全死
	BATTLE_END_BY_ROUND_OVER = 2 // 回合用完
)

// 最大回合数
const (
	BATTLE_ROUND_MAX_NUM = 30
)

const (
	BATTLE_TEAM_MEMBER_INIT_ENERGY       = 30 // 初始能量
	BATTLE_TEAM_MEMBER_MAX_ENERGY        = 60 // 最大能量
	BATTLE_TEAM_MEMBER_ADD_ENERGY        = 20 // 能量增加量
	BATTLE_TEAM_MEMBER_MAX_NUM           = 9  // 最大人数
	BATTLE_FORMATION_LINE_NUM            = 3  // 阵型列数
	BATTLE_FORMATION_ONE_LINE_MEMBER_NUM = 3  // 每列人数
)

// 阵容类型
const (
	BATTLE_ATTACK_TEAM       = 1 // pvp attack
	BATTLE_DEFENSE_TEAM      = 2 // pvp defense
	BATTLE_CAMPAIN_TEAM      = 3 // campaign
	BATTLE_TOWER_TEAM        = 4 // tower
	BATTLE_ACTIVE_STAGE_TEAM = 5 // active stage
	BATTLE_FRIEND_BOSS_TEAM  = 6 // friend boss
	BATTLE_EXPLORE_TEAM      = 7 // explore
	BATTLE_GUILD_STAGE_TEAM  = 8 // guild stage
	BATTLE_MAX_TEAM          = 100
)

const (
	USE_PASSIVE_LIST = false
)

type PassiveTriggerData struct {
	skill      *table_config.XmlSkillItem
	battle_num int32
	round_num  int32
	next       *PassiveTriggerData
}

type PassiveTriggerDataList struct {
	head *PassiveTriggerData
	tail *PassiveTriggerData
}

func (this *PassiveTriggerDataList) clear() {
	t := this.head
	for t != nil {
		n := t.next
		passive_trigger_data_pool.Put(t)
		t = n
	}
	this.head = nil
	this.tail = nil
}

func (this *PassiveTriggerDataList) push_back(node *PassiveTriggerData) {
	if this.head == nil {
		this.head = node
		this.tail = node
	} else {
		this.tail.next = node
		this.tail = node
	}
}

func (this *PassiveTriggerDataList) remove(pnode, node *PassiveTriggerData) bool {
	if pnode != nil && pnode.next != node {
		log.Warn("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX pnode's next node is not node")
		return false
	}

	if node == this.head {
		this.head = node.next
	}
	if pnode != nil {
		pnode.next = node.next
	}
	if node == this.tail {
		this.tail = pnode
	}
	return true
}

func (this *PassiveTriggerDataList) remove_by_skill(skill_id int32) bool {
	var p *PassiveTriggerData
	d := this.head
	for d != nil {
		if d.skill.Id == skill_id {
			if this.remove(p, d) {
				passive_trigger_data_pool.Put(d)
				return true
			}
		}
		p = d
		d = d.next
	}
	return false
}

func (this *PassiveTriggerDataList) event_num() int32 {
	n := int32(0)
	t := this.head
	for t != nil {
		if t.battle_num != 0 && t.round_num != 0 {
			n += 1
			break
		}
		t = t.next
	}
	return n
}

func (this *PassiveTriggerDataList) can_trigger(skill_id int32) bool {
	t := this.head
	for t != nil {
		if t.skill.Id == skill_id && t.battle_num != 0 && t.round_num != 0 {
			return true
		}
		t = t.next
	}
	return false
}

func (this *PassiveTriggerDataList) used(skill_id int32) (can_delete bool) {
	//var p *PassiveTriggerData
	t := this.head
	for t != nil {
		if t.skill.Id == skill_id {
			if t.battle_num > 0 {
				t.battle_num -= 1
				log.Debug("减少一次技能[%v]战斗触发事件次数", skill_id)
			}
			if t.round_num > 0 {
				t.round_num -= 1
				log.Debug("减少一次技能[%v]回合触发事件次数", skill_id)
			}
			if t.battle_num == 0 || t.round_num == 0 {
				// 不用删除，留着下一回合初始化时用
				/*if this.remove(p, t) {
					passive_trigger_data_pool.Put(t)
					can_delete = true
				}*/
			}
			break
		}
		//p = t
		t = t.next
	}
	return
}

type DelaySkill struct {
	trigger_event int32
	skill         *table_config.XmlSkillItem
	user          *TeamMember
	target_team   *BattleTeam
	trigger_pos   []int32
	next          *DelaySkill
}

type TeamMember struct {
	team                    *BattleTeam
	pos                     int32
	id                      int32
	level                   int32
	card                    *table_config.XmlCardItem
	hp                      int32
	energy                  int32
	attack                  int32
	defense                 int32
	act_num                 int32                             // 行动次数
	attrs                   []int32                           // 属性
	bufflist_arr            []*BuffList                       // BUFF
	passive_triggers        map[int32][]*PassiveTriggerData   // 被动技触发事件
	passive_trigger_lists   map[int32]*PassiveTriggerDataList // 被动技触发事件
	temp_normal_skill       int32                             // 临时普通攻击
	temp_super_skill        int32                             // 临时怒气攻击
	use_temp_skill          bool                              // 是否使用临时技能
	temp_changed_attrs      map[int32]int32                   // 临时改变的属性
	temp_changed_attrs_used int32                             // 临时改变属性计算状态 0 忽略 1 已初始化 2 已计算
	passive_skills          map[int32]int32                   // 被动技
	attacker                *TeamMember                       // 攻击者
	attacker_skill_data     *table_config.XmlSkillItem        // 攻击者使用的技能
}

func (this *TeamMember) add_attrs(attrs []int32) {
	for i := 0; i < len(attrs)/2; i++ {
		attr := attrs[2*i]
		this.add_attr(attr, attrs[2*i+1])
	}
}

func (this *TeamMember) add_skill_attr(skill_id int32) {
	skill := skill_table_mgr.Get(skill_id)
	if skill == nil {
		return
	}
	this.add_attrs(skill.SkillAttr)
	//log.Debug("!!!!!!!!!!!!! add skill[%v] attrs[%v]", skill_id, skill.SkillAttr)
}

func (this *TeamMember) init_passive_data(skills []int32) {
	if skills == nil {
		return
	}
	for i := 0; i < len(skills); i++ {
		if !this.add_passive_trigger(skills[i]) {
			log.Warn("Team[%v] member[%v] add passive skill[%v] failed", this.team.side, this.pos, skills[i])
		} else {
			log.Debug("Team[%v] member[%v] add passive skill[%v]", this.team.side, this.pos, skills[i])
		}
	}
}

func (this *TeamMember) init_passive_round_num() bool {
	if USE_PASSIVE_LIST {
		if this.passive_triggers == nil {
			return false
		}
		for _, d := range this.passive_triggers {
			for i := 0; i < len(d); i++ {
				if d[i] != nil && d[i].skill.TriggerRoundMax > 0 {
					d[i].round_num = d[i].skill.TriggerRoundMax
				}
			}
		}
	} else {
		if this.passive_trigger_lists == nil {
			return false
		}
		for _, d := range this.passive_trigger_lists {
			t := d.head
			for t != nil {
				if t.skill.TriggerRoundMax > 0 {
					t.round_num = t.skill.TriggerRoundMax
				}
				t = t.next
			}
		}
	}
	return true
}

func (this *TeamMember) add_passive_trigger(skill_id int32) bool {
	skill := skill_table_mgr.Get(skill_id)
	if skill == nil || skill.Type != SKILL_TYPE_PASSIVE {
		return false
	}

	if this.passive_skills == nil {
		this.passive_skills = make(map[int32]int32)
	}
	if _, o := this.passive_skills[skill_id]; o {
		log.Warn("########### Team[%v] member[%v] already has passive skill %v", this.team.side, this.pos, skill_id)
		return false
	}

	d := passive_trigger_data_pool.Get()
	d.skill = skill
	d.battle_num = skill.TriggerBattleMax
	d.round_num = skill.TriggerRoundMax
	if d.battle_num == 0 {
		d.battle_num = -1
	}
	if d.round_num == 0 {
		d.round_num = -1
	}
	d.next = nil

	// ***********************************************
	if USE_PASSIVE_LIST {
		if this.passive_triggers == nil {
			this.passive_triggers = make(map[int32][]*PassiveTriggerData)
		}

		datas := this.passive_triggers[skill.SkillTriggerType]
		if datas == nil {
			this.passive_triggers[skill.SkillTriggerType] = []*PassiveTriggerData{d}
		} else {
			this.passive_triggers[skill.SkillTriggerType] = append(datas, d)
		}
	} else {
		// ************************************************
		if this.passive_trigger_lists == nil {
			this.passive_trigger_lists = make(map[int32]*PassiveTriggerDataList)
		}
		trigger_list := this.passive_trigger_lists[skill.SkillTriggerType]
		if trigger_list == nil {
			trigger_list = &PassiveTriggerDataList{}
			this.passive_trigger_lists[skill.SkillTriggerType] = trigger_list
		}
		trigger_list.push_back(d)
	}
	// ************************************************
	this.passive_skills[skill_id] = skill_id

	return true
}

func (this *TeamMember) delete_passive_trigger(skill_id int32) bool {
	skill := skill_table_mgr.Get(skill_id)
	if skill == nil || skill.Type != SKILL_TYPE_PASSIVE {
		return false
	}

	if this.passive_skills == nil {
		this.passive_skills = make(map[int32]int32)
	}
	if _, o := this.passive_skills[skill_id]; !o {
		return false
	}

	// ***********************************************************
	var d *PassiveTriggerData
	if USE_PASSIVE_LIST {
		if this.passive_triggers == nil {
			return false
		}
		triggers := this.passive_triggers[skill.SkillTriggerType]
		if triggers == nil {
			return false
		}
		l := len(triggers)
		i := l - 1
		for ; i >= 0; i-- {
			if triggers[i] == nil {
				continue
			}
			if triggers[i].skill.Id == skill_id {
				d = triggers[i]
				triggers[i] = nil
				break
			}
		}
		passive_trigger_data_pool.Put(d)
		if i >= 0 {
			for n := i; n < l-1; n++ {
				triggers[n] = triggers[n+1]
			}
			if l > 1 {
				this.passive_triggers[skill.SkillTriggerType] = triggers[:l-1]
			} else {
				delete(this.passive_triggers, skill.SkillTriggerType)
			}
		}
		delete(this.passive_skills, skill_id)
	} else {
		if this.passive_trigger_lists == nil {
			return false
		}
		trigger_list := this.passive_trigger_lists[skill.SkillTriggerType]
		if trigger_list == nil {
			return false
		}

		if trigger_list.remove_by_skill(skill.Id) {
			delete(this.passive_skills, skill_id)
		}
	}
	// ***********************************************************

	return true
}

func (this *TeamMember) can_passive_trigger(trigger_event int32, skill_id int32) (trigger bool) {
	// *************************************************
	if USE_PASSIVE_LIST {
		d, o := this.passive_triggers[trigger_event]
		if !o || d == nil {
			return
		}

		for i := 0; i < len(d); i++ {
			if d[i] == nil {
				continue
			}
			if d[i].skill.Id != skill_id {
				continue
			}
			if d[i].battle_num != 0 && d[i].round_num != 0 {
				trigger = true
			}
			break
		}
	} else {
		trigger_list := this.passive_trigger_lists[trigger_event]
		if trigger_list == nil {
			return
		}
		trigger = trigger_list.can_trigger(skill_id)
	}
	// *************************************************

	return
}

func (this *TeamMember) used_passive_trigger_count(trigger_event int32, skill_id int32) {
	// ************************************************************************
	if USE_PASSIVE_LIST {
		d, o := this.passive_triggers[trigger_event]
		if !o || d == nil {
			return
		}

		for i := 0; i < len(d); i++ {
			if d[i] != nil && d[i].skill.Id == skill_id {
				if d[i].battle_num > 0 {
					d[i].battle_num -= 1
					log.Debug("Team[%v] member[%v] 减少一次技能[%v]战斗触发事件[%v]次数", this.team.side, this.pos, skill_id, trigger_event)
				}
				if d[i].round_num > 0 {
					d[i].round_num -= 1
					log.Debug("Team[%v] member[%v] 减少一次技能[%v]回合触发事件[%v]次数", this.team.side, this.pos, skill_id, trigger_event)
				}
				if d[i].battle_num == 0 || d[i].round_num == 0 {
					//passive_trigger_data_pool.Put(d[i])
				}
				break
			}
		}
	} else {
		trigger_list := this.passive_trigger_lists[trigger_event]
		if trigger_list == nil {
			return
		}
		if trigger_list.used(skill_id) {

		}
	}

	// ************************************************************************
}

func (this *TeamMember) has_trigger_event(trigger_events []int32) bool {
	n := int32(0)
	for i := 0; i < len(trigger_events); i++ {
		// *************************************************
		if USE_PASSIVE_LIST {
			d, o := this.passive_triggers[trigger_events[i]]
			if !o || d == nil {
				break
			}
			for j := 0; j < len(d); j++ {
				if d[i] == nil {
					continue
				}
				if d[i].battle_num != 0 && d[i].round_num != 0 {
					n += 1
				}
				break
			}
		} else {
			trigger_list := this.passive_trigger_lists[trigger_events[i]]
			if trigger_list == nil {
				break
			}
			n += trigger_list.event_num()
		}
		// *************************************************
	}
	if int(n) != len(trigger_events) {
		return false
	}
	return true
}

func (this *TeamMember) add_passive_skill(skill_id int32) {
	this.add_passive_trigger(skill_id)
	this.add_skill_attr(skill_id)
}

func (this *TeamMember) init_equip(equip_id int32) {
	d := item_table_mgr.Get(equip_id)
	if d == nil {
		return
	}
	this.init_passive_data(d.EquipSkill)
	if d.EquipSkill != nil {
		for i := 0; i < len(d.EquipSkill); i++ {
			this.add_skill_attr(d.EquipSkill[i])
		}
	}
	this.add_attrs(d.EquipAttr)
	log.Debug("@@@@@@@@@@@@@@############## team[%v] member[%v] init equip [%v] skill[%v] attrs[%v]", this.team.side, this.pos, equip_id, d.EquipSkill, d.EquipAttr)
}

func (this *TeamMember) init_equips(equips []int32) {
	/*if this.team == nil || this.team.player == nil {
		return
	}
	equips, o := this.team.player.db.Roles.GetEquip(this.id)
	if !o {
		return
	}*/
	if equips == nil || len(equips) == 0 {
		return
	}
	for i := 0; i < len(equips); i++ {
		this.init_equip(equips[i])
	}
}

func (this *TeamMember) calculate_max_hp() {
	max_hp := int64(this.attrs[ATTR_HP_MAX]+this.card.BaseHP+(this.level-1)*this.card.GrowthHP/100) * int64(10000+this.attrs[ATTR_HP_PERCENT_BONUS]) / 10000
	this.attrs[ATTR_HP_MAX] = int32(max_hp)
	this.attrs[ATTR_HP] = this.attrs[ATTR_HP_MAX]
	this.hp = this.attrs[ATTR_HP]
}

func (this *TeamMember) calculate_attack() {
	attack := int64(this.attack+this.card.BaseAttack+(this.level-1)*this.card.GrowthAttack/100) * int64(10000+this.attrs[ATTR_ATTACK_PERCENT_BONUS]) / 10000
	this.attrs[ATTR_ATTACK] = int32(attack)
	this.attack = this.attrs[ATTR_ATTACK]
}

func (this *TeamMember) calculate_defense() {
	defense := int64(this.defense+this.card.BaseDefence+(this.level-1)*this.card.GrowthDefence/100) * int64(10000+this.attrs[ATTR_DEFENSE_PERCENT_BONUS]) / 10000
	this.attrs[ATTR_DEFENSE] = int32(defense)
	this.defense = this.attrs[ATTR_DEFENSE]
}

func (this *TeamMember) calculate_hp_attack_defense() {
	this.calculate_max_hp()
	this.calculate_attack()
	this.calculate_defense()
}

func (this *TeamMember) init_attrs_equips_skills(level int32, role_card *table_config.XmlCardItem, equips, extra_equips []int32) {
	if this.attrs == nil {
		this.attrs = make([]int32, ATTR_COUNT_MAX)
	} else {
		for i := 0; i < len(this.attrs); i++ {
			this.attrs[i] = 0
		}
	}

	this.passive_skills = make(map[int32]int32)

	this.level = level
	this.card = role_card

	// 技能增加属性
	if role_card.NormalSkillID > 0 {
		this.add_skill_attr(role_card.NormalSkillID)
	}
	if role_card.SuperSkillID > 0 {
		this.add_skill_attr(role_card.SuperSkillID)
	}
	for i := 0; i < len(role_card.PassiveSkillIds); i++ {
		this.add_skill_attr(role_card.PassiveSkillIds[i])
	}

	this.init_passive_data(role_card.PassiveSkillIds)
	this.init_equips(equips)
	this.init_equips(extra_equips)
}

func (this *TeamMember) init_with_team(team *BattleTeam, id int32, pos int32) {
	this.team = team
	this.id = id
	this.pos = pos
	this.energy = global_config.InitEnergy
	if this.energy == 0 {
		this.energy = BATTLE_TEAM_MEMBER_INIT_ENERGY
	}
	this.act_num = 0
}

func (this *TeamMember) init_all_no_calc(team *BattleTeam, id int32, level int32, role_card *table_config.XmlCardItem, pos int32, equips, extra_equips []int32) {
	this.init_with_team(team, id, pos)
	if this.bufflist_arr != nil {
		for i := 0; i < len(this.bufflist_arr); i++ {
			this.bufflist_arr[i].clear()
			this.bufflist_arr[i].owner = this
		}
	}
	this.hp = 0
	this.attack = 0
	this.defense = 0
	this.init_attrs_equips_skills(level, role_card, equips, extra_equips)
	if team != nil && team.player != nil {
		team.player.role_update_suit_attr_power(id, true, false)
		if id > 0 {
			team.player.add_talent_attr(this)
		} else if id < 0 {
			if team.player.assist_friend != nil {
				team.player.assist_friend.add_talent_attr(this)
			}
		}
	}
}

func (this *TeamMember) init_all(team *BattleTeam, id int32, level int32, role_card *table_config.XmlCardItem, pos int32, equips, extra_equips []int32) {
	this.init_all_no_calc(team, id, level, role_card, pos, equips, extra_equips)
	this.calculate_hp_attack_defense()
}

func (this *TeamMember) init_for_summon(user *TeamMember, team *BattleTeam, id int32, level int32, role_card *table_config.XmlCardItem, pos int32) {
	this.init_all_no_calc(team, id, level, role_card, pos, nil, nil)
	for i := 0; i < len(user.attrs); i++ {
		this.attrs[i] = user.attrs[i]
	}
	// 技能增加属性
	if role_card.NormalSkillID > 0 {
		this.add_skill_attr(role_card.NormalSkillID)
	}
	if role_card.SuperSkillID > 0 {
		this.add_skill_attr(role_card.SuperSkillID)
	}
	for i := 0; i < len(role_card.PassiveSkillIds); i++ {
		this.add_skill_attr(role_card.PassiveSkillIds[i])
	}
	this.calculate_hp_attack_defense()
}

func (this *TeamMember) add_attr(attr int32, value int32) {
	if attr == ATTR_HP {
		this.add_hp(value)
	} else if attr == ATTR_HP_MAX {
		this.add_max_hp(value)
		this.attrs[ATTR_HP] = this.attrs[ATTR_HP_MAX]
	} else {
		this.attrs[attr] += value
		if attr == ATTR_ATTACK {
			this.attack = this.attrs[attr]
		} else if attr == ATTR_DEFENSE {
			this.defense = this.attrs[attr]
		}
	}
}

func (this *TeamMember) add_hp(hp int32) {
	if hp > 0 {
		if this.attrs[ATTR_HP]+hp > this.attrs[ATTR_HP_MAX] {
			this.attrs[ATTR_HP] = this.attrs[ATTR_HP_MAX]
		} else {
			this.attrs[ATTR_HP] += hp
		}
	} else if hp < 0 {
		if this.attrs[ATTR_HP]+hp < 0 {
			this.attrs[ATTR_HP] = 0
		} else {
			this.attrs[ATTR_HP] += hp
		}
	}
	this.hp = this.attrs[ATTR_HP]
	if hp != 0 && this.hp == 0 {
		log.Debug("+++++++++++++++++++++++++++ team[%v] mem[%v] 将死", this.team.side, this.pos)
	}
}

func (this *TeamMember) add_max_hp(add int32) {
	if add < 0 {
		if this.attrs[ATTR_HP_MAX]+add < this.attrs[ATTR_HP] {
			this.attrs[ATTR_HP] = this.attrs[ATTR_HP_MAX] + add
		}
	}
	this.attrs[ATTR_HP_MAX] += add
}

func (this *TeamMember) round_start() {
	this.act_num += 1
	this.init_passive_round_num()
}

func (this *TeamMember) round_end() {
	for i := 0; i < len(this.bufflist_arr); i++ {
		buffs := this.bufflist_arr[i]
		buffs.on_round_end()
	}

	for _, v := range this.passive_triggers {
		if v == nil {
			continue
		}
		for i := 0; i < len(v); i++ {
			if v[i].skill.TriggerRoundMax > 0 {
				v[i].round_num = v[i].skill.TriggerRoundMax
			}
		}
	}

	if this.can_action() {
		add_energy := global_config.EnergyAdd
		if add_energy == 0 {
			add_energy = BATTLE_TEAM_MEMBER_ADD_ENERGY
		}
		this.energy += add_energy
	}
}

func (this *TeamMember) get_use_skill() (skill_id int32) {
	if this.act_num <= 0 {
		return
	}

	max_energy := global_config.MaxEnergy
	if max_energy == 0 {
		max_energy = BATTLE_TEAM_MEMBER_MAX_ENERGY
	}

	// 能量满用绝杀
	if this.energy >= max_energy && this.card.SuperSkillID > 0 {
		skill_id = this.card.SuperSkillID
	} else {
		skill_id = this.card.NormalSkillID
	}
	return
}

func (this *TeamMember) act_done() {
	if this.act_num > 0 {
		this.act_num -= 1
	}
}

func (this *TeamMember) used_skill(skill *table_config.XmlSkillItem) {
	if skill.Type != SKILL_TYPE_SUPER {
		return
	}
	max_energy := global_config.MaxEnergy
	if max_energy == 0 {
		max_energy = BATTLE_TEAM_MEMBER_MAX_ENERGY
	}
	if this.energy >= max_energy {
		this.energy -= max_energy
	}
}

func (this *TeamMember) add_buff(attacker *TeamMember, skill_effect []int32) (buff_id int32) {
	b := buff_table_mgr.Get(skill_effect[1])
	if b == nil {
		return
	}

	if this.bufflist_arr == nil {
		this.bufflist_arr = make([]*BuffList, BUFF_EFFECT_TYPE_COUNT)
		for i := 0; i < BUFF_EFFECT_TYPE_COUNT; i++ {
			this.bufflist_arr[i] = &BuffList{}
			this.bufflist_arr[i].owner = this
		}
	}

	// 互斥
	for i := 0; i < len(this.bufflist_arr); i++ {
		h := this.bufflist_arr[i]
		if h != nil && h.check_buff_mutex(b) {
			return
		}
	}

	if rand.Int31n(10000) >= skill_effect[2] {
		return
	}

	buff_id = this.bufflist_arr[b.Effect[0]].add_buff(attacker, b, skill_effect)
	return buff_id
}

func (this *TeamMember) has_buff(buff_id int32) bool {
	if this.bufflist_arr != nil {
		for i := 0; i < len(this.bufflist_arr); i++ {
			bufflist := this.bufflist_arr[i]
			buff := bufflist.head
			for buff != nil {
				if buff.buff.Id == buff_id {
					return true
				}
				buff = buff.next
			}
		}
	}
	return false
}

func (this *TeamMember) add_buff_effect(buff *Buff, skill_effects []int32) {
	if buff.buff.Effect != nil && len(buff.buff.Effect) >= 2 {
		if buff.buff.Effect[0] == BUFF_EFFECT_TYPE_MODIFY_ATTR {
			this.add_attr(buff.buff.Effect[1], skill_effects[3])
		} else if buff.buff.Effect[0] == BUFF_EFFECT_TYPE_TRIGGER_SKILL {
			this.add_passive_trigger(buff.buff.Effect[1])
			log.Debug("Team[%v] member[%v] 添加BUFF[%v] 增加了被动技[%v]", this.team.side, this.pos, buff.buff.Id, buff.buff.Effect[1])
		}
	}
}

func (this *TeamMember) remove_buff_effect(buff *Buff) {
	if buff.buff == nil || buff.buff.Effect == nil {
		return
	}

	if len(buff.buff.Effect) >= 2 {
		effect_type := buff.buff.Effect[0]
		if effect_type == BUFF_EFFECT_TYPE_MODIFY_ATTR {
			this.add_attr(buff.buff.Effect[1], -buff.param)
		} else if effect_type == BUFF_EFFECT_TYPE_TRIGGER_SKILL {
			this.delete_passive_trigger(buff.buff.Effect[1])
		}
	}
}

func (this *TeamMember) is_disable_normal_attack() bool {
	if this.bufflist_arr == nil {
		return false
	}
	disable := false
	bufflist := this.bufflist_arr[BUFF_EFFECT_TYPE_DISABLE_NORMAL_ATTACK]
	if bufflist.head != nil {
		disable = true
	}
	return disable
}

func (this *TeamMember) is_disable_super_attack() bool {
	if this.bufflist_arr == nil {
		return false
	}
	disable := false
	bufflist := this.bufflist_arr[BUFF_EFFECT_TYPE_DISABLE_SUPER_ATTACK]
	if bufflist.head != nil {
		disable = true
	}
	return disable
}

func (this *TeamMember) is_disable_attack() bool {
	if this.bufflist_arr == nil {
		return false
	}
	disable := false
	bufflist := this.bufflist_arr[BUFF_EFFECT_TYPE_DISABLE_ACTION]
	if bufflist.head != nil {
		disable = true
	}
	return disable
}

func (this *TeamMember) can_action() bool {
	if this.bufflist_arr != nil {
		if this.bufflist_arr[BUFF_EFFECT_TYPE_DISABLE_ACTION].head != nil || this.bufflist_arr[BUFF_EFFECT_TYPE_DISABLE_SUPER_ATTACK] != nil {
			return false
		}
	}
	return true
}

func (this *TeamMember) is_dead() bool {
	if this.hp < 0 {
		return true
	}
	return false
}

func (this *TeamMember) is_will_dead() bool {
	if this.hp == 0 {
		return true
	}
	return false
}

func (this *TeamMember) set_dead(attacker *TeamMember, skill_data *table_config.XmlSkillItem) {
	this.hp = -1
	this.on_dead(attacker, skill_data)
	log.Debug("+++++++++++++++++++++++++ team[%v] mem[%v] 死了", this.team.side, this.pos)
}

func (this *TeamMember) on_will_dead(attacker *TeamMember) {
	if passive_skill_effect_with_self_pos(EVENT_BEFORE_TARGET_DEAD, this.team, this.pos, nil, nil, true) {
		log.Debug("Team[%v] member[%v] 触发了死亡前被动技能", attacker.team.side, attacker.pos)
	}
}

func (this *TeamMember) on_after_will_dead(attacker *TeamMember) {
	passive_skill_effect_with_self_pos(EVENT_AFTER_TARGET_DEAD, this.team, this.pos, attacker.team, nil, true)
	log.Debug("+++++++++++++ Team[%v] member[%v] 触发死亡后触发器", this.team.side, this.pos)
}

func (this *TeamMember) on_dead(attacker *TeamMember, skill_data *table_config.XmlSkillItem) {
	// 被动技，被主动技杀死时触发
	if skill_data != nil && (skill_data.Type == SKILL_TYPE_NORMAL || skill_data.Type == SKILL_TYPE_SUPER) {
		passive_skill_effect_with_self_pos(EVENT_KILL_ENEMY, attacker.team, attacker.pos, this.team, []int32{this.pos}, true)
	}

	// 作为队友死亡触发
	for pos := int32(0); pos < BATTLE_TEAM_MEMBER_MAX_NUM; pos++ {
		team_mem := this.team.members[pos]
		if team_mem == nil || team_mem.is_dead() {
			continue
		}
		if pos != this.pos {
			passive_skill_effect_with_self_pos(EVENT_AFTER_TEAMMATE_DEAD, this.team, pos, this.team, []int32{this.pos}, true)
		}
	}
	// 相对于敌方死亡时触发
	for pos := int32(0); pos < BATTLE_TEAM_MEMBER_MAX_NUM; pos++ {
		team_mem := attacker.team.members[pos]
		if team_mem == nil || team_mem.is_dead() {
			continue
		}
		passive_skill_effect_with_self_pos(EVENT_AFTER_ENEMY_DEAD, attacker.team, pos, this.team, []int32{this.pos}, true)
	}
}

func (this *TeamMember) on_battle_finish() {
	if USE_PASSIVE_LIST {
		if this.passive_triggers != nil {
			for _, d := range this.passive_triggers {
				if d == nil {
					continue
				}
				for i := 0; i < len(d); i++ {
					if d[i] != nil {
						passive_trigger_data_pool.Put(d[i])
					}
				}
			}
			this.passive_triggers = nil
		}
	} else {
		if this.passive_trigger_lists != nil {
			for _, d := range this.passive_trigger_lists {
				d.clear()
			}
		}
	}
	if this.passive_skills != nil {
		this.passive_skills = nil
	}
}
