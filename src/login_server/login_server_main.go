package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"ih_server/libs/log"
	"io/ioutil"
	"os"
)

type ServerConfig struct {
	ServerId       int32
	InnerVersion   string
	ServerName     string
	ListenClientIP string

	ListenMatchIP       string // 监听game_server连接
	MaxMatchConnections int32  // match_server最大连接数
	LogConfigDir        string // 日志配置文件路径

	CenterServerIP string // 连接AssistServer
	RedisServerIP  string // 连接redis
}

var config ServerConfig
var shutingdown bool

func main() {
	defer func() {
		log.Event("关闭服务器", nil)
		if err := recover(); err != nil {
			log.Stack(err)
		}
		server.Shutdown()
	}()

	config_file := "../run/ih_server/conf/login_server.json"
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
	err = json.Unmarshal(data, &config)
	if err != nil {
		fmt.Printf("解析配置文件失败 %v", err)
		return
	}

	server_list.ReadConfig("../run/ih_server/conf/server_list.json")

	// 加载日志配置
	log.Init("", config.LogConfigDir, true)
	log.Event("配置:服务器ID", config.ServerId)
	log.Event("配置:服务器名称", config.ServerName)
	log.Event("配置:服务器地址(对Client)", config.ListenClientIP)
	log.Event("配置:服务器地址(对Match)", config.ListenMatchIP)

	if !global_config_load() {
		log.Error("global_config_load failed !")
		return
	}

	server = new(LoginServer)
	if !server.Init() {
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
