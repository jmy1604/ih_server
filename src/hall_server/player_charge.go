package main

import (
	"bytes"
	"compress/gzip"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"ih_server/libs/log"
	"ih_server/libs/utils"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"ih_server/src/table_config"
	"io/ioutil"
	"net/http"
	"net/url"
	_ "strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/gomodule/redigo/redis"
)

type ChargeMonthCardManager struct {
	players_map     map[int32]*Player
	player_ids_chan chan int32
	to_end          int32
}

var charge_month_card_manager ChargeMonthCardManager

func (this *ChargeMonthCardManager) Init() {
	this.players_map = make(map[int32]*Player)
	this.player_ids_chan = make(chan int32, 10000)
}

func (this *ChargeMonthCardManager) InsertPlayer(player_id int32) {
	this.player_ids_chan <- player_id
}

func (this *ChargeMonthCardManager) _process_send_mails() {
	month_cards := pay_table_mgr.GetMonthCards()
	if month_cards == nil {
		return
	}

	now_time := time.Now()
	var to_delete_players map[int32]int32
	for pid, p := range this.players_map {
		if p == nil || !p.charge_month_card_award(month_cards, now_time) {
			if to_delete_players == nil {
				to_delete_players = make(map[int32]int32)
			}
			to_delete_players[pid] = 1
		}
	}

	if to_delete_players != nil {
		for pid, _ := range to_delete_players {
			delete(this.players_map, pid)
		}
	}
}

func (this *ChargeMonthCardManager) Run() {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	for {
		if atomic.LoadInt32(&this.to_end) > 0 {
			break
		}

		var is_break bool
		for !is_break {
			select {
			case player_id, o := <-this.player_ids_chan:
				{
					if !o {
						log.Error("conn timer wheel op chan receive invalid !!!!!")
						return
					}
					if this.players_map[player_id] == nil {
						this.players_map[player_id] = player_mgr.GetPlayerById(player_id)
					}
				}
			default:
				{
					is_break = true
				}
			}
		}

		this._process_send_mails()

		time.Sleep(time.Minute)
	}
}

func (this *ChargeMonthCardManager) End() {
	atomic.StoreInt32(&this.to_end, 1)
}

const (
	GOOGLE_PAY_REDIS_KEY = "ih:hall_server:google_pay"
	APPLE_PAY_REDIS_KEY  = "ih:hall_server:apple_pay"
)

type RedisPayInfo struct {
	OrderId  string
	BundleId string
	Account  string
	PlayerId int32
	PayTime  int32
}

func google_pay_save(order_id, bundle_id, account string, player_id int32) {
	var pay RedisPayInfo
	pay.OrderId = order_id
	pay.BundleId = bundle_id
	pay.Account = account
	pay.PlayerId = player_id
	pay.PayTime = int32(time.Now().Unix())

	// serialize to redis
	bytes, err := json.Marshal(&pay)
	if err != nil {
		log.Error("##### Serialize RedisPayInfo[%v] error[%v]", pay, err.Error())
		return
	}
	err = hall_server.redis_conn.Post("HSET", GOOGLE_PAY_REDIS_KEY, order_id, string(bytes))
	if err != nil {
		log.Error("redis设置集合[%v]数据失败[%v]", GOOGLE_PAY_REDIS_KEY, err.Error())
		return
	}

	log.Info("save google pay: player_id(%v), order_id(%v), bundle_id(%v)", player_id, order_id, bundle_id)
}

func check_google_order_exist(order_id string) bool {
	exist, err := redis.Int(hall_server.redis_conn.Do("HEXISTS", GOOGLE_PAY_REDIS_KEY, order_id))
	if err != nil {
		log.Error("redis do err %v", err.Error())
		return false
	}
	if exist <= 0 {
		return false
	}
	return true
}

