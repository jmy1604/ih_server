package main

import (
	_ "ih_server/libs/log"
	"ih_server/libs/server_conn"
	"ih_server/proto/gen_go/server_message"
	"sync"
)

type LoginInfo struct {
	Id             int32
	Name           string
	ListenGameIP   string
	ListenClientIP string
}

type LoginInfoManager struct {
	logins      map[int32]*LoginInfo
	logins_conn map[*server_conn.ServerConn]*LoginInfo
	locker      *sync.RWMutex
	inited      bool
}

var login_info_mgr LoginInfoManager

func (this *LoginInfoManager) Init() bool {
	if this.inited {
		return true
	}
	this.logins = make(map[int32]*LoginInfo)
	this.logins_conn = make(map[*server_conn.ServerConn]*LoginInfo)
	this.locker = &sync.RWMutex{}
	this.inited = true
	return true
}

func (this *LoginInfoManager) Has(id int32) (ok bool) {
	if !this.inited {
		return
	}
	this.locker.RLock()
	defer this.locker.RUnlock()
	_, o := this.logins[id]
	if o {
		ok = true
	}
	return
}

func (this *LoginInfoManager) Get(id int32) (info *LoginInfo) {
	if !this.inited {
		return
	}

	this.locker.RLock()
	defer this.locker.RUnlock()

	in, o := this.logins[id]
	if !o {
		return
	}

	info = in
	return
}

func (this *LoginInfoManager) Add(conn *server_conn.ServerConn, id int32, name string, listen_game_ip string) (ok bool) {
	if !this.inited {
		return
	}

	this.locker.Lock()
	defer this.locker.Unlock()

	_, o := this.logins[id]
	if o {
		return
	}

	login_info := &LoginInfo{}
	login_info.Id = id
	login_info.ListenGameIP = listen_game_ip
	login_info.Name = name
	this.logins[id] = login_info
	this.logins_conn[conn] = login_info
	conn.T = id

	ok = true
	return
}

func (this *LoginInfoManager) Remove(id int32) (ok bool) {
	if !this.inited {
		return
	}

	this.locker.Lock()
	defer this.locker.Unlock()

	info, o := this.logins[id]
	if !o {
		return
	}

	delete(this.logins, id)

	for k, v := range this.logins_conn {
		if info == v {
			delete(this.logins_conn, k)
			break
		}
	}
	ok = true
	return
}

func (this *LoginInfoManager) RemoveByConn(conn *server_conn.ServerConn) (ok bool) {
	if !this.inited {
		return
	}

	this.locker.Lock()
	defer this.locker.Unlock()

	info, o := this.logins_conn[conn]
	if !o {
		return
	}

	delete(this.logins_conn, conn)

	for k, v := range this.logins {
		if info == v {
			delete(this.logins, k)
			break
		}
	}

	ok = true
	return
}

func (this *LoginInfoManager) GetInfoList() (info_list []*msg_server_message.LoginServerInfo) {
	if !this.inited {
		return
	}

	this.locker.RLock()
	defer this.locker.RUnlock()

	info_list = make([]*msg_server_message.LoginServerInfo, len(this.logins))
	i := 0
	for _, v := range this.logins {
		info_list[i] = &msg_server_message.LoginServerInfo{}
		info_list[i].ServerId = v.Id
		info_list[i].ListenGameIP = v.ListenGameIP
		info_list[i].ListenClientIP = v.ListenClientIP
		i += 1
	}
	return
}
