package main

import (
	"ih_server/libs/log"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
)

const (
	PLAYER_DATA_UP_FRAME_COUNT = 30
	PLAYER_CAMP_1              = 1 // 玩家阵营1
	PLAYER_CAMP_2              = 2 // 玩家阵营2
	DEFAULT_PLAYER_ARRAY_MAX   = 1
	PLAYER_ARRAY_MAX_ADD_STEP  = 1
)

type PlayerManager struct {
	id2players      map[int32]*Player
	id2players_lock *sync.RWMutex

	acc2players      map[string]*Player
	acc2Players_lock *sync.RWMutex

	all_player_array []*Player
	cur_all_count    int32
	cur_all_max      int32

	ol_player_array []*Player
	cur_ol_count    int32
	cur_ol_max      int32
}

var player_mgr PlayerManager

func (this *PlayerManager) Init() bool {
	this.id2players = make(map[int32]*Player)
	this.id2players_lock = &sync.RWMutex{}
	this.acc2players = make(map[string]*Player)
	this.acc2Players_lock = &sync.RWMutex{}

	this.ol_player_array = make([]*Player, DEFAULT_PLAYER_ARRAY_MAX)
	this.cur_ol_count = 0
	this.cur_ol_max = DEFAULT_PLAYER_ARRAY_MAX

	this.all_player_array = make([]*Player, DEFAULT_PLAYER_ARRAY_MAX)
	this.cur_all_count = 0
	this.cur_all_max = DEFAULT_PLAYER_ARRAY_MAX

	return true
}

func (this *PlayerManager) GetPlayerById(id int32) *Player {
	this.id2players_lock.Lock()
	defer this.id2players_lock.Unlock()

	return this.id2players[id]
}

func (this *PlayerManager) GetAllPlayers() []*Player {
	this.id2players_lock.RLock()
	defer this.id2players_lock.RUnlock()

	ret_ps := make([]*Player, 0, len(this.id2players))
	for _, p := range this.id2players {
		ret_ps = append(ret_ps, p)
	}

	return ret_ps
}

func (this *PlayerManager) Add2IdMap(p *Player) {
	if nil == p {
		log.Error("Player_agent_mgr Add2IdMap p nil !")
		return
	}
	this.id2players_lock.Lock()
	defer this.id2players_lock.Unlock()

	if nil != this.id2players[p.Id] {
		log.Error("PlayerManager Add2IdMap already have player(%d)", p.Id)
	}

	this.id2players[p.Id] = p

	if this.cur_all_count >= this.cur_all_max {
		this.cur_all_max = this.cur_all_max + PLAYER_ARRAY_MAX_ADD_STEP
		new_all_array := make([]*Player, this.cur_all_max)
		for idx := int32(0); idx < this.cur_all_count; idx++ {
			new_all_array[idx] = this.all_player_array[idx]
		}

		this.all_player_array = new_all_array
	}

	this.all_player_array[this.cur_all_count] = p
	p.all_array_idx = this.cur_all_count
	this.cur_all_count++

	return
}

func (this *PlayerManager) RemoveFromIdMap(id int32) {
	this.id2players_lock.Lock()
	defer this.id2players_lock.Unlock()

	cur_p := this.id2players[id]
	if nil != cur_p {
		delete(this.id2players, id)
	}

	if -1 != cur_p.all_array_idx {
		if cur_p.all_array_idx != this.cur_all_count-1 {
			this.all_player_array[cur_p.all_array_idx] = this.all_player_array[this.cur_all_count-1]
			this.all_player_array[cur_p.all_array_idx].all_array_idx = cur_p.all_array_idx
		}
		this.cur_all_count--
	}

	return
}

func (this *PlayerManager) GetAllPlayerNum() int32 {
	return this.cur_all_count
}

