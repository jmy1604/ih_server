package main

import (
	_ "ih_server/libs/log"
	_ "ih_server/libs/utils"
	_ "ih_server/proto/gen_go/client_message"
	_ "ih_server/proto/gen_go/client_message_id"
	_ "ih_server/src/table_config"
	"sync"
	_ "sync/atomic"
	_ "time"

	_ "github.com/golang/protobuf/proto"
)

// 最高战力区间
type TopPowerPlayers struct {
	player2index map[int32]int32
	players      []int32
	locker       sync.RWMutex
}

func (this *TopPowerPlayers) Init() {
	this.player2index = make(map[int32]int32)
}

func (this *TopPowerPlayers) Insert(player_id int32) bool {
	this.locker.RLock()
	_, o := this.player2index[player_id]
	if o {
		this.locker.RUnlock()
		return false
	}
	this.locker.RUnlock()

	this.locker.Lock()

	_, o = this.player2index[player_id]
	if o {
		this.locker.Unlock()
		return false
	}
	var idx int
	if len(this.players) > len(this.player2index) {
		idx = len(this.player2index)
		this.players[idx] = player_id
	} else {
		this.players = append(this.players, player_id)
		idx = len(this.players)
	}
	this.player2index[player_id] = int32(idx)

	this.locker.Unlock()

	return true
}

func (this *TopPowerPlayers) Delete(player_id int32) bool {
	this.locker.RLock()
	idx, o := this.player2index[player_id]
	if !o {
		this.locker.RUnlock()
		return false
	}

	this.locker.Lock()

	idx, o = this.player2index[player_id]
	if !o {
		this.locker.Unlock()
		return false
	}
	if int(idx) >= len(this.player2index) {
		this.locker.Unlock()
		return false
	}
	this.players[idx] = this.players[len(this.player2index)-1]
	this.players[len(this.player2index)-1] = 0
	delete(this.player2index, player_id)

	this.locker.Unlock()

	return true
}

func (this *TopPowerPlayers) IsEmpty() bool {
	var is_empty bool
	this.locker.RLock()
	if this.player2index == nil || len(this.player2index) == 0 {
		is_empty = true
	}
	this.locker.RUnlock()
	return is_empty
}

// 最高战力区间管理
type TopPowerAreaManager struct {
	player2power map[int32]int32
	players_list map[int32]*TopPowerPlayers

	locker sync.RWMutex
}

var top_power_area_mgr TopPowerAreaManager

func (this *TopPowerAreaManager) Init() {
	this.player2power = make(map[int32]int32)
	this.players_list = make(map[int32]*TopPowerPlayers)
}

func (this *TopPowerAreaManager) Update(p *Player, power int32) {
	if p == nil || power <= 0 {
		return
	}
	this.locker.RLock()
	var old_pl *TopPowerPlayers
	if old_power, o := this.player2power[p.Id]; o {
		if old_power == power {
			this.locker.RUnlock()
			return
		}
		old_pl := this.players_list[old_power]
		if old_pl == nil {
			this.locker.RUnlock()
			return
		}
	}
	pl := this.players_list[power]
	if pl == nil {
		this.locker.RUnlock()

		this.locker.Lock()
		pl = this.players_list[power]
		if pl == nil {
			pl = &TopPowerPlayers{}
			pl.Init()
			this.players_list[power] = pl
		}
		this.locker.Unlock()
	} else {
		this.locker.RUnlock()
	}

	old_pl.Delete(p.Id)
	pl.Insert(p.Id)

	// 更新排行榜
	/*sid := atomic.AddInt64(&top_power_serial_id, 1)
	this.db.ExpeditionPlayers.SetSerialId(sid)
	var item = TopPowerRankItem{
		SerialId: sid,
		TopPower: this.get_defense_team_power(),
		PlayerId: p.Id,
	}
	rank_list_mgr.UpdateItem(RANK_LIST_TYPE_TOP_DEFENSE_TOWER, &item)*/
}

func (this *TopPowerAreaManager) Match(power int32) (player_id int32) {
	this.locker.RLock()
	this.locker.RUnlock()
	return
}

func (this *Player) ChangeTopDefensePower() {
	dt := this.db.BattleTeam.GetDefenseMembers()
	if dt == nil || len(dt) == 0 {
		return
	}
}
