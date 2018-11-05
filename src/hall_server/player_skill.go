package main

import (
	"errors"
	"fmt"
	"ih_server/libs/log"
	"ih_server/proto/gen_go/client_message"
	"ih_server/src/table_config"
	"math"
	"math/rand"
)

// 技能类型
const (
	SKILL_TYPE_NORMAL  = 1 // 普攻
	SKILL_TYPE_SUPER   = 2 // 绝杀
	SKILL_TYPE_PASSIVE = 3 // 被动
	SKILL_TYPE_NEXT    = 4 // 连携
)

// 技能战斗类型
const (
	SKILL_FIGHT_TYPE_NONE   = iota
	SKILL_FIGHT_TYPE_MELEE  = 1 // 近战
	SKILL_FIGHT_TYPE_REMOTE = 2 // 远程
)

// 技能敌我类型
const (
	SKILL_ENEMY_TYPE_OUR   = 1 // 我方
	SKILL_ENEMY_TYPE_ENEMY = 2 // 敌方
)

// 技能攻击范围
const (
	SKILL_RANGE_TYPE_SINGLE    = 1 // 单个
	SKILL_RANGE_TYPE_ROW       = 2 // 横排
	SKILL_RANGE_TYPE_COLUMN    = 3 // 竖排
	SKILL_RANGE_TYPE_MULTI     = 4 // 多个
	SKILL_RANGE_TYPE_CROSS     = 5 // 十字
	SKILL_RANGE_TYPE_BIG_CROSS = 6 // 大十字
	SKILL_RANGE_TYPE_ALL       = 7 // 全体
)

// 技能目标类型
const (
	SKILL_TARGET_TYPE_DEFAULT        = 1 // 默认
	SKILL_TARGET_TYPE_BACK           = 2 // 后排
	SKILL_TARGET_TYPE_HP_MIN         = 3 // 血最少
	SKILL_TARGET_TYPE_RANDOM         = 4 // 随机
	SKILL_TARGET_TYPE_SELF           = 5 // 自身
	SKILL_TARGET_TYPE_TRIGGER_OBJECT = 6 // 触发器的另一个对象
	SKILL_TARGET_TYPE_CROPSE         = 7 // 尸体
	SKILL_TARGET_TYPE_EMPTY_POS      = 8 // 空位
)

// BUFF效果类型
const (
	BUFF_EFFECT_TYPE_DAMAGE                = 1
	BUFF_EFFECT_TYPE_DISABLE_NORMAL_ATTACK = 2
	BUFF_EFFECT_TYPE_DISABLE_SUPER_ATTACK  = 3
	BUFF_EFFECT_TYPE_DISABLE_ACTION        = 4
	BUFF_EFFECT_TYPE_MODIFY_ATTR           = 5
	BUFF_EFFECT_TYPE_DODGE                 = 6
	BUFF_EFFECT_TYPE_TRIGGER_SKILL         = 7
	BUFF_EFFECT_TYPE_COUNT                 = 8
)

// 获取行数顺序
func _get_rows_order(self_pos int32) (rows_order []int32) {
	if self_pos%BATTLE_FORMATION_ONE_LINE_MEMBER_NUM == 0 {
		rows_order = []int32{0, 1, 2}
	} else if self_pos%BATTLE_FORMATION_ONE_LINE_MEMBER_NUM == 1 {
		rows_order = []int32{1, 0, 2}
	} else if self_pos%BATTLE_FORMATION_ONE_LINE_MEMBER_NUM == 2 {
		rows_order = []int32{2, 1, 0}
	} else {
		log.Warn("not impossible self_pos[%v]", self_pos)
	}
	return
}

// 获取行
func _get_row_indexes() [][]int32 {
	return [][]int32{
		[]int32{0, 3, 6},
		[]int32{1, 4, 7},
		[]int32{2, 5, 8},
	}
}

// 获取列
func _get_column_indexes() [][]int32 {
	return [][]int32{
		[]int32{0, 1, 2},
		[]int32{3, 4, 5},
		[]int32{6, 7, 8},
	}
}

// 行是否为空
func _check_team_row(row_index int32, target_team *BattleTeam) (is_empty bool, pos []int32) {
	is_empty = true
	for i := 0; i < BATTLE_FORMATION_ONE_LINE_MEMBER_NUM; i++ {
		p := row_index + int32(BATTLE_FORMATION_LINE_NUM*i)
		m := target_team.members[p]
		if m != nil && !m.is_dead() {
			pos = append(pos, p)
			if is_empty {
				is_empty = false
			}
		}
	}
	return
}

// 列是否为空
func _check_team_column(self_pos, column_index int32, target_team *BattleTeam) (is_empty bool, pos []int32) {
	is_empty = true
	first_pos := self_pos%BATTLE_FORMATION_ONE_LINE_MEMBER_NUM + column_index*BATTLE_FORMATION_ONE_LINE_MEMBER_NUM
	m := target_team.members[first_pos]
	if m != nil && !m.is_dead() {
		pos = []int32{first_pos}
		is_empty = false
	}
	for i := 0; i < BATTLE_FORMATION_LINE_NUM; i++ {
		p := int(column_index)*BATTLE_FORMATION_ONE_LINE_MEMBER_NUM + i
		if p == int(first_pos) {
			continue
		}
		m = target_team.members[p]
		if m != nil && !m.is_dead() {
			pos = append(pos, int32(p))
			if is_empty {
				is_empty = false
			}
		}
	}
	return
}

// 十字攻击范围
func _get_team_cross_targets() [][]int32 {
	return [][]int32{
		[]int32{0, 1, 3},
		[]int32{1, 0, 2, 4},
		[]int32{2, 1, 5},
		[]int32{3, 0, 4, 6},
		[]int32{4, 1, 3, 5, 7},
		[]int32{5, 2, 4, 8},
		[]int32{6, 3, 7},
		[]int32{7, 4, 6, 8},
		[]int32{8, 5, 7},
	}
}

// 大十字攻击范围
func _get_team_big_cross_targets() [][]int32 {
	return [][]int32{
		[]int32{0, 1, 2, 3, 6},
		[]int32{1, 0, 2, 4, 7},
		[]int32{2, 1, 0, 5, 8},
		[]int32{3, 0, 6, 4, 5},
		[]int32{4, 1, 3, 5, 7},
		[]int32{5, 2, 4, 3, 8},
		[]int32{6, 3, 0, 7, 8},
		[]int32{7, 4, 1, 6, 8},
		[]int32{8, 5, 3, 7, 6},
	}
}

