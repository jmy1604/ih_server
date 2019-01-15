package main

import (
	"encoding/json"
	"ih_server/libs/log"
	"ih_server/proto/gen_go/client_message"
	"ih_server/src/rpc_common"
)

var gm_handles = map[int32]gm_handle{
	rpc_common.GM_CMD_TEST:        gm_test,
	rpc_common.GM_CMD_ANOUNCEMENT: gm_anouncement,
	rpc_common.GM_CMD_SYS_MAIL:    gm_mail,
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
