package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"

	"ih_server/libs/log"
	"time"

	"ih_server/proto/gen_go/client_message"

	"github.com/gomodule/redigo/redis"
)

const (
	GOOGLE_PAY_REDIS_KEY = "ih:hall_server:google_pay"
	APPLE_PAY_REDIS_KEY  = "ih:hall_server:apple_pay"
)

type RedisPayInfo struct {
	BundleId   string
	Account    string
	PlayerId   int32
	PayTime    int32
	PayTimeStr string
}

func google_pay_save(order_id, bundle_id, account string, player *Player) {
	var pay RedisPayInfo
	pay.BundleId = bundle_id
	pay.Account = account
	pay.PlayerId = player.Id
	now_time := time.Now()
	pay.PayTime = int32(now_time.Unix())
	pay.PayTimeStr = now_time.Format("2006-01-02 15:04:05")

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

	player.rpc_charge_save(1, order_id, bundle_id, account, player.Id, int32(now_time.Unix()), pay.PayTimeStr)

	log.Trace("save google pay: player_id(%v), order_id(%v), bundle_id(%v)", player.Id, order_id, bundle_id)
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

func apple_pay_save(order_id, bundle_id, account string, player *Player) {
	var pay RedisPayInfo
	pay.BundleId = bundle_id
	pay.Account = account
	pay.PlayerId = player.Id
	now_time := time.Now()
	pay.PayTime = int32(now_time.Unix())
	pay.PayTimeStr = now_time.Format("2006-01-02 15:04:05")
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

	player.rpc_charge_save(2, order_id, bundle_id, account, player.Id, int32(now_time.Unix()), pay.PayTimeStr)

	log.Trace("save apple pay: player_id(%v), order_id(%v), bundle_id(%v)", player.Id, order_id, bundle_id)
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

func verify_google_purchase_data(player *Player, bundle_id string, purchase_data, signature []byte) int32 {
	data := &GooglePurchaseInfo{}
	err := json.Unmarshal(purchase_data, &data)
	if err != nil {
		log.Error("Player[%v] unmarshal Purchase data error %v", GOOGLE_PAY_REDIS_KEY, err.Error())
		return -1
	}

	if !atomic.CompareAndSwapInt32(&player.is_paying, 0, 1) {
		log.Error("Player[%v] is paying for google purchase", player.Id)
		return int32(msg_client_message.E_ERR_CHARGE_PAY_REPEATED_VERIFY)
	}

	if check_google_order_exist(data.OrderId) {
		atomic.CompareAndSwapInt32(&player.is_paying, 1, 0)
		log.Error("Player[%v] google order[%v] already exists", player.Id, data.OrderId)
		return int32(msg_client_message.E_ERR_CHARGE_GOOGLE_ORDER_ALREADY_EXIST)
	}

	// 验证签名
	var decodedSignature []byte
	decodedSignature, err = base64.StdEncoding.DecodeString(string(signature))
	if err != nil {
		atomic.CompareAndSwapInt32(&player.is_paying, 1, 0)
		log.Error("Player[%v] failed to decode signature[%v], err %v", player.Id, signature, err.Error())
		return -1
	}
	sha1 := sha1.New()
	sha1.Write(purchase_data)
	hashedReceipt := sha1.Sum(nil)
	if hashedReceipt == nil {
		atomic.CompareAndSwapInt32(&player.is_paying, 1, 0)
		log.Error("Player[%v] purchase_data[%v] hased result is null", purchase_data)
		return -1
	}

	pay_channel := pay_list.Verify(hashedReceipt, decodedSignature)
	if pay_channel == nil {
		atomic.CompareAndSwapInt32(&player.is_paying, 1, 0)
		log.Error("Player[%v] failed to verify decoded signature[%v] with hashed purchase data[%v]: %v", player.Id, decodedSignature, hashedReceipt, err.Error())
		return int32(msg_client_message.E_ERR_CHARGE_GOOGLE_SIGNATURE_INVALID)
	}

	google_pay_save(data.OrderId, bundle_id, player.Account, player)

	atomic.CompareAndSwapInt32(&player.is_paying, 1, 0)

	// 验证PurchaseToken
	/*err = _verify_google_purchase_token(data.PackageName, data.ProductId, data.PurchaseToken)
	if err != nil {
		log.Error("Player[%v] verify google purchase token err %v", this.Id, err.Error())
		return int32(msg_client_message.E_ERR_CHARGE_GOOGLE_PURCHASE_TOKEN_INVALID)
	}*/

	pay_item := pay_table_mgr.GetByBundle(bundle_id)
	if pay_item != nil {
		_post_talking_data(player.Account, "google pay", config.ServerName, config.InnerVersion, pay_channel.Partner, data.OrderId, "android", "charge", "success", player.db.Info.GetLvl(), pay_item.RecordGold, "USD", float64(pay_item.GemReward))
	}

	log.Trace("Player[%v] google pay bunder_id[%v] purchase_data[%v] signature[%v] verify success", player.Id, bundle_id, purchase_data, signature)

	return 1
}

type AppleReceiptResponse struct {
	TransactionId string `json:"transaction_id"`
}

type ApplePurchaseCheckRes struct {
	Status  int32                `json:"status"`
	Receipt AppleReceiptResponse `json:"receipt"`
}

type AppleCheck struct {
	Receipt string `json:"receipt-data"`
}

func _send_apple_verify_url(player_id int32, url string, data []byte) (int32, *ApplePurchaseCheckRes) {
	var err error
	var req *http.Request
	req, err = http.NewRequest("POST", url, strings.NewReader(string(data)))
	if err != nil {
		log.Error("Player[%v] new apple pay request failed(%s) !", player_id, err.Error())
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
		log.Error("Player[%v] post apple pay request failed(%s) !", player_id, err.Error())
		return int32(msg_client_message.E_ERR_CHARGE_APPLE_PAY_REQUEST_FAILED), nil
	}

	defer resp.Body.Close()

	var body []byte
	body, err = ioutil.ReadAll(resp.Body)
	tmp_res := &ApplePurchaseCheckRes{}
	err = json.Unmarshal(body, tmp_res)
	if nil != err {
		log.Error("Player[%v] get apple pay verify result unmarshal failed(%s) !", player_id, err.Error())
		return int32(msg_client_message.E_ERR_CHARGE_APPLE_PAY_RESULT_UNMARSHAL_FAILED), nil
	}

	if check_apple_order_exist(tmp_res.Receipt.TransactionId) {
		log.Error("Player[%v] apple transaction is[%v] already exists", player_id, tmp_res.Receipt.TransactionId)
		return int32(msg_client_message.E_ERR_CHARGE_APPLE_ORDER_ALREADY_EXIST), nil
	}

	return 1, tmp_res
}

func verify_apple_purchase_data(player *Player, bundle_id string, purchase_data []byte) int32 { //ordername, receipt string
	if purchase_data == nil || len(purchase_data) == 0 {
		log.Error("Player[%v] apple purchase data empty!")
		return int32(msg_client_message.E_ERR_CHARGE_APPLE_PURCHASE_DATA_EMPTY)
	}

	check_data := &AppleCheck{Receipt: string(purchase_data)}
	var data []byte
	var err error
	data, err = json.Marshal(check_data)
	if nil != err {
		log.Error("Player[%v] marshal apple pay verify purchase_data[%v] failed: %v", player.Id, purchase_data, err.Error())
		return int32(msg_client_message.E_ERR_CHARGE_APPLE_PURCHASE_DATA_INVALID)
	}

	if !atomic.CompareAndSwapInt32(&player.is_paying, 0, 1) {
		log.Error("Player[%v] is paying for apple purchase", player.Id)
		return int32(msg_client_message.E_ERR_CHARGE_PAY_REPEATED_VERIFY)
	}

	res, tmp_res := _send_apple_verify_url(player.Id, global_config.ApplePayUrl, data)
	if res < 0 {
		atomic.CompareAndSwapInt32(&player.is_paying, 1, 0)
		return res
	}

	// 发送到沙箱验证
	var is_sandbox bool
	if tmp_res.Status == 21007 {
		res, tmp_res = _send_apple_verify_url(player.Id, global_config.ApplePaySandBoxUrl, data)
		if res < 0 {
			atomic.CompareAndSwapInt32(&player.is_paying, 1, 0)
			return res
		}
		is_sandbox = true
	} else if 0 != tmp_res.Status {
		atomic.CompareAndSwapInt32(&player.is_paying, 1, 0)
		log.Error("Player[%v] apple pay verify Receipt check failed(%d) !", player.Id, tmp_res.Status)
		return int32(msg_client_message.E_ERR_CHARGE_APPLE_PAY_VERIFY_NO_PASS)
	}

	apple_pay_save(tmp_res.Receipt.TransactionId, bundle_id, player.Account, player)

	atomic.CompareAndSwapInt32(&player.is_paying, 1, 0)

	pay_item := pay_table_mgr.GetByBundle(bundle_id)
	if pay_item != nil && !is_sandbox {
		_post_talking_data(player.Account, "apple pay", config.ServerName, config.InnerVersion, "AppStore", tmp_res.Receipt.TransactionId, "ios", "charge", "success", player.db.Info.GetLvl(), pay_item.RecordGold, "USD", float64(pay_item.GemReward))
	}

	log.Trace("Player[%v] apple pay bunder_id[%v] verify success, purchase data %v", player.Id, bundle_id, string(purchase_data))

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
	Partner               string  `json:"partner"`
}

type TalkingDataStatus struct {
	MsgId string `json:"msgID"`
	Code  int32  `json:"code"`
	Msg   string `json:"msg"`
}

type TalkingDataResponse struct {
	Code       int32                `json:"code"`
	Msg        string               `json:"msg"`
	DataStatus []*TalkingDataStatus `json:"dataStatus"`
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
	now_time := time.Now()
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
		ChargeTime:            (now_time.Unix())*1000 + now_time.UnixNano()/(1000*1000),
		Partner:               partner,
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
