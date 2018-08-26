package web

import "ih_server/third_party/code.google.com.protobuf/proto"

type IWebPlayer interface {
	ToC(msg proto.Message) bool
	GetRegData() proto.Message
	GetRemData() proto.Message
	SetWebSite(website int32)
	GetWebSite() int32
}

type WebPlayer struct {
	webid    int32
	webgroup IWebGroup
	website  int32
}

func (this *WebPlayer) ToC(msg proto.Message) bool {
	return false
}
func (this *WebPlayer) GetRegData() proto.Message {
	return nil
}
func (this *WebPlayer) GetRemData() proto.Message {
	return nil
}
func (this *WebPlayer) GetWebID() int32 {
	return this.webid
}
func (this *WebPlayer) GetWebSite() int32 {
	return this.website
}
func (this *WebPlayer) SetWebSite(website int32) {
	this.website = website
}
func (this *WebPlayer) ToG(msg proto.Message) bool {
	if this.webgroup == nil {
		return false
	}
	this.webgroup.ToG(msg)
	return true
}
func (this *WebPlayer) ToO(msg proto.Message) bool {
	if this.webgroup == nil {
		return false
	}
	this.webgroup.ToO(this.webid, msg)
	return true
}
func (this *WebPlayer) OnRegWebGroup(webgroup IWebGroup, iplayer IWebPlayer) bool {
	if this.webgroup != nil {
		return false
	}
	ok := webgroup.RegWebGroup(this.webid, iplayer, webgroup)
	if !ok {
		return false
	}
	this.webgroup = webgroup
	return ok
}
func (this *WebPlayer) OnRemWebGroup(sockclose bool) bool {
	if this.webgroup == nil {
		return false
	}
	if !this.webgroup.RemWebGroup(this.webid, sockclose) {
		return false
	}
	this.website = -1
	this.webgroup = nil
	return true
}

func NewWebPlayer(webid int32) *WebPlayer {
	if webid < 0 {
		return nil
	}
	return &WebPlayer{webid, nil, -1}
}