func (this *PlayerManager) Add2AccMap(p *Player) {
	if nil == p {
		log.Error("PlayerManager Add2AccMap p nil !")
		return
	}

	this.acc2Players_lock.RLock()
	defer this.acc2Players_lock.RUnlock()
	if nil != this.acc2players[p.Account] {
		log.Info("PlayerManager Add2AccMap old_p not nil")
		return
	}

	this.acc2players[p.Account] = p

	if this.cur_ol_count >= this.cur_ol_max {
		tmp_player_array := make([]*Player, this.cur_ol_max+PLAYER_ARRAY_MAX_ADD_STEP)
		for idx := int32(0); idx < this.cur_ol_max; idx++ {
			tmp_player_array[idx] = this.ol_player_array[idx]
		}

		this.cur_ol_max = this.cur_ol_count + PLAYER_ARRAY_MAX_ADD_STEP
		this.ol_player_array = tmp_player_array
	}

	this.ol_player_array[this.cur_ol_count] = p
	p.ol_array_idx = this.cur_ol_count
	this.cur_ol_count++

	return
}

func (this *PlayerManager) RemoveFromAccMap(acc string) {
	if "" == acc {
		log.Error("PlayerManager RemoveFromAccMap acc empty !")
		return
	}

	this.acc2Players_lock.Lock()
	defer this.acc2Players_lock.Unlock()
	cur_p := this.acc2players[acc]
	if nil != cur_p {
		if cur_p.ol_array_idx != -1 {
			if cur_p.ol_array_idx != this.cur_ol_count-1 {
				if nil != this.ol_player_array[this.cur_ol_count-1] {
					this.ol_player_array[this.cur_ol_count-1].ol_array_idx = cur_p.ol_array_idx
				}
				this.ol_player_array[cur_p.ol_array_idx] = this.ol_player_array[this.cur_ol_count-1]
			}
			this.cur_ol_count = this.cur_ol_count - 1
		}
		delete(this.acc2players, acc)
	}

	return
}

func (this *PlayerManager) GetCurOnlineNum() int32 {
	return this.cur_ol_count
}

func (this *PlayerManager) GetPlayerByAcc(acc string) *Player {
	if "" == acc {
		return nil
	}

	this.acc2Players_lock.Lock()
	defer this.acc2Players_lock.Unlock()

	return this.acc2players[acc]
}

func (this *PlayerManager) PlayerLogout(p *Player) {
	if nil == p {
		log.Error("PlayerManager PlayerLogout p nil !")
		return
	}

	this.RemoveFromAccMap(p.Account)

	p.OnLogout()
}

func (this *PlayerManager) OnTick() {

}

func (this *PlayerManager) SendMsgToAllPlayers(msg proto.Message) {
	if nil == msg {
		log.Error("PlayerManager SendMsgToAllPlayers msg nil !")
		return
	}
}

