package main

import (
	"ih_server/libs/log"
	"ih_server/libs/utils"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"ih_server/src/table_config"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
)

//const FRIEND_UNREAD_MESSAGE_MAX_NUM int = 200
//const FRIEND_MESSAGE_MAX_LENGTH int = 200

const MAX_FRIEND_RECOMMEND_PLAYER_NUM int32 = 10000

type FriendRecommendMgr struct {
	player_ids    map[int32]int32
	players_array []int32
	locker        *sync.RWMutex
	add_chan      chan int32
	to_end        int32
}

var friend_recommend_mgr FriendRecommendMgr

func (this *FriendRecommendMgr) Init() {
	this.player_ids = make(map[int32]int32)
	this.players_array = make([]int32, MAX_FRIEND_RECOMMEND_PLAYER_NUM)
	this.locker = &sync.RWMutex{}
	this.add_chan = make(chan int32, 10000)
	this.to_end = 0
}

func (this *FriendRecommendMgr) AddPlayer(player_id int32) {
	this.add_chan <- player_id
	log.Debug("Friend Recommend Manager to add player[%v]", player_id)
}

func (this *FriendRecommendMgr) CheckAndAddPlayer(player_id int32) bool {
	p := player_mgr.GetPlayerById(player_id)
	if p == nil {
		return false
	}

	if _, o := this.player_ids[player_id]; o {
		//log.Warn("Player[%v] already added Friend Recommend mgr", player_id)
		return false
	}

	var add_pos int32
	num := int32(len(this.player_ids))
	if num >= MAX_FRIEND_RECOMMEND_PLAYER_NUM {
		add_pos = rand.Int31n(num)
		// 删掉一个随机位置的
		delete(this.player_ids, this.players_array[add_pos])
		this.players_array[add_pos] = 0
	} else {
		add_pos = num
	}

	now_time := int32(time.Now().Unix())
	if now_time-p.db.Info.GetLastLogout() > 24*3600*2 && !p.is_login {
		return false
	}

	if p.db.Friends.NumAll() >= global_config.FriendMaxNum {
		return false
	}

	this.player_ids[player_id] = add_pos
	this.players_array[add_pos] = player_id

	//log.Debug("Friend Recommend Manager add player[%v], total count[%v], player_ids: %v, players_array: %v", player_id, len(this.player_ids), this.player_ids, this.players_array[:len(this.player_ids)])

	return true
}

func (this *FriendRecommendMgr) Run() {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	var last_check_remove_time int32
	for {
		if atomic.LoadInt32(&this.to_end) > 0 {
			break
		}
		// 处理操作队列
		is_break := false
		for !is_break {
			select {
			case player_id, ok := <-this.add_chan:
				{
					if !ok {
						log.Error("conn timer wheel op chan receive invalid !!!!!")
						return
					}
					this.CheckAndAddPlayer(player_id)
				}
			default:
				{
					is_break = true
				}
			}
		}

		now_time := int32(time.Now().Unix())
		if now_time-last_check_remove_time >= 60*10 {
			this.locker.Lock()
			player_num := len(this.player_ids)
			for i := 0; i < player_num; i++ {
				p := player_mgr.GetPlayerById(this.players_array[i])
				if p == nil {
					continue
				}
				if (now_time-p.db.Info.GetLastLogout() >= 2*24*3600 && !p.is_login) || p.db.Friends.NumAll() >= global_config.FriendMaxNum {
					delete(this.player_ids, this.players_array[i])
					this.players_array[i] = this.players_array[player_num-1]
					player_num -= 1
				}
			}
			this.locker.Unlock()
			last_check_remove_time = now_time
		}

		time.Sleep(time.Second * 1)
	}
}

func (this *FriendRecommendMgr) Random(player_id int32) (ids []int32) {
	player := player_mgr.GetPlayerById(player_id)
	if player == nil {
		return
	}

	this.locker.RLock()
	defer this.locker.RUnlock()

	cnt := int32(len(this.player_ids))
	if cnt == 0 {
		return
	}

	if cnt > global_config.FriendRecommendNum {
		cnt = global_config.FriendRecommendNum
	}

	rand.Seed(time.Now().Unix() + time.Now().UnixNano())
	for i := int32(0); i < cnt; i++ {
		r := rand.Int31n(int32(len(this.player_ids)))
		sr := r
		for {
			has := false
			if this.players_array[sr] == player_id || player.db.Friends.HasIndex(this.players_array[sr]) || player.db.FriendAsks.HasIndex(this.players_array[sr]) {
				has = true
			} else {
				if ids != nil {
					for n := 0; n < len(ids); n++ {
						if ids[n] == this.players_array[sr] {
							has = true
							break
						}
					}
				}
			}
			if !has {
				break
			}
			sr = (sr + 1) % int32(len(this.player_ids))
			if sr == r {
				log.Info("Friend Recommend Mgr player count[%v] not enough to random a player to recommend", len(this.player_ids))
				return
			}
		}
		pid := this.players_array[sr]
		if pid <= 0 {
			break
		}
		ids = append(ids, pid)
	}
	return ids
}

// ----------------------------------------------------------------------------

