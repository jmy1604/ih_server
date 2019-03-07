package main

import (
	"errors"
	"ih_server/libs/log"
	"ih_server/libs/rpc"
	"ih_server/src/rpc_proto"
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
	rpc_proto.RegisterRpcUserType()

	this.rpc_client = rpc.NewClient()
	var on_connect rpc.OnConnectFunc = func(args interface{}) {
		rpc_client := args.(*rpc.Client)
		proc_string := "H2R_ListenRPCProc.Do"
		var arg = rpc_proto.H2R_ListenIPNoitfy{config.ListenRpcServerIP, config.ServerId}
		var result = rpc_proto.H2R_ListenIPResult{}
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
	transfer_args := &rpc_proto.H2R_Transfer{}
	transfer_args.Method = method
	transfer_args.Args = args
	transfer_args.ReceivePlayerId = receive_player_id
	transfer_reply := &rpc_proto.H2R_TransferResult{}
	transfer_reply.Result = reply

	log.Debug("@@@@@ #####  transfer_args[%v]  transfer_reply[%v]", transfer_args.Args, transfer_reply.Result)

	err := this.rpc_client.Call("H2H_CallProc.Do", transfer_args, transfer_reply)
	if err != nil {
		log.Error("RPC @@@ H2H_CallProc.Do(%v,%v) error(%v)", transfer_args, transfer_reply, err.Error())
	}
	return err
}

// 充值记录
func (p *Player) rpc_charge_save(channel int32, order_id, bundle_id, account string, player_id, pay_time int32, pay_time_str string) (result *rpc_proto.H2R_ChargeSaveResult) {
	rpc_client := get_rpc_client()
	if rpc_client == nil {
		return nil
	}

	var args = rpc_proto.H2R_ChargeSave{
		Channel:    channel,
		OrderId:    order_id,
		BundleId:   bundle_id,
		Account:    account,
		PlayerId:   player_id,
		PayTime:    pay_time,
		PayTimeStr: pay_time_str,
	}

	result = &rpc_proto.H2R_ChargeSaveResult{}
	err := rpc_client.Call("H2R_GlobalProc.ChargeSave", &args, result)
	if err != nil {
		log.Error("RPC ### Player[%v] charge save err[%v]", p.Id, err.Error())
	}
	return
}
