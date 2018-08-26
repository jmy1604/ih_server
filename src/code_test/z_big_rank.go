package main

import (
	"ih_server/libs/log"
	"sync"
)

const (
	BIG_RANK_SORT_TYPE_B   = 0 // 大于规则
	BIG_RANK_SORT_TYPE_B_E = 1 // 大于等于规则
)

type BigRankRecordItem struct {
	Id   int32
	Rank int32
	Val  int32

	_arr *BigRankArrayItem
}

type BigRankArrayItem struct {
	CurCount  int32
	MaxCount  int32
	sort_type int32
	Array     []*BigRankRecordItem
	Id2Item   map[int32]*BigRankRecordItem

	_rank_base int32
	_rank      *BigRankService
	_arr_idx   int32
}

//-----------------------------------------------------------------------------

func NewBigRankArrayItem(rank_len, sort_type int32, rank *BigRankService) *BigRankArrayItem {
	ret_arr := &BigRankArrayItem{}
	ret_arr.MaxCount = rank_len
	ret_arr.Array = make([]*BigRankRecordItem, rank_len)
	for idx := int32(0); idx < ret_arr.MaxCount; idx++ {
		ret_arr.Array[idx] = &BigRankRecordItem{_arr: ret_arr}
	}
	ret_arr.Id2Item = make(map[int32]*BigRankRecordItem)
	ret_arr._rank = rank
	return ret_arr
}

func (this *BigRankArrayItem) SetVal(id, val int32) (*BigRankRecordItem, bool) {
	badd := false
	if this.CurCount == this.MaxCount && val < this.Array[this.MaxCount-1].Val {
		//fmt.Println("BigRankArrayItem SetVal %d %d  %dreturn !", id, val, this.Array[this.CurCount-1].Val)
		return nil, badd
	}

	var new_rank int32
	cur_rd := this.Id2Item[id]
	if nil == cur_rd {
		var end_rank int32
		for new_rank = 0; new_rank < this.CurCount; new_rank++ {
			if BIG_RANK_SORT_TYPE_B_E == this.sort_type {
				if val >= this.Array[new_rank].Val {
					break
				}
			} else {
				if val > this.Array[new_rank].Val {
					break
				}
			}
		}

		if new_rank >= this.MaxCount {
			return nil, badd
		}

		end_rank = this.CurCount
		if end_rank > this.MaxCount-1 {
			end_rank = this.MaxCount - 1
		}
		if this.CurCount == this.MaxCount && end_rank == this.MaxCount-1 {
			delete(this.Id2Item, this.Array[this.MaxCount-1].Id)
			delete(this._rank.id2item, this.Array[this.MaxCount-1].Id)
		}

		if g_log_out {
			log.Info("end_rank %d , new_rank %d", end_rank, new_rank)
		}

		for ; end_rank > new_rank; end_rank-- {
			*this.Array[end_rank] = *this.Array[end_rank-1]
			this.Array[end_rank].Rank = end_rank
			this.Id2Item[this.Array[end_rank].Id] = this.Array[end_rank]
			this._rank.id2item[this.Array[end_rank].Id] = this.Array[end_rank]
		}

		this.Array[new_rank].Rank = new_rank
		this.Array[new_rank].Id = id
		this.Array[new_rank].Val = val
		this.Id2Item[this.Array[new_rank].Id] = this.Array[new_rank]
		this._rank.id2item[id] = this.Array[new_rank]

		if this.CurCount < this.MaxCount {
			this.CurCount++
		}

		badd = true
	} else {
		old_rank := cur_rd.Rank
		for new_rank = 0; new_rank < this.CurCount; new_rank++ {
			if BIG_RANK_SORT_TYPE_B_E == this.sort_type {
				if val >= this.Array[new_rank].Val {
					break
				}
			} else {
				if val > this.Array[new_rank].Val {
					break
				}
			}
		}

		if old_rank == new_rank {
			cur_rd.Val = val
			return nil, badd
		} else if new_rank > old_rank {
			for idx := old_rank; idx+1 < new_rank; idx++ {
				*this.Array[idx] = *this.Array[idx+1]
				this.Array[idx].Rank = idx
				this.Id2Item[this.Array[idx].Id] = this.Array[idx]
				this._rank.id2item[this.Array[idx].Id] = this.Array[idx]
			}

			new_rank--
			this.Array[new_rank].Rank = new_rank
			this.Array[new_rank].Id = id
			this.Array[new_rank].Val = val
			this.Id2Item[id] = this.Array[new_rank]
			this._rank.id2item[id] = this.Array[new_rank]
		} else {
			//log.Info("old_rank %d new_rank %d", old_rank, new_rank)
			for idx := old_rank; idx > new_rank; idx-- {
				*this.Array[idx] = *this.Array[idx-1]
				this.Array[idx].Rank = idx
				this.Id2Item[this.Array[idx].Id] = this.Array[idx]
				this._rank.id2item[this.Array[idx].Id] = this.Array[idx]
			}

			this.Array[new_rank].Rank = new_rank
			this.Array[new_rank].Id = id
			this.Array[new_rank].Val = val
			this.Id2Item[id] = this.Array[new_rank]
			this._rank.id2item[id] = this.Array[new_rank]
		}
	}

	return this.Array[new_rank], badd
}

