package main

import (
	"ih_server/libs/log"
	"ih_server/libs/utils"
)

type RedisGlobalData struct {
	inited     bool
	redis_conn *utils.RedisConn // redis连接
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
