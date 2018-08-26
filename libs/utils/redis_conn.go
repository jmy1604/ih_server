package utils

import (
	"errors"
	"fmt"
	"ih_server/libs/log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gomodule/redigo/redis"
)

const (
	REDIS_CLIENT_RECONNECT_INTERVAL = 10
)

type RedisConn struct {
	conn      redis.Conn
	addr      string
	disc      int32
	disc_chan chan bool
	mtx       *sync.Mutex
}

func (this *RedisConn) InitWithLock() {
	this.mtx = &sync.Mutex{}
}

func (this *RedisConn) Connect(addr string) bool {
	if this.conn != nil {
		fmt.Printf("redis已经建立了连接\n")
		return true
	}

	conn, err := redis.Dial("tcp", addr)
	if err != nil {
		fmt.Printf("redis连接[%v]失败[%v]\n", addr, err.Error())
		return false
	}
	this.conn = conn
	this.addr = addr
	this.disc_chan = make(chan bool)
	fmt.Printf("redis连接[%v]成功\n", addr)
	return true
}

func (this *RedisConn) Clear() {
	if this.conn != nil {
		this.conn.Close()
		this.conn = nil
	}
}

func (this *RedisConn) reconnect() bool {
	if this.mtx != nil {
		this.mtx.Lock()
		defer this.mtx.Unlock()
	}
	conn, err := redis.Dial("tcp", this.addr)
	if err != nil {
		fmt.Printf("redis重连[%v]失败[%v]\n", this.addr, err.Error())
		return false
	}
	this.conn = conn
	fmt.Printf("redis重连[%v]成功\n", this.addr)
	return true
}

func (this *RedisConn) Close() {
	if this.conn == nil {
		return
	}
	if this.disc == 1 {
		return
	}
	atomic.StoreInt32(&this.disc, 1)
	select {
	case d := <-this.disc_chan:
		{
			for !d {
			}
		}
	}
	this.conn.Close()
	this.conn = nil
	if this.mtx != nil {
		this.mtx = nil
	}
}

func (this *RedisConn) Do(cmd string, args ...interface{}) (interface{}, error) {
	if this.mtx != nil {
		this.mtx.Lock()
		defer this.mtx.Unlock()
	}
	if this.conn == nil {
		return nil, errors.New("未建立redis连接")
	}
	return this.conn.Do(cmd, args...)
}

func (this *RedisConn) Post(cmd string, args ...interface{}) error {
	if this.mtx != nil {
		this.mtx.Lock()
		defer this.mtx.Unlock()
	}
	if this.conn == nil {
		return errors.New("未建立redis连接")
	}
	err := this.conn.Send(cmd, args...)
	if err != nil {
		return err
	}
	return this.conn.Flush()
}

func (this *RedisConn) Flush() error {
	if this.mtx != nil {
		this.mtx.Lock()
		defer this.mtx.Unlock()
	}
	if this.conn == nil {
		return errors.New("未建立redis连接")
	}
	return this.conn.Flush()
}

func (this *RedisConn) Receive() (interface{}, error) {
	if this.mtx != nil {
		this.mtx.Lock()
		defer this.mtx.Unlock()
	}
	if this.conn == nil {
		return nil, errors.New("未建立redis连接")
	}
	return this.conn.Receive()
}

func (this *RedisConn) Send(cmd string, args ...interface{}) error {
	if this.mtx != nil {
		this.mtx.Lock()
		defer this.mtx.Unlock()
	}
	if this.conn == nil {
		return errors.New("未建立redis连接")
	}
	return this.conn.Send(cmd, args...)
}

func (this *RedisConn) ping() error {
	if this.mtx != nil {
		this.mtx.Lock()
		defer this.mtx.Unlock()
	}
	_, err := this.conn.Do("Ping")
	if err != nil {
		return err
	}
	return nil
}

func (this *RedisConn) Run(interval_ms int) {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}
	}()

	if this.conn == nil {
		return
	}

	var rec bool

	for {
		if atomic.LoadInt32(&this.disc) == 1 {
			this.disc_chan <- true
			break
		}

		if rec {
			// 重连
			if !this.reconnect() {
				time.Sleep(time.Second * REDIS_CLIENT_RECONNECT_INTERVAL)
				continue
			}
			rec = false
		}

		err := this.ping()
		if err != nil {
			rec = true
			log.Info("redis连接已断开，准备重连 ...")
		}

		time.Sleep(time.Millisecond * time.Duration(interval_ms))
	}
}