func apple_pay_save(order_id, bundle_id, account string, player_id int32) {
	var pay RedisPayInfo
	pay.OrderId = order_id
	pay.BundleId = bundle_id
	pay.Account = account
	pay.PlayerId = player_id
	pay.PayTime = int32(time.Now().Unix())
	bytes, err := json.Marshal(&pay)
	if err != nil {
		log.Error("##### Serialize RedisPayInfo[%v] error[%v]", pay, err.Error())
		return
	}
	err = hall_server.redis_conn.Post("HSET", APPLE_PAY_REDIS_KEY, order_id, string(bytes))
	if err != nil {
		log.Error("redis设置集合[%v]数据失败[%v]", APPLE_PAY_REDIS_KEY, err.Error())
		return
	}
	log.Info("save apple pay: player_id(%v), order_id(%v), bundle_id(%v)", player_id, order_id, bundle_id)
}

func check_apple_order_exist(order_id string) bool {
	exist, err := redis.Int(hall_server.redis_conn.Do("HEXISTS", APPLE_PAY_REDIS_KEY, order_id))
	if err != nil {
		log.Error("redis do err %v", err.Error())
		return false
	}
	if exist <= 0 {
		return false
	}
	return true
}

func (this *Player) _charge_month_card_award(month_card *table_config.XmlPayItem, now_time time.Time) (send_num int32) {
	var bonus []int32 = []int32{ITEM_RESOURCE_ID_DIAMOND, month_card.MonthCardReward}
	// 大月卡
	if month_card.Id == 2 {
		vip_info := vip_table_mgr.Get(this.db.Info.GetVipLvl())
		if vip_info != nil && vip_info.MonthCardItemBonus != nil && len(vip_info.MonthCardItemBonus) > 0 {
			bonus = append(bonus, vip_info.MonthCardItemBonus...)
		}
	}
	RealSendMail(nil, this.Id, MAIL_TYPE_SYSTEM, 1105, "", "", bonus, 0)
	send_num = this.db.Pays.IncbySendMailNum(month_card.BundleId, 1)
	log.Debug("Player[%v] charge month card %v get reward, send_num %v", this.Id, month_card.BundleId, send_num)
	return
}

func (this *Player) charge_month_card_award(month_cards []*table_config.XmlPayItem, now_time time.Time) bool {
	if !this.charge_has_month_card() {
		return false
	}
	for _, m := range month_cards {
		last_award_time, o := this.db.Pays.GetLastAwardTime(m.BundleId)
		if !o {
			continue
		}
		send_num, _ := this.db.Pays.GetSendMailNum(m.BundleId)
		if send_num >= 30 {
			continue
		}
		num := utils.GetDaysNumToLastSaveTime(last_award_time, global_config.MonthCardSendRewardTime, now_time)
		for i := int32(0); i < num; i++ {
			send_num = this._charge_month_card_award(m, now_time)
			if send_num >= 30 {
				break
			}
		}
		if num > 0 {
			this.db.Pays.SetLastAwardTime(m.BundleId, int32(now_time.Unix()))
		}
	}
	return true
}

func (this *Player) charge_has_month_card() bool {
	// 获得月卡配置
	arr := pay_table_mgr.GetMonthCards()
	if arr == nil {
		return false
	}

	for i := 0; i < len(arr); i++ {
		bundle_id := arr[i].BundleId
		send_num, o := this.db.Pays.GetSendMailNum(bundle_id)
		if o && send_num < 30 {
			return true
		}
	}

	return false
}

func (this *Player) charge_data() int32 {
	var charged_ids []string
	var datas []*msg_client_message.MonthCardData
	all_index := this.db.Pays.GetAllIndex()
	for _, idx := range all_index {
		pay_item := pay_table_mgr.GetByBundle(idx)
		if pay_item == nil {
			continue
		}
		if pay_item.PayType == table_config.PAY_TYPE_MONTH_CARD {
			payed_time, _ := this.db.Pays.GetLastPayedTime(idx)
			send_mail_num, _ := this.db.Pays.GetSendMailNum(idx)
			datas = append(datas, &msg_client_message.MonthCardData{
				BundleId:    idx,
				EndTime:     payed_time + 30*24*3600,
				SendMailNum: send_mail_num,
			})
		}
		charged_ids = append(charged_ids, idx)
	}

	response := &msg_client_message.S2CChargeDataResponse{
		FirstChargeState: this.db.PayCommon.GetFirstPayState(),
		Datas:            datas,
		ChargedBundleIds: charged_ids,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_CHARGE_DATA_RESPONSE), response)

	log.Debug("Player[%v] charge data %v", this.Id, response)

	return 1
}

