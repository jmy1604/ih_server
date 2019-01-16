package main

import (
	"errors"
	"fmt"
	"ih_server/libs/log"
	"ih_server/src/rpc_common"
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
		row.SetTableId(args.MailTableID)
		row.AttachedItems.SetItemList(args.AttachItems)
		dbc.SysMailCommon.GetRow().SetCurrMailId(row.GetId())
	} else {
		res := RealSendMail(nil, args.PlayerId, MAIL_TYPE_SYSTEM, args.MailTableID, "", "", args.AttachItems, 0)
		result.Res = res
	}

	log.Trace("@@@ G2H_Proc::SysMail %v", args)
	return nil
}
