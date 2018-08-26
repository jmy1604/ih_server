package utils

type SimpleItemFactory interface {
	New() interface{}
}

type SimpleItemPool struct {
	items         []interface{}
	can_use_start int32
	can_use_num   int32
	item_factory  SimpleItemFactory
}

func (this *SimpleItemPool) Init(max_num int32, factory SimpleItemFactory) {
	this.items = make([]interface{}, max_num)
	for i := int32(0); i < max_num; i++ {
		this.items[i] = factory.New()
	}
	this.can_use_start = 0
	this.can_use_num = max_num
}

func (this *SimpleItemPool) GetFree() interface{} {
	if this.can_use_start < 0 {
		return nil
	}
	item := this.items[this.can_use_start]
	this.can_use_start += 1
	if this.can_use_start >= int32(len(this.items)) {
		this.can_use_start = -1
	}
	return item
}

func (this *SimpleItemPool) Recycle(item interface{}) bool {
	if this.can_use_start == 0 {
		return false
	}
	if this.can_use_start < 0 {
		this.can_use_start = int32(len(this.items)) - 1
	} else {
		this.can_use_start -= 1
	}
	this.items[this.can_use_start] = item
	return true
}

func (this *SimpleItemPool) HasFree() bool {
	if this.can_use_start >= 0 {
		return true
	}
	return false
}
