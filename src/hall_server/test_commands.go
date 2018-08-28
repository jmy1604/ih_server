package main

import (
	"ih_server/libs/log"
	"ih_server/libs/utils"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"net/http"
	"strconv"

	"github.com/golang/protobuf/proto"
)

func set_level_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}
	var level int
	var err error
	level, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}

	p.db.Info.SetLvl(int32(level))
	p.b_base_prop_chg = true
	p.send_info()
	return 1
}

func test_lua_cmd(p *Player, args []string) int32 {
	/*L := lua.NewState(lua.Options{SkipOpenLibs: true})
	defer L.Close()
	for _, pair := range []struct {
		n string
		f lua.LGFunction
	}{
		{lua.LoadLibName, lua.OpenPackage}, // Must be first
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
	} {
		if err := L.CallByParam(lua.P{
			Fn:      L.NewFunction(pair.f),
			NRet:    0,
			Protect: true,
		}, lua.LString(pair.n)); err != nil {
			panic(err)
		}
	}
	if err := L.DoFile("main.lua"); err != nil {
		panic(err)
	}*/
	return 1
}

func rand_role_cmd(p *Player, args []string) int32 {
	role_id := p.rand_role()
	if role_id <= 0 {
		log.Warn("Cant rand role")
	} else {
		log.Debug("Rand role: %v", role_id)
	}
	return 1
}

func new_role_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var table_id, num int
	var err error
	table_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("转换角色配置ID[%v]错误[%v]", args[0], err.Error())
		return -1
	}

	if len(args) > 1 {
		num, err = strconv.Atoi(args[1])
		if err != nil {
			log.Error("转换角色数量[%v]错误[%v]", args[1], err.Error())
			return -1
		}
	}

	if num == 0 {
		num = 1
	}
	for i := 0; i < num; i++ {
		p.new_role(int32(table_id), 1, 1)
	}
	return 1
}

func all_roles_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var num, level, rank int
	var err error
	num, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}
	if len(args) > 1 {
		level, err = strconv.Atoi(args[1])
		if err != nil {
			return -1
		}
	} else {
		level = 1
	}

	if len(args) > 2 {
		rank, err = strconv.Atoi(args[1])
		if err != nil {
			return -1
		}
	} else {
		rank = 1
	}

	for _, c := range card_table_mgr.Array {
		for i := 0; i < num; i++ {
			p.new_role(c.Id, int32(rank), int32(level))
		}
	}
	return 1
}

func list_role_cmd(p *Player, args []string) int32 {
	var camp, typ, star int
	var err error
	if len(args) > 0 {
		camp, err = strconv.Atoi(args[0])
		if err != nil {
			log.Error("转换阵营[%v]错误[%v]", args[0], err.Error())
			return -1
		}
		if len(args) > 1 {
			typ, err = strconv.Atoi(args[1])
			if err != nil {
				log.Error("转换卡牌类型[%v]错误[%v]", args[1], err.Error())
				return -1
			}
			if len(args) > 2 {
				star, err = strconv.Atoi(args[2])
				if err != nil {
					log.Error("转换卡牌星级[%v]错误[%v]", args[2], err.Error())
					return -1
				}
			}
		}
	}
	all := p.db.Roles.GetAllIndex()
	if all != nil {
		for i := 0; i < len(all); i++ {
			table_id, o := p.db.Roles.GetTableId(all[i])
			if !o {
				continue
			}

			level, _ := p.db.Roles.GetLevel(all[i])
			rank, _ := p.db.Roles.GetRank(all[i])

			card := card_table_mgr.GetRankCard(table_id, rank)
			if card == nil {
				continue
			}

			if camp > 0 && card.Camp != int32(camp) {
				continue
			}
			if typ > 0 && card.Type != int32(typ) {
				continue
			}
			if star > 0 && card.Rarity != int32(star) {
				continue
			}

			equips, _ := p.db.Roles.GetEquip(all[i])
			log.Debug("role_id:%v, table_id:%v, level:%v, rank:%v, camp:%v, type:%v, star:%v, equips:%v", all[i], table_id, level, rank, card.Camp, card.Type, card.Rarity, equips)
		}
	}
	return 1
}

func create_battle_team_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var err error
	var battle_type int
	battle_type, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("转换阵型类型[%v]错误[%v]", args[0], err.Error())
		return -1
	}

	if battle_type == 0 {

	}

	return 1
}

func set_attack_team_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var err error
	var role_id int
	var team []int32
	for i := 0; i < len(args); i++ {
		role_id, err = strconv.Atoi(args[i])
		if err != nil {
			log.Error("转换角色ID[%v]错误[%v]", role_id, err.Error())
			return -1
		}
		team = append(team, int32(role_id))
	}

	/*if p.SetAttackTeam(team) < 0 {
		log.Error("设置玩家[%v]攻击阵容失败", p.Id)
		return -1
	}*/

	return 1
}

func set_defense_team_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var err error
	var role_id int
	var team []int32
	for i := 0; i < len(args); i++ {
		role_id, err = strconv.Atoi(args[i])
		if err != nil {
			log.Error("转换角色ID[%v]错误[%v]", role_id, err.Error())
			return -1
		}
		team = append(team, int32(role_id))
	}

	if p.SetDefenseTeam(team) < 0 {
		log.Error("设置玩家[%v]防守阵容失败", p.Id)
		return -1
	}

	return 1
}