func (this *Player) _friend_get_give_remain_seconds(friend *Player, now_time int32) (give_remain_seconds int32) {
	last_give_time, _ := this.db.Friends.GetLastGivePointsTime(friend.Id)
	give_remain_seconds = utils.GetRemainSeconds2NextDayTime(last_give_time, global_config.FriendRefreshTime)
	return
}

func (this *Player) _friend_get_points(friend *Player, now_time int32) (get_points int32) {
	last_give_time, _ := friend.db.Friends.GetLastGivePointsTime(this.Id)
	get_points, _ = this.db.Friends.GetGetPoints(friend.Id)
	if utils.GetRemainSeconds2NextDayTime(last_give_time, global_config.FriendRefreshTime) == 0 {
		get_points = 0
	}
	return
}

func (this *Player) _check_friend_data_refresh(friend *Player, now_time int32) (give_remain_seconds, get_points int32) {
	give_remain_seconds = this._friend_get_give_remain_seconds(friend, now_time)
	get_points = this._friend_get_points(friend, now_time)
	return
}

func (this *Player) _format_friend_info(p *Player, now_time int32) (friend_info *msg_client_message.FriendInfo) {
	remain_seconds, get_points := this._check_friend_data_refresh(p, now_time)
	friend_info = &msg_client_message.FriendInfo{
		Id:                      p.Id,
		Name:                    p.db.GetName(),
		Level:                   p.db.Info.GetLvl(),
		Head:                    p.db.Info.GetHead(),
		IsOnline:                p.is_login,
		OfflineSeconds:          p._get_offline_seconds(),
		RemainGivePointsSeconds: remain_seconds,
		BossId:                  p.db.FriendCommon.GetFriendBossTableId(),
		BossHpPercent:           p.get_friend_boss_hp_percent(),
		Power:                   p.get_defense_team_power(),
		GetPoints:               get_points,
	}
	return
}

func (this *Player) _format_friends_info(friend_ids []int32) (friends_info []*msg_client_message.FriendInfo) {
	if friend_ids == nil || len(friend_ids) == 0 {
		friends_info = make([]*msg_client_message.FriendInfo, 0)
	} else {
		now_time := int32(time.Now().Unix())
		for i := 0; i < len(friend_ids); i++ {
			p := player_mgr.GetPlayerById(friend_ids[i])
			if p == nil {
				continue
			}
			player := this._format_friend_info(p, now_time)
			friends_info = append(friends_info, player)
		}
	}
	return
}

func _format_players_info(player_ids []int32) (players_info []*msg_client_message.PlayerInfo) {
	if player_ids == nil || len(player_ids) == 0 {
		players_info = make([]*msg_client_message.PlayerInfo, 0)
	} else {
		for i := 0; i < len(player_ids); i++ {
			p := player_mgr.GetPlayerById(player_ids[i])
			if p == nil {
				continue
			}

			player := &msg_client_message.PlayerInfo{
				Id:    player_ids[i],
				Name:  p.db.GetName(),
				Level: p.db.Info.GetLvl(),
				Head:  p.db.Info.GetHead(),
			}
			players_info = append(players_info, player)
		}
	}
	return
}

// 好友推荐列表
func (this *Player) send_recommend_friends() int32 {
	var player_ids []int32
	last_recommend_time := this.db.FriendCommon.GetLastRecommendTime()
	if last_recommend_time == 0 || utils.CheckDayTimeArrival(last_recommend_time, global_config.FriendRefreshTime) {
		player_ids = friend_recommend_mgr.Random(this.Id)
		if player_ids != nil {
			this.db.FriendRecommends.Clear()
			for i := 0; i < len(player_ids); i++ {
				this.db.FriendRecommends.Add(&dbPlayerFriendRecommendData{
					PlayerId: player_ids[i],
				})
			}
		}
		this.db.FriendCommon.SetLastRecommendTime(int32(time.Now().Unix()))
	} else {
		player_ids = this.db.FriendRecommends.GetAllIndex()
	}
	players := this._format_friends_info(player_ids)
	response := &msg_client_message.S2CFriendRecommendResponse{
		Players: players,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_FRIEND_RECOMMEND_RESPONSE), response)
	log.Debug("Player[%v] recommend friends %v", this.Id, response)
	return 1
}

// 好友列表
func (this *Player) send_friend_list() int32 {
	friend_ids := this.db.Friends.GetAllIndex()
	friends := this._format_friends_info(friend_ids)
	response := &msg_client_message.S2CFriendListResponse{
		Friends: friends,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_FRIEND_LIST_RESPONSE), response)
	log.Debug("Player[%v] friend list: %v", this.Id, response)
	return 1
}

// 检测是否好友增加
func (this *Player) check_and_send_friend_add() int32 {
	this.friend_add_locker.Lock()
	if this.friend_add == nil || len(this.friend_add) == 0 {
		this.friend_add_locker.Unlock()
		return 0
	}
	friends := this._format_friends_info(this.friend_add)
	this.friend_add = nil
	this.friend_add_locker.Unlock()

	response := &msg_client_message.S2CFriendListAddNotify{
		FriendsAdd: friends,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_FRIEND_LIST_ADD_NOTIFY), response)
	log.Debug("Player[%v] friend add: %v", this.Id, response)
	return 1
}