func (this *BigRankArrayItem) Remove(idx int32) bool {
	if idx < 0 {
		log.Error("BigRankArrayItem idx(%d) error !", idx)
		return false
	}

	cur_rd := this.Id2Item[idx]
	if nil == cur_rd {
		log.Error("BigRankArrayItem idx(%d) not exist !", idx)
		return false
	}

	delete(this.Id2Item, idx)
	delete(this._rank.id2item, idx)

	for tmp_idx := cur_rd.Rank; tmp_idx < this.CurCount-1; tmp_idx++ {
		*this.Array[tmp_idx] = *this.Array[tmp_idx+1]
		this.Array[tmp_idx].Rank = tmp_idx
		this.Id2Item[this.Array[tmp_idx].Id] = this.Array[tmp_idx]
		this._rank.id2item[this.Array[tmp_idx].Id] = this.Array[tmp_idx]
	}

	this.CurCount--
	if this.CurCount < 0 {
		this.CurCount = 0
		log.Error("BigRankArrayItem Remove[%d] CurCount[%d] error !", idx, this.CurCount)
		return false
	}

	return true
}

func (this *BigRankArrayItem) HalfBreak(array_item *BigRankArrayItem) {
	if nil == array_item {

		return
	}

	array_item.CurCount = 0
	half_count := this.CurCount / 2
	array_item.Id2Item = make(map[int32]*BigRankRecordItem)
	for idx := half_count; idx < this.CurCount; idx++ {
		array_item.Array[idx-half_count].Id = this.Array[idx].Id
		array_item.Array[idx-half_count].Val = this.Array[idx].Val
		array_item.Array[idx-half_count].Rank = array_item.CurCount
		array_item.Id2Item[this.Array[idx].Id] = array_item.Array[idx-half_count]
		this._rank.id2item[this.Array[idx].Id] = array_item.Array[idx-half_count]
		array_item.CurCount++

		delete(this.Id2Item, this.Array[idx].Id)
	}

	this.CurCount = half_count
	return
}

func (this *BigRankArrayItem) PrintInfo() {
	log.Info("	数组基础信息：CurCount:%d, _rank_base:%d", this.CurCount, this._rank_base)

	for idx := int32(0); idx < this.CurCount; idx++ {
		log.Info("	id:%d, rank:%d, val:%d", this.Array[idx].Id, this.Array[idx].Rank, this.Array[idx].Val)
	}
	return
}

//-----------------------------------------------------------------------------

type BigRankService struct {
	_op_lock       *sync.RWMutex
	Array          []*BigRankArrayItem
	id2item        map[int32]*BigRankRecordItem
	_array_len     int32
	_sub_array_len int32
	_sort_typ      int32
}

