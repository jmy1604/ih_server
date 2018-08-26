package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type HallServerInfo struct {
	Id   int32
	Name string
	IP   string
}

type ServerList struct {
	Servers []*HallServerInfo
	Data    string
}

func (this *ServerList) ReadConfig(filepath string) bool {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		fmt.Printf("读取配置文件失败 %v", err)
		return false
	}
	err = json.Unmarshal(data, this)
	if err != nil {
		fmt.Printf("解析配置文件失败 %v", err.Error())
		return false
	}
	this.Data = string(data)
	return true
}

func (this *ServerList) GetById(id int32) (info *HallServerInfo) {
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

var server_list ServerList