// 单个默认目标
func _get_single_default_target(self_pos int32, target_team *BattleTeam) (pos int32) {
	pos = int32(-1)
	rows := _get_rows_order(self_pos)
	if rows == nil {
		return
	}
	found := false
	for l := 0; l < len(rows); l++ {
		for i := int32(0); i < BATTLE_FORMATION_ONE_LINE_MEMBER_NUM; i++ {
			p := rows[l] + i*BATTLE_FORMATION_LINE_NUM
			m := target_team.members[p]
			if m != nil && !m.is_dead() {
				pos = int32(p)
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	return
}

// 默认目标选择
func skill_get_default_targets(self_pos int32, target_team *BattleTeam, skill_data *table_config.XmlSkillItem) (pos []int32) {
	if skill_data.RangeType == SKILL_RANGE_TYPE_SINGLE { // 单体
		pos = []int32{_get_single_default_target(self_pos, target_team)}
	} else if skill_data.RangeType == SKILL_RANGE_TYPE_ROW { //横排
		rows := _get_rows_order(self_pos)
		if rows == nil {
			log.Warn("get rows failed")
			return
		}
		is_empty := false
		for i := 0; i < len(rows); i++ {
			is_empty, pos = _check_team_row(rows[i], target_team)
			if !is_empty {
				break
			}
		}
	} else if skill_data.RangeType == SKILL_RANGE_TYPE_COLUMN { // 竖排
		for c := 0; c < BATTLE_FORMATION_LINE_NUM; c++ {
			is_empty := false
			is_empty, pos = _check_team_column(self_pos, int32(c), target_team)
			if !is_empty {
				break
			}
		}
	} else if skill_data.RangeType == SKILL_RANGE_TYPE_MULTI { // 多体
		// 默认多体不存在
		log.Warn("Cant get target pos on default multi members")
	} else if skill_data.RangeType == SKILL_RANGE_TYPE_CROSS { // 十字
		p := _get_single_default_target(self_pos, target_team)
		if p < 0 {
			log.Error("Get single target pos by self_pos[%v] failed", self_pos)
			return
		}
		ps := _get_team_cross_targets()[p]
		for i := 0; i < len(ps); i++ {
			m := target_team.members[ps[i]]
			if m != nil && !m.is_dead() {
				pos = append(pos, ps[i])
			}
		}
	} else if skill_data.RangeType == SKILL_RANGE_TYPE_BIG_CROSS { // 大十字
		p := _get_single_default_target(self_pos, target_team)
		if p < 0 {
			log.Error("Get single target pos by self_pos[%v] failed", self_pos)
			return
		}
		ps := _get_team_big_cross_targets()[p]
		for i := 0; i < len(ps); i++ {
			m := target_team.members[ps[i]]
			if m != nil && !m.is_dead() {
				pos = append(pos, ps[i])
			}
		}
	} else if skill_data.RangeType == SKILL_RANGE_TYPE_ALL { // 全体
		m := target_team.members[self_pos]
		if m != nil && !m.is_dead() {
			pos = []int32{self_pos}
		}
		for i := 0; i < BATTLE_TEAM_MEMBER_MAX_NUM; i++ {
			if self_pos == int32(i) {
				continue
			}
			m = target_team.members[i]
			if m != nil && !m.is_dead() {
				pos = append(pos, int32(i))
			}
		}
	} else {
		log.Error("Unknown skill range type: %v", skill_data.RangeType)
	}
	return
}

// 后排目标选择
func skill_get_back_targets(self_pos int32, target_team *BattleTeam, skill_data *table_config.XmlSkillItem) (pos []int32) {
	if skill_data.RangeType == SKILL_RANGE_TYPE_SINGLE { // 单体
		var found bool
		rows := _get_rows_order(self_pos)
		for i := BATTLE_FORMATION_LINE_NUM - 1; i >= 0; i-- {
			for j := 0; j < len(rows); j++ {
				p := int32(i)*BATTLE_FORMATION_ONE_LINE_MEMBER_NUM + rows[j]
				m := target_team.members[p]
				if m != nil && !m.is_dead() {
					pos = append(pos, p)
					found = true
					break
				}
			}
			if found {
				break
			}
		}
	} else if skill_data.RangeType == SKILL_RANGE_TYPE_COLUMN { // 竖排
		is_empty := false
		for i := BATTLE_FORMATION_LINE_NUM - 1; i >= 0; i-- {
			is_empty, pos = _check_team_column(self_pos, int32(i), target_team)
			if !is_empty {
				break
			}
		}
	} else {
		log.Warn("Range type %v cant get back targets", skill_data.RangeType)
	}
	return
}

// 血最少选择
func skill_get_hp_min_targets(self_pos int32, target_team *BattleTeam, skill_data *table_config.XmlSkillItem) (pos []int32) {
	if skill_data.RangeType == SKILL_RANGE_TYPE_SINGLE {
		hp := int32(0)
		p := int32(-1)
		for i := 0; i < BATTLE_TEAM_MEMBER_MAX_NUM; i++ {
			m := target_team.members[i]
			if m != nil && !m.is_dead() {
				if hp == 0 || hp > m.hp {
					hp = m.hp
					p = int32(i)
				}
			}
		}
		pos = append(pos, p)
	} else {
		log.Warn("Range type %v cant get hp min targets", skill_data.RangeType)
	}
	return
}

// 随机一个目标
func _random_one_target(self_pos int32, target_team *BattleTeam, except_pos []int32) (pos int32) {
	pos = int32(-1)
	c := int32(0)
	r := rand.Int31n(BATTLE_TEAM_MEMBER_MAX_NUM)
	for {
		used := false
		if except_pos != nil {
			for i := 0; i < len(except_pos); i++ {
				if r == except_pos[i] {
					used = true
					break
				}
			}
		}

		if !used {
			m := target_team.members[r]
			if m != nil && !m.is_dead() {
				pos = r
				break
			}
		}
		r = (r + 1) % BATTLE_TEAM_MEMBER_MAX_NUM
		c += 1
		if c >= BATTLE_TEAM_MEMBER_MAX_NUM {
			break
		}
	}
	return
}

// 随机选择
func skill_get_random_targets(self_pos int32, target_team *BattleTeam, skill_data *table_config.XmlSkillItem) (pos []int32) {
	if skill_data.RangeType == SKILL_RANGE_TYPE_SINGLE {
		//rand.Seed(time.Now().Unix())
		p := _random_one_target(self_pos, target_team, pos)
		if p < 0 {
			log.Error("Cant get random one target with self_pos %v", self_pos)
			return
		}
		pos = append(pos, p)
	} else if skill_data.RangeType == SKILL_RANGE_TYPE_MULTI {
		for i := int32(0); i < skill_data.MaxTarget; i++ {
			p := _random_one_target(self_pos, target_team, pos)
			if p >= 0 {
				pos = append(pos, p)
			}
		}
	} else {
		log.Warn("Range type %v cant get random targets", skill_data.RangeType)
	}
	return
}

// 强制自身目标选择
func skill_get_force_self_targets(self_pos int32, target_team *BattleTeam, skill_data *table_config.XmlSkillItem) (pos []int32) {
	var indexes []int32
	if skill_data.RangeType == SKILL_RANGE_TYPE_SINGLE {
		pos = []int32{self_pos}
		return
	} else if skill_data.RangeType == SKILL_RANGE_TYPE_ROW {
		y := self_pos % BATTLE_FORMATION_LINE_NUM
		indexes = _get_row_indexes()[y]
	} else if skill_data.RangeType == SKILL_RANGE_TYPE_COLUMN {
		l := self_pos / BATTLE_FORMATION_LINE_NUM
		indexes = _get_column_indexes()[l]
	} else if skill_data.RangeType == SKILL_RANGE_TYPE_CROSS {
		indexes = _get_team_cross_targets()[self_pos]
	} else if skill_data.RangeType == SKILL_RANGE_TYPE_BIG_CROSS {
		indexes = _get_team_big_cross_targets()[self_pos]
	} else if skill_data.RangeType == SKILL_RANGE_TYPE_ALL {
		indexes = []int32{0, 1, 2, 3, 4, 5, 6, 7, 8}
	}

	if indexes != nil {
		for i := 0; i < len(indexes); i++ {
			m := target_team.members[indexes[i]]
			if m != nil && !m.is_dead() {
				pos = append(pos, indexes[i])
			}
		}
	}
	return
}

// 寻找空位
func skill_get_empty_pos(target_team *BattleTeam, skill_data *table_config.XmlSkillItem) (pos []int32) {
	c := int32(0)
	for j := 0; j < len(target_team.members); j++ {
		if target_team.members[j] == nil {
			pos = append(pos, int32(j))
			c += 1
			if c >= skill_data.MaxTarget {
				break
			}
		}
	}
	return
}

// 技能条件
const (
	SKILL_COND_TYPE_NONE                        = iota
	SKILL_COND_TYPE_HAS_LABEL                   = 1
	SKILL_COND_TYPE_HAS_BUFF                    = 2
	SKILL_COND_TYPE_HP_NOT_LESS                 = 3
	SKILL_COND_TYPE_HP_GREATER                  = 4
	SKILL_COND_TYPE_HP_NOT_GREATER              = 5
	SKILL_COND_TYPE_HP_LESS                     = 6
	SKILL_COND_TYPE_MP_NOT_LESS                 = 7
	SKILL_COND_TYPE_MP_NOT_GREATER              = 8
	SKILL_COND_TYPE_TEAM_HAS_ROLE               = 9
	SKILL_COND_TYPE_IS_TYPE                     = 10
	SKILL_COND_TYPE_IS_CAMP                     = 11
	SKILL_COND_TYPE_NO_LABEL                    = 12
	SKILL_COND_TYPE_NO_BUFF                     = 13
	SKILL_COND_TYPE_NO_NPC_ID                   = 14
	SKILL_COND_TYPE_IS_NO_TYPE                  = 15
	SKILL_COND_TYPE_IS_NO_CAMP                  = 16
	SKILL_COND_TYPE_IN_COLUMN                   = 17
	SKILL_COND_TYPE_HAS_SHIELD                  = 18
	SKILL_COND_TYPE_NO_SHIELD                   = 19
	SKILL_COND_TYPE_SELF_TEAM_MEMBERS_GREATER   = 20
	SKILL_COND_TYPE_SELF_TEAM_MEMBERS_LESS      = 21
	SKILL_COND_TYPE_TARGET_TEAM_MEMBERS_GREATER = 22
	SKILL_COND_TYPE_TARGET_TEAM_MEMBERS_LESS    = 23
	SKILL_COND_TYPE_ROUND_GREATER               = 24
	SKILL_COND_TYPE_ROUND_LESS                  = 25
)

func _skill_check_cond(mem *TeamMember, effect_cond []int32) bool {
	if len(effect_cond) > 0 {
		if effect_cond[0] == SKILL_COND_TYPE_NONE {
			return true
		}
		if len(effect_cond) >= 1 {
			if len(effect_cond) >= 2 {
				if effect_cond[0] == SKILL_COND_TYPE_HAS_LABEL {
					if mem.card.Label != nil {
						for i := 0; i < len(mem.card.Label); i++ {
							if mem.card.Label[i] == effect_cond[1] {
								return true
							}
						}
					}
				} else if effect_cond[0] == SKILL_COND_TYPE_HAS_BUFF {
					if mem.has_buff(effect_cond[1]) {
						return true
					}
				} else if effect_cond[0] == SKILL_COND_TYPE_HP_NOT_LESS {
					if int64(mem.attrs[ATTR_HP])*10000 >= int64(mem.attrs[ATTR_HP_MAX])*int64(effect_cond[1]) {
						return true
					}
				} else if effect_cond[0] == SKILL_COND_TYPE_HP_GREATER {
					if int64(mem.attrs[ATTR_HP])*10000 > int64(mem.attrs[ATTR_HP_MAX])*int64(effect_cond[1]) {
						return true
					}
				} else if effect_cond[0] == SKILL_COND_TYPE_HP_NOT_GREATER {
					if int64(mem.attrs[ATTR_HP])*10000 <= int64(mem.attrs[ATTR_HP_MAX])*int64(effect_cond[1]) {
						return true
					}
				} else if effect_cond[0] == SKILL_COND_TYPE_HP_LESS {
					if int64(mem.attrs[ATTR_HP])*10000 < int64(mem.attrs[ATTR_HP_MAX])*int64(effect_cond[1]) {
						return true
					}
				} else if effect_cond[0] == SKILL_COND_TYPE_MP_NOT_LESS {
					if mem.attrs[ATTR_MP] >= effect_cond[1] {
						return true
					}
				} else if effect_cond[0] == SKILL_COND_TYPE_MP_NOT_GREATER {
					if mem.attrs[ATTR_MP] <= effect_cond[1] {
						return true
					}
				} else if effect_cond[0] == SKILL_COND_TYPE_TEAM_HAS_ROLE {
					if mem.team.HasRole(effect_cond[1]) {
						return true
					}
				} else if effect_cond[0] == SKILL_COND_TYPE_IS_TYPE {
					if mem.card.Type == effect_cond[1] {
						return true
					}
				} else if effect_cond[0] == SKILL_COND_TYPE_IS_CAMP {
					if mem.card.Camp == effect_cond[1] {
						return true
					}
				} else if effect_cond[0] == SKILL_COND_TYPE_NO_LABEL {
					if mem.card.Label != nil {
						for i := 0; i < len(mem.card.Label); i++ {
							if mem.card.Label[i] == effect_cond[1] {
								return false
							}
						}
					}
					return true
				} else if effect_cond[0] == SKILL_COND_TYPE_NO_BUFF {
					if !mem.has_buff(effect_cond[1]) {
						return true
					}
				} else if effect_cond[0] == SKILL_COND_TYPE_NO_NPC_ID {
					b := true
					if mem.team != nil {
						for i := 0; i < BATTLE_TEAM_MEMBER_MAX_NUM; i++ {
							m := mem.team.members[i]
							if m != nil && !m.is_dead() {
								if m.card.Id == effect_cond[1] {
									b = false
									break
								}
							}
						}
					}
					return b
				} else if effect_cond[0] == SKILL_COND_TYPE_IS_NO_TYPE {
					if mem.card.Type != effect_cond[1] {
						return true
					}
				} else if effect_cond[0] == SKILL_COND_TYPE_IS_NO_CAMP {
					if mem.card.Camp != effect_cond[1] {
						return true
					}
				} else if effect_cond[0] == SKILL_COND_TYPE_SELF_TEAM_MEMBERS_LESS {
					if mem.team != nil {
						if mem.team.MembersNum() < effect_cond[1] {
							return true
						}
					}
				} else if effect_cond[0] == SKILL_COND_TYPE_SELF_TEAM_MEMBERS_GREATER {
					if mem.team != nil {
						if mem.team.MembersNum() > effect_cond[1] {
							return true
						}
					}
				} else if effect_cond[0] == SKILL_COND_TYPE_ROUND_GREATER {
					if mem.team != nil {
						if mem.team.common_data.round_num > effect_cond[1] {
							return true
						}
					}
				} else if effect_cond[0] == SKILL_COND_TYPE_ROUND_LESS {
					if mem.team != nil {
						if mem.team.common_data.round_num < effect_cond[1] {
							return true
						}
					}
				} else if effect_cond[0] == SKILL_COND_TYPE_IN_COLUMN {
					if mem.pos/BATTLE_FORMATION_ONE_LINE_MEMBER_NUM == effect_cond[1]-1 {
						return true
					}
				} else {
					log.Warn("skill effect cond %v value %v unknown", effect_cond[0], effect_cond[1])
				}
			} else {

				if effect_cond[0] == SKILL_COND_TYPE_HAS_SHIELD {
					if mem.attrs[ATTR_SHIELD] > 0 {
						return true
					}
				} else if effect_cond[0] == SKILL_COND_TYPE_NO_SHIELD {
					if mem.attrs[ATTR_SHIELD] <= 0 {
						return true
					}
				} else {
					log.Warn("skill effect cond %v unknown", effect_cond[0])
				}
			}
		}
		return false
	}
	return true
}

func skill_check_cond(self *TeamMember, target_team *BattleTeam, target_pos []int32, effect_cond1 []int32, effect_cond2 []int32) bool {
	if (effect_cond1 == nil || len(effect_cond1) == 0) && (effect_cond2 == nil || len(effect_cond2) == 0) {
		return true
	}

	if self != nil && !_skill_check_cond(self, effect_cond1) {
		return false
	}

	if effect_cond2 == nil || len(effect_cond2) == 0 {
		return true
	}

	if target_team != nil {
		n := 0
		if target_pos != nil {
			n = len(target_pos)
		} else {
			n = BATTLE_TEAM_MEMBER_MAX_NUM
		}
		for i := 0; i < n; i++ {
			pos := int32(0)
			if target_pos != nil {
				pos = target_pos[i]
			} else {
				pos = int32(i)
			}
			target := target_team.members[pos]
			if target == nil {
				continue
			}
			if _skill_check_cond(target, effect_cond2) {
				break
			}
			return false
		}
	}

	return true
}

func skill_effect_cond_check(self *TeamMember, target *TeamMember, effect_cond1 []int32, effect_cond2 []int32) bool {
	if len(effect_cond1) == 0 && len(effect_cond2) == 0 {
		return true
	}

	if self != nil && !_skill_check_cond(self, effect_cond1) {
		return false
	}

	if target != nil && !_skill_check_cond(target, effect_cond2) {
		return false
	}

	return true
}

// 技能效果类型
const (
	SKILL_EFFECT_TYPE_DIRECT_INJURY         = 1  // 直接伤害
	SKILL_EFFECT_TYPE_CURE                  = 2  // 治疗
	SKILL_EFFECT_TYPE_ADD_BUFF              = 3  // 施加BUFF
	SKILL_EFFECT_TYPE_SUMMON                = 4  // 召唤技能
	SKILL_EFFECT_TYPE_MODIFY_ATTR           = 5  // 改变下次计算时的角色参数
	SKILL_EFFECT_TYPE_MODIFY_NORMAL_SKILL   = 6  // 改变普通攻击技能ID
	SKILL_EFFECT_TYPE_MODIFY_RAGE_SKILL     = 7  // 改变怒气攻击技能ID
	SKILL_EFFECT_TYPE_ADD_NORMAL_ATTACK_NUM = 8  // 增加普攻次数
	SKILL_EFFECT_TYPE_MODIFY_RAGE           = 9  // 改变怒气
	SKILL_EFFECT_TYPE_ADD_SHIELD            = 10 // 增加护盾
)

// 技能直接伤害
func skill_effect_direct_injury(self *TeamMember, target *TeamMember, skill_type, skill_fight_type int32, effect []int32) (target_damage, self_damage int32, is_block, is_critical, is_absorb bool, anti_type int32) {
	if len(effect) < 4 {
		log.Error("skill effect length %v not enough", len(effect))
		return
	}

	// 增伤减伤总和
	damage_add := self.attrs[ATTR_TOTAL_DAMAGE_ADD]
	damage_sub := target.attrs[ATTR_TOTAL_DAMAGE_SUB]

	// 类型
	if skill_type == SKILL_TYPE_NORMAL {
		damage_add += self.attrs[ATTR_NORMAL_DAMAGE_ADD]
		damage_sub += target.attrs[ATTR_NORMAL_DAMAGE_SUB]
	} else if skill_type == SKILL_TYPE_SUPER {
		damage_add += self.attrs[ATTR_RAGE_DAMAGE_ADD]
		damage_sub += target.attrs[ATTR_RAGE_DAMAGE_SUB]
	} else if skill_type == SKILL_TYPE_PASSIVE {

	} else {
		log.Error("Invalid skill type: %v", skill_type)
		return
	}

	// 战斗类型
	if skill_fight_type == SKILL_FIGHT_TYPE_MELEE {
		damage_add += self.attrs[ATTR_CLOSE_DAMAGE_ADD]
		damage_sub += target.attrs[ATTR_CLOSE_DAMAGE_SUB]
	} else if skill_fight_type == SKILL_FIGHT_TYPE_REMOTE {
		damage_add += self.attrs[ATTR_REMOTE_DAMAGE_ADD]
		damage_sub += target.attrs[ATTR_REMOTE_DAMAGE_SUB]
	} else if skill_fight_type == SKILL_FIGHT_TYPE_NONE {

	} else {
		log.Error("Invalid skill melee type: %v", skill_fight_type)
		return
	}

	// 角色类型克制
	if self.card.Type == table_config.CARD_ROLE_TYPE_ATTACK && target.card.Type == table_config.CARD_ROLE_TYPE_SKILL {
		damage_add += 1500
		anti_type = 1
	} else if self.card.Type == table_config.CARD_ROLE_TYPE_SKILL && target.card.Type == table_config.CARD_ROLE_TYPE_DEFENSE {
		damage_add += 1500
		anti_type = 1
	} else if self.card.Type == table_config.CARD_ROLE_TYPE_DEFENSE && target.card.Type == table_config.CARD_ROLE_TYPE_ATTACK {
		damage_add += 1500
		anti_type = 1
	} else if self.card.Type == table_config.CARD_ROLE_TYPE_ATTACK && target.card.Type == table_config.CARD_ROLE_TYPE_DEFENSE {
		damage_sub += 1500
		anti_type = -1
	} else if self.card.Type == table_config.CARD_ROLE_TYPE_SKILL && target.card.Type == table_config.CARD_ROLE_TYPE_ATTACK {
		damage_sub += 1500
		anti_type = -1
	} else if self.card.Type == table_config.CARD_ROLE_TYPE_DEFENSE && target.card.Type == table_config.CARD_ROLE_TYPE_SKILL {
		damage_sub += 1500
		anti_type = -1
	}

	// 反伤
	var reflect_damage int32
	if skill_fight_type == SKILL_FIGHT_TYPE_MELEE {
		reflect_damage = int32(int64(target.attrs[ATTR_ATTACK]) * int64(target.attrs[ATTR_CLOSE_REFLECT]) / 10000)
	} else if skill_fight_type == SKILL_FIGHT_TYPE_REMOTE {
		reflect_damage = int32(int64(target.attrs[ATTR_ATTACK]) * int64(target.attrs[ATTR_REMOTE_REFLECT]) / 10000)
	}
	if reflect_damage >= self.hp {
		reflect_damage = self.hp - 1
	}
	if reflect_damage > 0 {
		self_damage = reflect_damage
	}

	// 防御力
	defense := int32(int64(target.attrs[ATTR_DEFENSE]) * int64(10000-self.attrs[ATTR_BREAK_ARMOR]+target.attrs[ATTR_ARMOR_ADD]) / 10000)
	if defense < 0 {
		defense = 0
	}
	attack := self.attrs[ATTR_ATTACK] - defense
	attack1 := int32(int64(self.attrs[ATTR_ATTACK]) * int64(self.attrs[ATTR_ATTACK]) / int64(self.attrs[ATTR_ATTACK]+defense) / 2)
	if attack < attack1 {
		attack = attack1
	}
	if attack < 1 {
		attack = 1
	}

	// 基础技能伤害
	base_skill_damage := int32(int64(attack) * int64(effect[1]) / 10000)
	var delta_damage float64
	if damage_add-damage_sub < 0 {
		delta_damage = 10000 / float64(10000+(damage_sub-damage_add))
	} else {
		delta_damage = float64(10000+damage_add-damage_sub) / 10000
	}
	target_damage = int32(float64(base_skill_damage) * math.Max(0.1, delta_damage) * float64(10000+self.attrs[ATTR_DAMAGE_PERCENT_BONUS]) / 10000)
	if target_damage < 1 {
		target_damage = 1
	}

	// 实际暴击率
	critical := self.attrs[ATTR_CRITICAL] - target.attrs[ATTR_ANTI_CRITICAL] + 1000
	block := target.attrs[ATTR_BLOCK_RATE] - self.attrs[ATTR_BREAK_BLOCK_RATE] + 600
	if critical < 0 {
		critical = 0
	} else {
		// 触发暴击
		if float64(critical)*10000/(10000.0+math.Max(0.0, float64(block))) > float64(rand.Int31n(10000)) {
			target_damage = int32(float64(target_damage) * math.Max(1.5, float64(20000+self.attrs[ATTR_CRITICAL_MULTI])/10000))
			is_critical = true
		}
	}
	if !is_critical {
		// 实际格挡率
		if block > rand.Int31n(10000) {
			target_damage = int32(math.Max(1, float64(target_damage)*math.Max(0.1, math.Min(0.9, float64(5000)/float64(10000+target.attrs[ATTR_BLOCK_DEFENSE_RATE])))))
			is_block = true
		}
	}

	// 吸血
	var add_hp int32
	if skill_fight_type == SKILL_FIGHT_TYPE_MELEE {
		add_hp = int32(int64(target_damage) * int64(self.attrs[ATTR_CLOSE_VAMPIRE]) / 10000)
	} else if skill_fight_type == SKILL_FIGHT_TYPE_REMOTE {
		add_hp = int32(int64(target_damage) * int64(self.attrs[ATTR_REMOTE_VAMPIRE]) / 10000)
	}
	if add_hp > 0 {
		self_damage -= add_hp
	}

	// 贯通
	if effect[3] > 0 {
		if target.attrs[ATTR_SHIELD] < target_damage {
			target.attrs[ATTR_SHIELD] = 0
		} else {
			target.attrs[ATTR_SHIELD] -= target_damage
		}
	} else {
		if target.attrs[ATTR_SHIELD] < target_damage {
			target_damage -= target.attrs[ATTR_SHIELD]
			target.attrs[ATTR_SHIELD] = 0
		} else {
			target.attrs[ATTR_SHIELD] -= target_damage
			target_damage = 0
		}

		if target_damage == 0 {
			is_absorb = true
		}
	}

	return
}

// 技能治疗效果
func skill_effect_cure(self_mem *TeamMember, target_mem *TeamMember, effect []int32) (cure int32) {
	if len(effect) < 2 {
		log.Error("cure skill effect length %v not enough", len(effect))
		return
	}
	cure = int32(int64(self_mem.attrs[ATTR_ATTACK])*int64(effect[1])/10000 + int64(target_mem.attrs[ATTR_HP_MAX])*int64(effect[2])/10000)
	cure = int32(math.Max(0, float64(int64(cure)*int64(10000+self_mem.attrs[ATTR_CURE_RATE_CORRECT]+target_mem.attrs[ATTR_CURED_RATE_CORRECT])/10000)))
	return
}

// 技能增加护盾效果
func skill_effect_add_shield(self_mem *TeamMember, target_mem *TeamMember, effect []int32) (shield int32) {
	if len(effect) < 2 {
		log.Error("add shield skill effect length %v not enough", len(effect))
		return
	}

	shield = int32(int64(self_mem.attrs[ATTR_ATTACK])*int64(effect[1])/10000 + int64(target_mem.attrs[ATTR_HP_MAX])*int64(effect[2])/10000)
	if shield < 0 {
		shield = 0
	}
	return
}

// 施加BUFF
func skill_effect_add_buff(self_mem *TeamMember, target_mem *TeamMember, effect []int32) (buff_id int32) {
	if len(effect) < 5 {
		log.Error("add buff skill effect length %v not enough", len(effect))
		return
	}
	buff_id = target_mem.add_buff(self_mem, effect)
	return
}

// 召唤
func skill_effect_summon(self_mem *TeamMember, target_team *BattleTeam, empty_pos int32, effect []int32) (mem *TeamMember) {
	new_card := card_table_mgr.GetRankCard(effect[1], 1)
	if new_card == nil {
		log.Error("summon skill role[%v] not found", effect[1])
		return
	}

	mem = team_member_pool.Get()
	mem.init_for_summon(self_mem, target_team, self_mem.team.temp_curr_id, self_mem.level, new_card, empty_pos)
	self_mem.team.temp_curr_id += 1
	mem.hp = int32(int64(self_mem.hp) * int64(effect[2]) / 10000)
	mem.attrs[ATTR_HP] = mem.hp
	mem.attrs[ATTR_HP_MAX] = mem.hp
	mem.attack = self_mem.attack
	mem.attrs[ATTR_ATTACK] = mem.attack
	mem.attrs[ATTR_DAMAGE_PERCENT_BONUS] += effect[3]
	target_team.members[empty_pos] = mem
	return
}

// 临时改变角色属性效果
func skill_effect_temp_attrs(self_mem *TeamMember, effect []int32) {
	if self_mem == nil {
		return
	}
	if self_mem.temp_changed_attrs == nil {
		self_mem.temp_changed_attrs = make(map[int32]int32)
	}
	for i := 0; i < (len(effect)-1)/2; i++ {
		aid := effect[1+2*i]
		avalue := effect[1+2*i+1]
		self_mem.add_attr(aid, avalue)
		self_mem.temp_changed_attrs[aid] += avalue
	}
	self_mem.temp_changed_attrs_used = 1

	log.Debug("team[%v] member[%v] 增加了技能临时属性 %v", self_mem.team.side, self_mem.pos, self_mem.temp_changed_attrs)
}

// 设置临时属性已计算
func skill_effect_temp_attrs_used(self_mem *TeamMember) {
	if self_mem == nil {
		return
	}
	if self_mem.temp_changed_attrs_used == 1 {
		self_mem.temp_changed_attrs_used = 2
		log.Debug("team[%v] member[%v] 使用了技能临时属性", self_mem.team.side, self_mem.pos)
	}
}

// 清空临时属性
func skill_effect_clear_temp_attrs(self_mem *TeamMember) {
	if self_mem == nil {
		return
	}
	if self_mem.temp_changed_attrs_used == 2 && self_mem.temp_changed_attrs != nil {
		for k, v := range self_mem.temp_changed_attrs {
			self_mem.add_attr(k, -v)
		}
		self_mem.temp_changed_attrs_used = 0
		log.Debug("team[%v] member[%v] 清空了技能临时属性 %v", self_mem.team.side, self_mem.pos, self_mem.temp_changed_attrs)
		self_mem.temp_changed_attrs = nil
	}
}

func _get_battle_report(report *msg_client_message.BattleReportItem, skill_id int32, self_team *BattleTeam, self_pos, self_dmg int32, target_team *BattleTeam, target_pos, target_dmg int32, is_critical, is_block, is_absorb bool, anti_type int32) (*msg_client_message.BattleReportItem, *msg_client_message.BattleFighter) {
	if report == nil {
		report = build_battle_report_item(self_team, self_pos, 0, skill_id)
		if report == nil {
			return nil, nil
		}
		self_team.common_data.reports = append(self_team.common_data.reports, report)
	}
	report.User.Damage += self_dmg
	target := build_battle_report_item_add_target_item(report, target_team, target_pos, target_dmg, is_critical, is_block, is_absorb, anti_type)

	members_damage := self_team.common_data.members_damage
	members_cure := self_team.common_data.members_cure
	if target_dmg > 0 {
		members_damage[self_team.side][self_pos] += target_dmg
	} else if target_dmg < 0 {
		members_cure[self_team.side][self_pos] += target_dmg
	}
	if self_dmg > 0 {
		members_damage[target_team.side][target_pos] += self_dmg
	} else if self_dmg < 0 {
		members_cure[target_team.side][target_pos] += self_dmg
	}

	return report, target
}

// 技能效果
func skill_effect(self_team *BattleTeam, self_pos int32, target_team *BattleTeam, target_pos []int32, skill_data *table_config.XmlSkillItem) (used bool) {
	effects := skill_data.Effects
	self := self_team.members[self_pos]
	if self == nil || target_team == nil {
		return
	}

	if self.is_dead() {
		return
	}

	if skill_data.Type != SKILL_TYPE_PASSIVE {
		log.Debug("++++++++++++++++++++++ begin Team[%v] mem[%v] use skill[%v] to target_team[%v] target_pos[%v] ++++++++++++++++++++++++", self_team.side, self_pos, skill_data.Id, target_team.side, target_pos)
	} else {
		log.Debug("====================== begin Team[%v] mem[%v] use passive skill[%v] to target_team[%v] target_pos[%v] =======================", self_team.side, self_pos, skill_data.Id, target_team.side, target_pos)
	}

	var report, last_report *msg_client_message.BattleReportItem
	if !self_team.IsSweep() {
		last_report = self.team.GetLastReport()
	}

	// 对方是否有成员死亡
	has_target_dead := false

	for j := 0; j < len(target_pos); j++ {
		target := target_team.members[target_pos[j]]
		if target == nil && skill_data.SkillTarget != SKILL_TARGET_TYPE_EMPTY_POS {
			continue
		}

		if target != nil {
			target.attacker = self
			target.attacker_skill_data = skill_data
		}

		var report_target *msg_client_message.BattleFighter
		for i := 0; i < len(effects); i++ {
			if effects[i] == nil || len(effects[i]) < 1 {
				continue
			}

			effect_type := effects[i][0]

			if skill_data.SkillTarget != SKILL_TARGET_TYPE_EMPTY_POS {
				if !skill_effect_cond_check(self, target, skill_data.EffectsCond1s[i], skill_data.EffectsCond2s[i]) {
					log.Warn("self[%v] member[%v] cant use skill[%v] to target[%v] member[%v]", self_team.side, self_pos, skill_data.Id, target_team.side, target_pos[j])
					continue
				}
			}

			if effect_type == SKILL_EFFECT_TYPE_DIRECT_INJURY {
				if target == nil || target.is_dead() || target.is_will_dead() {
					continue
				}

				// 被动技，攻击计算伤害前触发
				if skill_data.Type != SKILL_TYPE_PASSIVE {
					passive_skill_effect_with_self_pos(EVENT_BEFORE_DAMAGE_ON_ATTACK, self_team, self_pos, target_team, []int32{target_pos[j]}, true)
				}

				// 被动技，被击计算伤害前触发
				if skill_data.Type != SKILL_TYPE_PASSIVE {
					passive_skill_effect_with_self_pos(EVENT_BEFORE_DAMAGE_ON_BE_ATTACK, target_team, target_pos[j], self_team, []int32{self_pos}, true)
				}

				is_target_dead := target.is_dead()

				// 直接伤害
				target_dmg, self_dmg, is_block, is_critical, is_absorb, anti_type := skill_effect_direct_injury(self, target, skill_data.Type, skill_data.SkillMelee, effects[i])

				if target_dmg != 0 {
					target.add_hp(-target_dmg)
				}

				if self_dmg != 0 {
					self.add_hp(-self_dmg)
				}

				//----------- 战报 -------------
				if !self_team.IsSweep() && skill_data.IsCancelReport == 0 {
					report, report_target = _get_battle_report(report, skill_data.Id, self_team, self_pos, self_dmg, target_team, target_pos[j], target_dmg, is_critical, is_block, is_absorb, anti_type)
				}
				//------------------------------

				used = true

				// 标记临时属性已使用
				skill_effect_temp_attrs_used(self)
				skill_effect_temp_attrs_used(target)

				// 使用一次技能即清空临时属性
				skill_effect_clear_temp_attrs(self)
				skill_effect_clear_temp_attrs(target)

				log.Debug("self_team[%v] member[%v] use skill[%v] to enemy target[%v] with dmg[%v], target hp[%v], reflect self dmg[%v], self hp[%v]", self_team.side, self.pos, skill_data.Id, target.pos, target_dmg, target.hp, self_dmg, self.hp)

				// 被动技，血量变化
				if !target.is_dead() && !target.is_will_dead() && target_dmg != 0 {
					passive_skill_effect_with_self_pos(EVENT_HP_CHANGED, target_team, target_pos[j], nil, nil, true)
				}
				// 被动技，血量变化
				if !self.is_will_dead() && self_dmg != 0 {
					passive_skill_effect_with_self_pos(EVENT_HP_CHANGED, self_team, self_pos, nil, nil, true)
				}

				if skill_data.Type != SKILL_TYPE_PASSIVE {
					// 格挡触发
					if is_block {
						if !self.is_will_dead() {
							passive_skill_effect_with_self_pos(EVENT_BE_BLOCK, self_team, self_pos, target_team, []int32{target_pos[j]}, true)
						}
						if !target.is_will_dead() {
							passive_skill_effect_with_self_pos(EVENT_BLOCK, target_team, target_pos[j], self_team, []int32{self_pos}, true)
						}
					}
					// 暴击触发
					if is_critical {
						if !self.is_will_dead() {
							passive_skill_effect_with_self_pos(EVENT_CRITICAL, self_team, self_pos, target_team, []int32{target_pos[j]}, true)
						}
						if !target.is_will_dead() {
							passive_skill_effect_with_self_pos(EVENT_BE_CRITICAL, target_team, target_pos[j], self_team, []int32{self_pos}, true)
						}
					}

					// 被击计算伤害后触发
					if !target.is_will_dead() {
						passive_skill_effect_with_self_pos(EVENT_AFTER_DAMAGE_ON_BE_ATTACK, target_team, target_pos[j], self_team, []int32{self_pos}, true)
					}
				}

				// 被动技，目标死亡前触发
				if target.is_will_dead() {
					target.on_will_dead(self)
					if report_target != nil {
						report_target.HP = target.hp
					}
				}

				// 再次判断是否真死
				if target.is_will_dead() {

					// 有死亡后触发的被动技
					if target.has_trigger_event([]int32{EVENT_AFTER_TARGET_DEAD}) {
						target.on_after_will_dead(self)
					}

					// 延迟被动技有没有死亡后触发
					if !self_team.HasDelayTriggerEventSkill(EVENT_AFTER_TARGET_DEAD, target) {
						target.set_dead(self, skill_data)
					} else {
						log.Debug("-+-+-+-+-+-+- 有延迟死亡后触发器 team[%v] member[%v]", target.team.side, target.pos)
					}

					// 修改战报目标血量表示真死
					if report_target != nil {
						report_target.HP = target.hp
					}
				}

				// 对方有死亡
				if !is_target_dead && target.is_dead() {
					has_target_dead = true
				}
			} else if effect_type == SKILL_EFFECT_TYPE_CURE {
				if target == nil || target.is_dead() {
					continue
				}
				// 治疗
				cure := skill_effect_cure(self, target, effects[i])
				if cure != 0 {
					target.add_hp(cure)
					// ------------------ 战报 -------------------
					if !self_team.IsSweep() && skill_data.IsCancelReport == 0 {
						report, _ = _get_battle_report(report, skill_data.Id, self_team, self_pos, 0, target_team, target_pos[j], -cure, false, false, false, 0)
					}
					// -------------------------------------------
					// 被动技，治疗时触发
					passive_skill_effect_with_self_pos(EVENT_ON_CURE, target_team, target_pos[j], self_team, []int32{self_pos}, true)
				}

				used = true
				log.Debug("self_team[%v] member[%v] use cure skill[%v] to self target[%v] with resume hp[%v]", self_team.side, self.pos, skill_data.Id, target.pos, cure)
			} else if effect_type == SKILL_EFFECT_TYPE_ADD_BUFF {
				if target == nil || target.is_dead() {
					continue
				}
				// 施加BUFF
				buff_id := skill_effect_add_buff(self, target, effects[i])
				if buff_id > 0 {
					// -------------------- 战报 --------------------
					if !self_team.IsSweep() && skill_data.IsCancelReport == 0 {
						report, _ = _get_battle_report(report, skill_data.Id, self_team, self_pos, 0, target_team, target_pos[j], 0, false, false, false, 0)
						build_battle_report_add_buff(report, target_team, target_pos[j], buff_id)
					}
					// ----------------------------------------------
					used = true
					log.Debug("self_team[%v] member[%v] use skill[%v] to target team[%v] member[%v] 触发 buff[%v]", self_team.side, self.pos, skill_data.Id, target_team.side, target.pos, buff_id)
				} else {
					log.Warn("self_team[%v] member[%v] use skill[%v] add buff failed", self_team.side, self.pos, skill_data.Id)
				}
			} else if effect_type == SKILL_EFFECT_TYPE_SUMMON {
				// 召唤
				mem := skill_effect_summon(self, target_team, target_pos[j], effects[i])
				if mem != nil {
					// --------------------- 战报 ----------------------
					if !self_team.IsSweep() && skill_data.IsCancelReport == 0 {
						report, _ = _get_battle_report(report, skill_data.Id, self_team, self_pos, 0, nil, 0, 0, false, false, false, 0)
						report.IsSummon = true
						build_battle_report_item_add_summon_npc(report, target_team, target_pos[j])
					}
					// -------------------------------------------------
					used = true
					log.Debug("self_team[%v] member[%v] use skill[%v] to summon npc[%v]", self_team.side, self.pos, skill_data.Id, mem.card.Id)
				}
			} else if effect_type == SKILL_EFFECT_TYPE_MODIFY_ATTR {
				if target == nil || target.is_dead() {
					continue
				}
				// 改变下次计算时的角色参数
				skill_effect_temp_attrs(self, effects[i])
				// -------------------- 战报 --------------------
				if !self_team.IsSweep() && skill_data.IsCancelReport == 0 {
					report, _ = _get_battle_report(report, skill_data.Id, self_team, self_pos, 0, target_team, target_pos[j], 0, false, false, false, 0)
				}
				// ----------------------------------------------
				used = true
				log.Debug("self_team[%v] member[%v] use skill[%v] to add temp attrs to target team[%v] member[%v]", self_team.side, self.pos, skill_data.Id, target_team.side, target.pos)
			} else if effect_type == SKILL_EFFECT_TYPE_MODIFY_NORMAL_SKILL {
				// 改变普通攻击技能ID
				if effects[i][1] > 0 {
					self.temp_normal_skill = effects[i][1]
					// -------------------- 战报 --------------------
					if !self_team.IsSweep() && skill_data.IsCancelReport == 0 {
						report, _ = _get_battle_report(report, skill_data.Id, self_team, self_pos, 0, self_team, self_pos, 0, false, false, false, 0)
					}
					// ----------------------------------------------
					used = true
					log.Debug("self_team[%v] pos[%v] role[%v] changed normal skill to %v", self_team.side, self_pos, self.id, self.temp_normal_skill)
				}
			} else if effect_type == SKILL_EFFECT_TYPE_MODIFY_RAGE_SKILL {
				// 改变必杀技ID
				if effects[i][1] > 0 {
					self.temp_super_skill = effects[i][1]
					// -------------------- 战报 --------------------
					if !self_team.IsSweep() && skill_data.IsCancelReport == 0 {
						report, _ = _get_battle_report(report, skill_data.Id, self_team, self_pos, 0, self_team, self_pos, 0, false, false, false, 0)
					}
					// ----------------------------------------------
					used = true
					log.Debug("self_team[%v] pos[%v] role[%v] changed super skill to %v", self_team.side, self_pos, self.id, self.temp_super_skill)
				}
			} else if effect_type == SKILL_EFFECT_TYPE_MODIFY_RAGE {
				// 改变怒气
				if effects[i][3] > 0 {
					if rand.Int31n(10000) < effects[i][3] {
						if target != nil && effects[i][1] > 0 {
							target.energy += effects[i][1]
							if target.energy < 0 {
								target.energy = 0
							}
							log.Debug("team[%v] member[%v] 增加了怒气 [%v]", target_team.side, target.pos, effects[i][1])
						}
						if effects[i][2] > 0 {
							self.energy += effects[i][2]
							if self.energy < 0 {
								self.energy = 0
							}
							log.Debug("team[%v] member[%v] 增加了怒气 [%v]", self_team.side, self.pos, effects[i][2])
						}
						// -------------------- 战报 ----------------------
						if !self_team.IsSweep() && (effects[i][1] > 0 || effects[i][2] > 0) && skill_data.IsCancelReport == 0 {
							report, report_target = _get_battle_report(report, skill_data.Id, self_team, self_pos, 0, target_team, target_pos[j], 0, false, false, false, 0)
							if report != nil {
								report.User.Energy = self.energy
							}
							if report_target != nil {
								report_target.Energy = target.energy
							}
						}
						// ------------------------------------------------
						used = true
					}
				}
			} else if effect_type == SKILL_EFFECT_TYPE_ADD_NORMAL_ATTACK_NUM {
				if target == nil || target.is_dead() {
					continue
				}
				// 增加行动次数
				target.act_num += effects[i][1]
				// -------------------- 战报 --------------------
				if !self_team.IsSweep() && skill_data.IsCancelReport == 0 {
					report, _ = _get_battle_report(report, skill_data.Id, self_team, self_pos, 0, target_team, target_pos[j], 0, false, false, false, 0)
				}
				// ----------------------------------------------
				used = true
				log.Debug("Team[%v] member[%v] 增加了行动次数 %v", target.team.side, target.pos, effects[i][1])
			} else if effect_type == SKILL_EFFECT_TYPE_ADD_SHIELD {
				if target == nil || target.is_dead() {
					continue
				}
				// 增加护盾
				shield := skill_effect_add_shield(self, target, effects[i])
				if shield != 0 {
					target.add_attr(ATTR_SHIELD, shield)
					// ----------------------- 战报 -------------------------
					if !self_team.IsSweep() && skill_data.IsCancelReport == 0 {
						report, _ = _get_battle_report(report, skill_data.Id, self_team, self_pos, 0, target_team, target_pos[j], 0, false, false, false, 0)
					}
					// ------------------------------------------------------
					used = true
				}
			}
		}
	}

	// 被动技，对方有死亡触发
	if /*skill_data.Type != SKILL_TYPE_PASSIVE &&*/ has_target_dead {
		passive_skill_effect_with_self_pos(EVENT_AFTER_ENEMY_DEAD, self_team, self_pos, target_team, target_pos, true)
	}

	if !self.is_will_dead() {
		// 被动技，普攻或大招后
		if skill_data.Type == SKILL_TYPE_NORMAL {
			passive_skill_effect_with_self_pos(EVENT_AFTER_USE_NORMAL_SKILL, self_team, self_pos, target_team, target_pos, true)
		} else if skill_data.Type == SKILL_TYPE_SUPER {
			passive_skill_effect_with_self_pos(EVENT_AFTER_USE_SUPER_SKILL, self_team, self_pos, target_team, target_pos, true)
		}

		// 被动技，攻击计算伤害后触发
		if skill_data.Type != SKILL_TYPE_PASSIVE {
			passive_skill_effect_with_self_pos(EVENT_AFTER_DAMAGE_ON_ATTACK, self_team, self_pos, target_team, target_pos, true)
		}
	}

	// 是否延迟被动技
	if skill_data.IsDelayLastSkill > 0 {
		if used {
			if last_report != nil {
				last_report.HasCombo = true
			}
		}
	}

	if self.is_will_dead() {
		self.set_dead(self.attacker, self.attacker_skill_data)
		if skill_data.IsDelayLastSkill > 0 {
			log.Debug("******************** Team[%v] member[%v] 释放了延迟死亡后被动技，正式死亡", self.team.side, self.pos)
		}
	}

	if report != nil {
		report.User.HP = self.hp
	}

	if skill_data.Type != SKILL_TYPE_PASSIVE {
		log.Debug("++++++++++++++++++++++ end Team[%v] mem[%v] used skill[%v] target_team[%v] target_pos[%v] ++++++++++++++++++++++++++", self_team.side, self_pos, skill_data.Id, target_team.side, target_pos)
	} else {
		log.Debug("====================== end Team[%v] mem[%v] used passive skill[%v] target_team[%v] target_pos[%v] ==========================", self_team.side, self_pos, skill_data.Id, target_team.side, target_pos)
	}

	return
}

// 单个被动技
func one_passive_skill_effect(trigger_event int32, skill *table_config.XmlSkillItem, self *TeamMember, target_team *BattleTeam, trigger_pos []int32, is_combo bool) (triggered bool) {
	if skill.SkillTriggerType != trigger_event {
		return
	}

	if !self.can_passive_trigger(trigger_event, skill.Id) {
		return
	}

	used := false
	r := self.team.GetLastReport()
	if !skill_check_cond(self, target_team, trigger_pos, skill.TriggerCondition1, skill.TriggerCondition2) {
		if target_team != nil {
			log.Debug("BattleTeam[%v] member[%v] use skill[%v] to target team[%v] targets[%v] with condition1[%v] condition2[%v] check failed, self_team[%p] target_team[%p]", self.team.side, self.pos, skill.Id, target_team.side, trigger_pos, skill.TriggerCondition1Str, skill.TriggerCondition2Str, self.team, target_team)
		}
		return
	}

	// 眩晕时禁止触发
	if self.is_disable_attack() && skill.StunDisableAction > 0 {
		return
	}

	target_pos, is_enemy := self.team.FindTargets(self, target_team, skill, trigger_pos)
	if target_pos == nil {
		return
	}

	if !is_enemy {
		target_team = self.team
	}

	used = skill_effect(self.team, self.pos, target_team, target_pos, skill)
	if used {
		if target_team != nil {
			log.Debug("Passive skill[%v] event: %v, self_team[%v] self_pos[%v] target_team[%v] trigger_pos[%v]", skill.Id, trigger_event, self.team.side, self.pos, target_team.side, target_pos)
		} else {
			log.Debug("Passive skill[%v] event: %v, self_team[%v] self_pos[%v]", skill.Id, trigger_event, self.team.side, self.pos)
		}
		if r != nil && is_combo {
			r.HasCombo = true
		}
	}

	self.used_passive_trigger_count(trigger_event, skill.Id)
	triggered = true
	return
}

// 被动技效果
func passive_skill_effect(trigger_event int32, user *TeamMember, target_team *BattleTeam, trigger_pos []int32, is_combo bool) bool {
	if user.passive_skills == nil {
		return false
	}
	effected := false
	for _, skill_id := range user.passive_skills {
		skill := skill_table_mgr.Get(skill_id)
		if skill == nil || skill.Type != SKILL_TYPE_PASSIVE {
			continue
		}

		// 延迟触发
		if skill.IsDelayLastSkill > 0 && trigger_event == skill.SkillTriggerType {
			user.team.PushDelaySkill(trigger_event, skill, user, target_team, trigger_pos)
			log.Debug("Team[%v] member[%v] 触发了延迟被动技[%v]事件[%v]", user.team.side, user.pos, skill.Id, trigger_event)
			continue
		}

		if one_passive_skill_effect(trigger_event, skill, user, target_team, trigger_pos, is_combo) {
			effected = true
		}
	}
	return effected
}

// 被动技效果
func passive_skill_effect_with_self_pos(trigger_event int32, user_team *BattleTeam, user_pos int32, target_team *BattleTeam, trigger_pos []int32, is_combo bool) bool {
	user := user_team.members[user_pos]
	if user == nil {
		return false
	}
	return passive_skill_effect(trigger_event, user, target_team, trigger_pos, is_combo)
}

type Buff struct {
	buff       *table_config.XmlStatusItem
	attacker   *TeamMember
	attack     int32
	dmg_add    int32
	param      int32
	round_num  int32
	resist_num int32
	next       *Buff
	prev       *Buff
	team_side  int32
	owner_pos  int32
}

func (this *Buff) clear() {
	this.buff = nil
	this.attack = 0
	this.dmg_add = 0
	this.param = 0
	this.round_num = 0
	this.resist_num = 0
	this.next = nil
	this.prev = nil
	this.team_side = 0
	this.owner_pos = 0
}

type BuffList struct {
	owner *TeamMember
	head  *Buff
	tail  *Buff
	buffs map[*Buff]*Buff
}

func (this *BuffList) clear() {
	b := this.head
	for b != nil {
		next := b.next
		buff_pool.Put(b)
		delete(this.buffs, b)
		b = next
	}
	this.head = nil
	this.tail = nil
	this.owner = nil
}

func (this *BuffList) remove_buff(buff *Buff) bool {
	if this.buffs == nil || this.buffs[buff] == nil {
		log.Error("XXXXXXXXXXXXXXXXXXXXX Team[%v] member[%v] no buff[%p, team_side:%v, owner_pos:%v] to remove", this.owner.team.side, this.owner.pos, buff, buff.team_side, buff.owner_pos)
		return false
	}
	if buff.prev != nil {
		buff.prev.next = buff.next
	}
	if buff.next != nil {
		buff.next.prev = buff.prev
	}
	if buff == this.head {
		this.head = buff.next
	}
	if buff == this.tail {
		this.tail = buff.prev
	}

	if this.owner != nil {
		this.owner.remove_buff_effect(buff)
	}

	buff_pool.Put(buff)
	delete(this.buffs, buff)

	// 测试
	b := this.head
	for b != nil {
		if this.buffs[b] == nil {
			s := fmt.Sprintf("============================ Team[%v] member[%v] no buff[%p,%v] after remove buff[%p,%v]", this.owner.team.side, this.owner.pos, b, b, buff, buff)
			panic(errors.New(s))
		}
		b = b.next
	}

	return true
}

// 战报删除BUFF
func (this *BuffList) add_remove_buff_report(buff_id int32) {
	report := this.owner.team.GetLastReport()
	if report != nil {
		buff := msg_battle_buff_item_pool.Get()
		buff.Pos = this.owner.pos
		buff.BuffId = buff_id
		buff.Side = this.owner.team.side
		report.RemoveBuffs = append(report.RemoveBuffs, buff)
	}
}

// 免疫次数
func (this *BuffList) cost_resist_num(buff *Buff) {
	// 免疫次数
	if buff.resist_num > 0 {
		if buff.resist_num > 1 {
			buff.resist_num -= 1
		} else {
			b := buff.buff
			this.remove_buff(buff)
			this.add_remove_buff_report(b.Id)
			log.Debug("Team[%v] member[%v] BUFF[%v]类型免疫次数[%v]用完", this.owner.team.side, this.owner.pos, b.Id, buff.buff.ResistCountMax)
		}
	}
}

// 检测BUFF互斥或免疫
func (this *BuffList) check_buff_mutex(b *table_config.XmlStatusItem) bool {
	hh := this.head
	for hh != nil {
		next := hh.next
		// 免疫类型
		for j := 0; j < len(hh.buff.ResistMutexTypes); j++ {
			if b.MutexType > 0 && b.MutexType == hh.buff.ResistMutexTypes[j] {
				this.cost_resist_num(hh)
				log.Debug("Team[%v] member[%v] BUFF[%v]类型[%v]排斥BUFF[%v]类型[%v]", this.owner.team.side, this.owner.pos, hh.buff.Id, hh.buff.MutexType, b.Id, b.MutexType)
				return true
			}
		}
		// 免疫BUFF ID
		for j := 0; j < len(hh.buff.ResistMutexIDs); j++ {
			if b.MutexType > 0 && b.Id == hh.buff.ResistMutexIDs[j] {
				this.cost_resist_num(hh)
				log.Debug("Team[%v] member[%v] BUFF[%v]排斥BUFF[%v]", this.owner.team.side, this.owner.pos, hh.buff.Id, b.Id)
				return true
			}
		}

		// 驱散类型
		for j := 0; j < len(b.CancelMutexTypes); j++ {
			if hh.buff.MutexType > 0 && hh.buff.MutexType == b.CancelMutexTypes[j] {
				this.remove_buff(hh)
				this.add_remove_buff_report(hh.buff.Id)
				log.Debug("Team[%v] member[%v] BUFF[%v]类型[%v]驱散了BUFF[%v]类型[%v]", this.owner.team.side, this.owner.pos, b.Id, b.CancelMutexTypes[j], hh.buff.Id, hh.buff.MutexType)
			}
		}
		// 驱散BUFF ID
		for j := 0; j < len(b.CancelMutexIDs); j++ {
			if hh.buff.MutexType > 0 && hh.buff.Id == b.CancelMutexIDs[j] {
				this.remove_buff(hh)
				this.add_remove_buff_report(hh.buff.Id)
				log.Debug("Team[%v] member[%v] BUFF[%v]驱散了BUFF[%v]", this.owner.team.side, this.owner.pos, b.Id, hh.buff.Id)
			}
		}
		hh = next
	}
	return false
}

func (this *BuffList) add_buff(attacker *TeamMember, b *table_config.XmlStatusItem, skill_effect []int32) (buff_id int32) {
	buff := buff_pool.Get()
	buff.clear()

	if this.buffs == nil {
		this.buffs = make(map[*Buff]*Buff)
	}
	if _, o := this.buffs[buff]; o {
		log.Error("XXXXXXXXXXXXXXXXXXX Team[%v] member[%v] add buff[%p] already exist", this.owner.team.side, this.owner.pos, buff)
		return
	}

	buff.buff = b
	buff.attacker = attacker
	buff.attack = attacker.attrs[ATTR_ATTACK]
	buff.dmg_add = attacker.attrs[ATTR_TOTAL_DAMAGE_ADD]
	buff.param = skill_effect[3]
	buff.round_num = skill_effect[4]
	buff.resist_num = b.ResistCountMax

	if this.head == nil {
		buff.prev = nil
		this.head = buff
	} else {
		buff.prev = this.tail
		this.tail.next = buff
	}
	this.tail = buff
	buff.next = nil
	buff.team_side = this.owner.team.side
	buff.owner_pos = this.owner.pos
	this.buffs[buff] = buff
	buff_id = b.Id

	if this.owner != nil {
		this.owner.add_buff_effect(buff, skill_effect)
	}

	// 测试
	bb := this.head
	for bb != nil {
		if this.buffs[bb] == nil {
			s := fmt.Sprintf("============================ Team[%v] member[%v] no buff[%p,%v] after add buff[%p,%v]", this.owner.team.side, this.owner.pos, bb, bb, buff, buff)
			panic(errors.New(s))
		}
		bb = bb.next
	}
	return
}

func (this *BuffList) on_round_end() {
	if this.owner.is_will_dead() || this.owner.is_dead() {
		return
	}

	var item *msg_client_message.BattleFighter
	members_damage := this.owner.team.common_data.members_damage
	members_cure := this.owner.team.common_data.members_cure
	bf := this.head
	for bf != nil {
		next := bf.next
		if bf.round_num > 0 {
			if bf.buff.Effect[0] == BUFF_EFFECT_TYPE_DAMAGE {
				dmg := buff_effect_damage(bf.attack, bf.dmg_add, bf.param, bf.buff.Effect[1], this.owner)
				if dmg != 0 {
					this.owner.add_hp(-dmg)
					if this.owner.is_will_dead() {
						this.owner.on_will_dead(bf.attacker)
					}
					if this.owner.is_will_dead() {
						// 有死亡后触发的被动技
						if this.owner.has_trigger_event([]int32{EVENT_AFTER_TARGET_DEAD}) {
							this.owner.on_after_will_dead(bf.attacker)
						}
						// 延迟被动技有没有死亡后触发
						if !bf.attacker.team.HasDelayTriggerEventSkill(EVENT_AFTER_TARGET_DEAD, this.owner) {
							this.owner.set_dead(bf.attacker, nil)
						}
					}
					// --------------------------- 战报 ---------------------------
					if !this.owner.team.IsSweep() {
						// 血量变化的成员
						if item == nil {
							item = this.owner.build_battle_fighter(0)
							item.Side = this.owner.team.side
							this.owner.team.common_data.changed_fighters = append(this.owner.team.common_data.changed_fighters, item)
						}
						item.Damage += dmg
						if dmg != 0 {
							if bf.attacker != nil {
								if dmg > 0 {
									members_damage[bf.attacker.team.side][bf.attacker.pos] += dmg
								} else {
									members_cure[bf.attacker.team.side][bf.attacker.pos] += -dmg
								}
							}
						}
					}
					// ------------------------------------------------------------
					log.Debug("Team[%v] member[%v] hp damage[%v] on buff[%v] left round[%v] end", this.owner.team.side, this.owner.pos, dmg, bf.buff.Id, bf.round_num)
				}
			}

			bf.round_num -= 1
			if bf.round_num <= 0 {
				buff_id := bf.buff.Id
				this.remove_buff(bf)
				// --------------------------- 战报 ---------------------------
				if !this.owner.team.IsSweep() {
					b := msg_battle_buff_item_pool.Get()
					b.BuffId = buff_id
					b.Pos = this.owner.pos
					b.Side = this.owner.team.side
					this.owner.team.common_data.remove_buffs = append(this.owner.team.common_data.remove_buffs, b)
				}
				// ------------------------------------------------------------
				log.Debug("Team[%v] member[%v] buff[%v] round over", this.owner.team.side, this.owner.pos, buff_id)
			}
		}
		bf = next
	}

	this.owner.team.DelaySkillEffect()

	if item != nil {
		item.HP = this.owner.hp
	}
}

// 状态伤害效果
func buff_effect_damage(user_attack, user_damage_add, skill_damage_coeff, attr int32, target *TeamMember) (damage int32) {
	base_damage := user_attack * skill_damage_coeff / 10000
	f := float64(10000 - target.attrs[attr])
	damage = int32(math.Max(1, float64(base_damage)*math.Max(0.1, f)/10000))
	return
}
