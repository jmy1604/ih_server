package main

import (
	"ih_server/libs/log"
	"ih_server/libs/utils"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"sync"

	"github.com/golang/protobuf/proto"
)

const (
	RANK_LIST_TYPE_NONE              = iota
	RANK_LIST_TYPE_ARENA             = 1
	RANK_LIST_TYPE_CAMPAIGN          = 2
	RANK_LIST_TYPE_ROLE_POWER        = 3
	RANK_LIST_TYPE_TOP_DEFENSE_TOWER = 15 // 最高防守阵型战力
	RANK_LIST_TYPE_MAX               = 16
)

type RankList struct {
	rank_list *utils.CommonRankingList
	item_pool *sync.Pool
	root_node utils.SkiplistNode
}

func (this *RankList) Init(root_node utils.SkiplistNode) {
	this.root_node = root_node
	this.rank_list = utils.NewCommonRankingList(this.root_node, ARENA_RANK_MAX)
	this.item_pool = &sync.Pool{
		New: func() interface{} {
			return this.root_node.New()
		},
	}
}

func (this *RankList) GetItemByKey(key interface{}) (item utils.SkiplistNode) {
	return this.rank_list.GetByKey(key)
}

func (this *RankList) GetRankByKey(key interface{}) int32 {
	return this.rank_list.GetRank(key)
}

func (this *RankList) GetItemByRank(rank int32) (item utils.SkiplistNode) {
	return this.rank_list.GetByRank(rank)
}

func (this *RankList) SetValueByKey(key interface{}, value interface{}) {
	this.rank_list.SetValueByKey(key, value)
}

func (this *RankList) RankNum() int32 {
	return this.rank_list.GetLength()
}

// 获取排名项
func (this *RankList) GetItemsByRange(key interface{}, start_rank, rank_num int32) (rank_items []utils.SkiplistNode, self_rank int32, self_value interface{}) {
	start_rank, rank_num = this.rank_list.GetRankRange(start_rank, rank_num)
	if start_rank == 0 {
		log.Error("Get rank list range with [%v,%v] failed", start_rank, rank_num)
		return nil, 0, nil
	}

	nodes := make([]interface{}, rank_num)
	for i := int32(0); i < rank_num; i++ {
		nodes[i] = this.item_pool.Get().(utils.SkiplistNode)
	}

	num := this.rank_list.GetRangeNodes(start_rank, rank_num, nodes)
	if num == 0 {
		log.Error("Get rank list nodes failed")
		return nil, 0, nil
	}

	rank_items = make([]utils.SkiplistNode, num)
	for i := int32(0); i < num; i++ {
		rank_items[i] = nodes[i].(utils.SkiplistNode)
	}

	self_rank, self_value = this.rank_list.GetRankAndValue(key)
	return

}

// 获取最后的几个排名
func (this *RankList) GetLastRankRange(rank_num int32) (int32, int32) {
	return this.rank_list.GetLastRankRange(rank_num)
}

// 更新排行榜
func (this *RankList) UpdateItem(item utils.SkiplistNode) bool {
	if !this.rank_list.Update(item) {
		log.Error("Update rank item[%v] failed", item)
		return false
	}
	return true
}

// 删除指定值
func (this *RankList) DeleteItem(key interface{}) bool {
	return this.DeleteItem(key)
}

var root_rank_item = []utils.SkiplistNode{
	nil,                   // 0
	&ArenaRankItem{},      // 1
	&CampaignRankItem{},   // 2
	&RolesPowerRankItem{}, // 3
	nil,                   // 4
	nil,                   // 5
	nil,                   // 6
	nil,                   // 7
	nil,                   // 8
	nil,                   // 9
	nil,                   // 10
	nil,                   // 11
	nil,                   // 12
	nil,                   // 13
	nil,                   // 14
	//&TopPowerRankItem{}, // 15
}

type RankListManager struct {
	rank_lists []*RankList
	rank_map   map[int32]*RankList
	locker     *sync.RWMutex
}

var rank_list_mgr RankListManager

func (this *RankListManager) Init() {
	this.rank_lists = make([]*RankList, RANK_LIST_TYPE_MAX)
	for i := int32(1); i < RANK_LIST_TYPE_MAX; i++ {
		if int(i) >= len(root_rank_item) {
			break
		}
		this.rank_lists[i] = &RankList{}
		this.rank_lists[i].Init(root_rank_item[i])
	}
	this.rank_map = make(map[int32]*RankList)
	this.locker = &sync.RWMutex{}
}

