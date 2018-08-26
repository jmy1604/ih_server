package main

import (
	"ih_server/libs/utils"
	"ih_server/src/rpc_common"
)

const (
	RANKING_LIST_TYPE_STAGE_TOTAL_SCORE = 1
	RANKING_LIST_TYPE_STAGE_SCORE       = 2
	RANKING_LIST_TYPE_CHARM             = 3
	RANKING_LIST_TYPE_CAT_OUQI          = 4
	RANKING_LIST_TYPE_ZANED             = 5
)

// 关卡总分排行项
type RankStageTotalScoreItem struct {
	PlayerId        int32
	PlayerLevel     int32
	StageTotalScore int32
	SaveTime        int32
}

// 每关排行项
type RankStageScoreItem struct {
	PlayerId    int32
	PlayerLevel int32
	StageId     int32
	StageScore  int32
	SaveTime    int32
}

// 魅力排行项
type RankCharmItem struct {
	PlayerId    int32
	PlayerLevel int32
	Charm       int32
	SaveTime    int32
}

// 猫欧气值排行项
type RankOuqiItem struct {
	PlayerId    int32
	PlayerLevel int32
	CatId       int32
	CatTableId  int32
	CatOuqi     int32
	CatLevel    int32
	CatStar     int32
	CatNick     string
	SaveTime    int32
}

// 被赞排行项
type RankZanedItem struct {
	PlayerId    int32
	PlayerLevel int32
	Zaned       int32
	SaveTime    int32
}

/* 关卡总积分 */
func (this *RankStageTotalScoreItem) Less(value interface{}) bool {
	item := value.(*RankStageTotalScoreItem)
	if item == nil {
		return false
	}
	if this.StageTotalScore < item.StageTotalScore {
		return true
	}
	if this.StageTotalScore == item.StageTotalScore {
		if this.SaveTime > item.SaveTime {
			return true
		}
		if this.SaveTime == item.SaveTime {
			if this.PlayerLevel < item.PlayerLevel {
				return true
			}
			if this.PlayerLevel == item.PlayerLevel {
				if this.PlayerId < item.PlayerId {
					return true
				}
			}
		}
	}
	return false
}

func (this *RankStageTotalScoreItem) Greater(value interface{}) bool {
	item := value.(*RankStageTotalScoreItem)
	if item == nil {
		return false
	}
	if this.StageTotalScore > item.StageTotalScore {
		return true
	}
	if this.StageTotalScore == item.StageTotalScore {
		if this.SaveTime < item.SaveTime {
			return true
		}
		if this.SaveTime == item.SaveTime {
			if this.PlayerLevel > item.PlayerLevel {
				return true
			}
			if this.PlayerLevel == item.PlayerLevel {
				if this.PlayerId > item.PlayerId {
					return true
				}
			}
		}
	}
	return false
}

