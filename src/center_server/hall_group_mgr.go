package main

import (
	"encoding/json"
	"fmt"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type SingleHallCfg struct {
	ServerIdx int32  // 内部编号
	ServerId  int32  // 服务器ID
	ServerIP  string // 服务器IP
}

type HallGroupCfg struct {
	GroupId   int32            // 组Id
	MinPid    int32            // 最小Pid
	MaxPid    int32            // 最大Pid
	ServerNum int32            // 服务器数目
	Servers   []*SingleHallCfg // 服务器IP信息
}

type HallGroups struct {
	HallServerGroups []*HallGroupCfg
}

type HallGroupMgr struct {
	hall_groups *HallGroups
}

//var hall_group_mgr HallGroupMgr

func (this *HallGroupMgr) Init() bool {

	if !this.load_hall_group_config() {
		log.Error("HallGroupMgr init db_group_mgr failed !")
		return false
	}

	return true
}

func (this *HallGroupMgr) load_hall_group_config() bool {
	this.hall_groups = &HallGroups{}

	hall_server_group_path := server_config.RuntimeRootDir + server_config.ConfigDir + config.HallServerGroupConfigFile
	data, err := ioutil.ReadFile(hall_server_group_path)
	if err != nil {
		fmt.Printf("HallGroupMgr读取配置文件(%s)失败 %s %s", hall_server_group_path, err)
		return false
	}
	err = json.Unmarshal(data, this.hall_groups)
	if err != nil {
		fmt.Printf("HallGroupMgr解析配置文件失败 %s", err.Error())
		return false
	}

	for _, groupinfo := range this.hall_groups.HallServerGroups {
		if nil == groupinfo {
			fmt.Printf("HallGroupMgr解析结果HallGroups包含空值")
			return false
		}

		for ival := int32(1); ival <= groupinfo.ServerNum; ival++ {
			bfind := false
			for _, svrinfo := range groupinfo.Servers {
				if nil == svrinfo {
					fmt.Printf("HallGroupMgr解析结果HallGroups包含空值")
					return false
				}

				log.Info("svr info %d", svrinfo.ServerIdx)
				if svrinfo.ServerIdx == ival {
					bfind = true
					break
				}
			}

			if !bfind {
				fmt.Printf("数据库集群里面缺少服务器%d", ival)
				return false
			}
		}
	}

	log.Info("Hall Group Info %v", *this.hall_groups)

	return true
}

// 先根据playerid找到group，然后再group的各个server里面依次散列Id
func (this *HallGroupMgr) GetHallCfgByPlayerId(playerid int32) *SingleHallCfg {
	svr_idx := int32(-1)
	for _, groupinfo := range this.hall_groups.HallServerGroups {
		if playerid < groupinfo.MinPid || playerid > groupinfo.MaxPid {
			log.Info("GetHallCfgByPlayerId 1 continue groupinfo.MinPId(%d) groupinfo.MaxPId(%d), playerid(%d)", groupinfo.MinPid, groupinfo.MaxPid, playerid)
			continue
		}

		svr_idx = playerid%groupinfo.ServerNum + 1
		for _, svr_info := range groupinfo.Servers {
			if svr_info.ServerIdx == svr_idx {
				return svr_info
			}
		}
	}

	return nil
}
