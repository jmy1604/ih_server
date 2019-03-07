package main

import (
	"errors"
	"fmt"
	"ih_server/libs/log"
	"ih_server/proto/gen_go/client_message"
	"ih_server/src/rpc_proto"
)

// 通用请求函数
func (this *HallServer) rpc_g2g_get(from_player_id, to_player_id, msg_id int32, msg_data []byte) (result_data []byte, err_code int32) {
	if this.rpc_client == nil {
		return nil, -1
	}
	var arg = rpc_proto.G2G_GetRequest{
		FromPlayerId: from_player_id,
		ToPlayerId:   to_player_id,
		MsgId:        msg_id,
		MsgData:      msg_data,
	}
	var result rpc_proto.G2G_GetResponse
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

// 请求多个玩家
func (this *HallServer) rpc_g2g_get_multi(from_player_id int32, to_player_ids []int32, msg_id int32, msg_data []byte) (datas []*rpc_proto.ServerResponseData) {
	if this.rpc_client == nil {
		log.Error("!!! rpc client is null")
		return nil
	}
	var arg = rpc_proto.G2G_MultiGetRequest{
		FromPlayerId: from_player_id,
		ToPlayerIds:  to_player_ids,
		MsgId:        msg_id,
		MsgData:      msg_data,
	}
	var result rpc_proto.G2G_MultiGetResponse
	err := this.rpc_client.Call("G2G_CommonProc.MultiGet", &arg, &result)
	if err != nil {
		log.Error("rpc_g2g_get_multi error(%v)", err.Error())
	} else {
		log.Trace("rpc_g2g_get_multi: arg %v, result %v", arg, result)
	}
	datas = result.Datas
	return
}

// 游戏服到游戏服通用rpc调用
type G2G_CommonProc struct {
}

func (this *G2G_CommonProc) Get(arg *rpc_proto.G2G_GetRequest, result *rpc_proto.G2G_GetResponse) (err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Stack(e)
		}
	}()

	handler := id2rpc_funcs[arg.MsgId]
	if handler == nil {
		err_str := fmt.Sprintf("RPC G2G_CommonProc.Get not found msg %v handler", arg.MsgId)
		log.Error(err_str)
		err = errors.New(err_str)
		return
	}

	result.Data.ResultData, result.Data.ErrorCode = handler(arg.ToPlayerId, arg.MsgData)

	log.Trace("RPC G2G_CommonProc.Get(%v,%v)", arg, result)

	return
}

// 注意：result参数是返回单个结果
func (this *G2G_CommonProc) MultiGet(arg *rpc_proto.G2G_MultiGetRequest, result *rpc_proto.G2G_GetResponse) (err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Stack(e)
		}
	}()

	handler := id2rpc_mfuncs[arg.MsgId]
	if handler == nil {
		err_str := fmt.Sprintf("RPC G2G_CommonProc.MultiGet not found msg %v handler", arg.MsgId)
		log.Error(err_str)
		err = errors.New(err_str)
		return
	}

	result.Data.ResultData, result.Data.ErrorCode = handler(arg.ToPlayerIds, arg.MsgData)

	log.Trace("RPC G2G_CommonProc.MultiGet(%v,%v)", arg, result)

	return
}
