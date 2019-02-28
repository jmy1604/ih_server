package main

import (
	"ih_server/libs/utils"
	"time"
)

type PlayerInt32RankItem struct {
	Value      int32
	UpdateTime int32
	PlayerId   int32
}

type PlayerInt64RankItem struct {
	Value      int64
	UpdateTime int32
	PlayerId   int32
}

func (this *PlayerInt32RankItem) Less(value interface{}) bool {
	item := value.(*PlayerInt32RankItem)
	if item == nil {
		return false
	}
	if this.Value < item.Value {
		return true
	} else if this.Value == item.Value {
		if this.UpdateTime > item.UpdateTime {
			return true
		}
		if this.UpdateTime == item.UpdateTime {
			if this.PlayerId > item.PlayerId {
				return true
			}
		}
	}
	return false
}

func (this *PlayerInt32RankItem) Greater(value interface{}) bool {
	item := value.(*PlayerInt32RankItem)
	if item == nil {
		return false
	}
	if this.Value > item.Value {
		return true
	} else if this.Value == item.Value {
		if this.UpdateTime < item.UpdateTime {
			return true
		}
		if this.UpdateTime == item.UpdateTime {
			if this.PlayerId < item.PlayerId {
				return true
			}
		}
	}
	return false
}

func (this *PlayerInt32RankItem) KeyEqual(value interface{}) bool {
	item := value.(*PlayerInt32RankItem)
	if item == nil {
		return false
	}
	if item == nil {
		return false
	}
	if this.PlayerId == item.PlayerId {
		return true
	}
	return false
}

func (this *PlayerInt32RankItem) GetKey() interface{} {
	return this.PlayerId
}

func (this *PlayerInt32RankItem) GetValue() interface{} {
	return this.Value
}

func (this *PlayerInt32RankItem) SetValue(value interface{}) {
	this.Value = value.(int32)
	this.UpdateTime = int32(time.Now().Unix())
}

func (this *PlayerInt32RankItem) New() utils.SkiplistNode {
	return &PlayerInt32RankItem{}
}

func (this *PlayerInt32RankItem) Assign(node utils.SkiplistNode) {
	n := node.(*PlayerInt32RankItem)
	if n == nil {
		return
	}
	this.Value = n.Value
	this.UpdateTime = n.UpdateTime
	this.PlayerId = n.PlayerId
}

func (this *PlayerInt32RankItem) CopyDataTo(node interface{}) {
	n := node.(*PlayerInt32RankItem)
	if n == nil {
		return
	}
	n.Value = this.Value
	n.UpdateTime = this.UpdateTime
	n.PlayerId = this.PlayerId
}

func (this *PlayerInt64RankItem) Less(value interface{}) bool {
	item := value.(*PlayerInt64RankItem)
	if item == nil {
		return false
	}
	if this.Value < item.Value {
		return true
	} else if this.Value == item.Value {
		if this.UpdateTime > item.UpdateTime {
			return true
		}
		if this.UpdateTime == item.UpdateTime {
			if this.PlayerId > item.PlayerId {
				return true
			}
		}
	}
	return false
}

func (this *PlayerInt64RankItem) Greater(value interface{}) bool {
	item := value.(*PlayerInt64RankItem)
	if item == nil {
		return false
	}
	if this.Value > item.Value {
		return true
	} else if this.Value == item.Value {
		if this.UpdateTime < item.UpdateTime {
			return true
		}
		if this.UpdateTime == item.UpdateTime {
			if this.PlayerId < item.PlayerId {
				return true
			}
		}
	}
	return false
}

func (this *PlayerInt64RankItem) KeyEqual(value interface{}) bool {
	item := value.(*PlayerInt64RankItem)
	if item == nil {
		return false
	}
	if item == nil {
		return false
	}
	if this.PlayerId == item.PlayerId {
		return true
	}
	return false
}

func (this *PlayerInt64RankItem) GetKey() interface{} {
	return this.PlayerId
}

func (this *PlayerInt64RankItem) GetValue() interface{} {
	return this.Value
}

func (this *PlayerInt64RankItem) SetValue(value interface{}) {
	this.Value = value.(int64)
	this.UpdateTime = int32(time.Now().Unix())
}

func (this *PlayerInt64RankItem) New() utils.SkiplistNode {
	return &PlayerInt64RankItem{}
}

func (this *PlayerInt64RankItem) Assign(node utils.SkiplistNode) {
	n := node.(*PlayerInt64RankItem)
	if n == nil {
		return
	}
	this.Value = n.Value
	this.UpdateTime = n.UpdateTime
	this.PlayerId = n.PlayerId
}

func (this *PlayerInt64RankItem) CopyDataTo(node interface{}) {
	n := node.(*PlayerInt64RankItem)
	if n == nil {
		return
	}
	n.Value = this.Value
	n.UpdateTime = this.UpdateTime
	n.PlayerId = this.PlayerId
}