func list_teams_cmd(p *Player, args []string) int32 {
	log.Debug("defense team: %v", p.db.BattleTeam.GetDefenseMembers())
	log.Debug("campaign team: %v", p.db.BattleTeam.GetCampaignMembers())
	return 1
}

func pvp_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var err error
	var player_id int
	player_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("转换玩家ID[%v]失败[%v]", args[0], err.Error())
		return -1
	}

	p.Fight2Player(1, int32(player_id))

	log.Debug("玩家[%v]pvp玩家[%v]", p.Id, player_id)
	return 1
}

func fight_stage_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var err error
	var stage_id, stage_type int
	stage_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("转换关卡[%v]失败[%v]", args[0], err.Error())
		return -1
	}

	stage := stage_table_mgr.Get(int32(stage_id))
	if stage == nil {
		log.Error("关卡[%v]不存在", stage_id)
		return -1
	}

	if len(args) > 1 {
		stage_type, err = strconv.Atoi(args[1])
		if err != nil {
			log.Error("转换关卡类型[%v]失败[%v]", args[1], err.Error())
			return -1
		}
	} else {
		stage_type = 1
	}

	err_code, is_win, my_team, target_team, enter_reports, rounds, has_next_wave := p.FightInStage(int32(stage_type), stage, nil, nil)
	if err_code < 0 {
		log.Error("Player[%v] fight stage %v, team is empty", p.Id, stage_id)
		return err_code
	}

	response := &msg_client_message.S2CBattleResultResponse{}
	response.IsWin = is_win
	response.MyTeam = my_team
	response.TargetTeam = target_team
	response.EnterReports = enter_reports
	response.Rounds = rounds
	response.HasNextWave = has_next_wave
	p.Send(uint16(msg_client_message_id.MSGID_S2C_BATTLE_RESULT_RESPONSE), response)
	Output_S2CBattleResult(p, response)
	log.Debug("玩家[%v]挑战了关卡[%v]", p.Id, stage_id)
	return 1
}

func fight_campaign_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var err error
	var campaign_id int
	campaign_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("转换关卡ID[%v]失败[%v]", args[0], err.Error())
		return -1
	}

	res := p.FightInCampaign(int32(campaign_id))
	if res < 0 {
		log.Error("玩家[%v]挑战战役关卡[%v]失败[%v]", p.Id, campaign_id, res)
	} else {
		log.Debug("玩家[%v]挑战了战役关卡[%v]", p.Id, campaign_id)
	}
	return res
}

func start_hangup_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var err error
	var campaign_id int
	campaign_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("转换战役ID[%v]失败[%v]", args[0], err.Error())
		return -1
	}

	res := p.set_hangup_campaign_id(int32(campaign_id))
	if res < 0 {
		return res
	}

	log.Debug("玩家[%v]设置了挂机战役关卡[%v]", p.Id, campaign_id)
	return 1
}

func hangup_income_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var err error
	var income_type int
	income_type, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("转换收益类型[%v]失败[%v]", args[0], err.Error())
		return -1
	}

	p.hangup_income_get(int32(income_type), false)

	log.Debug("玩家[%v]获取了类型[%v]挂机收益", p.Id, income_type)

	return 1
}

func campaign_data_cmd(p *Player, args []string) int32 {
	p.send_campaigns()
	return 1
}

func leave_game_cmd(p *Player, args []string) int32 {
	p.OnLogout()
	return 1
}

func add_item_cmd(p *Player, args []string) int32 {
	if len(args) < 2 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var err error
	var item_id, item_num int
	item_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("转换物品ID[%v]失败[%v]", args[0], err.Error())
		return -1
	}
	item_num, err = strconv.Atoi(args[1])
	if err != nil {
		log.Error("转换物品数量[%v]失败[%v]", args[1], err.Error())
		return -1
	}

	if !p.add_resource(int32(item_id), int32(item_num)) {
		return -1
	}

	log.Debug("玩家[%v]增加了资源[%v,%v]", p.Id, item_id, item_num)
	return 1
}

func all_items_cmd(p *Player, args []string) int32 {
	a := item_table_mgr.Array
	for _, item := range a {
		p.add_resource(item.Id, 10000)
	}
	return 1
}

func clear_items_cmd(p *Player, args []string) int32 {
	p.db.Items.Clear()
	p.send_items()
	return 1
}

func role_levelup_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var err error
	var role_id, up_num int
	role_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("转换角色ID[%v]失败[%v]", args[0], err.Error())
		return -1
	}
	if len(args) > 1 {
		up_num, err = strconv.Atoi(args[1])
		if err != nil {
			log.Error("转换升级次数[%v]失败[%v]", args[1], err.Error())
			return -1
		}
	}

	res := p.levelup_role(int32(role_id), int32(up_num))
	if res > 0 {
		log.Debug("玩家[%v]升级了角色[%v]等级[%v]", p.Id, role_id, res)
	}

	return res
}

func role_rankup_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var err error
	var role_id int
	role_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("转换角色ID[%v]失败[%v]", args[0], err.Error())
		return -1
	}

	res := p.rankup_role(int32(role_id))
	if res > 0 {
		log.Debug("玩家[%v]升级了角色[%v]品阶[%v]", p.Id, role_id, res)
	}

	return res
}

