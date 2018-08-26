package main

import (
	"ih_server/libs/log"
	"strconv"
	"sync"

	"github.com/gomodule/redigo/redis"
)

//const NICK_ID_SET = "mm:nick_id_set"
const ID_NICK_SET = "mm:id_nick_set"

type NickIdSet struct {
	//nick2id  map[string]int32
	id2nick    map[int32]string
	nick2ids   map[string][]int32
	string2ids map[string][]int32
	mtx        *sync.RWMutex
}

func (this *NickIdSet) Init() bool {
	//this.nick2id = make(map[string]int32)
	this.id2nick = make(map[int32]string)
	this.nick2ids = make(map[string][]int32)
	this.string2ids = make(map[string][]int32)
	this.mtx = &sync.RWMutex{}

	/*int_map, err := redis.IntMap(global_data.redis_conn.Do("HGETALL", NICK_ID_SET))
	if err != nil {
		log.Error("redis获取集合[%v]数据失败[%v]", NICK_ID_SET, err.Error())
		return false
	}

	for s, id := range int_map {
		if !this.AddNickId(s, int32(id)) {
			log.Warn("载入集合[%v]数据[%v,%v]失败", NICK_ID_SET, s, id)
		}
	}*/

	string_map, err := redis.StringMap(global_data.redis_conn.Do("HGETALL", ID_NICK_SET))
	if err != nil {
		log.Error("redis获取集合[%v]数据失败[%v]", ID_NICK_SET, err.Error())
		return false
	}

	for id_str, nick := range string_map {
		id, err := strconv.Atoi(id_str)
		if err != nil {
			log.Error("transfer set[%v] data[%v] to id error[%v]", ID_NICK_SET, id_str, err.Error())
			continue
		}
		if !this.AddIdNick(int32(id), nick) {
			log.Warn("Load set[%v] data[%v,%v] failed", ID_NICK_SET, id, nick)
		}

	}

	return true
}

func (this *NickIdSet) add_id_nick(id int32, nick string) bool {
	if _, o := this.id2nick[id]; o {
		log.Warn("Player Id[%v] already exists in [%v] set", id, ID_NICK_SET)
		return false
	}

	this.id2nick[id] = nick

	ids := this.nick2ids[nick]
	if ids == nil {
		this.nick2ids[nick] = []int32{id}
	} else {
		this.nick2ids[nick] = append(ids, id)
	}

	log.Debug("Add id,nick [%v,%v] in [%v] set", id, nick, ID_NICK_SET)

	return true
}

func (this *NickIdSet) AddIdNick(id int32, nick string) bool {
	this.mtx.Lock()
	defer this.mtx.Unlock()
	return this.add_id_nick(id, nick)
}

func (this *NickIdSet) remove_id_nick(id int32, nick string) bool {
	nick_ids, o := this.nick2ids[nick]
	if !o {
		return false
	}

	if nick_ids != nil {
		l := len(nick_ids)
		i := 0
		for ; i < l; i++ {
			if nick_ids[i] == id {
				break
			}
		}
		if i < l {
			for n := i; n < l-1; n++ {
				nick_ids[n] = nick_ids[n+1]
			}
			nick_ids = nick_ids[:l-1]
			this.nick2ids[nick] = nick_ids
		}
	}

	return true
}

func (this *NickIdSet) remove_id(id int32) bool {
	nick, o := this.id2nick[id]
	if !o {
		return false
	}
	if !this.remove_id_nick(id, nick) {
		return false
	}
	delete(this.id2nick, id)
	return true
}

func (this *NickIdSet) RemoveId(id int32) bool {
	this.mtx.Lock()
	defer this.mtx.Unlock()
	return this.remove_id(id)
}

func (this *NickIdSet) has_nick(nick string) bool {
	if _, o := this.nick2ids[nick]; !o {
		return false
	}
	return true
}

func (this *NickIdSet) HasNick(nick string) bool {
	this.mtx.RLock()
	defer this.mtx.RUnlock()
	return this.has_nick(nick)
}

func (this *NickIdSet) has_id(id int32) bool {
	if _, o := this.id2nick[id]; !o {
		return false
	}
	return true
}

func (this *NickIdSet) HasId(id int32) bool {
	this.mtx.RLock()
	defer this.mtx.RUnlock()
	return this.has_id(id)
}

func (this *NickIdSet) RenameNick(id int32, new_nick string) int32 {
	/*if old_nick == new_nick {
		return int32(msg_client_message.E_ERR_PLAYER_RENAME_NEW_CANT_SAME_TO_OLD)
	}*/

	if new_nick == "" {
		return int32( /*msg_client_message.E_ERR_PLAYER_RENAME_NEW_CANT_EMPTY*/ -1)
	}

	this.mtx.Lock()
	defer this.mtx.Unlock()

	if !this.remove_id(id) {
		log.Warn("Player[%v] no id nick pair, remove failed", id)
		//return int32(msg_client_message.E_ERR_PLAYER_RENAME_REMOVE_OLD_FAILED)
	}

	ids, o := this.nick2ids[new_nick]
	if !o {
		this.nick2ids[new_nick] = []int32{id}
	} else {
		ids = append(ids, id)
		this.nick2ids[new_nick] = ids
	}

	this.id2nick[id] = new_nick

	return 1
}

func (this *NickIdSet) GetNickById(id int32) (nick string, ok bool) {
	this.mtx.RLock()
	defer this.mtx.RUnlock()

	nick, ok = this.id2nick[id]
	return
}

func (this *NickIdSet) GetIdsByNick(nick string) []int32 {
	this.mtx.RLock()
	defer this.mtx.RUnlock()

	return this.nick2ids[nick]
}
