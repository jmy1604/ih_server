package main

import (
	_ "ih_server/libs/log"
	"ih_server/libs/utils"
	"time"
)

type RolePowerRankItem struct {
	Power  int32
	RoleId int32
}

func (this *RolePowerRankItem) Less(item utils.ShortRankItem) bool {
	it := item.(*RolePowerRankItem)
	if it == nil {
		return false
	}
	if this.Power < it.Power {
		return true
	} else if this.Power == it.Power {
	}
	return false
}

func (this *RolePowerRankItem) Greater(item utils.ShortRankItem) bool {
	it := item.(*RolePowerRankItem)
	if it == nil {
		return false
	}
	if this.Power > it.Power {
		return true
	} else if this.Power == it.Power {
	}
	return false
}

func (this *RolePowerRankItem) GetKey() interface{} {
	return this.RoleId
}

func (this *RolePowerRankItem) GetValue() interface{} {
	return this.Power
}

func (this *RolePowerRankItem) Assign(item utils.ShortRankItem) {
	it := item.(*RolePowerRankItem)
	if it == nil {
		return
	}
	this.RoleId = it.RoleId
	this.Power = it.Power
}

func (this *RolePowerRankItem) Add(item utils.ShortRankItem) {

}

func (this *RolePowerRankItem) New() utils.ShortRankItem {
	return &RolePowerRankItem{}
}

// 更新排名
const (
	MAX_ROLES_POWER_NUM_TO_RANK_ITEM = 4
)

func (this *Player) _update_role_power_rank_info(role_id, power int32) {
	var item = RolePowerRankItem{
		RoleId: role_id,
		Power:  power,
	}
	this.role_power_ranklist.Update(&item, false)
}

func (this *Player) _update_roles_power_rank_info(update_time int32) {
	// 放入所有玩家的角色战力中排序
	var power int32
	for r := 1; r <= MAX_ROLES_POWER_NUM_TO_RANK_ITEM; r++ {
		_, value := this.role_power_ranklist.GetByRank(int32(r))
		if value == nil {
			continue
		}
		p := value.(int32)
		power += p
	}
	if power <= 0 {
		return
	}
	var data = PlayerInt32RankItem{
		Value:      power,
		UpdateTime: update_time,
		PlayerId:   this.Id,
	}
	rank_list_mgr.UpdateItem(RANK_LIST_TYPE_ROLE_POWER, &data)
}

func (this *Player) _update_roles_power_rank_info_now() {
	now_time := int32(time.Now().Unix())
	this._update_roles_power_rank_info(now_time)
	this.db.RoleCommon.SetPowerUpdateTime(now_time)
}

func (this *Player) UpdateRolePowerRank(role_id int32) {
	this.role_update_suit_attr_power(role_id, false, true)
	power := this.get_role_power(role_id)
	before_rank := this.role_power_ranklist.GetRank(role_id)
	this._update_role_power_rank_info(role_id, power)
	after_rank := this.role_power_ranklist.GetRank(role_id)
	if (before_rank >= 1 && before_rank <= MAX_ROLES_POWER_NUM_TO_RANK_ITEM) || (after_rank >= 1 && after_rank <= MAX_ROLES_POWER_NUM_TO_RANK_ITEM) {
		this._update_roles_power_rank_info_now()
	}
}

func (this *Player) DeleteRolePowerRank(role_id int32) {
	if this.role_power_ranklist == nil {
		return
	}
	rank := this.role_power_ranklist.GetRank(role_id)
	this.role_power_ranklist.Delete(role_id)
	if rank >= 1 && rank <= MAX_ROLES_POWER_NUM_TO_RANK_ITEM {
		this._update_roles_power_rank_info_now()
	}
}

// 载入数据库角色战力
func (this *Player) LoadRolesPowerRankData() {
	ids := this.db.Roles.GetAllIndex()
	if ids == nil {
		return
	}

	// 载入个人角色战力并排序
	for _, id := range ids {
		this.role_update_suit_attr_power(id, false, true)
		power := this.get_role_power(id)
		if power > 0 {
			this._update_role_power_rank_info(id, power)
		}
	}

	update_time := this.db.RoleCommon.GetPowerUpdateTime()
	if update_time == 0 {
		update_time = this.db.Info.GetLastLogin()
	}
	this._update_roles_power_rank_info(update_time)
}