func role_decompose_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var err error
	var role_id int
	var role_ids []int32
	for i := 0; i < len(args); i++ {
		role_id, err = strconv.Atoi(args[i])
		if err != nil {
			log.Error("转换角色ID[%v]失败[%v]", args[i], err.Error())
			return -1
		}
		role_ids = append(role_ids, int32(role_id))
	}

	return p.decompose_role(role_ids)
}

func item_fusion_cmd(p *Player, args []string) int32 {
	if len(args) < 2 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var err error
	var piece_id, fusion_num int
	piece_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("转换碎片ID[%v]失败[%v]", args[0], err.Error())
		return -1
	}
	fusion_num, err = strconv.Atoi(args[1])
	if err != nil {
		log.Error("转换合成次数[%v]失败[%v]", args[1], err.Error())
		return -1
	}

	return p.fusion_item(int32(piece_id), int32(fusion_num))
}

func item_sell_cmd(p *Player, args []string) int32 {
	if len(args) < 2 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var err error
	var item_id, item_num int
	item_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("转换物品ID[%v]失败[%v]", args[0], err.Error())
		return -1
	}
	item_num, err = strconv.Atoi(args[1])
	if err != nil {
		log.Error("转换物品数量[%v]失败[%v]", args[1], err.Error())
		return -1
	}

	return p.sell_item(int32(item_id), int32(item_num))
}

func fusion_role_cmd(p *Player, args []string) int32 {
	if len(args) < 3 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var err error
	var fusion_id, main_card_id int
	fusion_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("转换合成角色ID[%v]失败[%v]", args[0], err.Error())
		return -1
	}
	main_card_id, err = strconv.Atoi(args[1])
	if err != nil {
		log.Error("转换主卡ID[%v]失败[%v]", args[1], err.Error())
		return -1
	}

	var cost1_ids, cost2_ids, cost3_ids []int32
	cost1_ids = parse_xml_str_arr2(args[2], "|")
	if cost1_ids == nil || len(cost1_ids) == 0 {
		log.Error("消耗角色1系列转换错误")
		return -1
	}
	if len(args) > 3 {
		cost2_ids = parse_xml_str_arr2(args[3], "|")
		if cost2_ids == nil || len(cost2_ids) == 0 {
			log.Error("消耗角色2系列转换错误")
			return -1
		}
		if len(args) > 4 {
			cost3_ids = parse_xml_str_arr2(args[4], "|")
			if cost3_ids == nil || len(cost3_ids) == 0 {
				log.Error("消耗角色3系列转换错误")
				return -1
			}
		}
	}

	return p.fusion_role(int32(fusion_id), int32(main_card_id), [][]int32{cost1_ids, cost2_ids, cost3_ids})
}

func send_mail_cmd(p *Player, args []string) int32 {
	if len(args) < 4 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var receiver_id, mail_type int
	//var title, content string
	var err error
	receiver_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("接收者ID[%v]转换失败[%v]", receiver_id, err.Error())
		return -1
	}
	mail_type, err = strconv.Atoi(args[1])
	if err != nil {
		log.Error("邮件类型[%v]转换失败[%v]", mail_type, err.Error())
		return -1
	}

	var attach_item_id int
	if len(args) > 4 {
		attach_item_id, err = strconv.Atoi(args[4])
		if err != nil {
			log.Error("邮件附件[%v]转换失败[%v]", attach_item_id, err.Error())
			return -1
		}
	}

	var items []*msg_client_message.ItemInfo
	if attach_item_id > 0 {
		item := &msg_client_message.ItemInfo{
			ItemCfgId: int32(attach_item_id),
			ItemNum:   1,
		}
		items = []*msg_client_message.ItemInfo{item}
	}
	return SendMail(p, int32(receiver_id), int32(mail_type), args[2], args[3], items)
}

func mail_list_cmd(p *Player, args []string) int32 {
	return p.GetMailList()
}

func mail_detail_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	mail_ids := parse_xml_str_arr2(args[0], "|")
	return p.GetMailDetail(mail_ids)
}

func mail_items_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var mail_ids []int32
	for i := 0; i < len(args); i++ {
		var mail_id int
		var err error
		mail_id, err = strconv.Atoi(args[0])
		if err != nil {
			log.Error("邮件ID[%v]转换失败[%v]", args[0], err.Error())
			return -1
		}
		mail_ids = append(mail_ids, int32(mail_id))
	}

	return p.GetMailAttachedItems(mail_ids)
}

func delete_mail_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	mail_ids := parse_xml_str_arr2(args[0], "|")
	return p.DeleteMails(mail_ids)
}

func talent_data_cmd(p *Player, args []string) int32 {
	return p.send_talent_list()
}

func up_talent_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var talent_id int
	var err error
	talent_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("天赋ID[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}

	return p.up_talent(int32(talent_id))
}

func talent_reset_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var tag int
	var err error
	tag, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}

	return p.talent_reset(int32(tag))
}

func tower_data_cmd(p *Player, args []string) int32 {
	return p.send_tower_data(true)
}

func fight_tower_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var tower_id int
	var err error
	tower_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("爬塔ID[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}

	return p.fight_tower(int32(tower_id))
}

func get_tower_key_cmd(p *Player, args []string) int32 {
	tower_key_max := global_config.TowerKeyMax
	p.db.TowerCommon.SetKeys(tower_key_max)
	return p.send_tower_data(false)
}

func tower_records_info_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var tower_id int
	var err error
	tower_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("爬塔ID[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}

	return p.get_tower_records_info(int32(tower_id))
}

