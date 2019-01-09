package main

import (
	"sync"
)

type AccountInfo struct {
	account   string
	token     string
	unique_id string
	state     int32 // 0 未登录   1 已登陆   2 已进入游戏
	client_os string
	locker    *sync.RWMutex
}

func (this *AccountInfo) get_account() string {
	this.locker.RLock()
	account := this.account
	this.locker.RUnlock()
	return account
}

func (this *AccountInfo) get_token() string {
	this.locker.RLock()
	token := this.token
	this.locker.RUnlock()
	return token
}

func (this *AccountInfo) get_state() int32 {
	this.locker.RLock()
	state := this.state
	this.locker.RUnlock()
	return state
}

func (this *AccountInfo) set_token(token string) {
	this.locker.Lock()
	defer this.locker.Unlock()
	this.token = token
}

func (this *AccountInfo) set_state(state int32) {
	this.locker.Lock()
	defer this.locker.Unlock()
	this.state = state
}

func (this *AccountInfo) get_client_os() string {
	this.locker.RLock()
	client_os := this.client_os
	defer this.locker.RUnlock()
	return client_os
}

func (this *AccountInfo) set_client_os(client_os string) {
	this.locker.Lock()
	defer this.locker.Unlock()
	this.client_os = client_os
}

var account_mgr map[string]*AccountInfo
var account_locker *sync.RWMutex

func account_mgr_init() {
	account_mgr = make(map[string]*AccountInfo)
	account_locker = &sync.RWMutex{}
}

func account_info_get(account string, first_create bool) *AccountInfo {
	account_locker.RLock()
	account_info := account_mgr[account]
	account_locker.RUnlock()

	if first_create && account_info == nil {
		account_locker.Lock()
		account_info = account_mgr[account]
		// double check
		if account_info == nil {
			account_info = &AccountInfo{
				account: account,
				locker:  &sync.RWMutex{},
			}
			account_mgr[account] = account_info
		}
		account_locker.Unlock()
	}

	return account_info
}

func has_account_login(acc string) bool {
	account_info := account_info_get(acc, false)
	if account_info == nil {
		return false
	}
	state := account_info.get_state()
	if state != 1 {
		return false
	}
	return true
}

func account_login(acc, token, client_os string) {
	account_info := account_info_get(acc, true)
	if account_info == nil {
		return
	}
	account_info.set_state(1)
	account_info.set_token(token)
	account_info.set_client_os(client_os)
}

func account_enter_game(acc string) {
	account_info := account_info_get(acc, false)
	if account_info == nil {
		return
	}
	account_info.set_state(2)
}

func account_logout(acc string) {
	account_info := account_info_get(acc, false)
	if account_info == nil {
		return
	}
	account_info.set_state(0)
}

func account_get_client_os(acc string) (bool, string) {
	account_info := account_info_get(acc, false)
	if account_info == nil {
		return false, ""
	}
	return true, account_info.get_client_os()
}
