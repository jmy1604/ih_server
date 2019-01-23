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
	UID_PLAYER_LIST_KEY = "ih:share_data:uid_player_list"
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

func UidPlayerListInit() {
	player_list_map = make(map[string]*PlayerList)
	player_list_map_locker = &sync.RWMutex{}
}

func LoadUidsPlayerList(redis_conn *utils.RedisConn) bool {
	UidPlayerListInit()

	var values map[string]string
	values, err := redis.StringMap(redis_conn.Do("HGETALL", UID_PLAYER_LIST_KEY))
	if err != nil {
		log.Error("redis get hashset %v all data err %v", UID_PLAYER_LIST_KEY, err.Error())
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

func LoadUidPlayerList(redis_conn *utils.RedisConn, uid string) bool {
	data, err := redis.Bytes(redis_conn.Do("HGET", UID_PLAYER_LIST_KEY, uid))
	if err != nil {
		log.Error("redis get hashset %v with key %v err %v", UID_PLAYER_LIST_KEY, uid, err.Error())
		return false
	}

	var msg msg_client_message.S2CAccountPlayerListResponse
	err = proto.Unmarshal(data, &msg)
	if err != nil {
		log.Error("uid %v S2CAccountPlayerListResponse data unmarshal err %v", uid, err.Error())
		return false
	}

	player_list_map_locker.Lock()
	pl := &PlayerList{
		player_list:        msg.InfoList,
		player_list_locker: &sync.RWMutex{},
	}
	player_list_map[uid] = pl
	player_list_map_locker.Unlock()

	return true
}

func AddUidPlayerInfo(redis_conn *utils.RedisConn, uid string, info *msg_client_message.AccountPlayerInfo) {
	player_list_map_locker.RLock()
	pl := player_list_map[uid]
	player_list_map_locker.RUnlock()
	if pl == nil {
		LoadUidPlayerList(redis_conn, uid)
	}
	SaveUidPlayerInfo(redis_conn, uid, info)
}

func SaveUidPlayerInfo(redis_conn *utils.RedisConn, uid string, info *msg_client_message.AccountPlayerInfo) {
	player_list_map_locker.RLock()
	player_list := player_list_map[uid]
	player_list_map_locker.RUnlock()

	if player_list == nil {
		player_list_map_locker.Lock()
		player_list = player_list_map[uid]
		// double check
		if player_list == nil {
			player_list = &PlayerList{
				player_list_locker: &sync.RWMutex{},
			}
			player_list_map[uid] = player_list
		}
		player_list_map_locker.Unlock()
	}

	i := 0
	for ; i < len(player_list.player_list); i++ {
		p := player_list.player_list[i]
		if p == nil {
			continue
		}
		if p.GetServerId() == info.GetServerId() {
			p.PlayerName = info.GetPlayerName()
			p.PlayerLevel = info.GetPlayerLevel()
			p.PlayerHead = info.GetPlayerHead()
			break
		}
	}
	if i >= len(player_list.player_list) {
		player_list.player_list = append(player_list.player_list, info)
		player_list_map_locker.Lock()
		player_list_map[uid] = player_list
		player_list_map_locker.Unlock()
	}

	var msg msg_client_message.S2CAccountPlayerListResponse
	msg.InfoList = player_list.player_list
	data, err := proto.Marshal(&msg)
	if err != nil {
		log.Error("redis marshal account %v info err %v", uid, err.Error())
		return
	}
	err = redis_conn.Post("HSET", UID_PLAYER_LIST_KEY, uid, data)
	if err != nil {
		log.Error("redis set hashset %v key %v data %v err %v", UID_PLAYER_LIST_KEY, uid, data, err.Error())
		return
	}
}

func GetUidPlayerList(uid string) []*msg_client_message.AccountPlayerInfo {
	player_list_map_locker.RLock()
	defer player_list_map_locker.RUnlock()
	pl := player_list_map[uid]
	if pl == nil {
		return nil
	}
	return pl.GetList()
}

func GetUidPlayer(uid string, server_id int32) *msg_client_message.AccountPlayerInfo {
	player_list := GetUidPlayerList(uid)
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