func tower_record_data_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var tower_fight_id int
	var err error
	tower_fight_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("爬塔战斗ID[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}

	return p.get_tower_record_data(int32(tower_fight_id))
}

func tower_ranklist_cmd(p *Player, args []string) int32 {
	rank_list := tower_ranking_list.player_list[:tower_ranking_list.player_num]
	log.Debug("TowerRankList: %v", rank_list)
	return 1
}

func battle_recordlist_cmd(p *Player, args []string) int32 {
	return p.GetBattleRecordList()
}

func battle_record_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var record_id int
	var err error
	record_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("战斗录像ID[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}

	return p.GetBattleRecord(int32(record_id))
}

func tw_func1(param interface{}) int32 {
	p := param.(int32)
	log.Debug("tw_func1 param %v", p)
	return 1
}

func test_stw_cmd(p *Player, args []string) int32 {
	stw := utils.NewSimpleTimeWheel()
	stw.Run()
	return 1
}

func item_upgrade_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var item_id, item_num int
	var err error
	item_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("物品ID[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}

	if len(args) > 1 {
		item_num, err = strconv.Atoi(args[1])
		if err != nil {
			log.Error("物品数量[%v]转换失败[%v]", args[1], err.Error())
			return -1
		}
	}

	return p.item_upgrade(0, int32(item_id), int32(item_num), 0)
}

func role_item_upgrade_cmd(p *Player, args []string) int32 {
	if len(args) < 2 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var role_id, item_id, item_num, upgrade_type int
	var err error
	role_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("角色ID[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}
	item_id, err = strconv.Atoi(args[1])
	if err != nil {
		log.Error("物品ID[%v]转换失败[%v]", args[1], err.Error())
		return -1
	}
	if len(args) > 2 {
		item_num, err = strconv.Atoi(args[1])
		if err != nil {
			log.Error("物品数量[%v]转换失败[%v]", args[1], err.Error())
			return -1
		}
	}
	if len(args) > 3 {
		upgrade_type, err = strconv.Atoi(args[2])
		if err != nil {
			log.Error("升级类型[%v]转换失败[%v]", args[2], err.Error())
			return -1
		}
	}

	return p.item_upgrade(int32(role_id), int32(item_id), int32(item_num), int32(upgrade_type))
}

func equip_item_cmd(p *Player, args []string) int32 {
	if len(args) < 2 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var role_id, equip_id int
	var err error
	role_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("角色ID[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}
	equip_id, err = strconv.Atoi(args[1])
	if err != nil {
		log.Error("物品ID[%v]转换失败[%v]", args[1], err.Error())
		return -1
	}
	return p.equip(int32(role_id), int32(equip_id))
}

func unequip_item_cmd(p *Player, args []string) int32 {
	if len(args) < 2 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var role_id, equip_type int
	var err error
	role_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("角色ID[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}
	equip_type, err = strconv.Atoi(args[1])
	if err != nil {
		log.Error("装备类型[%v]转换失败[%v]", args[1], err.Error())
		return -1
	}
	return p.equip(int32(role_id), int32(equip_type))
}

func list_items_cmd(p *Player, args []string) int32 {
	items := p.db.Items.GetAllIndex()
	if items == nil {
		return 0
	}

	log.Debug("Player[%v] Items:", p.Id)
	for _, item := range items {
		c, _ := p.db.Items.GetCount(item)
		log.Debug("    item: %v,  num: %v", item, c)
	}
	return 1
}

func get_role_attrs_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var role_id int
	var err error
	role_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("角色ID[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}

	return p.send_role_attrs(int32(role_id))
}

func onekey_equip_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var role_id int
	var err error
	role_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("角色id[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}

	return p.role_one_key_equip(int32(role_id), nil)
}

func onekey_unequip_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var role_id int
	var err error
	role_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("角色id[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}

	return p.role_one_key_unequip(int32(role_id))
}

func left_slot_open_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var role_id int
	var err error
	role_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("角色id[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}

	return p.role_open_left_slot(int32(role_id))
}

func role_equips_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var role_id int
	var err error
	role_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("角色id[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}

	equips, o := p.db.Roles.GetEquip(int32(role_id))
	if !o {
		log.Error("玩家[%v]没有角色[%v]", p.Id, role_id)
		return -1
	}

	log.Debug("玩家[%v]角色[%v]已装备物品[%v]", p.Id, role_id, equips)

	return 1
}

func draw_card_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var draw_type int
	var err error
	draw_type, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("抽卡类型[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}

	return p.draw_card(int32(draw_type))
}

func draw_data_cmd(p *Player, args []string) int32 {
	return p.send_draw_data()
}

func shop_data_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var shop_id int
	var err error
	shop_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("商店ID[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}

	return p.send_shop(int32(shop_id))
}

func buy_item_cmd(p *Player, args []string) int32 {
	if len(args) < 2 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var shop_id, item_id, item_num int
	var err error
	shop_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("商店[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}
	item_id, err = strconv.Atoi(args[1])
	if err != nil {
		log.Error("商品[%v]转换失败[%v]", args[1], err.Error())
		return -1
	}

	if len(args) > 2 {
		item_num, err = strconv.Atoi(args[2])
		if err != nil {
			log.Error("商品数量[%v]转换失败[%v]", args[2], err.Error())
			return -1
		}
	}

	return p.shop_buy_item(int32(shop_id), int32(item_id), int32(item_num))
}

func shop_refresh_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var shop_id int
	var err error
	shop_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("商店[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}

	return p.shop_refresh(int32(shop_id))
}

func arena_ranklist_cmd(p *Player, args []string) int32 {
	if len(args) < 2 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var start, num int
	var err error
	start, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("开始排名[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}
	num, err = strconv.Atoi(args[1])
	if err != nil {
		log.Error("排名数[%v]转换失败[%v]", args[1], err.Error())
		return -1
	}

	p.OutputArenaRankItems(int32(start), int32(num))

	return 1
}

func arena_data_cmd(p *Player, args []string) int32 {
	return p.send_arena_data()
}

func arena_player_team_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var player_id int
	var err error
	player_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("玩家ID[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}

	return p.arena_player_defense_team(int32(player_id))
}

func arena_match_cmd(p *Player, args []string) int32 {
	return p.arena_match()
}

func arena_reset_cmd(p *Player, args []string) int32 {
	if arena_season_mgr.IsSeasonStart() {
		arena_season_mgr.SeasonEnd()
	}
	arena_season_mgr.Reset()
	arena_season_mgr.SeasonStart()
	return 1
}

func rank_list_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var rank_type int
	var err error
	rank_type, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("排行榜类型[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}

	return p.get_rank_list_items(int32(rank_type), 1, global_config.ArenaGetTopRankNum)
}

func player_info_cmd(p *Player, args []string) int32 {
	p.send_info()
	return 1
}

func item_onekey_upgrade_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var item_ids []int32
	var item_id int
	var err error
	item_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("物品ID[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}
	item_ids = append(item_ids, int32(item_id))
	if len(args) > 1 {
		for i := 1; i < len(args); i++ {
			item_id, err = strconv.Atoi(args[i])
			if err != nil {
				return -1
			}
			item_ids = append(item_ids, int32(item_id))
		}
	}

	return p.items_one_key_upgrade(item_ids)
}

func active_stage_data_cmd(p *Player, args []string) int32 {
	return p.send_active_stage_data(0)
}

func active_stage_buy_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var active_stage_type int
	var err error
	active_stage_type, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("活动副本类型[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}
	return p.active_stage_challenge_num_purchase(int32(active_stage_type))
}

func fight_active_stage_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var active_stage_id int
	var err error
	active_stage_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("活动副本ID[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}

	return p.fight_active_stage(int32(active_stage_id))
}

func friend_recommend_cmd(p *Player, args []string) int32 {
	player_ids := friend_recommend_mgr.Random(p.Id)
	log.Debug("Recommended friend ids: %v", player_ids)
	return 1
}

func friend_data_cmd(p *Player, args []string) int32 {
	return p.friend_data(true)
}

func friend_ask_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var player_id int
	var err error
	player_id, err = strconv.Atoi(args[0])
	if err != nil {
		log.Error("玩家ID[%v]转换失败[%v]", args[0], err.Error())
		return -1
	}

	return p.friend_ask([]int32{int32(player_id)})
}

func friend_agree_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var player_id int
	var err error
	player_id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}

	return p.agree_friend_ask([]int32{int32(player_id)})
}

func friend_refuse_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var player_id int
	var err error
	player_id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}

	return p.refuse_friend_ask([]int32{int32(player_id)})
}

