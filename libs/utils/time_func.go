package utils

import (
	"ih_server/libs/log"
	"time"
)

const (
	TIME_LAYOUT      = "15:04:05"
	TIME_WEEK_LAYOUT = "Monday 15:04:05"
)

func _get_today_config_time(now_time time.Time, day_time_config string) (res int32, today_config_time time.Time) {
	var loc *time.Location
	var err error
	loc, err = time.LoadLocation("Local")
	if err != nil {
		log.Error("!!!!!!! Load Location Local error[%v]", err.Error())
		res = -1
		return
	}

	var tm time.Time
	tm, err = time.ParseInLocation(TIME_LAYOUT, day_time_config, loc)
	if err != nil {
		log.Error("parse time format[%v] failed, err[%v]", day_time_config, err.Error())
		res = -1
		return
	}

	today_config_time = time.Date(now_time.Year(), now_time.Month(), now_time.Day(), tm.Hour(), tm.Minute(), tm.Second(), tm.Nanosecond(), tm.Location())
	if tm.Unix() >= today_config_time.Unix() {
		log.Error("!!!!!!! config day time greater today config time")
		res = -1
		return
	}

	res = 1
	return
}

func CheckDayTimeArrival(last_time_point int32, day_time_string string) bool {
	last_time := time.Unix(int64(last_time_point), 0)
	now_time := time.Now()

	if now_time.Unix() <= last_time.Unix() {
		return false
	}

	res, tmp := _get_today_config_time(now_time, day_time_string)
	if res < 0 {
		return false
	}

	last_refresh_time := tmp.Unix()
	if now_time.Unix() >= last_refresh_time && last_refresh_time > int64(last_time_point) {
		return true
	}

	return false
}

func GetRemainSeconds2NextDayTime(last_time_point int32, day_time_config string) int32 {
	last_time := time.Unix(int64(last_time_point), 0)
	now_time := time.Now()

	if now_time.Unix() < last_time.Unix() {
		return -1
	}

	res, today_tm := _get_today_config_time(now_time, day_time_config)
	if res < 0 {
		return res
	}

	if last_time.Unix() < today_tm.Unix() {
		if now_time.Unix() < today_tm.Unix() {
			return int32(today_tm.Unix() - now_time.Unix())
		} else {
			return 0
		}
	} else {
		return int32(today_tm.Unix() + int64(24*3600) - now_time.Unix())
	}
}

func GetDaysNumToLastSaveTime(last_save int32, day_time_config string, now_time time.Time) (num int32) {
	if last_save < 0 {
		return -1
	} else if last_save == 0 {
		num = 1
		last_save = int32(now_time.Unix())
	}

	last_time := time.Unix(int64(last_save), 0)
	if last_time.Unix() > now_time.Unix() {
		return -1
	}

	res, last_config_time := _get_today_config_time(last_time, day_time_config)
	if res < 0 {
		return -1
	}

	if last_save < int32(last_config_time.Unix()) {
		if now_time.Unix() < last_config_time.Unix() {

		} else {
			num += (int32(now_time.Unix()-last_config_time.Unix())/(24*3600) + 1)
		}
	} else {
		num += int32(now_time.Unix()-last_config_time.Unix()) / (24 * 3600)
	}

	return
}

type DaysTimeChecker struct {
	time_tm       time.Time // 配置时间
	interval_days int32
	next_time     int64
}

func (this *DaysTimeChecker) Init(last_save int32, time_value string, interval_days int32) bool {
	var loc *time.Location
	var err error
	loc, err = time.LoadLocation("Local")
	if err != nil {
		log.Error("!!!!!!! Load Location Local error[%v]", err.Error())
		return false
	}

	this.time_tm, err = time.ParseInLocation(TIME_LAYOUT, time_value, loc)
	if err != nil {
		log.Error("!!!!!!! Parse start time layout[%v] failed, err[%v]", TIME_LAYOUT, err.Error())
		return false
	}

	if this.time_tm.Unix() >= time.Now().Unix() {
		log.Error("!!!!!!! Now time is Early to start time")
		return false
	}

	if interval_days <= 0 {
		log.Error("!!!!!!! Interval Days %v invalid", interval_days)
		return false
	}

	this.interval_days = interval_days

	this._init_next_time(last_save)

	return true
}

func (this *DaysTimeChecker) _init_next_time(last_save int32) {
	if this.next_time != 0 {
		return
	}

	now_time := time.Now()
	if last_save == 0 {
		last_save = int32(now_time.Unix())
	}
	last_time := time.Unix(int64(last_save), 0)

	// 上次重置的当天，用配置的时间
	last_date := time.Date(last_time.Year(), last_time.Month(), last_time.Day(), this.time_tm.Hour(), this.time_tm.Minute(), this.time_tm.Second(), this.time_tm.Nanosecond(), this.time_tm.Location())
	if last_date.Unix() < now_time.Unix() {
		this.next_time = last_date.Unix() + int64(24*3600*this.interval_days)
	} else {
		this.next_time = last_date.Unix() + int64(24*3600*(this.interval_days-1))
	}
	for {
		if this.next_time > now_time.Unix() {
			break
		}
		this.next_time += int64(24 * 3600 * this.interval_days)
	}
}

func (this *DaysTimeChecker) ToNextTimePoint() {
	if this.next_time == 0 {
		log.Warn("DaysTimeChecker function ToNextTimePoint call must after Init")
		return
	}
	this.next_time += int64(24 * 3600 * this.interval_days)
}

func (this *DaysTimeChecker) IsArrival(last_save int32) bool {
	now_time := time.Now()
	if now_time.Unix() < this.next_time {
		return false
	}
	return true
}

func (this *DaysTimeChecker) RemainSecondsToNextRefresh(last_save int32) (remain_seconds int32) {
	now_time := time.Now()
	remain_seconds = int32(this.next_time - now_time.Unix())
	if remain_seconds < 0 {
		remain_seconds = 0
	}
	return
}

func GetRemainSeconds4NextRefresh(config_hour, config_minute, config_second int32, last_save_time int32) (next_refresh_remain_seconds int32) {
	now_time := time.Now()
	if int32(now_time.Unix()) < last_save_time {
		return 0
	}
	today_refresh_time := time.Date(now_time.Year(), now_time.Month(), now_time.Day(), int(config_hour), int(config_minute), int(config_second), 0, time.Local)
	if now_time.Unix() < today_refresh_time.Unix() {
		if int32(today_refresh_time.Unix())-24*3600 > last_save_time {
			next_refresh_remain_seconds = 0
		} else {
			next_refresh_remain_seconds = int32(today_refresh_time.Unix() - now_time.Unix())
		}
	} else {
		if int32(today_refresh_time.Unix()) > last_save_time {
			next_refresh_remain_seconds = 0
		} else {
			next_refresh_remain_seconds = 24*3600 - int32(now_time.Unix()-today_refresh_time.Unix())
		}
	}
	return
}

func IsDayTimeRefresh(config_hour, config_minute, config_second int32, last_unix_time int32) bool {
	now_time := time.Now()
	if int32(now_time.Unix()) < last_unix_time {
		return false
	}

	today_refresh_time := time.Date(now_time.Year(), now_time.Month(), now_time.Day(), int(config_hour), int(config_minute), int(config_second), 0, time.Local)
	if int32(today_refresh_time.Unix()) < last_unix_time {
		return false
	}

	return true
}
