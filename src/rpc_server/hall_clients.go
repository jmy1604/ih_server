package main

import (
	"ih_server/libs/log"
	"ih_server/libs/rpc"
	"ih_server/src/share_data"
)

type HallRpcClient struct {
	server_idx int32
	server_id  int32
	server_ip  string
	rpc_client *rpc.Client
}

// 通过ServerId对应rpc客户端
func GetRpcClientByServerId(server_id int32) *rpc.Client {
	server_info := server_list.GetServerById(server_id)
	if server_info == nil {
		log.Error("get server info by server_id[%v] from failed", server_id)
		return nil
	}
	r := server.hall_rpc_clients[server_id]
	if r == nil {
		log.Error("通过ServerID[%v]获取rpc客户端失败", server_id)
		return nil
	}
	return r.rpc_client
}

// 通过玩家ID对应大厅的rpc客户端
func GetRpcClientByPlayerId(player_id int32) *rpc.Client {
	server_id := share_data.GetServerIdByPlayerId(player_id)
	return GetRpcClientByServerId(server_id)
}
