package rpc_proto

const (
	GM_CMD_TEST              = iota // 测试
	GM_CMD_ANOUNCEMENT       = 1    // 公告
	GM_CMD_ADD_ITEMS         = 2    // 增加物品
	GM_CMD_SYS_MAIL          = 3    // 系统邮件
	GM_CMD_PLAYER_INFO       = 4    // 查询玩家信息
	GM_CMD_ONLINE_PLAYER_NUM = 5    // 在线人数查询
	GM_CMD_MONTH_CARD_SEND   = 6    // 月卡发送
	GM_CMD_BAN_PLAYER        = 7    // 封号
	GM_CMD_BAN_LIST          = 8    // 封号玩家列表
	GM_CMD_GUILD_INFO        = 9    // 公会信息
	GM_CMD_GUILD_LIST        = 10   // 公会列表
)

const (
	GM_CMD_TEST_STRING              = "test"
	GM_CMD_ANOUNCEMENT_STRING       = "anounce"
	GM_CMD_ADD_ITEMS_STRING         = "add_items"
	GM_CMD_SYS_MAIL_STRING          = "sys_mail"
	GM_CMD_PLAYER_INFO_STRING       = "player_info"
	GM_CMD_ONLINE_PLAYER_NUM_STRING = "online_player_num"
	GM_CMD_MONTH_CARD_SEND_STRING   = "month_card_send"
	GM_CMD_BAN_PLAYER_STRING        = "ban_player"
	GM_CMD_BAN_LIST_STRING          = "ban_list"
	GM_CMD_GUILD_INFO_STRING        = "guild_info"
	GM_CMD_GUILD_LIST_STRING        = "guild_list"
)

// 通用GM命令结构
type GmCmd struct {
	Id     int32  `json:"Id"`
	Data   []byte `json:"Data"`
	String string `json:"String"`
}

// GM命令返回结构
type GmResponse struct {
	Id   int32  `json:"Id"`
	Res  int32  `json:"Res"`
	Data []byte `json:"Data"`
}

// 通用返回
type GmCommonResponse struct {
	Res int32
}

// 测试命令
type GmTestCmd struct {
	NumValue    int32  `json:"NumValue"`
	StringValue string `json:"StringValue"`
	BytesValue  []byte `json:"BytesValue"`
}

// 公告
type GmAnouncementCmd struct {
	Content       []byte `json:"Content"`
	RemainSeconds int32  `json:"RemainSeconds"`
}

// 增加物品
type GmAddItemCmd struct {
	Items     []int32 `json:"Items"`
	PlayerIds []int32 `json:"PlayerIds"`
}

// 发送系统邮件
type GmSendSysMailCmd struct {
	PlayerAccount string  `json:"PlayerAccount"`
	PlayerId      int32   `json:"PlayerId"`
	MailTableID   int32   `json:"MailTableID"`
	AttachItems   []int32 `json"AttachItems"`
}

// 查询玩家信息
type GmPlayerInfoCmd struct {
	Id int32
}

// 查询玩家信息结果
type GmPlayerInfoResponse struct {
	Id               int32
	Account          string
	UniqueId         string
	CreateTime       int32
	IsLogin          int32
	LastLoginTime    int32
	LogoutTime       int32
	Level            int32
	VipLevel         int32
	Gold             int32
	Diamond          int32
	GuildId          int32
	GuildName        string
	GuildLevel       int32
	UnlockCampaignId int32
	HungupCampaignId int32
	ArenaScore       int32
	TalentList       []int32
	TowerId          int32
	SignIn           int32
	Roles            []int32
	Items            []int32
}

// 查询在线人数
type GmOnlinePlayerNumCmd struct {
	ServerId int32
}

// 查询在线人数结果
type GmOnlinePlayerNumResponse struct {
	PlayerNum []int32
}

// 发送月卡
type GmMonthCardSendCmd struct {
	PlayerId int32
	BundleId string
}

// 封号
type GmBanPlayerCmd struct {
	PlayerId      int32
	PlayerAccount string
	BanOrFree     int32
}

// 获得玩家唯一ID
type GmGetPlayerUniqueIdCmd struct {
	PlayerId int32
}

// 获得玩家唯一ID结果
type GmGetPlayerUniqueIdResponse struct {
	PlayerUniqueId string
}

// 通过唯一ID封号
type GmBanPlayerByUniqueIdCmd struct {
	PlayerUniqueId string
	PlayerId       int32
	BanOrFree      int32
}

// 公会成员信息
type GmGuildMemberInfo struct {
	PlayerId          int32
	PlayerName        string
	PlayerLevel       int32
	Position          int32
	JoinTime          int32
	QuitTime          int32
	SignTime          int32
	DonateNum         int32
	LastAskDonateTime int32
	LastDonateTime    int32
}

// 公会信息
type GmGuildInfo struct {
	Id             int32
	Name           string
	Level          int32
	Logo           int32
	MaxMemNum      int32
	CurrMemNum     int32
	PresidentId    int32
	PresidentName  string
	PresidentLevel int32
	CreateTime     int32
	Creater        int32
	MemList        []*GmGuildMemberInfo
}

type GmGuildInfoCmd struct {
	GuildId int32
}

type GmGuildInfoResponse struct {
	Info GmGuildInfo
}

// 获取服务器公会列表
type GmGuildListCmd struct {
	ServerId int32
}

// 获取服务器公会列表结果
type GmGuildListResponse struct {
	ServerId  int32
	GuildList []*GmGuildInfo
}
