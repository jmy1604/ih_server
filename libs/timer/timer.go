package timer

import (
	"ih_server/libs/log"
	"sync"
	"time"
)

const (
	one_day_sec = 86400
)

// 获取时差
func get_zone_offset() int32 {
	_, zone_sec := time.Now().Zone()
	return int32(zone_sec)
}

func GetLocalUnix(sec int64) (local_sec int64) {
	return sec + int64(get_zone_offset()) //3600*8
}

type TickTime struct {
	Last time.Time
	Now  time.Time
}

func (this *TickTime) GetDuration() (dur time.Duration) {
	return this.Now.Sub(this.Last)
}

// 定时检查是否已过今天的某一时刻
func (this *TickTime) TickCheckDaily(config_time int32) bool {
	config_time_unix := time.Date(this.Last.Year(), this.Last.Month(), this.Last.Day(), 0, 0, 0, 0, time.Local).Unix() + int64(config_time)
	if (this.Now.Unix() > config_time_unix) && (config_time_unix >= this.Last.Unix()) {
		return true
	}
	return false
}

// 触发式检查是否已过今天的某个时刻
func CheckDailyByLastTime(config_time int32, last_time_unix int32) (bool, int32) {
	if config_time <= 0 {
		return false, 0
	}

	now := time.Now()
	config_time_sec := int32(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()) + config_time
	// 判断隔天条件(前提条件:保存时间小于重置panics时间): 1.超过重置时间 2.相隔超过1天
	if (last_time_unix < config_time_sec) && (config_time_sec <= int32(now.Unix()) || (config_time_sec-last_time_unix) > one_day_sec) {
		return true, int32(now.Unix()) + get_zone_offset()
	}
	return false, 0
}

// 触发式检查某天是否已经过某个时刻
func CheckDailyByLastYearDay(config_time int32, last_year_day int32) (bool, int32) {
	if config_time <= 0 {
		return false, 0
	}

	now_year_day := int32(time.Now().YearDay())
	if last_year_day == now_year_day {
		return false, 0
	}

	now_day_sec := int32(time.Now().Hour()*3600) + int32(time.Now().Minute()*60) + int32(time.Now().Second())
	if now_day_sec >= config_time || now_year_day > last_year_day+1 || now_year_day < last_year_day {
		return true, now_year_day
	}

	return false, 0
}

// 触发式检查时候不在同一周,周一为一周的第一天； week_day：星期几, config_time:配置时间的总秒数
// 条件：当前时间必须大于last_time_unix
func CheckWeekByLastTime(config_time int32, last_time_unix int32) bool {
	if config_time <= 0 {
		return false
	}

	now := time.Now()
	now_zero_sec := int32(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix())
	now_week_day := int32((now.Weekday() + 6) % 7)
	now_week_config_sec := now_zero_sec - now_week_day*86400 + config_time
	// 当前时间所在周的配置时间 大于 last_time_unix，来判断不在同一周
	if last_time_unix < now_week_config_sec && int32(now.Unix()) > now_week_config_sec {
		return true
	}
	return false
}

// config_time_list的时间必须是从小到大的
func CheckTimePointListByLastTime(config_time_list []int32, last_time_unix int32) bool {
	if len(config_time_list) < 2 {
		return false
	}

	now := time.Now()
	today_time_unix := int32(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix())
	yesterday_time_unix := today_time_unix - 24*60*60
	// 昨天到今天跨了好几个时间点
	if now.Unix() < int64(today_time_unix+config_time_list[0]) &&
		last_time_unix < yesterday_time_unix+config_time_list[len(config_time_list)-1] {
		return true
	}

	for index := 0; index < len(config_time_list)-1; index++ {
		config_time_unix := today_time_unix + config_time_list[index]
		if last_time_unix < config_time_unix && int64(config_time_unix) <= now.Unix() {
			return true
		}
	}
	return false
}

func CheckDelayReset(delay_time int32, last_time int32) (trigger bool, now int32) {
	now = int32(time.Now().Unix())
	if now-last_time >= delay_time {
		return true, now
	}

	return false, 0
}

func GetDayFrom1970WithCfg(cfgsec int32) int32 {
	_, zone_sec := time.Now().Zone()
	totalsec := time.Now().Unix() + int64(zone_sec) - int64(cfgsec)
	return int32(totalsec / (24 * 3600))
}

func GetDayFrom1970WithCfgAndSec(cfgsec, in_sec int32) int32 {
	_, zone_sec := time.Now().Zone()
	totalsec := int64(in_sec) + int64(zone_sec) - int64(cfgsec)
	return int32(totalsec / (24 * 3600))
}

type TickTimer struct {
	Chan chan TickTime

	runing   bool
	lock     *sync.Mutex
	dur_msec int32
}

func NewTickTimer(dur_msec int32) (t *TickTimer) {
	if dur_msec <= 0 {
		log.Error("tick dur should be greater than 0 msec:%v,now instead with 1000 msec default", dur_msec)
		dur_msec = 1000
	}

	t = &TickTimer{}
	t.runing = false
	t.lock = &sync.Mutex{}
	t.dur_msec = dur_msec
	return
}

func (this *TickTimer) loop() {
	for {
		defer func() {
			if err := recover(); err != nil {
				//log.Error(err)
			}
		}()

		last := time.Now()
		for {
			time.Sleep(time.Millisecond * time.Duration(this.dur_msec))

			if !this.runing {
				return
			}

			now := time.Now()
			this.Chan <- TickTime{last, now}
			last = now
		}
	}
}

func (this *TickTimer) Start() {
	this.lock.Lock()
	defer this.lock.Unlock()

	if this.runing {
		return
	}

	this.runing = true

	this.Chan = make(chan TickTime)

	go this.loop()
}

func (this *TickTimer) Stop() {
	this.lock.Lock()
	defer this.lock.Unlock()

	if !this.runing {
		return
	}

	close(this.Chan)

	this.runing = false
}
