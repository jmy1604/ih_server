package main

import (
	"errors"
	"fmt"
	"ih_server/libs/log"
	"ih_server/libs/rpc"
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

// 大厅到大厅直接调用
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

// 添加昵称，不需要返回值
func (p *Player) rpc_add_nick_id(nick string, id int32) error {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return errors.New("!!!! rpc client is null")
	}
	arg := &rpc_common.H2R_AddIdNick{}
	result := &rpc_common.H2R_AddIdNickResult{}
	arg.Id = id
	arg.Nick = nick
	err := rpc_client.Call("H2R_NickIdProc.AddIdNick", arg, result)
	if err != nil {
		err_str := fmt.Sprintf("RPC添加昵称[%v,%v]失败[%v]", nick, id, err.Error())
		log.Error(err_str)
		return err
	}
	log.Info("RPC添加昵称[%v,%v]成功", nick, id)
	return nil
}

// 修改昵称
func (this *Player) rpc_rename_nick(old_nick, new_nick string) *rpc_common.H2R_RenameNickResult {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}
	arg := &rpc_common.H2R_RenameNick{}
	result := &rpc_common.H2R_RenameNickResult{}
	arg.FromPlayerId = this.Id
	arg.OldNick = old_nick
	arg.NewNick = new_nick
	rpc_client.Call("H2R_NickIdProc.RenameNick", arg, result)
	if result.Error > 0 {
		log.Info("RPC修改昵称[%v,%v]成功", old_nick, new_nick)
	}
	return result
}

// 基本信息修改
func (this *Player) rpc_update_base_info() *rpc_common.H2H_BaseInfoResult {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}

	args := &rpc_common.H2H_BaseInfo{}
	result := &rpc_common.H2H_BaseInfoResult{}
	args.FromPlayerId = this.Id
	args.Nick = this.db.GetName()
	args.Level = this.db.Info.GetLvl()
	rpc_client.Call("H2H_PlayerProc.UpdateBaseInfo", args, result)
	if result.Error < 0 {
		log.Error("RPC Update Player[%v] base info error[%v]", this.Id, result.Error)
	} else {
		log.Debug("RPC Updated Player[%v] base info", this.Id)
	}
	return result
}

// 获得商店全服限量商品
func (this *Player) rpc_get_shop_limited_item(item_id int32) *rpc_common.H2R_ShopLimitedItemResult {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}
	log.Debug("####### rpc_client pointer is %v", rpc_client)
	args := &rpc_common.H2R_ShopLimitedItem{}
	result := new(rpc_common.H2R_ShopLimitedItemResult)
	args.ItemId = item_id
	err := rpc_client.Call("H2R_ShopProc.GetLimitedItemNum", args, result)
	if err != nil {
		log.Error("RPC获取商店限时商品[%v]失败[%v]", item_id, err.Error())
		return nil
	}
	log.Info("RPC获取商店限时商品[%v]成功", item_id)
	return result
}

// 购买商店限时商品
func (this *Player) rpc_buy_shop_limited_item(item_id int32, item_num int32) *rpc_common.H2R_BuyLimitedShopItemResult {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}
	log.Debug("####### rpc_client pointer is %v", rpc_client)
	args := &rpc_common.H2R_BuyLimitedShopItem{}
	var result = &rpc_common.H2R_BuyLimitedShopItemResult{}
	args.ItemId = item_id
	args.Num = item_num
	err := rpc_client.Call("H2R_ShopProc.BuyLimitedItem", args, result)
	if err != nil {
		log.Error("RPC购买商店限时商品[%v]失败[%v]", item_id, err.Error())
		return nil
	}
	log.Info("RPC限时商品[%v]购买成功", item_id)
	return result
}

// 刷新商店
func (this *Player) rpc_refresh_shop_limited_item() error {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return errors.New("!!!! rpc client is null")
	}
	log.Debug("####### rpc_client pointer is %v", rpc_client)
	args := &rpc_common.H2R_RefreshLimitedShopItem{}
	var result = &rpc_common.H2R_RefreshLimitedShopItemResult{}
	err := rpc_client.Call("H2R_ShopProc.RefreshLimitedItems", args, result)
	if err != nil {
		log.Error("RPC刷新商店失败[%v]", err.Error())
		return err
	}
	log.Info("RPC刷新商店成功")
	return nil
}

