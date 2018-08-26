package main

import (
	_ "ih_server/libs/log"
	_ "ih_server/libs/server_conn"
	_ "ih_server/proto/gen_go/server_message"
	"sync"
)

type PlayerManager struct {
	id2online_lock *sync.RWMutex
	id2online      map[int32]int32
}

var player_mgr PlayerManager

func (this *PlayerManager) Init() bool {
	this.id2online_lock = &sync.RWMutex{}
	this.id2online = make(map[int32]int32)

	//this.RegPlayerMsgHandler()

	return true
}

func (this *PlayerManager) SetOnOffline(pid, ifonline int32) {
	this.id2online_lock.Lock()
	defer this.id2online_lock.Unlock()
	if 1 == ifonline {
		this.id2online[pid] = 1
	} else {
		if 1 == this.id2online[pid] {
			delete(this.id2online, pid)
		}
	}
}

func (this *PlayerManager) GetOnlines(pids []int32) []int32 {
	tmp_len := int32(len(pids))
	ret_onlines := make([]int32, 0, tmp_len)
	this.id2online_lock.RLock()
	defer this.id2online_lock.RUnlock()
	for _, pid := range pids {
		if 1 == this.id2online[pid] {
			ret_onlines = append(ret_onlines, pid)
		}
	}

	return ret_onlines
}

/*
func (this *PlayerManager) RegPlayerMsgHandler() {
	hall_agent_mgr.SetMessageHandler(msg_server_message.ID_GetPlayerInfo, H2CGetPlayerInfoHandler)
	hall_agent_mgr.SetMessageHandler(msg_server_message.ID_RetPlayerInfo, H2CRetPlayerInfoHandler)
	hall_agent_mgr.SetMessageHandler(msg_server_message.ID_SetPlayerOnOffline, H2CSetPlayerOnOfflineHandler)
}

func H2CGetPlayerInfoHandler(c *server_conn.ServerConn, msg proto.Message) {
	req := msg.(*msg_server_message.GetPlayerInfo)
	if nil == c || nil == req {
		log.Error("H2CGetPlayerInfoHandler c or req nil [%v]", nil == req)
		return
	}

	tgt_pid := req.GetTgtPlayerId()
	hall_svrinfo := hall_group_mgr.GetHallCfgByPlayerId(tgt_pid)
	if nil == hall_svrinfo {
		log.Error("H2CGetPlayerInfoHandler failed to get hall_svrinfo[%d]", tgt_pid)
		return
	}

	hall_svr := hall_agent_mgr.GetAgentById(hall_svrinfo.ServerId)
	if nil == hall_svr {
		log.Error("H2CGetPlayerInfoHandler failed to get hall_svr [%d]", hall_svrinfo.ServerId)
		return
	}

	hall_svr.Send(req)
	return
}

func H2CRetPlayerInfoHandler(c *server_conn.ServerConn, msg proto.Message) {
	req := msg.(*msg_server_message.RetPlayerInfo)
	if nil == c || nil == req {
		log.Error("	H2CRetPlayerInfoHandler c or req nil [%v]", nil == req)
		return
	}

	pid := req.GetPlayerId()
	hall_svrinfo := hall_group_mgr.GetHallCfgByPlayerId(pid)
	if nil == hall_svrinfo {
		log.Error("H2CRetPlayerInfoHandler failed to find hall_svrinfo[%d]", pid)
		return
	}

	hall_svr := hall_agent_mgr.GetAgentById(hall_svrinfo.ServerId)
	if nil == hall_svr {
		log.Error("H2CRetPlayerInfoHandler failed to find hall_svr", hall_svrinfo.ServerId)
		return
	}

	hall_svr.Send(req)
}

func H2CSetPlayerOnOfflineHandler(c *server_conn.ServerConn, msg proto.Message) {
	req := msg.(*msg_server_message.SetPlayerOnOffline)
	if nil == req || nil == c {
		log.Error("TongMgr H2CSetPlayerOnOfflineHandler req or c nil[%v]", nil == c)
		return
	}

	pid := req.GetPlayerId()
	player_mgr.SetOnOffline(pid, req.GetOnOffLine())

	return
}
*/
