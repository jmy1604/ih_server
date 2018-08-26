package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"ih_server/libs/log"
	"io/ioutil"
	"os"
)

type ClientConfig struct {
	MatchServerIP     string
	LogConfigDir      string
	LoginUrl          string
	SelectServerUrl   string
	AccountPrefix     string
	AccountStartIndex int32
	AccountNum        int32
	UseHttps          bool
}

var config ClientConfig
var shutingdown bool

func main() {
	defer func() {
		log.Event("关闭测试客户端", nil)
		if err := recover(); err != nil {
			log.Stack(err)
		}
		test_client.Shutdown()
	}()

	config_file := "../conf/test_client.json"
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

	// 加载日志配置
	log.Init("", config.LogConfigDir, true)
	log.Event("配置:匹配服务器IP", config.MatchServerIP)

	msg_handler_mgr.Init()

	hall_conn_mgr.Init()

	if !test_client.Init() {
		return
	}

	test_client.Start()
}
