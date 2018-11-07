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
	UID_TOKEN_KEY = "ih:hall_server:uid_token_key"
)

type RedisLoginTokenInfo struct {
	Token      string
	PlayerId   int32
	CreateTime int32
}

type LoginTokenInfo struct {
	account      string
	token        string
	playerid     int32
	create_time  int32
	login_server *server_conn.ServerConn
}

type LoginTokenMgr struct {
	uid2token        map[string]*LoginTokenInfo
	uid2token_locker *sync.RWMutex

	//acc2token      map[string]*LoginTokenInfo
	//acc2token_lock *sync.RWMutex

	id2uid        map[int32]string
	id2uid_locker *sync.RWMutex
}

var login_token_mgr LoginTokenMgr

func (this *LoginTokenMgr) Init() bool {
	this.uid2token = make(map[string]*LoginTokenInfo)
	this.uid2token_locker = &sync.RWMutex{}

	//this.acc2token = make(map[string]*LoginTokenInfo)
	//this.acc2token_lock = &sync.RWMutex{}

	this.id2uid = make(map[int32]string)
	this.id2uid_locker = &sync.RWMutex{}
	return true
}

func (this *LoginTokenMgr) LoadRedisData() int32 {
	string_map, err := redis.StringMap(hall_server.redis_conn.Do("HGETALL", UID_TOKEN_KEY))
	if err != nil {
		log.Error("redis获取集合[%v]数据失败[%v]", UID_TOKEN_KEY, err.Error())
		return -1
	}

	for k, item := range string_map {
		jitem := &RedisLoginTokenInfo{}
		if err := json.Unmarshal([]byte(item), jitem); err != nil {
			log.Error("##### Load RedisLoginTokenInfo item[%v] error[%v]", item, err.Error())
			return -1
		}
		this.uid2token[k] = &LoginTokenInfo{
			token:       jitem.Token,
			create_time: jitem.CreateTime,
			playerid:    jitem.PlayerId,
		}
	}
	return 1
}

func (this *LoginTokenMgr) AddToUid2Token(uid, acc, token string, playerid int32, login_server *server_conn.ServerConn) {
	if uid == "" || acc == "" || token == "" {
		log.Error("LoginTokenMgr AddToUid2Token uid or acc or token empty")
		return
	}

	this.uid2token_locker.Lock()
	defer this.uid2token_locker.Unlock()

	now_time := int32(time.Now().Unix())
	this.uid2token[uid] = &LoginTokenInfo{account: acc, token: token, create_time: now_time, playerid: playerid, login_server: login_server}

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
	err = hall_server.redis_conn.Post("HSET", UID_TOKEN_KEY, uid, string(bytes))
	if err != nil {
		log.Error("redis设置集合[%v]数据失败[%v]", UID_TOKEN_KEY, err.Error())
		return
	}
}

func (this *LoginTokenMgr) BindNewAccount(uid, acc, new_acc string) bool {
	this.uid2token_locker.Lock()
	defer this.uid2token_locker.Unlock()

	token_info := this.uid2token[uid]
	if token_info.account != acc {
		log.Error("Bind New Account for old account %v invalid", acc)
		return false
	}

	token_info.account = new_acc

	return true
}

func (this *LoginTokenMgr) RemoveFromUid2Token(uid string) {
	if "" == uid {
		log.Error("LoginTokenMgr RemoveFromUid2Token uid empty !")
		return
	}

	this.uid2token_locker.Lock()
	defer this.uid2token_locker.Unlock()

	if nil != this.uid2token[uid] {
		delete(this.uid2token, uid)
	}

	return
}

func (this *LoginTokenMgr) GetTokenByUid(uid string) *LoginTokenInfo {
	if "" == uid {
		log.Error("LoginTokenMgr GetTockenByUid uid empty")
		return nil
	}

	this.uid2token_locker.Lock()
	defer this.uid2token_locker.Unlock()

	return this.uid2token[uid]
}

func (this *LoginTokenMgr) GetLoginServerByAcc(uid string) *server_conn.ServerConn {
	this.uid2token_locker.RLock()
	defer this.uid2token_locker.RUnlock()

	item := this.uid2token[uid]
	if item == nil {
		return nil
	}
	return item.login_server
}

func (this *LoginTokenMgr) AddToId2Uid(playerid int32, uid string) {
	if "" == uid {
		log.Error("LoginTokenMgr AddToId2Uid uid empty !")
		return
	}

	this.id2uid_locker.Lock()
	defer this.id2uid_locker.Unlock()

	this.id2uid[playerid] = uid
	return
}

func (this *LoginTokenMgr) RemoveFromId2Uid(playerid int32) {
	this.id2uid_locker.Lock()
	defer this.id2uid_locker.Unlock()

	if "" != this.id2uid[playerid] {
		delete(this.id2uid, playerid)
	}

	return
}

func (this *LoginTokenMgr) GetUidById(playerid int32) string {
	this.id2uid_locker.RLock()
	defer this.id2uid_locker.RUnlock()

	return this.id2uid[playerid]
}
