package main

//"libs/log"

const (
	DEFAULT_SMALL_RANK_LENTH = 200
)

const (
	SMALL_RANK_SORT_TYPE_B_E = 0 // 按大于等于规则进行排序
	SMALL_RANK_SORT_TYPE_B   = 1 // 按大于规则进行排序
)

const (
	SMALL_RANK_TYPE_STAGE_SCORE = 1 // 关卡积分榜
)

type SmallRankRecord struct {
	rank        int32
	id          int32
	val         int32
	name        string
	lvl         int32
	icon        string
	custom_icon string
}

type SmallRankService struct {
	rank_len    int32
	rank_lock   *RWMutex
	rankrecords []*SmallRankRecord         // 榜单数据
	id2record   map[int32]*SmallRankRecord // id到具体记录的映射
	brankchg    bool
	curcount    int32
	rank_type   int32
	sort_type   int32
}

func NewSmallRankService(rank_len, rank_type, sort_type int32) *SmallRankService {

	if rank_len <= 0 {
		rank_len = DEFAULT_SMALL_RANK_LENTH
	}
	retrank := &SmallRankService{}
	retrank.rank_len = rank_len
	retrank.rank_lock = NewRWMutex()
	retrank.rankrecords = make([]*SmallRankRecord, rank_len)
	for i := int32(0); i < rank_len; i++ {
		retrank.rankrecords[i] = &SmallRankRecord{}
	}
	retrank.id2record = make(map[int32]*SmallRankRecord)
	retrank.rank_type = rank_type
	retrank.curcount = 0

	if 0 != sort_type {
		retrank.sort_type = 1
	} else {
		retrank.sort_type = 0
	}

	return retrank
}

func (this *SmallRankService) ranksort(val1, val2 int32) bool {
	if 0 == this.sort_type {
		return val1 >= val2
	} else {
		return val1 > val2
	}
}

func (this *SmallRankService) SetUpdateRankExt(id, val, lvl int32, icon string, name, custom_icon string) (retrank, retval int32) {
	this.rank_lock.UnSafeLock("SmallRankService.SetUpdateRankExt")
	defer this.rank_lock.UnSafeUnlock()

	oldrankrecord := this.id2record[id]
	if nil != oldrankrecord {
		retrank = oldrankrecord.rank
		retval = oldrankrecord.val

		idx := int32(0)
		for ; idx < this.curcount; idx++ {
			if this.ranksort(val, this.rankrecords[idx].val) {
				break
			}
		}

		oldidx := oldrankrecord.rank

		if idx == oldidx {
			oldrankrecord.val = val
			retval = val
			this.brankchg = true
			return
		}

		if idx > oldidx {
			idx--
			if idx >= this.rank_len {
				idx = this.rank_len - 1
			}

			for i := oldidx; i < int32(idx); i++ {
				*this.rankrecords[i] = *this.rankrecords[i+1]
				this.rankrecords[i].rank = int32(i)
				if 0 != this.rankrecords[i].id {
					this.id2record[this.rankrecords[i].id] = this.rankrecords[i]
				}
			}
		} else {
			if idx >= this.rank_len {
				idx = this.rank_len - 1
			}

			for i := oldidx; i > int32(idx); i-- {
				*this.rankrecords[i] = *this.rankrecords[i-1]
				this.rankrecords[i].rank = int32(i)
				if 0 != this.rankrecords[i].id {
					this.id2record[this.rankrecords[i].id] = this.rankrecords[i]
				}
			}
		}

		this.rankrecords[idx].id = id
		this.rankrecords[idx].val = val
		this.rankrecords[idx].rank = idx
		this.rankrecords[idx].name = name
		this.rankrecords[idx].lvl = lvl
		this.rankrecords[idx].icon = icon
		this.rankrecords[idx].custom_icon = custom_icon
		this.id2record[id] = this.rankrecords[idx]

		retrank = idx
		retval = val
		//log.Info("排行榜数据更新到", this.rank_type, idx, this.id2record[id].id, val)

	} else {
		retrank = -1
		retval = val

		if val < this.rankrecords[this.rank_len-1].val {
			return
		}

		idx := int32(0)
		for ; idx < this.rank_len; idx++ {
			if this.ranksort(val, this.rankrecords[idx].val) {
				break
			}
		}

		if idx >= this.rank_len {
			return
		}

		//log.Info("求出的新Idx", idx)

		bottom := this.rankrecords[this.rank_len-1]
		if nil != bottom {
			this.id2record[bottom.id] = nil
		}
		for i := this.rank_len - 1; i > idx; i-- {
			*this.rankrecords[i] = *this.rankrecords[i-1]
			this.rankrecords[i].rank = int32(i)
			if 0 != this.rankrecords[i].id {
				this.id2record[this.rankrecords[i].id] = this.rankrecords[i]
			}
		}

		this.rankrecords[idx].id = id
		this.rankrecords[idx].val = val
		this.rankrecords[idx].rank = idx
		this.rankrecords[idx].name = name
		this.rankrecords[idx].icon = icon
		this.rankrecords[idx].custom_icon = custom_icon
		this.rankrecords[idx].lvl = lvl
		this.id2record[id] = this.rankrecords[idx]

		retrank = idx
		retval = val

		if this.curcount < this.rank_len {
			this.curcount++
		}

		//log.Info("新的排行榜数据添加到", this.rank_type, idx, val)
	}

	this.brankchg = true

	return
}

