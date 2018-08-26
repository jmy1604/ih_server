package main

import (
	"ih_server/libs/log"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

var gm_http_mux map[string]func(http.ResponseWriter, *http.Request)

type GmHttpHandle struct{}

func (this *GmHttpHandle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var act_str, url_str string
	url_str = r.URL.String()
	idx := strings.Index(url_str, "?")
	if -1 == idx {
		act_str = url_str
	} else {
		act_str = string([]byte(url_str)[:idx])
	}
	log.Info("ServeHTTP actstr(%s)", act_str)
	if h, ok := gm_http_mux[act_str]; ok {
		h(w, r)
	}

	return
}

//=======================================================

type GmMgr struct {
	gm_http_listener net.Listener
}

var gm_mgr GmMgr

func (this *GmMgr) StartHttp() {
	var err error
	this.reg_http_mux()

	this.gm_http_listener, err = net.Listen("tcp", "192.168.10.113:25000")
	if nil != err {
		log.Error("Center StartHttp Failed %s", err.Error())
		return
	}

	gm_http_server := http.Server{
		Handler:     &GmHttpHandle{},
		ReadTimeout: 6 * time.Second,
	}

	log.Info("启动Gm服务 IP:%s", "192.168.10.113:25000")
	err = gm_http_server.Serve(this.gm_http_listener)
	if err != nil {
		log.Error("启动Center gm Http Server %s", err.Error())
		return
	}

}

//=========================================================

func (this *GmMgr) reg_http_mux() {
	gm_http_mux = make(map[string]func(http.ResponseWriter, *http.Request))
	gm_http_mux["/gm_cmd"] = test_gm_command_http_handler
}

func test_gm_command_http_handler(w http.ResponseWriter, r *http.Request) {
	if "GET" == r.Method {
		gm_str := r.URL.Query().Get("cmd_str")

		tmp_arr := strings.Split(gm_str, " ")

		tmp_len := int32(len(tmp_arr))
		if len(tmp_arr) < 1 {
			log.Error("C2HGmCommandHandler tmp_arr empty")
			return
		}

		cmd := tmp_arr[0]
		tmp_len--
		switch cmd {
		case "reboot":
			{
				cmd := exec.Command("/bin/sh", "/root/server/bin/up_data_restart_all_server.sh")

				bytes, err := cmd.Output()
				if err != nil {
					log.Error("cmd.Output: ", bytes, err)
					return
				} else {
					log.Info("cmd.Output: ", bytes)
				}
			}
		}

		w.Write([]byte("MUM196-198"))
		r.Body.Close()
	} else {
		log.Error("test_gm_command_http_handler not support POST Method")
	}
}
