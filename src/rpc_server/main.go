package main

import (
	"fmt"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"ih_server/src/share_data"
	"time"
)

var config server_config.RpcServerConfig
var server_list share_data.ServerList
var dbc DBC

func main() {
	defer func() {
		log.Event("关闭rpc_server服务器", nil)
		if err := recover(); err != nil {
			log.Stack(err)
		}
		server.Shutdown()
		time.Sleep(time.Second * 5)
		log.Close()
	}()

	if !server_config.ServerConfigLoad("rpc_server.json", &config) {
		fmt.Printf("载入RPC Server配置失败")
		return
	}

	if !server_list.ReadConfig(server_config.GetConfPathFile("server_list.json")) {
		return
	}

	var err error
	if config.MYSQL_NAME != "" {
		log.Event("连接数据库", config.MYSQL_NAME, log.Property{"地址", config.MYSQL_IP})
		err = dbc.Conn(config.MYSQL_NAME, config.MYSQL_IP, config.MYSQL_ACCOUNT, config.MYSQL_PWD, "")
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
	}

	err = server.Init()
	if err != nil {
		log.Error("RPC Server init error[%v]", err.Error())
		return
	}

	if config.MYSQL_NAME != "" {
		if signal_mgr.IfClosing() {
			return
		}
	}

	if config.GmServerUseHttps {
		go gm_service.StartHttps(server_config.GetConfPathFile("server.crt"), server_config.GetConfPathFile("server.key"))
	} else {
		go gm_service.StartHttp()
	}

	fmt.Println("启动服务...")

	server.Start()

	fmt.Println("服务已停止!")
}
