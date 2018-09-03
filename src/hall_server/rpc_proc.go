package main

import (
	"errors"
	"fmt"
	"ih_server/libs/log"
	"ih_server/libs/rpc"
	"ih_server/proto/gen_go/client_message"
	"ih_server/src/rpc_common"
)

// ping RPC服务
type R2H_PingProc struct{}

func (this *R2H_PingProc) Do(args *rpc_common.R2H_Ping, reply *rpc_common.R2H_Pong) error {
	// 不做任何处理
	log.Info("收到rpc服务的ping请求")
	return nil
}

//// 玩家调用
type R2H_PlayerProc struct {
}

// 获取查找的玩家数据
func (this *R2H_PlayerProc) GetInfoToSearch(args *rpc_common.R2H_SearchPlayer, reply *rpc_common.R2H_SearchPlayerResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	p := player_mgr.GetPlayerById(args.Id)
	if p == nil {
		err_str := fmt.Sprintf("RPC R2H_PlayerProc @@@ Not found player[%v], get player info failed", args.Id)
		return errors.New(err_str)
	}
	reply.Nick = p.db.GetName()
	reply.Level = p.db.Info.GetLvl()

	log.Debug("RPC R2H_PlayerProc @@@ Get player[%v] info", args.Id)

	return nil
}

// 好友
type R2H_FriendProc struct {
}

// 申请添加好友
func (this *R2H_FriendProc) AddFriendById(args *rpc_common.R2H_AddFriendById, reply *rpc_common.R2H_AddFriendResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	p := player_mgr.GetPlayerById(args.AddPlayerId)
	if p == nil {
		reply.Error = int32(msg_client_message.E_ERR_PLAYER_NOT_EXIST)
		log.Error("RPC R2H_FriendProc @@@ not found player[%v], cant add player[%v] to friend", args.AddPlayerId, args.PlayerId)
	} else {
		if p.db.Friends.HasIndex(args.PlayerId) {

		} else {

		}
	}

	reply.AddPlayerId = args.AddPlayerId
	reply.PlayerId = args.PlayerId

	if reply.Error >= 0 {
		log.Debug("RPC R2H_FriendProc @@@ Player[%v] requested add friend[%v]", args.PlayerId, args.AddPlayerId)
	}

	return nil
}

// 同意或拒绝好友申请
func (this *R2H_FriendProc) AgreeAddFriend(args *rpc_common.R2H_AgreeAddFriend, reply *rpc_common.R2H_AgreeAddFriendResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	p := player_mgr.GetPlayerById(args.AgreePlayerId)
	if p == nil {
		err_str := fmt.Sprintf("RPC R2H_FriendProc @@@ Not found player[%v]，player[%v] agree add friend failed", args.AgreePlayerId, args.PlayerId)
		return errors.New(err_str)
	}

	if !args.IsAgree {
		return nil
	}

	d := &dbPlayerFriendData{}
	d.PlayerId = args.PlayerId
	p.db.Friends.Add(d)

	reply.IsAgree = args.IsAgree
	reply.PlayerId = args.PlayerId
	reply.AgreePlayerId = args.AgreePlayerId
	reply.AgreePlayerName = p.db.GetName()
	reply.AgreePlayerLevel = p.db.Info.GetLvl()
	reply.AgreePlayerVipLevel = p.db.Info.GetVipLvl()
	reply.AgreePlayerLastLogin = p.db.Info.GetLastLogin()

	log.Debug("RPC R2H_FriendProc @@@ Player[%v] agreed add friend[%v]", args.PlayerId, args.AgreePlayerId)

	return nil
}

// 删除好友
func (this *R2H_FriendProc) RemoveFriend(args *rpc_common.R2H_RemoveFriend, reply *rpc_common.R2H_RemoveFriendResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	p := player_mgr.GetPlayerById(args.RemovePlayerId)
	if p == nil {
		err_str := fmt.Sprintf("RPC R2H_FriendProc @@@ Not found player[%v], player[%v] remove friend failed", args.RemovePlayerId, args.PlayerId)
		return errors.New(err_str)
	}

	p.db.Friends.Remove(args.PlayerId)

	log.Debug("RPC R2H_FriendProc @@@ Player[%v] removed friend[%v]", args.PlayerId, args.RemovePlayerId)

	return nil
}

// 大厅到大厅的好友调用
type H2H_FriendProc struct {
}

