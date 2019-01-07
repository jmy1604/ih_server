package main

import (
	"fmt"
	"ih_server/libs/log"
	"ih_server/libs/utils"
	"math/rand"
	"sync"
	"time"
)

type TopPowerRankItem struct {
	TopPower int32
}

func (this *TopPowerRankItem) Less(value interface{}) bool {
	item := value.(*TopPowerRankItem)
	if item == nil {
		return false
	}
	if this.TopPower < item.TopPower {
		return true
	}
	return false
}

func (this *TopPowerRankItem) Greater(value interface{}) bool {
	item := value.(*TopPowerRankItem)
	if item == nil {
		return false
	}
	if this.TopPower > item.TopPower {
		return true
	}
	return false
}

func (this *TopPowerRankItem) KeyEqual(value interface{}) bool {
	item := value.(*TopPowerRankItem)
	if this.TopPower == item.TopPower {
		return true
	}
	return false
}

func (this *TopPowerRankItem) GetKey() interface{} {
	return this.TopPower
}

func (this *TopPowerRankItem) GetValue() interface{} {
	return this.TopPower
}

func (this *TopPowerRankItem) SetValue(value interface{}) {
}

func (this *TopPowerRankItem) New() utils.SkiplistNode {
	return &TopPowerRankItem{}
}

func (this *TopPowerRankItem) Assign(node utils.SkiplistNode) {
	n := node.(*TopPowerRankItem)
	if n == nil {
		return
	}
	this.TopPower = n.TopPower
}

func (this *TopPowerRankItem) CopyDataTo(node interface{}) {
	n := node.(*TopPowerRankItem)
	if n == nil {
		return
	}
	n.TopPower = this.TopPower
}

type TopPowerPlayers struct {
	player2index map[int32]int32
	players      []int32
	locker       sync.RWMutex
}

func (this *TopPowerPlayers) Init() {
	this.player2index = make(map[int32]int32)
}

func (this *TopPowerPlayers) Insert(player_id int32) bool {
	this.locker.Lock()
	defer this.locker.Unlock()
	_, o := this.player2index[player_id]
	if o {
		return false
	}
	var idx int = len(this.player2index)
	if this.players != nil && len(this.players) > idx {
		this.players[idx] = player_id
	} else {
		this.players = append(this.players, player_id)
	}
	this.player2index[player_id] = int32(idx)
	return true
}

func (this *TopPowerPlayers) Delete(player_id int32) bool {
	this.locker.Lock()
	defer this.locker.Unlock()

	idx, o := this.player2index[player_id]
	if !o {
		return false
	}

	if int(idx) >= len(this.players) {
		return false
	}

	if this.players[idx] != player_id {
		log.Error("Idx player id is not %v, else is %v", player_id, this.players[idx])
		return false
	}

	this.players[idx] = player_id
	delete(this.player2index, player_id)

	return true
}

func (this *TopPowerPlayers) IsEmpty() bool {
	this.locker.RLock()
	defer this.locker.RUnlock()
	if len(this.player2index) > 0 {
		return false
	}
	return true
}

func (this *TopPowerPlayers) Random() int32 {
	this.locker.RLock()
	defer this.locker.RUnlock()

	l := len(this.player2index)
	if l == 0 {
		return -1
	}
	rand.Seed(time.Now().Unix())
	r := rand.Int31n(int32(l))
	return this.players[r]
}

type TopPowerRanklist struct {
	rank_powers   *utils.Skiplist            // 战力排序
	player2power  map[int32]int32            // 保存玩家当前战力
	power2players map[int32]*TopPowerPlayers // 战力到玩家的映射
	root_node     *TopPowerRankItem
	max_rank      int32
	items_pool    *sync.Pool
	locker        *sync.RWMutex
}

func NewTopPowerRanklist(root_node *TopPowerRankItem, max_rank int32) *TopPowerRanklist {
	ranking_list := &TopPowerRanklist{
		rank_powers:   utils.NewSkiplist(),
		player2power:  make(map[int32]int32),
		power2players: make(map[int32]*TopPowerPlayers),
		root_node:     root_node,
		max_rank:      max_rank,
		items_pool: &sync.Pool{
			New: func() interface{} {
				return root_node.New()
			},
		},
		locker: &sync.RWMutex{},
	}

	return ranking_list
}

func (this *TopPowerRanklist) Update(player_id, power int32) bool {
	if power <= 0 || player_id == 0 {
		return false
	}

	this.locker.Lock()
	defer this.locker.Unlock()

	old_power, o := this.player2power[player_id]
	if o {
		if power == old_power {
			return false
		}

		ps, oo := this.power2players[old_power]
		if !oo {
			log.Error("No old power %v players", old_power)
			return false
		}

		if !ps.Delete(player_id) {
			log.Error("Update old_power[%v] to power[%v] for player %v failed", old_power, power, player_id)
			return false
		}

		if ps.IsEmpty() {
			delete(this.power2players, old_power)
			var dt = TopPowerRankItem{
				TopPower: old_power,
			}
			if !this.rank_powers.Delete(&dt) {
				log.Warn("Delete empty power %v players to top power rank list failed", old_power)
			}
		}
	}

	var players *TopPowerPlayers
	players, o = this.power2players[power]
	if !o {
		players = &TopPowerPlayers{}
		players.Init()
		this.power2players[power] = players
	}
	players.Insert(player_id)

	this.player2power[player_id] = power

	t := this.items_pool.Get().(*TopPowerRankItem)
	t.TopPower = power
	if this.rank_powers.GetNode(t) == nil {
		this.rank_powers.Insert(t)
	}

	return true
}

func (this *TopPowerRanklist) GetNearestRandPlayer(power int32) int32 {
	this.locker.RLock()
	defer this.locker.RUnlock()

	l := this.rank_powers.GetLength()
	if l < 1 {
		return -1
	}

	players, o := this.power2players[power]
	if !o {
		left := int32(1)
		right := int32(l)
		var r, new_power int32
		new_power = power
		for {
			_r := (left + right) / 2
			if r == _r {
				return _r
			}
			r = _r

			item := this.rank_powers.GetByRank(r)
			it := item.(*TopPowerRankItem)
			if it != nil {
				new_power = it.TopPower
				if it.TopPower < power {
					right = r
				} else if it.TopPower > power {
					left = r
				} else {
					break
				}
			}
		}
		players = this.power2players[new_power]
		if players == nil {
			log.Error("@@@@ New power %v have no players", new_power)
		}
	}

	return players.Random()
}

func (this *TopPowerRanklist) OutputList() {
	l := this.rank_powers.GetLength()
	if l > 0 {
		var s string
		for r := int32(1); r < l; r++ {
			v := this.rank_powers.GetByRank(r)
			vv := v.(*TopPowerRankItem)
			if vv != nil {
				if r > 1 {
					s = fmt.Sprintf("%v,%v", s, vv.TopPower)
				} else {
					s = fmt.Sprintf("%v", vv.TopPower)
				}
			}
		}
		log.Trace("@@@@@ TopPowerRanklist %v", s)
	}
}