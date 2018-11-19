package main

import (
	_ "encoding/json"
	_ "flag"
	_ "fmt"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	_ "io/ioutil"
	_ "os"
	"time"
)

var config server_config.CenterServerConfig
var shutingdown bool

func g_init() bool {
	if !signal_mgr.Init() {
		log.Error("signal mgr init failed !")
		return false
	} else {
		log.Event("初始化：signal mgr init succeed ！", nil)
	}

	if !hall_agent_mgr.Init() {
		log.Error("hall_agent_mgr init failed")
		return false
	} else {
		log.Event("初始化：hall_agent_mgr init succeed !", nil)
	}
	go hall_agent_mgr.Start(config.ListenGameIP, config.MaxGameConnections)

	if !login_info_mgr.Init() {
		log.Error("login_info_mgr Init failed !")
		return false
	} else {
		log.Event("初始化:login_info_mgr init succeed !", nil)
	}

	if !login_agent_mgr.Init() {
		log.Error("login_agent_mgr init failed")
		return false
	} else {
		log.Event("初始化:login_agent_mgr init succeed !", nil)
	}
	go login_agent_mgr.Start(config.ListenLoginIP, config.MaxLoginConnections)

	if nil != server.Init() {
		log.Error("server init failed\n")
		return false
	} else {
		log.Event("初始化:server init succeed !", nil)
	}

	return true
}

func main() {
	defer func() {
		log.Event("关闭center_server服务器", nil)
		if err := recover(); err != nil {
			log.Stack(err)
		}
		server.Shutdown()
		log.Close()
		time.Sleep(time.Second * 5)
	}()

	if !server_config.ServerConfigLoad("center_server.json", &config) {
		log.Error("载入CenterServer配置失败")
		return
	}

	log.Event("配置:监听LoginServer地址", config.ListenLoginIP)
	log.Event("配置:监听GameServer地址", config.ListenGameIP)
	log.Event("配置:日志配置文件", config.LogConfigFile)

	if !g_init() {
		return
	}

	log.Info("center server start ...\n")
	server.Start()
}
