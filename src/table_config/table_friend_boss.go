package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlFriendBossItem struct {
	Id               int32  `xml:"ID,attr"`
	LevelMin         int32  `xml:"LevelMin,attr"`
	LevelMax         int32  `xml:"LevelMax,attr"`
	SearchBossChance int32  `xml:"SearchBossChance,attr"`
	SearchItemDropID int32  `xml:"SearchItemDropID,attr"`
	BossStageID      int32  `xml:"BossStageID,attr"`
	ChallengeDropID  int32  `xml:"ChallengeDropID,attr"`
	RewardLastHitStr string `xml:"RewardLastHit,attr"`
	RewardLastHit    []int32
	RewardOwnerStr   string `xml:"RewardOwner,attr"`
	RewardOwner      []int32
}

type XmlFriendBossConfig struct {
	Items []XmlFriendBossItem `xml:"item"`
}

type FriendBossTableMgr struct {
	Map   map[int32]*XmlFriendBossItem
	Array []*XmlFriendBossItem
}

func (this *FriendBossTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("FriendBossTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *FriendBossTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "FriendBoss.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("FriendBossTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlFriendBossConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("FriendBossTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlFriendBossItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlFriendBossItem, 0)
	}
	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlFriendBossItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]

		tmp_item.RewardLastHit = parse_xml_str_arr2(tmp_item.RewardLastHitStr, ",")
		if tmp_item.RewardLastHit == nil || len(tmp_item.RewardLastHit)%2 != 0 {
			log.Error("FriendBossTableMgr parse column RewardLastHit with value[%v] failed, index is %v", tmp_item.RewardLastHitStr, idx)
			return false
		}

		tmp_item.RewardOwner = parse_xml_str_arr2(tmp_item.RewardOwnerStr, ",")
		if tmp_item.RewardOwner == nil || len(tmp_item.RewardOwner)%2 != 0 {
			log.Error("FriendBossTableMgr parse column RewardOwner with value[%v] failed, index is %v", tmp_item.RewardOwnerStr, idx)
			return false
		}

		this.Map[tmp_item.Id] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *FriendBossTableMgr) Get(id int32) *XmlFriendBossItem {
	return this.Map[id]
}

func (this *FriendBossTableMgr) GetWithLevel(level int32) *XmlFriendBossItem {
	for i := 0; i < len(this.Array); i++ {
		d := this.Array[i]
		if d.LevelMin <= level && level <= d.LevelMax {
			return d
		}
	}
	return nil
}
