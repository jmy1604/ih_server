package main

import (
	"crypto/tls"
	"ih_server/libs/log"
	"ih_server/proto/gen_go/client_message"
	"io/ioutil"
	"net"
	"net/http"
	_ "reflect"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
)

var msg_handler_http_mux map[string]func(http.ResponseWriter, *http.Request)

type MsgHttpHandle struct{}

func (this *MsgHttpHandle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var act_str, url_str string
	url_str = r.URL.String()
	idx := strings.Index(url_str, "?")
	if -1 == idx {
		act_str = url_str
	} else {
		act_str = string([]byte(url_str)[:idx])
	}
	//log.Debug("hall msg handler ServeHTTP actstr(%s)", act_str)
	if h, ok := msg_handler_http_mux[act_str]; ok {
		h(w, r)
	}

	return
}

//=======================================================

type CLIENT_MSG_HANDLER func(http.ResponseWriter, *http.Request /*proto.Message*/, []byte) (int32, *Player)

type CLIENT_PLAYER_MSG_HANDLER func(http.ResponseWriter, *http.Request, *Player /*proto.Message*/, []byte) int32

type MsgHandlerInfo struct {
	//typ                reflect.Type
	msg_handler        CLIENT_MSG_HANDLER
	player_msg_handler CLIENT_PLAYER_MSG_HANDLER
	if_player_msg      bool
}

type MsgHandlerMgr struct {
	msg_http_listener net.Listener
	login_http_server http.Server
	msgid2handler     map[int32]*MsgHandlerInfo
}

var msg_handler_mgr MsgHandlerMgr

func (this *MsgHandlerMgr) Init() bool {
	this.msgid2handler = make(map[int32]*MsgHandlerInfo)
	return true
}

func (this *MsgHandlerMgr) SetMsgHandler(msg_code uint16, msg_handler CLIENT_MSG_HANDLER) {
	//log.Info("set msg [%d] handler !", msg_code)
	this.msgid2handler[int32(msg_code)] = &MsgHandlerInfo{ /*typ: msg_client_message.MessageTypes[msg_code], */ msg_handler: msg_handler, if_player_msg: false}
}

func (this *MsgHandlerMgr) SetPlayerMsgHandler(msg_code uint16, msg_handler CLIENT_PLAYER_MSG_HANDLER) {
	//log.Info("set msg [%d] handler !", msg_code)
	this.msgid2handler[int32(msg_code)] = &MsgHandlerInfo{ /*typ: msg_client_message.MessageTypes[msg_code], */ player_msg_handler: msg_handler, if_player_msg: true}
}

func (this *MsgHandlerMgr) StartHttp() bool {
	var err error
	this.reg_http_mux()

	this.msg_http_listener, err = net.Listen("tcp", config.ListenClientInIP)
	if nil != err {
		log.Error("Center StartHttp Failed %s", err.Error())
		return false
	}

	signal_mgr.RegCloseFunc("msg_handler_mgr", this.CloseFunc)

	msg_http_server := http.Server{
		Handler:     &MsgHttpHandle{},
		ReadTimeout: 6 * time.Second,
	}

	log.Info("启动消息处理服务 IP:%s", config.ListenClientInIP)
	err = msg_http_server.Serve(this.msg_http_listener)
	if err != nil {
		log.Error("启动消息处理服务失败 %s", err.Error())
		return false
	}

	return true
}

func (this *MsgHandlerMgr) StartHttps(crt_file, key_file string) bool {
	this.reg_http_mux()

	signal_mgr.RegCloseFunc("msg_handler_mgr", this.CloseFunc)

	this.login_http_server = http.Server{
		Addr:        config.ListenClientInIP,
		Handler:     &MsgHttpHandle{},
		ReadTimeout: 6 * time.Second,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
	}

	err := this.login_http_server.ListenAndServeTLS(crt_file, key_file)
	if err != nil {
		log.Error("启动消息处理服务失败%s", err.Error())
		return false
	}

	return true
}

func (this *MsgHandlerMgr) CloseFunc(info *SignalRegRecod) {
	if nil != this.msg_http_listener {
		this.msg_http_listener.Close()
	}

	this.login_http_server.Close()

	info.close_flag = true
	return
}

//=========================================================

func (this *MsgHandlerMgr) reg_http_mux() {
	msg_handler_http_mux = make(map[string]func(http.ResponseWriter, *http.Request))
	msg_handler_http_mux["/client_msg"] = client_msg_handler
}

