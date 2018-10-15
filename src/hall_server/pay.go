package main

import (
	"crypto"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	_ "encoding/pem"
	"ih_server/libs/log"
	"ih_server/proto/gen_go/client_message"
	"ih_server/src/server_config"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	PAY_CHANNEL_APPLE  = 1 // 苹果渠道
	PAY_CHANNEL_GOOGLE = 2 // 谷歌渠道
	PAY_CHANNEL_FACEB  = 3 // FaceBook 渠道
)

var pay_http_mux map[string]func(http.ResponseWriter, *http.Request)

type PayHttpHandle struct{}

func (this *PayHttpHandle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var act_str, url_str string
	url_str = r.URL.String()
	idx := strings.Index(url_str, "?")
	if -1 == idx {
		act_str = url_str
	} else {
		act_str = string([]byte(url_str)[:idx])
	}
	log.Info("ServeHTTP actstr(%s)", act_str)
	if h, ok := pay_http_mux[act_str]; ok {
		h(w, r)
	}

	return
}

type JsonLoginRes struct {
	Code    int32
	Account string
	Token   string
	HallIP  string
}

func pay_http_handler(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
			return
		}
	}()

	order_str := r.URL.Query().Get("orderid")
	if "" == order_str {
		log.Error("login_http_handler get account failed")
		return
	}

	/*
		res_2c := &msg_server_message.L2CGetPlayerAccInfo{}
		res_2c.Account = proto.String(account_str)
		center_conn.Send(res_2c)

		log.Info("login_http_handler account(%s)", account_str)
		new_c_wait := &WaitCenterInfo{}
		new_c_wait.res_chan = make(chan *msg_server_message.C2LPlayerAccInfo)
		new_c_wait.create_time = int32(time.Now().Unix())
		server.add_to_c_wait(account_str, new_c_wait)

		c2l_res, ok := <-new_c_wait.res_chan
		if !ok || nil == c2l_res {
			log.Error("login_http_handler wait chan failed", ok)
			return
		}

		token_str := r.URL.Query().Get("token")
		if "" == token_str {
			log.Error("login_http_handler token empty")
			return
		}

		hall_id := c2l_res.GetHallId()
		hall_agent := hall_agent_manager.GetAgentByID(hall_id)
		if nil == hall_agent {
			log.Error("login_http_handler get hall_agent failed")
			return
		}

		inner_token := fmt.Sprintf("%d", time.Now().Unix())
		req_2h := &msg_server_message.L2HSyncAccountToken{}
		req_2h.Account = proto.String(account_str)
		req_2h.Token = proto.String(inner_token)
		req_2h.PlayerId = proto.Int64(c2l_res.GetPlayerId())
		hall_agent.Send(req_2h)

		http_res := &JsonLoginRes{Code: 0, Account: account_str, Token: inner_token, HallIP: hall_agent.listen_client_ip}
		data, err := json.Marshal(http_res)
		if nil != err {
			log.Error("login_http_handler json mashal error")
			return
		}
		w.Write(data)
	*/

	return
}

//=======================================================

type XmlPayBidItem struct {
	OrderID   int32  `xml:"OrderID,attr"`
	OrderName string `xml:"OrderName,attr"`
	RewardGem int32  `xml:"RewardGem,attr"`
}

type XmlPayBidConfig struct {
	Items []XmlPayBidItem `xml:"item"`
}

type PayMgr struct {
	//bid2infos         map[string]*XmlPayBidItem
	google_pay_pub    *rsa.PublicKey
	google_payed_sns  map[string]int32
	apple_payed_sns   map[string]int32
	faceb_payed_sns   map[string]int32
	pay_http_listener net.Listener
}

var pay_mgr PayMgr

func (this *PayMgr) init() bool {
	if !this.load_google_pay_pub() {
		log.Error("!!!!! Load googlepay.key failed")
		return false
	}
	return true
}

