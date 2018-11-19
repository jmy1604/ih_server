package main

import (
	"ih_server/libs/log"
	"ih_server/libs/server_conn"
	"ih_server/libs/timer"
	"ih_server/proto/gen_go/server_message"
	"time"

	"github.com/golang/protobuf/proto"
)

type HallAgent struct {
	conn             *server_conn.ServerConn
	id               int32
	name             string
	listen_room_ip   string
	listen_client_ip string
}

func new_match_agent(conn *server_conn.ServerConn, id int32, name, listen_room_ip, listen_client_ip string) *HallAgent {
	if nil == conn || id < 0 {
		log.Error("NewHallAgent param error !", id)
		return nil
	}

	retagent := &HallAgent{}
	retagent.conn = conn
	retagent.id = id
	retagent.name = name
	retagent.listen_room_ip = listen_room_ip
	retagent.listen_client_ip = listen_client_ip
	return retagent
}

func (this *HallAgent) Send(msg_id uint16, msg proto.Message) {
	this.conn.Send(msg_id, msg, true)
}

var hall_agent_mgr HallAgentManager

type HallAgentManager struct {
	start_time      time.Time
	server_node     *server_conn.Node
	id2agent        map[int32]*HallAgent
	conn2agent      map[*server_conn.ServerConn]*HallAgent
	conn2agent_lock *RWMutex
	initialized     bool
}

func (this *HallAgentManager) Init() (ok bool) {
	this.start_time = time.Now()
	this.id2agent = make(map[int32]*HallAgent)
	this.conn2agent = make(map[*server_conn.ServerConn]*HallAgent)
	this.conn2agent_lock = NewRWMutex()
	this.server_node = server_conn.NewNode(this, 0, 0, 5000, 0, 0, 0, 0, 0)
	this.server_node.SetDesc("HallAgent", "大厅服务器")

	this.RegisterMsgHandler()
	this.initialized = true
	ok = true
	return
}

func (this *HallAgentManager) Start(ip string, max_conn int32) (err error) {
	err = this.server_node.Listen(ip, max_conn)
	if err != nil {
		log.Error("启动服务(%v)失败 %v", ip, err)
		return
	}
	return
}

func (this *HallAgentManager) OnTick() {
}

func (this *HallAgentManager) OnAccept(conn *server_conn.ServerConn) {
	log.Info("新的Hall连接[%v]", conn.GetAddr())
}

func (this *HallAgentManager) OnConnect(conn *server_conn.ServerConn) {

}

func (this *HallAgentManager) OnUpdate(conn *server_conn.ServerConn, t timer.TickTime) {

}

func (this *HallAgentManager) OnDisconnect(conn *server_conn.ServerConn, reason server_conn.E_DISCONNECT_REASON) {
	log.Info("断开Hall连接[%v]", conn.GetAddr())
	this.RemoveAgent(conn)
}

func (this *HallAgentManager) CloseConnection(conn *server_conn.ServerConn, reason server_conn.E_DISCONNECT_REASON) {
	if nil == conn {
		log.Error("参数为空")
		return
	}

	conn.Close(reason)
}

func (this *HallAgentManager) SendToAllMatch(msg_id uint16, msg proto.Message) {
	this.server_node.Broadcast(msg_id, msg)
}

func (this *HallAgentManager) HasAgentByConn(conn *server_conn.ServerConn) bool {
	if nil == conn {
		return false
	}
	this.conn2agent_lock.UnSafeRLock("HallAgentManager HasAgentByConn")
	defer this.conn2agent_lock.UnSafeRUnlock()
	if nil != this.conn2agent[conn] {
		return true
	}

	return false
}

