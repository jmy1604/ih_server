package main

import (
	"errors"
	"fmt"
	"ih_server/libs/log"
	"ih_server/libs/rpc"
	"ih_server/src/rpc_common"
	"time"
)

// 通用调用过程
type G2G_CommonProc struct {
}

func (this *G2G_CommonProc) Get(arg *rpc_common.G2G_GetRequest, result *rpc_common.G2G_GetResponse) (err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Stack(e)
		}
	}()

	rpc_client := GetCrossRpcClientByPlayerId(arg.FromPlayerId, arg.ToPlayerId)
	if rpc_client == nil {
		return errors.New(fmt.Sprintf("!!!!!! Not found rpc client by player id %v", arg.ToPlayerId))
	}

	err = rpc_client.Call("G2G_CommonProc.Get", arg, result)
	if err != nil {
		log.Error("RPC @@@ G2G_CommonProc.Get(%v,%v) error(%v)", arg, result, err.Error())
	} else {
		log.Trace("RPC @@@ G2G_CommonProc.Get(%v,%v)", arg, result)
	}

	return err
}

func (this *G2G_CommonProc) MultiGet(arg *rpc_common.G2G_MultiGetRequest, result *rpc_common.G2G_MultiGetResponse) (err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Stack(e)
		}
	}()
	return nil
}

// 大厅到大厅的调用
type H2H_CallProc struct {
}

func (this *H2H_CallProc) Do(args *rpc_common.H2R_Transfer, reply *rpc_common.H2R_TransferResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()
	rpc_client := GetRpcClientByPlayerId(args.ReceivePlayerId)
	if rpc_client == nil {
		err_str := fmt.Sprintf("!!!!!! Not found rpc client by player id %v", args.ReceivePlayerId)
		return errors.New(err_str)
	}

	log.Debug("H2H_CallProc @@@@@@@ call method[%v] args[%v] reply[%v]", args.Method, args.Args, reply.Result)

	var result interface{}
	err := rpc_client.Call(args.Method, args.Args, result)
	if err != nil {
		return err
	}
	log.Debug("H2H_CallProc @@@@@@@ call method[%v] result[%v]", args.Method, result)
	reply.Result = result
	return nil
}

// ping 大厅
type H2R_PingProc struct {
}

func (this *H2R_PingProc) Do(args *rpc_common.H2R_Ping, result *rpc_common.H2R_Pong) error {
	// 不做任何处理
	return nil
}

/* 监听RPC调用 */
type H2R_ListenRPCProc struct {
}

func (this *H2R_ListenRPCProc) Do(args *rpc_common.H2R_ListenIPNoitfy, result *rpc_common.H2R_ListenIPResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	log.Info("get notify listen rpc ip: %v", args.ListenIP)
	// 再连接到HallServer

	if !server.connect_hall(args.ListenIP, args.ServerId) {
		err_str := fmt.Sprintf("不能连接到大厅[IP:%v, Id:%v]", args.ListenIP, args.ServerId)
		return errors.New(err_str)
	}

	time.Sleep(time.Second * 1)
	return nil
}

/* 好友RPC调用 */
type H2R_FriendProc struct {
}

// ID申请好友
func (this *H2R_FriendProc) AddFriendById(args *rpc_common.H2R_AddFriendById, result *rpc_common.H2R_AddFriendResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	rpc_client := GetRpcClientByPlayerId(args.AddPlayerId)
	if rpc_client == nil {
		return errors.New("获取rpc客户端失败")
	}

	call_args := rpc_common.R2H_AddFriendById{}
	call_args.PlayerId = args.PlayerId
	call_args.AddPlayerId = args.AddPlayerId
	call_args.PlayerName = args.PlayerName
	call_result := &rpc_common.R2H_AddFriendResult{}

	err := rpc_client.Call("R2H_FriendProc.AddFriendById", call_args, call_result)
	if err != nil {
		return err
	}

	result.AddPlayerId = call_result.AddPlayerId
	result.PlayerId = call_result.PlayerId
	result.Error = call_result.Error
	return nil
}