/*
func (this *PayMgr) load_bid_info() bool {
	this.bid2infos = make(map[string]*XmlPayBidItem)
	content, err := ioutil.ReadFile("../game_data/payConfig.xml")
	if nil != err {
		log.Error("PayMgr load_bid_info failed(%s) !", err.Error())
		return false
	}

	tmp_cfg := &XmlPayBidConfig{}
	err = json.Unmarshal(content, tmp_cfg)
	if nil != err {
		log.Error("PayMgr load_bid_info failed(%s) !", err.Error())
		return false
	}

	for idx := int32(0); idx < int32(len(tmp_cfg.Items)); idx++ {
		tmp_item := &tmp_cfg.Items[idx]
		if nil == tmp_item {
			log.Error("PayMgr load_bid_info tmp_item[%d] nil", idx)
			continue
		}

		this.bid2infos[tmp_item.OrderName] = tmp_item
	}

	return true
}
*/
func (this *PayMgr) load_google_pay_pub() bool {
	path := server_config.GetGameDataPathFile("googlepay.key")
	content, err := ioutil.ReadFile(path)
	if nil != err {
		log.Error("PayMgr Init failed (%s)!", err.Error())
		return false
	}

	block, err := base64.StdEncoding.DecodeString(string(content)) //pem.Decode([]byte(content))
	if err != nil {
		log.Error("failed to parse PEM block containing the public key, err %v", err.Error())
		return false
	}

	pub, err := x509.ParsePKIXPublicKey(block)
	if nil != err {
		log.Error("PayMgr Init failed to ParsePkXIPublicKey", err.Error())
		return false
	}

	this.google_pay_pub = pub.(*rsa.PublicKey)

	return true
}

func (this *PayMgr) load_google_pay_db() bool {
	this.google_payed_sns = make(map[string]int32)
	pre_max_id := dbc.GooglePayRecords.GetPreloadedMaxId()
	for idx := int32(0); idx < pre_max_id; idx++ {
		tmp_row := dbc.GooglePayRecords.GetRow(idx)
		if nil == tmp_row {
			log.Trace("PayMgr load_google_pay_db tmp_row[%d] nil", idx)
			continue
		}

		this.google_payed_sns[tmp_row.GetSn()] = idx
	}

	return true
}

func (this *PayMgr) load_faceb_pay_db() bool {
	this.faceb_payed_sns = make(map[string]int32)
	pre_max_id := dbc.FaceBPayRecords.GetPreloadedMaxId()
	for idx := int32(0); idx < pre_max_id; idx++ {
		tmp_row := dbc.FaceBPayRecords.GetRow(idx)
		if nil == tmp_row {
			log.Trace("PayMgr load_faceb_pay_db tmp_row[%d] nil", idx)
			continue
		}

		this.faceb_payed_sns[tmp_row.GetSn()] = idx
	}

	return true
}

func (this *PayMgr) load_apple_pay_db() bool {
	this.apple_payed_sns = make(map[string]int32)
	pre_max_id := dbc.ApplePayRecords.GetPreloadedMaxId()
	for idx := int32(0); idx < pre_max_id; idx++ {
		tmp_row := dbc.ApplePayRecords.GetRow(idx)
		if nil == tmp_row {
			log.Trace("PayMgr load_apple_pay_db tmp_row[%d] nil", idx)
			continue
		}

		this.apple_payed_sns[tmp_row.GetSn()] = idx
	}

	return true
}

func (this *PayMgr) StartHttp() {
	var err error
	this.reg_http_mux()

	this.pay_http_listener, err = net.Listen("tcp", config.ListenClientInIP)
	if nil != err {
		log.Error("LoginServer StartHttp Failed %s", err.Error())
		return
	}

	login_http_server := http.Server{
		Handler:     &PayHttpHandle{},
		ReadTimeout: 6 * time.Second,
	}

	err = login_http_server.Serve(this.pay_http_listener)
	if err != nil {
		log.Error("启动Login Http Server %s", err.Error())
		return
	}
}

