package main

import (
	_ "encoding/json"
	_ "ih_server/libs/log"
	_ "io/ioutil"
	"sync"
	_ "time"
)

type LoginServerInfo struct {
	IP string
}

type LoginServerList struct {
	logins_info []*LoginServerInfo
	read_locker *sync.RWMutex
	read_time   int64
	quit        bool
}

func (this *LoginServerList) init() (ok bool) {
	this.read_locker = &sync.RWMutex{}

	if this.ReadConfig() != nil {
		return
	}

	go this.read_loop()

	ok = true
	return
}

func (this *LoginServerList) read_loop() {
	/*now := time.Now()
	this.read_time = now.Unix()
	interval := config.ReadLoginListConfigInterval
	if interval == 0 {
		interval = 5 * 60
	}

	for {
		if this.quit { break }
		if now.Unix() - this.read_time >= int64(interval) {
			this.ReadConfig()
			this.read_time = now.Unix()
		}
		time.Sleep(time.Second * 5)
		now = time.Now()
	}*/
}

func (this *LoginServerList) ReadConfig() (err error) {
	/*var list_data []byte

	this.read_locker.Lock()
	defer this.read_locker.Unlock()

	list_data, err = ioutil.ReadFile(config.LoginListConfigPath)
	if err != nil {
		log.Error("读取LoginServer服务器列表文件失败 %v", err)
		return
	}

	err = json.Unmarshal(list_data, &this.logins_info)
	if err != nil {
		log.Error("解析LoginServer服务器列表失败 %v", err)
		return
	}

	log.Trace("读取了LoginServer服务器列表")*/

	return
}