// 获取好友数据
func (this *H2H_FriendProc) GetFriendInfo(args *rpc_common.H2H_GetFriendInfo, reply *rpc_common.H2H_GetFriendInfoResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	p := player_mgr.GetPlayerById(args.PlayerId)
	if p == nil {
		err_str := fmt.Sprintf("RPC H2H_FriendProc::GetFriendInfo @@@ Not found Player[%v], get player info failed", args.PlayerId)
		return errors.New(err_str)
	}

	reply.PlayerId = p.Id
	reply.PlayerName = p.db.GetName()
	reply.Level = p.db.Info.GetLvl()
	reply.VipLevel = p.db.Info.GetVipLvl()
	reply.LastLogin = p.db.Info.GetLastLogin()

	log.Debug("RPC H2H_FriendProc @@@ Get player[%v] info", args.PlayerId)

	return nil
}

// 加为好友
func (this *H2H_FriendProc) AddFriend(args *rpc_common.H2H_AddFriend, reply *rpc_common.H2H_AddFriendResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	/*p := player_mgr.GetPlayerById(args.ToPlayerId)
	if p == nil {
		err_str := fmt.Sprintf("RPC H2H_FriendProc::AddFriend @@@ not found player[%v], add friend failed", args.ToPlayerId)
		return errors.New(err_str)
	}

	if p.db.Friends.HasIndex(args.FromPlayerId) {
		// 已是好友
		reply.Error = int32(msg_client_message.E_ERR_FRIEND_THE_PLAYER_ALREADY_FRIEND)
	}

	// 已有申请
	from_player_name, _, _ := GetPlayerBaseInfo(args.FromPlayerId)
	res := p.db.FriendReqs.CheckAndAdd(args.FromPlayerId, from_player_name)
	if res < 0 {
		reply.Error = int32(msg_client_message.E_ERR_FRIEND_THE_PLAYER_REQUESTED)
	}

	reply.FromPlayerId = args.FromPlayerId
	reply.ToPlayerId = args.ToPlayerId

	log.Debug("RPC H2H_FriendProc @@@ player[%v] added friend[%v]", args.FromPlayerId, args.ToPlayerId)*/

	return nil
}

// 删除好友
func (this *H2H_FriendProc) RemoveFriend(args *rpc_common.H2H_RemoveFriend, reply *rpc_common.H2H_RemoveFriendResult) error {

	return nil
}

// 赠送友情点
func (this *H2H_FriendProc) GiveFriendPoints(args *rpc_common.H2H_GiveFriendPoints, reply *rpc_common.H2H_GiveFriendPointsResult) error {

	return nil
}

// 好友聊天
func (this *H2H_FriendProc) Chat(args *rpc_common.H2H_FriendChat, reply *rpc_common.H2H_FriendChatResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	/*p := player_mgr.GetPlayerById(args.ToPlayerId)
	if p == nil {
		err_str := fmt.Sprintf("RPC H2H_FriendProc::Chat @@@ not found Player[%v]", args.ToPlayerId)
		return errors.New(err_str)
	}

	res := p.friend_chat_add(args.FromPlayerId, args.Message)
	if res < 0 {
		reply.Error = res
		log.Error("RPC H2H_FriendProc::Chat @@@ player[%v] chat to friend[%v] error[%v]", args.FromPlayerId, args.ToPlayerId, res)
	} else {
		reply.FromPlayerId = args.FromPlayerId
		reply.ToPlayerId = args.ToPlayerId
		reply.Message = args.Message
		log.Debug("RPC H2H_FriendProc @@@ Player[%v] chat friend[%v] message[%v]", args.FromPlayerId, args.ToPlayerId, args.Message)
	}*/
	return nil
}

// 刷新赠送好友
func (this *H2H_FriendProc) RefreshGivePoints(args *rpc_common.H2H_RefreshGiveFriendPoints, reply *rpc_common.H2H_RefreshGiveFriendPointsResult) error {
	return nil
}

// 大厅到大厅玩家调用
type H2H_PlayerProc struct {
}

// 更新玩家基本信息
func (this *H2H_PlayerProc) UpdateBaseInfo(args *rpc_common.H2H_BaseInfo, result *rpc_common.H2H_BaseInfoResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	row := os_player_mgr.GetPlayer(args.FromPlayerId)
	if row == nil {
		err_str := fmt.Sprintf("RPC H2H_PlayerProc::UpdateBaseInfo @@@ not found player[%v]", args.FromPlayerId)
		return errors.New(err_str)
	}

	row.SetName(args.Nick)
	row.SetLevel(args.Level)
	row.SetHead(args.Head)

	log.Debug("RPC H2H_PlayerProc::UpdateBaseInfo @@@ player[%v] updated base info", args.FromPlayerId)

	return nil
}

// 向另一个HallServer请求玩家数据
type R2H_PlayerStageInfoProc struct {
}

