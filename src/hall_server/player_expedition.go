package main

import (
	_ "ih_server/libs/log"
	_ "ih_server/libs/utils"
	_ "ih_server/proto/gen_go/client_message"
	_ "ih_server/proto/gen_go/client_message_id"
	_ "ih_server/src/table_config"
	_ "sync"
	_ "sync/atomic"
	_ "time"

	_ "github.com/golang/protobuf/proto"
)

func (this *Player) ChangeTopDefensePower() {
	dt := this.db.BattleTeam.GetDefenseMembers()
	if dt == nil || len(dt) == 0 {
		return
	}
}