func NewBigRankService(array_len, sub_array_len, sort_typ int32) *BigRankService {
	ret_rank := &BigRankService{}
	ret_rank._array_len = array_len
	ret_rank._sub_array_len = sub_array_len
	ret_rank.Array = make([]*BigRankArrayItem, array_len)
	ret_rank.id2item = make(map[int32]*BigRankRecordItem)
	for idx := int32(0); idx < array_len; idx++ {
		ret_rank.Array[idx] = NewBigRankArrayItem(sub_array_len, sort_typ, ret_rank)
		ret_rank.Array[idx]._arr_idx = idx
		//ret_rank.Array[idx]._rank_base = sub_array_len * idx
	}

	return ret_rank
}

func (this *BigRankService) AddRankArrayItem(idx int32, array *BigRankArrayItem) {
	if idx < 0 || idx >= this._array_len {
		log.Error("BigRankService AddRankArrayItem idx[%d] error array_len[%d] !", idx, this._array_len)
		return
	}

	var rank_base = int32(0)
	for arr_idx := int32(0); arr_idx < idx; arr_idx++ {
		rank_base += this.Array[arr_idx].CurCount
	}

	for tmp_idx := this._array_len - 1; tmp_idx > idx; tmp_idx-- {
		this.Array[tmp_idx] = this.Array[tmp_idx-1]
		this.Array[tmp_idx]._arr_idx = tmp_idx
	}

	this.Array[idx] = array
	this.Array[idx]._rank = this
	this.Array[idx]._rank_base = rank_base
	this.Array[idx]._arr_idx = idx

	for arr_idx := idx + 1; arr_idx < this._array_len; arr_idx++ {
		rank_base += this.Array[arr_idx-1].CurCount
		this.Array[arr_idx]._rank_base = rank_base
	}
}

