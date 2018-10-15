package main

import (
	_ "ih_server/libs/log"
	"ih_server/libs/utils"
	"sync/atomic"
)

// 战力排行榜序号
//var role_power_rank_serial_id int32
var roles_power_rank_serial_id int32

type RolePowerRankItem struct {
	//SerialId int32
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
		/*if this.SerialId > it.SerialId {
			return true
		}*/
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
		/*if this.SerialId < it.SerialId {
			return true
		}*/
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
	//this.SerialId = it.SerialId
}

func (this *RolePowerRankItem) Add(item utils.ShortRankItem) {

}

func (this *RolePowerRankItem) New() utils.ShortRankItem {
	return &RolePowerRankItem{}
}

type RolesPowerRankItem struct {
	SerialId int32
	Power    int32
	PlayerId int32
}

func (this *RolesPowerRankItem) Less(value interface{}) bool {
	item := value.(*RolesPowerRankItem)
	if item == nil {
		return false
	}
	if this.Power < item.Power {
		return true
	} else if this.Power == item.Power {
		if this.SerialId > item.SerialId {
			return true
		}
	}
	return false
}

func (this *RolesPowerRankItem) Greater(value interface{}) bool {
	item := value.(*RolesPowerRankItem)
	if item == nil {
		return false
	}
	if this.Power > item.Power {
		return true
	} else if this.Power == item.Power {
		if this.SerialId < item.SerialId {
			return true
		}
	}
	return false
}

func (this *RolesPowerRankItem) KeyEqual(value interface{}) bool {
	item := value.(*RolesPowerRankItem)
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

func (this *RolesPowerRankItem) GetKey() interface{} {
	return this.PlayerId
}

func (this *RolesPowerRankItem) GetValue() interface{} {
	return this.Power
}

func (this *RolesPowerRankItem) SetValue(value interface{}) {
	this.Power = value.(int32)
	this.SerialId = atomic.AddInt32(&roles_power_rank_serial_id, 1)
}

func (this *RolesPowerRankItem) New() utils.SkiplistNode {
	return &RolesPowerRankItem{}
}

func (this *RolesPowerRankItem) Assign(node utils.SkiplistNode) {
	n := node.(*RolesPowerRankItem)
	if n == nil {
		return
	}
	this.PlayerId = n.PlayerId
	this.Power = n.Power
	this.SerialId = n.SerialId
}

func (this *RolesPowerRankItem) CopyDataTo(node interface{}) {
	n := node.(*RolesPowerRankItem)
	if n == nil {
		return
	}
	n.PlayerId = this.PlayerId
	n.Power = this.Power
	n.SerialId = this.SerialId
}

// 更新排名
const (
	MAX_ROLES_POWER_NUM_TO_RANK_ITEM = 4
)

func (this *Player) _update_role_power_rank_info(role_id, power int32) {
	var item = RolePowerRankItem{
		RoleId: role_id,
		Power:  power,
		//SerialId: atomic.AddInt32(&role_power_rank_serial_id, 1),
	}
	this.role_power_ranklist.Update(&item, false)
}

func (this *Player) _update_roles_power_rank_info(power int32) {
	sid := atomic.AddInt32(&roles_power_rank_serial_id, 1)
	var data = RolesPowerRankItem{
		SerialId: sid,
		Power:    power,
		PlayerId: this.Id,
	}
	rank_list_mgr.UpdateItem(RANK_LIST_TYPE_ROLE_POWER, &data)
}

func (this *Player) _update_roles_power_rank_info2() {
	var power int32
	// 放入所有玩家的角色战力中排序
	for r := 1; r <= MAX_ROLES_POWER_NUM_TO_RANK_ITEM; r++ {
		_, value := this.role_power_ranklist.GetByRank(int32(r))
		if value == nil {
			continue
		}
		p := value.(int32)
		power += p
	}
	this._update_roles_power_rank_info(power)
	//log.Debug("Player[%v] update roles power %v in rank list", this.Id, power)
}

func (this *Player) UpdateRolePowerRank(role_id int32) {
	this.role_update_suit_attr_power(role_id, false, true)
	power := this.get_role_power(role_id)
	this._update_role_power_rank_info(role_id, power)
	this._update_roles_power_rank_info2()
}

func (this *Player) DeleteRolePowerRank(role_id int32) {
	if this.role_power_ranklist == nil {
		return
	}
	this.role_power_ranklist.Delete(role_id)
	this._update_roles_power_rank_info2()
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
		//log.Debug("Player[%v] role[%v] power[%v]", this.Id, id, power)
		this._update_role_power_rank_info(id, power)
	}

	this._update_roles_power_rank_info2()
}
