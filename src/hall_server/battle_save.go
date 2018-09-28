package main

import (
	"bytes"
	"compress/flate"
	"ih_server/libs/log"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"io"
	"net/http"
	"time"

	"github.com/golang/protobuf/proto"
)

type BattleSaveManager struct {
	saves *dbBattleSaveTable
}

var battle_record_mgr BattleSaveManager

func compress_battle_record_data(data []byte) []byte {
	var b bytes.Buffer
	w, err := flate.NewWriter(&b, 1)
	if err != nil {
		log.Error("flate.NewWriter failed: %v", err.Error())
		return nil
	}
	_, err = w.Write(data)
	if err != nil {
		log.Error("write_msgs flate.Write failed: %v", err.Error())
		return nil
	}
	err = w.Close()
	if err != nil {
		log.Error("flate.Close failed: %v", err.Error())
		return nil
	}
	data = b.Bytes()
	return data
}

func decompress_battle_record_data(data []byte) []byte {
	b := new(bytes.Buffer)
	reader := bytes.NewReader(data)
	r := flate.NewReader(reader)
	_, err := io.Copy(b, r)
	if err != nil {
		defer r.Close()
		log.Error("decompress copy failed %v", err)
		return nil
	}
	err = r.Close()
	if err != nil {
		log.Error("flate Close failed %v", err)
		return nil
	}
	data = b.Bytes()
	return data
}

func (this *BattleSaveManager) Init() {
	this.saves = dbc.BattleSaves
}

func (this *BattleSaveManager) SaveNew(attacker_id, defenser_id int32, data []byte, is_win int32, add_score int32) bool {
	attacker := player_mgr.GetPlayerById(attacker_id)
	if attacker == nil {
		return false
	}
	defenser := player_mgr.GetPlayerById(defenser_id)
	if defenser == nil {
		//return false
	}

	row := this.saves.AddRow()
	if row != nil {
		row.SetAttacker(attacker_id)
		row.SetDefenser(defenser_id)
		row.SetIsWin(is_win)
		row.SetAddScore(add_score)
		data = compress_battle_record_data(data)
		if data == nil {
			return false
		}
		now_time := int32(time.Now().Unix())
		row.Data.SetData(data)
		row.SetSaveTime(now_time)

		attacker.db.BattleSaves.Add(&dbPlayerBattleSaveData{
			Id:       row.GetId(),
			Side:     0,
			SaveTime: now_time,
		})
		attacker.push_battle_record(row.GetId())

		if defenser != nil {
			defenser.db.BattleSaves.Add(&dbPlayerBattleSaveData{
				Id:       row.GetId(),
				Side:     1,
				SaveTime: now_time,
			})
			defenser.push_battle_record(row.GetId())
		}

		log.Debug("Battle Record[%v] saved with attacker[%v] and defenser[%v]", row.GetId(), attacker_id, defenser_id)
	}
	return true
}

func (this *BattleSaveManager) GetRecord(requester_id, record_id int32) (attacker_id, defenser_id int32, record_data []byte, save_time int32, is_win int32, add_score int32) {
	row := this.saves.GetRow(record_id)
	if row == nil {
		return
	}

	delete_state := row.GetDeleteState()

	if delete_state == 1 && attacker_id == requester_id {
		log.Error("Player[%v] is attacker, had deleted record", requester_id)
		return
	} else if delete_state == 2 && defenser_id == requester_id {
		log.Error("Player[%v] is defenser, had deleted record", requester_id)
		return
	}

	attacker_id = row.GetAttacker()
	defenser_id = row.GetDefenser()
	record_data = row.Data.GetData()
	save_time = row.GetSaveTime()
	is_win = row.GetIsWin()
	add_score = row.GetAddScore()
	return
}

