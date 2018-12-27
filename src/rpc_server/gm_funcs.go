package main

import (
	//"encoding/base64"
	"encoding/json"
	"ih_server/libs/log"
	"ih_server/src/rpc_common"
)

var gm_handles = map[int32]gm_handle{
	rpc_common.GM_CMD_ANOUNCEMENT: gm_anouncement,
}

func gm_anouncement(id int32, data []byte) (int32, []byte) {
	if id != rpc_common.GM_CMD_ANOUNCEMENT {
		log.Error("gm anouncement cmd id %v not correct", id)
		return -1, nil
	}

	var err error
	/*data, err = base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		log.Error("gm anouncement base64 decode err %v", err.Error())
		return -1, nil
	}*/

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
