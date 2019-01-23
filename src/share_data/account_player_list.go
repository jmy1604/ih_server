package share_data

import (
	"ih_server/libs/log"
	"ih_server/libs/utils"
	"sync"

	"ih_server/proto/gen_go/client_message"

	"github.com/golang/protobuf/proto"
	"github.com/gomodule/redigo/redis"
)

const (
	ACCOUNT_PLAYER_LIST_KEY = "ih:share_data:account_player_list"
)

type PlayerList struct {
	player_list        []*msg_client_message.AccountPlayerInfo
	player_list_locker *sync.RWMutex
}

func (this *PlayerList) GetList() []*msg_client_message.AccountPlayerInfo {
	this.player_list_locker.RLock()
	defer this.player_list_locker.RUnlock()
	return this.player_list
}

var player_list_map map[string]*PlayerList
var player_list_map_locker *sync.RWMutex

func AccountPlayerListInit() {
	player_list_map = make(map[string]*PlayerList)
	player_list_map_locker = &sync.RWMutex{}
}

func LoadAccountsPlayerList(redis_conn *utils.RedisConn) bool {
	AccountPlayerListInit()

	var values map[string]string
	values, err := redis.StringMap(redis_conn.Do("HGETALL", ACCOUNT_PLAYER_LIST_KEY))
	if err != nil {
		log.Error("redis get hashset %v all data err %v", ACCOUNT_PLAYER_LIST_KEY, err.Error())
		return false
	}
	var msg msg_client_message.S2CAccountPlayerListResponse
	for k, v := range values {
		err = proto.Unmarshal([]byte(v), &msg)
		if err != nil {
			log.Error("account %v S2CAccountPlayerListResponse data unmarshal err %v", k, err.Error())
			return false
		}

		pl := &PlayerList{
			player_list:        msg.InfoList,
			player_list_locker: &sync.RWMutex{},
		}
		player_list_map[k] = pl
	}

	return true
}

func LoadAccountPlayerList(redis_conn *utils.RedisConn, account string) bool {
	data, err := redis.Bytes(redis_conn.Do("HGET", ACCOUNT_PLAYER_LIST_KEY, account))
	if err != nil {
		log.Error("redis get hashset %v with key %v err %v", ACCOUNT_PLAYER_LIST_KEY, account, err.Error())
		return false
	}

	var msg msg_client_message.S2CAccountPlayerListResponse
	err = proto.Unmarshal(data, &msg)
	if err != nil {
		log.Error("account %v S2CAccountPlayerListResponse data unmarshal err %v", account, err.Error())
		return false
	}

	player_list_map_locker.Lock()
	pl := &PlayerList{
		player_list:        msg.InfoList,
		player_list_locker: &sync.RWMutex{},
	}
	player_list_map[account] = pl
	player_list_map_locker.Unlock()

	return true
}

func AddAccountPlayerInfo(redis_conn *utils.RedisConn, account string, info *msg_client_message.AccountPlayerInfo) {
	player_list_map_locker.RLock()
	pl := player_list_map[account]
	player_list_map_locker.RUnlock()
	if pl == nil {
		LoadAccountPlayerList(redis_conn, account)
	}
	SaveAccountPlayerInfo(redis_conn, account, info)
}

func SaveAccountPlayerInfo(redis_conn *utils.RedisConn, account string, info *msg_client_message.AccountPlayerInfo) {
	player_list_map_locker.RLock()
	player_list := player_list_map[account]
	player_list_map_locker.RUnlock()

	if player_list == nil {
		player_list_map_locker.Lock()
		player_list = player_list_map[account]
		if player_list != nil {
			return
		}
		player_list = &PlayerList{
			player_list:        []*msg_client_message.AccountPlayerInfo{info},
			player_list_locker: &sync.RWMutex{},
		}
		player_list_map[account] = player_list
		player_list_map_locker.Unlock()
	} else {
		i := 0
		for ; i < len(player_list.player_list); i++ {
			if player_list.player_list[i] == nil {
				continue
			}
			if player_list.player_list[i].GetServerId() == info.GetServerId() {
				player_list.player_list[i].PlayerName = info.GetPlayerName()
				player_list.player_list[i].PlayerLevel = info.GetPlayerLevel()
				player_list.player_list[i].PlayerHead = info.GetPlayerHead()
				break
			}
		}
		if i >= len(player_list.player_list) {
			player_list.player_list = append(player_list.player_list, info)
			player_list_map[account] = player_list
		}
	}

	var msg msg_client_message.S2CAccountPlayerListResponse
	msg.InfoList = player_list.player_list
	data, err := proto.Marshal(&msg)
	if err != nil {
		log.Error("redis marshal account %v info err %v", account, err.Error())
		return
	}
	err = redis_conn.Post("HSET", ACCOUNT_PLAYER_LIST_KEY, account, data)
	if err != nil {
		log.Error("redis set hashset %v key %v data %v err %v", ACCOUNT_PLAYER_LIST_KEY, account, data, err.Error())
		return
	}
}

func GetAccountPlayerList(account string) []*msg_client_message.AccountPlayerInfo {
	player_list_map_locker.RLock()
	defer player_list_map_locker.RUnlock()
	pl := player_list_map[account]
	if pl == nil {
		return nil
	}
	return pl.GetList()
}

func GetAccountPlayer(account string, server_id int32) *msg_client_message.AccountPlayerInfo {
	player_list := GetAccountPlayerList(account)
	if player_list == nil {
		return nil
	}
	for _, p := range player_list {
		if p.GetServerId() == server_id {
			return p
		}
	}
	return nil
}
