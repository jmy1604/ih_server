package main

import (
	"errors"
	"fmt"
	"ih_server/libs/log"
	"ih_server/proto/gen_go/client_message"
	"ih_server/src/rpc_common"
	"time"
)

// GM调用
type G2H_Proc struct {
}

func (this *G2H_Proc) Test(args *rpc_common.GmTestCmd, result *rpc_common.GmCommonResponse) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	result.Res = 1

	log.Trace("@@@ G2H_Proc::Test %v", args)
	return nil
}

func (this *G2H_Proc) Anouncement(args *rpc_common.GmAnouncementCmd, result *rpc_common.GmCommonResponse) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	if !system_chat_mgr.push_chat_msg(args.Content, args.RemainSeconds, 0, 0, "", 0) {
		err_str := fmt.Sprintf("@@@ G2H_Proc::Anouncement %v failed", args)
		return errors.New(err_str)
	}

	result.Res = 1

	log.Trace("@@@ G2H_Proc::Anouncement %v", args)
	return nil
}

func (this *G2H_Proc) SysMail(args *rpc_common.GmSendSysMailCmd, result *rpc_common.GmCommonResponse) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	// 群发邮件
	if args.PlayerId <= 0 {
		row := dbc.SysMails.AddRow()
		if row == nil {
			log.Error("@@@ G2H_Proc::SysMail add new db row failed")
			result.Res = -1
		}
		result.Res = mail_has_subtype(args.MailTableID)
		if result.Res > 0 {
			row.SetTableId(args.MailTableID)
			row.AttachedItems.SetItemList(args.AttachItems)
			row.SetSendTime(int32(time.Now().Unix()))
			dbc.SysMailCommon.GetRow().SetCurrMailId(row.GetId())
		}
	} else {
		result.Res = RealSendMail(nil, args.PlayerId, MAIL_TYPE_SYSTEM, args.MailTableID, "", "", args.AttachItems, 0)
	}

	log.Trace("@@@ G2H_Proc::SysMail %v", args)
	return nil
}

func (this *G2H_Proc) PlayerInfo(args *rpc_common.GmPlayerInfoCmd, result *rpc_common.GmPlayerInfoResponse) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	p := player_mgr.GetPlayerById(args.Id)
	if p == nil {
		result.Id = int32(msg_client_message.E_ERR_PLAYER_NOT_EXIST)
		return nil
	}

	result.Id = args.Id
	result.Account = p.db.GetAccount()
	result.UniqueId = p.db.GetUniqueId()
	result.Level = p.db.Info.GetLvl()
	result.VipLevel = p.db.Info.GetVipLvl()
	result.Gold = p.db.Info.GetGold()
	result.Diamond = p.db.Info.GetDiamond()
	result.GuildId = p.db.Guild.GetId()
	if result.GuildId > 0 {
		guild := guild_manager.GetGuild(result.GuildId)
		result.GuildName = guild.GetName()
		result.GuildLevel = guild.GetLevel()
	}
	result.UnlockCampaignId = p.db.CampaignCommon.GetCurrentCampaignId()
	result.HungupCampaignId = p.db.CampaignCommon.GetHangupCampaignId()
	result.ArenaScore = p.db.Arena.GetScore()
	talents := p.db.Talents.GetAllIndex()
	if talents != nil {
		for i := 0; i < len(talents); i++ {
			lvl, _ := p.db.Talents.GetLevel(talents[i])
			result.TalentList = append(result.TalentList, []int32{talents[i], lvl}...)
		}
	}
	result.TowerId = p.db.TowerCommon.GetCurrId()
	result.SignIn = p.db.Sign.GetSignedIndex()

	log.Trace("@@@ G2H_Proc::PlayerInfo %v %v", args, result)

	return nil
}

func (this *G2H_Proc) OnlinePlayerNum(args *rpc_common.GmOnlinePlayerNumCmd, result *rpc_common.GmOnlinePlayerNumResponse) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	result.PlayerNum = []int32{conn_timer_wheel.GetCurrPlayerNum()}

	log.Trace("@@@ G2H_Proc::OnlinePlayerNum")

	return nil
}

func (this *G2H_Proc) MonthCardSend(args *rpc_common.GmMonthCardSendCmd, result *rpc_common.GmCommonResponse) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	cards := pay_table_mgr.GetMonthCards()
	if cards == nil || len(cards) == 0 {
		log.Error("@@@ month cards is empty")
		result.Res = -1
		return nil
	}

	var found bool
	for i := 0; i < len(cards); i++ {
		if cards[i].BundleId == args.BundleId {
			found = true
			break
		}
	}

	if !found {
		log.Error("@@@ Not found month card with bundle id %v", args.BundleId)
		result.Res = -1
		return nil
	}

	p := player_mgr.GetPlayerById(args.PlayerId)
	if p == nil {
		log.Error("@@@ Month card send cant found player %v", args.PlayerId)
		result.Res = int32(msg_client_message.E_ERR_PLAYER_NOT_EXIST)
		return nil
	}

	res, _ := p._charge_with_bundle_id(0, args.BundleId, nil, nil, -1)
	if res < 0 {
		log.Error("@@@ Month card send with error %v", res)
		result.Res = res
		return nil
	}

	log.Trace("@@@ G2H_Proc::MonthCardSend %v", args)

	return nil
}

func (this *G2H_Proc) GetPlayerUniqueId(args *rpc_common.GmGetPlayerUniqueIdCmd, result *rpc_common.GmGetPlayerUniqueIdResponse) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	if args.PlayerId > 0 {
		p := player_mgr.GetPlayerById(args.PlayerId)
		if p == nil {
			result.PlayerUniqueId = "Cant found player"
			log.Error("@@@ Get player %v cant found", args.PlayerId)
			return nil
		}

		result.PlayerUniqueId = p.db.GetUniqueId()
	}

	log.Trace("@@@ G2H_Proc::GetPlayerUniqueId %v", args)

	return nil
}

func (this *G2H_Proc) BanPlayer(args *rpc_common.GmBanPlayerByUniqueIdCmd, result *rpc_common.GmCommonResponse) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	p := player_mgr.GetPlayerByUid(args.PlayerUniqueId)
	if p == nil {
		result.Res = int32(msg_client_message.E_ERR_PLAYER_NOT_EXIST)
		log.Error("@@@ Player cant get by unique id %v", args.PlayerUniqueId)
		return nil
	}

	row := dbc.BanPlayers.GetRow(args.PlayerUniqueId)
	if args.BanOrFree > 0 {
		now_time := time.Now()
		if row == nil {
			row = dbc.BanPlayers.AddRow(args.PlayerUniqueId)
			row.SetAccount(p.db.GetAccount())
			row.SetPlayerId(p.db.GetPlayerId())
		}
		row.SetStartTime(int32(now_time.Unix()))
		row.SetStartTimeStr(now_time.String())
	} else {
		if row != nil {
			row.SetStartTime(0)
			row.SetStartTimeStr("")
		}
	}

	log.Trace("@@@ G2H_Proc::BanPlayer %v", args)

	return nil
}