// 申请好友增加
func (this *Player) friend_ask_add_ids(player_ids []int32) {
	this.friend_ask_add_locker.Lock()
	defer this.friend_ask_add_locker.Unlock()
	this.friend_ask_add = append(this.friend_ask_add, player_ids...)
}

// 申请好友
func (this *Player) friend_ask(player_ids []int32) int32 {
	for i := 0; i < len(player_ids); i++ {
		player_id := player_ids[i]
		p := player_mgr.GetPlayerById(player_id)
		if p == nil {
			log.Error("Player[%v] not found", player_id)
			return int32(msg_client_message.E_ERR_PLAYER_NOT_EXIST)
		}

		if this.db.Friends.HasIndex(player_id) {
			log.Error("Player[%v] already add player[%v] to friend", this.Id, player_id)
			return int32(msg_client_message.E_ERR_PLAYER_FRIEND_ALREADY_ADD)
		}

		if p.db.FriendAsks.HasIndex(this.Id) {
			log.Error("Player[%v] already asked player[%v] to friend", this.Id, player_id)
			return int32(msg_client_message.E_ERR_PLAYER_FRIEND_ALREADY_ASKED)
		}
	}

	for i := 0; i < len(player_ids); i++ {
		player_id := player_ids[i]
		p := player_mgr.GetPlayerById(player_id)
		if p == nil {
			continue
		}
		p.db.FriendAsks.Add(&dbPlayerFriendAskData{
			PlayerId: this.Id,
		})
		p.friend_ask_add_ids([]int32{this.Id})
		this.db.FriendRecommends.Remove(p.Id)
	}

	response := &msg_client_message.S2CFriendAskResponse{
		PlayerIds: player_ids,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_FRIEND_ASK_RESPONSE), response)

	log.Debug("Player[%v] asked players[%v] to friend", this.Id, player_ids)

	return 1
}

// 检测好友申请增加
func (this *Player) check_and_send_friend_ask_add() int32 {
	this.friend_ask_add_locker.Lock()
	if this.friend_ask_add == nil || len(this.friend_ask_add) == 0 {
		this.friend_ask_add_locker.Unlock()
		return 0
	}
	players := _format_players_info(this.friend_ask_add)
	this.friend_ask_add = nil
	this.friend_ask_add_locker.Unlock()

	response := &msg_client_message.S2CFriendAskPlayerListAddNotify{
		PlayersAdd: players,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_FRIEND_ASK_PLAYER_LIST_ADD_NOTIFY), response)
	log.Debug("Player[%v] checked friend ask add %v", this.Id, response)
	return 1
}

// 好友申请列表
func (this *Player) send_friend_ask_list() int32 {
	friend_ask_ids := this.db.FriendAsks.GetAllIndex()
	players := _format_players_info(friend_ask_ids)
	response := &msg_client_message.S2CFriendAskPlayerListResponse{
		Players: players,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_FRIEND_ASK_PLAYER_LIST_RESPONSE), response)
	log.Debug("Player[%v] friend ask list %v", this.Id, response)
	return 1
}

// 好友增加
func (this *Player) friend_add_ids(player_ids []int32) {
	this.friend_add_locker.Lock()
	defer this.friend_add_locker.Unlock()
	this.friend_add = append(this.friend_add, player_ids...)
}

// 同意加为好友
func (this *Player) agree_friend_ask(player_ids []int32) int32 {
	for i := 0; i < len(player_ids); i++ {
		p := player_mgr.GetPlayerById(player_ids[i])
		if p == nil {
			log.Error("Player[%v] not found on agree friend ask", player_ids[i])
			return int32(msg_client_message.E_ERR_PLAYER_NOT_EXIST)
		}
		if !this.db.FriendAsks.HasIndex(player_ids[i]) {
			log.Error("Player[%v] friend ask list not player[%v]", this.Id, player_ids[i])
			return int32(msg_client_message.E_ERR_PLAYER_FRIEND_PLAYER_NO_IN_ASK_LIST)
		}
	}

	for i := 0; i < len(player_ids); i++ {
		p := player_mgr.GetPlayerById(player_ids[i])
		if p == nil {
			continue
		}
		p.db.Friends.Add(&dbPlayerFriendData{
			PlayerId: this.Id,
		})
		p.friend_add_ids([]int32{this.Id})
		this.db.Friends.Add(&dbPlayerFriendData{
			PlayerId: player_ids[i],
		})
		this.db.FriendAsks.Remove(player_ids[i])
		this.db.FriendRecommends.Remove(player_ids[i])
	}

	response := &msg_client_message.S2CFriendAgreeResponse{
		Friends: this._format_friends_info(player_ids),
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_FRIEND_AGREE_RESPONSE), response)

	this.friend_add_ids(player_ids)
	this.check_and_send_friend_add()

	log.Debug("Player[%v] agreed players[%v] friend ask", this.Id, player_ids)
	return 1
}

