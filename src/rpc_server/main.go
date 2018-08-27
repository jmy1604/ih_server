package main

import (
	_ "encoding/json"
	_ "flag"
	"fmt"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	_ "io/ioutil"
	_ "os"
	"time"
)

var rpc_config server_config.RpcServerConfig

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

	err := server.Init()
	if err != nil {
		log.Error("RPC Server init error[%v]", err.Error())
		return
	}

	fmt.Println("启动服务...")

	server.Start()

	fmt.Println("服务已停止!")
}
