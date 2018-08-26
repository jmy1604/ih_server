package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"ih_server/libs/log"
	"ih_server/libs/timer"
	"ih_server/libs/utils"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"ih_server/proto/gen_go/server_message"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
)

type WaitCenterInfo struct {
	res_chan    chan *msg_server_message.C2LPlayerAccInfo
	create_time int32
}

type LoginServer struct {
	start_time         time.Time
	quit               bool
	shutdown_lock      *sync.Mutex
	shutdown_completed bool
	ticker             *timer.TickTimer
	initialized        bool

	login_http_listener net.Listener
	login_http_server   http.Server
	use_https           bool

	redis_conn *utils.RedisConn

	acc2c_wait      map[string]*WaitCenterInfo
	acc2c_wait_lock *sync.RWMutex
}

var server *LoginServer

func (this *LoginServer) Init() (ok bool) {
	this.start_time = time.Now()
	this.shutdown_lock = &sync.Mutex{}
	this.acc2c_wait = make(map[string]*WaitCenterInfo)
	this.acc2c_wait_lock = &sync.RWMutex{}
	this.redis_conn = &utils.RedisConn{}

	this.initialized = true

	return true
}

func (this *LoginServer) Start(use_https bool) bool {
	if !this.redis_conn.Connect(config.RedisServerIP) {
		return false
	}

	if use_https {
		go this.StartHttps("../src/ih_server/conf/server.crt", "../src/ih_server/conf/server.key")
	} else {
		go this.StartHttp()
	}

	this.use_https = use_https
	log.Event("服务器已启动", nil, log.Property{"IP", config.ListenClientIP})
	log.Trace("**************************************************")

	this.Run()

	return true
}

func (this *LoginServer) Run() {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}

		this.shutdown_completed = true
	}()

	this.ticker = timer.NewTickTimer(1000)
	this.ticker.Start()
	defer this.ticker.Stop()

	go this.redis_conn.Run(100)

	for {
		select {
		case d, ok := <-this.ticker.Chan:
			{
				if !ok {
					return
				}

				begin := time.Now()
				this.OnTick(d)
				time_cost := time.Now().Sub(begin).Seconds()
				if time_cost > 1 {
					log.Trace("耗时 %v", time_cost)
					if time_cost > 30 {
						log.Error("耗时 %v", time_cost)
					}
				}
			}
		}
	}
}

func (this *LoginServer) Shutdown() {
	if !this.initialized {
		return
	}

	this.shutdown_lock.Lock()
	defer this.shutdown_lock.Unlock()

	if this.quit {
		return
	}
	this.quit = true

	this.redis_conn.Close()

	log.Trace("关闭游戏主循环")

	begin := time.Now()

	if this.ticker != nil {
		this.ticker.Stop()
	}

	for {
		if this.shutdown_completed {
			break
		}

		time.Sleep(time.Millisecond * 100)
	}

	this.login_http_listener.Close()
	center_conn.ShutDown()
	hall_agent_manager.net.Shutdown()
	log.Trace("关闭游戏主循环耗时 %v 秒", time.Now().Sub(begin).Seconds())
}

func (this *LoginServer) OnTick(t timer.TickTime) {
}

func (this *LoginServer) add_to_c_wait(acc string, c_wait *WaitCenterInfo) {
	this.acc2c_wait_lock.Lock()
	defer this.acc2c_wait_lock.Unlock()

	this.acc2c_wait[acc] = c_wait
}

func (this *LoginServer) remove_c_wait(acc string) {
	this.acc2c_wait_lock.Lock()
	defer this.acc2c_wait_lock.Unlock()

	delete(this.acc2c_wait, acc)
}

func (this *LoginServer) get_c_wait_by_acc(acc string) *WaitCenterInfo {
	this.acc2c_wait_lock.RLock()
	defer this.acc2c_wait_lock.RUnlock()

	return this.acc2c_wait[acc]
}

func (this *LoginServer) pop_c_wait_by_acc(acc string) *WaitCenterInfo {
	this.acc2c_wait_lock.Lock()
	defer this.acc2c_wait_lock.Unlock()

	cur_wait := this.acc2c_wait[acc]
	if nil != cur_wait {
		delete(this.acc2c_wait, acc)
		return cur_wait
	}

	return nil
}

//=================================================================================

type LoginHttpHandle struct{}

func (this *LoginServer) StartHttp() bool {
	var err error
	this.reg_http_mux()

	this.login_http_listener, err = net.Listen("tcp", config.ListenClientIP)
	if nil != err {
		log.Error("LoginServer StartHttp Failed %s", err.Error())
		return false
	}

	login_http_server := http.Server{
		Handler:     &LoginHttpHandle{},
		ReadTimeout: 6 * time.Second,
	}

	err = login_http_server.Serve(this.login_http_listener)
	if err != nil {
		log.Error("启动Login Http Server %s", err.Error())
		return false
	}

	return true
}