// 拒绝好友申请
func (this *Player) refuse_friend_ask(player_ids []int32) int32 {
	if player_ids == nil {
		return 0
	}

	for _, player_id := range player_ids {
		if !this.db.FriendAsks.HasIndex(player_id) {
			log.Error("Player[%v] ask list no player[%v]", this.Id, player_id)
			return int32(msg_client_message.E_ERR_PLAYER_FRIEND_PLAYER_NO_IN_ASK_LIST)
		}
	}

	for _, player_id := range player_ids {
		this.db.FriendAsks.Remove(player_id)
	}

	response := &msg_client_message.S2CFriendRefuseResponse{
		PlayerIds: player_ids,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_FRIEND_REFUSE_RESPONSE), response)

	log.Debug("Player[%v] refuse players %v friend ask", this.Id, player_ids)

	return 1
}

// 删除好友
func (this *Player) remove_friend(friend_ids []int32) int32 {
	for i := 0; i < len(friend_ids); i++ {
		if !this.db.Friends.HasIndex(friend_ids[i]) {
			log.Error("Player[%v] no friend[%v]", this.Id, friend_ids[i])
			return int32(msg_client_message.E_ERR_PLAYER_FRIEND_NOT_FOUND)
		}
	}

	for i := 0; i < len(friend_ids); i++ {
		this.db.Friends.Remove(friend_ids[i])
		friend := player_mgr.GetPlayerById(friend_ids[i])
		if friend != nil {
			friend.db.Friends.Remove(this.Id)
		}
	}

	response := &msg_client_message.S2CFriendRemoveResponse{
		PlayerIds: friend_ids,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_FRIEND_REMOVE_RESPONSE), response)

	log.Debug("Player[%v] removed friends: %v", this.Id, friend_ids)

	return 1
}

// 赠送友情点
func (this *Player) give_friends_points(friend_ids []int32) int32 {
	for i := 0; i < len(friend_ids); i++ {
		if !this.db.Friends.HasIndex(friend_ids[i]) {
			log.Error("Player[%v] no friend[%v]", this.Id, friend_ids[i])
			return int32(msg_client_message.E_ERR_PLAYER_FRIEND_NOT_FOUND)
		}
	}

	is_gived := make([]bool, len(friend_ids))
	now_time := int32(time.Now().Unix())
	for i := 0; i < len(friend_ids); i++ {
		friend := player_mgr.GetPlayerById(friend_ids[i])
		if friend == nil {
			continue
		}
		//last_give_time, _ := this.db.Friends.GetLastGivePointsTime(friend_ids[i])
		//if utils.CheckDayTimeArrival(last_give_time, global_config.FriendRefreshTime) {
		remain_seconds := this._friend_get_give_remain_seconds(friend, now_time)
		if remain_seconds == 0 {
			this.db.Friends.SetLastGivePointsTime(friend_ids[i], now_time)
			friend.db.Friends.SetGetPoints(this.Id, global_config.FriendPointsOnceGive)
			is_gived[i] = true
			// 更新任务
			this.TaskUpdate(table_config.TASK_COMPLETE_TYPE_GIVE_POINTS_NUM, false, 0, 1)
		}
	}

	response := &msg_client_message.S2CFriendGivePointsResponse{
		FriendIds:    friend_ids,
		IsGivePoints: is_gived,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_FRIEND_GIVE_POINTS_RESPONSE), response)

	log.Debug("Player[%v] give friends %v points, is gived %v", this.Id, friend_ids, is_gived)

	return 1
}

// 收取友情点
func (this *Player) get_friend_points(friend_ids []int32) int32 {
	for i := 0; i < len(friend_ids); i++ {
		if !this.db.Friends.HasIndex(friend_ids[i]) {
			log.Error("Player[%v] no friend[%v]", this.Id, friend_ids[i])
			return int32(msg_client_message.E_ERR_PLAYER_FRIEND_NOT_FOUND)
		}
	}

	get_points := make([]int32, len(friend_ids))
	for i := 0; i < len(friend_ids); i++ {
		friend := player_mgr.GetPlayerById(friend_ids[i])
		if friend == nil {
			continue
		}
		//last_give_time, _ := friend.db.Friends.GetLastGivePointsTime(this.Id)
		//if utils.GetRemainSeconds2NextDayTime(last_give_time, global_config.FriendRefreshTime) > 0 {
		get_point := this._friend_get_points(friend, int32(time.Now().Unix()))
		if get_point > 0 {
			this.add_resource(global_config.FriendPointItemId, get_point)
			this.db.Friends.SetGetPoints(friend_ids[i], -1)
			get_points[i] = get_point
		}
		//}
	}

	this.check_and_send_items_change()

	response := &msg_client_message.S2CFriendGetPointsResponse{
		FriendIds: friend_ids,
		GetPoints: get_points,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_FRIEND_GET_POINTS_RESPONSE), response)

	log.Debug("Player[%v] get friends %v points %v", this.Id, friend_ids, get_points)

	return 1
}

