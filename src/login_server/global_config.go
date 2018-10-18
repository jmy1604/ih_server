package main

import (
	"encoding/json"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type GlobalConfig struct {
	PlayerInitCards         []int32
	ArenaGoldAwardTimeLimit int32
	InitCoin                int32
	InitDiamond             int32
	ChestSlotCount          int32
	TimeExchangeRate        int32
	WoodChestUnlockTime     int32
	SilverChestUnlockTime   int32
	GoldenChestUnlockTime   int32
	GiantChestUnlockTime    int32
	MagicChestUnlockTime    int32
	RareChestUnlockTime     int32
	EpicChestUnlockTime     int32
	LegendryChestUnlockTime int32

	GoogleLoginVerifyUrl   string
	AppleLoginVerifyUrl    string
	FaceBookLoginVerifyUrl string
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

	return true
}