func (this *SmallRankService) SetUpdateRank(id, val, lvl int32, icon string, name, custom_icon string) (retrank, retval int32) {
	this.rank_lock.UnSafeLock("SmallRankService.SetUpdateRank")
	defer this.rank_lock.UnSafeUnlock()

	oldrankrecord := this.id2record[id]
	if nil != oldrankrecord {
		retrank = oldrankrecord.rank
		retval = oldrankrecord.val

		idx := int32(0)
		for ; idx < this.curcount; idx++ {
			if this.ranksort(val, this.rankrecords[idx].val) {
				break
			}
		}

		oldidx := oldrankrecord.rank

		if idx == oldidx {
			oldrankrecord.val = val
			retval = val
			this.brankchg = true
			return
		}

		if idx > oldidx {
			idx--
			if idx >= this.rank_len {
				idx = this.rank_len - 1
			}

			for i := oldidx; i < int32(idx); i++ {
				*this.rankrecords[i] = *this.rankrecords[i+1]
				this.rankrecords[i].rank = int32(i)
				if 0 != this.rankrecords[i].id {
					this.id2record[this.rankrecords[i].id] = this.rankrecords[i]
				}
			}
		} else {
			if idx >= this.rank_len {
				idx = this.rank_len - 1
			}

			for i := oldidx; i > int32(idx); i-- {
				*this.rankrecords[i] = *this.rankrecords[i-1]
				this.rankrecords[i].rank = int32(i)
				if 0 != this.rankrecords[i].id {
					this.id2record[this.rankrecords[i].id] = this.rankrecords[i]
				}
			}
		}

		this.rankrecords[idx].id = id
		this.rankrecords[idx].val = val
		this.rankrecords[idx].rank = idx
		this.rankrecords[idx].name = name
		this.rankrecords[idx].icon = icon
		this.rankrecords[idx].custom_icon = custom_icon
		this.rankrecords[idx].lvl = lvl
		this.id2record[id] = this.rankrecords[idx]

		retrank = idx
		retval = val
		//log.Info("排行榜数据更新到", this.rank_type, idx, this.id2record[id].id, val)

	} else {
		retrank = -1
		retval = val

		if val < this.rankrecords[this.rank_len-1].val {
			return
		}

		idx := int32(0)
		for ; idx < this.rank_len; idx++ {
			if this.ranksort(val, this.rankrecords[idx].val) {
				break
			}
		}

		if idx >= this.rank_len {
			return
		}

		//log.Info("求出的新Idx", idx)

		bottom := this.rankrecords[this.rank_len-1]
		if nil != bottom {
			this.id2record[bottom.id] = nil
		}
		for i := this.rank_len - 1; i > idx; i-- {
			*this.rankrecords[i] = *this.rankrecords[i-1]
			this.rankrecords[i].rank = int32(i)
			if 0 != this.rankrecords[i].id {
				this.id2record[this.rankrecords[i].id] = this.rankrecords[i]
			}
		}

		this.rankrecords[idx].id = id
		this.rankrecords[idx].val = val
		this.rankrecords[idx].rank = idx
		this.rankrecords[idx].name = name
		this.rankrecords[idx].icon = icon
		this.rankrecords[idx].custom_icon = custom_icon
		this.rankrecords[idx].lvl = lvl
		this.id2record[id] = this.rankrecords[idx]

		retrank = idx
		retval = val

		if this.curcount < this.rank_len {
			this.curcount++
		}

		//log.Info("新的排行榜数据添加到", this.rank_type, idx, val)
	}

	this.brankchg = true

	return
}