// 搜索BOSS
func (this *Player) friend_search_boss() int32 {
	now_time := int32(time.Now().Unix())
	last_refresh_time := this.db.FriendCommon.GetLastBossRefreshTime()
	if last_refresh_time > 0 && now_time-last_refresh_time < global_config.FriendSearchBossRefreshHours*3600 {
		log.Error("Player[%v] friend boss search is cool down", this.Id)
		return int32(msg_client_message.E_ERR_PLAYER_FRIEND_BOSS_REFRESH_IS_COOLDOWN)
	}

	friend_boss_tdata := friend_boss_table_mgr.GetWithLevel(this.db.Info.GetLvl())
	if friend_boss_tdata == nil {
		log.Error("Player[%v] cant searched friend boss with level %v", this.Id, this.db.Info.GetLvl())
		return int32(msg_client_message.E_ERR_PLAYER_FRIEND_BOSS_DATA_NOT_FOUND)
	}

	var boss_id int32
	var items []*msg_client_message.ItemInfo
	r := rand.Int31n(10000)
	if r >= friend_boss_tdata.SearchBossChance {
		// 掉落
		o, item := this.drop_item_by_id(friend_boss_tdata.SearchItemDropID, true, nil)
		if !o {
			log.Error("Player[%v] search friend boss to drop item with id %v failed", this.Id, friend_boss_tdata.SearchItemDropID)
			return -1
		}
		items = []*msg_client_message.ItemInfo{item}
	} else {
		stage_id := friend_boss_tdata.BossStageID
		stage := stage_table_mgr.Get(stage_id)
		if stage == nil {
			log.Error("Stage[%v] table data not found in friend boss", stage_id)
			return -1
		}
		if stage.Monsters == nil {
			log.Error("Stage[%v] monster list is empty", stage_id)
			return -1
		}

		this.db.FriendCommon.SetFriendBossTableId(friend_boss_tdata.Id)
		this.db.FriendBosss.Clear()
		for i := 0; i < len(stage.Monsters); i++ {
			this.db.FriendBosss.Add(&dbPlayerFriendBossData{
				MonsterPos: stage.Monsters[i].Slot - 1,
				MonsterId:  stage.Monsters[i].MonsterID,
			})
		}
		boss_id = friend_boss_tdata.Id
	}

	this.db.FriendCommon.SetLastBossRefreshTime(now_time)
	this.db.FriendCommon.SetAttackBossPlayerList(nil)

	response := &msg_client_message.S2CFriendSearchBossResponse{
		FriendBossTableId: boss_id,
		Items:             items,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_FRIEND_SEARCH_BOSS_RESPONSE), response)

	if boss_id > 0 {
		log.Debug("Player[%v] search friend boss %v", this.Id, boss_id)
	} else {
		log.Debug("Player[%v] search friend boss get items %v", this.Id, items)
	}

	return 1
}

// 获得好友BOSS列表
func (this *Player) get_friends_boss_list() int32 {
	friend_ids := this.db.Friends.GetAllIndex()
	if friend_ids == nil || len(friend_ids) == 0 {
		log.Error("Player[%v] no friends", this.Id)
		return int32(msg_client_message.E_ERR_PLAYER_FRIEND_NONE)
	}

	now_time := int32(time.Now().Unix())
	level := this.db.Info.GetLvl()
	// 加上自己的
	friend_ids = append(friend_ids, this.Id)
	var friend_boss_list []*msg_client_message.FriendBossInfo
	for i := 0; i < len(friend_ids); i++ {
		p := player_mgr.GetPlayerById(friend_ids[i])
		if p == nil {
			continue
		}
		last_refresh_time := p.db.FriendCommon.GetLastBossRefreshTime()
		if now_time-last_refresh_time >= global_config.FriendSearchBossRefreshHours*3600 {
			continue
		}
		friend_boss_table_id := p.db.FriendCommon.GetFriendBossTableId()
		if friend_boss_table_id == 0 {
			continue
		}
		friend_boss_tdata := friend_boss_table_mgr.Get(friend_boss_table_id)
		if friend_boss_tdata == nil {
			log.Error("Player[%v] stored friend boss table id[%v] not found", friend_ids[i], friend_boss_table_id)
			continue
		}

		if friend_boss_tdata.LevelMin > level || friend_boss_tdata.LevelMax < level {
			continue
		}

		hp_percent := p.get_friend_boss_hp_percent()
		friend_boss_info := &msg_client_message.FriendBossInfo{
			FriendId:            friend_ids[i],
			FriendBossTableId:   friend_boss_table_id,
			FriendBossHpPercent: hp_percent,
		}
		friend_boss_list = append(friend_boss_list, friend_boss_info)
	}

	response := &msg_client_message.S2CFriendsBossListResponse{
		BossList: friend_boss_list,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_FRIENDS_BOSS_LIST_RESPONSE), response)

	log.Debug("Player[%v] get friend boss list %v", this.Id, response)

	return 1
}

func (this *Player) can_friend_boss_to_fight() bool {
	return atomic.CompareAndSwapInt32(&this.fighing_friend_boss, 0, 1)
}

func (this *Player) cancel_friend_boss_fight() bool {
	return atomic.CompareAndSwapInt32(&this.fighing_friend_boss, 1, 0)
}

// 战斗随机奖励
func (this *Player) battle_random_reward_notify(reward_items map[int32]int32, drop_id int32) {
	if reward_items != nil {
		var items []*msg_client_message.ItemInfo
		for k, v := range reward_items {
			items = append(items, &msg_client_message.ItemInfo{
				ItemCfgId: k,
				ItemNum:   v,
			})
		}
		var fake_items []int32
		if this.sweep_num == 0 {
			for i := 0; i < 2; i++ {
				o, item := this.drop_item_by_id(drop_id, false, nil)
				if !o {
					log.Error("Player[%v] drop id %v invalid on friend boss attack", this.Id, drop_id)
				}
				fake_items = append(fake_items, item.ItemCfgId)
			}
		}
		if items != nil {
			notify := &msg_client_message.S2CBattleRandomRewardNotify{
				Items:     items,
				FakeItems: fake_items,
			}
			this.Send(uint16(msg_client_message_id.MSGID_S2C_BATTLE_RANDOM_REWARD_NOTIFY), notify)
			log.Debug("Player[%v] battle random reward %v", this.Id, notify)
		}
	}
}

