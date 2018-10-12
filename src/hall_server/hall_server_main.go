package main

import (
	"errors"
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

func table_init() error {
	if !position_table.Init("") {
		return errors.New("positioin_table init failed")
	} else {
		log.Info("position_table init succeed")
	}

	if !card_table_mgr.Init("") {
		log.Error("card_table_mgr init failed")
		return errors.New("card_table_mgr init failed")
	} else {
		log.Info("card_table_mgr init succeed")
	}

	if !skill_table_mgr.Init("") {
		log.Error("skill_table_mgr init failed")
		return errors.New("skill_table_mgr init failed")
	} else {
		log.Info("skill_table_mgr init succeed")
	}

	if !buff_table_mgr.Init("") {
		log.Error("buff_table_mgr init failed")
		return errors.New("buff_table_mgr init failed")
	} else {
		log.Info("buff_table_mgr init succeed")
	}

	if !stage_table_mgr.Init("") {
		log.Error("stage_table_mgr init failed")
		return errors.New("stage_table_mgr init failed")
	} else {
		log.Info("stage_table_mgr init succeed")
	}

	if !item_table_mgr.Init("") {
		log.Error("item_table_mgr init failed")
		return errors.New("item_table_mgr init failed")
	} else {
		log.Info("item_table_mgr init succeed")
	}

	if !campaign_table_mgr.Init("") {
		log.Error("campaign_table_mgr init failed")
		return errors.New("campaign_table_mgr init failed")
	} else {
		log.Info("campaign_table_gmr init succeed")
	}

	if !drop_table_mgr.Init("") {
		log.Error("drop_table_mgr init failed")
		return errors.New("drop_table_mgr init failed")
	} else {
		log.Info("drop_table_mgr init succeed")
	}

	if !shop_table_mgr.Init("") {
		log.Error("shop_table_mgr init failed")
		return errors.New("shop_table_mgr init failed")
	} else {
		log.Info("shop_table_mgr init success")
	}

	if !levelup_table_mgr.Init("") {
		log.Error("levelup_table_mgr init failed")
		return errors.New("levelup_table_mgr init failed")
	} else {
		log.Info("levelup_table_mgr init succeed")
	}

	if !rankup_table_mgr.Init("") {
		log.Error("rankup_table_mgr init failed")
		return errors.New("rankup_table_mgr init failed")
	} else {
		log.Info("rankup_table_mgr init succeed")
	}

	if !fusion_table_mgr.Init("") {
		log.Error("fusion_table_mgr init failed")
		return errors.New("fusion_table_mgr init failed")
	} else {
		log.Info("fusion_table_mgr init succeed")
	}

	if !talent_table_mgr.Init("") {
		log.Error("talent_table_mgr init failed")
		return errors.New("talent_table_mgr init failed")
	} else {
		log.Info("talent_table_mgr init success")
	}

	if !tower_table_mgr.Init("") {
		log.Error("tower_table_mgr init failed")
		return errors.New("tower_table_mgr init failed")
	} else {
		log.Info("tower_table_mgr init success")
	}

	if !item_upgrade_table_mgr.Init("") {
		log.Error("item_upgrade_table_mgr init failed")
		return errors.New("item_upgrade_table_mgr init failed")
	} else {
		log.Info("item_upgrade_table_mgr init success")
	}

	if !suit_table_mgr.Init("") {
		log.Error("suit_table_mgr init failed")
		return errors.New("suit_table_mgr init failed")
	} else {
		log.Info("suit_table_mgr init success")
	}

	if !draw_table_mgr.Init("") {
		log.Error("draw_table_mgr init failed")
		return errors.New("draw_table_mgr init failed")
	} else {
		log.Info("draw_table_mgr init success")
	}

	if !goldhand_table_mgr.Init("") {
		log.Error("goldhand_table_mgr init failed")
		return errors.New("goldhand_table_mgr init failed")
	} else {
		log.Info("goldhand_table_mgr init success")
	}

	if !shopitem_table_mgr.Init("") {
		log.Error("shopitem_table_mgr init failed")
		return errors.New("shopitem_table_mgr init failed")
	} else {
		log.Info("shopitem_table_mgr init success")
	}

	if !arena_bonus_table_mgr.Init("") {
		log.Error("arena_bonus_table_mgr init failed")
		return errors.New("arena_bonus_table_mgr init failed")
	} else {
		log.Info("arena_bonus_table_mgr init success")
	}

	if !arena_division_table_mgr.Init("") {
		log.Error("arena_division_table_mgr init failed")
		return errors.New("arena_division_table_mgr init failed")
	} else {
		log.Info("arena_division_table_mgr init success")
	}

	if !arena_robot_table_mgr.Init("") {
		log.Error("arena_robot_table_mgr init failed")
		return errors.New("arena_robot_table_mgr init failed")
	} else {
		log.Info("arena_robot_table_mgr init success")
	}

	if !active_stage_table_mgr.Init("") {
		log.Error("active_stage_table_mgr init failed")
		return errors.New("active_stage_mgr init failed")
	} else {
		log.Info("active_stage_table_mgr init success")
	}

	if !friend_boss_table_mgr.Init("") {
		log.Error("friend_boss_table_mgr init failed")
		return errors.New("friend_boss_table_mgr init failed")
	} else {
		log.Info("friend_boss_table_mgr init success")
	}

	if !explore_task_mgr.Init("") {
		log.Error("explore_task_mgr init failed")
		return errors.New("explore_task_mgr init failed")
	} else {
		log.Info("explore_task_mgr init success")
	}

	if !explore_task_boss_mgr.Init("") {
		log.Error("explore_task_boss_mgr init failed")
		return errors.New("explore_task_boss_mgr init failed")
	} else {
		log.Info("explore_task_boss_mgr init success")
	}

	if !task_table_mgr.Init("") {
		log.Error("task_table_mgr init failed")
		return errors.New("task_table_mgr init failed")
	} else {
		log.Info("task_table_mgr init success")
	}

	if !guild_mark_table_mgr.Init("") {
		return errors.New("guild_mark_table_mgr init failed")
	}

	if !guild_levelup_table_mgr.Init("") {
		return errors.New("guild_levelup_table_mgr init failed")
	}

	if !guild_donate_table_mgr.Init("") {
		return errors.New("guild_donate_table_mgr init failed")
	}

	if !guild_boss_table_mgr.Init("") {
		return errors.New("guild_boss_table_mgr init failed")
	}

	if !sign_table_mgr.Init("") {
		return errors.New("sign_table_mgr init failed")
	}

	if !seven_days_table_mgr.Init("") {
		return errors.New("seven_days_table_mgr init failed")
	}

	if !vip_table_mgr.Init("") {
		return errors.New("vip_table_mgr init failed")
	}

	if !pay_table_mgr.Init("") {
		return errors.New("pay_table_mgr init failed")
	}

	if !pay_mgr.init() {
		return errors.New("pay_mgr init failed")
	}

	return nil
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
		http.ListenAndServe("localhost:6060", nil)
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

	hall_server.Start(true)
}
