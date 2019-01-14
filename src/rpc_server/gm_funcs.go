package main

import (
	//"encoding/base64"
	"encoding/json"
	"ih_server/libs/log"
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

	var result rpc_common.GmTestResponse
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

	return 1, data
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

	var result rpc_common.GmAnouncementResponse
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

	return 1, data
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
	//player_account := args.PlayerAccount

	var result rpc_common.GmSendSysMailResponse
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
		if rpc_client != nil {
			err = rpc_client.Call("G2H_Proc.SysMail", &args, &result)
			if err != nil {
				log.Error("gm rpc call G2H_Proc.SysMail err %v", err.Error())
				return -1, nil
			}
		}
	}

	data, err = json.Marshal(&result)
	if err != nil {
		log.Error("marshal gm cmd response GmSendSysMailResponse err %v", err.Error())
		return -1, nil
	}

	return 1, data
}
