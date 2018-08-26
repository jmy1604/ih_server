package main

import (
	"ih_server/libs/log"
	"ih_server/libs/server_conn"
	"ih_server/libs/timer"
	"ih_server/proto/gen_go/server_message"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
	_ "ih_server/third_party/code.google.com.protobuf/proto"
)

const (
	CENTER_CONN_STATE_DISCONNECT  = 0
	CENTER_CONN_STATE_CONNECTED   = 1
	CENTER_CONN_STATE_FORCE_CLOSE = 2
)

type CenterConnection struct {
	client_node    *server_conn.Node
	state          int32
	last_conn_time int32

	connect_finished    bool
	connect_finish_chan chan int32
}

var center_conn CenterConnection

func (this *CenterConnection) Init() {
	this.client_node = server_conn.NewNode(this, 0, 0, 100, 0, 0, 0, 0, 0)
	this.client_node.SetDesc("中心服务器", "")
	this.state = CENTER_CONN_STATE_DISCONNECT
	this.RegisterMsgHandler()
	this.connect_finished = false
	this.connect_finish_chan = make(chan int32, 2)
}

func (this *CenterConnection) Start() {
	if this.Connect(CENTER_CONN_STATE_DISCONNECT) {
		log.Event("连接中心服务器成功", nil, log.Property{"IP", config.CenterServerIP})
	}
	for {
		state := atomic.LoadInt32(&this.state)
		if state == CENTER_CONN_STATE_CONNECTED {
			time.Sleep(time.Second * 2)
			continue
		}

		if state == CENTER_CONN_STATE_FORCE_CLOSE {
			this.client_node.ClientDisconnect()
			log.Event("与中心服务器的连接被强制关闭", nil)
			break
		}
		if this.Connect(state) {
			log.Event("连接中心服务器成功", nil, log.Property{"IP", config.CenterServerIP})
		}
	}
}

func (this *CenterConnection) Connect(state int32) (ok bool) {
	if CENTER_CONN_STATE_DISCONNECT == state {
		var err error
		for {
			log.Trace("连接中心服务器 %v", config.CenterServerIP)
			err = this.client_node.ClientConnect(config.CenterServerIP, time.Second*10)
			if nil == err {
				break
			}

			// 每隔30秒输出一次连接信息
			now := time.Now().Unix()
			if int32(now)-this.last_conn_time >= 30 {
				log.Trace("中心服务器连接中...")
				this.last_conn_time = int32(now)
			}
			time.Sleep(time.Second * 5)

			if signal_mgr.IfClosing() {
				this.state = CENTER_CONN_STATE_FORCE_CLOSE
				return
			}
		}
	}

	if atomic.CompareAndSwapInt32(&this.state, state, CENTER_CONN_STATE_CONNECTED) {
		go this.client_node.ClientRun()
		ok = true
	}
	return
}

func (this *CenterConnection) OnAccept(c *server_conn.ServerConn) {
	log.Error("Impossible accept")
}

func (this *CenterConnection) OnConnect(c *server_conn.ServerConn) {
	log.Trace("CenterServer [%v][%v] on CenterServer connect", config.ServerId, config.ServerName)

	notify := &msg_server_message.H2CHallServerRegister{}
	notify.ServerId = config.ServerId
	notify.ServerName = config.ServerName
	notify.ListenClientIP = config.ListenClientOutIP
	notify.ListenRoomIP = config.ListenRoomServerIP
	c.Send(uint16(msg_server_message.MSGID_H2C_HAll_SERVER_REGISTER), notify, true)
}

func (this *CenterConnection) WaitConnectFinished() {
	for {

		if this.connect_finished {
			break
		}

		time.Sleep(time.Microsecond * 50)
	}

}

func (this *CenterConnection) OnUpdate(c *server_conn.ServerConn, t timer.TickTime) {

}

