package main

import (
	"ih_server/libs/log"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/rpc_message"
	"ih_server/src/rpc_proto"

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

func RemoteGetUsePB(from_player_id, object_type, object_id int32, req_msg_id int32, req_msg proto.Message, resp_msg proto.Message) (err_code int32) {
	req_data, err := _marshal_msg(req_msg)
	if err != nil {
		err_code = -1
		return
	}

	var result_data []byte
	result_data, err_code = rpc_proto.RpcCommonGet(hall_server.rpc_client, "G2G_CommonProc.Get", from_player_id, object_type, object_id, req_msg_id, req_data)
	if err_code < 0 {
		return
	}

	err = _unmarshal_msg(result_data, resp_msg)
	if err != nil {
		err_code = -1
		return
	}

	err_code = 1
	return
}

// 获取玩家信息
func remote_get_player_info(from_player_id, to_player_id int32) (resp *msg_rpc_message.G2GPlayerInfoResponse, err_code int32) {
	var req msg_rpc_message.G2GPlayerInfoRequest
	var response msg_rpc_message.G2GPlayerInfoResponse
	err_code = RemoteGetUsePB(from_player_id, rpc_proto.OBJECT_TYPE_PLAYER, to_player_id, int32(msg_rpc_message.MSGID_G2G_PLAYER_INFO_REQUEST), &req, &response)
	resp = &response
	return
}

// 获取玩家信息返回
func remote_get_player_info_response(to_player_id int32, req_data []byte) (resp_data []byte, err_code int32) {
	var req msg_rpc_message.G2GPlayerInfoRequest
	err := _unmarshal_msg(req_data, &req)
	if err != nil {
		err_code = -1
		return
	}

	player := player_mgr.GetPlayerById(to_player_id)
	if player == nil {
		log.Error("remote request get player info by id %v not found", to_player_id)
		err_code = int32(msg_client_message.E_ERR_PLAYER_NOT_EXIST)
		return
	}

	var response = msg_rpc_message.G2GPlayerInfoResponse{
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

// 获取多个玩家信息
func remote_get_multi_player_info(from_player_id int32, to_player_ids []int32) (resp *msg_rpc_message.G2GPlayerMultiInfoResponse, err_code int32) {
	var req msg_rpc_message.G2GPlayerMultiInfoRequest

	req_data, err := _marshal_msg(&req)
	if err != nil {
		err_code = -1
		return
	}

	datas := rpc_proto.RpcCommonMultiGet(hall_server.rpc_client, "G2G_CommonProc.MultiGet", from_player_id, rpc_proto.OBJECT_TYPE_PLAYER, to_player_ids, int32(msg_rpc_message.MSGID_G2G_PLAYER_MULTI_INFO_REQUEST), req_data)
	if datas == nil || len(datas) == 0 {
		log.Error("get multi players %v empty", to_player_ids)
		err_code = -1
		return
	}

	var response msg_rpc_message.G2GPlayerMultiInfoResponse
	for i := 0; i < len(datas); i++ {
		if datas[i].ErrorCode < 0 {
			err_code = -1
			log.Error("get multi players %v error %v with index %v", to_player_ids, datas[i].ErrorCode, i)
			return
		}
		err = _unmarshal_msg(datas[i].ResultData, &response)
		if err != nil {
			err_code = -1
			return
		}
		if resp == nil {
			resp = &msg_rpc_message.G2GPlayerMultiInfoResponse{}
		}
		resp.PlayerInfos = append(resp.PlayerInfos, response.PlayerInfos...)
	}

	err_code = 1
	return
}

// 获取多个玩家信息返回
func remote_get_multi_player_info_response(to_player_ids []int32, req_data []byte) (resp_data []byte, err_code int32) {
	var req msg_rpc_message.G2GPlayerMultiInfoRequest
	err := _unmarshal_msg(req_data, &req)
	if err != nil {
		err_code = -1
		return
	}

	var players_info []*msg_rpc_message.PlayerInfo
	for i := 0; i < len(to_player_ids); i++ {
		id := to_player_ids[i]
		player := player_mgr.GetPlayerById(id)
		if player == nil {
			log.Warn("remote request get player info by id %v from %v not found", id, to_player_ids)
			continue
		}

		players_info = append(players_info, &msg_rpc_message.PlayerInfo{
			PlayerId: id,
			UniqueId: player.db.GetUniqueId(),
			Account:  player.db.GetAccount(),
			Level:    player.db.GetLevel(),
			Head:     player.db.Info.GetHead(),
		})
	}

	if err_code < 0 {
		return
	}

	if err_code >= 0 {
		var response = msg_rpc_message.G2GPlayerMultiInfoResponse{
			PlayerInfos: players_info,
		}
		resp_data, err = _marshal_msg(&response)
		if err != nil {
			err_code = -1
			return
		}
	}

	err_code = 1
	return
}

type rpc_func func(int32, []byte) ([]byte, int32)
type rpc_mfunc func([]int32, []byte) ([]byte, int32)
type rpc_broadcast_func func([]byte)

var id2rpc_funcs = map[int32]rpc_func{
	int32(msg_rpc_message.MSGID_G2G_PLAYER_INFO_REQUEST): remote_get_player_info_response,
}

var id2rpc_mfuncs = map[int32]rpc_mfunc{
	int32(msg_rpc_message.MSGID_G2G_PLAYER_MULTI_INFO_REQUEST): remote_get_multi_player_info_response,
}

var id2rpc_broadcast_func = map[int32]rpc_broadcast_func{}