//==============================================================================
func (this *PlayerManager) RegMsgHandler() {
	msg_handler_mgr.SetMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ENTER_GAME_REQUEST), C2SEnterGameRequestHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_LEAVE_GAME_REQUEST), C2SLeaveGameRequestHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_TEST_COMMAND), C2STestCommandHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_HEARTBEAT), C2SHeartbeatHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_DATA_SYNC_REQUEST), C2SDataSyncHandler)

	// 战役
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_BATTLE_RESULT_REQUEST), C2SFightHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_SET_TEAM_REQUEST), C2SSetTeamHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_BATTLE_SET_HANGUP_CAMPAIGN_REQUEST), C2SSetHangupCampaignHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_CAMPAIGN_HANGUP_INCOME_REQUEST), C2SCampaignHangupIncomeHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_CAMPAIGN_DATA_REQUEST), C2SCampaignDataHandler)

	// 角色
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ROLE_ATTRS_REQUEST), C2SRoleAttrsHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ROLE_LEVELUP_REQUEST), C2SRoleLevelUpHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ROLE_RANKUP_REQUEST), C2SRoleRankUpHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ROLE_DECOMPOSE_REQUEST), C2SRoleDecomposeHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ROLE_FUSION_REQUEST), C2SRoleFusionHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ROLE_LOCK_REQUEST), C2SRoleLockHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ROLE_HANDBOOK_REQUEST), C2SRoleHandbookHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ROLE_LEFTSLOT_OPEN_REQUEST), C2SRoleLeftSlotOpenHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ROLE_ONEKEY_EQUIP_REQUEST), C2SRoleOneKeyEquipHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ROLE_ONEKEY_UNEQUIP_REQUEST), C2SRoleOneKeyUnequipHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ROLE_LEFTSLOT_RESULT_SAVE_REQUEST), C2SRoleLeftSlotUpgradeSaveHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ROLE_LEFTSLOT_RESULT_CANCEL_REQUEST), C2SRoleLeftSlotResultCancelHandler)

	// 物品
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ITEM_FUSION_REQUEST), C2SItemFusionHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ITEM_SELL_REQUEST), C2SItemSellHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ITEM_EQUIP_REQUEST), C2SItemEquipHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ITEM_UNEQUIP_REQUEST), C2SItemUnequipHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ITEM_UPGRADE_REQUEST), C2SItemUpgradeHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ITEM_ONEKEY_UPGRADE_REQUEST), C2SItemOneKeyUpgradeHandler)

	// 邮件
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_MAIL_SEND_REQUEST), C2SMailSendHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_MAIL_LIST_REQUEST), C2SMailListHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_MAIL_DETAIL_REQUEST), C2SMailDetailHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_MAIL_GET_ATTACHED_ITEMS_REQUEST), C2SMailGetAttachedItemsHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_MAIL_DELETE_REQUEST), C2SMailDeleteHandler)

	// 录像
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_BATTLE_RECORD_LIST_REQUEST), C2SBattleRecordListHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_BATTLE_RECORD_REQUEST), C2SBattleRecordHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_BATTLE_RECORD_DELETE_REQUEST), C2SBattleRecordDeleteHandler)

	// 天赋
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_TALENT_UP_REQUEST), C2STalentListHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_TALENT_LIST_REQUEST), C2STalentListHandler)

	// 爬塔
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_TOWER_DATA_REQUEST), C2STowerDataHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_TOWER_RECORDS_INFO_REQUEST), C2STowerRecordsInfoHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_TOWER_RECORD_DATA_REQUEST), C2STowerRecordDataHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_TOWER_RANKING_LIST_REQUEST), C2STowerRankingListHandler)

	// 抽卡
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_DRAW_CARD_REQUEST), C2SDrawCardHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_DRAW_DATA_REQUEST), C2SDrawDataHandler)

	// 点金手
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GOLD_HAND_DATA_REQUEST), C2SGoldHandDataHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_TOUCH_GOLD_REQUEST), C2STouchGoldHandler)

	// 商店
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_SHOP_DATA_REQUEST), C2SShopDataHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_SHOP_BUY_ITEM_REQUEST), C2SShopBuyItemHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_SHOP_REFRESH_REQUEST), C2SShopRefreshHandler)

	// 竞技场
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ARENA_DATA_REQUEST), C2SArenaDataHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ARENA_PLAYER_DEFENSE_TEAM_REQUEST), C2SArenaPlayerDefenseTeamHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ARENA_MATCH_PLAYER_REQUEST), C2SArenaMatchPlayerHandler)

	// 排行榜
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_RANK_LIST_REQUEST), C2SRankListHandler)

	// 活动副本
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ACTIVE_STAGE_DATA_REQUEST), C2SActiveStageDataHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ACTIVE_STAGE_BUY_CHALLENGE_NUM_REQUEST), C2SActiveStageBuyChallengeNumHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_ACTIVE_STAGE_ASSIST_ROLE_LIST_REQUEST), C2SActiveStageGetAssistRoleListHandler)

	// 好友
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_FRIEND_RECOMMEND_REQUEST), C2SFriendsRecommendHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_FRIEND_LIST_REQUEST), C2SFriendListHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_FRIEND_ASK_PLAYER_LIST_REQUEST), C2SFriendAskListHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_FRIEND_ASK_REQUEST), C2SFriendAskHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_FRIEND_AGREE_REQUEST), C2SFriendAgreeHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_FRIEND_REFUSE_REQUEST), C2SFriendRefuseHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_FRIEND_REMOVE_REQUEST), C2SFriendRemoveHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_FRIEND_GIVE_POINTS_REQUEST), C2SFriendGivePointsHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_FRIEND_GET_POINTS_REQUEST), C2SFriendGetPointsHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_FRIEND_SEARCH_BOSS_REQUEST), C2SFriendSearchBossHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_FRIENDS_BOSS_LIST_REQUEST), C2SFriendGetBossListHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_FRIEND_BOSS_ATTACK_LIST_REQUEST), C2SFriendBossAttackListHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_FRIEND_DATA_REQUEST), C2SFriendDataHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_FRIEND_SET_ASSIST_ROLE_REQUEST), C2SFriendSetAssistRoleHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_FRIEND_GIVE_AND_GET_POINTS_REQUEST), C2SFriendGiveAndGetPointsHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_FRIEND_GET_ASSIST_POINTS_REQUEST), C2SFriendGetAssistPointsHandler)

	// 任务
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_TASK_DATA_REQUEST), C2STaskDataHanlder)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_TASK_REWARD_REQUEST), C2SGetTaskRewardHandler)

	// 探索
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_EXPLORE_DATA_REQUEST), C2SExploreDataHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_EXPLORE_SEL_ROLE_REQUEST), C2SExploreSelRoleHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_EXPLORE_START_REQUEST), C2SExploreStartHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_EXPLORE_SPEEDUP_REQUEST), C2SExploreSpeedupHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_EXPLORE_REFRESH_REQUEST), C2SExploreTasksRefreshHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_EXPLORE_LOCK_REQUEST), C2SExploreTaskLockHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_EXPLORE_GET_REWARD_REQUEST), C2SExploreGetRewardHandler)

	// 聊天
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_CHAT_REQUEST), C2SChatHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_CHAT_MSG_PULL_REQUEST), C2SChatPullMsgHandler)

	// 公会
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_DATA_REQUEST), C2SGuildDataHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_RECOMMEND_REQUEST), C2SGuildRecommendHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_SEARCH_REQUEST), C2SGuildSearchHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_DISMISS_REQUEST), C2SGuildDismissHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_CREATE_REQUEST), C2SGuildCreateHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_CANCEL_DISMISS_REQUEST), C2SGuildCancelDismissHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_INFO_MODIFY_REQUEST), C2SGuildInfoModifyHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_MEMBERS_REQUEST), C2SGuildMembersHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_ASK_JOIN_REQUEST), C2SGuildAskJoinHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_AGREE_JOIN_REQUEST), C2SGuildAgreeJoinHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_ASK_LIST_REQUEST), C2SGuildAskListHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_QUIT_REQUEST), C2SGuildQuitHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_LOGS_REQUEST), C2SGuildLogsHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_SIGN_IN_REQUEST), C2SGuildSignInHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_SET_OFFICER_REQUEST), C2SGuildSetOfficerHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_KICK_MEMBER_REQUEST), C2SGuildKickMemberHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_CHANGE_PRESIDENT_REQUEST), C2SGuildChangePresidentHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_RECRUIT_REQUEST), C2SGuildRecruitHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_DONATE_LIST_REQUEST), C2SGuildDonateListHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_ASK_DONATE_REQUEST), C2SGuildAskDonateHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_DONATE_REQUEST), C2SGuildDonateHandler)

	// 公会副本
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_STAGE_DATA_REQUEST), C2SGuildStageDataHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_STAGE_RANK_LIST_REQUEST), C2SGuildStageRankListHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_STAGE_RESET_REQUEST), C2SGuildStageResetHandler)
	msg_handler_mgr.SetPlayerMsgHandler(uint16(msg_client_message_id.MSGID_C2S_GUILD_STAGE_PLAYER_RESPAWN_REQUEST), C2SGuildStagePlayerRespawnHandler)
}