func (this *LoginServer) StartHttps(crt_file, key_file string) bool {
	this.reg_http_mux()

	this.login_http_server = http.Server{
		Addr:        config.ListenClientIP,
		Handler:     &LoginHttpHandle{},
		ReadTimeout: 6 * time.Second,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
	}

	err := this.login_http_server.ListenAndServeTLS(crt_file, key_file)
	if err != nil {
		log.Error("启动https server error[%v]", err.Error())
		return false
	}

	return true
}

var login_http_mux map[string]func(http.ResponseWriter, *http.Request)

func (this *LoginServer) reg_http_mux() {
	login_http_mux = make(map[string]func(http.ResponseWriter, *http.Request))
	login_http_mux["/login"] = login_http_handler
	login_http_mux["/select_server"] = select_server_http_handler
}

func (this *LoginHttpHandle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var act_str, url_str string
	url_str = r.URL.String()
	idx := strings.Index(url_str, "?")
	if -1 == idx {
		act_str = url_str
	} else {
		act_str = string([]byte(url_str)[:idx])
	}
	log.Info("ServeHTTP actstr(%s)", act_str)
	if h, ok := login_http_mux[act_str]; ok {
		h(w, r)
	}
	return
}

type JsonRequestData struct {
	MsgId   int32  // 消息ID
	MsgData []byte // 消息体
}

type JsonResponseData struct {
	Code    int32  // 错误码
	MsgId   int32  // 消息ID
	MsgData []byte // 消息体
}

func login_handler(account, password string) (err_code int32, resp_data []byte) {
	/*if has_account_login(account) {
		err_code = int32(msg_client_message.E_ERR_PLAYER_ALREADY_LOGINED)
		log.Error("account[%v] already logined", account)
		return
	}*/

	account_login(account)

	// 验证
	token := fmt.Sprintf("%v_%v", time.Now().Unix()+time.Now().UnixNano(), account)
	acc := get_account(account)
	acc.token = token

	response := &msg_client_message.S2CLoginResponse{
		Acc:   account,
		Token: token,
	}

	if server_list.Servers == nil {
		response.Servers = make([]*msg_client_message.ServerInfo, 0)
	} else {
		l := len(server_list.Servers)
		response.Servers = make([]*msg_client_message.ServerInfo, l)
		for i := 0; i < l; i++ {
			response.Servers[i] = &msg_client_message.ServerInfo{
				Id:   server_list.Servers[i].Id,
				Name: server_list.Servers[i].Name,
				IP:   server_list.Servers[i].IP,
			}
		}
	}

	var err error
	resp_data, err = proto.Marshal(response)
	if err != nil {
		err_code = int32(msg_client_message.E_ERR_INTERNAL)
		log.Error("login_handler marshal response error: %v", err.Error())
		return
	}

	log.Debug("Account[%v] logined", account)

	return
}

func select_server_handler(account, token string, server_id int32) (err_code int32, resp_data []byte) {
	/*var msg msg_client_message.C2SSelectServerRequest
	err := proto.Unmarshal(req_data, &msg)
	if err != nil {
		err_code = int32(msg_client_message.E_ERR_INTERNAL)
		log.Error("select_server_handler unmarshal proto error: %v", err.Error())
		return
	}*/

	acc := get_account(account)
	if acc == nil {
		err_code = int32(msg_client_message.E_ERR_PLAYER_NOT_EXIST)
		log.Error("select_server_handler player[%v] not found", account)
		return
	}

	if acc.state != 1 {
		err_code = int32(msg_client_message.E_ERR_PLAYER_ALREADY_SELECTED_SERVER)
		log.Error("select_server_handler player[%v] already selected server", account)
		return
	}

	if token != acc.token {
		err_code = int32(msg_client_message.E_ERR_PLAYER_TOKEN_ERROR)
		log.Error("select_server_handler player[%v] token[%v] invalid, need[%v]", account, token, acc.token)
		return
	}

	sinfo := server_list.GetById(server_id)
	if sinfo == nil {
		err_code = int32(msg_client_message.E_ERR_PLAYER_SELECT_SERVER_NOT_FOUND)
		log.Error("select_server_handler player[%v] select server[%v] not found")
		return
	}

	hall_agent := hall_agent_manager.GetAgentByID(server_id)
	if nil == hall_agent {
		err_code = int32(msg_client_message.E_ERR_PLAYER_SELECT_SERVER_NOT_FOUND)
		log.Error("login_http_handler get hall_agent failed")
		return
	}

	token = fmt.Sprintf("%v_%v", time.Now().Unix()+time.Now().UnixNano(), account)
	req_2h := &msg_server_message.L2HSyncAccountToken{}
	req_2h.Account = account
	req_2h.Token = token
	//req_2h.PlayerId = 0
	hall_agent.Send(uint16(msg_server_message.MSGID_L2H_SYNC_ACCOUNT_TOKEN), req_2h)

	var hall_ip string
	if server.use_https {
		hall_ip = "https://" + sinfo.IP
	} else {
		hall_ip = "http://" + sinfo.IP
	}
	response := &msg_client_message.S2CSelectServerResponse{
		Acc:   account,
		Token: token,
		IP:    hall_ip,
	}

	var err error
	resp_data, err = proto.Marshal(response)
	if err != nil {
		err_code = int32(msg_client_message.E_ERR_INTERNAL)
		log.Error("select_server_handler marshal response error: %v", err.Error())
		return
	}

	return
}

