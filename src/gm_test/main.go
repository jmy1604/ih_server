package main

import (
	"fmt"
	"ih_server/libs/log"
	"ih_server/src/server_config"
)

var config server_config.GmTestConfig
var gm_test GmTest

func main() {
	defer func() {
		log.Event("关闭测试客户端", nil)
		if err := recover(); err != nil {
			log.Stack(err)
		}
		gm_test.Shutdown()
		log.Close()
	}()

	if !server_config.ServerConfigLoad("gm_test.json", &config) {
		fmt.Printf("载入GmTest配置失败")
		return
	}

	//msg_handler_mgr.Init()

	if !gm_test.Init() {
		return
	}

	gm_test.Start()
}
