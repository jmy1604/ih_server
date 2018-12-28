package rpc_common

const (
	GM_CMD_NONE          = iota
	GM_CMD_ANOUNCEMENT   = 1
	GM_CMD_ADD_ITEMS     = 2
	GM_CMD_SEND_SYS_MAIL = 3
)

const (
	GM_CMD_TEST_STRING          = "test"
	GM_CMD_ANOUNCEMENT_STRING   = "anounce"
	GM_CMD_ADD_ITEMS_STRING     = "add_items"
	GM_CMD_SEND_SYS_MAIL_STRING = "send_sys_mail"
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
	Err  int32  `json:"Err"`
	Data []byte `json:"Data"`
}

// 测试命令
type GmTestCmd struct {
	NumValue    int32  `json:"NumValue"`
	StringValue string `json:"StringValue"`
	BytesValue  []byte `json:"BytesValue"`
}

type GmTestResponse struct {
}

// 公告
type GmAnouncementCmd struct {
	Content       []byte `json:"Content"`
	RemainSeconds int32  `json:"RemainSeconds"`
}

type GmAnouncementResponse struct {
}

// 增加物品
type GmAddItemCmd struct {
	Items     []int32 `json:"Items"`
	PlayerIds []int32 `json:"PlayerIds"`
}

type GmAddItemResponse struct {
}

// 发送系统邮件
type GmSendSysMailCmd struct {
	ReceivePlayerId int32   `json:"ReceivePlayerId"`
	MailType        int32   `json:"MailType"`
	AttachItems     []int32 `json"AttachItems"`
}

type GmSendSysMailResponse struct {
}