func response_error(err_code int32, w http.ResponseWriter) {
	err_response := JsonResponseData{
		Code: err_code,
	}
	data, err := json.Marshal(err_response)
	if nil != err {
		log.Error("login_http_handler json mashal error")
		return
	}
	w.Write(data)
}

func login_http_handler(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
			return
		}
	}()

	// account
	account := r.URL.Query().Get("account")
	if "" == account {
		response_error(int32(msg_client_message.E_ERR_PLAYER_ACC_OR_PASSWORD_ERROR), w)
		log.Error("login_http_handler get msg_id failed")
		return
	}

	// password
	password := r.URL.Query().Get("password")
	/*if "" == password {
		response_error(int32(msg_client_message.E_ERR_PLAYER_ACC_OR_PASSWORD_ERROR), w)
		log.Error("login_http_handler msg_data is empty")
		return
	}*/

	log.Debug("account: %v, password: %v", account, password)

	var err_code int32
	var data []byte
	err_code, data = login_handler(account, password)
	log.Info("@@@@@@ data = %v", data)

	if err_code < 0 {
		response_error(err_code, w)
		log.Error("login_http_handler err_code[%v]", err_code)
		return
	}

	if data == nil {
		response_error(-1, w)
		log.Error("cant get response data failed")
		return
	}

	http_res := &JsonResponseData{Code: 0, MsgId: int32(msg_client_message_id.MSGID_S2C_LOGIN_RESPONSE), MsgData: data}
	var err error
	data, err = json.Marshal(http_res)
	if nil != err {
		log.Error("login_http_handler json mashal error")
		return
	}
	w.Write(data)

}

func select_server_http_handler(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
			return
		}
	}()

	account := r.URL.Query().Get("account")
	if "" == account {
		response_error(-1, w)
		log.Error("login_http_handler get account is empty")
		return
	}

	token := r.URL.Query().Get("token")
	if "" == token {
		response_error(-1, w)
		log.Error("login_http_handler get token is empty")
		return
	}

	server_id_str := r.URL.Query().Get("server_id")
	if "" == server_id_str {
		response_error(-1, w)
		log.Error("login_http_handler get server_id is empty")
		return
	}

	server_id, err := strconv.Atoi(server_id_str)
	if err != nil {
		response_error(-1, w)
		log.Error("login_http_handler transfer server_id[%v] error[%v]", server_id_str, err.Error())
		return
	}
	log.Debug("account: %v, token: %v, server_id: %v", account, token, server_id)

	var err_code int32
	var data []byte
	err_code, data = select_server_handler(account, token, int32(server_id))

	/*res_2c := &msg_server_message.L2CGetPlayerAccInfo{}
	res_2c.Account = account
	center_conn.Send(uint16(msg_server_message.MSGID_L2C_GET_PLAYER_ACC_INFO), res_2c)

	log.Info("login_http_handler account(%s)", account)
	new_c_wait := &WaitCenterInfo{}
	new_c_wait.res_chan = make(chan *msg_server_message.C2LPlayerAccInfo)
	new_c_wait.create_time = int32(time.Now().Unix())
	server.add_to_c_wait(account, new_c_wait)

	c2l_res, ok := <-new_c_wait.res_chan
	if !ok || nil == c2l_res {
		log.Error("login_http_handler wait chan failed", ok)
		return
	}*/

	if err_code < 0 {
		response_error(err_code, w)
		log.Error("login_http_handler err_code[%v]", err_code)
		return
	}

	if data == nil {
		response_error(-1, w)
		log.Error("cant get response data")
		return
	}

	http_res := &JsonResponseData{Code: 0, MsgId: int32(msg_client_message_id.MSGID_S2C_SELECT_SERVER_RESPONSE), MsgData: data}
	data, err = json.Marshal(http_res)
	if nil != err {
		log.Error("login_http_handler json mashal error")
		return
	}
	w.Write(data)
}

func Google_Login_Verify(token string) bool {
	if "" == token {
		log.Error("Apple_Login_verify param token(%s) empty !", token)
		return false
	}

	url_str := global_config.GoogleLoginVerifyUrl + "?id_token=" + token

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Get(url_str)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	log.Info("%v", body)
	if 200 != resp.StatusCode {
		log.Error("Apple_Login_verify token failed(%d)", resp.StatusCode)
		return false
	}

	return true
}

func Apple_Login_Verify(token string) bool {
	return true
}
