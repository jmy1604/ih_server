package main

import (
	"errors"
	"fmt"
	"ih_server/libs/log"
	"ih_server/libs/rpc"
	"ih_server/libs/utils"
	"ih_server/src/rpc_common"
	"strconv"
	"sync"
	"time"
)

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

/* 昵称和ID RPC调用 */
type H2R_NickIdProc struct {
}

// 通知昵称
func (this *H2R_NickIdProc) AddIdNick(args *rpc_common.H2R_AddIdNick, result *rpc_common.H2R_AddIdNickResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	if !global_data.AddIdNick(args.Id, args.Nick) {
		err_str := fmt.Sprintf("增加昵称[%v,%v]失败", args.Nick, args.Id)
		log.Error(err_str)
		return errors.New(err_str)
	}
	return nil
}

// 修改昵称
func (this *H2R_NickIdProc) RenameNick(args *rpc_common.H2R_RenameNick, result *rpc_common.H2R_RenameNickResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	err_code := global_data.RenameNick(args.FromPlayerId, args.NewNick)
	if err_code < 0 {
		log.Error("修改昵称[%v,%v]失败, error[%v]", args.OldNick, args.NewNick, err_code)
	}
	result.Error = err_code
	return nil
}

/* 好友RPC调用 */
type H2R_FriendProc struct {
}

func (this *H2R_FriendProc) search_player(nick string, id int32, result *rpc_common.H2R_SearchFriendResult) error {
	rpc_client := GetRpcClientByPlayerId(id)
	if rpc_client == nil {
		err_str := fmt.Sprintf("无法获取玩家[%v,%v]相应的大厅", nick, id)
		return errors.New(err_str)
	}

	// 获取玩家数据
	call_args := &rpc_common.R2H_SearchPlayer{
		Id: id,
	}
	call_result := &rpc_common.R2H_SearchPlayerResult{}
	err := rpc_client.Call("R2H_PlayerProc.GetInfoToSearch", call_args, call_result)
	if err != nil {
		return err
	}

	r := &rpc_common.H2R_SearchPlayerInfo{
		Head:      call_result.Head,
		Id:        id,
		Nick:      nick,
		Level:     call_result.Level,
		VipLevel:  call_result.VipLevel,
		LastLogin: call_result.LastLogin,
	}

	result.Players = append(result.Players, r)

	return nil
}

// 用昵称查找
func (this *H2R_FriendProc) SearchByNick(args *rpc_common.H2R_SearchFriendByNick, result *rpc_common.H2R_SearchFriendResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	var err error
	ids := global_data.GetIdsByNick(args.Nick)
	if ids != nil {
		for i := 0; i < len(ids); i++ {
			err = this.search_player(args.Nick, ids[i], result)
			if err != nil {
				log.Debug("RPC @@@ Player[%v] search player by nick[%v] id[%v] error[%v]", args.FromPlayerId, args.Nick, ids[i], err.Error())
			}
		}
	}

	log.Debug("RPC @@@ Player[%v] searched nick[%v] ids[%v]", args.FromPlayerId, args.Nick, ids)

	return nil
}

// 用Id查找
func (this *H2R_FriendProc) SearchById(args *rpc_common.H2R_SearchFriendById, result *rpc_common.H2R_SearchFriendResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	nick, o := global_data.GetNickById(args.Id)
	if !o {
		return nil
	}
	return this.search_player(nick, args.Id, result)
}

