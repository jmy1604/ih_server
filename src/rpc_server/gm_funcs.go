package main

import (
	"encoding/json"
	"ih_server/libs/log"
	"ih_server/proto/gen_go/client_message"
	"ih_server/src/rpc_common"
)

var gm_handles = map[int32]gm_handle{
	rpc_common.GM_CMD_TEST:              gm_test,
	rpc_common.GM_CMD_ANOUNCEMENT:       gm_anouncement,
	rpc_common.GM_CMD_SYS_MAIL:          gm_mail,
	rpc_common.GM_CMD_PLAYER_INFO:       gm_player_info,
	rpc_common.GM_CMD_ONLINE_PLAYER_NUM: gm_online_player_num,
}

func gm_test(id int32, data []byte) (int32, []byte) {
	if id != rpc_common.GM_CMD_TEST {
		log.Error("gm test cmd id %v not correct", id)
		return -1, nil
	}

	var err error
	var args rpc_common.GmTestCmd
	err = json.Unmarshal(data, &args)
	if err != nil {
		log.Error("gm cmd GmTestCmd unmarshal failed")
		return -1, nil
	}

	var result rpc_common.GmCommonResponse
	for _, r := range server.hall_rpc_clients {
		if r.rpc_client != nil {
			err = r.rpc_client.Call("G2H_Proc.Test", &args, &result)
			if err != nil {
				log.Error("gm rpc call G2H_Proc.Test err %v", err.Error())
				continue
			}
		}
	}

	data, err = json.Marshal(&result)
	if err != nil {
		log.Error("marshal gm cmd response GmTestResponse err %v", err.Error())
		return -1, nil
	}

	return result.Res, data
}

func gm_anouncement(id int32, data []byte) (int32, []byte) {
	if id != rpc_common.GM_CMD_ANOUNCEMENT {
		log.Error("gm anouncement cmd id %v not correct", id)
		return -1, nil
	}

	var err error
	var args rpc_common.GmAnouncementCmd
	err = json.Unmarshal(data, &args)
	if err != nil {
		log.Error("gm cmd GmAnouncementCmd unmarshal failed")
		return -1, nil
	}

	var result rpc_common.GmCommonResponse
	for _, r := range server.hall_rpc_clients {
		if r.rpc_client != nil {
			err = r.rpc_client.Call("G2H_Proc.Anouncement", &args, &result)
			if err != nil {
				log.Error("gm rpc call G2H_Proc.Anouncement err %v", err.Error())
				continue
			}
		}
	}

	data, err = json.Marshal(&result)
	if err != nil {
		log.Error("marshal gm cmd response GmAnouncementResponse err %v", err.Error())
		return -1, nil
	}

	return result.Res, data
}

func gm_mail(id int32, data []byte) (int32, []byte) {
	if id != rpc_common.GM_CMD_SYS_MAIL {
		log.Error("gm sys mail cmd id %v not correct", id)
		return -1, nil
	}

	var err error
	var args rpc_common.GmSendSysMailCmd
	err = json.Unmarshal(data, &args)
	if err != nil {
		log.Error("gm cmd GmSendSysMailCmd unmarshal failed")
		return -1, nil
	}

	player_id := args.PlayerId

	var result rpc_common.GmCommonResponse
	if player_id <= 0 {
		for _, r := range server.hall_rpc_clients {
			if r.rpc_client != nil {
				err = r.rpc_client.Call("G2H_Proc.SysMail", &args, &result)
				if err != nil {
					log.Error("gm rpc call G2H_Proc.SysMail err %v", err.Error())
					continue
				}
			}
		}
	} else {
		rpc_client := GetRpcClientByPlayerId(player_id)
		if rpc_client == nil {
			log.Error("gm get rpc client by player id %v failed", player_id)
			return int32(msg_client_message.E_ERR_PLAYER_NOT_EXIST), nil
		}
		err = rpc_client.Call("G2H_Proc.SysMail", &args, &result)
		if err != nil {
			log.Error("gm rpc call G2H_Proc.SysMail err %v", err.Error())
			return -1, nil
		}
	}

	data, err = json.Marshal(&result)
	if err != nil {
		log.Error("marshal gm cmd response GmSendSysMailResponse err %v", err.Error())
		return -1, nil
	}

	return result.Res, data
}

func gm_player_info(id int32, data []byte) (int32, []byte) {
	if id != rpc_common.GM_CMD_PLAYER_INFO {
		log.Error("gm player info cmd id %v not correct", id)
		return -1, nil
	}

	var err error
	var args rpc_common.GmPlayerInfoCmd
	err = json.Unmarshal(data, &args)
	if err != nil {
		log.Error("gm cmd GmPlayerInfoCmd unmarshal err %v", err.Error())
		return -1, nil
	}

	rpc_client := GetRpcClientByPlayerId(args.Id)
	if rpc_client == nil {
		log.Error("gm get rpc client by player id %v failed", args.Id)
		return int32(msg_client_message.E_ERR_PLAYER_NOT_EXIST), nil
	}

	var result rpc_common.GmPlayerInfoResponse
	err = rpc_client.Call("G2H_Proc.PlayerInfo", &args, &result)
	if err != nil {
		log.Error("gm rpc call G2H_Proc.PlayerInfo err %v", err.Error())
		return -1, nil
	}

	data, err = json.Marshal(&result)
	if err != nil {
		log.Error("marshal gm cmd response GmPlayerInfoResponse err %v", err.Error())
		return -1, nil
	}

	if result.Id < 0 {
		log.Error("gm get player %v info return %v", args.Id, result.Id)
		return result.Id, nil
	}

	return 1, data
}

func gm_online_player_num(id int32, data []byte) (int32, []byte) {
	if id != rpc_common.GM_CMD_ONLINE_PLAYER_NUM {
		log.Error("gm online player num cmd id %v not correct", id)
		return -1, nil
	}

	var err error
	var args rpc_common.GmOnlinePlayerNumCmd
	err = json.Unmarshal(data, &args)
	if err != nil {
		log.Error("gm cmd GmOnlinePlayerNumCmd unmarshal err %v", err.Error())
		return -1, nil
	}

	var server_ids []int32
	if args.ServerId > 0 {
		server_ids = []int32{args.ServerId}
	} else {
		ss := server_list.Servers
		for i := 0; i < len(ss); i++ {
			server_ids = append(server_ids, ss[i].Id)
		}
	}

	var player_num []int32
	var result rpc_common.GmOnlinePlayerNumResponse
	for i := 0; i < len(server_ids); i++ {
		sid := server_ids[i]
		player_num = append(player_num, sid)

		rpc_client := GetRpcClientByServerId(sid)
		if rpc_client == nil {
			player_num = append(player_num, -1)
			log.Error("gm get rpc client by server id %v failed", sid)
			continue
		}

		args.ServerId = sid
		err = rpc_client.Call("G2H_Proc.OnlinePlayerNum", &args, &result)
		if err != nil {
			player_num = append(player_num, -1)
			log.Error("gm rpc call G2H_Proc.OnlinePlayerNum err %v", err.Error())
			continue
		}

		player_num = append(player_num, result.PlayerNum[0])
	}

	result.PlayerNum = player_num
	data, err = json.Marshal(&result)
	if err != nil {
		log.Error("marshal gm cmd response GmOnlinePlayerNumResponse err %v", err.Error())
		return -1, nil
	}

	return 1, data
}
