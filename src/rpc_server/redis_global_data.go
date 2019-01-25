package main

import (
	"ih_server/libs/log"
	"ih_server/libs/utils"
)

type RedisGlobalData struct {
	inited      bool
	nick_id_set *NickIdSet
	redis_conn  *utils.RedisConn // redis连接
}

var global_data RedisGlobalData

func (this *RedisGlobalData) Init() bool {
	this.redis_conn = &utils.RedisConn{}
	if this.redis_conn == nil {
		log.Error("redis客户端未初始化")
		return false
	}

	if !this.redis_conn.Connect(config.RedisServerIP) {
		return false
	}

	// 昵称集合生成
	this.nick_id_set = &NickIdSet{}
	if !this.nick_id_set.Init() {
		return false
	}

	/*--------------- 排行榜 --------------*/
	// 关卡总分
	/*if this.LoadStageTotalScoreRankItems() < 0 {
		return false
	}*/
	// 关卡积分
	/*if this.LoadStageScoreRankItems() < 0 {
		return false
	}*/
	// 魅力值
	/*if this.LoadCharmRankItems() < 0 {
		return false
	}*/
	// 被赞
	/*if this.LoadZanedRankItems() < 0 {
		return false
	}*/
	/*--------------------------------------*/

	this.inited = true
	log.Info("全局数据GlobalData载入完成")
	return true
}

func (this *RedisGlobalData) Close() {
	this.redis_conn.Close()
}

func (this *RedisGlobalData) RunRedis() {
	this.redis_conn.Run(1000)
}

func (this *RedisGlobalData) AddIdNick(id int32, nick string) bool {
	if !this.nick_id_set.AddIdNick(id, nick) {
		return false
	}

	err := this.redis_conn.Post("HSET", ID_NICK_SET, id, nick)
	if err != nil {
		log.Error("redis增加集合[%v]数据[%v,%v]错误[%v]", ID_NICK_SET, id, nick, err.Error())
		return false
	}

	log.Debug("加入昵称[%v]成功", nick)

	return true
}

func (this *RedisGlobalData) RenameNick(player_id int32, new_nick string) int32 {
	errcode := this.nick_id_set.RenameNick(player_id, new_nick)
	if errcode < 0 {
		return errcode
	}

	/*err := this.redis_conn.Post("HDEL", ID_NICK_SET, player_id)
	if err != nil {
		log.Error("redis删除集合[%v]数据[%v]错误[%v]", ID_NICK_SET, old_nick, err.Error())
		return -1
	}*/
	err := this.redis_conn.Post("HSET", ID_NICK_SET, player_id, new_nick)
	if err != nil {
		log.Error("redis增加集合[%v]数据[%v,%v]错误[%v]", ID_NICK_SET, player_id, new_nick, err.Error())
		return -1
	}
	log.Info("修改昵称到[%v]成功", new_nick)
	return 1
}

func (this *RedisGlobalData) GetNickById(id int32) (nick string, ok bool) {
	return this.nick_id_set.GetNickById(id)
}

func (this *RedisGlobalData) GetIdsByNick(nick string) []int32 {
	return this.nick_id_set.GetIdsByNick(nick)
}
