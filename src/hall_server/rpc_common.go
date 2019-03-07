package main

import (
	"errors"
	"fmt"
	"ih_server/libs/log"
	"ih_server/src/rpc_proto"
)

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

	result.Data.ResultData, result.Data.ErrorCode = handler(arg.ObjectId, arg.MsgData)

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

	result.Data.ResultData, result.Data.ErrorCode = handler(arg.ObjectIds, arg.MsgData)

	log.Trace("RPC G2G_CommonProc.MultiGet(%v,%v)", arg, result)

	return
}