func (this *SmallRankService) AddUpdateRank(id, val, lvl int32, icon string, name, custom_icon string) (retrank, retval int32) {
	this.rank_lock.UnSafeLock("SmallRankService.AddUpdateRank")
	defer this.rank_lock.UnSafeUnlock()

	oldrankrecord := this.id2record[id]
	if nil != oldrankrecord {
		retrank = oldrankrecord.rank
		val = oldrankrecord.val + val
		retval = oldrankrecord.val

		if val < 0 {
			val = 0
		}

		idx := int32(0)
		for ; idx < this.curcount; idx++ {
			if this.ranksort(val, this.rankrecords[idx].val) {
				break
			}
		}

		oldidx := oldrankrecord.rank

		if idx == oldidx {
			oldrankrecord.val = val
			retval = val
			this.brankchg = true
			return
		}

		if idx > oldidx {
			idx--
			if idx >= this.rank_len {
				idx = this.rank_len - 1
			}

			for i := oldidx; i < int32(idx); i++ {
				*this.rankrecords[i] = *this.rankrecords[i+1]
				this.rankrecords[i].rank = int32(i)
				if 0 != this.rankrecords[i].id {
					this.id2record[this.rankrecords[i].id] = this.rankrecords[i]
				}
			}
		} else {

			if idx >= this.rank_len {
				idx = this.rank_len - 1
			}

			for i := oldidx; i > int32(idx); i-- {
				*this.rankrecords[i] = *this.rankrecords[i-1]
				this.rankrecords[i].rank = int32(i)
				if 0 != this.rankrecords[i].id {
					this.id2record[this.rankrecords[i].id] = this.rankrecords[i]
				}
			}
		}

		this.rankrecords[idx].id = id
		this.rankrecords[idx].val = val
		this.rankrecords[idx].rank = idx
		this.rankrecords[idx].name = name
		this.rankrecords[idx].icon = icon
		this.rankrecords[idx].custom_icon = custom_icon
		this.rankrecords[idx].lvl = lvl

		this.id2record[id] = this.rankrecords[idx]

		retrank = idx
		retval = val
		//log.Info("排行榜数据更新到", this.rank_type, idx, this.id2record[id].id, val)

	} else {
		if val < 0 || val < this.rankrecords[this.rank_len-1].val {
			return
		}

		idx := int32(0)
		for ; idx < this.rank_len; idx++ {
			if this.ranksort(val, this.rankrecords[idx].val) {
				break
			}
		}

		if idx >= this.rank_len {
			return
		}

		for i := this.rank_len - 1; i > idx; i-- {
			*this.rankrecords[i] = *this.rankrecords[i-1]
			this.rankrecords[i].rank = int32(i)
			if 0 != this.rankrecords[i].id {
				this.id2record[this.rankrecords[i].id] = this.rankrecords[i]
			}
		}

		this.rankrecords[idx].id = id
		this.rankrecords[idx].val = val
		this.rankrecords[idx].rank = idx
		this.rankrecords[idx].name = name
		this.rankrecords[idx].icon = icon
		this.rankrecords[idx].custom_icon = custom_icon
		this.rankrecords[idx].lvl = lvl
		this.id2record[id] = this.rankrecords[idx]

		retrank = idx
		retval = val

		if this.curcount < this.rank_len {
			this.curcount++
		}

		//log.Info("新的排行榜数据添加到", this.rank_type, idx, val)
	}

	this.brankchg = true
	return
}

