package main

import (
	"errors"
	"fmt"
	"ih_server/libs/log"
	"ih_server/libs/rpc"
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

	var rpc_client *rpc.Client
	if arg.ObjectType == rpc_proto.OBJECT_TYPE_PLAYER {
		rpc_client = GetCrossRpcClientByPlayerId(arg.FromPlayerId, arg.ObjectId)
	} else if arg.ObjectType == rpc_proto.OBJECT_TYPE_GUILD {
		rpc_client = GetCrossRpcClientByGuildId(arg.FromPlayerId, arg.ObjectId)
	}
	if rpc_client == nil {
		return errors.New(fmt.Sprintf("!!!!!! Not found rpc client by object type %v object id %v", arg.ObjectType, arg.ObjectId))
	}

	err = rpc_client.Call("G2G_CommonProc.Get", arg, result)
	if err != nil {
		log.Error("RPC @@@ G2G_CommonProc.Get(%v,%v) error(%v)", arg, result, err.Error())
	} else {
		log.Trace("RPC @@@ G2G_CommonProc.Get(%v,%v)", arg, result)
	}

	return err
}

func split_object_ids_with_server(object_type int32, object_ids []int32) (serverid2objects map[int32][]int32) {
	if object_ids == nil || len(object_ids) == 0 {
		return
	}

	if object_type != rpc_proto.OBJECT_TYPE_GUILD && object_type != rpc_proto.OBJECT_TYPE_PLAYER {
		return
	}

	for i := 0; i < len(object_ids); i++ {
		id := object_ids[i]
		var server_id int32
		if object_type == rpc_proto.OBJECT_TYPE_PLAYER {
			server_id = share_data.GetServerIdByPlayerId(id)
		} else {
			server_id = share_data.GetServerIdByGuildId(id)
		}
		if !server_list.HasServerId(server_id) {
			continue
		}
		if serverid2objects == nil {
			serverid2objects = make(map[int32][]int32)
		}

		var objects []int32 = serverid2objects[server_id]
		if objects == nil {
			objects = []int32{id}
		} else {
			objects = append(objects, id)
		}
		serverid2objects[server_id] = objects
	}

	return
}

func (this *G2G_CommonProc) MultiGet(arg *rpc_proto.G2G_MultiGetRequest, result *rpc_proto.G2G_MultiGetResponse) (err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Stack(e)
		}
	}()

	var sid2objects map[int32][]int32 = split_object_ids_with_server(arg.ObjectType, arg.ObjectIds)
	if sid2objects == nil {
		return errors.New(fmt.Sprintf("!!!!!! split players result is empty from player id %v", arg.FromPlayerId))
	}

	var rpc_client *rpc.Client
	for sid, objects := range sid2objects {
		rpc_client = GetRpcClientByServerId(sid)
		if rpc_client == nil {
			return errors.New(fmt.Sprintf("!!!!!! Not found rpc client by server id %v", sid))
		}
		arg.ObjectIds = objects
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
