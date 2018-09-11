package main

import (
	"ih_server/libs/log"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"ih_server/src/table_config"
	"math/rand"
	"net/http"
	"time"

	"github.com/golang/protobuf/proto"
)

func (this *Player) drop_item_by_id(id int32, add bool, used_drop_ids map[int32]int32) (bool, *msg_client_message.ItemInfo) {
	drop_lib := drop_table_mgr.Map[id]
	if nil == drop_lib {
		return false, nil
	}
	item := this.drop_item(drop_lib, add, used_drop_ids)
	return true, item
}

func (this *Player) drop_item(drop_lib *table_config.DropTypeLib, badd bool, used_drop_ids map[int32]int32) (item *msg_client_message.ItemInfo) {
	get_same := false
	check_cnt := drop_lib.TotalCount
	rand_val := rand.Int31n(drop_lib.TotalWeight)
	var tmp_item *table_config.XmlDropItem
	for i := int32(0); i < drop_lib.TotalCount; i++ {
		tmp_item = drop_lib.DropItems[i]
		if nil == tmp_item {
			continue
		}

		if tmp_item.Weight > rand_val || get_same {
			if tmp_item.DropItemID == 0 {
				return nil
			}

			if used_drop_ids != nil {
				if _, o := used_drop_ids[tmp_item.DropItemID]; o {
					get_same = true
					check_cnt -= 1
					if check_cnt <= 0 {
						break
					}
					//log.Debug("!!!!!!!!!!! !!!!!!!! total_count[%v]  used_drop_ids len[%v]  i[%v]", drop_lib.TotalCount, len(used_drop_ids), i)
					continue
				}
			}
			_, num := rand31n_from_range(tmp_item.Min, tmp_item.Max)
			if nil != item_table_mgr.Map[tmp_item.DropItemID] {
				if badd {
					if !this.add_resource(tmp_item.DropItemID, num) {
						log.Error("Player[%v] rand dropid[%d] not item or cat or building or item resource", this.Id, tmp_item.DropItemID)
						continue
					}
				}
			} else {
				if card_table_mgr.GetCards(tmp_item.DropItemID) != nil {
					if badd {
						for j := int32(0); j < num; j++ {
							if this.new_role(tmp_item.DropItemID, 1, 1) == 0 {
								log.Error("Player[%v] rand dropid[%d] not item or cat or building or item resource", this.Id, tmp_item.DropItemID)
								continue
							}
						}
					}
				}
			}

			item = &msg_client_message.ItemInfo{Id: tmp_item.DropItemID, Value: num}
			if this.tmp_cache_items != nil {
				this.tmp_cache_items[tmp_item.DropItemID] += item.Value
			}
			break
		}

		rand_val -= tmp_item.Weight
	}

	return
}

