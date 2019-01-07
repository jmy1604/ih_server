package main

import (
	"ih_server/libs/log"
	"sync/atomic"
)

func (this *DBC) on_preload() (err error) {
	var p *Player
	for _, db := range this.Players.m_rows {
		if nil == db {
			log.Error("DBC on_preload Players have nil db !")
			continue
		}

		p = new_player_with_db(db.m_PlayerId, db)
		if nil == p {
			continue
		}

		player_mgr.Add2IdMap(p)
		player_mgr.Add2UidMap(p.UniqueId, p)

		friend_recommend_mgr.CheckAndAddPlayer(p.Id)

		if p.db.GetLevel() == 0 {
			p.db.SetLevel(p.db.Info.GetLvl())
		}

		defense_power := p.get_defense_team_power()
		if defense_power > 0 {
			arena_season_mgr.top_power_ranklist.Update(p.Id, defense_power)
		}

		//login_token_mgr.AddToAcc2Token(p.Account, p.Token, p.Id)
		//login_token_mgr.AddToId2Acc(p.Id, p.Account)
	}

	return
}

func (this *dbGlobalRow) GetNextPlayerId() int32 {
	curr_id := atomic.AddInt32(&this.m_CurrentPlayerId, 1)
	new_id := ((config.ServerId << 20) & 0x7ff00000) | curr_id
	this.m_lock.UnSafeLock("dbGlobalRow.GetNextPlayerId")
	this.m_CurrentPlayerId_changed = true
	this.m_lock.UnSafeUnlock()
	return new_id
}

func (this *dbGlobalRow) GetNextGuildId() int32 {
	curr_id := atomic.AddInt32(&this.m_CurrentGuildId, 1)
	new_id := ((config.ServerId << 20) & 0x7ff00000) | curr_id
	this.m_lock.UnSafeLock("dbGlobalRow.GetNextGuildId")
	this.m_CurrentGuildId_changed = true
	this.m_lock.UnSafeUnlock()
	return new_id
}
