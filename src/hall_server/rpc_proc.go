package main

import (
	"ih_server/libs/log"
	"ih_server/libs/rpc"
	"ih_server/src/rpc_proto"
)

// ping RPC服务
type R2H_PingProc struct{}

func (this *R2H_PingProc) Do(args *rpc_proto.R2H_Ping, reply *rpc_proto.R2H_Pong) error {
	// 不做任何处理
	log.Info("收到rpc服务的ping请求")
	return nil
}

// 全局调用
type H2H_GlobalProc struct {
}

func (this *H2H_GlobalProc) WorldChat(args *rpc_proto.H2H_WorldChat, result *rpc_proto.H2H_WorldChatResult) error {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	log.Debug("@@@ H2H_GlobalProc::WorldChat Player[%v] world chat content[%v]", args.FromPlayerId, args.ChatContent)
	return nil
}

// 初始化rpc服务
func (this *HallServer) init_rpc_service() bool {
	if this.rpc_service != nil {
		return true
	}
	this.rpc_service = &rpc.Service{}

	// 注册RPC服务
	if !this.rpc_service.Register(&R2H_PingProc{}) {
		return false
	}
	if !this.rpc_service.Register(&H2H_GlobalProc{}) {
		return false
	}

	if !this.rpc_service.Register(&G2G_CommonProc{}) {
		return false
	}

	if !this.rpc_service.Register(&G2H_Proc{}) {
		return false
	}

	if this.rpc_service.Listen(config.ListenRpcServerIP) != nil {
		log.Error("监听rpc服务端口[%v]失败", config.ListenRpcServerIP)
		return false
	}
	log.Info("监听rpc服务端口[%v]成功", config.ListenRpcServerIP)
	go this.rpc_service.Serve()
	return true
}

// 反初始化rpc服务
func (this *HallServer) uninit_rpc_service() {
	if this.rpc_service != nil {
		this.rpc_service.Close()
		this.rpc_service = nil
	}
}