func client_msg_handler(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
			debug.PrintStack()
		}
	}()

	defer r.Body.Close()

	data, err := ioutil.ReadAll(r.Body)
	if nil != err {
		log.Error("client_msg_handler ReadAll err[%s]", err.Error())
		return
	}
	//log.Debug("客户端发送过来的二进制流  %v", data)

	tmp_msg := &msg_client_message.C2S_MSG_DATA{}
	err = proto.Unmarshal(data, tmp_msg)
	if nil != err {
		log.Error("client_msg_handler proto Unmarshal err[%s]", err.Error())
		return
	}

	handlerinfo := msg_handler_mgr.msgid2handler[tmp_msg.GetMsgCode()]
	if nil == handlerinfo {
		log.Error("client_msg_handler msg_handler_mgr[%d] nil ", tmp_msg.GetMsgCode())
		return
	}

	var p *Player
	var ret_code int32
	if handlerinfo.if_player_msg {
		pid := tmp_msg.GetPlayerId()

		p = player_mgr.GetPlayerById(pid)
		if nil == p {
			log.Error("client_msg_handler failed to GetPlayerById [%d]", tmp_msg.GetPlayerId())
			return
		}

		tokeninfo := login_token_mgr.GetTokenByAcc(p.Account)
		if nil == tokeninfo || tokeninfo.token != tmp_msg.GetToken() {
			ret_code = int32(msg_client_message.E_ERR_PLAYER_OTHER_PLACE_LOGIN)
			if tokeninfo == nil {
				log.Warn("Account[%v] no token info", p.Account)
			} else {
				log.Warn("Account[%v] token[%v] invalid, need[%v]", p.Account, tmp_msg.GetToken(), tokeninfo.token)
			}
		} else {
			func() {
				defer func() {
					if err := recover(); err != nil {
						atomic.StoreInt32(&p.is_lock, 0)
						log.Stack(err)
					}
				}()
				if !atomic.CompareAndSwapInt32(&p.is_lock, 0, 1) {
					log.Debug("Player[%v] send msg[%v] cant process, because prev msg is processing", p.Id, tmp_msg.GetMsgCode())
					ret_code = int32(msg_client_message.E_ERR_PLAYER_SEND_TOO_FREQUENTLY)
				} else {
					p.b_base_prop_chg = false
					p.OnInit()
					ret_code = handlerinfo.player_msg_handler(w, r, p, tmp_msg.GetData())
					data = p.PopCurMsgData()
					if USE_CONN_TIMER_WHEEL == 0 {
						conn_timer_mgr.Insert(p.Id)
					} else {
						conn_timer_wheel.Insert(p.Id)
					}
					atomic.CompareAndSwapInt32(&p.is_lock, 1, 0)
				}
			}()
		}

	} else {
		ret_code, p = handlerinfo.msg_handler(w, r /*req*/, tmp_msg.GetData())
		data = p.PopCurMsgData()
	}

	var old_msg_num, msg_num int32
	if p != nil {
		row := dbc.Players.GetRow(p.Id)
		if row != nil {
			old_msg_num = row.GetCurrReplyMsgNum()
			msg_num = old_msg_num
			if msg_num < 10000 {
				msg_num += 1
			} else {
				msg_num = 1
			}
			row.SetCurrReplyMsgNum(msg_num)
		}
	}

	if ret_code <= 0 {
		log.Error("client_msg_handler exec msg_handler ret error_code %d", ret_code)
		res2cli := &msg_client_message.S2C_MSG_DATA{}
		res2cli.ErrorCode = ret_code
		if msg_num > 0 {
			res2cli.CurrMsgNum = msg_num
		}

		final_data, err := proto.Marshal(res2cli)
		if nil != err {
			log.Error("client_msg_handler marshal 1 client msg failed err(%s)", err.Error())
			return
		}

		iret, err := w.Write(final_data)
		if nil != err {
			log.Error("client_msg_handler write data 1 failed err[%s] ret %d", err.Error(), iret)
			return
		}
		//log.Info("write http resp data error %v", final_data)
	} else {
		if nil == p {
			log.Error("client_msg_handler after handle p nil")
			return
		}

		res2cli := &msg_client_message.S2C_MSG_DATA{}

		if nil == data || len(data) < 4 {
			//log.Error("client_msg_handler PopCurMsgDataError nil or len[%d] error", len(data))
			res2cli.ErrorCode = ret_code
		} else {
			//log.Trace("client_msg_handler pop data %v", data)
			res2cli.Data = data
		}

		if msg_num > 0 {
			res2cli.CurrMsgNum = msg_num
		}

		final_data, err := proto.Marshal(res2cli)
		if nil != err {
			log.Error("client_msg_handler marshal 2 client msg failed err(%s)", err.Error())
			return
		}

		iret, err := w.Write(final_data)
		if nil != err {
			log.Error("client_msg_handler write data 2 failed err[%s] ret %d", err.Error(), iret)
			return
		}
	}
}