func friend_remove_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var player_id int
	var err error
	player_id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}
	return p.remove_friend([]int32{int32(player_id)})
}

func friend_list_cmd(p *Player, args []string) int32 {
	return p.send_friend_list()
}

func friend_ask_list_cmd(p *Player, args []string) int32 {
	return p.send_friend_ask_list()
}

func friend_give_points_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var player_id int
	var err error
	player_id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}
	return p.give_friends_points([]int32{int32(player_id)})
}

func friend_get_points_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}
	var player_id int
	var err error
	player_id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}
	return p.get_friend_points([]int32{int32(player_id)})
}

func friend_search_boss_cmd(p *Player, args []string) int32 {
	return p.friend_search_boss()
}

func friend_boss_list_cmd(p *Player, args []string) int32 {
	return p.get_friends_boss_list()
}

func friend_boss_attacks_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var player_id int
	var err error
	player_id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}
	return p.friend_boss_get_attack_list(int32(player_id))
}

func friend_fight_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var friend_id int
	var err error
	friend_id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}

	var sweep_num int
	if len(args) > 1 {
		sweep_num, err = strconv.Atoi(args[1])
		if err != nil {
			return -1
		}
	}

	p.sweep_num = int32(sweep_num)
	p.curr_sweep = 0
	return p.friend_boss_challenge(int32(friend_id))
}

func assist_list_cmd(p *Player, args []string) int32 {
	return p.active_stage_get_friends_assist_role_list()
}

func friend_set_assist_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var role_id int
	var err error
	role_id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}
	return p.friend_set_assist_role(int32(role_id))
}

func use_assist_cmd(p *Player, args []string) int32 {
	if len(args) < 5 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var battle_type, battle_param, friend_id, role_id, member_pos int
	var err error
	battle_type, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}
	battle_param, err = strconv.Atoi(args[1])
	if err != nil {
		return -1
	}
	friend_id, err = strconv.Atoi(args[2])
	if err != nil {
		return -1
	}
	role_id, err = strconv.Atoi(args[3])
	if err != nil {
		return -1
	}
	member_pos, err = strconv.Atoi(args[4])
	if err != nil {
		return -1
	}
	return p.fight(nil, int32(battle_type), int32(battle_param), int32(friend_id), int32(role_id), int32(member_pos))
}

