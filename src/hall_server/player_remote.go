package main

import (
	"ih_server/libs/log"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/rpc_message"
	"ih_server/src/rpc_common"

	"github.com/golang/protobuf/proto"
)

func _marshal_msg(msg proto.Message) (msg_data []byte, err error) {
	msg_data, err = proto.Marshal(msg)
	if nil != err {
		log.Error("!!!!! marshal msg err: %s", err.Error())
		return
	}
	return
}

func _unmarshal_msg(msg_data []byte, msg proto.Message) (err error) {
	err = proto.Unmarshal(msg_data, msg)
	if err != nil {
		log.Error("!!!!! unmarshal msg err: %v", err.Error())
		return
	}
	return
}

// 获取玩家信息
func remote_get_player_info(from_player_id, to_player_id int32) (resp *msg_rpc_message.G2GPlayerInfoResponse, err_code int32) {
	req := &msg_rpc_message.G2GPlayerInfoRequest{
		PlayerId: to_player_id,
	}

	var req_data, result_data []byte
	var err error

	req_data, err = _marshal_msg(req)
	if err != nil {
		err_code = -1
		return
	}

	result_data, err_code = hall_server.rpc_g2g_get(from_player_id, to_player_id, int32(msg_rpc_message.MSGID_G2G_PLAYER_INFO_REQUEST), req_data)
	if err_code < 0 {
		return
	}

	var response msg_rpc_message.G2GPlayerInfoResponse
	err = _unmarshal_msg(result_data, &response)
	if err != nil {
		err_code = -1
		return
	}

	resp = &response
	err_code = 1
	return
}

// 返回
func remote_get_player_info_response(req_data []byte) (resp_data []byte, err_code int32) {
	var req msg_rpc_message.G2GPlayerInfoRequest
	err := _unmarshal_msg(req_data, &req)
	if err != nil {
		err_code = -1
		return
	}

	player := player_mgr.GetPlayerById(req.GetPlayerId())
	if player == nil {
		log.Error("remote request get player info by id %v not found", req.GetPlayerId())
		err_code = int32(msg_client_message.E_ERR_PLAYER_NOT_EXIST)
		return
	}

	var response = msg_rpc_message.G2GPlayerInfoResponse{
		PlayerId: req.GetPlayerId(),
		UniqueId: player.db.GetUniqueId(),
		Account:  player.db.GetAccount(),
		Level:    player.db.GetLevel(),
		Head:     player.db.Info.GetHead(),
	}

	resp_data, err = _marshal_msg(&response)
	if err != nil {
		err_code = -1
		return
	}

	err_code = 1
	return
}

type rpc_func func([]byte) ([]byte, int32)
type multi_rpc_func func([]byte) []*rpc_common.ServerResponseData

var id2rpcfuncs = map[int32]rpc_func{
	int32(msg_rpc_message.MSGID_G2G_PLAYER_INFO_REQUEST): remote_get_player_info_response,
}

var id2multi_rpcfuncs = map[int32]multi_rpc_func{}
