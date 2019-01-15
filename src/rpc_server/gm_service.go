package main

import (
	"crypto/tls"
	"encoding/json"
	"ih_server/libs/log"
	"ih_server/src/rpc_common"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"
)

//=================================================================================

type LoginHttpHandle struct{}

type GmService struct {
	login_http_listener net.Listener
	login_http_server   http.Server
}

var gm_service GmService

func (this *GmService) StartHttp() bool {
	var err error
	this.reg_http_mux()

	this.login_http_listener, err = net.Listen("tcp", rpc_config.GmIP)
	if nil != err {
		log.Error("Listen gm http server error %v", err.Error())
		return false
	}

	login_http_server := http.Server{
		Handler:     &LoginHttpHandle{},
		ReadTimeout: 6 * time.Second,
	}

	err = login_http_server.Serve(this.login_http_listener)
	if err != nil {
		log.Error("Start gm http server error %v", err.Error())
		return false
	}

	return true
}

func (this *GmService) StartHttps(crt_file, key_file string) bool {
	this.reg_http_mux()

	this.login_http_server = http.Server{
		Addr:        rpc_config.GmIP,
		Handler:     &LoginHttpHandle{},
		ReadTimeout: 6 * time.Second,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
	}

	err := this.login_http_server.ListenAndServeTLS(crt_file, key_file)
	if err != nil {
		log.Error("Listen gm https server error %v", err.Error())
		return false
	}

	return true
}

var gm_http_mux map[string]func(http.ResponseWriter, *http.Request)

func (this *GmService) reg_http_mux() {
	gm_http_mux = make(map[string]func(http.ResponseWriter, *http.Request))
	gm_http_mux["/gm"] = gm_http_handler
}

func (this *LoginHttpHandle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

type gm_handle func(id int32, data []byte) (int32, []byte)

func gm_http_handler(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
			return
		}
	}()

	data, err := ioutil.ReadAll(r.Body)
	if nil != err {
		log.Error("Gm read http data err[%s]", err.Error())
		return
	}

	var res int32
	var resp_data []byte
	var gm_cmd rpc_common.GmCmd
	err = json.Unmarshal(data, &gm_cmd)
	if err != nil {
		res = -1
		log.Error("Gm json unmarshal GmCmd err %v", err.Error())
	}

	if res >= 0 {
		f := gm_handles[gm_cmd.Id]
		if f == nil {
			res = -1
			log.Error("Unknown gm cmd %v %v", gm_cmd.Id, gm_cmd.String)
		} else {
			res, resp_data = f(gm_cmd.Id, gm_cmd.Data)
			if res < 0 {
				res = -1
				log.Error("Gm cmd %v %v execute failed %v", gm_cmd.Id, gm_cmd.String, res)
			}
		}
	}

	var gm_resp = rpc_common.GmResponse{
		Id:   gm_cmd.Id,
		Res:  res,
		Data: resp_data,
	}
	data, err = json.Marshal(&gm_resp)
	if err != nil {
		log.Error("Gm cmd %v %v marshal response err %v", gm_cmd.Id, gm_cmd.String, err.Error())
		return
	}

	w.Write(data)

	if res >= 0 {
		log.Debug("Gm cmd: %v", gm_cmd.String)
	}
}
