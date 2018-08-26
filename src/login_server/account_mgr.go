package main

type AccountInfo struct {
	account string
	token   string
	state   int32 // 0 未登录   1 已登陆   2 已进入游戏
}

var account_mgr map[string]*AccountInfo

func acc_mgr_check_create() map[string]*AccountInfo {
	if account_mgr == nil {
		account_mgr = make(map[string]*AccountInfo)
	}
	return account_mgr
}

func has_account_login(acc string) bool {
	mgr := acc_mgr_check_create()
	if mgr[acc] == nil {
		return false
	}
	if mgr[acc].state != 1 {
		return false
	}
	return true
}

func account_login(acc string) {
	mgr := acc_mgr_check_create()
	if mgr[acc] == nil {
		mgr[acc] = &AccountInfo{
			account: acc,
		}
	}
	mgr[acc].state = 1
}

func get_account(acc string) *AccountInfo {
	mgr := acc_mgr_check_create()
	return mgr[acc]
}