func (this *BigRankService) AddVal(id, val int32) {
	cur_rd := this.id2item[id]
	if nil == cur_rd {
		//log.Info("BigRankService AddVal cur_rd nil [%d, %d] !", id, val)
		var arr_idx, tmp_count int32
		var arr_item *BigRankArrayItem
		for arr_idx = 0; arr_idx < this._array_len; arr_idx++ {
			tmp_count = this.Array[arr_idx].CurCount
			if tmp_count <= 0 {
				break
			}

			if tmp_count < this._sub_array_len && arr_idx < this._array_len-1 {
				if BIG_RANK_SORT_TYPE_B == this._sort_typ {
					if val > this.Array[arr_idx+1].Array[0].Val {
						break
					}
				} else {
					if val >= this.Array[arr_idx].Array[0].Val {
						break
					}
				}

			}

			if BIG_RANK_SORT_TYPE_B == this._sort_typ {
				if val > this.Array[arr_idx].Array[tmp_count-1].Val {
					break
				}
			} else {
				if val >= this.Array[arr_idx].Array[tmp_count-1].Val {
					break
				}
			}
		}

		if arr_idx == this._array_len { // 如果超过最后一个
			arr_idx--
		}

		//log.Info("BigRankService AddVal arr_idx [%d] id[%d] val[%d] ", arr_idx, id, val)

		arr_item = this.Array[arr_idx]

		old_end_id := int32(-1)
		if arr_item.CurCount >= arr_item.MaxCount && nil != arr_item.Array[arr_item.MaxCount-1] {
			old_end_id = arr_item.Array[arr_item.MaxCount-1].Id
		}

		rd_item, badd := arr_item.SetVal(id, val)
		if nil != rd_item {
			//this.id2item[rd_item.Id] = rd_item
			if old_end_id > 0 && old_end_id != id {
				delete(this.id2item, old_end_id)
			}

			if badd {
				if arr_item.CurCount == 1 {
					tmp_base := int32(0)
					for tmp_idx := int32(0); tmp_idx < arr_idx; tmp_idx++ {
						tmp_base += this.Array[tmp_idx].CurCount
					}
					arr_item._rank_base = tmp_base
				}
				for tmp_idx := arr_idx + 1; tmp_idx < this._array_len; tmp_idx++ {
					if this.Array[arr_idx].CurCount > 0 {
						this.Array[tmp_idx]._rank_base++
					}
				}
			}

			if arr_item.CurCount >= arr_item.MaxCount && arr_idx != this._array_len-1 {
				end_arr_item := this.Array[this._array_len-1]
				for tmp_idx := int32(0); tmp_idx < end_arr_item.CurCount; tmp_idx++ {
					delete(this.id2item, end_arr_item.Array[tmp_idx].Id)
				}
				//log.Info("拆分前：")
				//log.Info("=====================整个数组=========================")
				//this.PrintAllRecords()
				//log.Info("=======================end===========================")
				//arr_item.PrintInfo()
				arr_item.HalfBreak(end_arr_item)
				//log.Info("拆分单数组结果：")
				//arr_item.PrintInfo()
				this.AddRankArrayItem(arr_idx+1, end_arr_item)
				//end_arr_item.PrintInfo()
				//log.Info("=====================最终结果=========================")
				//this.PrintAllRecords()
				//log.Info("=======================end===========================")
			}
		}

	} else {

		cur_arr := cur_rd._arr
		if g_log_out {
			log.Info("cur_map_val [id:%d, val:%d] ", cur_rd.Id, cur_rd.Val)
			if nil == cur_arr {
				log.Error("BigRankService AddVal cur_arr[id:%d, val:%d] nil", id, cur_rd.Val)
			} else {
				log.Info("cur_arr [id:%d, val:%d] ", cur_arr._arr_idx)
			}
		}

		var arr_idx, tmp_count int32
		var arr_item *BigRankArrayItem
		for arr_idx = 0; arr_idx < this._array_len; arr_idx++ {
			tmp_count = this.Array[arr_idx].CurCount
			if tmp_count <= 0 {
				break
			}

			if tmp_count < this._sub_array_len && arr_idx < this._array_len-1 {
				if BIG_RANK_SORT_TYPE_B == this._sort_typ {
					if val > this.Array[arr_idx+1].Array[0].Val {
						break
					}
				} else {
					if val >= this.Array[arr_idx].Array[0].Val {
						break
					}
				}

			}

			if BIG_RANK_SORT_TYPE_B == this._sort_typ {
				if val > this.Array[arr_idx].Array[tmp_count-1].Val {
					break
				}
			} else {
				if val >= this.Array[arr_idx].Array[tmp_count-1].Val {
					break
				}
			}
		}

		if arr_idx == this._array_len { // 如果超过最后一个
			arr_idx--
		}

		var tmp_idx, cur_arr_idx int32
		arr_item = this.Array[arr_idx]
		bdel := false
		if nil != cur_arr && arr_item._arr_idx != cur_arr._arr_idx {
			bdel = true

			if cur_arr.Remove(cur_rd.Id) {

				bfind := false
				cur_arr_idx = -1
				for tmp_idx = 0; tmp_idx < this._array_len; tmp_idx++ {
					if bfind {
						this.Array[tmp_idx]._rank_base--
					}
					if cur_arr == this.Array[tmp_idx] {
						bfind = true
						cur_arr_idx = tmp_idx
					}
				}

				if cur_arr_idx >= 0 && cur_arr.CurCount <= 0 {
					tmp_arr_item := this.Array[cur_arr_idx]

					for tmp_idx = cur_arr_idx; tmp_idx < this._array_len-1; tmp_idx++ {
						this.Array[tmp_idx] = this.Array[tmp_idx+1]
						this.Array[tmp_idx]._arr_idx = tmp_idx
						if arr_idx == tmp_idx+1 {
							arr_idx--
						}
					}

					tmp_arr_item._arr_idx = this._array_len - 1
					this.Array[this._array_len-1] = tmp_arr_item
					log.Info("===================空列调整 %d=====================")
					this.PrintAllRecords()
					log.Info("arr_idx %d", arr_idx)
					log.Info("=====================End %d=======================")
				}
			}
		}

		rd_item, badd := arr_item.SetVal(id, val)
		if nil != rd_item {
			if badd {
				if arr_item.CurCount == 1 {
					tmp_base := int32(0)
					for tmp_idx := int32(0); tmp_idx < arr_idx; tmp_idx++ {
						tmp_base += this.Array[tmp_idx].CurCount
					}
					arr_item._rank_base = tmp_base
				}
				//this.id2item[rd_item.Id] = rd_item
				for tmp_idx = arr_idx + 1; tmp_idx < this._array_len; tmp_idx++ {
					if this.Array[arr_idx].CurCount > 0 {
						this.Array[tmp_idx]._rank_base++
					}
				}
			}

			if arr_item.CurCount >= arr_item.MaxCount && arr_idx != this._array_len-1 {
				end_arr_item := this.Array[this._array_len-1]
				for tmp_idx := int32(0); tmp_idx < end_arr_item.CurCount; tmp_idx++ {
					delete(this.id2item, end_arr_item.Array[tmp_idx].Id)
				}
				arr_item.HalfBreak(end_arr_item)
				this.AddRankArrayItem(arr_idx+1, end_arr_item)
			}
		} else {
			if !bdel && nil != cur_arr && arr_item._arr_idx != cur_arr._arr_idx {
				if cur_arr.Remove(id) {

					bfind := false
					cur_arr_idx = -1
					for tmp_idx = 0; tmp_idx < this._array_len; tmp_idx++ {
						if bfind {
							this.Array[tmp_idx]._rank_base--
						}
						if cur_arr == this.Array[tmp_idx] {
							bfind = true
							cur_arr_idx = tmp_idx
						}
					}

					if cur_arr_idx >= 0 && cur_arr.CurCount <= 0 {
						tmp_arr_item := this.Array[cur_arr_idx]

						for tmp_idx = cur_arr_idx; tmp_idx < this._array_len-1; tmp_idx++ {
							this.Array[tmp_idx] = this.Array[tmp_idx+1]
							this.Array[tmp_idx]._arr_idx = tmp_idx
						}

						tmp_arr_item._arr_idx = this._array_len - 1
						this.Array[this._array_len-1] = tmp_arr_item
					}
				}
			}
		}

	}
}