func task_data_cmd(p *Player, args []string) int32 {
	var task_type int
	var err error
	if len(args) > 1 {
		task_type, err = strconv.Atoi(args[0])
		if err != nil {
			return -1
		}
	}
	return p.send_task(int32(task_type))
}

func task_reward_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}
	var task_id int
	var err error
	task_id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}
	return p.task_get_reward(int32(task_id))
}

func explore_data_cmd(p *Player, args []string) int32 {
	return p.send_explore_data()
}

func explore_sel_role_cmd(p *Player, args []string) int32 {
	if len(args) < 2 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var task_id, is_story int
	var err error
	task_id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}
	is_story, err = strconv.Atoi(args[1])
	if err != nil {
		return -1
	}

	var role_id int
	var role_ids []int32
	if len(args) > 2 {
		for i := 2; i < len(args); i++ {
			role_id, err = strconv.Atoi(args[i])
			if err != nil {
				return -1
			}
			role_ids = append(role_ids, int32(role_id))
		}
	}

	story := false
	if is_story > 0 {
		story = true
	}

	if role_ids == nil {
		role_ids = p.explore_one_key_sel_role(int32(task_id), story)
	}

	return p.explore_sel_role(int32(task_id), story, role_ids)
}

func explore_start_cmd(p *Player, args []string) int32 {
	if len(args) < 2 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var id, is_story int
	var err error
	id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}
	is_story, err = strconv.Atoi(args[1])
	if err != nil {
		return -1
	}

	story := false
	if is_story > 0 {
		story = true
	}

	return p.explore_task_start([]int32{int32(id)}, story)
}

func explore_reward_cmd(p *Player, args []string) int32 {
	if len(args) < 2 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var id, is_story int
	var err error
	id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}
	is_story, err = strconv.Atoi(args[1])
	if err != nil {
		return -1
	}

	story := false
	if is_story > 0 {
		story = true
	}

	return p.explore_get_reward(int32(id), story)
}

func explore_fight_cmd(p *Player, args []string) int32 {
	if len(args) < 2 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var id, is_story int
	var err error
	id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}
	is_story, err = strconv.Atoi(args[1])
	if err != nil {
		return -1
	}

	story := false
	if is_story > 0 {
		story = true
	}

	return p.explore_fight(int32(id), story)
}

func explore_refresh_cmd(p *Player, args []string) int32 {
	return p.explore_tasks_refresh()
}

func explore_lock_cmd(p *Player, args []string) int32 {
	if len(args) < 2 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var id, lock int
	var err error
	id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}
	lock, err = strconv.Atoi(args[1])
	if err != nil {
		return -1
	}

	is_lock := false
	if lock > 0 {
		is_lock = true
	}

	return p.explore_task_lock([]int32{int32(id)}, is_lock)
}

func explore_speedup_cmd(p *Player, args []string) int32 {
	if len(args) < 2 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var id, is_story int
	var err error
	id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}
	is_story, err = strconv.Atoi(args[1])
	if err != nil {
		return -1
	}

	story := false
	if is_story > 0 {
		story = true
	}

	return p.explore_speedup([]int32{int32(id)}, story)
}

func guild_search_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	return p.guild_search(args[0])
}

func guild_recommend_cmd(p *Player, args []string) int32 {
	return p.guild_recommend()
}

func guild_data_cmd(p *Player, args []string) int32 {
	return p.send_guild_data()
}

func guild_create_cmd(p *Player, args []string) int32 {
	if len(args) < 2 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var name string
	var logo int
	var err error
	name = args[0]
	logo, err = strconv.Atoi(args[1])
	if err != nil {
		return -1
	}

	return p.guild_create(name, int32(logo))
}

func guild_dismiss_cmd(p *Player, args []string) int32 {
	return p.guild_dismiss()
}

func guild_cancel_dismiss_cmd(p *Player, args []string) int32 {
	return p.guild_cancel_dismiss()
}

func guild_modify_cmd(p *Player, args []string) int32 {
	if len(args) < 2 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var name string
	var logo int
	var err error
	name = args[0]
	logo, err = strconv.Atoi(args[1])
	if err != nil {
		return -1
	}

	return p.guild_info_modify(name, int32(logo))
}

func guild_members_cmd(p *Player, args []string) int32 {
	return p.guild_members_list()
}

func guild_ask_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var guild_id int
	var err error
	guild_id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}

	return p.guild_ask_join(int32(guild_id))
}

func guild_agree_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var player_id int
	var err error
	player_id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}

	var is_refuse int
	if len(args) > 1 {
		is_refuse, err = strconv.Atoi(args[1])
		if err != nil {
			return -1
		}
	}

	return p.guild_agree_join([]int32{int32(player_id)}, func() bool {
		if is_refuse > 0 {
			return true
		}
		return false
	}())
}

func guild_ask_list_cmd(p *Player, args []string) int32 {
	return p.guild_ask_list()
}

func guild_quit_cmd(p *Player, args []string) int32 {
	return p.guild_quit()
}

func guild_logs_cmd(p *Player, args []string) int32 {
	return p.guild_logs()
}

func guild_sign_cmd(p *Player, args []string) int32 {
	return p.guild_sign_in()
}