func C2SEnterGameRequestHandler(w http.ResponseWriter, r *http.Request, msg_data []byte) (int32, *Player) {
	var p *Player
	var req msg_client_message.C2SEnterGameRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s) !", err.Error())
		return -1, p
	}

	acc := req.GetAcc()
	if "" == acc {
		log.Error("PlayerEnterGameHandler acc empty !")
		return -1, p
	}

	token_info := login_token_mgr.GetTokenByAcc(acc)
	if nil == token_info {
		log.Error("PlayerEnterGameHandler account[%v] no token info!", acc)
		return -2, p
	}

	if req.GetToken() != token_info.token {
		log.Error("PlayerEnterGameHandler token check failed !(%s) != (%s)", req.GetToken(), token_info.token)
		return -3, p
	}

	//p = player_mgr.GetPlayerById(player_id)
	is_new := false
	p = player_mgr.GetPlayerByAcc(acc)
	if nil == p {
		//pdb := dbc.Players.GetRow(p.Id)
		//if nil == pdb {
		global_row := dbc.Global.GetRow()
		player_id := global_row.GetNextPlayerId()
		pdb := dbc.Players.AddRow(player_id)
		if nil == pdb {
			log.Error("player_db_to_msg AddRow pid(%d) failed !", player_id)
			return -4, p
		}
		pdb.SetAccount(token_info.acc)
		pdb.SetCurrReplyMsgNum(0)
		p = new_player(player_id, token_info.acc, token_info.token, pdb)
		p.OnCreate()
		//} else {
		//	p = new_player(p.Id, token_info.acc, token_info.token, pdb)
		//}
		player_mgr.Add2AccMap(p)
		player_mgr.Add2IdMap(p)
		is_new = true
		log.Info("player_db_to_msg new player(%d) !", player_id)
	} else {
		p.Account = token_info.acc
		p.Token = token_info.token
		pdb := dbc.Players.GetRow(p.Id)
		if pdb != nil {
			pdb.SetCurrReplyMsgNum(0)
		}
	}

	ip_port := strings.Split(r.RemoteAddr, ":")
	if len(ip_port) >= 2 {
		p.pos = position_table.GetPosByIP(ip_port[0])
	}

	p.bhandling = true

	p.send_enter_game(acc, p.Id)
	p.OnLogin()
	if !is_new {
		p.send_items()
		p.send_roles()
	} else {
		p.check_and_send_items_change()
		p.check_and_send_roles_change()
	}
	p.send_info()
	p.send_teams()
	p.notify_enter_complete()

	log.Info("PlayerEnterGameHandler account[%s] token[%s]", req.GetAcc(), req.GetToken())

	return 1, p
}

