package utils

import (
	"ih_server/libs/log"
	"sync"
)

const (
	SHORT_RANK_ITEM_MAX_NUM = 500
)

type ShortRankItem interface {
	Less(item ShortRankItem) bool
	Greater(item ShortRankItem) bool
	GetKey() interface{}
	GetValue() interface{}
	Assign(item ShortRankItem)
	Add(item ShortRankItem)
	New() ShortRankItem
}

type ShortRankList struct {
	items    []ShortRankItem
	max_num  int32
	curr_num int32
	keys_map map[interface{}]int32
	locker   *sync.RWMutex
}

func (this *ShortRankList) Init(max_num int32) bool {
	if max_num <= 0 {
		return false
	}

	this.items = make([]ShortRankItem, max_num)
	this.max_num = max_num
	this.keys_map = make(map[interface{}]int32)
	this.locker = &sync.RWMutex{}
	return true
}

func (this *ShortRankList) GetLength() int32 {
	this.locker.RLock()
	defer this.locker.RUnlock()
	return this.curr_num
}

func (this *ShortRankList) Update(item ShortRankItem, add bool) bool {
	this.locker.Lock()
	defer this.locker.Unlock()

	idx, o := this.keys_map[item.GetKey()]
	if !o && this.curr_num >= this.max_num {
		log.Warn("Short Rank List length %v is max, cant insert new item", this.curr_num)
		return false
	}

	if !o {
		new_item := item.New()
		new_item.Assign(item)
		this.items[this.curr_num] = new_item
		this.keys_map[item.GetKey()] = this.curr_num

		i := this.curr_num - 1
		for ; i >= 0; i-- {
			if !item.Greater(this.items[i]) {
				break
			}
		}

		if i+1 != this.curr_num {
			for n := this.curr_num - 1; n >= i+1; n-- {
				this.items[n+1] = this.items[n]
				this.keys_map[this.items[n+1].GetKey()] = n + 1
			}
			this.items[i+1] = new_item
			this.keys_map[item.GetKey()] = i + 1
		}

		this.curr_num += 1
	} else {
		if this.items[idx] == nil {
			log.Error("!!!!!!!!!!!!! !!!!!!!!!!!!! !!!!!!!!!!!! Short Rank List idx[%v] item value is null", idx)
			return false
		}

		if add {
			item.Add(this.items[idx])
		}
		var i, b, e, pos int32
		if item.Greater(this.items[idx]) {
			i = idx - 1
			for ; i >= 0; i-- {
				if !item.Greater(this.items[i]) {
					break
				}
			}
			b = i + 1
			e = idx - 1
			pos = b
		} else if item.Less(this.items[idx]) {
			i = idx + 1
			for ; i < this.curr_num; i++ {
				if item.Greater(this.items[i]) {
					break
				}
			}
			b = idx + 1
			e = i - 1
			pos = e
		} else {
			return false
		}

		var the_item ShortRankItem
		if pos != idx {
			the_item = this.items[idx]
			if pos < idx {
				for i = e; i >= b; i-- {
					this.items[i+1] = this.items[i]
					this.keys_map[this.items[i+1].GetKey()] = i + 1
				}
			} else {
				for i = b; i <= e; i++ {
					this.items[i-1] = this.items[i]
					this.keys_map[this.items[i-1].GetKey()] = i - 1
				}
			}
		}
		if the_item != nil {
			this.items[pos] = the_item
		}
		this.items[pos].Assign(item)
		this.keys_map[this.items[pos].GetKey()] = pos
	}

	return true
}

func (this *ShortRankList) Delete(key interface{}) bool {
	this.locker.Lock()
	defer this.locker.Unlock()

	idx, o := this.keys_map[key]
	if !o {
		return false
	}

	for i := idx; i < this.curr_num-1; i++ {
		this.items[i] = this.items[i+1]
		this.keys_map[this.items[i+1].GetKey()] = i
	}
	this.items[this.curr_num-1] = nil

	delete(this.keys_map, key)

	this.curr_num -= 1

	return true
}

func (this *ShortRankList) Clear() {
	this.locker.Lock()
	defer this.locker.Unlock()

	for i := int32(0); i < this.max_num; i++ {
		this.items[i] = nil
	}

	this.curr_num = 0
	this.keys_map = make(map[interface{}]int32)
}

func (this *ShortRankList) GetRank(key interface{}) (rank int32) {
	this.locker.RLock()
	defer this.locker.RUnlock()

	var o bool
	rank, o = this.keys_map[key]
	if !o {
		return
	}
	rank += 1
	return
}

func (this *ShortRankList) GetByRank(rank int32) (key interface{}, value interface{}) {
	this.locker.RLock()
	defer this.locker.RUnlock()

	if this.curr_num < rank {
		return
	}
	item := this.items[rank-1]
	if item == nil {
		return
	}
	key = item.GetKey()
	value = item.GetValue()
	return
}

func (this *ShortRankList) GetIndex(rank int32) int32 {
	this.locker.RLock()
	defer this.locker.RUnlock()

	if this.curr_num < rank {
		return -1
	}
	item := this.items[rank-1]
	if item == nil {
		return -1
	}
	return this.keys_map[item.GetKey()]
}

type TestShortRankItem struct {
	Id    int32
	Value int32
}

func (this *TestShortRankItem) Less(item ShortRankItem) bool {
	it := item.(*TestShortRankItem)
	if it == nil {
		return false
	}
	if this.Value < it.Value {
		return true
	}
	return false
}

func (this *TestShortRankItem) Greater(item ShortRankItem) bool {
	it := item.(*TestShortRankItem)
	if it == nil {
		return false
	}
	if this.Value > it.Value {
		return true
	}
	return false
}

func (this *TestShortRankItem) GetKey() interface{} {
	return this.Id
}

func (this *TestShortRankItem) GetValue() interface{} {
	return this.Value
}

func (this *TestShortRankItem) Assign(item ShortRankItem) {
	it := item.(*TestShortRankItem)
	if it == nil {
		return
	}
	this.Id = it.Id
	this.Value = it.Value
}

func (this *TestShortRankItem) Add(item ShortRankItem) {
	it := item.(*TestShortRankItem)
	if it == nil {
		return
	}
	if this.Id == it.Id {
		this.Value += it.Value
	}
}

func (this *TestShortRankItem) New() ShortRankItem {
	return &TestShortRankItem{}
}
