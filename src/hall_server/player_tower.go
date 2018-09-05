package main

import (
	"ih_server/libs/log"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"ih_server/src/table_config"
	"net/http"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
)

func get_tower_fight_id(tower_id, i int32) int32 {
	return tower_id*10 + i
}

func (this *Player) send_tower_data(check bool) int32 {
	var tower_keys, remain_seconds int32
	if check {
		_, tower_keys, remain_seconds = this.check_tower_keys()
	}
	response := &msg_client_message.S2CTowerDataResponse{
		CurrTowerId:   this.db.TowerCommon.GetCurrId(),
		TowerKeys:     tower_keys,
		RemainSeconds: remain_seconds,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_TOWER_DATA_RESPONSE), response)
	log.Debug("Player[%v] tower data %v", this.Id, response)
	return 1
}

func (this *Player) check_tower_keys() (is_update bool, keys int32, next_remain_seconds int32) {
	tower_key_max := global_config.TowerKeyMax
	tower_key_get_interval := global_config.TowerKeyGetInterval
	//keys = this.db.TowerCommon.GetKeys()
	keys = this.get_resource(global_config.TowerKeyId)
	if keys < tower_key_max {
		now_time := int32(time.Now().Unix())
		last_time := this.db.TowerCommon.GetLastGetNewKeyTime()
		if last_time == 0 {
			keys = global_config.TowerKeyMax
			last_time = now_time
			this.db.TowerCommon.SetLastGetNewKeyTime(now_time)
			next_remain_seconds = global_config.TowerKeyGetInterval
		} else {
			keys_num := (now_time - last_time) / tower_key_get_interval
			y := (now_time - last_time) % tower_key_get_interval
			if keys_num > 0 {
				keys += keys_num
				if keys > tower_key_max {
					keys = tower_key_max
				}
			}
			if keys < tower_key_max {
				next_remain_seconds = global_config.TowerKeyGetInterval - y
			}
			this.db.TowerCommon.SetLastGetNewKeyTime(now_time - y)
		}
	} else if keys > tower_key_max {
		keys = tower_key_max
	}
	this.set_resource(global_config.TowerKeyId, keys)
	is_update = true
	return
}

func (this *Player) check_and_send_tower_data() {
	//is_update, _, _ := this.check_tower_keys()
	//if is_update {
	this.send_tower_data(true)
	//}
}

