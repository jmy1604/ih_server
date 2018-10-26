package main

import (
	"encoding/json"
	"ih_server/libs/log"
	"ih_server/libs/server_conn"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"
)

const (
	ACCOUNT_TOKEN_KEY = "ih:hall_server:account_token_key"
)

type RedisLoginTokenInfo struct {
	Token      string
	PlayerId   int32
	CreateTime int32
}

type LoginTokenInfo struct {
	acc          string
	token        string
	playerid     int32
	create_time  int32
	login_server *server_conn.ServerConn
}

type LoginTokenMgr struct {
	acc2token      map[string]*LoginTokenInfo
	acc2token_lock *sync.RWMutex

	id2acc      map[int32]string
	id2acc_lock *sync.RWMutex
}

var login_token_mgr LoginTokenMgr

func (this *LoginTokenMgr) Init() bool {
	this.acc2token = make(map[string]*LoginTokenInfo)
	this.acc2token_lock = &sync.RWMutex{}

	this.id2acc = make(map[int32]string)
	this.id2acc_lock = &sync.RWMutex{}
	return true
}

func (this *LoginTokenMgr) LoadRedisData() int32 {
	string_map, err := redis.StringMap(hall_server.redis_conn.Do("HGETALL", ACCOUNT_TOKEN_KEY))
	if err != nil {
		log.Error("redis获取集合[%v]数据失败[%v]", ACCOUNT_TOKEN_KEY, err.Error())
		return -1
	}

	for k, item := range string_map {
		jitem := &RedisLoginTokenInfo{}
		if err := json.Unmarshal([]byte(item), jitem); err != nil {
			log.Error("##### Load RedisLoginTokenInfo item[%v] error[%v]", item, err.Error())
			return -1
		}
		this.acc2token[k] = &LoginTokenInfo{
			acc:         k,
			token:       jitem.Token,
			create_time: jitem.CreateTime,
			playerid:    jitem.PlayerId,
		}
	}
	return 1
}

func (this *LoginTokenMgr) AddToAcc2Token(acc, token string, playerid int32, login_server *server_conn.ServerConn) {
	if "" == acc {
		log.Error("LoginTokenMgr AddToAcc2Token acc empty")
		return
	}

	this.acc2token_lock.Lock()
	defer this.acc2token_lock.Unlock()

	now_time := int32(time.Now().Unix())
	this.acc2token[acc] = &LoginTokenInfo{acc: acc, token: token, create_time: now_time, playerid: playerid, login_server: login_server}

	// serialize to redis
	item := &RedisLoginTokenInfo{
		Token:      token,
		CreateTime: now_time,
		PlayerId:   playerid,
	}
	bytes, err := json.Marshal(item)
	if err != nil {
		log.Error("##### Serialize item[%v] error[%v]", *item, err.Error())
		return
	}
	err = hall_server.redis_conn.Post("HSET", ACCOUNT_TOKEN_KEY, acc, string(bytes))
	if err != nil {
		log.Error("redis设置集合[%v]数据失败[%v]", ACCOUNT_TOKEN_KEY, err.Error())
		return
	}
}

func (this *LoginTokenMgr) RemoveFromAcc2Token(acc string) {
	if "" == acc {
		log.Error("LoginTokenMgr RemoveFromAcc2Token acc empty !")
		return
	}

	this.acc2token_lock.Lock()
	defer this.acc2token_lock.Unlock()

	if nil != this.acc2token[acc] {
		delete(this.acc2token, acc)
	}

	return
}

func (this *LoginTokenMgr) GetTokenByAcc(acc string) *LoginTokenInfo {
	if "" == acc {
		log.Error("LoginTokenMgr GetTockenByAcc acc empty")
		return nil
	}

	this.acc2token_lock.Lock()
	defer this.acc2token_lock.Unlock()

	return this.acc2token[acc]
}

func (this *LoginTokenMgr) GetLoginServerByAcc(acc string) *server_conn.ServerConn {
	this.acc2token_lock.RLock()
	defer this.acc2token_lock.RUnlock()

	item := this.acc2token[acc]
	if item == nil {
		return nil
	}
	return item.login_server
}

func (this *LoginTokenMgr) AddToId2Acc(playerid int32, acc string) {
	if "" == acc {
		log.Error("LoginTokenMgr AddToId2Acc acc empty !")
		return
	}

	this.id2acc_lock.Lock()
	defer this.id2acc_lock.Unlock()

	this.id2acc[playerid] = acc
	return
}

func (this *LoginTokenMgr) RemoveFromId2Acc(playerid int32) {
	this.id2acc_lock.Lock()
	defer this.id2acc_lock.Unlock()
	if "" != this.id2acc[playerid] {
		delete(this.id2acc, playerid)
	}

	return
}

func (this *LoginTokenMgr) GetAccById(playerid int32) string {
	this.id2acc_lock.RLock()
	defer this.id2acc_lock.RUnlock()

	return this.id2acc[playerid]
}
