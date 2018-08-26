package main

import (
	"ih_server/libs/log"
	"sync"
	"time"
)

type Mutex struct {
	Mutex       *sync.Mutex
	m_timer     *time.Timer
	TimeoutMSec int32
}

func NewMutex() (this *Mutex) {
	this = &Mutex{}
	this.Mutex = &sync.Mutex{}
	this.TimeoutMSec = 2000

	return this
}

func NewMutexWithTime(timeout_msec int32) (this *Mutex) {
	this = &Mutex{}
	this.Mutex = &sync.Mutex{}
	this.TimeoutMSec = timeout_msec
	return this
}

func (this *Mutex) Lock(name string) {
	this.Mutex.Lock()

	if this.m_timer != nil {
		this.m_timer.Stop()
	}

	this.m_timer = time.AfterFunc(time.Millisecond*time.Duration(this.TimeoutMSec), func() {
		log.Error("RWMutex Lock timeout %v" + name)
	})
}

func (this *Mutex) Unlock() {
	if this.m_timer != nil {
		this.m_timer.Stop()
		this.m_timer = nil
	}
	this.Mutex.Unlock()
}

func (this *Mutex) UnSafeLock(name string) {
	this.Mutex.Lock()
}

func (this *Mutex) UnSafeUnlock() {
	this.Mutex.Unlock()
}

type RWMutex struct {
	RWMutex     *sync.RWMutex
	m_timer     *time.Timer
	TimeoutMSec int32
}

func NewRWMutex() (this *RWMutex) {
	this = &RWMutex{}
	this.RWMutex = &sync.RWMutex{}
	this.TimeoutMSec = 5000

	return this
}

func NewRWMutexWithTime(timeout_msec int32) (this *RWMutex) {
	this = &RWMutex{}
	this.RWMutex = &sync.RWMutex{}
	this.TimeoutMSec = timeout_msec
	return this
}

func (this *RWMutex) Lock(name string) {
	this.RWMutex.Lock()

	if this.m_timer != nil {
		this.m_timer.Stop()
	}
	this.m_timer = time.AfterFunc(time.Millisecond*time.Duration(this.TimeoutMSec), func() {
		log.Error("RWMutex Lock timeout %v" + name)
	})
}

func (this *RWMutex) Unlock() {
	if this.m_timer != nil {
		this.m_timer.Stop()
		this.m_timer = nil
	}
	this.RWMutex.Unlock()
}

func (this *RWMutex) RLock(name string) {
	this.RWMutex.RLock()
	/*
		if this.m_timer != nil {
			this.m_timer.Stop()
		}
		this.m_timer = time.AfterFunc(time.Millisecond*time.Duration(this.TimeoutMSec), func() {
			log.Error("RWMutex RLock timeout %v" + name)
		})
	*/
}

func (this *RWMutex) RUnlock() {
	/*
		if this.m_timer != nil {
			this.m_timer.Stop()
			//this.m_timer = nil
		}
	*/
	this.RWMutex.RUnlock()
}

func (this *RWMutex) UnSafeLock(name string) {
	this.RWMutex.Lock()
}

func (this *RWMutex) UnSafeUnlock() {
	this.RWMutex.Unlock()
}

func (this *RWMutex) UnSafeRLock(name string) {
	this.RWMutex.RLock()
}

func (this *RWMutex) UnSafeRUnlock() {
	this.RWMutex.RUnlock()
}
