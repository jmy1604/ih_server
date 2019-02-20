package main

import (
	"bytes"
	"compress/zlib"
	"crypto/tls"
	"ih_server/libs/log"
	"ih_server/proto/gen_go/client_message"
	"ih_server/proto/gen_go/client_message_id"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/golang/protobuf/proto"
	"github.com/golang/snappy"
)

const (
	HALL_CONN_STATE_DISCONNECT  = 0
	HALL_CONN_STATE_CONNECTED   = 1
	HALL_CONN_STATE_FORCE_CLOSE = 2
)

// ========================================================================================

type HallConnection struct {
	use_https      bool
	state          int32
	last_conn_time int32
	acc            string
	token          string
	hall_ip        string
	playerid       int32

	blogin bool

	last_send_time int64
	server_id      int32
}

var hall_conn HallConnection

func new_hall_connect(hall_ip, acc, token string, use_https bool) *HallConnection {
	ret_conn := &HallConnection{}
	ret_conn.acc = acc
	ret_conn.hall_ip = hall_ip
	ret_conn.token = token
	ret_conn.use_https = use_https

	log.Info("new hall connection to ip %v", hall_ip)

	return ret_conn
}

type ResponseData struct {
	CompressType int32
	Data         []byte
}

func (this *HallConnection) Send(msg_id uint16, msg proto.Message) {
	data, err := proto.Marshal(msg)
	if nil != err {
		log.Error("login unmarshal failed err[%s]", err.Error())
		return
	}

	C2S_MSG := &msg_client_message.C2S_MSG_DATA{}
	C2S_MSG.Token = this.token
	one_msg := &msg_client_message.C2S_ONE_MSG{
		MsgCode: int32(msg_id),
		Data:    data,
	}
	C2S_MSG.MsgList = []*msg_client_message.C2S_ONE_MSG{one_msg}

	data, err = proto.Marshal(C2S_MSG)
	if nil != err {
		log.Error("login C2S_MSG Marshal err(%s) !", err.Error())
		return
	}

	var resp *http.Response
	if this.use_https {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: tr}
		resp, err = client.Post(this.hall_ip+"/client_msg", "application/x-www-form-urlencoded", bytes.NewReader(data))
	} else {
		resp, err = http.Post(this.hall_ip+"/client_msg", "application/x-www-form-urlencoded", bytes.NewReader(data))
	}
	if nil != err {
		log.Error("login C2S_MSG http post[%s] error[%s]", this.hall_ip+"/client_msg", err.Error())
		return
	}

	defer resp.Body.Close()

	data, err = ioutil.ReadAll(resp.Body)
	if nil != err {
		log.Error("HallConnection Send read resp body err [%s]", err.Error())
		return
	}

	//log.Info("接收到的二进制流 长度[%v] 数据[%v]", len(data), data)
	if len(data) < 1 {
		log.Error("data length %v invalid", len(data))
		return
	}

	var d []byte
	compress_type := int32(data[len(data)-1])
	if len(data) == 1 {
		d = []byte{}
	} else {
		d = data[:len(data)-1]
		if compress_type != 0 {
			if compress_type == 1 {
				in := bytes.NewBuffer(d)
				r, e := zlib.NewReader(in)
				if e != nil {
					log.Error("Zlib New Reader err %v", e.Error())
					return
				}
				var out bytes.Buffer
				io.Copy(&out, r)
				d = out.Bytes()
			} else if compress_type == 2 {
				d, err = snappy.Decode(nil, d)
				if err != nil {
					log.Error("Snappy decode %v err %v", d, err.Error())
					return
				}
			} else {
				log.Error("Compress type %v not supported", compress_type)
				return
			}
		}
	}

	S2C_MSG := &msg_client_message.S2C_MSG_DATA{}
	err = proto.Unmarshal(d, S2C_MSG)
	if nil != err {
		log.Error("HallConnection unmarshal resp data err(%s) !", err.Error())
		return
	}

	if S2C_MSG.MsgList == nil {
		log.Warn("Server return empty msg list !!!")
		return
	}

	for _, m := range S2C_MSG.MsgList {
		if m.GetErrorCode() < 0 {
			log.Error("服务器返回错误码[%d]", m.GetErrorCode())
			return
		}

		var msg_code uint16
		var cur_len, sub_len int32
		total_data_len := int32(len(m.Data))
		for cur_len < total_data_len {
			msg_code = uint16(m.Data[cur_len])<<8 + uint16(m.Data[cur_len+1])
			sub_len = int32(m.Data[cur_len+2])<<8 + int32(m.Data[cur_len+3])
			sub_data := m.Data[cur_len+4 : cur_len+4+sub_len]
			cur_len = cur_len + 4 + sub_len

			handler_info := msg_handler_mgr.msgid2handler[int32(msg_code)]
			if nil == handler_info {
				continue
			}

			new_msg := hall_conn_msgid2msg(msg_code)
			if new_msg == nil {
				return
			}
			log.Trace("玩家[%d:%s]收到服务器返回%v:[%s]", this.playerid, this.acc, msg_code, new_msg.String())
			err = proto.Unmarshal(sub_data, new_msg)
			if nil != err {
				log.Error("HallConnection failed unmarshal msg data !", msg_code)
				return
			}

			handler_info(this, new_msg)
		}
	}

	return
}