// 挑战好友BOSS
func (this *Player) friend_boss_challenge(friend_id int32) int32 {
	if friend_id == 0 {
		friend_id = this.Id
	}

	p := player_mgr.GetPlayerById(friend_id)
	if p == nil {
		log.Error("Player[%v] not found", friend_id)
		return int32(msg_client_message.E_ERR_PLAYER_NOT_EXIST)
	}

	if friend_id == this.Id {
		p = this
	}

	last_refresh_time := p.db.FriendCommon.GetLastBossRefreshTime()
	if last_refresh_time == 0 {
		log.Error("Player[%v] fight friend[%v] boss not found")
		return int32(msg_client_message.E_ERR_PLAYER_FRIEND_BOSS_NOT_FOUND)
	}

	// 是否正在挑战好友BOSS
	if !p.can_friend_boss_to_fight() {
		log.Warn("Player[%v] friend boss is fighting", p.Id)
		return int32(msg_client_message.E_ERR_PLAYER_FRIEND_BOSS_IS_FIGHTING)
	}

	// 获取好友BOSS
	friend_boss_table_id := p.db.FriendCommon.GetFriendBossTableId()
	if friend_boss_table_id == 0 {
		p.cancel_friend_boss_fight()
		log.Error("Player[%v] fight friend[%v] boss is finished or not refreshed", this.Id, friend_id)
		return int32(msg_client_message.E_ERR_PLAYER_FRIEND_BOSS_NOT_FOUND)
	}

	friend_boss_tdata := friend_boss_table_mgr.Get(friend_boss_table_id)
	if friend_boss_tdata == nil {
		p.cancel_friend_boss_fight()
		log.Error("Player[%v] stored friend boss table id %v not found", p.Id, friend_boss_table_id)
		return int32(msg_client_message.E_ERR_PLAYER_FRIEND_BOSS_DATA_NOT_FOUND)
	}

	stage := stage_table_mgr.Get(friend_boss_tdata.BossStageID)
	if stage == nil {
		p.cancel_friend_boss_fight()
		log.Error("Friend Boss Stage %v not found")
		return int32(msg_client_message.E_ERR_PLAYER_STAGE_TABLE_DATA_NOT_FOUND)
	}

	// 体力
	var need_stamina int32
	if p.sweep_num == 0 {
		need_stamina = global_config.FriendBossAttackCostStamina
	} else {
		need_stamina = global_config.FriendBossAttackCostStamina * p.sweep_num
	}
	if this.get_resource(global_config.FriendStaminaItemId) < need_stamina {
		p.cancel_friend_boss_fight()
		log.Error("Player[%v] friend stamina item not enough", this.Id)
		return int32(msg_client_message.E_ERR_PLAYER_FRIEND_BOSS_STAMINA_NOT_ENOUGH)
	}

	var fight_num int32
	if p.sweep_num == 0 {
		fight_num = 1
	} else {
		fight_num = p.sweep_num
	}

	var err, n int32
	var is_win, has_next_wave bool
	var my_team, target_team []*msg_client_message.BattleMemberItem
	var enter_reports []*msg_client_message.BattleReportItem
	var rounds []*msg_client_message.BattleRoundReports
	var reward_items map[int32]int32
	for ; n < fight_num; n++ {
		err, is_win, my_team, target_team, enter_reports, rounds, has_next_wave = this.FightInStage(5, stage, p, nil)
		if err < 0 {
			p.cancel_friend_boss_fight()
			log.Error("Player[%v] fight friend %v boss %v failed, team is empty", this.Id, friend_id, friend_boss_table_id)
			return err
		}

		// 助战玩家列表
		attack_list := p.db.FriendCommon.GetAttackBossPlayerList()
		if attack_list == nil {
			attack_list = []int32{this.Id}
			p.db.FriendCommon.SetAttackBossPlayerList(attack_list)
		} else {
			has := false
			for i := 0; i < len(attack_list); i++ {
				if attack_list[i] == this.Id {
					has = true
					break
				}
			}
			if !has {
				attack_list = append(attack_list, this.Id)
				p.db.FriendCommon.SetAttackBossPlayerList(attack_list)
			}
		}

		if is_win {
			p.db.FriendCommon.SetFriendBossTableId(0)
			p.db.FriendCommon.SetFriendBossHpPercent(0)
			if p.sweep_num > 0 {
				n += 1
				break
			}
		} else {
			o, item := this.drop_item_by_id(friend_boss_tdata.ChallengeDropID, true, nil)
			if !o {
				log.Error("Player[%v] drop id %v invalid on friend boss attack", this.Id, friend_boss_tdata.ChallengeDropID)
			}
			if reward_items == nil {
				reward_items = make(map[int32]int32)
			}
			reward_items[item.ItemCfgId] += item.ItemNum
		}
	}

	// 退出挑战
	p.cancel_friend_boss_fight()

	// 实际消耗体力
	this.add_resource(global_config.FriendStaminaItemId, -n*global_config.FriendBossAttackCostStamina)

	member_damages := this.friend_boss_team.common_data.members_damage
	member_cures := this.friend_boss_team.common_data.members_cure
	response := &msg_client_message.S2CBattleResultResponse{
		IsWin:               is_win,
		EnterReports:        enter_reports,
		Rounds:              rounds,
		MyTeam:              my_team,
		TargetTeam:          target_team,
		MyMemberDamages:     member_damages[this.friend_boss_team.side],
		TargetMemberDamages: member_damages[this.target_stage_team.side],
		MyMemberCures:       member_cures[this.friend_boss_team.side],
		TargetMemberCures:   member_cures[this.target_stage_team.side],
		HasNextWave:         has_next_wave,
		BattleType:          5,
		BattleParam:         friend_id,
		SweepNum:            p.sweep_num,
		ExtraValue:          p.get_friend_boss_hp_percent(),
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_BATTLE_RESULT_RESPONSE), response)

	// 最后一击
	if is_win {
		this.send_stage_reward(stage, 5)
		SendMail2(nil, this.Id, MAIL_TYPE_SYSTEM, "Friend Boss Last Hit Reward", "Friend Boss Last Hit Reward", friend_boss_tdata.RewardLastHit)
		SendMail2(nil, friend_id, MAIL_TYPE_SYSTEM, "Friend Boss Reward Owner", "Friend Boss Reward Owner", friend_boss_tdata.RewardOwner)
	} else {
		this.battle_random_reward_notify(reward_items, friend_boss_tdata.ChallengeDropID)
	}

	Output_S2CBattleResult(this, response)

	return 1
}

