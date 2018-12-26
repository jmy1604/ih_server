package main

import (
	"fmt"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"ih_server/src/share_data"
	"time"
)

var rpc_config server_config.RpcServerConfig
var server_list share_data.ServerList

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

	if !server_config.ServerConfigLoad("rpc_server.json", &rpc_config) {
		fmt.Printf("载入RPC Server配置失败")
		return
	}

	if !server_list.ReadConfig(server_config.GetConfPathFile("server_list.json")) {
		return
	}

	err := server.Init()
	if err != nil {
		log.Error("RPC Server init error[%v]", err.Error())
		return
	}

	go gm_service.StartHttp()

	fmt.Println("启动服务...")

	server.Start()

	fmt.Println("服务已停止!")
}