//========================================================================

type CLIENT_MSG_HANDLER func(*HallConnection, proto.Message)

type NEW_MSG_FUNC func() proto.Message

type MsgHandlerMgr struct {
	msgid2handler map[int32]CLIENT_MSG_HANDLER
}

var msg_handler_mgr MsgHandlerMgr

func (this *MsgHandlerMgr) Init() bool {
	this.msgid2handler = make(map[int32]CLIENT_MSG_HANDLER)
	this.RegisterMsgHandler()
	return true
}

func (this *MsgHandlerMgr) SetMsgHandler(msg_code uint16, msg_handler CLIENT_MSG_HANDLER) {
	log.Info("set msg [%d] handler !", msg_code)
	this.msgid2handler[int32(msg_code)] = msg_handler
}

func (this *MsgHandlerMgr) RegisterMsgHandler() {
	this.SetMsgHandler(uint16(msg_client_message_id.MSGID_S2C_ENTER_GAME_RESPONSE), S2CEnterGameHandler)
	this.SetMsgHandler(uint16(msg_client_message_id.MSGID_S2C_BATTLE_RESULT_RESPONSE), S2CBattleResultHandler)
}

func hall_conn_msgid2msg(msg_id uint16) proto.Message {
	if msg_id == uint16(msg_client_message_id.MSGID_S2C_ENTER_GAME_RESPONSE) {
		return &msg_client_message.S2CEnterGameResponse{}
	} else if msg_id == uint16(msg_client_message_id.MSGID_S2C_ENTER_GAME_COMPLETE_NOTIFY) {
		return &msg_client_message.S2CEnterGameCompleteNotify{}
	} else if msg_id == uint16(msg_client_message_id.MSGID_S2C_BATTLE_RESULT_RESPONSE) {
		return &msg_client_message.S2CBattleResultResponse{}
	} else {
		log.Error("Cant found proto message by msg_id[%v]", msg_id)
	}
	return nil
}

func S2CEnterGameHandler(hall_conn *HallConnection, m proto.Message) {
	res := m.(*msg_client_message.S2CEnterGameResponse)
	cur_hall_conn = hall_conn_mgr.GetHallConnByAcc(res.GetAcc())
	if nil == cur_hall_conn {
		log.Error("S2CLoginResponseHandler failed to get cur hall[%s]", res.GetAcc())
		return
	}

	hall_conn.playerid = res.GetPlayerId()
	hall_conn.blogin = true
	log.Trace("player[%v]收到进入游戏服务器返回 %v", res.GetAcc(), res)

	return
}

