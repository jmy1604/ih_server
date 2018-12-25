package main

import (
	"fmt"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"net/http"
	_ "net/http/pprof"
	"runtime/debug"
	"time"
)

var config server_config.GameServerConfig
var shutingdown bool
var dbc DBC

func after_center_match_conn() {
	if signal_mgr.IfClosing() {
		return
	}
}

func main() {
	defer func() {
		log.Event("关闭服务器", nil)
		if err := recover(); err != nil {
			log.Stack(err)
			debug.PrintStack()
		}
		time.Sleep(time.Second * 5)
		hall_server.Shutdown()
	}()

	if !server_config.ServerConfigLoad("hall_server.json", &config) {
		fmt.Printf("载入GameServer配置失败")
		return
	}

	log.Event("配置:服务器监听客户端地址", config.ListenClientInIP)
	log.Event("配置:最大客户端连接数)", config.MaxClientConnections)
	log.Event("连接数据库", config.MYSQL_NAME, log.Property{"地址", config.MYSQL_IP})
	err := dbc.Conn(config.MYSQL_NAME, config.MYSQL_IP, config.MYSQL_ACCOUNT, config.MYSQL_PWD, config.MYSQL_COPY_PATH)
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

	// 配置加载
	if !global_config.Init("global.json") {
		log.Error("global_config_load failed !")
		return
	} else {
		log.Info("global_config_load succeed !")
	}

	if !msg_handler_mgr.Init() {
		log.Error("msg_handler_mgr init failed !")
		return
	} else {
		log.Info("msg_handler_mgr init succeed !")
	}

	if !player_mgr.Init() {
		log.Error("player_mgr init failed !")
		return
	} else {
		log.Info("player_mgr init succeed !")
	}

	if !login_token_mgr.Init() {
		log.Error("启动login_token_mgr失败")
		return
	}

	if err := table_init(); err != nil {
		log.Error("%v", err.Error())
		return
	}

	// pprof
	go func() {
		http.ListenAndServe("0.0.0.0:6060", nil)
	}()

	// 排行榜
	rank_list_mgr.Init()

	// 好友推荐
	friend_recommend_mgr.Init()

	// 月卡管理
	charge_month_card_manager.Init()

	if nil != dbc.Preload() {
		log.Error("dbc Preload Failed !!")
		return
	} else {
		log.Info("dbc Preload succeed !!")
	}

	if !login_conn_mgr.Init() {
		log.Error("login_conn_mgr init failed")
		return
	}

	// 初始化CenterServer
	center_conn.Init()

	// 初始化大厅
	if !hall_server.Init() {
		log.Error("hall_server init failed !")
		return
	} else {
		log.Info("hall_server init succeed !")
	}

	if signal_mgr.IfClosing() {
		return
	}

	// 连接CenterServer
	log.Info("连接中心服务器！！")
	go center_conn.Start()
	center_conn.WaitConnectFinished()

	after_center_match_conn()

	hall_server.Start(config.UseHttps)
}
