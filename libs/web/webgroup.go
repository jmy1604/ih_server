package web

import "ih_server/third_party/code.google.com.protobuf/proto"

type IWebGroup interface {
	RegWebGroup(webid int32, iplayer IWebPlayer, iwebgroup IWebGroup) bool
	RemWebGroup(webid int32, sockclose bool) bool
	GetWebGroupData() proto.Message
	ToG(msg proto.Message)
	ToO(webid int32, msg proto.Message)
}

type WebGroup struct {
	groupid    int32
	maxplayers int32
	webplayers map[int32]IWebPlayer
	website    []int32
}

func (this *WebGroup) GetWebGroupData() proto.Message {
	return nil
}
func (this *WebGroup) GetCurPlayerNumber() int32 {
	return int32(len(this.webplayers))
}
func (this *WebGroup) GetMaxPlayerNumber() int32 {
	return this.maxplayers
}
func (this *WebGroup) GetGroupID() int32 {
	return this.groupid
}
func (this *WebGroup) GetWebPlayers() map[int32]IWebPlayer {
	return this.webplayers
}
func (this *WebGroup) GetWebPlayerByIndex(index int32) (bool, IWebPlayer) {
	if index < 0 || index >= this.maxplayers {
		return false, nil
	}
	webid := this.website[index]
	if webid < 0 {
		return false, nil
	}
	return this.GetWebPlayerByWebid(webid)
}
func (this *WebGroup) GetWebPlayerByWebid(webid int32) (bool, IWebPlayer) {
	value, ok := this.webplayers[webid]
	return ok, value
}
func (this *WebGroup) RegWebGroup(webid int32, iplayer IWebPlayer, iwebgroup IWebGroup) bool {
	if this.maxplayers <= int32(len(this.webplayers)) {
		return false
	}
	_, ok := this.webplayers[webid]
	if ok {
		return false
	}
	this.webplayers[webid] = iplayer
	website := this.setsite(webid)
	iplayer.SetWebSite(website)
	iplayer.ToC(iwebgroup.GetWebGroupData())
	this.ToO(webid, iplayer.GetRegData())
	for _, v := range this.webplayers {
		iplayer.ToC(v.GetRegData())
	}
	return true
}
func (this *WebGroup) RemWebGroup(webid int32, sockclose bool) bool {
	v, ok := this.webplayers[webid]
	if !ok {
		return true
	}
	if sockclose {
		this.ToO(webid, v.GetRemData())
	} else {
		this.ToG(v.GetRemData())
	}
	delete(this.webplayers, webid)
	this.clrsite(webid)
	return true
}
func (this *WebGroup) ToG(msg proto.Message) {
	for _, value := range this.webplayers {
		value.ToC(msg)
	}
}
func (this *WebGroup) ToO(webid int32, msg proto.Message) {
	for id, value := range this.webplayers {
		if id == webid {
			continue
		}
		value.ToC(msg)
	}
}
func (this *WebGroup) setsite(webid int32) int32 {
	for i := int32(0); i < this.maxplayers; i++ {
		if this.website[i] < 0 {
			this.website[i] = webid
			return i
		}
	}
	return -1
}
func (this *WebGroup) clrsite(webid int32) {
	for i := int32(0); i < this.maxplayers; i++ {
		if this.website[i] == webid {
			this.website[i] = -1
			return
		}
	}
}

func NewWebGroup(groupid int32, maxplayers int32) *WebGroup {
	if maxplayers <= 0 {
		return nil
	}
	web := &WebGroup{}
	web.groupid = groupid
	web.maxplayers = maxplayers
	web.webplayers = make(map[int32]IWebPlayer, maxplayers)
	web.website = make([]int32, maxplayers)
	for i := int32(0); i < maxplayers; i++ {
		web.website[i] = -1
	}
	return web
}
