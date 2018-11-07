package share_data

import (
	"encoding/base64"
	"encoding/json"
	"ih_server/libs/log"

	"time"

	_ "github.com/gomodule/redigo/redis"
)

const (
	DEFAULT_ACCESS_TOKEN_EXIST_SECONDS = 24 * 3600
)

type AccessTokenInfo struct {
	UniqueId      string
	ExpireSeconds int32
}

func (this *AccessTokenInfo) Init(unique_id string, exist_seconds int32) {
	this.UniqueId = unique_id
	now_time := int32(time.Now().Unix())
	this.ExpireSeconds = now_time + exist_seconds
}

func (this *AccessTokenInfo) GetString() (bool, string) {
	bytes, err := json.Marshal(this)
	if err != nil {
		log.Error("Access Token marshal json data err %v", err.Error())
		return false, ""
	}
	return true, base64.StdEncoding.EncodeToString(bytes)
}

func (this *AccessTokenInfo) ParseString(token_string string) bool {
	bytes, err := base64.StdEncoding.DecodeString(token_string)
	if err != nil {
		log.Error("Access token decode token string %v err %v", token_string, err.Error())
		return false
	}
	err = json.Unmarshal(bytes, this)
	if err != nil {
		log.Error("Access token decode json data err %v", err.Error())
		return false
	}
	return true
}

func (this *AccessTokenInfo) IsExpired() bool {
	now_time := int32(time.Now().Unix())
	if now_time >= this.ExpireSeconds {
		return true
	}
	return false
}

func GenerateAccessToken(unique_id string) string {
	var token AccessTokenInfo
	token.Init(unique_id, DEFAULT_ACCESS_TOKEN_EXIST_SECONDS)
	b, token_string := token.GetString()
	if !b {
		return ""
	}
	return token_string
}
