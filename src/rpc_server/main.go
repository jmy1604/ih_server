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

	/*config_file := "../run/ih_server/conf/rpc_server.json"
	if len(os.Args) > 1 {
		arg_config_file := flag.String("f", "", "config file path")
		if nil != arg_config_file && "" != *arg_config_file {
			flag.Parse()
			fmt.Printf("配置参数 %v", *arg_config_file)
			config_file = *arg_config_file
		}
	}

	data, err := ioutil.ReadFile(config_file)
	if err != nil {
		fmt.Printf("读取配置文件失败 %v", err)
		return
	}

	err = json.Unmarshal(data, &rpc_config)
	if err != nil {
		fmt.Printf("解析配置文件失败 %v", err)
		return
	}

	// 加载日志配置
	log.Init("", rpc_config.LogConfigPath, true)*/

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
