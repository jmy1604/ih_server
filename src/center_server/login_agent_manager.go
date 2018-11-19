package main

import (
	"ih_server/libs/log"
	"ih_server/libs/server_conn"
	"ih_server/libs/timer"
	"ih_server/proto/gen_go/server_message"
	"time"

	"github.com/golang/protobuf/proto"
)

type LoginAgentManager struct {
	start_time  time.Time
	server_node *server_conn.Node
	initialized bool
}

var login_agent_mgr LoginAgentManager

func (this *LoginAgentManager) Init() (ok bool) {
	this.start_time = time.Now()
	this.server_node = server_conn.NewNode(this, 0, 0, 5000, 0, 0, 0, 0, 0)
	this.server_node.SetDesc("LoginAgent", "登录服务器")
	this.RegMsgHandler()

	this.initialized = true
	ok = true
	return
}

func (this *LoginAgentManager) Start(ip string, max_conn int32) (err error) {
	err = this.server_node.Listen(ip, max_conn)
	if err != nil {
		log.Error("启动服务(%v)失败 %v", ip, err)
		return
	}
	return
}

func (this *LoginAgentManager) OnTick(t timer.TickTime) {
}

func (this *LoginAgentManager) OnAccept(c *server_conn.ServerConn) {
	log.Trace("新的连接[%v]", c.GetAddr())
}

func (this *LoginAgentManager) OnConnect(c *server_conn.ServerConn) {

}

func (this *LoginAgentManager) OnUpdate(c *server_conn.ServerConn, t timer.TickTime) {

}

func (this *LoginAgentManager) OnDisconnect(c *server_conn.ServerConn, reason server_conn.E_DISCONNECT_REASON) {
	login_info_mgr.RemoveByConn(c)
	if c.T > 0 {
		login_info_mgr.Remove(c.T)
		rm_notify := &msg_server_message.C2HLoginServerRemove{}
		rm_notify.ServerId = c.T
		hall_agent_mgr.Broadcast(uint16(msg_server_message.MSGID_C2H_LOGIN_SERVER_REMOVE), rm_notify)
	}

	log.Trace("登录服务器[%d]断开连接[%v]", c.T, c.GetAddr())
}

func (this *LoginAgentManager) CloseConnection(c *server_conn.ServerConn, reason server_conn.E_DISCONNECT_REASON) {
	if c == nil {
		log.Error("参数为空")
		return
	}

	c.Close(reason)
}

//==================================================================================================
//type MessageHandler func(conn *server_conn.ServerConn, m proto.Message)

func (this *LoginAgentManager) RegMsgHandler() {
	this.server_node.SetPid2P(login_agent_msgid2msg)
	this.SetMessageHandler(uint16(msg_server_message.MSGID_L2C_LOGIN_SERVER_REGISTER), L2CLoginServerRegisterHandler)
	this.SetMessageHandler(uint16(msg_server_message.MSGID_L2C_GET_PLAYER_ACC_INFO), L2CGetPlayerAccInfoHandler)
}

func (this *LoginAgentManager) SetMessageHandler(type_id uint16, h server_conn.Handler) {
	this.server_node.SetHandler(type_id, h)
}

func login_agent_msgid2msg(msg_id uint16) proto.Message {
	if msg_id == uint16(msg_server_message.MSGID_L2C_LOGIN_SERVER_REGISTER) {
		return &msg_server_message.L2CLoginServerRegister{}
	} else if msg_id == uint16(msg_server_message.MSGID_L2C_GET_PLAYER_ACC_INFO) {
		return &msg_server_message.L2CGetPlayerAccInfo{}
	} else {
		log.Error("Cant found proto message for msg_id[%v]", msg_id)
	}
	return nil
}

func L2CLoginServerRegisterHandler(conn *server_conn.ServerConn, msg proto.Message) {
	req := msg.(*msg_server_message.L2CLoginServerRegister)
	if nil == conn || nil == req {
		log.Error("L2CLoginServerRegisterHandler param error !")
		return
	}

	login_serverid := req.GetServerId()
	old_login := login_info_mgr.Get(login_serverid)
	if nil != old_login {
		log.Error("L2CLoginServerRegisterHandler serverid(%d) conflict old_name(%s)!", login_serverid, old_login.Name)
		login_info_mgr.Remove(login_serverid)
		rm_notify := &msg_server_message.C2HLoginServerRemove{}
		rm_notify.ServerId = login_serverid
		hall_agent_mgr.Broadcast(uint16(msg_server_message.MSGID_C2H_LOGIN_SERVER_REMOVE), rm_notify)
	}

	add_notify := &msg_server_message.C2HNewLoginServerAdd{}
	add_notify.Server = &msg_server_message.LoginServerInfo{}
	add_notify.Server.ServerId = login_serverid
	add_notify.Server.ServerName = req.GetServerName()
	add_notify.Server.ListenGameIP = req.GetListenGameIP()
	add_notify.Server.ListenClientIP = req.GetListenClientIP()

	hall_agent_mgr.Broadcast(uint16(msg_server_message.MSGID_C2H_NEW_LOGIN_SERVER_ADD), add_notify)
	login_info_mgr.Add(conn, login_serverid, req.GetServerName(), req.GetListenGameIP())
}

func L2CGetPlayerAccInfoHandler(conn *server_conn.ServerConn, m proto.Message) {
	/*req := m.(*msg_server_message.L2CGetPlayerAccInfo)
	if nil == conn || nil == req {
		log.Error("L2CGetPlayerAccInfoHandler param error !")
		return
	}

	acc := req.GetAccount()
	res := &msg_server_message.C2LPlayerAccInfo{}
	res.Account = acc

	player_id := dbc_account.AccountsMgr.TryGetAccountPid(req.GetAccount())

	if -1 == player_id {
		log.Error("L2CGetPlayerAccInfoHandler failed to TryGetAccountPid [%s]", req.GetAccount())
		return
	}

	// 检查玩家是否被封
	forbid_l_db := dbc.ForbidLogins.GetRow(player_id)
	if nil != forbid_l_db && forbid_l_db.GetEndUnix() > int32(time.Now().Unix()) {
		end_t := time.Unix(int64(forbid_l_db.GetEndUnix()), 0)
		res.IfForbidLogin = 1
		res.ForbidEndTime = end_t.Format("2006-01-02 15:04:05.999999999")
	}*/

	/*hall_cfg := hall_group_mgr.GetHallCfgByPlayerId(player_id)
	if nil == hall_cfg {
		log.Trace("L2CGetPlayerAccInfoHandler gethall by player id failed !")
		return
	}
	res.PlayerId = int64(player_id)
	res.HallId = hall_cfg.ServerId
	res.HallIP = hall_cfg.ServerIP
	conn.Send(uint16(msg_server_message.MSGID_C2L_PLAYER_ACC_INFO), res, true)*/

	return
}
