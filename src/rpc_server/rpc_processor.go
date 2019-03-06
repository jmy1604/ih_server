package main

import (
	"errors"
	"fmt"
	"ih_server/libs/log"
	"ih_server/libs/rpc"
	"ih_server/src/rpc_common"
	"ih_server/src/share_data"
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

func split_player_ids_with_server(player_ids []int32) (serverid2players map[int32][]int32) {
	if player_ids == nil || len(player_ids) == 0 {
		return
	}

	for i := 0; i < len(player_ids); i++ {
		pid := player_ids[i]
		server_id := share_data.GetServerIdByPlayerId(pid)
		if !server_list.HasServerId(server_id) {
			continue
		}
		if serverid2players == nil {
			serverid2players = make(map[int32][]int32)
		}

		var players []int32 = serverid2players[server_id]
		if players == nil {
			players = []int32{pid}
		} else {
			players = append(players, pid)
		}
		serverid2players[server_id] = players
	}

	return
}

func (this *G2G_CommonProc) MultiGet(arg *rpc_common.G2G_MultiGetRequest, result *rpc_common.G2G_MultiGetResponse) (err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Stack(e)
		}
	}()

	var serverid2players map[int32][]int32 = split_player_ids_with_server(arg.ToPlayerIds)
	if serverid2players == nil {
		return errors.New(fmt.Sprintf("!!!!!! split players result is empty from player id %v", arg.FromPlayerId))
	}

	log.Trace("serverid2players %v", serverid2players)

	for sid, players := range serverid2players {
		rpc_client := GetRpcClientByServerId(sid)
		if rpc_client == nil {
			return errors.New(fmt.Sprintf("!!!!!! Not found rpc client by server id %v", sid))
		}
		arg.ToPlayerIds = players
		var tmp_result rpc_common.G2G_GetResponse
		err = rpc_client.Call("G2G_CommonProc.MultiGet", arg, &tmp_result)
		if err != nil {
			err_str := fmt.Sprintf("RPC @@@ G2G_CommonProc.MultiGet(%v,%v) error(%v)", arg, tmp_result, err.Error())
			log.Error(err_str)
			return errors.New(err_str)
		}
		result.Datas = append(result.Datas, &tmp_result.Data)
		log.Trace("RPC @@@ G2G_CommonProc.MultiGet(%v,%v)", arg, tmp_result)
	}

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