func (this *R2H_PlayerStageInfoProc) Do(args *rpc_common.R2H_PlayerStageInfoReq, result *rpc_common.R2H_PlayerStageInfoResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	p := player_mgr.GetPlayerById(args.PlayerId)
	if p == nil {
		return errors.New("无法找到玩家[%v]数据")
	}
	result.Level = p.db.Info.GetLvl()
	result.Nick = p.db.GetName()
	log.Info("获取玩家[%v]的关卡[%v]信息[%v]", args.PlayerId, args.StageId, *result)
	return nil
}

// 全局调用
type H2H_GlobalProc struct {
}

func (this *H2H_GlobalProc) WorldChat(args *rpc_common.H2H_WorldChat, result *rpc_common.H2H_WorldChatResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	/*(if !world_chat_mgr.push_chat_msg(args.ChatContent, args.FromPlayerId, args.FromPlayerLevel, args.FromPlayerName, args.FromPlayerHead) {
		err_str := fmt.Sprintf("@@@ H2H_GlobalProc::WorldChat Player[%v] world chat content[%v] failed", args.FromPlayerId, args.ChatContent)
		return errors.New(err_str)
	}*/
	log.Debug("@@@ H2H_GlobalProc::WorldChat Player[%v] world chat content[%v]", args.FromPlayerId, args.ChatContent)
	return nil
}

func (this *H2H_GlobalProc) Anouncement(args *rpc_common.H2H_Anouncement, result *rpc_common.H2H_AnouncementResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	if !anouncement_mgr.PushNew(args.MsgType, true, args.FromPlayerId, args.MsgParam1, args.MsgParam2, args.MsgParam3, args.MsgText) {
		err_str := fmt.Sprintf("@@@ H2H_GlobalProc::Anouncement Player[%v] anouncement msg_type[%v] msg_param[%v] failed", args.FromPlayerId, args.MsgType, args.MsgParam1)
		return errors.New(err_str)
	}
	log.Debug("@@@ H2H_GlobalProc::Anouncement Player[%v] anouncement msg_type[%v] msg_param[%v]", args.FromPlayerId, args.MsgType, args.MsgParam1)
	return nil
}

// 排行榜调用
type R2H_RanklistProc struct {
}

func (this *R2H_RanklistProc) AnouncementFirstRank(args *rpc_common.R2H_RanklistPlayerFirstRank, result *rpc_common.R2H_RanklistPlayerFirstRankResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	if !anouncement_mgr.PushNew(ANOUNCEMENT_TYPE_RANKING_LIST_FIRST_RANK, false, args.PlayerId /*args.PlayerName, args.PlayerLevel, */, args.RankType, args.RankParam, 0, "") {
		err_str := fmt.Sprintf("@@@ R2H_RanklistProc::AnouncementFirstRank Push Player[%v] first rank in ranklist[%v] failed", args.PlayerId /*args.PlayerName, args.PlayerLevel,*/, args.RankType)
		return errors.New(err_str)
	}
	log.Debug("@@@ R2H_RanklistProc::AnouncementFirstRank Pushed Player[%v] is first rank in ranklist[%v]", args.PlayerId /*args.PlayerName, args.PlayerLevel,*/, args.RankType)
	return nil
}

// 初始化rpc服务
func (this *HallServer) init_rpc_service() bool {
	if this.rpc_service != nil {
		return true
	}
	this.rpc_service = &rpc.Service{}

	// 注册RPC服务
	if !this.rpc_service.Register(&R2H_PingProc{}) {
		return false
	}
	if !this.rpc_service.Register(&R2H_PlayerProc{}) {
		return false
	}
	if !this.rpc_service.Register(&R2H_PlayerStageInfoProc{}) {
		return false
	}
	if !this.rpc_service.Register(&R2H_FriendProc{}) {
		return false
	}
	if !this.rpc_service.Register(&H2H_FriendProc{}) {
		return false
	}
	if !this.rpc_service.Register(&H2H_PlayerProc{}) {
		return false
	}
	if !this.rpc_service.Register(&H2H_GlobalProc{}) {
		return false
	}
	if !this.rpc_service.Register(&R2H_RanklistProc{}) {
		return false
	}

	if this.rpc_service.Listen(config.ListenRpcServerIP) != nil {
		log.Error("监听rpc服务端口[%v]失败", config.ListenRpcServerIP)
		return false
	}
	log.Info("监听rpc服务端口[%v]成功", config.ListenRpcServerIP)
	go this.rpc_service.Serve()
	return true
}

// 反初始化rpc服务
func (this *HallServer) uninit_rpc_service() {
	if this.rpc_service != nil {
		this.rpc_service.Close()
		this.rpc_service = nil
	}
}