func (this *Player) fight_tower(tower_id int32) int32 {
	// 是否时当前层的下一层
	var tower *table_config.XmlTowerItem
	curr_id := this.db.TowerCommon.GetCurrId()
	if curr_id == 0 {
		tower = tower_table_mgr.Get(tower_id)
	} else {
		if curr_id >= tower_id {
			log.Error("Player[%v] already fight tower[%v]", this.Id, tower_id)
			return int32(msg_client_message.E_ERR_PLAYER_TOWER_ALREADY_FIGHTED)
		}
		curr := tower_table_mgr.Get(curr_id)
		if curr == nil {
			log.Error("Tower[%v] data not found", curr_id)
			return int32(msg_client_message.E_ERR_PLAYER_TOWER_NOT_FOUND)
		}
		if curr.Next == nil {
			log.Error("Tower[%v] no next", curr_id)
			return int32(msg_client_message.E_ERR_PLAYER_TOWER_ALREADY_HIGHEST)
		}
		if curr.Next.Id != tower_id {
			log.Error("Cant fight tower[%v]", tower_id)
			return int32(msg_client_message.E_ERR_PLAYER_TOWER_CANT_FIGHT)
		}
		tower = curr.Next
	}
	/////////////////////////

	stage_id := tower.StageId
	stage := stage_table_mgr.Get(stage_id)
	if stage == nil {
		log.Error("Tower[%v] stage[%v] not found", tower_id, stage_id)
		return int32(msg_client_message.E_ERR_PLAYER_TOWER_NOT_FOUND)
	}
	_, keys, _ := this.check_tower_keys()
	if keys <= 0 {
		log.Error("Player[%v] fight tower not enough key", this.Id)
		return int32(msg_client_message.E_ERR_PLAYER_TOWER_NOT_ENOUGH_STAMINA)
	}

	err, is_win, my_team, target_team, enter_reports, rounds, _ := this.FightInStage(3, stage, nil, nil)
	if err < 0 {
		log.Error("Player[%v] fight tower %v failed, team is empty", this.Id, tower_id)
		return err
	}

	//this.db.TowerCommon.SetKeys(keys - 1)
	this.add_resource(global_config.TowerKeyId, -1)
	tower_key_max := global_config.TowerKeyMax
	if keys >= tower_key_max {
		this.db.TowerCommon.SetLastGetNewKeyTime(int32(time.Now().Unix()))
	}
	member_damages := this.tower_team.common_data.members_damage
	member_cures := this.tower_team.common_data.members_cure
	response := &msg_client_message.S2CBattleResultResponse{
		IsWin:               is_win,
		MyTeam:              my_team,
		TargetTeam:          target_team,
		EnterReports:        enter_reports,
		Rounds:              rounds,
		MyMemberDamages:     member_damages[this.tower_team.side],
		TargetMemberDamages: member_damages[this.target_stage_team.side],
		MyMemberCures:       member_cures[this.tower_team.side],
		TargetMemberCures:   member_cures[this.target_stage_team.side],
		BattleType:          3,
		BattleParam:         tower_id,
	}
	data := this.Send(uint16(msg_client_message_id.MSGID_S2C_BATTLE_RESULT_RESPONSE), response)

	if is_win {
		this.db.TowerCommon.SetCurrId(tower_id)
		// 名次
		tower_ranking_list.Update(this.Id, tower_id)
		// 奖励
		this.send_stage_reward(stage.RewardList, 3)
		// 录像
		for i := int32(1); i <= 3; i++ {
			tower_fight_id := get_tower_fight_id(tower_id, i)
			row := dbc.TowerFightSaves.GetRow(tower_fight_id)
			if row == nil {
				row = dbc.TowerFightSaves.AddRow(tower_fight_id)
				data = compress_battle_record_data(data)
				if data != nil {
					row.Data.SetData(data)
					row.SetAttacker(this.Id)
					row.SetTowerId(tower_id)
					row.SetSaveTime(int32(time.Now().Unix()))
				}
				break
			}
		}
	}

	this.send_tower_data(true)

	Output_S2CBattleResult(this, response)

	return 1
}

func (this *Player) get_tower_records_info(tower_id int32) int32 {
	var records []*msg_client_message.TowerFightRecord
	for i := int32(1); i <= 3; i++ {
		tower_fight_id := get_tower_fight_id(tower_id, i)
		row := dbc.TowerFightSaves.GetRow(tower_fight_id)
		if row == nil {
			continue
		}
		attacker_id := row.GetAttacker()
		attacker := player_mgr.GetPlayerById(attacker_id)
		if attacker == nil {
			continue
		}
		records = append(records, &msg_client_message.TowerFightRecord{
			TowerFightId:  tower_fight_id,
			AttackerId:    attacker_id,
			AttackerName:  attacker.db.GetName(),
			CreateTime:    row.GetSaveTime(),
			AttackerHead:  attacker.db.Info.GetHead(),
			AttackerLevel: attacker.db.Info.GetLvl(),
		})
	}
	response := &msg_client_message.S2CTowerRecordsInfoResponse{
		Records: records,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_TOWER_RECORDS_INFO_RESPONSE), response)
	return 1
}

func (this *Player) get_tower_record_data(tower_fight_id int32) int32 {
	row := dbc.TowerFightSaves.GetRow(tower_fight_id)
	if row == nil {
		log.Error("Tower fight record[%v] not found", tower_fight_id)
		return int32(msg_client_message.E_ERR_PLAYER_TOWER_FIGHT_RECORD_NOT_FOUND)
	}

	record_data := row.Data.GetData()
	record_data = decompress_battle_record_data(record_data)
	if record_data == nil {
		return int32(msg_client_message.E_ERR_PLAYER_BATTLE_RECORD_DATA_INVALID)
	}
	response := &msg_client_message.S2CTowerRecordDataResponse{
		RecordData: record_data,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_TOWER_RECORD_DATA_RESPONSE), response)

	return 1
}

const (
	TOWER_RANKING_LIST_MAX = 50
)

type TowerRankingList struct {
	player_list []int32
	player_num  int32
	player_map  map[int32]int32
	locker      *sync.RWMutex
}

var tower_ranking_list TowerRankingList

