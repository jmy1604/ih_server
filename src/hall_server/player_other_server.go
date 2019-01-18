package main

type OtherServerPlayerMgr struct {
	players *dbOtherServerPlayerTable
}

var os_player_mgr OtherServerPlayerMgr

func (this *OtherServerPlayerMgr) Init() {
	this.players = dbc.OtherServerPlayers
}

func (this *OtherServerPlayerMgr) GetPlayer(player_id int32) *dbOtherServerPlayerRow {
	if this.players == nil {
		return nil
	}
	row := this.players.GetRow(player_id)
	if row == nil {
		row = this.players.AddRow(player_id)
	}
	return row
}
