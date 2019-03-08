package share_data

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

const (
	CLIENT_OS_DEFAULT = ""
	CLIENT_OS_ANDROID = "android"
	CLIENT_OS_IOS     = "ios"
)

type HallServerInfo struct {
	Id        int32
	Name      string
	IP        string
	Weight    int32
	ClientOS  string
	VerifyUse bool
}

type CrossInfo struct {
	Id        int32
	ServerIds []int32
}

type ServerList struct {
	CommonIP          string
	Servers           []*HallServerInfo
	TotalWeight       int32
	CrossList         []*CrossInfo
	Id2Cross          map[int32]*CrossInfo
	ServerId2Cross    map[int32]*CrossInfo
	IosVerifyServerId int32
	ConfigPath        string
	MD5Str            string
	LastTime          int32
	Locker            sync.RWMutex
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

	var total_weight int32
	if this.Servers != nil {
		for i := 0; i < len(this.Servers); i++ {
			s := this.Servers[i]
			if s.Weight < 0 {
				log.Error("Server Id %v Weight invalid %v", s.Id, s.Weight)
				return false
			} else if s.Weight == 0 {
				log.Trace("Server Id %v Weight %v", s.Id, s.Weight)
			}
			total_weight += s.Weight
		}
	}

	if total_weight <= 0 {
		log.Error("Server List Total Weight is invalid %v", total_weight)
		return false
	}

	this.TotalWeight = total_weight

	if this.CrossList != nil {
		this.Id2Cross = make(map[int32]*CrossInfo)
		this.ServerId2Cross = make(map[int32]*CrossInfo)
		for i := 0; i < len(this.CrossList); i++ {
			c := this.CrossList[i]
			if c == nil {
				continue
			}
			this.Id2Cross[c.Id] = c
			if c.ServerIds != nil {
				for n := 0; n < len(c.ServerIds); n++ {
					this.ServerId2Cross[c.ServerIds[n]] = c
				}
			}
		}
	}

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
	this.Locker.Lock()
	defer this.Locker.Unlock()

	data, err := ioutil.ReadFile(this.ConfigPath)
	if err != nil {
		fmt.Printf("重新读取配置文件失败 %v", err)
		return false
	}

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

func (this *ServerList) GetServerById(id int32) (info *HallServerInfo) {
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

func (this *ServerList) RandomOneServer(client_os string) (info *HallServerInfo) {
	this.Locker.RLock()
	defer this.Locker.RUnlock()

	var total_weight int32
	total_weight = this.TotalWeight

	now_time := time.Now()
	rand.Seed(now_time.Unix() + now_time.UnixNano())
	r := rand.Int31n(total_weight)

	log.Debug("!!!!! Server List Random %v with client os %v", r, client_os)

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

func (this *ServerList) GetIosVerifyServerId() int32 {
	return this.IosVerifyServerId
}

func (this *ServerList) GetServers(client_os string) (servers []*HallServerInfo) {
	this.Locker.RLock()
	defer this.Locker.RUnlock()

	for i := 0; i < len(this.Servers); i++ {
		s := this.Servers[i]
		servers = append(servers, s)
	}
	return
}

func (this *ServerList) HasServerId(server_id int32) bool {
	this.Locker.RLock()
	defer this.Locker.RUnlock()

	var found bool
	for i := 0; i < len(this.Servers); i++ {
		s := this.Servers[i]
		if s.Id == server_id {
			found = true
			break
		}
	}
	return found
}

func (this *ServerList) GetCrossServers(id int32) *CrossInfo {
	this.Locker.RLock()
	defer this.Locker.RUnlock()
	return this.Id2Cross[id]
}

func (this *ServerList) GetCrossByServerId(server_id int32) *CrossInfo {
	this.Locker.RLock()
	defer this.Locker.RUnlock()
	return this.ServerId2Cross[server_id]
}

func (this *ServerList) IsSameCross(server1_id, server2_id int32) bool {
	this.Locker.RLock()
	defer this.Locker.RUnlock()

	return this.ServerId2Cross[server1_id] == this.ServerId2Cross[server2_id]
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
