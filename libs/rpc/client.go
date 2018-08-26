package rpc

import (
	"encoding/gob"
	"errors"
	"ih_server/libs/log"
	"net/rpc"
	"sync/atomic"
	"time"
)

const (
	RPC_CLIENT_STATE_NONE = iota
	RPC_CLIENT_STATE_CONNECTING
	RPC_CLIENT_STATE_CONNECTED
	RPC_CLIENT_STATE_DISCONNECT
)

const (
	PING_INTERVAL = 5
)

type OnConnectFunc func(arg interface{})

type Client struct {
	c          *rpc.Client
	state      int32 // 只在Run协程中修改
	addr       string
	on_connect OnConnectFunc
	to_close   int32
}

func NewClient() *Client {
	client := &Client{}
	return client
}

type PingArgs struct {
}

type PongReply struct {
}

func (this *Client) ping() error {
	args := &PingArgs{}
	reply := &PongReply{}
	err := this.Call("PingProc.Ping", args, reply)
	if err != nil {
		log.Warn("RPC client ping error[%v]", err.Error())
	}
	return err
}

func (this *Client) SetOnConnect(on_connect OnConnectFunc) {
	this.on_connect = on_connect
}

func (this *Client) Run() {
	go func() {
		for {
			to_close := atomic.LoadInt32(&this.to_close)
			if to_close > 0 {
				break
			}
			if this.state == RPC_CLIENT_STATE_DISCONNECT {
				if !this.Dial(this.addr) {
					log.Info("RPC reconnect addr[%v] failed", this.addr)
				} else {
					log.Info("RPC reconnect addr[%v] succeed", this.addr)
				}
			} else {
				err := this.ping()
				if err != nil {
					atomic.CompareAndSwapInt32(&this.state, RPC_CLIENT_STATE_CONNECTED, RPC_CLIENT_STATE_DISCONNECT)
					log.Warn("RPC connection disconnected, ready to reconnect...")
					time.Sleep(time.Second * PING_INTERVAL)
					continue
				}
			}
			time.Sleep(time.Second * PING_INTERVAL)
		}
	}()
}

func (this *Client) Dial(addr string) bool {
	c, e := rpc.Dial("tcp", addr)
	if e != nil {
		log.Error("RPC Dial addr[%v] error[%v]", addr, e.Error())
		return false
	}
	this.c = c
	this.state = RPC_CLIENT_STATE_CONNECTED
	this.addr = addr
	this.to_close = 0
	if this.on_connect != nil {
		this.on_connect(this)
	}
	return true
}

func (this *Client) Call(method string, args interface{}, reply interface{}) error {
	if this.c == nil {
		return errors.New("not create rpc client")
	}
	err := this.c.Call(method, args, reply)
	return err
}

func (this *Client) Close() {
	if this.c != nil {
		this.c.Close()
		this.c = nil
		atomic.StoreInt32(&this.to_close, 1)
	}
}

func (this *Client) GetState() int32 {
	return this.state
}

func RegisterUserType(rcvr interface{}) {
	gob.Register(rcvr)
}