// 获取好友BOSS助战列表
func (this *Player) friend_boss_get_attack_list(friend_id int32) int32 {
	friend := player_mgr.GetPlayerById(friend_id)
	if friend == nil {
		log.Error("Player[%v] not found", friend_id)
		return int32(msg_client_message.E_ERR_PLAYER_NOT_EXIST)
	}

	if friend.db.FriendCommon.GetFriendBossTableId() <= 0 {
		log.Error("Player[%v] friend boss is finished")
		return int32(msg_client_message.E_ERR_PLAYER_FRIEND_BOSS_IS_FINISHED)
	}

	var player_list []*msg_client_message.PlayerInfo
	attack_list := friend.db.FriendCommon.GetAttackBossPlayerList()
	if attack_list == nil || len(attack_list) == 0 {
		player_list = make([]*msg_client_message.PlayerInfo, 0)
	} else {
		for i := 0; i < len(attack_list); i++ {
			attacker := player_mgr.GetPlayerById(attack_list[i])
			if attacker == nil {
				continue
			}
			player_info := &msg_client_message.PlayerInfo{
				Id:    attack_list[i],
				Name:  attacker.db.GetName(),
				Level: attacker.db.Info.GetLvl(),
				Head:  attacker.db.Info.GetHead(),
			}
			player_list = append(player_list, player_info)
		}
	}

	response := &msg_client_message.S2CFriendBossAttackListResponse{
		AttackList: player_list,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_FRIEND_BOSS_ATTACK_LIST_RESPONSE), response)

	log.Debug("Player[%v] get friend[%v] boss attack list: %v", this.Id, friend_id, response)

	return 1
}

// 检测好友体力数据
func (this *Player) check_and_add_friend_stamina() (add_stamina int32, remain_seconds int32) {
	now_time := int32(time.Now().Unix())
	last_get_stamina_time := this.db.FriendCommon.GetLastGetStaminaTime()
	if last_get_stamina_time == 0 {
		this.add_resource(global_config.FriendStaminaItemId, global_config.FriendStartStamina)
		add_stamina = global_config.FriendStartStamina
		remain_seconds = global_config.FriendStaminaResumeOnePointNeedHours * 3600
	} else {
		cost_seconds := now_time - last_get_stamina_time
		y := cost_seconds % (global_config.FriendStaminaResumeOnePointNeedHours * 3600)
		add_stamina = (cost_seconds - y) / (global_config.FriendStaminaResumeOnePointNeedHours * 3600)
		if add_stamina > 0 {
			this.add_resource(global_config.FriendStaminaItemId, add_stamina)
		}
		now_time -= y
		remain_seconds = global_config.FriendStaminaResumeOnePointNeedHours*3600 - y
	}

	this.db.FriendCommon.SetLastGetStaminaTime(now_time)
	return
}

// 助战友情点
func (this *Player) get_assist_points() int32 {
	total_points := this.db.ActiveStageCommon.GetGetPointsDay()
	withdraw_points := this.db.ActiveStageCommon.GetWithdrawPoints()
	get_points := total_points - withdraw_points
	if get_points < 0 {
		get_points = 0
	}
	log.Debug("Player[%v] assist points %v", this.Id, get_points)
	return get_points
}