func (this *PayMgr) reg_http_mux() {
	pay_http_mux = make(map[string]func(http.ResponseWriter, *http.Request))
	pay_http_mux["/login"] = pay_http_handler
}

//=========================================================
type GoogleOrderDetail struct {
	Sn        string
	OrderName string
}

type GooglePayOrder struct {
	signture     string
	signtureData string
}

func google_pay_order_verify(p *Player, item_id int32, order_data []byte) int32 { // order *GooglePayOrder
	if len(order_data) <= 0 || nil == p {
		log.Error("google_pay_order_verify order_data nil or p nil(%v)", nil == p)
		return int32(msg_client_message.E_ERR_CHARGE_ORDER_DATA_EMPTY)
	}

	order := &GooglePayOrder{}
	err := json.Unmarshal(order_data, order)
	if nil == err {
		log.Error("google_pay_order_verify unmarshal order_data failed(%s)", err.Error())
		return int32(msg_client_message.E_ERR_CHARGE_ORDER_DATA_INVALID)
	}

	tmp_info := &GoogleOrderDetail{}
	err = json.Unmarshal([]byte(order.signtureData), tmp_info)
	if nil != err {
		log.Error("google_pay_order_verify json unmarshal error(%s) !", err.Error())
		return int32(msg_client_message.E_ERR_CHARGE_ORDER_SIGNATURE_INVALID)
	}

	/*bid_info := pay_mgr.bid2infos[tmp_info.OrderName]
	if nil == bid_info {
		log.Error("google_pay_order_verify can not find bidinfo[%s]", tmp_info.OrderName)
		return false
	}*/

	if "" == tmp_info.Sn {
		log.Error("google_pay_order_verify sn empty !")
		return int32(msg_client_message.E_ERR_CHARGE_ORDER_SN_EMPTY)
	}

	if 0 < pay_mgr.google_payed_sns[tmp_info.Sn] {
		log.Error("google_pay_order_verify sn[%d] already used !", tmp_info.Sn)
		return int32(msg_client_message.E_ERR_CHARGE_ORDER_SN_ALREDY_USED)
	}

	signed := base64.StdEncoding.EncodeToString([]byte(order.signtureData))

	err = rsa.VerifyPKCS1v15(pay_mgr.google_pay_pub, crypto.SHA3_256, []byte(signed)[:], []byte(order.signture))
	if nil != err {
		log.Error("google_pay_order_verify failed !")
		return int32(msg_client_message.E_ERR_CHARGE_ORDER_VERIFY_FAILED)
	}

	new_row := dbc.GooglePayRecords.AddRow()
	if nil == new_row {
		log.Error("google_pay_order_verify failed to add_row pid[%d] order_name[%s] !", p.Id, tmp_info.OrderName)
		return -1
	}

	pay_mgr.google_payed_sns[tmp_info.Sn] = new_row.GetKeyId()
	new_row.SetPlayerId(p.Id)
	new_row.SetSn(tmp_info.Sn)
	new_row.SetBid(tmp_info.OrderName)
	new_row.SetPayTime(int32(time.Now().Unix()))

	//p.AddDiamond(bid_info.RewardGem, "Pay", "PayMgr")
	/*res := p.buy_item(item_id, 1, true)
	if res < 0 {
		return res
	}*/

	/*if PLAYER_FIRST_PAY_NOT_ACT == p.db.Info.GetFirstPayState() {
		p.db.Info.SetFirstPayState(PLAYER_FIRST_PAY_ACT)
		p.SyncPlayerFirstPayState()
	}*/

	return 1
}

type FaceBookPayOrder struct {
}

type ApplePayOrder struct {
	OrderName string `json:"ordername"`
	Receipt   string `json:"receipt-data"`
}

type AppleCheck struct {
	Receipt string `json:"receipt-data"`
}

