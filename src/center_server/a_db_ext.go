package main

import (
	_ "ih_server/proto/gen_go/server_message"
	_ "time"
)

func (this *DBC) on_preload() (err error) {

	return
}

func (this *dbPlayerIdMaxRow) Inc() (id int32) {
	this.m_lock.Lock("dbPlayerIdMaxRow.Inc")
	defer this.m_lock.Unlock()

	this.m_PlayerIdMax++
	id = this.m_PlayerIdMax

	this.m_PlayerIdMax_changed = true

	return
}