func (this *Player) draw_card(draw_type int32) int32 {
	draw := draw_table_mgr.Get(draw_type)
	if draw == nil {
		log.Error("Player[%v] draw id[%v] not found", this.Id, draw_type)
		return -1
	}

	is_free := false
	now_time := int32(time.Now().Unix())
	if draw.FreeExtractTime > 0 {
		last_draw, o := this.db.Draws.GetLastDrawTime(draw_type)
		if !o || now_time-last_draw >= draw.FreeExtractTime {
			is_free = true
		}
	}

	n := int32(0)
	for i := 0; i < len(draw.DropId)/2; i++ {
		n += draw.DropId[2*i+1]
	}
	if this.db.Roles.NumAll()+n > global_config.MaxRoleCount {
		log.Error("Player[%v] role inventory not enough space", this.Id)
		return int32(msg_client_message.E_ERR_PLAYER_ROLE_INVENTORY_NOT_ENOUGH_SPACE)
	}

	// 资源
	is_enough := 0
	var res_condition []int32
	if !is_free {
		if (draw.ResCondition1 == nil || len(draw.ResCondition1) == 0) && (draw.ResCondition2 == nil || len(draw.ResCondition2) == 0) {
			is_enough = 3
		}
		if draw.ResCondition1 != nil && len(draw.ResCondition1) > 0 {
			i := 0
			for ; i < len(draw.ResCondition1)/2; i++ {
				res_id := draw.ResCondition1[2*i]
				res_num := draw.ResCondition1[2*i+1]
				if this.get_resource(res_id) < res_num {
					break
				}
			}
			if i >= len(draw.ResCondition1)/2 {
				res_condition = draw.ResCondition1
				is_enough = 1
			}
		}
		if is_enough == 0 {
			if draw.ResCondition2 != nil && len(draw.ResCondition2) > 0 {
				i := 0
				for ; i < len(draw.ResCondition2)/2; i++ {
					res_id := draw.ResCondition2[2*i]
					res_num := draw.ResCondition2[2*i+1]
					if this.get_resource(res_id) < res_num {
						break
					}
				}
				if i >= len(draw.ResCondition2)/2 {
					res_condition = draw.ResCondition2
					is_enough = 2
				}
			}
		}
		if is_enough == 0 {
			log.Error("Player[%v] not enough res to draw card", this.Id)
			return int32(msg_client_message.E_ERR_PLAYER_ITEM_NUM_NOT_ENOUGH)
		}
	}

	var role_ids []int32
	for i := 0; i < len(draw.DropId)/2; i++ {
		did := draw.DropId[2*i]
		dn := draw.DropId[2*i+1]
		for j := 0; j < int(dn); j++ {
			o, item := this.drop_item_by_id(did, true, nil)
			if !o {
				log.Error("Player[%v] draw type[%v] with drop_id[%v] failed", this.Id, draw_type, did)
				return -1
			}
			role_ids = append(role_ids, item.GetId())
		}
	}

	if !is_free {
		if res_condition != nil {
			for i := 0; i < len(res_condition)/2; i++ {
				res_id := res_condition[2*i]
				res_num := res_condition[2*i+1]
				this.add_resource(res_id, -res_num)
			}
		}
	} else {
		if !this.db.Draws.HasIndex(draw_type) {
			this.db.Draws.Add(&dbPlayerDrawData{
				Type:         draw_type,
				LastDrawTime: now_time,
			})
		} else {
			this.db.Draws.SetLastDrawTime(draw_type, now_time)
		}
	}

	response := &msg_client_message.S2CDrawCardResponse{
		DrawType:    draw_type,
		RoleTableId: role_ids,
		IsFreeDraw:  is_free,
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_DRAW_CARD_RESPONSE), response)

	if is_free {
		this.send_draw_data()
	}

	// 任务更新
	var a int32
	if draw_type == 1 || draw_type == 2 {
		a = 1
	} else if draw_type == 3 || draw_type == 4 {
		a = 2
	}
	this.TaskUpdate(table_config.TASK_COMPLETE_TYPE_DRAW_NUM, false, a, 1)

	log.Debug("Player[%v] drawed card[%v] with draw type[%v], is free[%v]", this.Id, role_ids, draw_type, is_free)

	return 1
}

func (this *Player) send_draw_data() int32 {
	free_secs := make(map[int32]int32)
	all_type := this.db.Draws.GetAllIndex()
	if all_type != nil && len(all_type) > 0 {
		now_time := int32(time.Now().Unix())
		for _, t := range all_type {
			draw_time, _ := this.db.Draws.GetLastDrawTime(t)
			draw_data := draw_table_mgr.Get(t)
			if draw_data == nil {
				log.Warn("Cant found draw data with id[%v] in send player[%v] data", t, this.Id)
				continue
			}
			remain_seconds := draw_data.FreeExtractTime - (now_time - draw_time)
			if remain_seconds < 0 {
				remain_seconds = 0
			}
			free_secs[t] = remain_seconds
		}
	} else {
		for _, d := range draw_table_mgr.Array {
			if d != nil && d.FreeExtractTime > 0 {
				free_secs[d.Id] = 0
			}
		}
	}

	response := &msg_client_message.S2CDrawDataResponse{
		FreeDrawRemainSeconds: Map2ItemInfos(free_secs),
	}
	this.Send(uint16(msg_client_message_id.MSGID_S2C_DRAW_DATA_RESPONSE), response)

	log.Debug("Player[%v] draw data is %v", this.Id, response)

	return 1
}

func C2SDrawCardHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SDrawCardRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}
	return p.draw_card(req.GetDrawType())
}

func C2SDrawDataHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SDrawDataRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}

	return p.send_draw_data()
}
