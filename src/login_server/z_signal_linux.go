package main

import (
	"ih_server/libs/log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type CLOSE_FUNC func(info *SignalRegRecod)

type SignalRegRecod struct {
	close_func CLOSE_FUNC
	close_flag bool
}

var signal_mgr SignalMgr

type SignalMgr struct {
	signal_c  chan os.Signal
	close_map map[string]*SignalRegRecod
	b_closing bool
}

func (this *SignalMgr) Init() bool {
	this.signal_c = make(chan os.Signal, 10)
	this.close_map = make(map[string]*SignalRegRecod)
	this.b_closing = false
	signal.Notify(this.signal_c, os.Interrupt, syscall.SIGTERM)

	go this.DoAllCloseFunc()
	return true
}

func (this *SignalMgr) RegCloseFunc(modname string, close_func CLOSE_FUNC) {
	if nil == this.close_map[modname] {
		this.close_map[modname] = &SignalRegRecod{
			close_func: close_func,
			close_flag: false,
		}
	} else {
		this.close_map[modname].close_func = close_func
	}

	return
}

func (this *SignalMgr) DoAllCloseFunc() {

	select {
	case <-this.signal_c:
		{
			this.b_closing = true
			server.Shutdown()
			for _, info := range this.close_map {
				info.close_func(info)
			}
			break
		}
	}
}

func (this *SignalMgr) WaitAllCloseOver() {
	for index, info := range this.close_map {
		if nil == info {
			continue
		}

		for {
			time.Sleep(time.Millisecond * 10)
			if info.close_flag {
				log.Info(index + " ended !")
				break
			}
		}
	}

	return
}

func (this *SignalMgr) Close() {
	signal.Stop(this.signal_c)
}

func (this *SignalMgr) IfClosing() bool {
	return this.b_closing
}