func (this *Player) charge(channel, id int32) int32 {
	pay_item := pay_table_mgr.Get(id)
	if pay_item == nil {
		return -1
	}
	return this.charge_with_bundle_id(channel, pay_item.BundleId, nil, nil, 0)
}

type GooglePurchaseInfo struct {
	OrderId          string `json:"orderId"`
	PackageName      string `json:"packageName"`
	ProductId        string `json:"productId"`
	PurchaseTime     int64  `json:"purchaseTime"`
	PurchaseState    int32  `json:"purchaseState"`
	DeveloperPayload string `json:"developerPayload"`
	PurchaseToken    string `json:"purchaseToken"`
	AutoRenewing     bool   `json:"autoRenewing"`
}

type GoogleAccessTokenResp struct {
	AccessToken string  `json:"access_token"`
	ExpiresIn   float64 `json:"expires_in"`
}

type GoogleVerifyData struct {
	Kind               string
	DeveloperPayload   string
	PurchaseTimeMillis string
	PurchaseState      int
	ConsumptionState   int
}

func _get_google_access_token() (string, error) {
	var client_id, client_secret, refresh_token string

	v := url.Values{}
	v.Set("grant_type", "refresh_token")
	v.Set("client_id", client_id)
	v.Set("client_secret", client_secret)
	v.Set("refresh_token", refresh_token)
	form_body := ioutil.NopCloser(strings.NewReader(v.Encode()))
	req, err := http.NewRequest("POST", "https://accounts.google.com/o/oauth2/token", form_body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err != nil {
		log.Error("new request err %v", err.Error())
		return "", err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	defer resp.Body.Close()
	if err != nil {
		log.Error("post request err %v", err.Error())
		return "", err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error("read response err %v", err.Error())
		return "", err
	}
	if bytes.Contains(body, []byte("access_token")) {
		atr := GoogleAccessTokenResp{}
		err = json.Unmarshal(body, &atr)
		if err != nil {
			log.Error("unmarshal err %v", err.Error())
			return "", err
		}
		return atr.AccessToken, nil
	} else {
		if err != nil {
			log.Error("contains err %v", err.Error())
			return "", err
		}
		return "", fmt.Errorf("failed to get access_token")
	}
}

func _verify_google_purchase_token(package_name, product_id, purchase_token string) error {
	access_token, err := _get_google_access_token()
	if err != nil {
		return err
	}

	var verifyUrl string = "https://www.googleapis.com/androidpublisher/v2/applications/%s/purchases/products/%s/tokens/%s?access_token=%s"
	url := fmt.Sprintf(verifyUrl, package_name, product_id, purchase_token, access_token)
	resp, err := http.Get(url)
	defer resp.Body.Close()
	if err != nil {
		return err
	}

	body, _ := ioutil.ReadAll(resp.Body)
	data := GoogleVerifyData{}
	err = json.Unmarshal(body, &data)
	if err != nil || data.PurchaseState != 0 {
		fmt.Println(err)
		return err
	}
	return nil
}

func (this *Player) verify_google_purchase_data(bundle_id string, purchase_data, signature []byte) int32 {
	data := &GooglePurchaseInfo{}
	err := json.Unmarshal(purchase_data, &data)
	if err != nil {
		log.Error("Player[%v] unmarshal Purchase data error %v", this.Id, err.Error())
		return -1
	}

	if !atomic.CompareAndSwapInt32(&this.is_paying, 0, 1) {
		log.Error("Player[%v] is paying for google purchase", this.Id)
		return int32(msg_client_message.E_ERR_CHARGE_PAY_REPEATED_VERIFY)
	}

	if check_google_order_exist(data.OrderId) {
		atomic.CompareAndSwapInt32(&this.is_paying, 1, 0)
		log.Error("Player[%v] google order[%v] already exists", this.Id, data.OrderId)
		return int32(msg_client_message.E_ERR_CHARGE_GOOGLE_ORDER_ALREADY_EXIST)
	}

	// 验证签名
	var decodedSignature []byte
	decodedSignature, err = base64.StdEncoding.DecodeString(string(signature))
	if err != nil {
		atomic.CompareAndSwapInt32(&this.is_paying, 1, 0)
		log.Error("Player[%v] failed to decode signature[%v], err %v", this.Id, signature, err.Error())
		return -1
	}
	sha1 := sha1.New()
	sha1.Write(purchase_data)
	hashedReceipt := sha1.Sum(nil)
	if hashedReceipt == nil {
		atomic.CompareAndSwapInt32(&this.is_paying, 1, 0)
		log.Error("Player[%v] purchase_data[%v] hased result is null", purchase_data)
		return -1
	}

	err = rsa.VerifyPKCS1v15(pay_mgr.google_pay_pub, crypto.SHA1, hashedReceipt, decodedSignature)
	if err != nil {
		atomic.CompareAndSwapInt32(&this.is_paying, 1, 0)
		log.Error("Player[%v] failed to verify decoded signature[%v] with hashed purchase data[%v]: %v", this.Id, decodedSignature, hashedReceipt, err.Error())
		return int32(msg_client_message.E_ERR_CHARGE_GOOGLE_SIGNATURE_INVALID)
	}

	google_pay_save(data.OrderId, bundle_id, this.Account, this.Id)

	atomic.CompareAndSwapInt32(&this.is_paying, 1, 0)

	// 验证PurchaseToken
	/*err = _verify_google_purchase_token(data.PackageName, data.ProductId, data.PurchaseToken)
	if err != nil {
		log.Error("Player[%v] verify google purchase token err %v", this.Id, err.Error())
		return int32(msg_client_message.E_ERR_CHARGE_GOOGLE_PURCHASE_TOKEN_INVALID)
	}*/

	pay_item := pay_table_mgr.GetByBundle(bundle_id)
	if pay_item != nil {
		_post_talking_data(this.Account, "google pay", config.ServerName, config.InnerVersion, "google", data.OrderId, "android", "charge", "success", this.db.Info.GetLvl(), pay_item.RecordGold, "USD", float64(pay_item.GemReward))
	}

	log.Info("Player[%v] google pay bunder_id[%v] purchase_data[%v] signature[%v] verify success", this.Id, bundle_id, purchase_data, signature)

	return 1
}

type AppleReceiptResponse struct {
	TransactionId string `json:"transaction_id"`
}

type ApplePurchaseCheckRes struct {
	Status  int32                `json:"status"`
	Receipt AppleReceiptResponse `json:"receipt"`
}

func (this *Player) _send_apple_verify_url(url string, data []byte) (int32, *ApplePurchaseCheckRes) {
	var err error
	var req *http.Request
	req, err = http.NewRequest("POST", url, strings.NewReader(string(data)))
	if err != nil {
		log.Error("Player[%v] new apple pay request failed(%s) !", this.Id, err.Error())
		return int32(msg_client_message.E_ERR_CHARGE_APPLE_PAY_NEW_REQUEST_FAILED), nil
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var resp *http.Response
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	resp, err = client.Do(req)

	if err != nil {
		log.Error("Player[%v] post apple pay request failed(%s) !", this.Id, err.Error())
		return int32(msg_client_message.E_ERR_CHARGE_APPLE_PAY_REQUEST_FAILED), nil
	}

	defer resp.Body.Close()

	var body []byte
	body, err = ioutil.ReadAll(resp.Body)
	tmp_res := &ApplePurchaseCheckRes{}
	err = json.Unmarshal(body, tmp_res)
	if nil != err {
		log.Error("Player[%v] get apple pay verify result unmarshal failed(%s) !", this.Id, err.Error())
		return int32(msg_client_message.E_ERR_CHARGE_APPLE_PAY_RESULT_UNMARSHAL_FAILED), nil
	}

	if check_apple_order_exist(tmp_res.Receipt.TransactionId) {
		log.Error("Player[%v] apple transaction is[%v] already exists", this.Id, tmp_res.Receipt.TransactionId)
		return int32(msg_client_message.E_ERR_CHARGE_APPLE_ORDER_ALREADY_EXIST), nil
	}

	return 1, tmp_res
}

func (this *Player) verify_apple_purchase_data(bundle_id string, purchase_data []byte) int32 { //ordername, receipt string
	if purchase_data == nil || len(purchase_data) == 0 {
		log.Error("Player[%v] apple purchase data empty!")
		return int32(msg_client_message.E_ERR_CHARGE_APPLE_PURCHASE_DATA_EMPTY)
	}

	check_data := &AppleCheck{Receipt: string(purchase_data)}
	var data []byte
	var err error
	data, err = json.Marshal(check_data)
	if nil != err {
		log.Error("Player[%v] marshal apple pay verify purchase_data[%v] failed: %v", this.Id, purchase_data, err.Error())
		return int32(msg_client_message.E_ERR_CHARGE_APPLE_PURCHASE_DATA_INVALID)
	}

	if !atomic.CompareAndSwapInt32(&this.is_paying, 0, 1) {
		log.Error("Player[%v] is paying for apple purchase", this.Id)
		return int32(msg_client_message.E_ERR_CHARGE_PAY_REPEATED_VERIFY)
	}

	res, tmp_res := this._send_apple_verify_url(global_config.ApplePayUrl, data)
	if res < 0 {
		atomic.CompareAndSwapInt32(&this.is_paying, 1, 0)
		return res
	}

	// 发送到沙箱验证
	if tmp_res.Status == 21007 {
		res, tmp_res = this._send_apple_verify_url(global_config.ApplePaySandBoxUrl, data)
		if res < 0 {
			atomic.CompareAndSwapInt32(&this.is_paying, 1, 0)
			return res
		}
	} else if 0 != tmp_res.Status {
		atomic.CompareAndSwapInt32(&this.is_paying, 1, 0)
		log.Error("Player[%v] apple pay verify Receipt check failed(%d) !", this.Id, tmp_res.Status)
		return int32(msg_client_message.E_ERR_CHARGE_APPLE_PAY_VERIFY_NO_PASS)
	}

	apple_pay_save(tmp_res.Receipt.TransactionId, bundle_id, this.Account, this.Id)

	atomic.CompareAndSwapInt32(&this.is_paying, 1, 0)

	pay_item := pay_table_mgr.GetByBundle(bundle_id)
	if pay_item != nil {
		_post_talking_data(this.Account, "apple pay", config.ServerName, config.InnerVersion, "apple", tmp_res.Receipt.TransactionId, "ios", "charge", "success", this.db.Info.GetLvl(), pay_item.RecordGold, "USD", float64(pay_item.GemReward))
	}

	log.Info("Player[%v] apple pay bunder_id[%v] verify success", this.Id, bundle_id)

	return 1
}

type TalkingData struct {
	MsgId                 string `json:"msgID"`
	GameVersion           string `json:"gameVersion"`
	OS                    string
	AccountId             string  `json:"accountID"`
	Level                 int32   `json:"level"`
	GameServer            string  `json:"gameServer"`
	OrderId               string  `json:"orderID"`
	IapId                 string  `json:"iapID"`
	CurrencyAmount        float64 `json:"currencyAmount"`
	CurrencyType          string  `json:"currencyType"`
	VirtualCurrencyAmount float64 `json:"virtualCurrencyAmount"`
	PaymentType           string  `json:"paymentType"`
	Status                string  `json:"status"`
	ChargeTime            int64   `json:"chargeTime"`
	Mission               string  `json:"mission"`
}

type TalkingDataStatus struct {
	MsgId string `json:"msgID"`
	Code  int32  `json:"code"`
	Msg   string `json:"msg"`
}

type TalkingDataResponse struct {
	Code       int32  `json:"code"`
	Msg        string `json:"msg"`
	DataStatus []*TalkingDataStatus
}

func _gzip_encode(data []byte) ([]byte, error) {
	var (
		buffer bytes.Buffer
		out    []byte
		err    error
	)
	writer := gzip.NewWriter(&buffer)
	_, err = writer.Write(data)
	if err != nil {
		writer.Close()
		return out, err
	}
	err = writer.Close()
	if err != nil {
		return out, err
	}

	return buffer.Bytes(), nil
}

func _post_talking_data(account, pay_type, game_server, game_version, partner, order_id, os, iap_id, status string, level int32, currency_amount float64, currency_type string, virtual_currency_amount float64) {
	pay := &TalkingData{
		MsgId:                 "Charge",
		GameVersion:           game_version,
		OS:                    os,
		AccountId:             account,
		Level:                 level,
		GameServer:            game_server,
		OrderId:               order_id,
		IapId:                 iap_id,
		CurrencyAmount:        currency_amount,
		CurrencyType:          currency_type,
		VirtualCurrencyAmount: virtual_currency_amount,
		PaymentType:           pay_type,
		Status:                status,
		ChargeTime:            (time.Now().Unix())*1000 + time.Now().UnixNano()/(1000*1000),
	}
	bytes, err := json.Marshal([]*TalkingData{pay})
	if err != nil {
		log.Error("Account %v serialize TalkingData[%v] error[%v]", account, pay, err.Error())
		return
	}
	bytes, err = _gzip_encode(bytes)
	if err != nil {
		log.Error("Account %v talking data gzip encode %v error %v", account, bytes, err.Error())
		return
	}
	req, err := http.NewRequest("POST", "http://api.talkinggame.com/api/charge/78B77DA4D9BE48599D6482350A9B8976", strings.NewReader(string(bytes)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err != nil {
		log.Error("Account %v talking data new request err %v", account, err.Error())
		return
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	defer resp.Body.Close()
	if err != nil {
		log.Error("Account %v post talking data request err %v", account, err.Error())
		return
	}

	var body []byte
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error("Account %v read talking data response err %v", account, err.Error())
		return
	}

	var tr TalkingDataResponse
	err = json.Unmarshal(body, &tr)
	if err != nil {
		log.Error("Account %v unmarshal talking data resposne error %v", account, err.Error())
		return
	}

	if tr.Code != 100 {
		log.Error("Account %v post talking data with order_id %v error %v", account, order_id, tr.Code)
		return
	}

	if tr.DataStatus != nil && len(tr.DataStatus) > 0 {
		if tr.DataStatus[0].Code != 1 {
			log.Error("Account %v post talking data with order_id %v to only data error %v", account, order_id, tr.DataStatus[0].Code)
			return
		}
	}

	log.Trace("Account %v posted talking data payment order id %v", account, order_id)
}

func (this *Player) _charge_with_bundle_id(channel int32, bundle_id string, purchase_data []byte, extra_data []byte, index int32) (int32, bool) {
	pay_item := pay_table_mgr.GetByBundle(bundle_id)
	if pay_item == nil {
		log.Error("pay %v table data not found", bundle_id)
		return int32(msg_client_message.E_ERR_CHARGE_TABLE_DATA_NOT_FOUND), false
	}

	var has bool
	has = this.db.Pays.HasIndex(bundle_id)
	if has {
		if pay_item.PayType == table_config.PAY_TYPE_MONTH_CARD {
			mail_num, o := this.db.Pays.GetSendMailNum(bundle_id)
			if o && mail_num < 30 {
				log.Error("Player[%v] payed month card %v is using, not outdate", this.Id, bundle_id)
				return int32(msg_client_message.E_ERR_CHARGE_MONTH_CARD_ALREADY_PAYED), false
			}
		}
	}

	if channel == 1 {
		err_code := this.verify_google_purchase_data(bundle_id, purchase_data, extra_data)
		if err_code < 0 {
			return err_code, false
		}
	} else if channel == 2 {
		err_code := this.verify_apple_purchase_data(bundle_id, purchase_data)
		if err_code < 0 {
			return err_code, false
		}
	} else if channel != 0 {
		log.Error("Player[%v] charge channel[%v] invalid", this.Id, channel)
		return int32(msg_client_message.E_ERR_CHARGE_CHANNEL_INVALID), false
	}

	if has {
		this.db.Pays.IncbyChargeNum(bundle_id, 1)
		this.add_diamond(pay_item.GemReward) // 充值获得钻石
		if pay_item.PayType == table_config.PAY_TYPE_MONTH_CARD {
			this.db.Pays.SetSendMailNum(bundle_id, 0)
		}
	} else {
		this.db.Pays.Add(&dbPlayerPayData{
			BundleId: bundle_id,
		})
		this.add_diamond(pay_item.GemRewardFirst) // 首次充值奖励
	}

	this.add_resources(pay_item.ItemReward)

	if pay_item.PayType == table_config.PAY_TYPE_MONTH_CARD {
		charge_month_card_manager.InsertPlayer(this.Id)
	}

	now_time := time.Now()
	this.db.Pays.SetLastPayedTime(bundle_id, int32(now_time.Unix()))

	if this.db.PayCommon.GetFirstPayState() == 0 {
		this.db.PayCommon.SetFirstPayState(1)
		// 首充通知
		notify := &msg_client_message.S2CChargeFirstRewardNotify{}
		this.Send(uint16(msg_client_message_id.MSGID_S2C_CHARGE_FIRST_REWARD_NOTIFY), notify)
	}

	if pay_item.ActivePay > 0 {
		this.activity_update(ACTIVITY_EVENT_CHARGE, 0, 0, 0, 0, bundle_id)
	}

	return 1, !has
}

func (this *Player) charge_with_bundle_id(channel int32, bundle_id string, purchase_data []byte, extra_data []byte, index int32) int32 {
	res, is_first := this._charge_with_bundle_id(channel, bundle_id, purchase_data, extra_data, index)
	if res < 0 {
		return res
	}

	response := &msg_client_message.S2CChargeResponse{
		BundleId:    bundle_id,
		IsFirst:     is_first,
		ClientIndex: index,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_CHARGE_RESPONSE), response)

	log.Debug("Player[%v] charged bundle %v", this.Id, response)

	return 1
}

func (this *Player) charge_first_award() int32 {
	pay_state := this.db.PayCommon.GetFirstPayState()
	if pay_state == 0 {
		log.Error("Player[%v] not first charge, cant award", this.Id)
		return int32(msg_client_message.E_ERR_CHARGE_FIRST_NO_DONE)
	} else if pay_state == 2 {
		log.Error("Player[%v] first charge cant award repeated", this.Id)
		return int32(msg_client_message.E_ERR_CHARGE_FIRST_ALREADY_AWARD)
	} else if pay_state != 1 {
		log.Error("Player[%v] first charge state %v invalid", this.Id, pay_state)
		return -1
	}

	var rewards map[int32]int32
	first_charge_rewards := global_config.FirstChargeRewards
	if first_charge_rewards != nil {
		this.add_resources(first_charge_rewards)

		for i := 0; i < len(first_charge_rewards)/2; i++ {
			rid := first_charge_rewards[2*i]
			rn := first_charge_rewards[2*i+1]
			if rewards == nil {
				rewards = make(map[int32]int32)
			}
			rewards[rid] += rn
		}
	}

	this.db.PayCommon.SetFirstPayState(2)

	response := &msg_client_message.S2CChargeFirstAwardResponse{
		Rewards: Map2ItemInfos(rewards),
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_CHARGE_FIRST_AWARD_RESPONSE), response)

	log.Debug("Player[%v] first charge award %v", this.Id, response)

	return 1
}

func C2SChargeDataHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SChargeDataRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%v)", err.Error())
		return -1
	}
	return p.charge_data()
}

func C2SChargeHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SChargeRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%v)", err.Error())
		return -1
	}
	return p.charge_with_bundle_id(req.GetChannel(), req.GetBundleId(), req.GetPurchareData(), req.GetExtraData(), req.GetClientIndex())
}

func C2SChargeFirstAwardHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SChargeFirstAwardRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%v)", err.Error())
		return -1
	}
	return p.charge_first_award()
}