// 关键字查找
func (this *H2R_FriendProc) SearchByKey(args *rpc_common.H2R_SearchFriendByKey, result *rpc_common.H2R_SearchFriendResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	nick_args := &rpc_common.H2R_SearchFriendByNick{
		FromPlayerId: args.FromPlayerId,
		Nick:         args.Key,
	}

	err := this.SearchByNick(nick_args, result)
	if err != nil {
		log.Warn("RPC @@@ Player[%v] search friend by nick key[%v] error[%v]", args.FromPlayerId, args.Key, err.Error())
	} else {

	}

	var a int
	a, err = strconv.Atoi(args.Key)
	if err == nil {
		call_args := &rpc_common.H2R_SearchFriendById{
			Id: int32(a),
		}
		err = this.SearchById(call_args, result)
		if err != nil {
			log.Warn("RPC @@@ Player[%v] search friend by id key[%v] error[%v]", args.FromPlayerId, a, err.Error())
		} else {
		}
	}

	log.Debug("RPC @@@ Player[%v] searched players: %v", args.FromPlayerId, result.Players)
	return nil
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

// 昵称申请好友
func (this *H2R_FriendProc) AddFriendByName(args *rpc_common.H2R_AddFriendByName, result *rpc_common.H2R_AddFriendResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	add_ids := global_data.GetIdsByNick(args.AddPlayerName)
	if add_ids == nil {
		err_str := fmt.Sprintf("找不到昵称[%v]对应的ID，申请好友失败", args.AddPlayerName)
		return errors.New(err_str)
	}

	rpc_client := GetRpcClientByPlayerId(add_ids[0])
	if rpc_client == nil {
		err_str := fmt.Sprintf("通过玩家ID[%v]获取rpc客户端失败", add_ids[0])
		return errors.New(err_str)
	}

	call_args := rpc_common.R2H_AddFriendById{}
	call_args.AddPlayerId = add_ids[0]
	call_args.PlayerId = args.PlayerId
	call_args.PlayerName = args.PlayerName
	call_result := &rpc_common.R2H_AddFriendResult{}

	return rpc_client.Call("R2H_FriendProc.AddFriendById", call_args, call_result)
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

/* 商店调用 */
type H2R_ShopProc struct {
}

//// 关卡调用
type H2R_StageProc struct {
}

// 获取好友的关卡得分
func (this *H2R_StageProc) GetFriendsStageInfo(args *rpc_common.H2R_FriendsStagePassDataRequest, result *rpc_common.H2R_FriendsStagePassDataResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	var call_args = rpc_common.R2H_PlayerStageInfoReq{}
	var call_result = rpc_common.R2H_PlayerStageInfoResult{}

	result.StageInfos = make([]*rpc_common.H2R_PlayerStageInfo, len(args.FriendIds))
	for i, id := range args.FriendIds {
		rp := GetRpcClientByPlayerId(id)
		if rp == nil {
			log.Warn("玩家id[%v]没有对应的大厅rpc客户端", id)
			continue
		}

		// 请求大厅获取玩家关卡数据
		call_args.PlayerId = id
		call_args.StageId = args.StageId
		e := rp.Call("R2H_PlayerStageInfoProc.Do", call_args, &call_result)
		if e != nil {
			log.Error("获取玩家[%v]关卡数据失败")
			continue
		}

		result.StageInfos[i] = &rpc_common.H2R_PlayerStageInfo{}
		result.StageInfos[i].Head = call_result.Head
		result.StageInfos[i].Name = call_result.Nick
		result.StageInfos[i].PlayerId = id
		result.StageInfos[i].TopScore = call_result.TopScore
	}

	log.Info("get player[%v] friends[%v] stage rank info", args.PlayerId, args.FriendIds)
	return nil
}

type H2H_PlayerProc struct {
}

// 更新玩家基本信息
func (this *H2H_PlayerProc) UpdateBaseInfo(args *rpc_common.H2H_BaseInfo, result *rpc_common.H2H_BaseInfoResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	if _, o := global_data.GetNickById(args.FromPlayerId); !o {
		if !global_data.AddIdNick(args.FromPlayerId, args.Nick) {
			log.Error("add player id nick set[%,%v] failed", args.FromPlayerId, args.Nick)
		}
	} else {
		if args.Nick != "" {
			err_code := global_data.RenameNick(args.FromPlayerId, args.Nick)
			if err_code < 0 {
				log.Error("modify player id nick set[%v,%v] error[%v]", args.FromPlayerId, args.Nick, err_code)
			}
		}
	}

	local_rpc_client := GetRpcClientByPlayerId(args.FromPlayerId)
	for _, r := range server.hall_rpc_clients {
		if r.rpc_client != nil && local_rpc_client != r.rpc_client {
			err := r.rpc_client.Call("H2H_PlayerProc.UpdateBaseInfo", args, result)
			if err != nil {
				err_str := fmt.Sprintf("@@@ H2H_PlayerProc::UpdateBaseInfo Player[%v] update base info error[%v]", args.FromPlayerId, err.Error())
				return errors.New(err_str)
			}
		}
	}

	log.Debug("@@@ H2H_PlayerProc::UpdateBaseInfo Player[%v] updated base info", args.FromPlayerId)

	return nil
}

// 点赞
func (this *H2H_PlayerProc) Zan(args *rpc_common.H2H_ZanPlayer, result *rpc_common.H2H_ZanPlayerResult) error {
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

	err := rpc_client.Call("H2H_PlayerProc.Zan", args, result)
	if err != nil {
		return err
	}

	log.Debug("@@@ player[%v] zan player[%v]", args.FromPlayerId, args.ToPlayerId)

	return nil
}

type H2R_RankingListProc struct {
	stage_total_score_ranking_list  *utils.CommonRankingList
	stage_score_ranking_list_map    map[int32]*utils.CommonRankingList
	stage_score_ranking_list_locker *sync.RWMutex
	charm_ranking_list              *utils.CommonRankingList
	zaned_ranking_list              *utils.CommonRankingList
	total_score_item_pool           *sync.Pool
	score_item_pool                 *sync.Pool
	charm_item_pool                 *sync.Pool
	ouqi_item_pool                  *sync.Pool
	zaned_item_pool                 *sync.Pool
}

const RANKING_LIST_MAX_RANK int32 = 10000

func (this *H2R_RankingListProc) Init() {
	this.stage_total_score_ranking_list = utils.NewCommonRankingList(&RankStageTotalScoreItem{}, RANKING_LIST_MAX_RANK)
	this.stage_score_ranking_list_map = make(map[int32]*utils.CommonRankingList)
	this.stage_score_ranking_list_locker = &sync.RWMutex{}
	this.charm_ranking_list = utils.NewCommonRankingList(&RankCharmItem{}, RANKING_LIST_MAX_RANK)
	this.zaned_ranking_list = utils.NewCommonRankingList(&RankZanedItem{}, RANKING_LIST_MAX_RANK)
	this.total_score_item_pool = &sync.Pool{
		New: func() interface{} {
			return &rpc_common.H2R_RankStageTotalScore{}
		},
	}
	this.score_item_pool = &sync.Pool{
		New: func() interface{} {
			return &rpc_common.H2R_RankStageScore{}
		},
	}
	this.charm_item_pool = &sync.Pool{
		New: func() interface{} {
			return &rpc_common.H2R_RankCharm{}
		},
	}
	this.zaned_item_pool = &sync.Pool{
		New: func() interface{} {
			return &rpc_common.H2R_RankZaned{}
		},
	}
	//RankItemPoolInit()
}

// 更新关卡总分排行
func (this *H2R_RankingListProc) UpdateStageTotalScore(args *rpc_common.H2R_RankStageTotalScore, result *rpc_common.H2R_RankStageTotalScoreResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	var item = RankStageTotalScoreItem{
		PlayerId:        args.PlayerId,
		PlayerLevel:     args.PlayerLevel,
		StageTotalScore: args.TotalScore,
		SaveTime:        int32(time.Now().Unix()),
	}

	before_first_item := this.stage_total_score_ranking_list.GetByRank(1)
	if !this.stage_total_score_ranking_list.Update(&item) {
		err_str := fmt.Sprintf("StageTotalScore RankList update player[%v] total score[%v] failed", args.PlayerId, args.TotalScore)
		return errors.New(err_str)
	}

	global_data.UpdateRankStageTotalScore(&item)

	this.anouncement_stage_total_score_first_rank(before_first_item)

	log.Debug("@@@ H2R_RankingListProc::UpdateStageTotalScore Player[%v] TotalScore[%v]", args.PlayerId, args.TotalScore)

	return nil
}

// 获取关卡总分排行
func (this *H2R_RankingListProc) GetStageTotalScoreRankRange(args *rpc_common.H2R_RanklistGetStageTotalScore, result *rpc_common.H2R_RanklistGetStageTotalScoreResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	start, num := this.stage_total_score_ranking_list.GetRankRange(args.RankStart, args.RankNum)
	if start == 0 {
		result.RankItems = make([]*rpc_common.H2R_RankStageTotalScore, 0)
	} else {
		nodes := make([]interface{}, num)
		for i := int32(0); i < num; i++ {
			nodes[i] = this.total_score_item_pool.Get().(*rpc_common.H2R_RankStageTotalScore)
		}
		num := this.stage_total_score_ranking_list.GetRangeNodes(start, num, nodes)
		if num == 0 {
			err_str := fmt.Sprintf("@@@ Player[%v] GetStageTotalScoreRankList failed", args.PlayerId)
			return errors.New(err_str)
		}

		result.RankItems = make([]*rpc_common.H2R_RankStageTotalScore, num)
		for i := int32(0); i < num; i++ {
			result.RankItems[i] = nodes[i].(*rpc_common.H2R_RankStageTotalScore)
		}

		var self_score_interface interface{}
		result.SelfRank, self_score_interface = this.stage_total_score_ranking_list.GetRankAndValue(args.PlayerId)
		if self_score_interface != nil {
			result.SelfTotalScore = self_score_interface.(int32)
		}
	}

	log.Debug("@@@ H2R_RankingListProc::GetStageTotalScoreRankRange Player[%v] get Rankinglist[rank_start:%v rank_num:%v] SelfRank[%v]", args.PlayerId, args.RankStart, args.RankNum, result.SelfRank)
	return nil
}

func (this *H2R_RankingListProc) GetStageScoreRankList(stage_id int32) *utils.CommonRankingList {
	this.stage_score_ranking_list_locker.RLock()
	rank_list := this.stage_score_ranking_list_map[stage_id]
	this.stage_score_ranking_list_locker.RUnlock()
	if rank_list == nil {
		this.stage_score_ranking_list_locker.Lock()
		rank_list = utils.NewCommonRankingList(&RankStageScoreItem{}, RANKING_LIST_MAX_RANK)
		this.stage_score_ranking_list_map[stage_id] = rank_list
		this.stage_score_ranking_list_locker.Unlock()
	}
	return rank_list
}

// 更新关卡积分排行
func (this *H2R_RankingListProc) UpdateStageScore(args *rpc_common.H2R_RankStageScore, result *rpc_common.H2R_RankStageScoreResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	rank_list := this.GetStageScoreRankList(args.StageId)

	var update_item = RankStageScoreItem{
		PlayerId:    args.PlayerId,
		PlayerLevel: args.PlayerLevel,
		StageId:     args.StageId,
		StageScore:  args.StageScore,
		SaveTime:    int32(time.Now().Unix()),
	}

	before_first_item := rank_list.GetByRank(1)
	if !rank_list.Update(&update_item) {
		err_str := fmt.Sprintf("@@@ Player[%v] update stage[%v] score[%v] failed", args.PlayerId, args.StageId, args.StageScore)
		return errors.New(err_str)
	}

	global_data.UpdateRankStageScore(&update_item)

	this.anouncement_stage_score_first_rank(rank_list, args.StageId, before_first_item)

	log.Debug("@@@ H2R_RankingListProc::UpdateStageScore Player[%v] Score[%v]", args.PlayerId, args.StageScore)
	return nil
}

// 获取关卡积分排行
func (this *H2R_RankingListProc) GetStageScoreRankRange(args *rpc_common.H2R_RanklistGetStageScore, result *rpc_common.H2R_RanklistGetStageScoreResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	rank_list := this.GetStageScoreRankList(args.StageId)
	start, num := rank_list.GetRankRange(args.RankStart, args.RankNum)
	if start == 0 {
		result.RankItems = make([]*rpc_common.H2R_RankStageScore, 0)
	} else {
		items := make([]interface{}, num)
		for i := int32(0); i < num; i++ {
			items[i] = this.score_item_pool.Get().(*rpc_common.H2R_RankStageScore)
		}
		num = rank_list.GetRangeNodes(start, num, items)
		if num == 0 {
			err_str := fmt.Sprintf("@@@ Player[%v] get stage[%v] score rank list failed", args.PlayerId, args.StageId)
			return errors.New(err_str)
		}

		result.RankItems = make([]*rpc_common.H2R_RankStageScore, num)
		for i := int32(0); i < num; i++ {
			result.RankItems[i] = items[i].(*rpc_common.H2R_RankStageScore)
		}

		var self_score_interface interface{}
		result.SelfRank, self_score_interface = rank_list.GetRankAndValue(args.PlayerId)
		if self_score_interface != nil {
			result.SelfScore = self_score_interface.(int32)
		}
	}

	log.Debug("@@@ H2R_RankingListProc::GetStageScoreRankRange Player[%v] get Rankinglist[rank_start:%v rank_num:%v] SelfRank[%v]", args.PlayerId, args.RankStart, args.RankNum, result.SelfRank)
	return nil
}

// 更新魅力排行
func (this *H2R_RankingListProc) UpdateCharm(args *rpc_common.H2R_RankCharm, result *rpc_common.H2R_RankCharmResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	var update_item = RankCharmItem{
		PlayerId:    args.PlayerId,
		PlayerLevel: args.PlayerLevel,
		Charm:       args.Charm,
		SaveTime:    int32(time.Now().Unix()),
	}

	before_first_item := this.charm_ranking_list.GetByRank(1)
	if args.Charm == 0 {
		if !this.charm_ranking_list.Delete(update_item.GetKey()) {
			log.Warn("@@@ Player[%v] delete charm[%v] for rank list failed", args.PlayerId, args.Charm)
		}
	} else {
		if !this.charm_ranking_list.Update(&update_item) {
			err_str := fmt.Sprintf("@@@ Player[%v] update charm[%v] for rank list failed", args.PlayerId, args.Charm)
			return errors.New(err_str)
		}
	}

	global_data.UpdateRankCharm(&update_item)

	this.anouncement_charm_first_rank(before_first_item)

	log.Debug("@@@ H2R_RankingListProc::UpdateCharm Player[%v] updated charm[%v]", args.PlayerId, args.Charm)
	return nil
}

// 获取魅力排行榜
func (this *H2R_RankingListProc) GetCharmRankRange(args *rpc_common.H2R_RanklistGetCharm, result *rpc_common.H2R_RanklistGetCharmResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	start, num := this.charm_ranking_list.GetRankRange(args.RankStart, args.RankNum)
	if start == 0 {
		result.RankItems = make([]*rpc_common.H2R_RankCharm, 0)
	} else {
		items := make([]interface{}, num)
		for i := int32(0); i < num; i++ {
			items[i] = this.charm_item_pool.Get().(*rpc_common.H2R_RankCharm)
		}
		num := this.charm_ranking_list.GetRangeNodes(start, num, items)
		if num == 0 {
			err_str := fmt.Sprintf("@@@ Player[%v] get charm rank list failed", args.PlayerId)
			return errors.New(err_str)
		}

		result.RankItems = make([]*rpc_common.H2R_RankCharm, num)
		for i := int32(0); i < num; i++ {
			result.RankItems[i] = items[i].(*rpc_common.H2R_RankCharm)
		}

		var self_charm interface{}
		result.SelfRank, self_charm = this.charm_ranking_list.GetRankAndValue(args.PlayerId)
		if self_charm != nil {
			result.SelfCharm = self_charm.(int32)
		}
	}

	log.Debug("@@@ H2R_RankingListProc::GetCharmRankRange Player[%v] get charm rank_list[rank_start:%v, rank_num:%v]", args.PlayerId, args.RankStart, args.RankNum)
	return nil
}

// 更新被赞排行榜
func (this *H2R_RankingListProc) UpdateZaned(args *rpc_common.H2R_RankZaned, result *rpc_common.H2R_RankZanedResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	var update_item = RankZanedItem{
		PlayerId:    args.PlayerId,
		PlayerLevel: args.PlayerLevel,
		Zaned:       args.Zaned,
		SaveTime:    int32(time.Now().Unix()),
	}

	before_first_item := this.zaned_ranking_list.GetByRank(1)
	if !this.zaned_ranking_list.Update(&update_item) {
		err_str := fmt.Sprintf("@@@ Player[%v] update zan[%v] failed", args.PlayerId)
		return errors.New(err_str)
	}

	global_data.UpdateRankZaned(&update_item)

	this.anouncement_zaned_first_rank(before_first_item)

	log.Debug("@@@ H2R_RankingListProc::UpdateZaned Player[%v] updated zan[%v]", args.PlayerId, args.Zaned)

	return nil
}

// 获取被赞排行榜
func (this *H2R_RankingListProc) GetZanedRankRange(args *rpc_common.H2R_RanklistGetZaned, result *rpc_common.H2R_RanklistGetZanedResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	start, num := this.zaned_ranking_list.GetRankRange(args.RankStart, args.RankNum)
	if start == 0 {
		result.RankItems = make([]*rpc_common.H2R_RankZaned, 0)
	} else {
		items := make([]interface{}, num)
		for i := int32(0); i < num; i++ {
			items[i] = this.zaned_item_pool.Get().(*rpc_common.H2R_RankZaned)
		}
		num = this.zaned_ranking_list.GetRangeNodes(start, num, items)
		if num == 0 {
			err_str := fmt.Sprintf("@@@ Player[%v] get zaned rank list failed", args.PlayerId)
			return errors.New(err_str)
		}

		result.RankItems = make([]*rpc_common.H2R_RankZaned, len(items))
		for i := 0; i < len(items); i++ {
			result.RankItems[i] = items[i].(*rpc_common.H2R_RankZaned)
		}
		var self_zaned interface{}
		result.SelfRank, self_zaned = this.zaned_ranking_list.GetRankAndValue(args.PlayerId)
		if self_zaned != nil {
			result.SendZaned = self_zaned.(int32)
		}
	}
	log.Debug("@@@ H2R_RankingListProc::GetZanedRankRange Player[%v] get zaned rank list", args.PlayerId)
	return nil
}

func rpc_call_anouncement_player_first_rank(rank_type int32, rank_param int32, player_id int32, player_name string, player_level int32) error {
	args := rpc_common.R2H_RanklistPlayerFirstRank{}
	args.PlayerId = player_id
	//args.PlayerName = player_name
	//args.PlayerLevel = player_level
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

func (this *H2R_RankingListProc) anouncement_stage_total_score_first_rank(before_first_item utils.SkiplistNode) {
	first_item := this.stage_total_score_ranking_list.GetByRank(1)
	if before_first_item == nil || !before_first_item.KeyEqual(first_item) {
		first_item_node := first_item.(*RankStageScoreItem)
		if first_item_node != nil {
			nick, _ := global_data.GetNickById(first_item_node.PlayerId)
			rpc_call_anouncement_player_first_rank(1, 0, first_item_node.PlayerId, nick, first_item_node.PlayerLevel)
		}
	}
}

func (this *H2R_RankingListProc) anouncement_stage_score_first_rank(rank_list *utils.CommonRankingList, stage_id int32, before_first_item utils.SkiplistNode) {
	first_item := rank_list.GetByRank(1)
	if before_first_item == nil || !before_first_item.KeyEqual(first_item) {
		first_item_node := first_item.(*RankStageScoreItem)
		if first_item_node != nil {
			nick, _ := global_data.GetNickById(first_item_node.PlayerId)
			rpc_call_anouncement_player_first_rank(2, stage_id, first_item_node.PlayerId, nick, first_item_node.PlayerLevel)
		}
	}
}

func (this *H2R_RankingListProc) anouncement_charm_first_rank(before_first_item utils.SkiplistNode) {
	first_item := this.charm_ranking_list.GetByRank(1)
	if before_first_item == nil || !before_first_item.KeyEqual(first_item) {
		first_item_node := first_item.(*RankCharmItem)
		if first_item_node != nil {
			nick, _ := global_data.GetNickById(first_item_node.PlayerId)
			rpc_call_anouncement_player_first_rank(3, 0, first_item_node.PlayerId, nick, first_item_node.PlayerLevel)
		}
	}
}

func (this *H2R_RankingListProc) anouncement_zaned_first_rank(before_first_item utils.SkiplistNode) {
	first_item := this.zaned_ranking_list.GetByRank(1)
	if before_first_item == nil || !before_first_item.KeyEqual(first_item) {
		first_item_node := first_item.(*RankZanedItem)
		if first_item_node != nil {
			nick, _ := global_data.GetNickById(first_item_node.PlayerId)
			rpc_call_anouncement_player_first_rank(5, 0, first_item_node.PlayerId, nick, first_item_node.PlayerLevel)
		}
	}
}

// 删除排名
func (this *H2R_RankingListProc) RankDelete(args *rpc_common.H2R_RankDelete, result *rpc_common.H2R_RankDeleteResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	if args.RankType == RANKING_LIST_TYPE_STAGE_TOTAL_SCORE { // 关卡总分
		before_first_item := this.stage_total_score_ranking_list.GetByRank(1)
		if !this.stage_total_score_ranking_list.Delete(args.PlayerId) {
			log.Error("@@@ Player[%v] delete rank for total score rank list failed", args.PlayerId)
		}
		global_data.DeleteRankStageTotalScore(args.PlayerId)
		this.anouncement_stage_total_score_first_rank(before_first_item)
	} else if args.RankType == RANKING_LIST_TYPE_STAGE_SCORE { // 关卡积分
		rank_list := this.GetStageScoreRankList(args.Param)
		if rank_list == nil {
			err_str := fmt.Sprintf("@@@ Player[%v] get ranklist[%v] failed", args.PlayerId, args.RankType)
			return errors.New(err_str)
		}
		before_first_item := rank_list.GetByRank(1)
		if !rank_list.Delete(args.PlayerId) {
			log.Error("@@@ Player[%v] delet rank for stage[%v] score rank list failed", args.PlayerId, args.Param)
		}
		global_data.DeleteRankStageScore(args.PlayerId, args.Param)
		this.anouncement_stage_score_first_rank(rank_list, args.Param, before_first_item)
	} else if args.RankType == RANKING_LIST_TYPE_CHARM { // 魅力
		before_first_item := this.charm_ranking_list.GetByRank(1)
		if !this.charm_ranking_list.Delete(args.PlayerId) {
			log.Error("@@@ Player[%v] delete rank for charm rank list failed", args.PlayerId)
		}
		global_data.DeleteRankCharm(args.PlayerId)
		this.anouncement_charm_first_rank(before_first_item)
	} else if args.RankType == RANKING_LIST_TYPE_ZANED { // 被赞
		before_first_item := this.zaned_ranking_list.GetByRank(1)
		if !this.zaned_ranking_list.Delete(args.PlayerId) {
			log.Error("@@@ Player[%v] delete zaned rank list failed", args.PlayerId)
		}
		global_data.DeleteRankZaned(args.PlayerId)
		this.anouncement_zaned_first_rank(before_first_item)
	} else {
		log.Warn("@@@ H2R_RankingListProc::RankDelete not found RankType[%v]", args.RankType)
	}
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

var ranking_list_proc *H2R_RankingListProc

// 初始化
func (this *RpcServer) init_proc_service() bool {
	this.rpc_service = &rpc.Service{}

	if !this.rpc_service.Register(&H2H_CallProc{}) {
		return false
	}

	// 监听RPC调用注册
	if !this.rpc_service.Register(&H2R_ListenRPCProc{}) {
		return false
	}

	// 世界聊天调用注册
	if !this.rpc_service.Register(&H2H_GlobalProc{}) {
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
