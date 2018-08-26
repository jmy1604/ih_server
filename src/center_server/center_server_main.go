package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"ih_server/libs/log"
	"io/ioutil"
	"os"
	"time"
)

type ServerConfig struct {
	LogConfigPath     string // 日志配置文件地址
	ListenLoginIP     string // 监听LoginServer
	MaxLoginConntions int32  // 最大Login连接数
	ListenHallIP      string // 监听HallServer的IP
	MaxHallConntions  int32  // 最大Hall连接数
	GmIP              string // GM命令的地址
	HallServerCfgDir  string // 大厅配置文件地址

	MYSQL_NAME    string
	MYSQL_IP      string
	MYSQL_ACCOUNT string
	MYSQL_PWD     string
	DBCST_MIN     int
	DBCST_MAX     int

	MYSQL_COPY_PATH string
}

var dbc DBC
var dbc_account AccountDBC
var config ServerConfig
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

	/*if !hall_group_mgr.Init() {
		log.Error("hall_group_mgr init failed !")
		return false
	} else {
		log.Event("初始化：hall_group_mgr init succeed !", nil)
	}*/

	if !hall_agent_mgr.Init() {
		log.Error("hall_agent_mgr init failed")
		return false
	} else {
		log.Event("初始化：hall_agent_mgr init succeed !", nil)
	}
	go hall_agent_mgr.Start(config.ListenHallIP, config.MaxHallConntions)

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
	go login_agent_mgr.Start(config.ListenLoginIP, config.MaxLoginConntions)

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

	config_file := "../run/ih_server/conf/center_server.json"
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
	log.Init("", config.LogConfigPath, true)
	defer log.Close()

	log.Event("配置:监听LoginServer地址", config.ListenLoginIP)
	log.Event("配置:监听HallServer地址", config.ListenHallIP)
	log.Event("配置:日志配置目录", config.LogConfigPath)

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