func (this *SmallRankService) GetRankById(id int32) int32 {
	this.rank_lock.UnSafeRLock("SmallRankService.GetRankById")
	defer this.rank_lock.UnSafeRUnlock()

	if nil == this.id2record[id] {
		return -1
	}

	return this.id2record[id].rank
}

func (this *SmallRankService) GetAllIfChanged() (bchg bool, records []*SmallRankRecord) {
	this.rank_lock.UnSafeRLock("SmallRankService.GetRankById")
	defer this.rank_lock.UnSafeRUnlock()

	bchg = this.brankchg
	if !bchg {
		return
	}

	records = make([]*SmallRankRecord, this.curcount)
	for idx := int32(0); idx < this.curcount; idx++ { //for idx, val := range this.rankrecords {
		val := this.rankrecords[idx]
		if nil == val {
			continue
		}

		if 0 >= val.val {
			continue
		}

		records[idx] = &SmallRankRecord{}
		*records[idx] = *val
	}

	this.brankchg = true

	return
}

func (this *SmallRankService) GetTopN(n int32) (records []*SmallRankRecord) {
	this.rank_lock.UnSafeRLock("SmallRankService.GetTopN")
	defer this.rank_lock.UnSafeRUnlock()

	if n <= 0 {
		return
	}

	if n > this.curcount {
		n = this.curcount
	}

	records = make([]*SmallRankRecord, n)
	for idx := int32(0); idx < n; idx++ {
		records[idx] = &SmallRankRecord{}
		*records[idx] = *this.rankrecords[idx]
	}

	return
}

func (this *SmallRankService) GetNFromRank(ids []int32) (records []*SmallRankRecord) {
	this.rank_lock.UnSafeRLock("SmallRankService.GetNFromRank")
	defer this.rank_lock.UnSafeRUnlock()

	if nil == ids || len(ids) < 1 {
		return
	}

	records = make([]*SmallRankRecord, 0, len(ids))

	for _, pid := range ids {
		cur_rd := this.id2record[pid]
		if nil != cur_rd {
			records = append(records, &SmallRankRecord{
				rank: cur_rd.rank,
				id:   pid,
				val:  cur_rd.val,
			})
		} else {
			records = append(records, &SmallRankRecord{
				rank: -1,
				id:   pid,
				val:  0,
			})
		}
	}

	return
}

func (this *SmallRankService) GetNMapFromRank(ids []int32) (mrecords map[int32]*SmallRankRecord) {
	this.rank_lock.UnSafeRLock("SmallRankService.GetNFromRank")
	defer this.rank_lock.UnSafeRUnlock()

	mrecords = make(map[int32]*SmallRankRecord)
	if nil == ids || len(ids) < 1 {
		return
	}

	for _, pid := range ids {
		cur_rd := this.id2record[pid]
		if nil != cur_rd {
			mrecords[pid] = &SmallRankRecord{
				rank: cur_rd.rank,
				id:   pid,
				val:  cur_rd.val,
			}
		} else {
			mrecords[pid] = &SmallRankRecord{
				rank: -1,
				id:   pid,
				val:  0,
			}
		}
	}

	return
}

func (this *SmallRankService) IfIdInTopN(chkid, n int32) bool {
	this.rank_lock.UnSafeRLock("SmallRankService.GetNFromRank")
	defer this.rank_lock.UnSafeRUnlock()

	oldrankrecord := this.id2record[chkid]
	if nil == oldrankrecord || oldrankrecord.rank > n {
		return false
	}

	return true
}

func (this *SmallRankService) GetAllAndReset() (records []*SmallRankRecord) {
	this.rank_lock.UnSafeRLock("SmallRankService.GetAllAndReset")
	defer this.rank_lock.UnSafeRUnlock()

	records = make([]*SmallRankRecord, this.curcount)
	for idx := int32(0); idx < this.curcount; idx++ {
		if this.rankrecords[idx].id <= 0 {
			continue
		}

		records[idx] = &SmallRankRecord{}
		*records[idx] = *this.rankrecords[idx]
	}

	this.curcount = 0
	this.brankchg = true

	return
}

func (this *SmallRankService) Reset() {
	this.rank_lock.UnSafeRLock("SmallRankService.Reset")
	defer this.rank_lock.UnSafeRUnlock()

	this.curcount = 0
	this.brankchg = true

	return
}
