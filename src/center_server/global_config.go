package main

import (
	"encoding/json"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type WeekTime struct {
	WeekDay int32
	Hour    int32
	Minute  int32
	Second  int32
}

type CampFightOpenPeriod struct {
	FightIdx      int32    // 战斗编号
	StartTime     WeekTime // 开始时间
	StartWeekSec  int32    // 开始时间换算成的在一周的第几秒
	ActEndTime    WeekTime // 结束时间
	ActEndWeekSec int32    // 结束时间换算成的在一周的第几秒
}
type CampFightRewardPeriod struct {
	FightIdx      int32    // 战斗编号
	StartTime     WeekTime // 开始时间
	StartWeekSec  int32    // 开始时间换算成的在一周的第几秒
	ActEndTime    WeekTime // 结束时间
	ActEndWeekSec int32    // 结束时间换算成的在一周的第几秒
}

type GlobalConfig struct {
	CampFightAllStartWeekSec int32                   // 整个阵营战的开启时间（换算成一周的第几秒）
	CampFightAllEndWeekSec   int32                   // 整个阵营战的结束时间（换算成一周的第几秒）
	CampFightOpenTimes       []CampFightOpenPeriod   // 阵营战的开启时间
	CampFightRewardTimes     []CampFightRewardPeriod // 阵营战的领奖时间
}

var global_config GlobalConfig

func global_config_load() bool {
	data, err := ioutil.ReadFile(server_config.GetGameDataPathFile("global.json"))
	if nil != err {
		log.Error("global_config_load failed to readfile err(%s)!", err.Error())
		return false
	}

	err = json.Unmarshal(data, &global_config)
	if nil != err {
		log.Error("global_config_load json unmarshal failed err(%s)!", err.Error())
		return false
	}

	for idx := int32(0); idx < int32(len(global_config.CampFightOpenTimes)); idx++ {
		tmp_val := &global_config.CampFightOpenTimes[idx]
		tmp_val.StartWeekSec = tmp_val.StartTime.WeekDay*24*3600 + tmp_val.StartTime.Hour*3600 + tmp_val.StartTime.Minute*60 + tmp_val.StartTime.Second
		if 0 >= global_config.CampFightAllStartWeekSec || tmp_val.StartWeekSec < global_config.CampFightAllStartWeekSec {
			global_config.CampFightAllStartWeekSec = tmp_val.StartWeekSec
		}
		tmp_val.ActEndWeekSec = tmp_val.ActEndTime.WeekDay*24*3600 + tmp_val.ActEndTime.Hour*3600 + tmp_val.ActEndTime.Minute*60 + tmp_val.ActEndTime.Second
		if 0 >= global_config.CampFightAllEndWeekSec || tmp_val.ActEndWeekSec > global_config.CampFightAllEndWeekSec {
			global_config.CampFightAllEndWeekSec = tmp_val.ActEndWeekSec
		}
	}

	for idx := int32(0); idx < int32(len(global_config.CampFightRewardTimes)); idx++ {
		tmp_val := &global_config.CampFightRewardTimes[idx]
		tmp_val.StartWeekSec = tmp_val.StartTime.WeekDay*24*3600 + tmp_val.StartTime.Hour*3600 + tmp_val.StartTime.Minute*60 + tmp_val.StartTime.Second
		tmp_val.ActEndWeekSec = tmp_val.ActEndTime.WeekDay*24*3600 + tmp_val.ActEndTime.Hour*3600 + tmp_val.ActEndTime.Minute*60 + tmp_val.ActEndTime.Second
	}

	log.Info("全局配置 %v", global_config)

	return true
}