func (this *BattleSaveManager) DeleteRecord(requester_id, record_id int32) int32 {
	requester := player_mgr.GetPlayerById(requester_id)
	if requester == nil {
		return int32(msg_client_message.E_ERR_PLAYER_NOT_EXIST)
	}

	row := this.saves.GetRow(record_id)
	if row == nil {
		if requester.db.BattleSaves.HasIndex(record_id) {
			requester.db.BattleSaves.Remove(record_id)
		}
		log.Error("Not found battle record[%v]", record_id)
		return int32(msg_client_message.E_ERR_PLAYER_BATTLE_RECORD_NOT_FOUND)
	}

	attacker_id := row.GetAttacker()
	defenser_id := row.GetDefenser()
	if requester_id != attacker_id && requester_id != defenser_id {
		log.Error("Battle record[%v] cant delete by player[%v]", record_id, requester_id)
		return int32(msg_client_message.E_ERR_PLAYER_BATTLE_RECORD_FORBIDDEN_DELETE)
	}

	delete_state := row.GetDeleteState()
	if requester_id == attacker_id && delete_state == 1 {
		log.Error("Player[%v] already deleted battle record[%v] as attacker", requester_id, record_id)
		return int32(msg_client_message.E_ERR_PLAYER_BATTLE_RECORD_FORBIDDEN_DELETE)
	}

	if requester_id == defenser_id && delete_state == 2 {
		log.Error("Player[%v] already deleted battle record[%v] as defenser", requester_id, record_id)
		return int32(msg_client_message.E_ERR_PLAYER_BATTLE_RECORD_FORBIDDEN_DELETE)
	}

	attacker := player_mgr.GetPlayerById(attacker_id)
	defenser := player_mgr.GetPlayerById(defenser_id)
	if delete_state == 0 {
		if requester_id == attacker_id {
			row.SetDeleteState(1)
			if attacker != nil {
				attacker.db.BattleSaves.Remove(record_id)
			}
		} else {
			row.SetDeleteState(2)
			if defenser != nil {
				defenser.db.BattleSaves.Remove(record_id)
			}
		}
	} else if delete_state == 1 {
		this.saves.RemoveRow(record_id)
		if defenser != nil {
			defenser.db.BattleSaves.Remove(record_id)
		}
	} else if delete_state == 2 {
		this.saves.RemoveRow(record_id)
		if attacker != nil {
			attacker.db.BattleSaves.Remove(record_id)
		}
	} else {
		return int32(msg_client_message.E_ERR_PLAYER_BATTLE_RECORD_FORBIDDEN_DELETE)
	}

	return 1
}

func (this *Player) push_battle_record(record_id int32) (pushed bool, remove_record int32) {
	// 未上线的玩家battle_record_list很可能为空，直接返回
	if this.battle_record_list == nil {
		return
	}
	rt, o := this.db.BattleSaves.GetSaveTime(record_id)
	if !o {
		return
	}
	max_count := global_config.PlayerBattleRecordMaxCount
	i := int32(0)
	insert := false
	for ; i < this.battle_record_count; i++ {
		var st int32
		st, o = this.db.BattleSaves.GetSaveTime(this.battle_record_list[i])
		if !o {
			for j := i; j < this.battle_record_count-1; j++ {
				this.battle_record_list[j] = this.battle_record_list[j+1]
			}
			this.battle_record_count -= 1
		}
		if rt > st {
			insert = true
			break
		}
	}
	if insert {
		if this.battle_record_count >= max_count {
			battle_record_mgr.DeleteRecord(this.Id, this.battle_record_list[max_count-1])
			remove_record = this.battle_record_list[max_count-1]
		}
		// 往后挪
		for j := this.battle_record_count - 1; j >= i; j-- {
			if j >= max_count-1 {
				continue
			}
			this.battle_record_list[j+1] = this.battle_record_list[j]
		}
		this.battle_record_list[i] = record_id
		if this.battle_record_count < max_count {
			this.battle_record_count += 1
		}
		pushed = true
	} else {
		if i < max_count {
			this.battle_record_list[i] = record_id
			this.battle_record_count += 1
			pushed = true
		} else {
			battle_record_mgr.DeleteRecord(this.Id, record_id)
		}
	}
	return
}