func (this *RankListManager) GetRankList(rank_type int32) (rank_list *RankList) {
	if int(rank_type) >= len(this.rank_lists) {
		return nil
	}
	if this.rank_lists[rank_type] == nil {
		return nil
	}
	return this.rank_lists[rank_type]
}

func (this *RankListManager) GetItemByKey(rank_type int32, key interface{}) (item utils.SkiplistNode) {
	if int(rank_type) >= len(this.rank_lists) {
		return nil
	}
	if this.rank_lists[rank_type] == nil {
		return nil
	}
	return this.rank_lists[rank_type].GetItemByKey(key)
}

func (this *RankListManager) GetRankByKey(rank_type int32, key interface{}) int32 {
	if int(rank_type) >= len(this.rank_lists) {
		return -1
	}
	if this.rank_lists[rank_type] == nil {
		return -1
	}
	return this.rank_lists[rank_type].GetRankByKey(key)
}

func (this *RankListManager) GetItemByRank(rank_type, rank int32) (item utils.SkiplistNode) {
	if int(rank_type) >= len(this.rank_lists) {
		return nil
	}
	if this.rank_lists[rank_type] == nil {
		return nil
	}
	return this.rank_lists[rank_type].GetItemByRank(rank)
}

func (this *RankListManager) GetItemsByRange(rank_type int32, key interface{}, start_rank, rank_num int32) (rank_items []utils.SkiplistNode, self_rank int32, self_value interface{}) {
	if int(rank_type) >= len(this.rank_lists) {
		return nil, 0, nil
	}
	if this.rank_lists[rank_type] == nil {
		return nil, 0, nil
	}
	return this.rank_lists[rank_type].GetItemsByRange(key, start_rank, rank_num)
}

func (this *RankListManager) GetLastRankRange(rank_type, rank_num int32) (int32, int32) {
	if int(rank_type) >= len(this.rank_lists) {
		return -1, -1
	}
	if this.rank_lists[rank_type] == nil {
		return -1, -1
	}
	return this.rank_lists[rank_type].GetLastRankRange(rank_num)
}

func (this *RankListManager) UpdateItem(rank_type int32, item utils.SkiplistNode) bool {
	if int(rank_type) >= len(this.rank_lists) {
		return false
	}
	if this.rank_lists[rank_type] == nil {
		return false
	}
	return this.rank_lists[rank_type].UpdateItem(item)
}

func (this *RankListManager) DeleteItem(rank_type int32, key interface{}) bool {
	if int(rank_type) >= len(this.rank_lists) {
		return false
	}
	if this.rank_lists[rank_type] == nil {
		return false
	}
	return this.rank_lists[rank_type].DeleteItem(key)
}

func (this *RankListManager) GetRankList2(rank_type int32) (rank_list *RankList) {
	this.locker.RLock()
	rank_list = this.rank_map[rank_type]
	if rank_list == nil {
		rank_list = &RankList{}
		this.rank_map[rank_type] = rank_list
	}
	this.locker.RUnlock()
	return
}

func (this *RankListManager) GetItemByKey2(rank_type int32, key interface{}) (item utils.SkiplistNode) {
	rank_list := this.GetRankList2(rank_type)
	if rank_list == nil {
		return
	}
	return rank_list.GetItemByKey(key)
}

func (this *RankListManager) GetRankByKey2(rank_type int32, key interface{}) int32 {
	rank_list := this.GetRankList2(rank_type)
	if rank_list == nil {
		return 0
	}
	return rank_list.GetRankByKey(key)
}

func (this *RankListManager) GetItemByRank2(rank_type, rank int32) (item utils.SkiplistNode) {
	rank_list := this.GetRankList2(rank_type)
	if rank_list == nil {
		return
	}
	return rank_list.GetItemByRank(rank)
}

func (this *RankListManager) GetItemsByRange2(rank_type int32, key interface{}, start_rank, rank_num int32) (rank_items []utils.SkiplistNode, self_rank int32, self_value interface{}) {
	rank_list := this.GetRankList2(rank_type)
	if rank_list == nil {
		return
	}
	return rank_list.GetItemsByRange(key, start_rank, rank_num)
}

func (this *RankListManager) GetLastRankRange2(rank_type, rank_num int32) (int32, int32) {
	rank_list := this.GetRankList2(rank_type)
	if rank_list == nil {
		return -1, -1
	}
	return rank_list.GetLastRankRange(rank_num)
}