func C2SLeaveGameRequestHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SLeaveGameRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s) !", err.Error())
		return -1
	}
	p.OnLogout()
	return 1
}

func C2SHeartbeatHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SHeartbeat
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s) !", err.Error())
		return -1
	}

	if p.IsOffline() {
		log.Error("Player[%v] is offline", p.Id)
		return int32(msg_client_message.E_ERR_PLAYER_IS_OFFLINE)
	}

	p.send_notify_state()
	p.check_and_send_tower_data()
	p.check_and_send_friend_ask_add()
	p.check_and_send_friend_add()

	response := &msg_client_message.S2CHeartbeat{
		SysTime: int32(time.Now().Unix()),
	}
	p.Send(uint16(msg_client_message_id.MSGID_S2C_HEARTBEAT), response)

	return 1
}

func C2SDataSyncHandler(w http.ResponseWriter, r *http.Request, p *Player, msg_data []byte) int32 {
	var req msg_client_message.C2SDataSyncRequest
	err := proto.Unmarshal(msg_data, &req)
	if err != nil {
		log.Error("Unmarshal msg failed err(%s)!", err.Error())
		return -1
	}
	if req.Base {
		p.send_info()
	}
	if req.Items {
		p.send_items()
	}
	if req.Roles {
		p.send_roles()
	}
	if req.Teams {
		p.send_teams()
	}
	if req.Campaigns {
		p.send_campaigns()
	}
	return 1
}
