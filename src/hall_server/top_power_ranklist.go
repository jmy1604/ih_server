package main

import (
	"ih_server/libs/log"
	"ih_server/libs/utils"
	"sync"
)

type TopPowerRankItem struct {
	TopPower     int32
	Player2Index map[int32]int32
	Players      []int32
}

func (this *TopPowerRankItem) Init() {
	this.Player2Index = make(map[int32]int32)
	this.Players = make([]int32, 0)
}

func (this *TopPowerRankItem) Insert(player_id int32) bool {
	idx, o := this.Player2Index[player_id]
	if o {
		return false
	}
	this.Players = append(this.Players, player_id)
	idx = int32(len(this.Players)) - 1
	this.Player2Index[player_id] = idx
	return true
}

func (this *TopPowerRankItem) Delete(player_id int32) bool {
	idx, o := this.Player2Index[player_id]
	if !o {
		return false
	}
	l := len(this.Player2Index) - 1
	this.Players[idx] = this.Players[l]
	this.Players = this.Players[:l]
	delete(this.Player2Index, player_id)
	return true
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
	return this.Players
}

func (this *TopPowerRankItem) SetValue(value interface{}) {
}

func (this *TopPowerRankItem) New() utils.SkiplistNode {
	item := &TopPowerRankItem{}
	item.Init()
	return item
}

func (this *TopPowerRankItem) Assign(node utils.SkiplistNode) {
	n := node.(*TopPowerRankItem)
	if n == nil {
		return
	}
	this.TopPower = n.TopPower
	this.Player2Index = n.Player2Index
	this.Players = n.Players
}

func (this *TopPowerRankItem) CopyDataTo(node interface{}) {
	n := node.(*TopPowerRankItem)
	if n == nil {
		return
	}
	n.TopPower = this.TopPower
	n.Player2Index = this.Player2Index
	n.Players = this.Players
}

type TopPowerRanklist struct {
	rank_powers   *utils.Skiplist
	power2players map[int32]*TopPowerRankItem
	root_node     *TopPowerRankItem
	max_rank      int32
	items_pool    *sync.Pool
	locker        *sync.RWMutex
}

func NewTopPowerRanklist(root_node *TopPowerRankItem, max_rank int32) *TopPowerRanklist {
	ranking_list := &TopPowerRanklist{
		rank_powers:   utils.NewSkiplist(),
		power2players: make(map[int32]*TopPowerRankItem),
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

func (this *TopPowerRanklist) GetByRank(rank int32) *TopPowerRankItem {
	this.locker.RLock()
	defer this.locker.RUnlock()

	item := this.rank_powers.GetByRank(rank)
	if item == nil {
		return nil
	}
	new_item := this.items_pool.Get().(*TopPowerRankItem)
	new_item.Assign(item)
	return new_item
}

func (this *TopPowerRanklist) GetByKey(power int32) *TopPowerRankItem {
	this.locker.RLock()
	defer this.locker.RUnlock()

	item, o := this.power2players[power]
	if !o || item == nil {
		return nil
	}
	new_item := this.items_pool.Get().(*TopPowerRankItem)
	new_item.Assign(item)
	return new_item
}

func (this *TopPowerRanklist) Update(player_id, old_power, power int32) bool {
	if old_power == power || player_id == 0 {
		return false
	}

	this.locker.Lock()
	defer this.locker.Unlock()

	old_item, o := this.power2players[old_power]
	if o {
		if !old_item.Delete(player_id) {
			log.Error("Update old_power[%v] to power[%v] for player %v failed", old_power, power, player_id)
			return false
		}
	}
	var item *TopPowerRankItem
	item, o = this.power2players[power]
	if !o {
		item = this.items_pool.Get().(*TopPowerRankItem)
	}
	item.Insert(player_id)
	return true
}

func (this *TopPowerRanklist) GetRank(power int32) int32 {
	this.locker.RLock()
	defer this.locker.RUnlock()

	item, o := this.power2players[power]
	if !o {
		return 0
	}
	return this.rank_powers.GetRank(item)
}

func (this *TopPowerRanklist) GetLength() int32 {
	this.locker.RLock()
	defer this.locker.RUnlock()
	return this.rank_powers.GetLength()
}

func (this *TopPowerRanklist) GetNearestRank(power int32) int32 {
	this.locker.RLock()
	defer this.locker.RUnlock()

	l := this.rank_powers.GetLength()
	if l < 2 {
		return -1
	}

	left := int32(1)
	right := int32(l)
	var r int32
	for {
		_r := (left + right) / 2
		if r == _r {
			return _r
		}
		r = _r

		item := this.rank_powers.GetByRank(r)
		it := item.(*TopPowerRankItem)
		if it != nil {
			if it.TopPower < power {
				left = r
			} else if it.TopPower > power {
				right = r
			} else {
				break
			}
		}
	}
	return r
}
