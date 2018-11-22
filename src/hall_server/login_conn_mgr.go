package main

import (
	"ih_server/libs/log"
	"ih_server/proto/gen_go/server_message"
	"sync"

	"github.com/golang/protobuf/proto"
)

type LoginConnectionMgr struct {
	id2loginconn      map[int32]*LoginConnection
	id2loginconn_lock *sync.RWMutex
}

var login_conn_mgr LoginConnectionMgr

func (this *LoginConnectionMgr) Init() bool {
	this.id2loginconn = make(map[int32]*LoginConnection)
	this.id2loginconn_lock = &sync.RWMutex{}
	return true
}

func (this *LoginConnectionMgr) DisconnectAll() {
	log.Info("LoginConnectionMgr DisconnectAll")

	cur_conns := this.reset()
	for _, conn := range cur_conns {
		if nil != conn {
			conn.ForceClose(true)
		}
	}

	return
}

func (this *LoginConnectionMgr) reset() []*LoginConnection {
	log.Info("LoginConnectionMgr reset")

	this.id2loginconn_lock.Lock()
	defer this.id2loginconn_lock.Unlock()
	ret_conns := make([]*LoginConnection, len(this.id2loginconn))
	for _, conn := range this.id2loginconn {
		if nil == conn {
			continue
		}

		ret_conns = append(ret_conns, conn)
	}

	this.id2loginconn = make(map[int32]*LoginConnection)
	return ret_conns
}

func (this *LoginConnectionMgr) GetLoginById(id int32) *LoginConnection {
	this.id2loginconn_lock.RLock()
	defer this.id2loginconn_lock.RUnlock()
	return this.id2loginconn[id]
}

func (this *LoginConnectionMgr) AddLogin(msg_login *msg_server_message.LoginServerInfo) {
	if nil == msg_login {
		log.Error("LoginConnectionMgr AddLogin msg_login empty")
		return
	}

	serverid := msg_login.GetServerId()

	this.id2loginconn_lock.Lock()
	defer this.id2loginconn_lock.Unlock()

	old_conn := this.id2loginconn[serverid]
	if nil != old_conn {
		//old_conn.Connect(LOGIN_CONN_STATE_FORCE_CLOSE)
		delete(this.id2loginconn, serverid)
	}

	log.Info("LoginConnectionMgr AddLogin", serverid, msg_login.GetServerName())
	new_conn := new_login_conn(serverid, msg_login.GetServerName(), msg_login.GetListenGameIP())
	if nil == new_conn {
		log.Info("LoginConnectionMgr AddLogin new login conn failed", serverid, msg_login.GetServerName(), msg_login.GetListenGameIP())
		return
	}
	this.id2loginconn[serverid] = new_conn
}

func (this *LoginConnectionMgr) RemoveLogin(id int32) {
	this.id2loginconn_lock.Lock()
	defer this.id2loginconn_lock.Unlock()
	if nil != this.id2loginconn[id] {
		delete(this.id2loginconn, id)
	}

	return
}

func (this *LoginConnectionMgr) Send(msg_id uint16, msg proto.Message) {
	this.id2loginconn_lock.Lock()
	defer this.id2loginconn_lock.Unlock()

	for _, c := range this.id2loginconn {
		if c != nil {
			c.Send(msg_id, msg)
		}
	}
}
