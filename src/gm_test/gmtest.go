package main

import (
	"bytes"
	//"encoding/base64"
	"encoding/json"
	"fmt"
	"ih_server/libs/log"
	"ih_server/libs/timer"
	"ih_server/src/rpc_common"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type GmTest struct {
	start_time         time.Time
	quit               bool
	shutdown_lock      *sync.Mutex
	shutdown_completed bool
	ticker             *timer.TickTimer
	initialized        bool
}

func (this *GmTest) Init() (ok bool) {
	this.start_time = time.Now()
	this.shutdown_lock = &sync.Mutex{}
	this.initialized = true
	return true
}

func (this *GmTest) Start() (err error) {
	log.Event("已启动", nil)
	log.Trace("**************************************************")
	this.Run()
	return
}

func (this *GmTest) Run() {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}

		this.shutdown_completed = true
	}()

	this.ticker = timer.NewTickTimer(1000)
	this.ticker.Start()
	defer this.ticker.Stop()

	for {
		select {
		case d, ok := <-this.ticker.Chan:
			{
				if !ok {
					return
				}

				this.OnTick(d)
			}
		}
	}
}

func (this *GmTest) Shutdown() {
	if !this.initialized {
		return
	}

	this.shutdown_lock.Lock()
	defer this.shutdown_lock.Unlock()

	if this.quit {
		return
	}
	this.quit = true

	log.Trace("关闭主循环")

	begin := time.Now()

	if this.ticker != nil {
		this.ticker.Stop()
	}

	for {
		if this.shutdown_completed {
			break
		}

		time.Sleep(time.Millisecond * 100)
	}

	log.Trace("关闭主循环耗时 %v 秒", time.Now().Sub(begin).Seconds())
}

func (this *GmTest) OnTick(t timer.TickTime) {
	fmt.Printf("请输入测试命令:\n")
	var cmd_str string
	fmt.Scanf("%s\n", &cmd_str)
	switch cmd_str {
	case "anounce":
		{
			var content, duration string
			fmt.Printf("请输入公告内容: ")
			fmt.Scanf("%s\n", &content)
			fmt.Printf("请输入公告持续时间: ")
			fmt.Scanf("%s\n", &duration)
			d, e := strconv.Atoi(duration)
			if e != nil {
				log.Error("duration %v transfer err %v", duration, e.Error())
				return
			}
			var cmd = rpc_common.GmAnouncementCmd{
				Content:       []byte(content),
				RemainSeconds: int32(d),
			}
			data, err := json.Marshal(&cmd)
			if err != nil {
				log.Error("Marshal GmAnouncementCmd err %v", err.Error())
				return
			}
			_send_func(rpc_common.GM_CMD_ANOUNCEMENT, data, rpc_common.GM_CMD_ANOUNCEMENT_STRING)
		}
	default:
		{
			if cmd_str != "" {
				strs := strings.Split(cmd_str, ",")
				fmt.Printf("strs[%v] length is %v\n", strs, len(strs))
				if len(strs) == 1 {
					//fmt.Printf("命令[%v]参数不够，至少一个\n", strs[0])
					//return
				} else if len(strs) == 0 {
					fmt.Printf("没有输入命令\n")
					return
				}
			}
		}
	}
}

func _send_func(msg_id int32, msg_data []byte, msg_str string) (int32, []byte) {
	//md := base64.StdEncoding.EncodeToString(msg_data)

	var gm_cmd = rpc_common.GmCmd{
		Id:     msg_id,
		Data:   msg_data,
		String: msg_str,
	}

	data, err := json.Marshal(&gm_cmd)
	if err != nil {
		log.Error("Marshal GmCmd %v %v err %v", msg_id, msg_str, err.Error())
		return -1, nil
	}

	//d := base64.StdEncoding.EncodeToString(data)

	var resp *http.Response
	url := "http://" + config.GmServerIP + "/gm"
	resp, err = http.Post(url, "application/x-www-form-urlencoded", bytes.NewReader(data))

	if nil != err {
		log.Error("Post[%s] GmCmd %v %v error[%s]", url, msg_id, msg_str, err.Error())
		return -1, nil
	}

	defer resp.Body.Close()

	data, err = ioutil.ReadAll(resp.Body)
	if nil != err {
		log.Error("Read post resp body err [%s]", err.Error())
		return -1, nil
	}

	var gm_resp rpc_common.GmResponse
	err = json.Unmarshal(data, &gm_resp)
	if err != nil {
		log.Error("Unmarshal GmResponse %v %v err %v", msg_id, msg_str, err.Error())
		return -1, nil
	}

	return 1, data
}