func (this *BigRankService) PrintAllRecords() {
	var tmp_arr *BigRankArrayItem
	var arr_idx int32
	var tmp_rank_val int32
	for arr_idx = 0; arr_idx < this._array_len; arr_idx++ {
		tmp_arr = this.Array[arr_idx]
		if nil == tmp_arr {
			log.Error("BigRankService PrintAllRecords tmp_arr[%d] nil !", arr_idx)
			continue
		}

		log.Info("Array:[arr_idx:%d/%d, _rank_base:%d, count:%d, max_count:%d]", arr_idx, tmp_arr._arr_idx, tmp_arr._rank_base, tmp_arr.CurCount, tmp_arr.MaxCount)
		arr_ids := make(map[int32]bool)
		for rd_idx := int32(0); rd_idx < tmp_arr.CurCount; rd_idx++ {

			log.Info("	val %v :[rank:%d, Val:%d], Id:%d,  array_idx:%d", tmp_arr.Array[rd_idx], rd_idx+tmp_arr._rank_base, tmp_arr.Array[rd_idx].Val, tmp_arr.Array[rd_idx].Id, tmp_arr.Array[rd_idx]._arr._arr_idx)
			arr_ids[tmp_arr.Array[rd_idx].Id] = true
			if tmp_rank_val != rd_idx+tmp_arr._rank_base {
				log.Info("rank_un_match !!")
			}
			tmp_rank_val++
		}
		log.Info("	-----cur map---- ")
		for map_idx, val := range tmp_arr.Id2Item {
			log.Info("	map_idd:%d, val:%v", map_idx, val)
		}
		if int32(len(tmp_arr.Id2Item)) != tmp_arr.CurCount {
			log.Info("count not_match !!")
		}

		for map_idx, _ := range tmp_arr.Id2Item {
			if !arr_ids[map_idx] {
				log.Info("map_idx_not_in !!")
				break
			}
		}
	}

	log.Info("rank map %v", this.id2item)
	log.Info("BigRankService PrintAllRecords over arr_idx[%d] !", arr_idx)
}