func (this *TowerRankingList) LoadDB() {
	ids := dbc.TowerRankingList.GetRow().Players.GetIds()
	if ids == nil {
		return
	}

	this.player_list = make([]int32, TOWER_RANKING_LIST_MAX)
	this.player_map = make(map[int32]int32)
	this.locker = &sync.RWMutex{}

	for i := 0; i < len(ids); i++ {
		p := player_mgr.GetPlayerById(ids[i])
		if p == nil {
			continue
		}
		this.update(ids[i], p.db.TowerCommon.GetCurrId())
	}
}

func (this *TowerRankingList) Update(player_id, tower_id int32) bool {
	this.locker.Lock()
	defer this.locker.Unlock()

	if !this.update(player_id, tower_id) {
		return false
	}
	dbc.TowerRankingList.GetRow().Players.SetIds(this.player_list[:this.player_num])
	return true
}

func (this *TowerRankingList) update(player_id, tower_id int32) bool {
	player := player_mgr.GetPlayerById(player_id)
	if player == nil {
		log.Error("Tower Ranking List cant update player[%v], because of not found", player_id)
		return false
	}

	has := false
	if this.player_map[player_id] != 0 {
		has = true
	}

	if has {
		i := int32(0)
		for ; i < this.player_num; i++ {
			if this.player_list[i] == player_id {
				break
			}
		}
		if i >= this.player_num {
			log.Error("Tower Ranking List not found player[%v] to update", player_id)
			return false
		}
		n := i - 1
		for ; n >= 0; n-- {
			p := player_mgr.GetPlayerById(this.player_list[n])
			if p == nil {
				log.Error("Tower Ranking List not found player[%v]", this.player_list[n])
				return false
			}
			if p.db.TowerCommon.GetCurrId() > tower_id {
				break
			}
		}
		for j := i - 1; j >= n+1; j-- {
			this.player_list[j+1] = this.player_list[j]
		}
		this.player_list[n+1] = player_id
	} else {
		i := this.player_num - 1
		for ; i >= 0; i-- {
			p := player_mgr.GetPlayerById(this.player_list[i])
			if p == nil {
				log.Error("TowerRankingList not found player[%v]", this.player_list[i])
				return false
			}
			if p.db.TowerCommon.GetCurrId() > tower_id {
				break
			}
		}
		for n := this.player_num - 1; n >= i+1; n-- {
			if n+1 >= TOWER_RANKING_LIST_MAX {
				delete(this.player_map, this.player_list[n])
				continue
			}
			this.player_list[n+1] = this.player_list[n]
		}
		if this.player_num < TOWER_RANKING_LIST_MAX {
			this.player_list[i+1] = player_id
			this.player_num += 1
		}
		this.player_map[player_id] = player_id
	}

	return true
}

func (this *TowerRankingList) GetMsgs() (ranking_list []*msg_client_message.TowerRankInfo) {
	this.locker.RLock()
	defer this.locker.RUnlock()

	if this.player_list == nil || this.player_num == 0 {
		ranking_list = make([]*msg_client_message.TowerRankInfo, 0)
		return
	}

	for i := 0; i < int(this.player_num); i++ {
		if i >= len(this.player_list) || this.player_list[i] <= 0 {
			break
		}

		p := player_mgr.GetPlayerById(this.player_list[i])
		if p == nil {
			log.Error("Player[%v] on tower rankling list not found", this.player_list[i])
			return
		}

		ranking_list = append(ranking_list, &msg_client_message.TowerRankInfo{
			PlayerId:    this.player_list[i],
			PlayerName:  p.db.GetName(),
			TowerId:     p.db.TowerCommon.GetCurrId(),
			PlayerLevel: p.db.Info.GetLvl(),
			PlayerHead:  p.db.Info.GetHead(),
		})
	}
	return
}

func C2STowerRecordsInfoHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2STowerRecordsInfoRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}

	return p.get_tower_records_info(req.GetTowerId())
}

func C2STowerRecordDataHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2STowerRecordDataRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}

	return p.get_tower_record_data(req.GetTowerFightId())
}

func C2STowerDataHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2STowerDataRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}

	return p.send_tower_data(true)
}

func C2STowerRankingListHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2STowerRankingListRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}

	response := &msg_client_message.S2CTowerRankingListResponse{
		Ranks: tower_ranking_list.GetMsgs(),
	}
	p.Send(uint16(msg_client_message_id.MSGID_S2C_TOWER_RANKING_LIST_RESPONSE), response)
	return 1
}