func guild_set_officer_cmd(p *Player, args []string) int32 {
	if len(args) < 2 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var player_id, set_type int
	var err error
	player_id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}
	set_type, err = strconv.Atoi(args[1])
	if err != nil {
		return -1
	}

	return p.guild_set_officer([]int32{int32(player_id)}, int32(set_type))
}

func guild_kick_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var member_id int
	var err error
	member_id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}

	return p.guild_kick_member([]int32{int32(member_id)})
}

func guild_change_president_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var member_id int
	var err error
	member_id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}

	return p.guild_change_president(int32(member_id))
}

func guild_recruit_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	return p.guild_recruit([]byte(args[0]))
}

func guild_donate_list_cmd(p *Player, args []string) int32 {
	return p.guild_donate_list()
}

func guild_ask_donate_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var item_id int
	var err error
	item_id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}

	return p.guild_ask_donate(int32(item_id))
}

func guild_donate_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var player_id int
	var err error
	player_id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}

	return p.guild_donate(int32(player_id))
}

func chat_cmd(p *Player, args []string) int32 {
	if len(args) < 2 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var content []byte
	var channel int
	var err error
	content = []byte(args[0])
	channel, err = strconv.Atoi(args[1])
	if err != nil {
		return -1
	}

	return p.chat(int32(channel), content)
}

func pull_chat_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var channel int
	var err error
	channel, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}

	return p.pull_chat(int32(channel))
}

func guild_stage_data_cmd(p *Player, args []string) int32 {
	return p.send_guild_stage_data(true)
}

func guild_stage_ranklist_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var boss_id int
	var err error
	boss_id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}

	return p.guild_stage_rank_list(int32(boss_id))
}

func guild_stage_fight_cmd(p *Player, args []string) int32 {
	if len(args) < 1 {
		log.Error("参数[%v]不够", len(args))
		return -1
	}

	var boss_id int
	var err error
	boss_id, err = strconv.Atoi(args[0])
	if err != nil {
		return -1
	}

	return p.guild_stage_fight(int32(boss_id))
}

func guild_stage_reset_cmd(p *Player, args []string) int32 {
	return p.guild_stage_reset()
}

func guild_stage_respawn_cmd(p *Player, args []string) int32 {
	return p.guild_stage_player_respawn()
}

func test_short_rank_cmd(p *Player, args []string) int32 {
	var items []*utils.TestShortRankItem = []*utils.TestShortRankItem{
		{Id: 1, Value: 1},
		{Id: 2, Value: 2},
		{Id: 3, Value: 3},
		{Id: 4, Value: 4},
		{Id: 5, Value: 5},
		{Id: 6, Value: 6},
		{Id: 7, Value: 7},
		{Id: 8, Value: 8},
		{Id: 9, Value: 9},
		{Id: 10, Value: 10},
		{Id: 11, Value: 11},
		{Id: 12, Value: 12},
		{Id: 1, Value: 11},
		{Id: 3, Value: 33},
		{Id: 9, Value: 99},
		{Id: 3, Value: 3},
		{Id: 1, Value: 2},
		{Id: 4, Value: -1},
		{Id: 4, Value: 3},
		{Id: 8, Value: 8},
	}

	var rank_list utils.ShortRankList
	rank_list.Init(10)
	for _, item := range items {
		rank_list.Update(item, true)
	}

	log.Debug("Test Short Rank Item List:")
	for r := int32(1); r <= rank_list.GetLength(); r++ {
		k, v := rank_list.GetByRank(r)
		idx := rank_list.GetIndex(r)
		log.Debug("    rank: %v,  key[%v] value[%v] index[%v]", r, k, v, idx)
	}

	return 1
}

type test_cmd_func func(*Player, []string) int32

