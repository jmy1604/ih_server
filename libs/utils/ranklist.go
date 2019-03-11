package utils

import (
	"ih_server/libs/log"
	"sync"
)

// -----------------------------------------------------------------------------
/* 通用排行榜 */
// -----------------------------------------------------------------------------
type CommonRankingList struct {
	ranking_items *Skiplist
	key2item      map[interface{}]SkiplistNode
	root_node     SkiplistNode
	max_rank      int32
	items_pool    *sync.Pool
	locker        *sync.RWMutex
}

func NewCommonRankingList(root_node SkiplistNode, max_rank int32) *CommonRankingList {
	ranking_list := &CommonRankingList{
		ranking_items: NewSkiplist(),
		key2item:      make(map[interface{}]SkiplistNode),
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

func (this *CommonRankingList) GetByRank(rank int32) SkiplistNode {
	this.locker.RLock()
	defer this.locker.RUnlock()

	item := this.ranking_items.GetByRank(rank)
	if item == nil {
		return nil
	}
	new_item := this.items_pool.Get().(SkiplistNode)
	new_item.Assign(item)
	return new_item
}

func (this *CommonRankingList) GetByKey(key interface{}) SkiplistNode {
	this.locker.RLock()
	defer this.locker.RUnlock()

	item, o := this.key2item[key]
	if !o || item == nil {
		return nil
	}
	new_item := this.items_pool.Get().(SkiplistNode)
	new_item.Assign(item)
	return new_item
}

func (this *CommonRankingList) SetValueByKey(key interface{}, value interface{}) {
	this.locker.RLock()
	defer this.locker.RUnlock()

	item, o := this.key2item[key]
	if !o || item == nil {
		return
	}
	item.SetValue(value)
}

func (this *CommonRankingList) insert(key interface{}, item SkiplistNode, is_lock bool) bool {
	if is_lock {
		this.locker.Lock()
		defer this.locker.Unlock()
	}
	this.ranking_items.Insert(item)
	this.key2item[key] = item
	return true
}

func (this *CommonRankingList) Insert(key interface{}, item SkiplistNode) bool {
	return this.insert(key, item, true)
}

func (this *CommonRankingList) delete(key interface{}, is_lock bool) bool {
	if is_lock {
		this.locker.Lock()
		defer this.locker.Unlock()
	}

	item, o := this.key2item[key]
	if !o {
		log.Error("CommonRankingList key[%v] not found", key)
		return false
	}
	if !this.ranking_items.Delete(item) {
		log.Error("CommonRankingList delete key[%v] value[%v] in ranking list failed", key, item.GetValue())
		return false
	}
	if is_lock {
		this.items_pool.Put(item)
	}
	//delete(this.key2item, key)
	return true
}

func (this *CommonRankingList) Delete(key interface{}) bool {
	return this.delete(key, true)
}

func (this *CommonRankingList) Update(item SkiplistNode) bool {
	this.locker.Lock()
	defer this.locker.Unlock()

	old_item, o := this.key2item[item.GetKey()]
	if o {
		if !this.delete(item.GetKey(), false) {
			log.Error("Update key[%v] for Ranking List failed", item)
			return false
		}
		old_item.Assign(item)
		return this.insert(item.GetKey(), old_item, false)
	} else {
		new_item := this.items_pool.Get().(SkiplistNode)
		new_item.Assign(item)
		return this.insert(item.GetKey(), new_item, false)
	}
}

func (this *CommonRankingList) GetRangeNodes(rank_start, rank_num int32, nodes []interface{}) (num int32) {
	if rank_start <= int32(0) || rank_start > this.max_rank {
		log.Warn("Ranking list rank_start[%v] invalid", rank_start)
		return
	}

	this.locker.RLock()
	defer this.locker.RUnlock()

	if int(rank_start) > len(this.key2item) {
		log.Warn("Ranking List rank range[1,%v], rank_start[%v] over rank list", len(this.key2item), rank_start)
		return
	}

	real_num := int32(len(this.key2item)) - rank_start + 1
	if real_num < rank_num {
		rank_num = real_num
	}

	items := make([]SkiplistNode, rank_num)
	b := this.ranking_items.GetByRankRange(rank_start, rank_num, items)
	if !b {
		log.Warn("Ranking List rank range[%v,%v] is empty", rank_start, rank_num)
		return
	}

	for i := int32(0); i < rank_num; i++ {
		item := items[i]
		if item == nil {
			log.Error("Get Rank[%v] for Ranking List failed")
			continue
		}
		node := nodes[i]
		item.CopyDataTo(node)
		num += 1
	}
	return
}

func (this *CommonRankingList) GetRank(key interface{}) int32 {
	this.locker.RLock()
	defer this.locker.RUnlock()

	item, o := this.key2item[key]
	if !o {
		return 0
	}
	return this.ranking_items.GetRank(item)
}

func (this *CommonRankingList) GetRankAndValue(key interface{}) (rank int32, value interface{}) {
	this.locker.RLock()
	defer this.locker.RUnlock()

	item, o := this.key2item[key]
	if !o {
		return 0, nil
	}

	return this.ranking_items.GetRank(item), item.GetValue()
}

func (this *CommonRankingList) GetRankRange(start, num int32) (int32, int32) {
	this.locker.RLock()
	defer this.locker.RUnlock()

	l := int32(len(this.key2item))
	if this.key2item == nil || l == 0 {
		return 0, 0
	}

	if start > l {
		return 0, 0
	}

	if l-start+1 < num {
		num = l - start + 1
	}
	return start, num
}

func (this *CommonRankingList) GetLastRankRange(num int32) (int32, int32) {
	this.locker.RLock()
	defer this.locker.RUnlock()

	l := int32(len(this.key2item))
	if this.key2item == nil || l == 0 {
		return 0, 0
	}

	if num > l {
		num = l
	}

	return l - num + 1, num
}

func (this *CommonRankingList) GetLastRank() int32 {
	this.locker.RLock()
	defer this.locker.RUnlock()

	return int32(len(this.key2item))
}

func (this *CommonRankingList) GetLength() int32 {
	this.locker.RLock()
	defer this.locker.RUnlock()
	return this.ranking_items.GetLength()
}
