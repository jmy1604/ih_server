package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"ih_server/libs/log"
	"ih_server/libs/timer"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
)

type TestClient struct {
	start_time         time.Time
	quit               bool
	shutdown_lock      *sync.Mutex
	shutdown_completed bool
	ticker             *timer.TickTimer
	initialized        bool
	last_heartbeat     int32
	cmd_chan           chan *msg_client_message.C2S_TEST_COMMAND
}

var test_client TestClient

func (this *TestClient) Init() (ok bool) {
	this.start_time = time.Now()
	this.shutdown_lock = &sync.Mutex{}
	this.cmd_chan = make(chan *msg_client_message.C2S_TEST_COMMAND)
	this.initialized = true

	return true
}

func (this *TestClient) Start() (err error) {

	log.Event("客户端已启动", nil)
	log.Trace("**************************************************")

	this.Run()

	return
}

func (this *TestClient) Run() {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}

		this.shutdown_completed = true
	}()

	this.ticker = timer.NewTickTimer(1000)
	this.ticker.Start()
	defer this.ticker.Stop()

	go this.SendCmd()

	for {
		select {
		case d, ok := <-this.ticker.Chan:
			{
				if !ok {
					return
				}

				this.OnTick(d)
			}
		}
	}

	/*var t timer.TickTime
	for {
		this.OnTick(t)
		time.Sleep(time.Millisecond * 1)
	}*/
}

func (this *TestClient) Shutdown() {
	if !this.initialized {
		return
	}

	this.shutdown_lock.Lock()
	defer this.shutdown_lock.Unlock()

	if this.quit {
		return
	}
	this.quit = true

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

	log.Trace("关闭游戏主循环耗时 %v 秒", time.Now().Sub(begin).Seconds())
}

const (
	CMD_TYPE_LOGIN = 1 // 登录命令
)

type JsonRequestData struct {
	MsgId   int32  // 消息ID
	MsgData []byte // 消息体
}

type JsonResponseData struct {
	Code    int32  // 错误码
	MsgId   int32  // 消息ID
	MsgData []byte // 消息体
}

var cur_hall_conn *HallConnection

func get_res(url string) []byte {
	return nil
}

func register_func(account, password string, is_guest int32) {
	url_str := fmt.Sprintf(config.RegisterUrl, config.LoginServerIP, account, password, is_guest)
	log.Debug("Register Url %s,  account %s,  password %s,  is_guest %s", url_str, account, password, is_guest)

	var resp *http.Response
	var err error
	if config.UseHttps {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: tr}
		resp, err = client.Get(url_str)
	} else {
		resp, err = http.Get(url_str)
	}
	if nil != err {
		log.Error("register http get err (%s)", err.Error())
		return
	}

	var data []byte
	data, err = ioutil.ReadAll(resp.Body)
	if nil != err {
		log.Error("register ioutil readall failed err(%s) !", err.Error())
		return
	}

	log.Debug("register result data: %v", data)

	res := &JsonResponseData{}
	err = json.Unmarshal(data, res)
	if nil != err {
		log.Error("login ummarshal failed err(%s)", err.Error())
		return
	}

	if res.Code < 0 {
		log.Warn("return error_code[%v]", res.Code)
		return
	}

	if res.MsgId != int32(msg_client_message_id.MSGID_S2C_REGISTER_RESPONSE) {
		log.Warn("returned msg_id[%v] is not correct")
		return
	}

	var msg msg_client_message.S2CRegisterResponse
	err = proto.Unmarshal(res.MsgData, &msg)
	if err != nil {
		log.Error("unmarshal error[%v]", err.Error())
		return
	}

	log.Debug("Account[%v] registered, password is %v", account, password)
}