// 刷新部分商店商品
func (this *Player) rpc_refresh_some_shop_limited_item(items []int32) error {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return errors.New("!!!! rpc client is null")
	}
	log.Debug("####### rpc_client pointer is %v", rpc_client)
	args := &rpc_common.H2R_RefreshSomeShopLimitedItem{}
	var result = &rpc_common.H2R_RefreshSomeShopLimitedItemResult{}
	err := rpc_client.Call("H2R_ShopProc.RefreshSomeShopLimitedItems", args, result)
	if err != nil {
		log.Error("RPC刷新商店部分物品[%v]错误[%v]", items, err.Error())
		return err
	}
	log.Info("RPC刷新商店部分商品成功")
	return nil
}

// 检查刷新商店
func (this *Player) rpc_check_refresh_shop_limited_item(days int32) error {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return errors.New("!!!! rpc client is null")
	}
	log.Debug("####### rpc_client pointer is %v", rpc_client)
	args := &rpc_common.H2R_CheckRefreshShop{}
	args.Days = days
	var result = &rpc_common.H2R_CheckRefreshShopResult{}
	err := rpc_client.Call("H2R_ShopProc.CheckRefreshShop4Days", args, result)
	if err != nil {
		log.Error("RPC刷新商店限时[%v]天商品错误[%v]", days, err.Error())
		return err
	}
	if result.Result > 0 {
		log.Info("RPC刷新商店限时[%v]天商品成功", days)
	}
	return nil
}

// 查找玩家
func (this *Player) rpc_search_friend(nick string) (result *rpc_common.H2R_SearchFriendResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}
	arg := &rpc_common.H2R_SearchFriendByNick{}
	result = &rpc_common.H2R_SearchFriendResult{}
	arg.Nick = nick
	err := rpc_client.Call("H2R_FriendProc.SearchByNick", arg, result)
	if err != nil {
		log.Error("RPC昵称[%v]查找好友错误[%v]", nick, err.Error())
		return nil
	}
	log.Info("RPC搜索好友昵称[%v]成功", nick)
	return result
}

// id查找玩家
func (this *Player) rpc_search_friend_by_id(id int32) (result *rpc_common.H2R_SearchFriendResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}
	arg := &rpc_common.H2R_SearchFriendById{}
	arg.Id = id
	result = &rpc_common.H2R_SearchFriendResult{}
	err := rpc_client.Call("H2R_FriendProc.SearchById", arg, result)
	if err != nil {
		log.Error("RPC[%v]查找玩家ID[%v]错误[%v]", id, err.Error())
		return nil
	}
	return
}

// 关键字查找玩家
func (this *Player) rpc_search_friend_by_key(key string) (result *rpc_common.H2R_SearchFriendResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}
	args := &rpc_common.H2R_SearchFriendByKey{
		Key: key,
	}
	result = &rpc_common.H2R_SearchFriendResult{}
	err := rpc_client.Call("H2R_FriendProc.SearchByKey", args, result)
	if err != nil {
		log.Error("RPC @@@ Player[%v] search friend by key[%v] error[%v]", this.Id, key, err.Error())
		return nil
	}
	return
}