func (this *HallAgentManager) AddAgent(conn *server_conn.ServerConn, id int32, name, listen_room_ip, listen_client_ip string) *HallAgent {
	new_agent := new_match_agent(conn, id, name, listen_room_ip, listen_client_ip)
	if nil == new_agent {
		log.Error("HallAgentManager AddAgent new_agent nil ", conn, id, name, listen_room_ip, listen_client_ip)
		return nil
	}

	this.conn2agent_lock.UnSafeLock("HallAgentManager AddAgent")
	defer this.conn2agent_lock.UnSafeUnlock()
	this.conn2agent[conn] = new_agent
	conn.T = id
	this.id2agent[id] = new_agent
	return new_agent
}

func (this *HallAgentManager) GetAgentById(id int32) *HallAgent {
	this.conn2agent_lock.UnSafeRLock("HallAgentMananger GetAgentById")
	defer this.conn2agent_lock.UnSafeRUnlock()

	return this.id2agent[id]
}

func (this *HallAgentManager) RemoveAgent(conn *server_conn.ServerConn) {
	this.conn2agent_lock.UnSafeLock("HallAgent RemoveAgent")
	defer this.conn2agent_lock.UnSafeUnlock()
	cur_agent := this.conn2agent[conn]
	if nil != cur_agent {
		if nil != this.id2agent[cur_agent.id] {
			delete(this.id2agent, cur_agent.id)
		}
		delete(this.conn2agent, conn)
	}
	return
}

func (this *HallAgentManager) Broadcast(msg_id uint16, msg proto.Message) {
	this.server_node.Broadcast(msg_id, msg)
}

func (this *HallAgentManager) RandOneAgent() *HallAgent {
	this.conn2agent_lock.UnSafeLock("HallAgent RemoveAgent")
	defer this.conn2agent_lock.UnSafeUnlock()
	for _, hall := range this.id2agent {
		return hall
	}

	return nil
}

//==========================================================================================================

func (this *HallAgentManager) RegisterMsgHandler() {
	this.server_node.SetPid2P(hall_agent_msgid2msg)
	this.SetMessageHandler(uint16(msg_server_message.MSGID_H2C_HAll_SERVER_REGISTER), H2CHallServerRegisterHandler)
}

func (this *HallAgentManager) SetMessageHandler(type_id uint16, h server_conn.Handler) {
	this.server_node.SetHandler(type_id, h)
}

func hall_agent_msgid2msg(msg_id uint16) proto.Message {
	if msg_id == uint16(msg_server_message.MSGID_H2C_HAll_SERVER_REGISTER) {
		return &msg_server_message.H2CHallServerRegister{}
	} else {
		log.Error("Cant found proto message by msg_id[%v]", msg_id)
	}
	return nil
}

func H2CHallServerRegisterHandler(conn *server_conn.ServerConn, m proto.Message) {
	req := m.(*msg_server_message.H2CHallServerRegister)
	if nil == conn || nil == req {
		log.Error("H2CHallServerRegisterHandler param error !")
		return
	}

	cur_agent := hall_agent_mgr.GetAgentById(req.GetServerId())
	if nil != cur_agent {
		conn.Close(server_conn.E_DISCONNECT_REASON_FORCE_CLOSED)
		log.Error("H2MHallServerRegisterHandler Server Id [%v] Already Registered, Check server config file !!!!!!!!!!!!!!! ", req.GetServerId())
		return
	}

	new_agent := hall_agent_mgr.AddAgent(conn, req.GetServerId(), req.GetServerName(), req.GetListenRoomIP(), req.GetListenClientIP())
	log.Info("M2C New HallServer(Id:%d Name:%s) Register", req.GetServerId(), req.GetServerName())

	if nil == new_agent {
		log.Error("H2CHallServerRegisterHandler agent nil ")
		return
	}

	res := &msg_server_message.C2HLoginServerList{}
	res.ServerList = login_info_mgr.GetInfoList()
	if len(res.ServerList) > 0 {
		conn.Send(uint16(msg_server_message.MSGID_C2H_LOGIN_SERVER_LIST), res, true)
	}

	return
}