func bind_new_account_func(account, password, new_account, new_password string) {
	url_str := fmt.Sprintf(config.BindNewAccountUrl, config.LoginServerIP, account, password, new_account, new_password)
	log.Debug("Bind New Account Url %s,  account %s,  password %s,  new_account %s,  new_password %s", url_str, account, password, new_account, new_password)

	var resp *http.Response
	var err error
	if config.UseHttps {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: tr}
		resp, err = client.Get(url_str)
	} else {
		resp, err = http.Get(url_str)
	}
	if nil != err {
		log.Error("register http get err (%s)", err.Error())
		return
	}

	var data []byte
	data, err = ioutil.ReadAll(resp.Body)
	if nil != err {
		log.Error("register ioutil readall failed err(%s) !", err.Error())
		return
	}

	log.Debug("bind result data: %v", data)

	res := &JsonResponseData{}
	err = json.Unmarshal(data, res)
	if nil != err {
		log.Error("login ummarshal failed err(%s)", err.Error())
		return
	}

	if res.Code < 0 {
		log.Warn("return error_code[%v]", res.Code)
		return
	}

	if res.MsgId != int32(msg_client_message_id.MSGID_S2C_GUEST_BIND_NEW_ACCOUNT_RESPONSE) {
		log.Warn("returned msg_id[%v] is not correct")
		return
	}

	var msg msg_client_message.S2CGuestBindNewAccountResponse
	err = proto.Unmarshal(res.MsgData, &msg)
	if err != nil {
		log.Error("unmarshal error[%v]", err.Error())
		return
	}

	log.Debug("Account[%v] bind new account %v, password is %v", msg.Account, msg.NewAccount, msg.NewPassword)
}

func login_func(account, password, channel string) {
	url_str := fmt.Sprintf(config.LoginUrl, config.LoginServerIP, account, password, channel)
	log.Debug("login Url str %s", url_str)

	var resp *http.Response
	var err error
	if config.UseHttps {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: tr}
		resp, err = client.Get(url_str)
	} else {
		resp, err = http.Get(url_str)
	}
	if nil != err {
		log.Error("login http get err (%s)", err.Error())
		return
	}

	data, err := ioutil.ReadAll(resp.Body)
	if nil != err {
		log.Error("login ioutil readall failed err(%s) !", err.Error())
		return
	}

	log.Debug("login result data: %v", data)

	res := &JsonResponseData{}
	err = json.Unmarshal(data, res)
	if nil != err {
		log.Error("login ummarshal failed err(%s)", err.Error())
		return
	}

	if res.Code < 0 {
		log.Warn("return error_code[%v]", res.Code)
		return
	}

	if res.MsgId != int32(msg_client_message_id.MSGID_S2C_LOGIN_RESPONSE) {
		log.Warn("returned msg_id[%v] is not correct")
		return
	}

	var msg msg_client_message.S2CLoginResponse
	err = proto.Unmarshal(res.MsgData, &msg)
	if err != nil {
		log.Error("unmarshal error[%v]", err.Error())
		return
	}

	if len(msg.GetServers()) == 0 {
		log.Warn("no servers in server list")
		return
	}

	select_server_func(account, msg.GetToken(), msg.GetServers()[0].GetId())
}

func select_server_func(account string, token string, server_id int32) {
	url_str := fmt.Sprintf(config.SelectServerUrl, config.LoginServerIP, account, token, server_id)
	log.Debug("select server Url str %s", url_str)

	var resp *http.Response
	var err error
	if config.UseHttps {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: tr}
		resp, err = client.Get(url_str)
	} else {
		resp, err = http.Get(url_str)
	}
	if nil != err {
		log.Error("login http get err (%s)", err.Error())
		return
	}

	data, err := ioutil.ReadAll(resp.Body)
	if nil != err {
		log.Error("login ioutil readall failed err(%s) !", err.Error())
		return
	}

	res := &JsonResponseData{}
	err = json.Unmarshal(data, res)
	if nil != err {
		log.Error("login ummarshal failed err(%s)", err.Error())
		return
	}

	if res.Code < 0 {
		log.Warn("return error_code[%v]", res.Code)
		return
	}

	if res.MsgId != int32(msg_client_message_id.MSGID_S2C_SELECT_SERVER_RESPONSE) {
		log.Warn("returned msg_id[%v] is not correct")
		return
	}

	var msg msg_client_message.S2CSelectServerResponse
	err = proto.Unmarshal(res.MsgData, &msg)
	if err != nil {
		log.Error("unmarshal error[%v]", err.Error())
		return
	}

	cur_hall_conn := new_hall_connect(msg.GetIP(), account, msg.GetToken(), config.UseHttps)
	hall_conn_mgr.AddHallConn(cur_hall_conn)
	req2s := &msg_client_message.C2SEnterGameRequest{}
	req2s.Acc = account
	req2s.Token = msg.GetToken()
	cur_hall_conn.Send(uint16(msg_client_message_id.MSGID_C2S_ENTER_GAME_REQUEST), req2s)
}

