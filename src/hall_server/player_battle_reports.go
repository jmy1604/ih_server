package main

import (
	"ih_server/libs/log"
	"ih_server/proto/gen_go/client_message"
)

func (this *TeamMember) build_battle_fighter(damage int32) *msg_client_message.BattleFighter {
	item := msg_battle_fighter_pool.Get()
	item.Side = this.team.side
	item.Pos = this.pos
	item.HP = this.hp
	item.MaxHP = this.attrs[ATTR_HP_MAX]
	item.Energy = this.energy
	item.Damage = damage
	item.Shield = this.attrs[ATTR_SHIELD]
	return item
}

func (this *TeamMember) build_battle_member() *msg_client_message.BattleMemberItem {
	mem := msg_battle_member_item_pool.Get()
	mem.Side = this.team.side
	mem.Id = this.id
	mem.TableId = this.card.Id
	mem.Rank = this.card.Rank
	mem.Level = this.level
	mem.Pos = this.pos
	mem.HP = this.hp
	mem.MaxHP = this.attrs[ATTR_HP_MAX]
	mem.Energy = this.energy
	return mem
}

func build_battle_report_item(self_team *BattleTeam, self_pos int32, self_damage int32, skill_id int32) *msg_client_message.BattleReportItem {
	self := self_team.members[self_pos]
	if self == nil {
		log.Error("+++++++++++++++++++++++++-------------------- self_team[%v] member[%v] not found", self_team.side, self_pos)
		return nil
	}
	item := msg_battle_reports_item_pool.Get()
	item.Side = self_team.side
	item.SkillId = skill_id
	item.User = self.build_battle_fighter(self_damage)
	item.BeHiters = make([]*msg_client_message.BattleFighter, 0)
	item.SummonNpcs = make([]*msg_client_message.BattleMemberItem, 0)
	item.AddBuffs = make([]*msg_client_message.BattleMemberBuff, 0)
	item.RemoveBuffs = make([]*msg_client_message.BattleMemberBuff, 0)
	item.HasCombo = false
	return item
}

func build_battle_report_item_add_target_item(item *msg_client_message.BattleReportItem, target_team *BattleTeam, target_pos int32, target_damage int32, is_critical, is_block, is_absorb bool, anti_type int32) *msg_client_message.BattleFighter {
	if item == nil || target_team == nil || target_team.members == nil {
		return nil
	}
	target := target_team.members[target_pos]
	if target == nil {
		return nil
	}

	// 已经加到列表中的受击对象
	if item.BeHiters != nil {
		for i := 0; i < len(item.BeHiters); i++ {
			behiter := item.BeHiters[i]
			if behiter.Side == target.team.side && behiter.Pos == target.pos {
				behiter.Damage += target_damage
				return behiter
			}
		}
	}

	mem_item := target.build_battle_fighter(target_damage)
	mem_item.IsCritical = is_critical
	mem_item.IsBlock = is_block
	mem_item.IsAbsorb = is_absorb
	mem_item.AntiType = anti_type

	if item.BeHiters == nil {
		item.BeHiters = []*msg_client_message.BattleFighter{mem_item}
	} else {
		item.BeHiters = append(item.BeHiters, mem_item)
	}
	return mem_item
}

func build_battle_report_item_add_summon_npc(item *msg_client_message.BattleReportItem, target_team *BattleTeam, target_pos int32) {
	if item == nil {
		return
	}
	target := target_team.members[target_pos]
	if target == nil {
		return
	}
	npc := target.build_battle_member()
	if item.SummonNpcs == nil {
		item.SummonNpcs = []*msg_client_message.BattleMemberItem{npc}
	} else {
		item.SummonNpcs = append(item.SummonNpcs, npc)
	}
}

func build_battle_report_add_buff(item *msg_client_message.BattleReportItem, target_team *BattleTeam, target_pos int32, buff_id int32) {
	if item == nil {
		return
	}
	buff := msg_battle_buff_item_pool.Get()
	buff.Side = target_team.side
	buff.Pos = target_pos
	buff.BuffId = buff_id
	if item.AddBuffs == nil {
		item.AddBuffs = []*msg_client_message.BattleMemberBuff{buff}
	} else {
		item.AddBuffs = append(item.AddBuffs, buff)
	}
}

func build_battle_report_remove_buff(item *msg_client_message.BattleReportItem, target_team *BattleTeam, target_pos int32, buff_id int32) {
	if item == nil {
		return
	}
	buff := msg_battle_buff_item_pool.Get()
	buff.Side = target_team.side
	buff.Pos = target_pos
	buff.BuffId = buff_id
	if item.RemoveBuffs == nil {
		item.RemoveBuffs = []*msg_client_message.BattleMemberBuff{buff}
	} else {
		item.RemoveBuffs = append(item.RemoveBuffs, buff)
	}
}
