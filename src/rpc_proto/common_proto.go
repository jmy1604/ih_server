package rpc_proto

import (
	"ih_server/libs/log"
	"ih_server/libs/rpc"

	"ih_server/proto/gen_go/client_message"
)

const (
	OBJECT_TYPE_PLAYER = 1
	OBJECT_TYPE_GUILD  = 2
)

type ServerResponseData struct {
	ResultData []byte
	ErrorCode  int32
}

type G2G_GetRequest struct {
	FromPlayerId int32
	ObjectType   int32
	ObjectId     int32
	MsgId        int32
	MsgData      []byte
}

type G2G_GetResponse struct {
	Data ServerResponseData
}

type G2G_MultiGetRequest struct {
	FromPlayerId int32
	ObjectType   int32
	ObjectIds    []int32
	MsgId        int32
	MsgData      []byte
}

type G2G_MultiGetResponse struct {
	Datas []*ServerResponseData
}

type G2G_DataNotify struct {
	FromPlayerId int32
	ObjectType   int32
	ObjectId     []int32
	MsgId        int32
	MsgData      []byte
}

type G2G_DataNotifyResult struct {
	ErrorCode int32
}

type G2G_MultiDataNotify struct {
	FromPlayerId int32
	ObjectType   int32
	ObjectIds    []int32
	MsgId        int32
	MsgData      []byte
}

type G2G_MultiDataNotifyResult struct {
	ErrorCodes []int32
}

// 通用请求函数
func RpcCommonGet(rpc_client *rpc.Client, rpc_func_name string, from_player_id, object_type, object_id, msg_id int32, msg_data []byte) (result_data []byte, err_code int32) {
	var arg = G2G_GetRequest{
		FromPlayerId: from_player_id,
		ObjectType:   object_type,
		ObjectId:     object_id,
		MsgId:        msg_id,
		MsgData:      msg_data,
	}
	var result G2G_GetResponse
	err := rpc_client.Call(rpc_func_name, &arg, &result)
	if err != nil {
		err_code = int32(msg_client_message.E_ERR_REMOTE_FUNC_CALL_ERROR)
		log.Error("RpcCommonGet error(%v)", err.Error())
		return
	}

	result_data = result.Data.ResultData
	err_code = result.Data.ErrorCode

	log.Trace("RpcCommonGet: arg %v, result %v", arg, result)

	return
}

// 请求多个玩家
func RpcCommonMultiGet(rpc_client *rpc.Client, rpc_func_name string, from_player_id, object_type int32, object_ids []int32, msg_id int32, msg_data []byte) (datas []*ServerResponseData) {
	var arg = G2G_MultiGetRequest{
		FromPlayerId: from_player_id,
		ObjectType:   object_type,
		ObjectIds:    object_ids,
		MsgId:        msg_id,
		MsgData:      msg_data,
	}
	var result G2G_MultiGetResponse
	err := rpc_client.Call(rpc_func_name, &arg, &result)
	if err != nil {
		log.Error("RpcCommonMultiGet error(%v)", err.Error())
	} else {
		log.Trace("RpcCommonMultiGet: arg %v, result %v", arg, result)
	}
	datas = result.Datas
	return
}