func (this *RankStageTotalScoreItem) KeyEqual(value interface{}) bool {
	item := value.(*RankStageTotalScoreItem)
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

func (this *RankStageTotalScoreItem) GetKey() interface{} {
	return this.PlayerId
}

func (this *RankStageTotalScoreItem) GetValue() interface{} {
	return this.StageTotalScore
}

func (this *RankStageTotalScoreItem) SetValue(value interface{}) {

}

func (this *RankStageTotalScoreItem) New() utils.SkiplistNode {
	return &RankStageTotalScoreItem{}
}

func (this *RankStageTotalScoreItem) Assign(node utils.SkiplistNode) {
	n := node.(*RankStageTotalScoreItem)
	if n == nil {
		return
	}
	this.PlayerId = n.PlayerId
	this.PlayerLevel = n.PlayerLevel
	this.StageTotalScore = n.StageTotalScore
	this.SaveTime = n.SaveTime
}

func (this *RankStageTotalScoreItem) CopyDataTo(node interface{}) {
	n := node.(*rpc_common.H2R_RankStageTotalScore)
	if n == nil {
		return
	}
	n.PlayerId = this.PlayerId
	n.PlayerLevel = this.PlayerLevel
	n.TotalScore = this.StageTotalScore
}

/* 关卡积分*/
func (this *RankStageScoreItem) Less(value interface{}) bool {
	item := value.(*RankStageScoreItem)
	if item == nil {
		return false
	}
	if this.StageScore < item.StageScore {
		return true
	}
	if this.StageScore == item.StageScore {
		if this.SaveTime > item.SaveTime {
			return true
		}
		if this.SaveTime == item.SaveTime {
			if this.PlayerLevel < item.PlayerLevel {
				return true
			}
			if this.PlayerLevel == item.PlayerLevel {
				if this.PlayerId < item.PlayerId {
					return true
				}
			}
		}
	}
	return false
}

func (this *RankStageScoreItem) Greater(value interface{}) bool {
	item := value.(*RankStageScoreItem)
	if item == nil {
		return false
	}
	if this.StageScore > item.StageScore {
		return true
	}
	if this.StageScore == item.StageScore {
		if this.SaveTime < item.SaveTime {
			return true
		}
		if this.SaveTime == item.SaveTime {
			if this.PlayerLevel > item.PlayerLevel {
				return true
			}
			if this.PlayerLevel == item.PlayerLevel {
				if this.PlayerId > item.PlayerId {
					return true
				}
			}
		}
	}
	return false
}

func (this *RankStageScoreItem) KeyEqual(value interface{}) bool {
	item := value.(*RankStageScoreItem)
	if item == nil {
		return false
	}
	if this.PlayerId == item.PlayerId {
		return true
	}
	return false
}

func (this *RankStageScoreItem) GetKey() interface{} {
	return this.PlayerId
}

func (this *RankStageScoreItem) GetValue() interface{} {
	return this.StageScore
}

func (this *RankStageScoreItem) SetValue(value interface{}) {

}

func (this *RankStageScoreItem) New() utils.SkiplistNode {
	return &RankStageScoreItem{}
}

func (this *RankStageScoreItem) Assign(node utils.SkiplistNode) {
	n := node.(*RankStageScoreItem)
	if n != nil {
		this.PlayerId = n.PlayerId
		this.PlayerLevel = n.PlayerLevel
		this.StageId = n.StageId
		this.StageScore = n.StageScore
		this.SaveTime = n.SaveTime
	}
}

func (this *RankStageScoreItem) CopyDataTo(node interface{}) {
	n := node.(*rpc_common.H2R_RankStageScore)
	if n == nil {
		return
	}
	n.PlayerId = this.PlayerId
	n.PlayerLevel = this.PlayerLevel
	n.StageId = this.StageId
	n.StageScore = this.StageScore
}

/*魅力值*/
func (this *RankCharmItem) Less(value interface{}) bool {
	item := value.(*RankCharmItem)
	if item == nil {
		return false
	}
	if this.Charm < item.Charm {
		return true
	}
	if this.Charm == item.Charm {
		if this.SaveTime > item.SaveTime {
			return true
		}
		if this.SaveTime == item.SaveTime {
			if this.PlayerLevel < item.PlayerLevel {
				return true
			}
			if this.PlayerLevel == item.PlayerLevel {
				if this.PlayerId < item.PlayerId {
					return true
				}
			}
		}
	}
	return false
}

func (this *RankCharmItem) Greater(value interface{}) bool {
	item := value.(*RankCharmItem)
	if item == nil {
		return false
	}
	if this.Charm > item.Charm {
		return true
	}
	if this.Charm == item.Charm {
		if this.SaveTime < item.SaveTime {
			return true
		}
		if this.SaveTime == item.SaveTime {
			if this.PlayerLevel > item.PlayerLevel {
				return true
			}
			if this.PlayerLevel == item.PlayerLevel {
				if this.PlayerId > item.PlayerId {
					return true
				}
			}
		}
	}
	return false
}

func (this *RankCharmItem) KeyEqual(value interface{}) bool {
	item := value.(*RankCharmItem)
	if item == nil {
		return false
	}
	if this.PlayerId == item.PlayerId {
		return true
	}
	return false
}

func (this *RankCharmItem) GetKey() interface{} {
	return this.PlayerId
}

func (this *RankCharmItem) GetValue() interface{} {
	return this.Charm
}

func (this *RankCharmItem) SetValue(value interface{}) {

}

func (this *RankCharmItem) New() utils.SkiplistNode {
	return &RankCharmItem{}
}

func (this *RankCharmItem) Assign(node utils.SkiplistNode) {
	n := node.(*RankCharmItem)
	if n != nil {
		this.PlayerId = n.PlayerId
		this.PlayerLevel = n.PlayerLevel
		this.Charm = n.Charm
		this.SaveTime = n.SaveTime
	}
}

func (this *RankCharmItem) CopyDataTo(node interface{}) {
	n := node.(*rpc_common.H2R_RankCharm)
	if n == nil {
		return
	}
	n.PlayerId = this.PlayerId
	n.PlayerLevel = this.PlayerLevel
	n.Charm = this.Charm
}

/*被赞*/
func (this *RankZanedItem) Less(value interface{}) bool {
	item := value.(*RankZanedItem)
	if item == nil {
		return false
	}
	if this.Zaned < item.Zaned {
		return true
	}
	if this.Zaned == item.Zaned {
		if this.SaveTime > item.SaveTime {
			return true
		}
		if this.SaveTime == item.SaveTime {
			if this.PlayerLevel < item.PlayerLevel {
				return true
			}
			if this.PlayerLevel == item.PlayerLevel {
				if this.PlayerId < item.PlayerId {
					return true
				}
			}
		}
	}
	return false
}

func (this *RankZanedItem) Greater(value interface{}) bool {
	item := value.(*RankZanedItem)
	if item == nil {
		return false
	}
	if this.Zaned > item.Zaned {
		return true
	}
	if this.Zaned == item.Zaned {
		if this.SaveTime < item.SaveTime {
			return true
		}
		if this.SaveTime == item.SaveTime {
			if this.PlayerLevel > item.PlayerLevel {
				return true
			}
			if this.PlayerLevel == item.PlayerLevel {
				if this.PlayerId > item.PlayerId {
					return true
				}
			}
		}
	}
	return false
}

func (this *RankZanedItem) KeyEqual(value interface{}) bool {
	item := value.(*RankZanedItem)
	if item == nil {
		return false
	}
	if this.PlayerId == item.PlayerId {
		return true
	}
	return false
}

func (this *RankZanedItem) GetKey() interface{} {
	return this.PlayerId
}

func (this *RankZanedItem) GetValue() interface{} {
	return this.Zaned
}

func (this *RankZanedItem) SetValue(value interface{}) {

}

func (this *RankZanedItem) New() utils.SkiplistNode {
	return &RankZanedItem{}
}

func (this *RankZanedItem) Assign(node utils.SkiplistNode) {
	n := node.(*RankZanedItem)
	if n != nil {
		this.PlayerId = n.PlayerId
		this.PlayerLevel = n.PlayerLevel
		this.Zaned = n.Zaned
		this.SaveTime = n.SaveTime
	}
}

func (this *RankZanedItem) CopyDataTo(node interface{}) {
	n := node.(*rpc_common.H2R_RankZaned)
	if n == nil {
		return
	}
	n.PlayerId = this.PlayerId
	n.PlayerLevel = this.PlayerLevel
	n.Zaned = this.Zaned
}
