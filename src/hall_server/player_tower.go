package main

import (
	"ih_server/libs/log"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"ih_server/src/table_config"
	"time"

	"github.com/golang/protobuf/proto"
)

func get_tower_fight_id(tower_id, i int32) int32 {
	return tower_id*10 + i
}

func (this *Player) send_tower_data(send bool) int32 {
	need_level := system_unlock_table_mgr.GetUnlockLevel("TowerEnterLevel")
	if need_level > this.db.Info.GetLvl() {
		log.Error("Player[%v] level not enough level %v enter tower", this.Id, need_level)
		return int32(msg_client_message.E_ERR_PLAYER_LEVEL_NOT_ENOUGH)
	}

	var tower_keys, remain_seconds int32
	var updated bool
	updated, tower_keys, remain_seconds = this.check_tower_keys()
	if send || updated {
		response := &msg_client_message.S2CTowerDataResponse{
			CurrTowerId:   this.db.TowerCommon.GetCurrId(),
			TowerKeys:     tower_keys,
			RemainSeconds: remain_seconds,
		}
		this.Send(uint16(msg_client_message_id.MSGID_S2C_TOWER_DATA_RESPONSE), response)
		log.Debug("Player[%v] tower data %v", this.Id, response)
	}
	return 1
}

func (this *Player) check_tower_keys() (is_update bool, keys int32, next_remain_seconds int32) {
	tower_key_max := global_config.TowerKeyMax
	tower_key_get_interval := global_config.TowerKeyGetInterval
	//keys = this.db.TowerCommon.GetKeys()
	keys = this.get_resource(global_config.TowerKeyId)
	old_keys := keys
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
				this.db.TowerCommon.SetLastGetNewKeyTime(now_time - y)
			}
			if keys < tower_key_max {
				next_remain_seconds = global_config.TowerKeyGetInterval - y
			}
		}
	} else if keys > tower_key_max {
		//keys = tower_key_max
	}
	if old_keys != keys {
		this.set_resource(global_config.TowerKeyId, keys)
		is_update = true
	}
	return
}

func (this *Player) check_and_send_tower_data() {
	this.send_tower_data(false)
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

	err, is_win, my_team, target_team, my_artifact_id, target_artifact_id, enter_reports, rounds, _ := this.FightInStage(3, stage, nil, nil)
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
		MyArtifactId:        my_artifact_id,
		TargetArtifactId:    target_artifact_id,
	}
	data := this.Send(uint16(msg_client_message_id.MSGID_S2C_BATTLE_RESULT_RESPONSE), response)

	if is_win {
		this.db.TowerCommon.SetCurrId(tower_id)
		now_time := int32(time.Now().Unix())
		this.db.TowerCommon.SetPassTowerTime(now_time)
		// 名次
		this.tower_update_rank(tower_id, now_time)
		// 奖励
		this.send_stage_reward(stage.RewardList, 3, 0)
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

	log.Trace("Player %v fight tower %v", this.Id, tower_id)

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
func (this *Player) tower_update_rank(tower_id, update_time int32) {
	var data = PlayerInt32RankItem{
		Value:      tower_id,
		UpdateTime: update_time,
		PlayerId:   this.Id,
	}
	rank_list_mgr.UpdateItem(RANK_LIST_TYPE_TOWER, &data)
}

func (this *Player) load_tower_db_data() {
	tower_id := this.db.TowerCommon.GetCurrId()
	if tower_id <= 0 {
		return
	}
	update_time := this.db.TowerCommon.GetPassTowerTime()
	if update_time == 0 {
		update_time = this.db.Info.GetLastLogin()
	}
	this.tower_update_rank(tower_id, update_time)
}

const (
	TOWER_RANKING_LIST_MAX = 50
)

func C2STowerRecordsInfoHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2STowerRecordsInfoRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}

	return p.get_tower_records_info(req.GetTowerId())
}

func C2STowerRecordDataHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2STowerRecordDataRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}

	return p.get_tower_record_data(req.GetTowerFightId())
}

func C2STowerDataHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2STowerDataRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}

	return p.send_tower_data(true)
}

func get_tower_rank_list() (ranking_list []*msg_client_message.TowerRankInfo) {
	items, _, _ := rank_list_mgr.GetItemsByRange(RANK_LIST_TYPE_TOWER, 0, 1, TOWER_RANKING_LIST_MAX)
	if items == nil {
		return
	}

	for i := int32(0); i < int32(len(items)); i++ {
		item := (items[i]).(*PlayerInt32RankItem)
		if item == nil {
			continue
		}

		p := player_mgr.GetPlayerById(item.PlayerId)
		if p == nil {
			log.Error("Player[%v] on tower rankling list not found", item.PlayerId)
			return
		}

		ranking_list = append(ranking_list, &msg_client_message.TowerRankInfo{
			PlayerId:    item.PlayerId,
			PlayerName:  p.db.GetName(),
			TowerId:     p.db.TowerCommon.GetCurrId(),
			PlayerLevel: p.db.Info.GetLvl(),
			PlayerHead:  p.db.Info.GetHead(),
		})
	}

	log.Debug("Tower Rank list %v", ranking_list)
	return
}

func C2STowerRankingListHandler(p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2STowerRankingListRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}

	response := &msg_client_message.S2CTowerRankingListResponse{
		Ranks: get_tower_rank_list(),
	}
	p.Send(uint16(msg_client_message_id.MSGID_S2C_TOWER_RANKING_LIST_RESPONSE), response)
	return 1
}
