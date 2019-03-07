package rpc_proto

import (
	"ih_server/libs/rpc"
)

// 修改基本信息
type H2H_BaseInfo struct {
	FromPlayerId int32
	Nick         string
	Level        int32
	Head         string
}
type H2H_BaseInfoResult struct {
	Error int32
}

// 搜索好友
type H2H_SearchFriend struct {
	PlayerId int32
}
type H2H_SearchFriendResult struct {
	PlayerId   int32
	PlayerName string
}

// 添加好友
type H2H_AddFriend struct {
	FromPlayerId int32
	ToPlayerId   int32
}
type H2H_AddFriendResult struct {
	FromPlayerId int32
	ToPlayerId   int32
	Error        int32 // 1 对方好友已满
}

// 同意加为好友
type H2H_AgreeAddFriend struct {
	FromPlayerId int32
	ToPlayerId   int32
}
type H2H_AgreeAddFriendResult struct {
	FromPlayerId int32
	ToPlayerId   int32
}

// 删除好友
type H2H_RemoveFriend struct {
	FromPlayerId int32
	ToPlayerId   int32
}
type H2H_RemoveFriendResult struct {
	FromPlayerId int32
	ToPlayerId   int32
}

// 获取好友数据
type H2H_GetFriendInfo struct {
	PlayerId int32
}
type H2H_GetFriendInfoResult struct {
	PlayerId   int32
	PlayerName string
	Level      int32
	VipLevel   int32
	Head       string
	LastLogin  int32
}

// 赠送友情点
type H2H_GiveFriendPoints struct {
	FromPlayerId int32
	ToPlayerId   int32
	GivePoints   int32
}
type H2H_GiveFriendPointsResult struct {
	FromPlayerId  int32
	ToPlayerId    int32
	GivePoints    int32
	LastSave      int32
	RemainSeconds int32
	Error         int32
}

// 好友聊天
type H2H_FriendChat struct {
	FromPlayerId int32
	ToPlayerId   int32
	Message      []byte
}
type H2H_FriendChatResult struct {
	FromPlayerId int32
	ToPlayerId   int32
	Message      []byte
	Error        int32
}

// 刷新赠送友情点
type H2H_RefreshGiveFriendPoints struct {
	FromPlayerId int32
	ToPlayerId   int32
}
type H2H_RefreshGiveFriendPointsResult struct {
}

// 赞
type H2H_ZanPlayer struct {
	FromPlayerId int32
	ToPlayerId   int32
}
type H2H_ZanPlayerResult struct {
	FromPlayerId   int32
	ToPlayerId     int32
	ToPlayerZanNum int32
}

// 通知世界聊天
type H2H_WorldChat struct {
	FromPlayerId    int32
	FromPlayerLevel int32
	FromPlayerName  string
	FromPlayerHead  string
	ChatContent     []byte
}
type H2H_WorldChatResult struct {
}

// 公告
type H2H_Anouncement struct {
	MsgType      int32
	FromPlayerId int32
	MsgParam1    int32
	MsgParam2    int32
	MsgParam3    int32
	MsgText      string
}
type H2H_AnouncementResult struct {
}

func RegisterRpcUserType() {
	rpc.RegisterUserType(&H2H_BaseInfo{})
	rpc.RegisterUserType(&H2H_BaseInfoResult{})
	rpc.RegisterUserType(&H2H_GetFriendInfo{})
	rpc.RegisterUserType(&H2H_GetFriendInfoResult{})
	rpc.RegisterUserType(&H2H_SearchFriend{})
	rpc.RegisterUserType(&H2H_AddFriend{})
	rpc.RegisterUserType(&H2H_AgreeAddFriend{})
	rpc.RegisterUserType(&H2H_RemoveFriend{})
	rpc.RegisterUserType(&H2H_GiveFriendPoints{})
	rpc.RegisterUserType(&H2H_FriendChat{})
	rpc.RegisterUserType(&H2H_ZanPlayer{})
	rpc.RegisterUserType(&H2H_WorldChat{})
	rpc.RegisterUserType(&H2H_WorldChatResult{})
	rpc.RegisterUserType(&H2H_Anouncement{})
	rpc.RegisterUserType(&H2H_AnouncementResult{})
}