func (this *Player) init_battle_record_list() {
	if this.battle_record_list != nil {
		return
	}

	max_count := global_config.PlayerBattleRecordMaxCount
	this.battle_record_list = make([]int32, max_count)
	record_ids := this.db.BattleSaves.GetAllIndex()
	if record_ids == nil {
		return
	}
	for i := 0; i < len(record_ids); i++ {
		row := dbc.BattleSaves.GetRow(record_ids[i])
		if row == nil {
			battle_record_mgr.DeleteRecord(this.Id, record_ids[i])
		}
		this.push_battle_record(record_ids[i])
	}

	record_ids = this.db.BattleSaves.GetAllIndex()
	if record_ids != nil {
		log.Debug("++++++++++++++++++++++ Player[%v] inited battle record list: %v", this.Id, this.battle_record_list)
	}
}

func (this *Player) GetBattleRecordList() int32 {
	var record_list []*msg_client_message.BattleRecordData
	if this.battle_record_list != nil {
		for i := 0; i < len(this.battle_record_list); i++ {
			row := dbc.BattleSaves.GetRow(this.battle_record_list[i])
			if row != nil {
				record := &msg_client_message.BattleRecordData{}
				record.RecordId = row.GetId()
				record.RecordTime = row.GetSaveTime()
				record.AddScore = row.GetAddScore()
				record.IsWin = func() bool {
					if row.GetIsWin() > 0 {
						return true
					}
					return false
				}()

				record.AttackerId = row.GetAttacker()
				attacker := player_mgr.GetPlayerById(record.AttackerId)
				if attacker != nil {
					record.AttackerName = attacker.db.GetName()
				}
				record.AttackerLevel = attacker.db.Info.GetLvl()
				record.AttackerHead = attacker.db.Info.GetHead()

				record.DefenserId = row.GetDefenser()
				name, level, head, _, _, _ := GetFighterInfo(record.DefenserId)
				record.DefenserName = name
				record.DefenserLevel = level
				record.DefenserHead = head

				record_list = append(record_list, record)
			}
		}
	}

	if record_list == nil {
		record_list = make([]*msg_client_message.BattleRecordData, 0)
	}

	response := &msg_client_message.S2CBattleRecordListResponse{
		Records: record_list,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_BATTLE_RECORD_LIST_RESPONSE), response)

	return 1
}

func (this *Player) GetBattleRecord(record_id int32) int32 {
	attacker_id, defenser_id, record_data, record_time, _, _ := battle_record_mgr.GetRecord(this.Id, record_id)
	if attacker_id == 0 {
		return int32(msg_client_message.E_ERR_PLAYER_BATTLE_RECORD_NOT_FOUND)
	}

	var attacker_name, defenser_name string
	attacker := player_mgr.GetPlayerById(attacker_id)
	if attacker != nil {
		attacker_name = attacker.db.GetName()
	}
	defenser := player_mgr.GetPlayerById(defenser_id)
	if defenser != nil {
		defenser_name = defenser.db.GetName()
	}

	record_data = decompress_battle_record_data(record_data)
	if record_data == nil {
		return int32(msg_client_message.E_ERR_PLAYER_BATTLE_RECORD_DATA_INVALID)
	}

	response := &msg_client_message.S2CBattleRecordResponse{
		Id:           record_id,
		AttackerId:   attacker_id,
		AttackerName: attacker_name,
		DefenserId:   defenser_id,
		DefenserName: defenser_name,
		RecordData:   record_data,
		RecordTime:   record_time,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_BATTLE_RECORD_RESPONSE), response)

	return 1
}

func C2SBattleRecordListHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SBattleRecordListRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}
	return p.GetBattleRecordList()
}

func C2SBattleRecordHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SBattleRecordRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}
	return p.GetBattleRecord(req.GetId())
}

func C2SBattleRecordDeleteHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SBattleRecordRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}
	return battle_record_mgr.DeleteRecord(p.Id, req.GetId())
}
