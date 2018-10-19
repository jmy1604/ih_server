package main

import (
	"fmt"
	"ih_server/libs/log"
	"ih_server/src/server_config"
)

var config server_config.LoginServerConfig
var shutingdown bool
var dbc DBC

func main() {
	defer func() {
		log.Event("关闭服务器", nil)
		if err := recover(); err != nil {
			log.Stack(err)
		}
		server.Shutdown()
		log.Close()
	}()

	if !server_config.ServerConfigLoad("login_server.json", &config) {
		fmt.Printf("载入LoginServer配置失败")
		return
	}

	log.Event("配置:服务器ID", config.ServerId)
	log.Event("配置:服务器名称", config.ServerName)
	log.Event("配置:服务器地址(对Client)", config.ListenClientIP)
	log.Event("配置:服务器地址(对Game)", config.ListenGameIP)

	server_list.ReadConfig(server_config.GetConfPathFile("server_list.json"))

	if !global_config_load() {
		log.Error("global_config_load failed !")
		return
	}

	log.Event("连接数据库", config.MYSQL_NAME, log.Property{"地址", config.MYSQL_IP})
	err := dbc.Conn(config.MYSQL_NAME, config.MYSQL_IP, config.MYSQL_ACCOUNT, config.MYSQL_PWD, "")
	if err != nil {
		log.Error("连接数据库失败 %v", err)
		return
	} else {
		log.Event("连接数据库成功", nil)
		go dbc.Loop()
	}

	if !signal_mgr.Init() {
		log.Error("signal_mgr init failed")
		return
	}

	if nil != dbc.Preload() {
		log.Error("dbc Preload Failed !!")
		return
	} else {
		log.Info("dbc Preload succeed !!")
	}

	server = new(LoginServer)
	if !server.Init() {
		return
	}

	if signal_mgr.IfClosing() {
		return
	}

	if !hall_agent_manager.Init() {
		return
	}

	center_conn.Init()
	go center_conn.Start()

	err = hall_agent_manager.Start()
	if err != nil {
		return
	}

	server.Start(true)
}