func (this *CenterConnection) OnDisconnect(c *server_conn.ServerConn, reason server_conn.E_DISCONNECT_REASON) {
	if reason == server_conn.E_DISCONNECT_REASON_FORCE_CLOSED {
		this.state = CENTER_CONN_STATE_FORCE_CLOSE
	} else {
		this.state = CENTER_CONN_STATE_DISCONNECT
	}
	log.Event("与中心服务器连接断开", nil)
}

func (this *CenterConnection) SetMessageHandler(type_id uint16, h server_conn.Handler) {
	this.client_node.SetHandler(type_id, h)
}

func (this *CenterConnection) Send(msg_id uint16, msg proto.Message) {
	if CENTER_CONN_STATE_CONNECTED != this.state {
		log.Info("中心服务器未连接!!!")
		return
	}
	if nil == this.client_node {
		return
	}
	this.client_node.GetClient().Send(msg_id, msg, true)
}

//========================================================================

func (this *CenterConnection) RegisterMsgHandler() {
	this.client_node.SetPid2P(center_conn_msgid2msg)
	this.SetMessageHandler(uint16(msg_server_message.MSGID_C2H_LOGIN_SERVER_LIST), C2HLoginServerListHandler)
	this.SetMessageHandler(uint16(msg_server_message.MSGID_C2H_NEW_LOGIN_SERVER_ADD), C2HNewLoginServerAddHandler)
	this.SetMessageHandler(uint16(msg_server_message.MSGID_C2H_LOGIN_SERVER_REMOVE), C2HLoginServerRemoveHandler)
}

func center_conn_msgid2msg(msg_id uint16) proto.Message {
	if msg_id == uint16(msg_server_message.MSGID_C2H_LOGIN_SERVER_LIST) {
		return &msg_server_message.C2HLoginServerList{}
	} else if msg_id == uint16(msg_server_message.MSGID_C2H_NEW_LOGIN_SERVER_ADD) {
		return &msg_server_message.C2HNewLoginServerAdd{}
	} else if msg_id == uint16(msg_server_message.MSGID_C2H_LOGIN_SERVER_REMOVE) {
		return &msg_server_message.C2HLoginServerRemove{}
	} else {
		log.Error("Cant found proto message by msg_id[%v]", msg_id)
	}
	return nil
}

func C2HLoginServerListHandler(conn *server_conn.ServerConn, msg proto.Message) {
	req := msg.(*msg_server_message.C2HLoginServerList)
	if nil == conn || nil == req {
		log.Error("C2HLoginServerListHandler param error !")
		return
	}

	log.Info("中心服务器同步 登录服务器列表", req.GetServerList())

	login_conn_mgr.DisconnectAll()
	for _, info := range req.GetServerList() {
		login_conn_mgr.AddLogin(info)
	}

	center_conn.connect_finished = true
}

func C2HNewLoginServerAddHandler(conn *server_conn.ServerConn, msg proto.Message) {
	req := msg.(*msg_server_message.C2HNewLoginServerAdd)
	if nil == conn || nil == req || nil == req.GetServer() {
		log.Error("C2HNewLoginServerAddHandler param error !")
		return
	}

	cur_login := login_conn_mgr.GetLoginById(req.GetServer().GetServerId())
	if nil != cur_login {
		cur_login.ForceClose(true)
	}

	login_conn_mgr.AddLogin(req.GetServer())

	center_conn.connect_finished = true
}

func C2HLoginServerRemoveHandler(conn *server_conn.ServerConn, msg proto.Message) {
	req := msg.(*msg_server_message.C2HLoginServerRemove)
	if nil == conn || nil == req {
		log.Error("C2HLoginServerRemoveHandler param error !")
		return
	}

	serverid := req.GetServerId()
	cur_login := login_conn_mgr.GetLoginById(serverid)
	if nil != cur_login {
		log.Info("C2HLoginServerRemoveHandler 登录服务器[%d]连接还在，断开连接", serverid)
		cur_login.ForceClose(true)
		login_conn_mgr.RemoveLogin(serverid)
	}

	log.Info("中心服务器通知 LoginServer[%d] 断开", serverid)

	return
}
