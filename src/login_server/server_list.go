package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"ih_server/libs/log"
	"io/ioutil"
	"math/rand"
	"sync"
	"time"
)

type HallServerInfo struct {
	Id     int32
	Name   string
	IP     string
	Weight int32
}

type ServerList struct {
	Servers     []*HallServerInfo
	Data        string
	TotalWeight int32
	ConfigPath  string
	MD5Str      string
	LastTime    int32
	Locker      sync.RWMutex
}

func _get_md5(data []byte) string {
	// md5校验码
	md5_ctx := md5.New()
	md5_ctx.Write(data)
	cipherStr := md5_ctx.Sum(nil)
	return hex.EncodeToString(cipherStr)
}

func (this *ServerList) _read_config(data []byte) bool {
	err := json.Unmarshal(data, this)
	if err != nil {
		fmt.Printf("解析配置文件失败 %v", err.Error())
		return false
	}

	if this.Servers != nil {
		for i := 0; i < len(this.Servers); i++ {
			s := this.Servers[i]
			if s.Weight < 0 {
				log.Error("Server Id %v Weight invalid %v", s.Id, s.Weight)
				return false
			} else if s.Weight == 0 {
				log.Trace("Server Id %v Weight %v", s.Id, s.Weight)
			}
			this.TotalWeight += s.Weight
		}
	}

	if this.TotalWeight <= 0 {
		log.Error("Server List Total Weight is invalid %v", this.TotalWeight)
		return false
	}

	this.Data = string(data)
	this.MD5Str = _get_md5(data)

	return true
}

func (this *ServerList) ReadConfig(filepath string) bool {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		fmt.Printf("读取配置文件失败 %v", err)
		return false
	}

	if !this._read_config(data) {
		return false
	}

	this.ConfigPath = filepath

	return true
}

func (this *ServerList) RereadConfig() bool {
	data, err := ioutil.ReadFile(this.ConfigPath)
	if err != nil {
		fmt.Printf("重新读取配置文件失败 %v", err)
		return false
	}

	this.Locker.Lock()
	defer this.Locker.Unlock()

	md5_str := _get_md5(data)
	if md5_str == this.MD5Str {
		return true
	}

	this.Servers = nil
	this.TotalWeight = 0

	if !this._read_config(data) {
		return false
	}

	log.Trace("**** Server List updated")

	return true
}

func (this *ServerList) GetById(id int32) (info *HallServerInfo) {
	this.Locker.RLock()
	defer this.Locker.RUnlock()

	if this.Servers == nil {
		return
	}

	for i := 0; i < len(this.Servers); i++ {
		if this.Servers[i] == nil {
			continue
		}
		if this.Servers[i].Id == id {
			info = this.Servers[i]
			break
		}
	}
	return
}

func (this *ServerList) RandomOne() (info *HallServerInfo) {
	this.Locker.RLock()
	defer this.Locker.RUnlock()

	now_time := time.Now()
	rand.Seed(now_time.Unix() + now_time.UnixNano())
	r := rand.Int31n(this.TotalWeight)

	log.Debug("!!!!! Server List Random %v", r)

	for i := 0; i < len(this.Servers); i++ {
		s := this.Servers[i]
		if s.Weight <= 0 {
			continue
		}
		if r < s.Weight {
			info = s
			break
		}
		r -= s.Weight
	}

	return
}

func (this *ServerList) Run() {
	for {
		now_time := time.Now()
		if int32(now_time.Unix())-this.LastTime >= 60 {
			this.RereadConfig()
			this.LastTime = int32(now_time.Unix())
		}
		time.Sleep(time.Minute)
	}
}

var server_list ServerList