func (this *H2R_FriendProc) AgreeAddFriend(args *rpc_common.H2R_AgreeAddFriend, result *rpc_common.H2R_AgreeAddFriendResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	rpc_client := GetRpcClientByPlayerId(args.AgreePlayerId)
	if rpc_client == nil {
		err_str := fmt.Sprintf("通过玩家ID[%v]获取rpc客户端失败", args.AgreePlayerId)
		return errors.New(err_str)
	}

	call_args := rpc_common.R2H_AgreeAddFriend{}
	call_args.AgreePlayerId = args.AgreePlayerId
	call_args.IsAgree = args.IsAgree
	call_args.PlayerId = args.PlayerId
	call_args.PlayerName = args.PlayerName
	call_result := &rpc_common.R2H_AgreeAddFriendResult{}
	err := rpc_client.Call("R2H_FriendProc.AgreeAddFriend", call_args, call_result)
	if err != nil {
		return err
	}

	result.IsAgree = args.IsAgree
	result.PlayerId = args.PlayerId
	result.AgreePlayerId = args.AgreePlayerId
	result.AgreePlayerName = call_result.AgreePlayerName
	result.AgreePlayerLevel = call_result.AgreePlayerLevel
	result.AgreePlayerVipLevel = call_result.AgreePlayerVipLevel
	result.AgreePlayerHead = call_result.AgreePlayerHead
	result.AgreePlayerLastLogin = call_result.AgreePlayerLastLogin

	return nil
}

func (this *H2R_FriendProc) RemoveFriend(args *rpc_common.H2R_RemoveFriend, result *rpc_common.H2R_RemoveFriendResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	rpc_client := GetRpcClientByPlayerId(args.RemovePlayerId)
	if rpc_client == nil {
		err_str := fmt.Sprintf("RPC FriendProc @@@ get rpc client by player_id[%v] failed", args.RemovePlayerId)
		return errors.New(err_str)
	}

	call_args := rpc_common.R2H_RemoveFriend{}
	call_args.PlayerId = args.PlayerId
	call_args.RemovePlayerId = args.RemovePlayerId
	call_result := &rpc_common.R2H_RemoveFriendResult{}
	err := rpc_client.Call("R2H_FriendProc.RemoveFriend", call_args, call_result)
	if err != nil {
		return err
	}

	return nil
}

type H2H_FriendProc struct {
}

// 添加好友
func (this *H2H_FriendProc) AddFriend(args *rpc_common.H2H_AddFriend, result *rpc_common.H2H_AddFriendResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	rpc_client := GetRpcClientByPlayerId(args.ToPlayerId)
	if rpc_client == nil {
		err_str := fmt.Sprintf("not found rpc client for player id[%v]", args.ToPlayerId)
		return errors.New(err_str)
	}

	err := rpc_client.Call("H2H_FriendProc.AddFriend", args, result)
	if err != nil {
		return err
	}

	return nil
}

// 赠送友情点
func (this *H2H_FriendProc) GiveFriendPoints(args *rpc_common.H2H_GiveFriendPoints, result *rpc_common.H2H_GiveFriendPointsResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	rpc_client := GetRpcClientByPlayerId(args.ToPlayerId)
	if rpc_client == nil {
		err_str := fmt.Sprintf("not found rpc client for player id[%v]", args.ToPlayerId)
		return errors.New(err_str)
	}

	err := rpc_client.Call("H2H_FriendProc.GiveFriendPoints", args, result)
	if err != nil {
		return err
	}

	return nil
}

// 刷新友情点
func (this *H2H_FriendProc) RefreshGivePoints(args *rpc_common.H2H_RefreshGiveFriendPoints, result *rpc_common.H2H_RefreshGiveFriendPointsResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	rpc_client := GetRpcClientByPlayerId(args.ToPlayerId)
	if rpc_client == nil {
		err_str := fmt.Sprintf("RPC FriendProc @@@ get rpc client by player_id[%v] failed", args.ToPlayerId)
		return errors.New(err_str)
	}

	err := rpc_client.Call("H2H_FriendProc.RefreshGivePoints", args, result)
	if err != nil {
		return err
	}

	return nil
}

func rpc_call_anouncement_player_first_rank(rank_type int32, rank_param int32, player_id int32, player_name string, player_level int32) error {
	args := rpc_common.R2H_RanklistPlayerFirstRank{}
	args.PlayerId = player_id
	args.RankType = rank_type
	args.RankParam = rank_param
	result := &rpc_common.R2H_RanklistPlayerFirstRankResult{}
	for _, r := range server.hall_rpc_clients {
		if r.rpc_client != nil {
			err := r.rpc_client.Call("R2H_RanklistProc.AnouncementFirstRank", args, result)
			if err != nil {
				err_str := fmt.Sprintf("@@@ R2H_RanklistProc::AnouncementFirstRank Player[%v] anouncement first rank for ranklist[%v] error[%v]", args.PlayerId, args.RankType, err.Error())
				return errors.New(err_str)
			}
		}
	}
	log.Debug("@@@ R2H_RanklistProc::AnouncementFirstRank Player[%v] anouncement first rank for ranklist[%v]", args.PlayerId, args.RankType)
	return nil
}

