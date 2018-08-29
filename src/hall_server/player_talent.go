package main

import (
	"ih_server/libs/log"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"net/http"

	"github.com/golang/protobuf/proto"
)

func (this *Player) get_talent_list() []*msg_client_message.TalentInfo {
	all := this.db.Talents.GetAllIndex()
	if all == nil || len(all) == 0 {
		return make([]*msg_client_message.TalentInfo, 0)
	}

	var talents []*msg_client_message.TalentInfo
	for i := 0; i < len(all); i++ {
		lvl, o := this.db.Talents.GetLevel(all[i])
		if !o {
			continue
		}
		talents = append(talents, &msg_client_message.TalentInfo{
			Id:    all[i],
			Level: lvl,
		})
	}
	return talents
}

func (this *Player) send_talent_list() int32 {
	talents := this.get_talent_list()
	response := &msg_client_message.S2CTalentListResponse{
		Talents: talents,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_TALENT_LIST_RESPONSE), response)
	log.Debug("Player[%v] send talent list %v", this.Id, talents)
	return 1
}

func (this *Player) up_talent(talent_id int32) int32 {
	level, _ := this.db.Talents.GetLevel(talent_id)
	talent := talent_table_mgr.GetByIdLevel(talent_id, level+1)
	if talent == nil {
		log.Error("Talent[%v,%v] data not found", talent_id, level+1)
		return int32(msg_client_message.E_ERR_PLAYER_TALENT_NOT_FOUND)
	}

	if talent.CanLearn <= 0 {
		log.Error("talent[%v] cant learn", talent_id)
		return -1
	}

	if talent.PrevSkillCond > 0 {
		prev_level, o := this.db.Talents.GetLevel(talent.PrevSkillCond)
		if !o || prev_level < talent.PreSkillLevCond {
			log.Error("Player[%v] up talent %v need prev talent[%v] level[%v]", this.Id, talent_id, talent.PrevSkillCond, talent.PreSkillLevCond)
			return int32(msg_client_message.E_ERR_PLAYER_TALENT_UP_NEED_PREV_TALENT)
		}
	}

	// check cost
	for i := 0; i < len(talent.UpgradeCost)/2; i++ {
		rid := talent.UpgradeCost[2*i]
		rct := talent.UpgradeCost[2*i+1]
		if this.get_resource(rid) < rct {
			log.Error("Player[%v] up talent[%v] not enough resource[%v]", this.Id, talent_id, rid)
			return int32(msg_client_message.E_ERR_PLAYER_TALENT_UP_NOT_ENOUGH_RESOURCE)
		}
	}

	// cost resource
	for i := 0; i < len(talent.UpgradeCost)/2; i++ {
		rid := talent.UpgradeCost[2*i]
		rct := talent.UpgradeCost[2*i+1]
		this.add_resource(rid, -rct)
	}

	if level == 0 {
		level += 1
		this.db.Talents.Add(&dbPlayerTalentData{
			Id:    talent_id,
			Level: level,
		})
	} else {
		level += 1
		this.db.Talents.SetLevel(talent_id, level)
	}

	response := &msg_client_message.S2CTalentUpResponse{
		TalentId: talent_id,
		Level:    level,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_TALENT_UP_RESPONSE), response)

	log.Debug("Player[%v] update talent[%v] to level[%v]", this.Id, talent_id, level)

	return 1
}

func (this *Player) talent_reset(tag int32) int32 {
	if this.db.Talents.NumAll() <= 0 {
		return -1
	}

	if this.get_diamond() < global_config.TalentResetCostDiamond {
		log.Error("Player[%v] reset talent need diamond not enough", this.Id)
		return int32(msg_client_message.E_ERR_PLAYER_DIAMOND_NOT_ENOUGH)
	}

	return_items := make(map[int32]int32)
	talent_ids := this.db.Talents.GetAllIndex()
	for i := 0; i < len(talent_ids); i++ {
		talent_id := talent_ids[i]
		level, _ := this.db.Talents.GetLevel(talent_id)
		for l := int32(1); l <= level; l++ {
			t := talent_table_mgr.GetByIdLevel(talent_id, l)
			if t == nil {
				continue
			}
			if t.Tag != tag {
				continue
			}
			for n := 0; n < len(t.UpgradeCost)/2; n++ {
				return_items[t.UpgradeCost[2*n]] += t.UpgradeCost[2*n+1]
			}
			if this.db.Talents.HasIndex(talent_id) {
				this.db.Talents.Remove(talent_id)
			}
		}
	}

	this.add_diamond(-global_config.TalentResetCostDiamond)

	var items []int32
	for k, v := range return_items {
		items = append(items, []int32{k, v}...)
	}
	response := &msg_client_message.S2CTalentResetResponse{
		Tag:         tag,
		ReturnItems: items,
		CostDiamond: global_config.TalentResetCostDiamond,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_TALENT_RESET_RESPONSE), response)

	log.Debug("Player[%v] reset talents tag[%v], return items %v, cost diamond %v", this.Id, tag, items, response.GetCostDiamond())
	return 1
}

func (this *Player) add_talent_attr(member *TeamMember) {
	all_tid := this.db.Talents.GetAllIndex()
	if all_tid == nil {
		return
	}

	for i := 0; i < len(all_tid); i++ {
		lvl, _ := this.db.Talents.GetLevel(all_tid[i])
		t := talent_table_mgr.GetByIdLevel(all_tid[i], lvl)
		if t == nil {
			log.Error("Player[%v] talent[%v] level[%v] data not found", this.Id, all_tid[i], lvl)
			continue
		}

		log.Debug("talent[%v] effect_cond[%v] attrs[%v] skills[%v] first_hand[%v]", all_tid[i], t.TalentEffectCond, t.TalentAttr, t.TalentSkillList, t.TeamSpeedBonus)
		if member != nil && !member.is_dead() {
			if !_skill_check_cond(member, t.TalentEffectCond) {
				continue
			}
			member.add_attrs(t.TalentAttr)
			for k := 0; k < len(t.TalentSkillList); k++ {
				member.add_passive_skill(t.TalentSkillList[k])
			}
		}

		if member.team != nil {
			member.team.first_hand += t.TeamSpeedBonus
		}
	}
}

func C2STalentListHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2STalentListRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}
	return p.send_talent_list()
}

func C2STalentUpHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2STalentUpRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}
	return p.up_talent(req.GetTalentId())
}

func C2STalentResetHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2STalentResetRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}
	return p.talent_reset(req.GetTag())
}
