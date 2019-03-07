package rpc_proto

// ping大厅
type R2H_Ping struct {
}
type R2H_Pong struct {
}

// 查找玩家数据
type R2H_SearchPlayer struct {
	Id int32
}
type R2H_SearchPlayerResult struct {
	Nick      string
	Head      string
	Level     int32
	VipLevel  int32
	LastLogin int32
}

// 申请好友
type R2H_AddFriendById struct {
	PlayerId    int32
	PlayerName  string
	AddPlayerId int32
}
type R2H_AddFriendResult struct {
	PlayerId    int32
	AddPlayerId int32
	Error       int32
}

// 同意或拒绝好友申请
type R2H_AgreeAddFriend struct {
	IsAgree       bool
	PlayerId      int32
	PlayerName    string
	AgreePlayerId int32
}
type R2H_AgreeAddFriendResult struct {
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
type R2H_RemoveFriend struct {
	PlayerId       int32
	RemovePlayerId int32
}
type R2H_RemoveFriendResult struct {
}

// 获取好友数据
type R2H_GetFriendInfo struct {
	PlayerId int32
}
type R2H_GetFriendInfoResult struct {
	PlayerId   int32
	PlayerName string
	Level      int32
	VipLevel   int32
	Head       string
}

// RPC向大厅获取玩家关卡数据
type R2H_PlayerStageInfoReq struct {
	PlayerId int32
	StageId  int32
}

type R2H_PlayerStageInfoResult struct {
	PlayerId        int32
	StageId         int32
	Head            string
	Level           int32
	Nick            string
	PersonalitySign string
	TopScore        int32
}

// 排行榜公告
type R2H_RanklistPlayerFirstRank struct {
	PlayerId  int32
	RankType  int32
	RankParam int32
}
type R2H_RanklistPlayerFirstRankResult struct {
}