type AppleCheckRes struct {
	Status  int32  `json:"status"`
	Receipt string `json:"receipt"`
}

func apple_pay_verify(p *Player, order_data []byte) bool { //ordername, receipt string
	if nil == p || len(order_data) <= 0 {
		log.Error("apple_pay_verify param(%v) error !", order_data)
		return false
	}

	tmp_order := &ApplePayOrder{}
	err := json.Unmarshal(order_data, tmp_order)
	if nil != err {
		log.Error("apple_pay_verify Unmarshal order_data failed(%s) !", err.Error())
		return false
	}

	ordername := tmp_order.OrderName
	receipt := tmp_order.Receipt

	/*bid_info := pay_mgr.bid2infos[ordername]
	if nil == bid_info {
		log.Error("apple_pay_verify can not find bidinfo[%s]", ordername)
		return false
	}*/

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	check_data := &AppleCheck{Receipt: receipt}
	data, err := json.Marshal(check_data)
	if nil != err {
		log.Error("apple_pay_verify json Marshal failed !")
		return false
	}
	final_str := base64.StdEncoding.EncodeToString(data)

	req, err := http.NewRequest("POST", global_config.ApplePayUrl, strings.NewReader(final_str))
	if err != nil {
		log.Error("apple_pay_verify http NewRequest failed(%s) !", err.Error())
		return false
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)

	if err != nil {
		return false
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	tmp_res := &AppleCheckRes{}
	err = json.Unmarshal(body, tmp_res)
	if nil != err {
		log.Error("apple_pay_verify unmarshal res body failed(%s) !", err.Error())
		return false
	}

	if 0 != tmp_res.Status {
		log.Error("apple_pay_verify Receipt check failed(%d) !", tmp_res.Status)
		return false
	}

	new_row := dbc.ApplePayRecords.AddRow()
	if nil == new_row {
		log.Error("apple_pay_verify failed to add_row pid[%d] order_name[%s] !", p.Id, ordername)
		return false
	}

	pay_mgr.apple_payed_sns[receipt] = new_row.GetKeyId()
	new_row.SetPlayerId(p.Id)
	new_row.SetSn(receipt)
	new_row.SetBid(ordername)
	new_row.SetPayTime(int32(time.Now().Unix()))

	//p.AddDiamond(bid_info.RewardGem, "Pay", "PayMgr")

	/*if PLAYER_FIRST_PAY_NOT_ACT == p.db.Info.GetFirstPayState() {
		p.db.Info.SetFirstPayState(PLAYER_FIRST_PAY_ACT)
		p.SyncPlayerFirstPayState()
	}*/

	return true
}

// ============================================

/*func C2SPayOrderHandler(w http.ResponseWriter, r *http.Request, p *Player, msg proto.Message) int32 {
	req := msg.(*msg_client_message.C2SPayOrder)
	if nil == req {
		log.Error("C2SPayOrderHandler req invalid")
		return -1
	}

	if nil == p {
		log.Error("C2SPayOrderHandler not login")
		return -1
	}

	var res int32
	pay_channel := req.GetChannel()
	switch pay_channel {
	case PAY_CHANNEL_APPLE:
		{
			go apple_pay_verify(p, req.GetOrderData())
		}
	case PAY_CHANNEL_FACEB:
		{
			log.Error("C2SC2SPayOrderHandler pay data error !")
		}
	case PAY_CHANNEL_GOOGLE:
		{
			res = google_pay_order_verify(p, req.GetItemId(), req.GetOrderData())
		}
	default:
		{
			log.Warn("Player[%v] pay channel[%v] invalid", p.Id, pay_channel)
			return int32(msg_client_message.E_ERR_CHARGE_CHANNEL_INVALID)
		}
	}

	response := &msg_client_message.S2CPayOrder{}
	if res == 2 {
		response.IsMonthCard = proto.Int32(1)
	} else {
		response.IsMonthCard = proto.Int32(0)
	}
	p.Send(response)

	return res
}*/