func output_report(rr *msg_client_message.BattleReportItem) {
	log.Debug("		 	report: side[%v]", rr.Side)
	log.Debug("					 skill_id: %v", rr.SkillId)
	log.Debug("					 user: Side[%v], Pos[%v], HP[%v], MaxHP[%v], Energy[%v], Damage[%v]", rr.User.Side, rr.User.Pos, rr.User.HP, rr.User.MaxHP, rr.User.Energy, rr.User.Damage)
	if rr.IsSummon {
		if rr.SummonNpcs != nil {
			for n := 0; n < len(rr.SummonNpcs); n++ {
				rrs := rr.SummonNpcs[n]
				if rrs != nil {
					log.Debug("					 summon npc: Side[%v], Pos[%v], Id[%v], TableId[%v], HP[%v], MaxHP[%v], Energy[%v]", rrs.Side, rrs.Pos, rrs.Id, rrs.TableId, rrs.HP, rrs.MaxHP, rrs.Energy)
				}
			}
		}
	} else {
		if rr.BeHiters != nil {
			for n := 0; n < len(rr.BeHiters); n++ {
				rrb := rr.BeHiters[n]
				log.Debug("					 behiter: Side[%v], Pos[%v], HP[%v], MaxHP[%v], Energy[%v], Damage[%v], IsCritical[%v], IsBlock[%v]",
					rrb.Side, rrb.Pos, rrb.HP, rrb.MaxHP, rrb.Energy, rrb.Damage, rrb.IsCritical, rrb.IsBlock)
			}
		}
	}
	if rr.AddBuffs != nil {
		for n := 0; n < len(rr.AddBuffs); n++ {
			log.Debug("					 add buff: Side[%v], Pos[%v], BuffId[%v]", rr.AddBuffs[n].Side, rr.AddBuffs[n].Pos, rr.AddBuffs[n].BuffId)
		}
	}
	if rr.RemoveBuffs != nil {
		for n := 0; n < len(rr.RemoveBuffs); n++ {
			log.Debug("					 remove buff: Side[%v], Pos[%v], BuffId[%v]", rr.RemoveBuffs[n].Side, rr.RemoveBuffs[n].Pos, rr.RemoveBuffs[n].BuffId)
		}
	}

	log.Debug("					 has_combo: %v", rr.HasCombo)
}

func S2CBattleResultHandler(hall_conn *HallConnection, m proto.Message) {
	response := m.(*msg_client_message.S2CBattleResultResponse)
	if response.IsWin {
		log.Debug("Player[%v] wins", hall_conn.playerid)
	} else {
		log.Debug("Player[%v] lost", hall_conn.playerid)
	}

	if response.MyTeam != nil {
		log.Debug("My team:")
		log.Debug("		 MyArtifact: %v", response.GetMyArtifactId())
		for i := 0; i < len(response.MyTeam); i++ {
			m := response.MyTeam[i]
			if m == nil {
				continue
			}
			log.Debug("		 Side:%v Id:%v Pos:%v HP:%v MaxHP:%v Energy:%v TableId:%v", m.Side, m.Id, m.Pos, m.HP, m.MaxHP, m.Energy, m.TableId)
		}
	}
	if response.TargetTeam != nil {
		log.Debug("Target team:")
		log.Debug("      TargetArtifact: %v", response.GetTargetArtifactId())
		for i := 0; i < len(response.TargetTeam); i++ {
			m := response.TargetTeam[i]
			if m == nil {
				continue
			}
			log.Debug("		 Side:%v Id:%v Pos:%v HP:%v MaxHP:%v Energy:%v TableId:%v", m.Side, m.Id, m.Pos, m.HP, m.MaxHP, m.Energy, m.TableId)
		}
	}

	if response.EnterReports != nil {
		log.Debug("   before enter:")
		for i := 0; i < len(response.EnterReports); i++ {
			r := response.EnterReports[i]
			output_report(r)
		}
	}

	if response.Rounds != nil {
		log.Debug("Round num: %v", len(response.Rounds))
		for i := 0; i < len(response.Rounds); i++ {
			r := response.Rounds[i]
			log.Debug("	  round[%v]", r.RoundNum)
			if r.Reports != nil {
				for j := 0; j < len(r.Reports); j++ {
					rr := r.Reports[j]
					output_report(rr)
				}
			}
			if r.RemoveBuffs != nil {
				for j := 0; j < len(r.RemoveBuffs); j++ {
					b := r.RemoveBuffs[j]
					log.Debug("		 	remove buffs: Side[%v], Pos[%v], BuffId[%v]", b.Side, b.Pos, b.BuffId)
				}
			}
			if r.ChangedFighters != nil {
				for j := 0; j < len(r.ChangedFighters); j++ {
					m := r.ChangedFighters[j]
					log.Debug("			changed member: Side[%v], Pos[%v], HP[%v], MaxHP[%v], Energy[%v], Damage[%v]", m.Side, m.Pos, m.HP, m.MaxHP, m.Energy, m.Damage)
				}
			}
		}
	}
}
