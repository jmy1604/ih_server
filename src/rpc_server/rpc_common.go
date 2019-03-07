package main

import (
	"errors"
	"fmt"
	"ih_server/libs/log"
	"ih_server/src/rpc_proto"
	"ih_server/src/share_data"
)

// 通用调用过程
type G2G_CommonProc struct {
}

func (this *G2G_CommonProc) Get(arg *rpc_proto.G2G_GetRequest, result *rpc_proto.G2G_GetResponse) (err error) {
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

func (this *G2G_CommonProc) MultiGet(arg *rpc_proto.G2G_MultiGetRequest, result *rpc_proto.G2G_MultiGetResponse) (err error) {
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
		var tmp_result rpc_proto.G2G_GetResponse
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