// 全局调用
type H2H_GlobalProc struct {
}

func (this *H2H_GlobalProc) WorldChat(args *rpc_common.H2H_WorldChat, result *rpc_common.H2H_WorldChatResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Error(err)
		}
	}()

	from_client := GetRpcClientByPlayerId(args.FromPlayerId)
	for _, r := range server.hall_rpc_clients {
		if r.rpc_client != nil && r.rpc_client != from_client {
			err := r.rpc_client.Call("H2H_GlobalProc.WorldChat", args, result)
			if err != nil {
				err_str := fmt.Sprintf("@@@ H2H_GlobalProc::WorldChat Player[%v] world chat error[%v]", args.FromPlayerId, err.Error())
				return errors.New(err_str)
			}
		}
	}
	log.Debug("@@@ H2H_GlobalProc::WorldChat Player[%v] world chat message[%v]", args.FromPlayerId, args.ChatContent)
	return nil
}

func (this *H2H_GlobalProc) Anouncement(args *rpc_common.H2H_Anouncement, result *rpc_common.H2H_AnouncementResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Error(err)
		}
	}()

	from_client := GetRpcClientByPlayerId(args.FromPlayerId)
	for _, r := range server.hall_rpc_clients {
		if r.rpc_client != nil && r.rpc_client != from_client {
			err := r.rpc_client.Call("H2H_GlobalProc.Anouncement", args, result)
			if err != nil {
				err_str := fmt.Sprintf("@@@ H2H_GlobalProc::Anouncement Player[%v] anouncement error[%v]", args.FromPlayerId, err.Error())
				return errors.New(err_str)
			}
		}
	}
	log.Debug("@@@ H2H_GlobalProc::Anouncement Player[%v] anouncement type[%v] param[%v]", args.FromPlayerId, args.MsgType, args.MsgParam1)
	return nil
}

// 全局调用
type H2R_GlobalProc struct {
}

func (this *H2R_GlobalProc) ChargeSave(args *rpc_common.H2R_ChargeSave, result *rpc_common.H2R_ChargeSaveResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Error(err)
		}
	}()

	if args.Channel == 1 {
		row := dbc.GooglePays.GetRow(args.OrderId)
		if row == nil {
			row = dbc.GooglePays.AddRow(args.OrderId)
			row.SetBundleId(args.BundleId)
			row.SetAccount(args.Account)
			row.SetPlayerId(args.PlayerId)
			row.SetPayTime(args.PayTime)
			row.SetPayTimeStr(args.PayTimeStr)
		}
	} else if args.Channel == 2 {
		row := dbc.ApplePays.GetRow(args.OrderId)
		if row == nil {
			row = dbc.ApplePays.AddRow(args.OrderId)
			row.SetBundleId(args.BundleId)
			row.SetAccount(args.Account)
			row.SetPlayerId(args.PlayerId)
			row.SetPayTime(args.PayTime)
			row.SetPayTimeStr(args.PayTimeStr)
		}
	} else {
		err_str := fmt.Sprintf("@@@ H2R_GlobalProc::ChargeSave Player[%v,%v], Unknown Channel %v", args.Account, args.PlayerId, args.Channel)
		return errors.New(err_str)
	}

	log.Trace("@@@ Charge Save %v", args)

	return nil
}

// 初始化
func (this *RpcServer) init_proc_service() bool {
	this.rpc_service = &rpc.Service{}

	if !this.rpc_service.Register(&H2H_CallProc{}) {
		return false
	}

	if !this.rpc_service.Register(&H2R_ListenRPCProc{}) {
		return false
	}

	if !this.rpc_service.Register(&H2H_GlobalProc{}) {
		return false
	}

	if !this.rpc_service.Register(&H2R_GlobalProc{}) {
		return false
	}

	if !this.rpc_service.Register(&G2G_CommonProc{}) {
		return false
	}

	// 注册用户自定义RPC数据类型
	rpc_common.RegisterRpcUserType()

	if this.rpc_service.Listen(config.ListenIP) != nil {
		return false
	}
	return true
}

// 反初始化
func (this *RpcServer) uninit_proc_service() {
	if this.rpc_service != nil {
		this.rpc_service.Close()
		this.rpc_service = nil
	}
}