// 通过ID申请好友
func (this *Player) rpc_add_friend(add_id int32) (result *rpc_common.H2R_AddFriendResult) {
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

// 删除好友2
func (this *Player) rpc_remove_friend2(friend_id int32) (result *rpc_common.H2H_RemoveFriendResult) {
	var args = rpc_common.H2H_RemoveFriend{}
	args.FromPlayerId = this.Id
	args.ToPlayerId = friend_id
	result = &rpc_common.H2H_RemoveFriendResult{}
	err := hall_server.rpc_hall2hall(friend_id, "H2H_FriendProc.RemoveFriend", args, result)
	if err != nil {
		log.Error("RPC ### Player[%v] remove friend[%v] error[%v]", this.Id, friend_id, err.Error())
		return nil
	}
	log.Debug("RPC ### Player[%v] removed friend[%v]", this.Id, friend_id)
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

// 获取好友信息2
func (this *Player) rpc_get_friend_info2(player_id int32) (result *rpc_common.H2H_GetFriendInfoResult) {
	arg := &rpc_common.H2H_GetFriendInfo{}
	arg.PlayerId = player_id
	result = &rpc_common.H2H_GetFriendInfoResult{}
	err := hall_server.rpc_hall2hall(player_id, "H2H_FriendProc.GetFriendInfo", arg, result)
	if err != nil {
		log.Error("RPC ### Player[%v] to player[%v] H2H_GetFriendInfo error[%v]", this.Id, player_id, err.Error())
		return nil
	}
	log.Debug("RPC ### Player[%v] to player[%v] H2H_GetFriendInfo done", this.Id, player_id)
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

// rpc请求 获取好友的关卡数据
func (p *Player) rpc_get_friends_stage_info(stage_id int32) (result *rpc_common.H2R_FriendsStagePassDataResult) {
	/*ids := p.db.Friends.GetAllIds()
	if ids == nil || len(ids) == 0 {
		return
	}
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}
	proc_string := "H2R_StageProc.GetFriendsStageInfo"
	var arg = &rpc_common.H2R_FriendsStagePassDataRequest{}
	arg.FriendIds = ids
	arg.PlayerId = p.Id
	arg.StageId = stage_id
	result = &rpc_common.H2R_FriendsStagePassDataResult{}
	err := rpc_client.Call(proc_string, arg, result)
	if err != nil {
		log.Error("RPC调用[%v]失败, err:%v", proc_string, err.Error())
		return nil
	}
	log.Info("RPC调用[%v]成功", proc_string)*/
	return result
}

// 点赞
func (p *Player) rpc_zan_player(player_id int32) (result *rpc_common.H2H_ZanPlayerResult) {
	args := &rpc_common.H2H_ZanPlayer{}
	args.FromPlayerId = p.Id
	args.ToPlayerId = player_id
	result = &rpc_common.H2H_ZanPlayerResult{}
	err := hall_server.rpc_hall2hall(player_id, "H2H_PlayerProc.Zan", args, result)
	if err != nil {
		log.Error("RPC ### Player[%v] zan to player[%v] error[%v]", p.Id, player_id, err.Error())
		return nil
	}
	return
}

func (p *Player) rpc_zan_player2(player_id int32) (result *rpc_common.H2H_ZanPlayerResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}
	args := &rpc_common.H2H_ZanPlayer{}
	args.FromPlayerId = p.Id
	args.ToPlayerId = player_id
	result = &rpc_common.H2H_ZanPlayerResult{}
	err := rpc_client.Call("H2H_PlayerProc.Zan", args, result)
	if err != nil {
		log.Error("RPC ### Player[%v] zan to player[%v] error[%v]", p.Id, player_id, err.Error())
		return nil
	}
	return
}

// 排行榜关卡总分更新
func (p *Player) rpc_rank_update_stage_total_score(total_score int32) (result *rpc_common.H2R_RankStageTotalScoreResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}
	args := &rpc_common.H2R_RankStageTotalScore{}
	args.PlayerId = p.Id
	args.PlayerLevel = p.db.Info.GetLvl()
	args.TotalScore = total_score
	result = &rpc_common.H2R_RankStageTotalScoreResult{}
	err := rpc_client.Call("H2R_RankingListProc.UpdateStageTotalScore", args, result)
	if err != nil {
		log.Error("RPC ### Player[%v] update stage total score[%v] error[%v]", p.Id, total_score, err.Error())
		return nil
	}
	return
}

// 获取关卡总分更新
func (p *Player) rpc_ranklist_stage_total_score(rank_start, rank_num int32) (result *rpc_common.H2R_RanklistGetStageTotalScoreResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}
	args := &rpc_common.H2R_RanklistGetStageTotalScore{}
	args.PlayerId = p.Id
	args.RankStart = rank_start
	args.RankNum = rank_num
	result = &rpc_common.H2R_RanklistGetStageTotalScoreResult{}
	err := rpc_client.Call("H2R_RankingListProc.GetStageTotalScoreRankRange", args, result)
	if err != nil {
		log.Error("RPC ### Player[%v] get stage total score rank list range[%v,%v] error[%v]", p.Id, rank_start, rank_num, err.Error())
		return nil
	}
	return
}

// 排行榜关卡积分更新
func (p *Player) rpc_rank_update_stage_score(stage_id, score int32) (result *rpc_common.H2R_RankStageScoreResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}
	args := &rpc_common.H2R_RankStageScore{}
	args.PlayerId = p.Id
	args.PlayerLevel = p.db.Info.GetLvl()
	args.StageId = stage_id
	args.StageScore = score
	result = &rpc_common.H2R_RankStageScoreResult{}
	err := rpc_client.Call("H2R_RankingListProc.UpdateStageScore", args, result)
	if err != nil {
		log.Error("RPC ### Player[%v] update stage score[%v] error[%v]", p.Id, score, err.Error())
		return nil
	}
	return
}

