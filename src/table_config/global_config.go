package table_config

import (
	"encoding/json"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type ChatConfig struct {
	MaxMsgNum       int32 // 最大消息数
	PullMaxMsgNum   int32 // 拉取的消息数量最大值
	PullMsgCooldown int32 // 拉取CD
	MsgMaxBytes     int32 // 消息最大长度
	MsgExistTime    int32 // 消息存在时间
	SendMsgCooldown int32 // 冷却时间
}

type GlobalConfig struct {
	InitRoles                            []int32 // 初始角色
	InitItems                            []int32 // 初始物品
	InitHeads                            []int32 // 初始头像
	InitDiamond                          int32   // 初始钻石
	InitCoin                             int32   // 初始金币
	InitEnergy                           int32   // 初始怒气
	MaxEnergy                            int32   // 最大怒气
	EnergyAdd                            int32   // 怒气增量
	HeartbeatInterval                    int32   // 心跳间隔
	MaxRoleCount                         int32   // 最大角色数量
	MailTitleBytes                       int32   // 邮件标题最大字节数
	MailContentBytes                     int32   // 邮件内容最大字节数
	MailMaxCount                         int32   // 邮件最大数量
	MailNormalExistDays                  int32   // 最大无附件邮件保存天数
	MailAttachExistDays                  int32   // 最大附件邮件保存天数
	MailPlayerSendCooldown               int32   // 个人邮件发送间隔(秒)
	PlayerBattleRecordMaxCount           int32   // 玩家战斗录像最大数量
	TowerKeyMax                          int32   // 爬塔钥匙最大值
	TowerKeyGetInterval                  int32   // 爬塔获取钥匙的时间间隔(秒)
	TowerKeyId                           int32   // 爬塔门票
	ItemLeftSlotOpenLevel                int32   // 左槽开启等级
	LeftSlotDropId                       int32   // 左槽掉落ID
	ArenaTicketItemId                    int32   // 竞技场门票ID
	ArenaTicketsDay                      int32   // 竞技场每天的门票
	ArenaTicketRefreshTime               string  // 竞技场门票刷新时间
	ArenaEnterLevel                      int32   // 竞技场进入等级
	ArenaGetTopRankNum                   int32   // 竞技场取最高排名数
	ArenaMatchPlayerNum                  int32   // 竞技场匹配人数
	ArenaRepeatedWinNum                  int32   // 竞技场连胜场数
	ArenaLoseRepeatedNum                 int32   // 竞技场连败场数
	ArenaHighGradeStart                  int32   // 竞技场高段位开始
	ArenaSeasonDays                      int32   // 竞技场赛季天数
	ArenaDayResetTime                    string  // 竞技场每天重置时间
	ArenaSeasonResetTime                 string  // 竞技场赛季重置时间
	ArenaBattleRewardDropId              int32   // 竞技场挑战奖励
	ActiveStageRefreshTime               string  // 活动副本数据重置时间
	ActiveStageChallengeNumOfDay         int32   // 活动副本重置挑战次数
	ActiveStageChallengeNumPrice         int32   // 活动副本挑战次数购买价格
	ActiveStagePurchaseNum               int32   // 活动副本每天购买次数
	FriendMaxNum                         int32   // 好友最大数量
	FriendRecommendNum                   int32   // 好友推荐数
	FriendPointItemId                    int32   // 友情点ID
	FriendPointsOnceGive                 int32   // 一次赠送的友情点
	FriendStaminaItemId                  int32   // 友情体力ID
	FriendStaminaLimit                   int32   // 友情体力上限
	FriendStartStamina                   int32   // 开始体力
	FriendStaminaResumeOnePointNeedHours int32   // 友情体力恢复一点需要时间(小时)
	FriendBossAttackCostStamina          int32   // 攻击好友BOSS消耗体力
	FriendBossAttackCooldown             int32   // 攻击好友BOSS冷却时间
	FriendRefreshTime                    string  // 好友刷新时间
	FriendSearchBossRefreshMinutes       int32   // 好友BOSS刷新时间间隔
	FriendAssistPointsGet                int32   // 助战好友获得友情点数
	FriendPointsGetLimitDay              int32   // 每天获得友情点上限
	FriendAssistPointsGetLimitDay        int32   // 每天通过助战获得友情点上限
	DailyTaskRefreshTime                 string  // 日常任务刷新时间
	ExploreTaskRefreshCostDiamond        int32   // 探索任务刷新花费钻石
	ExploreTaskRefreshTime               string  // 探索任务刷新时间
	GuildOpenLevel                       int32   // 公会开启等级
	GuildNameLength                      int32   // 公会名称长度限制
	GuildCreateCostGem                   int32   // 公会创建消耗钻石
	GuildChangeNameCostGem               int32   // 公会改名消耗钻石
	GuildSignReward                      []int32 // 公会签到奖励
	GuildSignAddExp                      int32   // 公会签到奖励经验
	GuildMailSendIntervalSecs            int32   // 公会邮件发送间隔
	GuildAskDonateCDSecs                 int32   // 公会请求捐献CD
	GuildQuitAskJoinCDSecs               int32   // 公会退出再加入的CD
	GuildDonateLimitDay                  int32   // 公会每天捐献上限
	GuildStageResetCDSecs                int32   // 公会副本重置CD
	GuildStageResurrectionGem            []int32 // 公会副本复活消耗钻石
	GuildDismissWaitingSeconds           int32   // 公会解散等待秒数
	GuildSignRefreshTime                 string  // 公会签到刷新时间点
	GuildAskDonateExistSeconds           int32   // 公会请求捐赠存在秒数
	GuildDonateRefreshTime               string  // 公会捐赠重置时间点
	GuildStageRefreshTime                string  // 公会副本重置时间点
	TalentResetCostDiamond               int32   // 重置天赋花费钻石

	GooglePayUrl       string
	FaceBookPayUrl     string
	ApplePayUrl        string
	ApplePaySandBoxUrl string

	MaxNameLen     int32   // 最大名字长度
	ChgNameCost    []int32 // 改名消耗的钻石
	ChgNameCostLen int32   // 消耗数组的长度

	FirstPayReward int32

	WorldChatData   ChatConfig // 世界频道
	GuildChatData   ChatConfig // 公会频道
	RecruitChatData ChatConfig // 招募频道
	SystemChatData  ChatConfig // 系统公告频道

	AnouncementMaxNum       int32 // 公告最大数量
	AnouncementSendCooldown int32 // 公告发送间隔冷却时间(分钟)
	AnouncementSendMaxNum   int32 // 公告一次发送最大数量
	AnouncementExistTime    int32 // 公告存在时间

	FirstChargeRewards []int32 // 首充奖励

	MonthCardSendRewardTime string // 月卡发奖时间

	AccelHungupRefreshCostDiamond int32 // 加速挂机刷新花费钻石

	ExpeditionRefreshTime      string  // 远征重置时间
	ExpeditionPurifyChangeCost int32   // 远征净化消耗
	ExpeditionPurifyChangeItem []int32 // 远征净化获得道具
}

func (this *GlobalConfig) Init(config_file string) bool {
	config_path := server_config.GetGameDataPathFile(config_file)
	data, err := ioutil.ReadFile(config_path)
	if nil != err {
		log.Error("GlobalConfigManager::Init failed to readfile err(%s)!", err.Error())
		return false
	}

	err = json.Unmarshal(data, this)
	if nil != err {
		log.Error("GlobalConfigManager::Init json unmarshal failed err(%s)!", err.Error())
		return false
	}

	return true
}