var test_cmd2funcs = map[string]test_cmd_func{
	"set_level":              set_level_cmd,
	"test_lua":               test_lua_cmd,
	"rand_role":              rand_role_cmd,
	"new_role":               new_role_cmd,
	"all_roles":              all_roles_cmd,
	"list_role":              list_role_cmd,
	"set_attack_team":        set_attack_team_cmd,
	"set_defense_team":       set_defense_team_cmd,
	"list_teams":             list_teams_cmd,
	"pvp":                    pvp_cmd,
	"fight_stage":            fight_stage_cmd,
	"fight_campaign":         fight_campaign_cmd,
	"start_hangup":           start_hangup_cmd,
	"hangup_income":          hangup_income_cmd,
	"campaign_data":          campaign_data_cmd,
	"leave_game":             leave_game_cmd,
	"add_item":               add_item_cmd,
	"all_items":              all_items_cmd,
	"clear_items":            clear_items_cmd,
	"role_levelup":           role_levelup_cmd,
	"role_rankup":            role_rankup_cmd,
	"role_decompose":         role_decompose_cmd,
	"item_fusion":            item_fusion_cmd,
	"fusion_role":            fusion_role_cmd,
	"item_sell":              item_sell_cmd,
	"send_mail":              send_mail_cmd,
	"mail_list":              mail_list_cmd,
	"mail_detail":            mail_detail_cmd,
	"mail_items":             mail_items_cmd,
	"delete_mail":            delete_mail_cmd,
	"talent_data":            talent_data_cmd,
	"talent_up":              up_talent_cmd,
	"talent_reset":           talent_reset_cmd,
	"tower_data":             tower_data_cmd,
	"get_tower_key":          get_tower_key_cmd,
	"fight_tower":            fight_tower_cmd,
	"tower_records_info":     tower_records_info_cmd,
	"tower_record_data":      tower_record_data_cmd,
	"tower_ranklist":         tower_ranklist_cmd,
	"battle_recordlist":      battle_recordlist_cmd,
	"battle_record":          battle_record_cmd,
	"test_stw":               test_stw_cmd,
	"item_upgrade":           item_upgrade_cmd,
	"role_item_up":           role_item_upgrade_cmd,
	"equip_item":             equip_item_cmd,
	"unequip_item":           unequip_item_cmd,
	"list_item":              list_items_cmd,
	"role_attrs":             get_role_attrs_cmd,
	"onekey_equip":           onekey_equip_cmd,
	"onekey_unequip":         onekey_unequip_cmd,
	"left_slot_open":         left_slot_open_cmd,
	"role_equips":            role_equips_cmd,
	"draw_card":              draw_card_cmd,
	"draw_data":              draw_data_cmd,
	"shop_data":              shop_data_cmd,
	"buy_item":               buy_item_cmd,
	"shop_refresh":           shop_refresh_cmd,
	"arena_ranklist":         arena_ranklist_cmd,
	"arena_data":             arena_data_cmd,
	"arena_player_team":      arena_player_team_cmd,
	"arena_match":            arena_match_cmd,
	"arena_reset":            arena_reset_cmd,
	"rank_list":              rank_list_cmd,
	"player_info":            player_info_cmd,
	"item_onekey_upgrade":    item_onekey_upgrade_cmd,
	"active_stage_data":      active_stage_data_cmd,
	"active_stage_buy":       active_stage_buy_cmd,
	"fight_active_stage":     fight_active_stage_cmd,
	"friend_recommend":       friend_recommend_cmd,
	"friend_data":            friend_data_cmd,
	"friend_ask":             friend_ask_cmd,
	"friend_agree":           friend_agree_cmd,
	"friend_refuse":          friend_refuse_cmd,
	"friend_remove":          friend_remove_cmd,
	"friend_list":            friend_list_cmd,
	"friend_ask_list":        friend_ask_list_cmd,
	"friend_give_points":     friend_give_points_cmd,
	"friend_get_points":      friend_get_points_cmd,
	"friend_search_boss":     friend_search_boss_cmd,
	"friend_boss_list":       friend_boss_list_cmd,
	"friend_boss_attacks":    friend_boss_attacks_cmd,
	"friend_fight":           friend_fight_cmd,
	"assist_list":            assist_list_cmd,
	"set_assist":             friend_set_assist_cmd,
	"use_assist":             use_assist_cmd,
	"task_data":              task_data_cmd,
	"task_reward":            task_reward_cmd,
	"explore_data":           explore_data_cmd,
	"explore_sel_role":       explore_sel_role_cmd,
	"explore_start":          explore_start_cmd,
	"explore_reward":         explore_reward_cmd,
	"explore_fight":          explore_fight_cmd,
	"explore_refresh":        explore_refresh_cmd,
	"explore_lock":           explore_lock_cmd,
	"explore_speedup":        explore_speedup_cmd,
	"guild_search":           guild_search_cmd,
	"guild_recommend":        guild_recommend_cmd,
	"guild_data":             guild_data_cmd,
	"guild_create":           guild_create_cmd,
	"guild_dismiss":          guild_dismiss_cmd,
	"guild_cancel_dismiss":   guild_cancel_dismiss_cmd,
	"guild_modify":           guild_modify_cmd,
	"guild_members":          guild_members_cmd,
	"guild_ask":              guild_ask_cmd,
	"guild_agree":            guild_agree_cmd,
	"guild_ask_list":         guild_ask_list_cmd,
	"guild_quit":             guild_quit_cmd,
	"guild_logs":             guild_logs_cmd,
	"guild_sign":             guild_sign_cmd,
	"guild_kick":             guild_kick_cmd,
	"guild_set_officer":      guild_set_officer_cmd,
	"guild_change_president": guild_change_president_cmd,
	"guild_recruit":          guild_recruit_cmd,
	"guild_donate_list":      guild_donate_list_cmd,
	"guild_ask_donate":       guild_ask_donate_cmd,
	"guild_donate":           guild_donate_cmd,
	"chat":                   chat_cmd,
	"pull_chat":              pull_chat_cmd,
	"guild_stage_data":       guild_stage_data_cmd,
	"guild_stage_ranklist":   guild_stage_ranklist_cmd,
	"guild_stage_fight":      guild_stage_fight_cmd,
	"guild_stage_reset":      guild_stage_reset_cmd,
	"guild_stage_respawn":    guild_stage_respawn_cmd,
	"test_short_rank":        test_short_rank_cmd,
}

func C2STestCommandHandler(w http.ResponseWriter, r *http.Request, p *Player /*msg proto.Message*/, msg_data []byte) int32 {
	var req msg_client_message.C2S_TEST_COMMAND
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("client_msg_handler unmarshal sub msg failed err(%s) !", err.Error())
		return -1
	}

	cmd := req.GetCmd()
	args := req.GetArgs()
	res := int32(0)

	fun := test_cmd2funcs[cmd]
	if fun != nil {
		res = fun(p, args)
	} else {
		log.Warn("不支持的测试命令[%v]", cmd)
	}

	return res
}
