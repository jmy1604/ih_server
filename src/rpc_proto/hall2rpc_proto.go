package rpc_proto

// 转发消息
type H2R_Transfer struct {
	Method          string
	Args            interface{}
	ReceivePlayerId int32
}
type H2R_TransferResult struct {
	Result interface{}
}

// ping RPC服务
type H2R_Ping struct {
}

type H2R_Pong struct {
}

// 大厅通知RPC监听端口
type H2R_ListenIPNoitfy struct {
	ListenIP string
	ServerId int32
}
type H2R_ListenIPResult struct {
}

// 玩家搜索好友数据
type H2R_SearchPlayerInfo struct {
	Id        int32
	Nick      string
	Level     int32
	VipLevel  int32
	Head      string
	LastLogin int32
}

// 搜索好友结果
type H2R_SearchFriendResult struct {
	Players []*H2R_SearchPlayerInfo
}

// 好友申请
type H2R_AddFriendById struct {
	PlayerId    int32
	PlayerName  string
	AddPlayerId int32
}
type H2R_AddFriendByName struct {
	PlayerId      int32
	PlayerName    string
	AddPlayerName string
}
type H2R_AddFriendResult struct {
	PlayerId    int32
	AddPlayerId int32
	Error       int32
}

// 同意或拒绝好友申请
type H2R_AgreeAddFriend struct {
	IsAgree       bool
	PlayerId      int32
	PlayerName    string
	AgreePlayerId int32
}
type H2R_AgreeAddFriendResult struct {
	IsAgree              bool
	PlayerId             int32
	AgreePlayerId        int32
	AgreePlayerName      string
	AgreePlayerLevel     int32
	AgreePlayerVipLevel  int32
	AgreePlayerHead      string
	AgreePlayerLastLogin int32
}

// 删除好友
type H2R_RemoveFriend struct {
	PlayerId       int32
	RemovePlayerId int32
}
type H2R_RemoveFriendResult struct {
}

// 获取好友数据
type H2R_GetFriendInfo struct {
	PlayerId int32
}
type H2R_GetFriendInfoResult struct {
	PlayerId   int32
	PlayerName string
	Head       string
	Level      int32
	VipLevel   int32
	LastLogin  int32
}

// 删除玩家排名
type H2R_RankDelete struct {
	PlayerId int32
	RankType int32
	Param    int32
}
type H2R_RankDeleteResult struct {
	PlayerId int32
	RankType int32
	Param    int32
}

// 充值记录
type H2R_ChargeSave struct {
	Channel    int32
	OrderId    string
	BundleId   string
	Account    string
	PlayerId   int32
	PayTime    int32
	PayTimeStr string
}

type H2R_ChargeSaveResult struct {
}
