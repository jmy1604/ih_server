package table_config

import (
	"encoding/xml"
	"ih_server/libs/log"
	"ih_server/src/server_config"
	"io/ioutil"
)

type XmlArtifactUnlockItem struct {
	ArtifactId       int32  `xml:"ArtifactID,attr"`
	UnLockLevel      int32  `xml:"UnLockLevel,attr"`
	UnLockVIPLevel   int32  `xml:"UnLockVIPLevel,attr"`
	UnLockResCostStr string `xml:"UnLockResCost,attr"`
	UnLockResCost    []int32
	MaxRank          int32 `xml:"MaxRank,attr"`
}

type XmlArtifactUnlockConfig struct {
	Items []XmlArtifactUnlockItem `xml:"item"`
}

type ArtifactUnlockTableMgr struct {
	Map   map[int32]*XmlArtifactUnlockItem
	Array []*XmlArtifactUnlockItem
}

func (this *ArtifactUnlockTableMgr) Init(table_file string) bool {
	if !this.Load(table_file) {
		log.Error("ArtifactUnlockTableMgr Init load failed !")
		return false
	}
	return true
}

func (this *ArtifactUnlockTableMgr) Load(table_file string) bool {
	if table_file == "" {
		table_file = "ArtifactUnlock.xml"
	}
	table_path := server_config.GetGameDataPathFile(table_file)
	data, err := ioutil.ReadFile(table_path)
	if nil != err {
		log.Error("ArtifactUnlockTableMgr read file err[%s] !", err.Error())
		return false
	}

	tmp_cfg := &XmlArtifactUnlockConfig{}
	err = xml.Unmarshal(data, tmp_cfg)
	if nil != err {
		log.Error("ArtifactUnlockTableMgr xml Unmarshal failed error [%s] !", err.Error())
		return false
	}

	if this.Map == nil {
		this.Map = make(map[int32]*XmlArtifactUnlockItem)
	}
	if this.Array == nil {
		this.Array = make([]*XmlArtifactUnlockItem, 0)
	}

	tmp_len := int32(len(tmp_cfg.Items))

	var tmp_item *XmlArtifactUnlockItem
	for idx := int32(0); idx < tmp_len; idx++ {
		tmp_item = &tmp_cfg.Items[idx]
		tmp_item.UnLockResCost = parse_xml_str_arr2(tmp_item.UnLockResCostStr, ",")
		if tmp_item.UnLockResCost != nil && len(tmp_item.UnLockResCost)%2 > 0 {
			log.Error("ArtifactUnlockTableMgr parse column UnLockResCost data %v with row %v failed", tmp_item.UnLockResCostStr, idx+1)
			return false
		}

		this.Map[tmp_item.ArtifactId] = tmp_item
		this.Array = append(this.Array, tmp_item)
	}

	return true
}

func (this *ArtifactUnlockTableMgr) Get(id int32) *XmlArtifactUnlockItem {
	return this.Map[id]
}
