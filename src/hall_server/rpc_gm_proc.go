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

func (this *G2H_Proc) Test(args *rpc_common.GmTestCmd, result *rpc_common.GmTestResponse) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	log.Trace("@@@ G2H_Proc::Test %v", args)
	return nil
}

func (this *G2H_Proc) Anouncement(args *rpc_common.GmAnouncementCmd, result *rpc_common.GmAnouncementResponse) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	if !system_chat_mgr.push_chat_msg(args.Content, args.RemainSeconds, 0, 0, "", 0) {
		err_str := fmt.Sprintf("@@@ G2H_Proc::Anouncement %v failed", args)
		return errors.New(err_str)
	}

	log.Trace("@@@ G2H_Proc::Anouncement %v", args)
	return nil
}
