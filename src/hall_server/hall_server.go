package main

import (
	"errors"
	"ih_server/libs/log"
	"ih_server/libs/rpc"
	"ih_server/libs/socket"
	"ih_server/libs/timer"
	"ih_server/libs/utils"
	"ih_server/src/server_config"
	"ih_server/src/table_config"
	"sync"
	"time"
)

type HallServer struct {
	start_time         time.Time
	net                *socket.Node
	quit               bool
	shutdown_lock      *sync.Mutex
	shutdown_completed bool
	ticker             *timer.TickTimer
	initialized        bool
	last_gc_time       int32
	rpc_client         *rpc.Client  // 连接到rpc服务
	rpc_service        *rpc.Service // 接受rpc连接
	redis_conn         *utils.RedisConn

	server_info_row *dbServerInfoRow
}

var hall_server HallServer

func (this *HallServer) Init() (ok bool) {
	this.start_time = time.Now()
	this.shutdown_lock = &sync.Mutex{}
	this.net = socket.NewNode(&hall_server, time.Duration(config.RecvMaxMSec), time.Duration(config.SendMaxMSec), 5000, nil) //(this, 0, 0, 5000, 0, 0, 0, 0, 0)

	this.redis_conn = &utils.RedisConn{}
	if !this.redis_conn.Connect(config.RedisServerIP) {
		return
	}
	login_token_mgr.LoadRedisData()

	// rpc初始化
	if !this.init_rpc_service() {
		return
	}
	if !this.init_rpc_client() {
		return
	}

	err := this.OnInit()
	if err != nil {
		log.Error("服务器初始化失败[%s]", err.Error())
		return
	}

	// 世界频道
	world_chat_mgr.Init(CHAT_CHANNEL_WORLD)
	// 招募频道
	recruit_chat_mgr.Init(CHAT_CHANNEL_RECRUIT)
	// 公告
	anouncement_mgr.Init()
	// 录像
	battle_record_mgr.Init()
	// 爬塔排行榜
	tower_ranking_list.LoadDB()
	// 公会
	guild_manager.Init()
	// 公会副本
	guild_stage_manager.Init()
	// 载入公会副本伤害列表
	guild_manager.LoadDB4StageDamageList()

	this.initialized = true

	ok = true
	return
}

func (this *HallServer) OnInit() (err error) {
	team_member_pool.Init()
	battle_report_pool.Init()
	buff_pool.Init()
	passive_trigger_data_pool.Init()
	msg_battle_member_item_pool.Init()
	msg_battle_fighter_pool.Init()
	msg_battle_buff_item_pool.Init()
	msg_battle_reports_item_pool.Init()
	msg_battle_round_reports_pool.Init()
	delay_skill_pool.Init()

	player_mgr.RegMsgHandler()

	if !arena_season_mgr.Init() {
		log.Error("arena_season_mgr init failed")
		return errors.New("arena_season_mgr init failed")
	} else {
		log.Info("arena_season_mgr init success")
	}
	arena_robot_mgr.Init()

	if USE_CONN_TIMER_WHEEL == 0 {
		conn_timer_mgr.Init()
	} else {
		conn_timer_wheel.Init()
	}

	return
}

func (this *HallServer) Start(use_https bool) (err error) {
	log.Event("服务器已启动", nil, log.Property{"IP", config.ListenClientInIP})
	log.Trace("**************************************************")

	go this.Run()

	if use_https {
		crt_path := server_config.GetConfPathFile("server.crt")
		key_path := server_config.GetConfPathFile("server.key")
		msg_handler_mgr.StartHttps(crt_path, key_path)
	} else {
		msg_handler_mgr.StartHttp()
	}

	return
}

func (this *HallServer) Run() {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}

		this.shutdown_completed = true
	}()

	this.ticker = timer.NewTickTimer(1000)
	this.ticker.Start()
	defer this.ticker.Stop()

	go this.redis_conn.Run(1000)
	if USE_CONN_TIMER_WHEEL == 0 {
		go conn_timer_mgr.Run()
	} else {
		go conn_timer_wheel.Run()
	}

	go arena_season_mgr.Run()

	go friend_recommend_mgr.Run()

	for {
		select {
		case d, ok := <-this.ticker.Chan:
			{
				if !ok {
					return
				}

				begin := time.Now()
				this.OnTick(d)
				time_cost := time.Now().Sub(begin).Seconds()
				if time_cost > 1 {
					log.Trace("耗时 %v", time_cost)
					if time_cost > 30 {
						log.Error("耗时 %v", time_cost)
					}
				}
			}
		}
	}
}

