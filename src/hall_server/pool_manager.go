package main

import (
	"ih_server/proto/gen_go/client_message"
	"sync"
)

// 战报
type BattleReportPool struct {
	pool *sync.Pool
}

func (this *BattleReportPool) Init() {
	this.pool = &sync.Pool{
		New: func() interface{} {
			m := &msg_client_message.BattleReportItem{}
			return m
		},
	}
}

func (this *BattleReportPool) Get() *msg_client_message.BattleReportItem {
	return this.pool.Get().(*msg_client_message.BattleReportItem)
}

func (this *BattleReportPool) Put(m *msg_client_message.BattleReportItem) {
	this.pool.Put(m)
}

// 阵型成员
type TeamMemberPool struct {
	pool   *sync.Pool
	locker *sync.RWMutex
}

func (this *TeamMemberPool) Init() {
	this.pool = &sync.Pool{
		New: func() interface{} {
			m := &TeamMember{}
			return m
		},
	}
	this.locker = &sync.RWMutex{}
}

func (this *TeamMemberPool) Get() *TeamMember {
	return this.pool.Get().(*TeamMember)
}

func (this *TeamMemberPool) Put(m *TeamMember) {
	this.pool.Put(m)
}

// BUFF
type BuffPool struct {
	pool *sync.Pool
}

func (this *BuffPool) Init() {
	this.pool = &sync.Pool{
		New: func() interface{} {
			return &Buff{}
		},
	}
}

func (this *BuffPool) Get() *Buff {
	return this.pool.Get().(*Buff)
}

func (this *BuffPool) Put(b *Buff) {
	this.pool.Put(b)
}

// MemberPassiveTriggerData
type MemberPassiveTriggerDataPool struct {
	pool *sync.Pool
}

func (this *MemberPassiveTriggerDataPool) Init() {
	this.pool = &sync.Pool{
		New: func() interface{} {
			return &PassiveTriggerData{}
		},
	}
}

func (this *MemberPassiveTriggerDataPool) Get() *PassiveTriggerData {
	return this.pool.Get().(*PassiveTriggerData)
}

func (this *MemberPassiveTriggerDataPool) Put(d *PassiveTriggerData) {
	this.pool.Put(d)
}

// MsgBattleMemberItemPool
type MsgBattleMemberItemPool struct {
	pool *sync.Pool
}

func (this *MsgBattleMemberItemPool) Init() {
	this.pool = &sync.Pool{
		New: func() interface{} {
			return &msg_client_message.BattleMemberItem{}
		},
	}
}

func (this *MsgBattleMemberItemPool) Get() *msg_client_message.BattleMemberItem {
	return this.pool.Get().(*msg_client_message.BattleMemberItem)
}

func (this *MsgBattleMemberItemPool) Put(item *msg_client_message.BattleMemberItem) {
	this.pool.Put(item)
}

// MsgBattleFighterPool
type MsgBattleFighterPool struct {
	pool *sync.Pool
}

func (this *MsgBattleFighterPool) Init() {
	this.pool = &sync.Pool{
		New: func() interface{} {
			return &msg_client_message.BattleFighter{}
		},
	}
}

func (this *MsgBattleFighterPool) Get() *msg_client_message.BattleFighter {
	return this.pool.Get().(*msg_client_message.BattleFighter)
}

func (this *MsgBattleFighterPool) Put(fighter *msg_client_message.BattleFighter) {
	this.pool.Put(fighter)
}

// MsgBattleMemberBuffPool
type MsgBattleMemberBuffPool struct {
	pool *sync.Pool
}

func (this *MsgBattleMemberBuffPool) Init() {
	this.pool = &sync.Pool{
		New: func() interface{} {
			return &msg_client_message.BattleMemberBuff{}
		},
	}
}

func (this *MsgBattleMemberBuffPool) Get() *msg_client_message.BattleMemberBuff {
	return this.pool.Get().(*msg_client_message.BattleMemberBuff)
}

func (this *MsgBattleMemberBuffPool) Put(buff *msg_client_message.BattleMemberBuff) {
	this.pool.Put(buff)
}

// MsgBattleReportItemPool
type MsgBattleReportItemPool struct {
	pool *sync.Pool
}

func (this *MsgBattleReportItemPool) Init() {
	this.pool = &sync.Pool{
		New: func() interface{} {
			return &msg_client_message.BattleReportItem{}
		},
	}
}

func (this *MsgBattleReportItemPool) Get() *msg_client_message.BattleReportItem {
	return this.pool.Get().(*msg_client_message.BattleReportItem)
}

func (this *MsgBattleReportItemPool) Put(item *msg_client_message.BattleReportItem) {
	this.pool.Put(item)
}

// MsgBattleRoundReportsPool
type MsgBattleRoundReportsPool struct {
	pool *sync.Pool
}

func (this *MsgBattleRoundReportsPool) Init() {
	this.pool = &sync.Pool{
		New: func() interface{} {
			return &msg_client_message.BattleRoundReports{}
		},
	}
}

func (this *MsgBattleRoundReportsPool) Get() *msg_client_message.BattleRoundReports {
	return this.pool.Get().(*msg_client_message.BattleRoundReports)
}

func (this *MsgBattleRoundReportsPool) Put(reports *msg_client_message.BattleRoundReports) {
	this.pool.Put(reports)
}

// DelaySkillPool
type DelaySkillPool struct {
	pool *sync.Pool
}

func (this *DelaySkillPool) Init() {
	this.pool = &sync.Pool{
		New: func() interface{} {
			return &DelaySkill{}
		},
	}
}

func (this *DelaySkillPool) Get() *DelaySkill {
	return this.pool.Get().(*DelaySkill)
}

func (this *DelaySkillPool) Put(ds *DelaySkill) {
	this.pool.Put(ds)
}
