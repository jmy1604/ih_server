package main

import (
	"ih_server/libs/log"
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
		player_mgr.Add2AccMap(p)

		friend_recommend_mgr.CheckAndAddPlayer(p.Id)

		//login_token_mgr.AddToAcc2Token(p.Account, p.Token, p.Id)
		//login_token_mgr.AddToId2Acc(p.Id, p.Account)
	}

	return
}

func (this *dbGlobalRow) GetNextPlayerId() int32 {
	this.m_lock.UnSafeLock("dbGlobalRow.SetdbGlobalCurrentPlayerIdColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_CurrentPlayerId += 1
	new_id := ((config.ServerId << 24) & 0x7f000000) | this.m_CurrentPlayerId
	this.m_CurrentPlayerId_changed = true
	return new_id
}
