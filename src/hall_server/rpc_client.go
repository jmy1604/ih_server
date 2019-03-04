package main

import (
	"errors"
	"ih_server/libs/log"
	"ih_server/libs/rpc"
	"ih_server/proto/gen_go/client_message"
	"ih_server/src/rpc_common"
)

func get_rpc_client() *rpc.Client {
	if hall_server.rpc_client == nil {
		log.Error("!!!!!!!!!! RPC Client is nil")
		return nil
	}
	return hall_server.rpc_client
}

func (this *HallServer) init_rpc_client() bool {
	// 注册用户自定义RPC数据类型
	rpc_common.RegisterRpcUserType()

	this.rpc_client = rpc.NewClient()
	var on_connect rpc.OnConnectFunc = func(args interface{}) {
		rpc_client := args.(*rpc.Client)
		proc_string := "H2R_ListenRPCProc.Do"
		var arg = rpc_common.H2R_ListenIPNoitfy{config.ListenRpcServerIP, config.ServerId}
		var result = rpc_common.H2R_ListenIPResult{}
		err := rpc_client.Call(proc_string, arg, &result)
		if err != nil {
			log.Error("RPC调用[%v]失败, err:%v", proc_string, err.Error())
			return
		}
		log.Info("RPC调用[%v]成功", proc_string)
	}
	this.rpc_client.SetOnConnect(on_connect)

	if !this.rpc_client.Dial(config.RpcServerIP) {
		log.Error("连接rpc服务器[%v]失败", config.RpcServerIP)
		return false
	}
	log.Info("连接rpc服务器[%v]成功!!!", config.RpcServerIP)

	this.rpc_client.Run()

	return true
}

func (this *HallServer) uninit_rpc_client() {
	if this.rpc_client != nil {
		this.rpc_client.Close()
		this.rpc_client = nil
	}
}

// 游戏服到游戏服调用
func (this *HallServer) rpc_hall2hall(receive_player_id int32, method string, args interface{}, reply interface{}) error {
	if this.rpc_client == nil {
		err := errors.New("!!!! rpc client is null")
		return err
	}
	transfer_args := &rpc_common.H2R_Transfer{}
	transfer_args.Method = method
	transfer_args.Args = args
	transfer_args.ReceivePlayerId = receive_player_id
	transfer_reply := &rpc_common.H2R_TransferResult{}
	transfer_reply.Result = reply

	log.Debug("@@@@@ #####  transfer_args[%v]  transfer_reply[%v]", transfer_args.Args, transfer_reply.Result)

	err := this.rpc_client.Call("H2H_CallProc.Do", transfer_args, transfer_reply)
	if err != nil {
		log.Error("RPC @@@ H2H_CallProc.Do(%v,%v) error(%v)", transfer_args, transfer_reply, err.Error())
	}
	return err
}

// 通用请求函数
func (this *HallServer) rpc_g2g_get(from_player_id, to_player_id, msg_id int32, msg_data []byte) (result_data []byte, err_code int32) {
	if this.rpc_client == nil {
		return nil, -1
	}
	var arg = rpc_common.G2G_GetRequest{
		FromPlayerId: from_player_id,
		ToPlayerId:   to_player_id,
		MsgId:        msg_id,
		MsgData:      msg_data,
	}
	var result rpc_common.G2G_GetResponse
	err := this.rpc_client.Call("G2G_CommonProc.Get", &arg, &result)
	if err != nil {
		err_code = int32(msg_client_message.E_ERR_REMOTE_FUNC_CALL_ERROR)
		log.Error("rpc_g2g_get error(%v)", err.Error())
		return
	}

	result_data = result.Data.ResultData
	err_code = result.Data.ErrorCode

	log.Trace("rpc_g2g_get: arg %v, result %v", arg, result)

	return
}