func (this *RankListManager) UpdateItem2(rank_type int32, item utils.SkiplistNode) bool {
	rank_list := this.GetRankList2(rank_type)
	if rank_list == nil {
		return false
	}
	return rank_list.UpdateItem(item)
}

func (this *RankListManager) DeleteItem2(rank_type int32, key interface{}) bool {
	rank_list := this.GetRankList2(rank_type)
	if rank_list == nil {
		return false
	}
	return rank_list.DeleteItem(key)
}

func transfer_nodes_to_rank_items(rank_type int32, start_rank int32, items []utils.SkiplistNode) (rank_items []*msg_client_message.RankItemInfo) {
	if rank_type == RANK_LIST_TYPE_ARENA {
		for i := int32(0); i < int32(len(items)); i++ {
			item := (items[i]).(*ArenaRankItem)
			if item == nil {
				continue
			}
			name, level, head, score, grade, power := GetFighterInfo(item.PlayerId)
			rank_item := &msg_client_message.RankItemInfo{
				Rank:             start_rank + i,
				PlayerId:         item.PlayerId,
				PlayerName:       name,
				PlayerLevel:      level,
				PlayerHead:       head,
				PlayerArenaScore: score,
				PlayerArenaGrade: grade,
				PlayerPower:      power,
			}
			rank_items = append(rank_items, rank_item)
		}
	} else if rank_type == RANK_LIST_TYPE_CAMPAIGN {
		for i := int32(0); i < int32(len(items)); i++ {
			item := (items[i]).(*CampaignRankItem)
			if item == nil {
				continue
			}
			name, level, head, campaign_id := GetPlayerCampaignInfo(item.PlayerId)
			rank_item := &msg_client_message.RankItemInfo{
				Rank:                   start_rank + i,
				PlayerId:               item.PlayerId,
				PlayerName:             name,
				PlayerLevel:            level,
				PlayerHead:             head,
				PlayerPassedCampaignId: campaign_id,
			}
			rank_items = append(rank_items, rank_item)
		}
	} else if rank_type == RANK_LIST_TYPE_ROLE_POWER {
		for i := int32(0); i < int32(len(items)); i++ {
			item := (items[i]).(*RolesPowerRankItem)
			if item == nil {
				continue
			}
			name, level, head := GetPlayerBaseInfo(item.PlayerId)
			rank_item := &msg_client_message.RankItemInfo{
				Rank:             start_rank + i,
				PlayerId:         item.PlayerId,
				PlayerName:       name,
				PlayerLevel:      level,
				PlayerHead:       head,
				PlayerRolesPower: item.Power,
			}
			rank_items = append(rank_items, rank_item)
		}
	} else {
		log.Error("invalid rank type[%v] transfer nodes to rank items", rank_type)
	}
	return
}

func (this *Player) get_rank_list_items(rank_type, start_rank, num int32) int32 {
	items, self_rank, value := rank_list_mgr.GetItemsByRange(rank_type, this.Id, start_rank, num)
	if items == nil {
		return int32(msg_client_message.E_ERR_RANK_LIST_TYPE_INVALID)
	}

	var self_value, self_value2, self_top_rank int32
	if value != nil {
		self_value = value.(int32)
	}
	if rank_type == RANK_LIST_TYPE_ARENA {
		self_top_rank = this.db.Arena.GetHistoryTopRank()
		self_value = this.db.Arena.GetScore()
	} else if rank_type == RANK_LIST_TYPE_CAMPAIGN {
		self_value = this.db.CampaignCommon.GetLastestPassedCampaignId()
	} else if rank_type == RANK_LIST_TYPE_ROLE_POWER {
	}
	rank_items := transfer_nodes_to_rank_items(rank_type, start_rank, items)
	response := &msg_client_message.S2CRankListResponse{
		RankListType:       rank_type,
		RankItems:          rank_items,
		SelfRank:           self_rank,
		SelfHistoryTopRank: self_top_rank,
		SelfValue:          self_value,
		SelfValue2:         self_value2,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_RANK_LIST_RESPONSE), response)
	log.Debug("Player[%v] get rank type[%v] list response", this.Id, rank_type)
	return 1
}

func C2SRankListHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SRankListRequest
	err := proto.Unmarshal(msg_data, &req)
	if nil != err {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}
	return p.get_rank_list_items(req.GetRankListType(), 1, global_config.ArenaGetTopRankNum)
}
