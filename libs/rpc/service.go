package rpc

import (
	"ih_server/libs/log"
	"net"
	"net/rpc"
)

type Service struct {
	listener *net.TCPListener
}

func (this *Service) Register(rcvr interface{}) bool {
	err := rpc.Register(rcvr)
	if err != nil {
		log.Debug("rpc service register error[%v]", err.Error())
		return false
	}
	return true
}

type PingProc struct {
}

func (this *PingProc) Ping(args *PingArgs, reply *PongReply) error {
	return nil
}

func (this *Service) Listen(addr string) error {
	ping_proc := &PingProc{}
	this.Register(ping_proc)

	var address, _ = net.ResolveTCPAddr("tcp", addr)
	l, e := net.ListenTCP("tcp", address)
	if e != nil {
		return e
	}
	this.listener = l

	log.Info("rpc service listen to %v", addr)
	return nil
}

func (this *Service) Serve() {
	var i = 1
	for {
		conn, err := this.listener.Accept()
		if err != nil {
			continue
		}
		log.Info("rpc service accept a new connection[%v]", i)
		i += 1
		go rpc.ServeConn(conn)
	}
}

func (this *Service) Close() {
	this.listener.Close()
}