func (this *Player) get_friend_boss_hp_percent() int32 {
	if this.db.FriendCommon.GetFriendBossTableId() <= 0 {
		return 0
	}
	hp_percent := this.db.FriendCommon.GetFriendBossHpPercent()
	if hp_percent == 0 {
		hp_percent = 100
	}
	return hp_percent
}

// 获取好友相关数据
func (this *Player) friend_data(send bool) int32 {
	add_stamina, remain_seconds := this.check_and_add_friend_stamina()
	if send {
		last_refresh_boss_time := this.db.FriendCommon.GetLastBossRefreshTime()
		now_time := int32(time.Now().Unix())
		remain_seconds = global_config.FriendSearchBossRefreshHours*3600 - (now_time - last_refresh_boss_time)
		if remain_seconds < 0 {
			remain_seconds = 0
		}
		response := &msg_client_message.S2CFriendDataResponse{
			StaminaItemId:            global_config.FriendStaminaItemId,
			AddStamina:               add_stamina,
			RemainSecondsNextStamina: remain_seconds,
			StaminaLimit:             global_config.FriendStaminaLimit,
			StaminaResumeOneCostTime: global_config.FriendStaminaResumeOnePointNeedHours,
			BossId:                  this.db.FriendCommon.GetFriendBossTableId(),
			BossHpPercent:           this.get_friend_boss_hp_percent(),
			AssistGetPoints:         this.get_assist_points(),
			SearchBossRemainSeconds: remain_seconds,
			AssistRoleId:            this.db.FriendCommon.GetAssistRoleId(),
		}
		this.Send(uint16(msg_client_message_id.MSGID_S2C_FRIEND_DATA_RESPONSE), response)

		log.Debug("Player[%v] friend data %v", this.Id, response)
	}

	return 1
}

// 设置助战角色
func (this *Player) friend_set_assist_role(role_id int32) int32 {
	if !this.db.Roles.HasIndex(role_id) {
		log.Error("Player[%v] not have role %v", this.Id, role_id)
		return int32(msg_client_message.E_ERR_PLAYER_ROLE_NOT_FOUND)
	}

	_, o := this.db.Roles.GetIsLock(role_id)
	if !o {
		log.Error("Player[%v] role[%v] is locked", this.Id, role_id)
		return int32(msg_client_message.E_ERR_PLAYER_ROLE_IS_LOCKED)
	}

	old_assist_role := this.db.FriendCommon.GetAssistRoleId()
	if old_assist_role > 0 {
		//this.db.Roles.SetIsLock(old_assist_role, 0)
	}
	this.db.FriendCommon.SetAssistRoleId(role_id)
	//this.db.Roles.SetIsLock(role_id, 1)

	response := &msg_client_message.S2CFriendSetAssistRoleResponse{
		RoleId: role_id,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_FRIEND_SET_ASSIST_ROLE_RESPONSE), response)

	log.Debug("Player[%v] set assist role %v for friends", this.Id, role_id)

	return 1
}

// 提现助战友情点
func (this *Player) active_stage_withdraw_assist_points() int32 {
	get_points := this.get_assist_points()
	if get_points > 0 {
		this.db.ActiveStageCommon.IncbyWithdrawPoints(get_points)
	}
	response := &msg_client_message.S2CFriendGetAssistPointsResponse{
		GetPoints: get_points,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_FRIEND_GET_ASSIST_POINTS_RESPONSE), response)
	return 1
}

// ------------------------------------------------------

func C2SFriendsRecommendHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SFriendRecommendRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.send_recommend_friends()
}

func C2SFriendListHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SFriendListRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	p.friend_data(true)
	return p.send_friend_list()
}

func C2SFriendAskListHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SFriendAskPlayerListRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.send_friend_ask_list()
}

func C2SFriendAskHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SFriendAskRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.friend_ask(req.GetPlayerIds())
}

func C2SFriendAgreeHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SFriendAgreeRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.agree_friend_ask(req.GetPlayerIds())
}

func C2SFriendRefuseHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SFriendRefuseRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.refuse_friend_ask(req.GetPlayerIds())
}

func C2SFriendRemoveHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SFriendRemoveRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.remove_friend(req.GetPlayerIds())
}

func C2SFriendGivePointsHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SFriendGivePointsRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.give_friends_points(req.GetFriendIds())
}

func C2SFriendGetPointsHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SFriendGetPointsRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.get_friend_points(req.GetFriendIds())
}

func C2SFriendSearchBossHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SFriendSearchBossRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.friend_search_boss()
}

func C2SFriendGetBossListHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SFriendsBossListRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.get_friends_boss_list()
}

func C2SFriendBossAttackListHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SFriendBossAttackListRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.friend_boss_get_attack_list(req.GetFriendId())
}

func C2SFriendDataHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SFriendDataRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.friend_data(true)
}

func C2SFriendSetAssistRoleHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SFriendSetAssistRoleRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.friend_set_assist_role(req.GetRoleId())
}

func C2SFriendGiveAndGetPointsHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SFriendGiveAndGetPointsRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}

	res := p.give_friends_points(req.GetFriendIds())
	if res < 0 {
		return res
	}
	return p.get_friend_points(req.GetFriendIds())
}

func C2SFriendGetAssistPointsHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SFriendGetAssistPointsRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)", err.Error())
		return -1
	}
	return p.active_stage_withdraw_assist_points()
}