func (this *HallServer) Shutdown() {
	if !this.initialized {
		return
	}

	this.shutdown_lock.Lock()
	defer this.shutdown_lock.Unlock()

	if this.quit {
		return
	}
	this.quit = true

	this.redis_conn.Close()
	arena_season_mgr.ToEnd()

	log.Trace("关闭游戏主循环")

	begin := time.Now()

	if this.ticker != nil {
		this.ticker.Stop()
		for {
			if this.shutdown_completed {
				break
			}

			time.Sleep(time.Millisecond * 100)
		}
	}

	log.Trace("等待 shutdown_completed 完毕")
	center_conn.client_node.Shutdown()
	this.net.Shutdown()
	if nil != msg_handler_mgr.msg_http_listener {
		msg_handler_mgr.msg_http_listener.Close()
	}

	this.uninit_rpc_service()
	this.uninit_rpc_client()

	log.Trace("关闭游戏主循环耗时 %v 秒", time.Now().Sub(begin).Seconds())

	dbc.Save(false)
	dbc.Shutdown()
}

func (this *HallServer) OnTick(t timer.TickTime) {
	player_mgr.OnTick()
}

func (this *HallServer) OnAccept(c *socket.TcpConn) {
	log.Info("HallServer OnAccept [%s]", c.GetAddr())
}

func (this *HallServer) OnConnect(c *socket.TcpConn) {

}

func (this *HallServer) OnDisconnect(c *socket.TcpConn, reason socket.E_DISCONNECT_REASON) {
	if c.T > 0 {
		cur_p := player_mgr.GetPlayerById(int32(c.T))
		if nil != cur_p {
			player_mgr.PlayerLogout(cur_p)
		}
	}
	log.Trace("玩家[%d] 断开连接[%v]", c.T, c.GetAddr())
}

func (this *HallServer) CloseConnection(c *socket.TcpConn, reason socket.E_DISCONNECT_REASON) {
	if c == nil {
		log.Error("参数为空")
		return
	}

	c.Close(reason)
}

func (this *HallServer) OnUpdate(c *socket.TcpConn, t timer.TickTime) {

}

var global_config table_config.GlobalConfig
var task_table_mgr table_config.TaskTableMgr
var item_table_mgr table_config.ItemTableMgr
var drop_table_mgr table_config.DropManager
var shop_table_mgr table_config.ShopTableManager
var shopitem_table_mgr table_config.ShopItemTableManager
var handbook_table_mgr table_config.HandbookTableMgr
var suit_table_mgr table_config.SuitTableMgr
var position_table table_config.PositionTable

var card_table_mgr table_config.CardTableMgr
var skill_table_mgr table_config.SkillTableMgr
var buff_table_mgr table_config.StatusTableMgr
var stage_table_mgr table_config.PassTableMgr
var campaign_table_mgr table_config.CampaignTableMgr
var levelup_table_mgr table_config.LevelUpTableMgr
var rankup_table_mgr table_config.RankUpTableMgr
var fusion_table_mgr table_config.FusionTableMgr
var talent_table_mgr table_config.TalentTableMgr
var tower_table_mgr table_config.TowerTableMgr
var item_upgrade_table_mgr table_config.ItemUpgradeTableMgr
var draw_table_mgr table_config.DrawTableMgr
var goldhand_table_mgr table_config.GoldHandTableMgr
var arena_division_table_mgr table_config.ArenaDivisionTableMgr
var arena_robot_table_mgr table_config.ArenaRobotTableMgr
var arena_bonus_table_mgr table_config.ArenaBonusTableMgr
var active_stage_table_mgr table_config.ActiveStageTableMgr
var friend_boss_table_mgr table_config.FriendBossTableMgr
var explore_task_mgr table_config.SearchTaskTableMgr
var explore_task_boss_mgr table_config.SearchTaskBossTableMgr
var guild_mark_table_mgr table_config.GuildMarkTableMgr
var guild_levelup_table_mgr table_config.GuildLevelUpTableMgr
var guild_donate_table_mgr table_config.GuildDonateTableMgr
var guild_boss_table_mgr table_config.GuildBossTableMgr

var team_member_pool TeamMemberPool
var battle_report_pool BattleReportPool
var buff_pool BuffPool
var passive_trigger_data_pool MemberPassiveTriggerDataPool
var msg_battle_member_item_pool MsgBattleMemberItemPool
var msg_battle_fighter_pool MsgBattleFighterPool
var msg_battle_buff_item_pool MsgBattleMemberBuffPool
var msg_battle_reports_item_pool MsgBattleReportItemPool
var msg_battle_round_reports_pool MsgBattleRoundReportsPool
var delay_skill_pool DelaySkillPool
