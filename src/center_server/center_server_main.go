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

var dbc DBC
var dbc_account AccountDBC
var config server_config.CenterServerConfig
var shutingdown bool

func g_init() bool {
	/*err := dbc.Preload()
	if nil != err {
		log.Error("连接数据库预加载失败！！")
		return false
	} else {
		log.Event("数据库预加载成功!", nil)
	}

	err = dbc_account.Preload()
	if nil != err {
		log.Error("dbc_account Preload Failed !!")
		return false
	} else {
		log.Info("dbc_account Preload succeed !!")
	}*/

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

	/*log.Event("连接数据库", config.MYSQL_NAME, log.Property{"地址", config.MYSQL_IP})
	err = dbc.Conn(config.MYSQL_NAME, config.MYSQL_IP, config.MYSQL_ACCOUNT, config.MYSQL_PWD, config.MYSQL_COPY_PATH)
	if err != nil {
		log.Error("连接数据库失败 %v", err)
		return
	} else {
		log.Event("连接数据库成功", nil)
		go dbc.Loop()
	}

	err = dbc_account.Conn(config.MYSQL_NAME, config.MYSQL_IP, config.MYSQL_ACCOUNT, config.MYSQL_PWD, config.MYSQL_COPY_PATH)
	if err != nil {
		log.Error("连接账号数据库失败 %v", err)
		return
	} else {
		log.Event("连接账号数据库成功", nil)
		go dbc_account.Loop()
	}*/

	if !g_init() {
		return
	}

	log.Info("center server start ...\n")
	server.Start()
}