func (this *HallServer) rpc_g2g_get_multi(from_player_id int32, to_player_ids []int32, msg_id int32, msg_data []byte) (datas []*rpc_common.ServerResponseData) {
	if this.rpc_client == nil {
		log.Error("!!! rpc client is null")
		return nil
	}
	var arg = rpc_common.G2G_MultiGetRequest{
		FromPlayerId: from_player_id,
		ToPlayerIds:  to_player_ids,
		MsgId:        msg_id,
		MsgData:      msg_data,
	}
	var result rpc_common.G2G_MultiGetResponse
	err := this.rpc_client.Call("G2G_CommonProc.MultiGet", &arg, &result)
	if err != nil {
		log.Error("rpc_g2g_get_multi error(%v)", err.Error())
	} else {
		log.Trace("rpc_g2g_get_multi: arg %v, result %v", arg, result)
	}
	datas = result.Datas
	return
}

// 通过ID申请好友
/*func (this *Player) rpc_add_friend(add_id int32) (result *rpc_common.H2R_AddFriendResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}
	arg := &rpc_common.H2R_AddFriendById{}
	arg.PlayerId = this.Id
	arg.PlayerName = this.db.GetName()
	arg.AddPlayerId = add_id
	result = &rpc_common.H2R_AddFriendResult{}
	err := rpc_client.Call("H2R_FriendProc.AddFriendById", arg, result)
	if err != nil {
		log.Error("RPC[%v]申请好友[%v]错误[%v]", this.Id, add_id, err.Error())
		return nil
	}
	return
}

// 通过昵称申请好友
func (this *Player) rpc_add_friend_by_name(add_name string) (result *rpc_common.H2R_AddFriendResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}
	arg := &rpc_common.H2R_AddFriendByName{}
	arg.PlayerName = this.db.GetName()
	arg.PlayerId = this.Id
	arg.AddPlayerName = add_name
	result = &rpc_common.H2R_AddFriendResult{}
	err := rpc_client.Call("H2R_FriendProc.AddFriendByName", arg, result)
	if err != nil {
		log.Error("RPC[%v]申请好友[%v]错误[%v]", this.Id, add_name, err.Error())
		return nil
	}
	return
}

// 同意或拒绝好友申请
func (this *Player) rpc_agree_add_friend(from_id int32, is_agree bool) (result *rpc_common.H2R_AgreeAddFriendResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}
	arg := rpc_common.H2R_AgreeAddFriend{}
	arg.PlayerId = this.Id
	arg.PlayerName = this.db.GetName()
	arg.AgreePlayerId = from_id
	arg.IsAgree = is_agree
	result = &rpc_common.H2R_AgreeAddFriendResult{}
	err := rpc_client.Call("H2R_FriendProc.AgreeAddFriend", arg, result)
	if err != nil {
		log.Error("RPC [%v]同意好友[%v]申请错误[%v]", this.Id, from_id, err.Error())
		return nil
	}
	return
}

// 删除好友
func (this *Player) rpc_remove_friend(player_id int32) (result *rpc_common.H2R_RemoveFriendResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}
	arg := &rpc_common.H2R_RemoveFriend{}
	arg.PlayerId = this.Id
	arg.RemovePlayerId = player_id
	result = &rpc_common.H2R_RemoveFriendResult{}
	err := rpc_client.Call("H2R_FriendProc.RemoveFriend", arg, result)
	if err != nil {
		log.Error("RPC ### Player[%v] remove friend[%v] error[%v]", this.Id, player_id, err.Error())
		return nil
	}
	return
}

// 获取好友信息
func (this *Player) rpc_get_friend_info(player_id int32) (result *rpc_common.H2R_GetFriendInfoResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}
	var arg = rpc_common.H2R_GetFriendInfo{}
	arg.PlayerId = player_id
	result = &rpc_common.H2R_GetFriendInfoResult{}
	err := rpc_client.Call("H2R_FriendProc.GetFriendInfo", arg, result)
	if err != nil {
		log.Error("RPC ### Player[%v] get friend[%v] info error[%v]", this.Id, player_id, err.Error())
		return nil
	}
	return
}

// 赠送友情点
func (this *Player) rpc_give_friend_points(player_id int32) (result *rpc_common.H2H_GiveFriendPointsResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}

	arg := &rpc_common.H2H_GiveFriendPoints{}
	arg.FromPlayerId = this.Id
	arg.ToPlayerId = player_id
	result = &rpc_common.H2H_GiveFriendPointsResult{}
	err := rpc_client.Call("H2H_FriendProc.GiveFriendPoints", arg, result)
	if err != nil {
		log.Error("RPC ### Player[%v] to player[%v] H2H_GiveFriendPoints error[%v]", this.Id, player_id, err.Error())
	} else {
		log.Debug("RPC ### Player[%v] to player[%v] H2H_GiveFriendPoints done", this.Id, player_id)
	}
	return
}

// 刷新友情点
func (this *Player) rpc_refresh_give_friend_point(friend_id int32) (result *rpc_common.H2H_RefreshGiveFriendPointsResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}
	args := &rpc_common.H2H_RefreshGiveFriendPoints{
		FromPlayerId: this.Id,
		ToPlayerId:   friend_id,
	}
	result = &rpc_common.H2H_RefreshGiveFriendPointsResult{}
	err := rpc_client.Call("H2H_FriendProc.RefreshGivePoints", args, result)
	if err != nil {
		log.Error("RPC ### Player[%v] refresh give points to friend[%v] error[%v]", this.Id, args.ToPlayerId, err.Error())
		return nil
	}
	return
}

// 好友聊天
func (this *Player) rpc_friend_chat(player_id int32, message []byte) (result *rpc_common.H2H_FriendChatResult) {
	args := &rpc_common.H2H_FriendChat{}
	args.FromPlayerId = this.Id
	args.ToPlayerId = player_id
	args.Message = message
	result = &rpc_common.H2H_FriendChatResult{}
	err := hall_server.rpc_hall2hall(player_id, "H2H_FriendProc.Chat", args, result)
	if err != nil {
		log.Error("RPC ### Player[%v] to friend[%v] H2H_FriendChat error[%v]", this.Id, player_id, err.Error())
		return nil
	}
	if result.Error < 0 {
		err_str := fmt.Sprintf("RPC ### Player[%v] to friend[%v] H2H_FriendChat error[%v]", this.Id, player_id, result.Error)
		log.Error(err_str)
		return nil
	}
	return
}

// 世界聊天
func (p *Player) rpc_world_chat(content []byte) (result *rpc_common.H2H_WorldChatResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}

	args := &rpc_common.H2H_WorldChat{}
	args.FromPlayerId = p.Id
	args.FromPlayerLevel = p.db.Info.GetLvl()
	args.FromPlayerName = p.db.GetName()
	args.ChatContent = content

	result = &rpc_common.H2H_WorldChatResult{}
	err := rpc_client.Call("H2H_GlobalProc.WorldChat", args, result)
	if err != nil {
		log.Error("RPC ### Player[%v] send world chat to broadcast error[%v]", p.Id, err.Error())
		return nil
	}
	return
}*/

// 充值记录
func (p *Player) rpc_charge_save(channel int32, order_id, bundle_id, account string, player_id, pay_time int32, pay_time_str string) (result *rpc_common.H2R_ChargeSaveResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}

	var args = rpc_common.H2R_ChargeSave{
		Channel:    channel,
		OrderId:    order_id,
		BundleId:   bundle_id,
		Account:    account,
		PlayerId:   player_id,
		PayTime:    pay_time,
		PayTimeStr: pay_time_str,
	}

	result = &rpc_common.H2R_ChargeSaveResult{}
	err := rpc_client.Call("H2R_GlobalProc.ChargeSave", &args, result)
	if err != nil {
		log.Error("RPC ### Player[%v] charge save err[%v]", p.Id, err.Error())
	}
	return
}
