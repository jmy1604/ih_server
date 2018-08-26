package main

import (
	"ih_server/libs/log"
	"ih_server/libs/rpc"
)

type HallRpcClient struct {
	server_idx int32
	server_id  int32
	server_ip  string
	rpc_client *rpc.Client
}

// 通过玩家ID对应大厅的rpc客户端
func get_hall_rpc_client_by_player_id(player_id int32) *rpc.Client {
	hc := hall_group_mgr.GetHallCfgByPlayerId(player_id)
	if hc == nil {
		log.Error("通过玩家ID[%v]获取大厅配置失败", player_id)
		return nil
	}
	r := server.hall_rpc_clients[hc.ServerId]
	if r == nil {
		log.Error("通过ServerID[%v]获取rpc客户端失败", hc.ServerId)
		return nil
	}
	return r.rpc_client
}