func set_password_func(account, password, new_password string) {
	url_str := fmt.Sprintf(config.SetPasswordUrl, config.LoginServerIP, account, password, new_password)
	log.Debug("set password Url str %s", url_str)

	var resp *http.Response
	var err error
	if config.UseHttps {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: tr}
		resp, err = client.Get(url_str)
	} else {
		resp, err = http.Get(url_str)
	}
	if nil != err {
		log.Error("set password http get err (%s)", err.Error())
		return
	}

	data, err := ioutil.ReadAll(resp.Body)
	if nil != err {
		log.Error("set password ioutil readall failed err(%s) !", err.Error())
		return
	}

	res := &JsonResponseData{}
	err = json.Unmarshal(data, res)
	if nil != err {
		log.Error("set password ummarshal failed err(%s)", err.Error())
		return
	}

	if res.Code < 0 {
		log.Warn("return error_code[%v]", res.Code)
		return
	}

	if res.MsgId != int32(msg_client_message_id.MSGID_S2C_SET_LOGIN_PASSWORD_RESPONSE) {
		log.Warn("returned msg_id[%v] is not correct")
		return
	}

	var msg msg_client_message.S2CSetLoginPasswordResponse
	err = proto.Unmarshal(res.MsgData, &msg)
	if err != nil {
		log.Error("unmarshal error[%v]", err.Error())
		return
	}

	log.Debug("Account[%v] set password[%v] replace old password[%v]", account, new_password, password)
}

func (this *TestClient) cmd_register(use_https bool) {
	fmt.Printf("请输入账号: ")
	var acc, pwd, is_guest string
	fmt.Scanf("%s\n", &acc)
	fmt.Printf("请输入密码: ")
	fmt.Scanf("%s\n", &pwd)
	fmt.Printf("是否游客: y/n? ")
	fmt.Scanf("%s\n", &is_guest)

	var ig int32
	if is_guest == "y" || is_guest == "Y" || is_guest == "" {
		ig = 1
	}

	if config.AccountNum == 0 {
		config.AccountNum = 1
	}

	for i := int32(0); i < config.AccountNum; i++ {
		account := acc
		if config.AccountNum > 1 {
			account = fmt.Sprintf("%v_%s", acc, i)
		}

		register_func(acc, pwd, ig)

		if config.AccountNum > 1 {
			log.Debug("Account[%v] registered, total count %v", account, i+1)
		}
	}
}

func (this *TestClient) cmd_bind_new_account(use_https bool) {
	var account, password, new_account, new_password string
	fmt.Printf("输入旧帐号: ")
	fmt.Scanf("%s\n", &account)
	fmt.Printf("输入旧密码: ")
	fmt.Scanf("%s\n", &password)
	fmt.Printf("输入新账号: ")
	fmt.Scanf("%s\n", &new_account)
	fmt.Printf("输入新密码: ")
	fmt.Scanf("%s\n", &new_password)

	for i := int32(0); i < config.AccountNum; i++ {
		acc := account
		if config.AccountNum > 1 {
			acc = fmt.Sprintf("%v_%s", acc, i)
		}
		bind_new_account_func(account, password, new_account, new_password)
		if config.AccountNum > 1 {
			log.Debug("Account[%v] bind new account %v, total count %v", account, new_account, i+1)
		}
	}
}

func (this *TestClient) cmd_login(use_https bool) {
	var acc, pwd, chl string
	fmt.Printf("请输入账号: ")
	fmt.Scanf("%s\n", &acc)
	fmt.Printf("请输入密码: ")
	fmt.Scanf("%s\n", &pwd)
	fmt.Printf("请输入渠道: ")
	fmt.Scanf("%s\n", &chl)
	cur_hall_conn = hall_conn_mgr.GetHallConnByAcc(acc)
	if nil != cur_hall_conn && cur_hall_conn.blogin {
		log.Info("[%s] already login", acc)
		return
	}

	if config.AccountNum == 0 {
		config.AccountNum = 1
	}
	for i := int32(0); i < config.AccountNum; i++ {
		account := acc
		if config.AccountNum > 1 {
			account = fmt.Sprintf("%s_%v", acc, i)
		}

		login_func(account, pwd, chl)

		if config.AccountNum > 1 {
			log.Debug("Account[%v] logined, total count[%v]", account, i+1)
		}
	}
}