// 获取关卡积分更新
func (p *Player) rpc_ranklist_stage_score(stage_id, rank_start, rank_num int32) (result *rpc_common.H2R_RanklistGetStageScoreResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}

	args := &rpc_common.H2R_RanklistGetStageScore{}
	args.PlayerId = p.Id
	args.StageId = stage_id
	args.RankStart = rank_start
	args.RankNum = rank_num
	result = &rpc_common.H2R_RanklistGetStageScoreResult{}
	err := rpc_client.Call("H2R_RankingListProc.GetStageScoreRankRange", args, result)
	if err != nil {
		log.Error("RPC ### Player[%v] get stage[%v] score rank list range[%v,%v] error[%v]", p.Id, stage_id, rank_start, rank_num, err.Error())
		return nil
	}
	return
}

// 排行榜魅力值更新
func (p *Player) rpc_rank_update_charm(charm int32) (result *rpc_common.H2R_RankCharmResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}
	args := &rpc_common.H2R_RankCharm{}
	args.PlayerId = p.Id
	args.PlayerLevel = p.db.Info.GetLvl()
	args.Charm = charm
	result = &rpc_common.H2R_RankCharmResult{}
	err := rpc_client.Call("H2R_RankingListProc.UpdateCharm", args, result)
	if err != nil {
		log.Error("RPC ### Player[%v] update charm[%v] error[%v]", p.Id, charm, err.Error())
		return nil
	}
	return
}

// 获取魅力值排行榜
func (p *Player) rpc_ranklist_charm(rank_start, rank_num int32) (result *rpc_common.H2R_RanklistGetCharmResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}
	args := &rpc_common.H2R_RanklistGetCharm{}
	args.PlayerId = p.Id
	args.RankStart = rank_start
	args.RankNum = rank_num
	result = &rpc_common.H2R_RanklistGetCharmResult{}
	err := rpc_client.Call("H2R_RankingListProc.GetCharmRankRange", args, result)
	if err != nil {
		log.Error("RPC ### Player[%v] get charm rank list range[%v,%v] error[%v]", p.Id, rank_start, rank_num, err.Error())
		return nil
	}
	return
}

// 排行榜被赞更新
func (p *Player) rpc_rank_update_zaned(player_id, zan int32) (result *rpc_common.H2R_RankZanedResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}
	args := &rpc_common.H2R_RankZaned{}
	args.PlayerId = player_id
	args.Zaned = zan
	result = &rpc_common.H2R_RankZanedResult{}
	err := rpc_client.Call("H2R_RankingListProc.UpdateZaned", args, result)
	if err != nil {
		log.Error("RPC ### Player[%v] update player[%v] zaned[%v] error[%v]", p.Id, player_id, zan, err.Error())
		return nil
	}
	return
}

// 获取被赞排行榜
func (p *Player) rpc_ranklist_get_zaned(rank_start, rank_num int32) (result *rpc_common.H2R_RanklistGetZanedResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}
	args := &rpc_common.H2R_RanklistGetZaned{}
	args.PlayerId = p.Id
	args.RankStart = rank_start
	args.RankNum = rank_num
	result = &rpc_common.H2R_RanklistGetZanedResult{}
	err := rpc_client.Call("H2R_RankingListProc.GetZanedRankRange", args, result)
	if err != nil {
		log.Error("RPC ### Player[%v] get zaned rank list range[%v,%v] error[%v]", p.Id, rank_start, rank_num, err.Error())
		return nil
	}
	return
}

// 删除排名
func (p *Player) rpc_delete_rank(rank_type int32, param int32) (result *rpc_common.H2R_RankDeleteResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}
	args := &rpc_common.H2R_RankDelete{}
	args.PlayerId = p.Id
	args.RankType = rank_type
	args.Param = param
	result = &rpc_common.H2R_RankDeleteResult{}
	err := rpc_client.Call("H2R_RankingListProc.RankDelete", args, result)
	if err != nil {
		log.Error("RPC ### Player[%v] delete param[%v] for rank_type[%v] error[%v]", p.Id, param, rank_type, err.Error())
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
}

// 公告
func (p *Player) rpc_anouncement(msg_type int32, msg_param int32, msg_text string) (result *rpc_common.H2H_AnouncementResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}

	args := &rpc_common.H2H_Anouncement{}
	args.FromPlayerId = p.Id
	args.MsgType = msg_type
	args.MsgParam1 = msg_param
	args.MsgText = msg_text

	result = &rpc_common.H2H_AnouncementResult{}
	err := rpc_client.Call("H2H_GlobalProc.Anouncement", args, result)
	if err != nil {
		log.Error("RPC ### Player[%v] anouncement error[%v]", p.Id, err.Error())
	}

	return
}

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
	err := rpc_client.Call("H2R_GlobalProc.ChargeSave", args, result)
	if err != nil {
		log.Error("RPC ### Player[%v] charge save err[%v]", p.Id, err.Error())
	}
	return
}
