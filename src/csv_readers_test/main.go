package main

import (
	"ih_server/src/csv_readers"
	"log"
)

func main() {
	var ac_mgr csv_readers.AccelcostMgr
	if !ac_mgr.Read("") {
		log.Printf("读取表AccelcostMgr失败\n")
		return
	}

	for i := int32(0); i < ac_mgr.GetNum(); i++ {
		ac := ac_mgr.GetByIndex(i)
		if ac == nil {
			continue
		}
		log.Printf("index %v data: %v", i, ac)
	}
}