func (this *TestClient) cmd_set_password(use_https bool) {
	var acc, pwd, new_pwd string
	fmt.Printf("请输入账号: ")
	fmt.Scanf("%s\n", &acc)
	fmt.Printf("请输入密码: ")
	fmt.Scanf("%s\n", &pwd)
	fmt.Printf("请输入新密码: ")
	fmt.Scanf("%s\n", &new_pwd)
	cur_hall_conn = hall_conn_mgr.GetHallConnByAcc(acc)
	if nil != cur_hall_conn && cur_hall_conn.blogin {
		log.Info("[%s] already login", acc)
		return
	}

	if config.AccountNum == 0 {
		config.AccountNum = 1
	}
	for i := int32(0); i < config.AccountNum; i++ {
		account := acc
		if config.AccountNum > 1 {
			account = fmt.Sprintf("%s_%v", acc, i)
		}

		set_password_func(account, pwd, new_pwd)

		if config.AccountNum > 1 {
			log.Debug("Account[%v] set password, total count[%v]", account, i+1)
		}
	}
}

var is_test bool

func (this *TestClient) OnTick(t timer.TickTime) {
	if !is_test {
		fmt.Printf("请输入命令:\n")
		var cmd_str string
		fmt.Scanf("%s\n", &cmd_str)
		switch cmd_str {
		case "register":
			{
				this.cmd_register(true)
			}
		case "bind_new_account":
			{
				this.cmd_bind_new_account(true)
			}
		case "login":
			{
				this.cmd_login(true)
				is_test = true
			}
		case "set_password":
			{
				this.cmd_set_password(true)
			}
		case "enter_test":
			{
				is_test = true
			}
		}
	} else {
		fmt.Printf("请输入测试命令:\n")
		var cmd_str string
		fmt.Scanln(&cmd_str, "\n")
		switch cmd_str {
		case "leave_test":
			{
				is_test = false
			}
		default:
			{
				if cmd_str != "" {
					strs := strings.Split(cmd_str, ",")
					fmt.Printf("strs[%v] length is %v\n", strs, len(strs))
					if len(strs) == 1 {
						//fmt.Printf("命令[%v]参数不够，至少一个\n", strs[0])
						//return
					} else if len(strs) == 0 {
						fmt.Printf("没有输入命令\n")
						return
					}
					req := &msg_client_message.C2S_TEST_COMMAND{}
					req.Cmd = strs[0]
					if len(strs) > 1 {
						req.Args = strs[1:]
					} else {
						req.Args = make([]string, 0)
					}
					this.cmd_chan <- req
				}
			}
		}
	}
	this._heartbeat()
}

func (this *TestClient) _heartbeat() {
	now_time := int32(time.Now().Unix())
	if this.last_heartbeat == 0 {
		this.last_heartbeat = now_time
	}
	if now_time-this.last_heartbeat >= 50 {
		var heartbeat msg_client_message.C2SHeartbeat
		if config.AccountNum > 1 {
			for i := int32(0); i < config.AccountNum; i++ {
				if hall_conn_mgr.acc_arr == nil || len(hall_conn_mgr.acc_arr) < int(i)+1 {
					break
				}
				c := hall_conn_mgr.acc_arr[i]
				if c != nil {
					c.Send(uint16(msg_client_message_id.MSGID_C2S_HEARTBEAT), &heartbeat)
				}
			}
		} else {
			if cur_hall_conn != nil {
				cur_hall_conn.Send(uint16(msg_client_message_id.MSGID_C2S_HEARTBEAT), &heartbeat)
			}
		}
		this.last_heartbeat = now_time
	}
}

func (this *TestClient) _cmd(cmd *msg_client_message.C2S_TEST_COMMAND) {
	if config.AccountNum > 1 {
		log.Debug("############## hall conns length %v, config.AccountNum %v", len(hall_conn_mgr.acc_arr), config.AccountNum)
		for i := int32(0); i < config.AccountNum; i++ {
			c := hall_conn_mgr.acc_arr[i]
			if c == nil {
				continue
			}
			go func(conn *HallConnection) {
				defer func() {
					if err := recover(); err != nil {
						log.Stack(err)
					}

					this.shutdown_completed = true
				}()
				conn.Send(uint16(msg_client_message_id.MSGID_C2S_TEST_COMMAND), cmd)
			}(c)
		}
	} else {
		if cur_hall_conn == nil {
			log.Error("hall connection is not estabulished")
			return
		}
		cur_hall_conn.Send(uint16(msg_client_message_id.MSGID_C2S_TEST_COMMAND), cmd)
	}
}

// 发送消息
func (this *TestClient) SendCmd() {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()
	for {
		is_break := false
		for !is_break {
			select {
			case cmd, ok := <-this.cmd_chan:
				{
					if !ok {
						log.Error("cmd chan receive invalid !!!!!")
						return
					}
					this._cmd(cmd)
				}
			default:
				{
					is_break = true
				}
			}
		}

		this._heartbeat()
		time.Sleep(time.Second * 1)
	}
}

//=================================================================================
