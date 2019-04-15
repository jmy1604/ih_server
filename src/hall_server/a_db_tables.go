package main

import (
	"github.com/golang/protobuf/proto"
	_ "github.com/go-sql-driver/mysql"
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"ih_server/libs/log"
	"math/rand"
	"os"
	"os/exec"
	"ih_server/proto/gen_go/db_hall"
	"strings"
	"sync/atomic"
	"time"
)

type dbArgs struct {
	args  []interface{}
	count int32
}

func new_db_args(count int32) (this *dbArgs) {
	this = &dbArgs{}
	this.args = make([]interface{}, count)
	this.count = 0
	return this
}
func (this *dbArgs) Push(arg interface{}) {
	this.args[this.count] = arg
	this.count++
}
func (this *dbArgs) GetArgs() (args []interface{}) {
	return this.args[0:this.count]
}
func (this *DBC) StmtPrepare(s string) (r *sql.Stmt, e error) {
	this.m_db_lock.Lock("DBC.StmtPrepare")
	defer this.m_db_lock.Unlock()
	return this.m_db.Prepare(s)
}
func (this *DBC) StmtExec(stmt *sql.Stmt, args ...interface{}) (r sql.Result, err error) {
	this.m_db_lock.Lock("DBC.StmtExec")
	defer this.m_db_lock.Unlock()
	return stmt.Exec(args...)
}
func (this *DBC) StmtQuery(stmt *sql.Stmt, args ...interface{}) (r *sql.Rows, err error) {
	this.m_db_lock.Lock("DBC.StmtQuery")
	defer this.m_db_lock.Unlock()
	return stmt.Query(args...)
}
func (this *DBC) StmtQueryRow(stmt *sql.Stmt, args ...interface{}) (r *sql.Row) {
	this.m_db_lock.Lock("DBC.StmtQueryRow")
	defer this.m_db_lock.Unlock()
	return stmt.QueryRow(args...)
}
func (this *DBC) Query(s string, args ...interface{}) (r *sql.Rows, e error) {
	this.m_db_lock.Lock("DBC.Query")
	defer this.m_db_lock.Unlock()
	return this.m_db.Query(s, args...)
}
func (this *DBC) QueryRow(s string, args ...interface{}) (r *sql.Row) {
	this.m_db_lock.Lock("DBC.QueryRow")
	defer this.m_db_lock.Unlock()
	return this.m_db.QueryRow(s, args...)
}
func (this *DBC) Exec(s string, args ...interface{}) (r sql.Result, e error) {
	this.m_db_lock.Lock("DBC.Exec")
	defer this.m_db_lock.Unlock()
	return this.m_db.Exec(s, args...)
}
func (this *DBC) Conn(name string, addr string, acc string, pwd string, db_copy_path string) (err error) {
	log.Trace("%v %v %v %v", name, addr, acc, pwd)
	this.m_db_name = name
	source := acc + ":" + pwd + "@tcp(" + addr + ")/" + name + "?charset=utf8"
	this.m_db, err = sql.Open("mysql", source)
	if err != nil {
		log.Error("open db failed %v", err)
		return
	}
	
	this.m_db.SetConnMaxLifetime(time.Second * 5)

	this.m_db_lock = NewMutex()
	this.m_shutdown_lock = NewMutex()

	if config.DBCST_MAX-config.DBCST_MIN <= 1 {
		return errors.New("DBCST_MAX sub DBCST_MIN should greater than 1s")
	}

	err = this.init_tables()
	if err != nil {
		log.Error("init tables failed")
		return
	}

	if os.MkdirAll(db_copy_path, os.ModePerm) == nil {
		os.Chmod(db_copy_path, os.ModePerm)
	}
	
	this.m_db_last_copy_time = int32(time.Now().Hour())
	this.m_db_copy_path = db_copy_path
	addr_list := strings.Split(addr, ":")
	this.m_db_addr = addr_list[0]
	this.m_db_account = acc
	this.m_db_password = pwd
	this.m_initialized = true

	return
}
func (this *DBC) check_files_exist() (file_name string) {
	f_name := fmt.Sprintf("%v/%v_%v", this.m_db_copy_path, this.m_db_name, time.Now().Format("20060102-15"))
	num := int32(0)
	for {
		if num == 0 {
			file_name = f_name
		} else {
			file_name = f_name + fmt.Sprintf("_%v", num)
		}
		_, err := os.Lstat(file_name)
		if err != nil {
			break
		}
		num++
	}
	return file_name
}
func (this *DBC) Loop() {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}

		log.Trace("数据库主循环退出")
		this.m_shutdown_completed = true
	}()

	for {
		t := config.DBCST_MIN + rand.Intn(config.DBCST_MAX-config.DBCST_MIN)
		if t <= 0 {
			t = 600
		}

		for i := 0; i < t; i++ {
			time.Sleep(time.Second)
			if this.m_quit {
				break
			}
		}

		if this.m_quit {
			break
		}

		begin := time.Now()
		err := this.Save(false)
		if err != nil {
			log.Error("save db failed %v", err)
		}
		log.Trace("db存数据花费时长: %vms", time.Now().Sub(begin).Nanoseconds()/1000000)
		
		now_time := time.Now()
		if int32(now_time.Unix())-24*3600 >= this.m_db_last_copy_time {
			args := []string {
				fmt.Sprintf("-h%v", this.m_db_addr),
				fmt.Sprintf("-u%v", this.m_db_account),
				fmt.Sprintf("-p%v", this.m_db_password),
				this.m_db_name,
			}
			cmd := exec.Command("mysqldump", args...)
			var out bytes.Buffer
			cmd.Stdout = &out
			cmd_err := cmd.Run()
			if cmd_err == nil {
				file_name := this.check_files_exist()
				file, file_err := os.OpenFile(file_name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0660)
				defer file.Close()
				if file_err == nil {
					_, write_err := file.Write(out.Bytes())
					if write_err == nil {
						log.Trace("数据库备份成功！备份文件名:%v", file_name)
					} else {
						log.Error("数据库备份文件写入失败！备份文件名%v", file_name)
					}
				} else {
					log.Error("数据库备份文件打开失败！备份文件名%v", file_name)
				}
				file.Close()
			} else {
				log.Error("数据库备份失败！")
			}
			this.m_db_last_copy_time = int32(now_time.Unix())
		}
		
		if this.m_quit {
			break
		}
	}

	log.Trace("数据库缓存主循环退出，保存所有数据")

	err := this.Save(true)
	if err != nil {
		log.Error("shutdwon save db failed %v", err)
		return
	}

	err = this.m_db.Close()
	if err != nil {
		log.Error("close db failed %v", err)
		return
	}
}
func (this *DBC) Shutdown() {
	if !this.m_initialized {
		return
	}

	this.m_shutdown_lock.UnSafeLock("DBC.Shutdown")
	defer this.m_shutdown_lock.UnSafeUnlock()

	if this.m_quit {
		return
	}
	this.m_quit = true

	log.Trace("关闭数据库缓存")

	begin := time.Now()

	for {
		if this.m_shutdown_completed {
			break
		}

		time.Sleep(time.Millisecond * 100)
	}

	log.Trace("关闭数据库缓存耗时 %v 秒", time.Now().Sub(begin).Seconds())
}


const DBC_VERSION = 1
const DBC_SUB_VERSION = 0

type dbPlayerInfoData struct{
	Lvl int32
	Exp int32
	CreateUnix int32
	Gold int32
	Diamond int32
	LastLogout int32
	LastLogin int32
	VipLvl int32
	Head int32
}
func (this* dbPlayerInfoData)from_pb(pb *db.PlayerInfo){
	if pb == nil {
		return
	}
	this.Lvl = pb.GetLvl()
	this.Exp = pb.GetExp()
	this.CreateUnix = pb.GetCreateUnix()
	this.Gold = pb.GetGold()
	this.Diamond = pb.GetDiamond()
	this.LastLogout = pb.GetLastLogout()
	this.LastLogin = pb.GetLastLogin()
	this.VipLvl = pb.GetVipLvl()
	this.Head = pb.GetHead()
	return
}
func (this* dbPlayerInfoData)to_pb()(pb *db.PlayerInfo){
	pb = &db.PlayerInfo{}
	pb.Lvl = proto.Int32(this.Lvl)
	pb.Exp = proto.Int32(this.Exp)
	pb.CreateUnix = proto.Int32(this.CreateUnix)
	pb.Gold = proto.Int32(this.Gold)
	pb.Diamond = proto.Int32(this.Diamond)
	pb.LastLogout = proto.Int32(this.LastLogout)
	pb.LastLogin = proto.Int32(this.LastLogin)
	pb.VipLvl = proto.Int32(this.VipLvl)
	pb.Head = proto.Int32(this.Head)
	return
}
func (this* dbPlayerInfoData)clone_to(d *dbPlayerInfoData){
	d.Lvl = this.Lvl
	d.Exp = this.Exp
	d.CreateUnix = this.CreateUnix
	d.Gold = this.Gold
	d.Diamond = this.Diamond
	d.LastLogout = this.LastLogout
	d.LastLogin = this.LastLogin
	d.VipLvl = this.VipLvl
	d.Head = this.Head
	return
}
type dbPlayerGlobalData struct{
	CurrentRoleId int32
}
func (this* dbPlayerGlobalData)from_pb(pb *db.PlayerGlobal){
	if pb == nil {
		return
	}
	this.CurrentRoleId = pb.GetCurrentRoleId()
	return
}
func (this* dbPlayerGlobalData)to_pb()(pb *db.PlayerGlobal){
	pb = &db.PlayerGlobal{}
	pb.CurrentRoleId = proto.Int32(this.CurrentRoleId)
	return
}
func (this* dbPlayerGlobalData)clone_to(d *dbPlayerGlobalData){
	d.CurrentRoleId = this.CurrentRoleId
	return
}
type dbPlayerItemData struct{
	Id int32
	Count int32
}
func (this* dbPlayerItemData)from_pb(pb *db.PlayerItem){
	if pb == nil {
		return
	}
	this.Id = pb.GetId()
	this.Count = pb.GetCount()
	return
}
func (this* dbPlayerItemData)to_pb()(pb *db.PlayerItem){
	pb = &db.PlayerItem{}
	pb.Id = proto.Int32(this.Id)
	pb.Count = proto.Int32(this.Count)
	return
}
func (this* dbPlayerItemData)clone_to(d *dbPlayerItemData){
	d.Id = this.Id
	d.Count = this.Count
	return
}
type dbPlayerRoleCommonData struct{
	DisplaceRoleId int32
	DisplacedNewRoleTableId int32
	DisplaceGroupId int32
	PowerUpdateTime int32
}
func (this* dbPlayerRoleCommonData)from_pb(pb *db.PlayerRoleCommon){
	if pb == nil {
		return
	}
	this.DisplaceRoleId = pb.GetDisplaceRoleId()
	this.DisplacedNewRoleTableId = pb.GetDisplacedNewRoleTableId()
	this.DisplaceGroupId = pb.GetDisplaceGroupId()
	this.PowerUpdateTime = pb.GetPowerUpdateTime()
	return
}
func (this* dbPlayerRoleCommonData)to_pb()(pb *db.PlayerRoleCommon){
	pb = &db.PlayerRoleCommon{}
	pb.DisplaceRoleId = proto.Int32(this.DisplaceRoleId)
	pb.DisplacedNewRoleTableId = proto.Int32(this.DisplacedNewRoleTableId)
	pb.DisplaceGroupId = proto.Int32(this.DisplaceGroupId)
	pb.PowerUpdateTime = proto.Int32(this.PowerUpdateTime)
	return
}
func (this* dbPlayerRoleCommonData)clone_to(d *dbPlayerRoleCommonData){
	d.DisplaceRoleId = this.DisplaceRoleId
	d.DisplacedNewRoleTableId = this.DisplacedNewRoleTableId
	d.DisplaceGroupId = this.DisplaceGroupId
	d.PowerUpdateTime = this.PowerUpdateTime
	return
}
type dbPlayerRoleData struct{
	Id int32
	TableId int32
	Rank int32
	Level int32
	Equip []int32
	IsLock int32
	State int32
}
func (this* dbPlayerRoleData)from_pb(pb *db.PlayerRole){
	if pb == nil {
		this.Equip = make([]int32,0)
		return
	}
	this.Id = pb.GetId()
	this.TableId = pb.GetTableId()
	this.Rank = pb.GetRank()
	this.Level = pb.GetLevel()
	this.Equip = make([]int32,len(pb.GetEquip()))
	for i, v := range pb.GetEquip() {
		this.Equip[i] = v
	}
	this.IsLock = pb.GetIsLock()
	this.State = pb.GetState()
	return
}
func (this* dbPlayerRoleData)to_pb()(pb *db.PlayerRole){
	pb = &db.PlayerRole{}
	pb.Id = proto.Int32(this.Id)
	pb.TableId = proto.Int32(this.TableId)
	pb.Rank = proto.Int32(this.Rank)
	pb.Level = proto.Int32(this.Level)
	pb.Equip = make([]int32, len(this.Equip))
	for i, v := range this.Equip {
		pb.Equip[i]=v
	}
	pb.IsLock = proto.Int32(this.IsLock)
	pb.State = proto.Int32(this.State)
	return
}
func (this* dbPlayerRoleData)clone_to(d *dbPlayerRoleData){
	d.Id = this.Id
	d.TableId = this.TableId
	d.Rank = this.Rank
	d.Level = this.Level
	d.Equip = make([]int32, len(this.Equip))
	for _ii, _vv := range this.Equip {
		d.Equip[_ii]=_vv
	}
	d.IsLock = this.IsLock
	d.State = this.State
	return
}
type dbPlayerRoleHandbookData struct{
	Role []int32
}
func (this* dbPlayerRoleHandbookData)from_pb(pb *db.PlayerRoleHandbook){
	if pb == nil {
		this.Role = make([]int32,0)
		return
	}
	this.Role = make([]int32,len(pb.GetRole()))
	for i, v := range pb.GetRole() {
		this.Role[i] = v
	}
	return
}
func (this* dbPlayerRoleHandbookData)to_pb()(pb *db.PlayerRoleHandbook){
	pb = &db.PlayerRoleHandbook{}
	pb.Role = make([]int32, len(this.Role))
	for i, v := range this.Role {
		pb.Role[i]=v
	}
	return
}
func (this* dbPlayerRoleHandbookData)clone_to(d *dbPlayerRoleHandbookData){
	d.Role = make([]int32, len(this.Role))
	for _ii, _vv := range this.Role {
		d.Role[_ii]=_vv
	}
	return
}
type dbPlayerBattleTeamData struct{
	DefenseMembers []int32
	CampaignMembers []int32
	DefenseArtifactId int32
	CampaignArtifactId int32
}
func (this* dbPlayerBattleTeamData)from_pb(pb *db.PlayerBattleTeam){
	if pb == nil {
		this.DefenseMembers = make([]int32,0)
		this.CampaignMembers = make([]int32,0)
		return
	}
	this.DefenseMembers = make([]int32,len(pb.GetDefenseMembers()))
	for i, v := range pb.GetDefenseMembers() {
		this.DefenseMembers[i] = v
	}
	this.CampaignMembers = make([]int32,len(pb.GetCampaignMembers()))
	for i, v := range pb.GetCampaignMembers() {
		this.CampaignMembers[i] = v
	}
	this.DefenseArtifactId = pb.GetDefenseArtifactId()
	this.CampaignArtifactId = pb.GetCampaignArtifactId()
	return
}
func (this* dbPlayerBattleTeamData)to_pb()(pb *db.PlayerBattleTeam){
	pb = &db.PlayerBattleTeam{}
	pb.DefenseMembers = make([]int32, len(this.DefenseMembers))
	for i, v := range this.DefenseMembers {
		pb.DefenseMembers[i]=v
	}
	pb.CampaignMembers = make([]int32, len(this.CampaignMembers))
	for i, v := range this.CampaignMembers {
		pb.CampaignMembers[i]=v
	}
	pb.DefenseArtifactId = proto.Int32(this.DefenseArtifactId)
	pb.CampaignArtifactId = proto.Int32(this.CampaignArtifactId)
	return
}
func (this* dbPlayerBattleTeamData)clone_to(d *dbPlayerBattleTeamData){
	d.DefenseMembers = make([]int32, len(this.DefenseMembers))
	for _ii, _vv := range this.DefenseMembers {
		d.DefenseMembers[_ii]=_vv
	}
	d.CampaignMembers = make([]int32, len(this.CampaignMembers))
	for _ii, _vv := range this.CampaignMembers {
		d.CampaignMembers[_ii]=_vv
	}
	d.DefenseArtifactId = this.DefenseArtifactId
	d.CampaignArtifactId = this.CampaignArtifactId
	return
}
type dbPlayerCampaignCommonData struct{
	CurrentCampaignId int32
	HangupLastDropStaticIncomeTime int32
	HangupLastDropRandomIncomeTime int32
	HangupCampaignId int32
	LastestPassedCampaignId int32
	RankSerialId int32
	VipAccelNum int32
	VipAccelRefreshTime int32
	PassCampaginTime int32
}
func (this* dbPlayerCampaignCommonData)from_pb(pb *db.PlayerCampaignCommon){
	if pb == nil {
		return
	}
	this.CurrentCampaignId = pb.GetCurrentCampaignId()
	this.HangupLastDropStaticIncomeTime = pb.GetHangupLastDropStaticIncomeTime()
	this.HangupLastDropRandomIncomeTime = pb.GetHangupLastDropRandomIncomeTime()
	this.HangupCampaignId = pb.GetHangupCampaignId()
	this.LastestPassedCampaignId = pb.GetLastestPassedCampaignId()
	this.RankSerialId = pb.GetRankSerialId()
	this.VipAccelNum = pb.GetVipAccelNum()
	this.VipAccelRefreshTime = pb.GetVipAccelRefreshTime()
	this.PassCampaginTime = pb.GetPassCampaginTime()
	return
}
func (this* dbPlayerCampaignCommonData)to_pb()(pb *db.PlayerCampaignCommon){
	pb = &db.PlayerCampaignCommon{}
	pb.CurrentCampaignId = proto.Int32(this.CurrentCampaignId)
	pb.HangupLastDropStaticIncomeTime = proto.Int32(this.HangupLastDropStaticIncomeTime)
	pb.HangupLastDropRandomIncomeTime = proto.Int32(this.HangupLastDropRandomIncomeTime)
	pb.HangupCampaignId = proto.Int32(this.HangupCampaignId)
	pb.LastestPassedCampaignId = proto.Int32(this.LastestPassedCampaignId)
	pb.RankSerialId = proto.Int32(this.RankSerialId)
	pb.VipAccelNum = proto.Int32(this.VipAccelNum)
	pb.VipAccelRefreshTime = proto.Int32(this.VipAccelRefreshTime)
	pb.PassCampaginTime = proto.Int32(this.PassCampaginTime)
	return
}
func (this* dbPlayerCampaignCommonData)clone_to(d *dbPlayerCampaignCommonData){
	d.CurrentCampaignId = this.CurrentCampaignId
	d.HangupLastDropStaticIncomeTime = this.HangupLastDropStaticIncomeTime
	d.HangupLastDropRandomIncomeTime = this.HangupLastDropRandomIncomeTime
	d.HangupCampaignId = this.HangupCampaignId
	d.LastestPassedCampaignId = this.LastestPassedCampaignId
	d.RankSerialId = this.RankSerialId
	d.VipAccelNum = this.VipAccelNum
	d.VipAccelRefreshTime = this.VipAccelRefreshTime
	d.PassCampaginTime = this.PassCampaginTime
	return
}
type dbPlayerCampaignData struct{
	CampaignId int32
}
func (this* dbPlayerCampaignData)from_pb(pb *db.PlayerCampaign){
	if pb == nil {
		return
	}
	this.CampaignId = pb.GetCampaignId()
	return
}
func (this* dbPlayerCampaignData)to_pb()(pb *db.PlayerCampaign){
	pb = &db.PlayerCampaign{}
	pb.CampaignId = proto.Int32(this.CampaignId)
	return
}
func (this* dbPlayerCampaignData)clone_to(d *dbPlayerCampaignData){
	d.CampaignId = this.CampaignId
	return
}
type dbPlayerCampaignStaticIncomeData struct{
	ItemId int32
	ItemNum int32
}
func (this* dbPlayerCampaignStaticIncomeData)from_pb(pb *db.PlayerCampaignStaticIncome){
	if pb == nil {
		return
	}
	this.ItemId = pb.GetItemId()
	this.ItemNum = pb.GetItemNum()
	return
}
func (this* dbPlayerCampaignStaticIncomeData)to_pb()(pb *db.PlayerCampaignStaticIncome){
	pb = &db.PlayerCampaignStaticIncome{}
	pb.ItemId = proto.Int32(this.ItemId)
	pb.ItemNum = proto.Int32(this.ItemNum)
	return
}
func (this* dbPlayerCampaignStaticIncomeData)clone_to(d *dbPlayerCampaignStaticIncomeData){
	d.ItemId = this.ItemId
	d.ItemNum = this.ItemNum
	return
}
type dbPlayerCampaignRandomIncomeData struct{
	ItemId int32
	ItemNum int32
}
func (this* dbPlayerCampaignRandomIncomeData)from_pb(pb *db.PlayerCampaignRandomIncome){
	if pb == nil {
		return
	}
	this.ItemId = pb.GetItemId()
	this.ItemNum = pb.GetItemNum()
	return
}
func (this* dbPlayerCampaignRandomIncomeData)to_pb()(pb *db.PlayerCampaignRandomIncome){
	pb = &db.PlayerCampaignRandomIncome{}
	pb.ItemId = proto.Int32(this.ItemId)
	pb.ItemNum = proto.Int32(this.ItemNum)
	return
}
func (this* dbPlayerCampaignRandomIncomeData)clone_to(d *dbPlayerCampaignRandomIncomeData){
	d.ItemId = this.ItemId
	d.ItemNum = this.ItemNum
	return
}
type dbPlayerMailCommonData struct{
	CurrId int32
	LastSendPlayerMailTime int32
}
func (this* dbPlayerMailCommonData)from_pb(pb *db.PlayerMailCommon){
	if pb == nil {
		return
	}
	this.CurrId = pb.GetCurrId()
	this.LastSendPlayerMailTime = pb.GetLastSendPlayerMailTime()
	return
}
func (this* dbPlayerMailCommonData)to_pb()(pb *db.PlayerMailCommon){
	pb = &db.PlayerMailCommon{}
	pb.CurrId = proto.Int32(this.CurrId)
	pb.LastSendPlayerMailTime = proto.Int32(this.LastSendPlayerMailTime)
	return
}
func (this* dbPlayerMailCommonData)clone_to(d *dbPlayerMailCommonData){
	d.CurrId = this.CurrId
	d.LastSendPlayerMailTime = this.LastSendPlayerMailTime
	return
}
type dbPlayerMailData struct{
	Id int32
	Type int8
	Title string
	Content string
	SendUnix int32
	AttachItemIds []int32
	AttachItemNums []int32
	IsRead int32
	IsGetAttached int32
	SenderId int32
	SenderName string
	Subtype int32
	ExtraValue int32
}
func (this* dbPlayerMailData)from_pb(pb *db.PlayerMail){
	if pb == nil {
		this.AttachItemIds = make([]int32,0)
		this.AttachItemNums = make([]int32,0)
		return
	}
	this.Id = pb.GetId()
	this.Type = int8(pb.GetType())
	this.Title = pb.GetTitle()
	this.Content = pb.GetContent()
	this.SendUnix = pb.GetSendUnix()
	this.AttachItemIds = make([]int32,len(pb.GetAttachItemIds()))
	for i, v := range pb.GetAttachItemIds() {
		this.AttachItemIds[i] = v
	}
	this.AttachItemNums = make([]int32,len(pb.GetAttachItemNums()))
	for i, v := range pb.GetAttachItemNums() {
		this.AttachItemNums[i] = v
	}
	this.IsRead = pb.GetIsRead()
	this.IsGetAttached = pb.GetIsGetAttached()
	this.SenderId = pb.GetSenderId()
	this.SenderName = pb.GetSenderName()
	this.Subtype = pb.GetSubtype()
	this.ExtraValue = pb.GetExtraValue()
	return
}
func (this* dbPlayerMailData)to_pb()(pb *db.PlayerMail){
	pb = &db.PlayerMail{}
	pb.Id = proto.Int32(this.Id)
	temp_Type:=int32(this.Type)
	pb.Type = proto.Int32(temp_Type)
	pb.Title = proto.String(this.Title)
	pb.Content = proto.String(this.Content)
	pb.SendUnix = proto.Int32(this.SendUnix)
	pb.AttachItemIds = make([]int32, len(this.AttachItemIds))
	for i, v := range this.AttachItemIds {
		pb.AttachItemIds[i]=v
	}
	pb.AttachItemNums = make([]int32, len(this.AttachItemNums))
	for i, v := range this.AttachItemNums {
		pb.AttachItemNums[i]=v
	}
	pb.IsRead = proto.Int32(this.IsRead)
	pb.IsGetAttached = proto.Int32(this.IsGetAttached)
	pb.SenderId = proto.Int32(this.SenderId)
	pb.SenderName = proto.String(this.SenderName)
	pb.Subtype = proto.Int32(this.Subtype)
	pb.ExtraValue = proto.Int32(this.ExtraValue)
	return
}
func (this* dbPlayerMailData)clone_to(d *dbPlayerMailData){
	d.Id = this.Id
	d.Type = int8(this.Type)
	d.Title = this.Title
	d.Content = this.Content
	d.SendUnix = this.SendUnix
	d.AttachItemIds = make([]int32, len(this.AttachItemIds))
	for _ii, _vv := range this.AttachItemIds {
		d.AttachItemIds[_ii]=_vv
	}
	d.AttachItemNums = make([]int32, len(this.AttachItemNums))
	for _ii, _vv := range this.AttachItemNums {
		d.AttachItemNums[_ii]=_vv
	}
	d.IsRead = this.IsRead
	d.IsGetAttached = this.IsGetAttached
	d.SenderId = this.SenderId
	d.SenderName = this.SenderName
	d.Subtype = this.Subtype
	d.ExtraValue = this.ExtraValue
	return
}
type dbPlayerBattleSaveData struct{
	Id int32
	Side int32
	SaveTime int32
}
func (this* dbPlayerBattleSaveData)from_pb(pb *db.PlayerBattleSave){
	if pb == nil {
		return
	}
	this.Id = pb.GetId()
	this.Side = pb.GetSide()
	this.SaveTime = pb.GetSaveTime()
	return
}
func (this* dbPlayerBattleSaveData)to_pb()(pb *db.PlayerBattleSave){
	pb = &db.PlayerBattleSave{}
	pb.Id = proto.Int32(this.Id)
	pb.Side = proto.Int32(this.Side)
	pb.SaveTime = proto.Int32(this.SaveTime)
	return
}
func (this* dbPlayerBattleSaveData)clone_to(d *dbPlayerBattleSaveData){
	d.Id = this.Id
	d.Side = this.Side
	d.SaveTime = this.SaveTime
	return
}
type dbPlayerTalentData struct{
	Id int32
	Level int32
}
func (this* dbPlayerTalentData)from_pb(pb *db.PlayerTalent){
	if pb == nil {
		return
	}
	this.Id = pb.GetId()
	this.Level = pb.GetLevel()
	return
}
func (this* dbPlayerTalentData)to_pb()(pb *db.PlayerTalent){
	pb = &db.PlayerTalent{}
	pb.Id = proto.Int32(this.Id)
	pb.Level = proto.Int32(this.Level)
	return
}
func (this* dbPlayerTalentData)clone_to(d *dbPlayerTalentData){
	d.Id = this.Id
	d.Level = this.Level
	return
}
type dbPlayerTowerCommonData struct{
	CurrId int32
	Keys int32
	LastGetNewKeyTime int32
	RankSerialId int32
	PassTowerTime int32
}
func (this* dbPlayerTowerCommonData)from_pb(pb *db.PlayerTowerCommon){
	if pb == nil {
		return
	}
	this.CurrId = pb.GetCurrId()
	this.Keys = pb.GetKeys()
	this.LastGetNewKeyTime = pb.GetLastGetNewKeyTime()
	this.RankSerialId = pb.GetRankSerialId()
	this.PassTowerTime = pb.GetPassTowerTime()
	return
}
func (this* dbPlayerTowerCommonData)to_pb()(pb *db.PlayerTowerCommon){
	pb = &db.PlayerTowerCommon{}
	pb.CurrId = proto.Int32(this.CurrId)
	pb.Keys = proto.Int32(this.Keys)
	pb.LastGetNewKeyTime = proto.Int32(this.LastGetNewKeyTime)
	pb.RankSerialId = proto.Int32(this.RankSerialId)
	pb.PassTowerTime = proto.Int32(this.PassTowerTime)
	return
}
func (this* dbPlayerTowerCommonData)clone_to(d *dbPlayerTowerCommonData){
	d.CurrId = this.CurrId
	d.Keys = this.Keys
	d.LastGetNewKeyTime = this.LastGetNewKeyTime
	d.RankSerialId = this.RankSerialId
	d.PassTowerTime = this.PassTowerTime
	return
}
type dbPlayerTowerData struct{
	Id int32
}
func (this* dbPlayerTowerData)from_pb(pb *db.PlayerTower){
	if pb == nil {
		return
	}
	this.Id = pb.GetId()
	return
}
func (this* dbPlayerTowerData)to_pb()(pb *db.PlayerTower){
	pb = &db.PlayerTower{}
	pb.Id = proto.Int32(this.Id)
	return
}
func (this* dbPlayerTowerData)clone_to(d *dbPlayerTowerData){
	d.Id = this.Id
	return
}
type dbPlayerDrawData struct{
	Type int32
	LastDrawTime int32
	Num int32
}
func (this* dbPlayerDrawData)from_pb(pb *db.PlayerDraw){
	if pb == nil {
		return
	}
	this.Type = pb.GetType()
	this.LastDrawTime = pb.GetLastDrawTime()
	this.Num = pb.GetNum()
	return
}
func (this* dbPlayerDrawData)to_pb()(pb *db.PlayerDraw){
	pb = &db.PlayerDraw{}
	pb.Type = proto.Int32(this.Type)
	pb.LastDrawTime = proto.Int32(this.LastDrawTime)
	pb.Num = proto.Int32(this.Num)
	return
}
func (this* dbPlayerDrawData)clone_to(d *dbPlayerDrawData){
	d.Type = this.Type
	d.LastDrawTime = this.LastDrawTime
	d.Num = this.Num
	return
}
type dbPlayerGoldHandData struct{
	LastRefreshTime int32
	LeftNum []int32
}
func (this* dbPlayerGoldHandData)from_pb(pb *db.PlayerGoldHand){
	if pb == nil {
		this.LeftNum = make([]int32,0)
		return
	}
	this.LastRefreshTime = pb.GetLastRefreshTime()
	this.LeftNum = make([]int32,len(pb.GetLeftNum()))
	for i, v := range pb.GetLeftNum() {
		this.LeftNum[i] = v
	}
	return
}
func (this* dbPlayerGoldHandData)to_pb()(pb *db.PlayerGoldHand){
	pb = &db.PlayerGoldHand{}
	pb.LastRefreshTime = proto.Int32(this.LastRefreshTime)
	pb.LeftNum = make([]int32, len(this.LeftNum))
	for i, v := range this.LeftNum {
		pb.LeftNum[i]=v
	}
	return
}
func (this* dbPlayerGoldHandData)clone_to(d *dbPlayerGoldHandData){
	d.LastRefreshTime = this.LastRefreshTime
	d.LeftNum = make([]int32, len(this.LeftNum))
	for _ii, _vv := range this.LeftNum {
		d.LeftNum[_ii]=_vv
	}
	return
}
type dbPlayerShopData struct{
	Id int32
	LastFreeRefreshTime int32
	LastAutoRefreshTime int32
	CurrAutoId int32
}
func (this* dbPlayerShopData)from_pb(pb *db.PlayerShop){
	if pb == nil {
		return
	}
	this.Id = pb.GetId()
	this.LastFreeRefreshTime = pb.GetLastFreeRefreshTime()
	this.LastAutoRefreshTime = pb.GetLastAutoRefreshTime()
	this.CurrAutoId = pb.GetCurrAutoId()
	return
}
func (this* dbPlayerShopData)to_pb()(pb *db.PlayerShop){
	pb = &db.PlayerShop{}
	pb.Id = proto.Int32(this.Id)
	pb.LastFreeRefreshTime = proto.Int32(this.LastFreeRefreshTime)
	pb.LastAutoRefreshTime = proto.Int32(this.LastAutoRefreshTime)
	pb.CurrAutoId = proto.Int32(this.CurrAutoId)
	return
}
func (this* dbPlayerShopData)clone_to(d *dbPlayerShopData){
	d.Id = this.Id
	d.LastFreeRefreshTime = this.LastFreeRefreshTime
	d.LastAutoRefreshTime = this.LastAutoRefreshTime
	d.CurrAutoId = this.CurrAutoId
	return
}
type dbPlayerShopItemData struct{
	Id int32
	ShopItemId int32
	LeftNum int32
	ShopId int32
	BuyNum int32
}
func (this* dbPlayerShopItemData)from_pb(pb *db.PlayerShopItem){
	if pb == nil {
		return
	}
	this.Id = pb.GetId()
	this.ShopItemId = pb.GetShopItemId()
	this.LeftNum = pb.GetLeftNum()
	this.ShopId = pb.GetShopId()
	this.BuyNum = pb.GetBuyNum()
	return
}
func (this* dbPlayerShopItemData)to_pb()(pb *db.PlayerShopItem){
	pb = &db.PlayerShopItem{}
	pb.Id = proto.Int32(this.Id)
	pb.ShopItemId = proto.Int32(this.ShopItemId)
	pb.LeftNum = proto.Int32(this.LeftNum)
	pb.ShopId = proto.Int32(this.ShopId)
	pb.BuyNum = proto.Int32(this.BuyNum)
	return
}
func (this* dbPlayerShopItemData)clone_to(d *dbPlayerShopItemData){
	d.Id = this.Id
	d.ShopItemId = this.ShopItemId
	d.LeftNum = this.LeftNum
	d.ShopId = this.ShopId
	d.BuyNum = this.BuyNum
	return
}
type dbPlayerArenaData struct{
	RepeatedWinNum int32
	RepeatedLoseNum int32
	Score int32
	UpdateScoreTime int32
	MatchedPlayerId int32
	HistoryTopRank int32
	FirstGetTicket int32
	LastTicketsRefreshTime int32
	SerialId int32
}
func (this* dbPlayerArenaData)from_pb(pb *db.PlayerArena){
	if pb == nil {
		return
	}
	this.RepeatedWinNum = pb.GetRepeatedWinNum()
	this.RepeatedLoseNum = pb.GetRepeatedLoseNum()
	this.Score = pb.GetScore()
	this.UpdateScoreTime = pb.GetUpdateScoreTime()
	this.MatchedPlayerId = pb.GetMatchedPlayerId()
	this.HistoryTopRank = pb.GetHistoryTopRank()
	this.FirstGetTicket = pb.GetFirstGetTicket()
	this.LastTicketsRefreshTime = pb.GetLastTicketsRefreshTime()
	this.SerialId = pb.GetSerialId()
	return
}
func (this* dbPlayerArenaData)to_pb()(pb *db.PlayerArena){
	pb = &db.PlayerArena{}
	pb.RepeatedWinNum = proto.Int32(this.RepeatedWinNum)
	pb.RepeatedLoseNum = proto.Int32(this.RepeatedLoseNum)
	pb.Score = proto.Int32(this.Score)
	pb.UpdateScoreTime = proto.Int32(this.UpdateScoreTime)
	pb.MatchedPlayerId = proto.Int32(this.MatchedPlayerId)
	pb.HistoryTopRank = proto.Int32(this.HistoryTopRank)
	pb.FirstGetTicket = proto.Int32(this.FirstGetTicket)
	pb.LastTicketsRefreshTime = proto.Int32(this.LastTicketsRefreshTime)
	pb.SerialId = proto.Int32(this.SerialId)
	return
}
func (this* dbPlayerArenaData)clone_to(d *dbPlayerArenaData){
	d.RepeatedWinNum = this.RepeatedWinNum
	d.RepeatedLoseNum = this.RepeatedLoseNum
	d.Score = this.Score
	d.UpdateScoreTime = this.UpdateScoreTime
	d.MatchedPlayerId = this.MatchedPlayerId
	d.HistoryTopRank = this.HistoryTopRank
	d.FirstGetTicket = this.FirstGetTicket
	d.LastTicketsRefreshTime = this.LastTicketsRefreshTime
	d.SerialId = this.SerialId
	return
}
type dbPlayerEquipData struct{
	TmpSaveLeftSlotRoleId int32
	TmpLeftSlotItemId int32
}
func (this* dbPlayerEquipData)from_pb(pb *db.PlayerEquip){
	if pb == nil {
		return
	}
	this.TmpSaveLeftSlotRoleId = pb.GetTmpSaveLeftSlotRoleId()
	this.TmpLeftSlotItemId = pb.GetTmpLeftSlotItemId()
	return
}
func (this* dbPlayerEquipData)to_pb()(pb *db.PlayerEquip){
	pb = &db.PlayerEquip{}
	pb.TmpSaveLeftSlotRoleId = proto.Int32(this.TmpSaveLeftSlotRoleId)
	pb.TmpLeftSlotItemId = proto.Int32(this.TmpLeftSlotItemId)
	return
}
func (this* dbPlayerEquipData)clone_to(d *dbPlayerEquipData){
	d.TmpSaveLeftSlotRoleId = this.TmpSaveLeftSlotRoleId
	d.TmpLeftSlotItemId = this.TmpLeftSlotItemId
	return
}
type dbPlayerActiveStageCommonData struct{
	LastRefreshTime int32
	GetPointsDay int32
	WithdrawPoints int32
}
func (this* dbPlayerActiveStageCommonData)from_pb(pb *db.PlayerActiveStageCommon){
	if pb == nil {
		return
	}
	this.LastRefreshTime = pb.GetLastRefreshTime()
	this.GetPointsDay = pb.GetGetPointsDay()
	this.WithdrawPoints = pb.GetWithdrawPoints()
	return
}
func (this* dbPlayerActiveStageCommonData)to_pb()(pb *db.PlayerActiveStageCommon){
	pb = &db.PlayerActiveStageCommon{}
	pb.LastRefreshTime = proto.Int32(this.LastRefreshTime)
	pb.GetPointsDay = proto.Int32(this.GetPointsDay)
	pb.WithdrawPoints = proto.Int32(this.WithdrawPoints)
	return
}
func (this* dbPlayerActiveStageCommonData)clone_to(d *dbPlayerActiveStageCommonData){
	d.LastRefreshTime = this.LastRefreshTime
	d.GetPointsDay = this.GetPointsDay
	d.WithdrawPoints = this.WithdrawPoints
	return
}
type dbPlayerActiveStageData struct{
	Type int32
	CanChallengeNum int32
	PurchasedNum int32
	BuyNum int32
}
func (this* dbPlayerActiveStageData)from_pb(pb *db.PlayerActiveStage){
	if pb == nil {
		return
	}
	this.Type = pb.GetType()
	this.CanChallengeNum = pb.GetCanChallengeNum()
	this.PurchasedNum = pb.GetPurchasedNum()
	this.BuyNum = pb.GetBuyNum()
	return
}
func (this* dbPlayerActiveStageData)to_pb()(pb *db.PlayerActiveStage){
	pb = &db.PlayerActiveStage{}
	pb.Type = proto.Int32(this.Type)
	pb.CanChallengeNum = proto.Int32(this.CanChallengeNum)
	pb.PurchasedNum = proto.Int32(this.PurchasedNum)
	pb.BuyNum = proto.Int32(this.BuyNum)
	return
}
func (this* dbPlayerActiveStageData)clone_to(d *dbPlayerActiveStageData){
	d.Type = this.Type
	d.CanChallengeNum = this.CanChallengeNum
	d.PurchasedNum = this.PurchasedNum
	d.BuyNum = this.BuyNum
	return
}
type dbPlayerFriendCommonData struct{
	LastRecommendTime int32
	LastBossRefreshTime int32
	FriendBossTableId int32
	FriendBossHpPercent int32
	AttackBossPlayerList []int32
	LastGetStaminaTime int32
	AssistRoleId int32
	LastGetPointsTime int32
	GetPointsDay int32
	SearchedBossNum int32
	LastSearchBossNumRefreshTime int32
}
func (this* dbPlayerFriendCommonData)from_pb(pb *db.PlayerFriendCommon){
	if pb == nil {
		this.AttackBossPlayerList = make([]int32,0)
		return
	}
	this.LastRecommendTime = pb.GetLastRecommendTime()
	this.LastBossRefreshTime = pb.GetLastBossRefreshTime()
	this.FriendBossTableId = pb.GetFriendBossTableId()
	this.FriendBossHpPercent = pb.GetFriendBossHpPercent()
	this.AttackBossPlayerList = make([]int32,len(pb.GetAttackBossPlayerList()))
	for i, v := range pb.GetAttackBossPlayerList() {
		this.AttackBossPlayerList[i] = v
	}
	this.LastGetStaminaTime = pb.GetLastGetStaminaTime()
	this.AssistRoleId = pb.GetAssistRoleId()
	this.LastGetPointsTime = pb.GetLastGetPointsTime()
	this.GetPointsDay = pb.GetGetPointsDay()
	this.SearchedBossNum = pb.GetSearchedBossNum()
	this.LastSearchBossNumRefreshTime = pb.GetLastSearchBossNumRefreshTime()
	return
}
func (this* dbPlayerFriendCommonData)to_pb()(pb *db.PlayerFriendCommon){
	pb = &db.PlayerFriendCommon{}
	pb.LastRecommendTime = proto.Int32(this.LastRecommendTime)
	pb.LastBossRefreshTime = proto.Int32(this.LastBossRefreshTime)
	pb.FriendBossTableId = proto.Int32(this.FriendBossTableId)
	pb.FriendBossHpPercent = proto.Int32(this.FriendBossHpPercent)
	pb.AttackBossPlayerList = make([]int32, len(this.AttackBossPlayerList))
	for i, v := range this.AttackBossPlayerList {
		pb.AttackBossPlayerList[i]=v
	}
	pb.LastGetStaminaTime = proto.Int32(this.LastGetStaminaTime)
	pb.AssistRoleId = proto.Int32(this.AssistRoleId)
	pb.LastGetPointsTime = proto.Int32(this.LastGetPointsTime)
	pb.GetPointsDay = proto.Int32(this.GetPointsDay)
	pb.SearchedBossNum = proto.Int32(this.SearchedBossNum)
	pb.LastSearchBossNumRefreshTime = proto.Int32(this.LastSearchBossNumRefreshTime)
	return
}
func (this* dbPlayerFriendCommonData)clone_to(d *dbPlayerFriendCommonData){
	d.LastRecommendTime = this.LastRecommendTime
	d.LastBossRefreshTime = this.LastBossRefreshTime
	d.FriendBossTableId = this.FriendBossTableId
	d.FriendBossHpPercent = this.FriendBossHpPercent
	d.AttackBossPlayerList = make([]int32, len(this.AttackBossPlayerList))
	for _ii, _vv := range this.AttackBossPlayerList {
		d.AttackBossPlayerList[_ii]=_vv
	}
	d.LastGetStaminaTime = this.LastGetStaminaTime
	d.AssistRoleId = this.AssistRoleId
	d.LastGetPointsTime = this.LastGetPointsTime
	d.GetPointsDay = this.GetPointsDay
	d.SearchedBossNum = this.SearchedBossNum
	d.LastSearchBossNumRefreshTime = this.LastSearchBossNumRefreshTime
	return
}
type dbPlayerFriendData struct{
	PlayerId int32
	LastGivePointsTime int32
	GetPoints int32
}
func (this* dbPlayerFriendData)from_pb(pb *db.PlayerFriend){
	if pb == nil {
		return
	}
	this.PlayerId = pb.GetPlayerId()
	this.LastGivePointsTime = pb.GetLastGivePointsTime()
	this.GetPoints = pb.GetGetPoints()
	return
}
func (this* dbPlayerFriendData)to_pb()(pb *db.PlayerFriend){
	pb = &db.PlayerFriend{}
	pb.PlayerId = proto.Int32(this.PlayerId)
	pb.LastGivePointsTime = proto.Int32(this.LastGivePointsTime)
	pb.GetPoints = proto.Int32(this.GetPoints)
	return
}
func (this* dbPlayerFriendData)clone_to(d *dbPlayerFriendData){
	d.PlayerId = this.PlayerId
	d.LastGivePointsTime = this.LastGivePointsTime
	d.GetPoints = this.GetPoints
	return
}
type dbPlayerFriendRecommendData struct{
	PlayerId int32
}
func (this* dbPlayerFriendRecommendData)from_pb(pb *db.PlayerFriendRecommend){
	if pb == nil {
		return
	}
	this.PlayerId = pb.GetPlayerId()
	return
}
func (this* dbPlayerFriendRecommendData)to_pb()(pb *db.PlayerFriendRecommend){
	pb = &db.PlayerFriendRecommend{}
	pb.PlayerId = proto.Int32(this.PlayerId)
	return
}
func (this* dbPlayerFriendRecommendData)clone_to(d *dbPlayerFriendRecommendData){
	d.PlayerId = this.PlayerId
	return
}
type dbPlayerFriendAskData struct{
	PlayerId int32
}
func (this* dbPlayerFriendAskData)from_pb(pb *db.PlayerFriendAsk){
	if pb == nil {
		return
	}
	this.PlayerId = pb.GetPlayerId()
	return
}
func (this* dbPlayerFriendAskData)to_pb()(pb *db.PlayerFriendAsk){
	pb = &db.PlayerFriendAsk{}
	pb.PlayerId = proto.Int32(this.PlayerId)
	return
}
func (this* dbPlayerFriendAskData)clone_to(d *dbPlayerFriendAskData){
	d.PlayerId = this.PlayerId
	return
}
type dbPlayerFriendBossData struct{
	MonsterPos int32
	MonsterId int32
	MonsterHp int32
	MonsterMaxHp int32
}
func (this* dbPlayerFriendBossData)from_pb(pb *db.PlayerFriendBoss){
	if pb == nil {
		return
	}
	this.MonsterPos = pb.GetMonsterPos()
	this.MonsterId = pb.GetMonsterId()
	this.MonsterHp = pb.GetMonsterHp()
	this.MonsterMaxHp = pb.GetMonsterMaxHp()
	return
}
func (this* dbPlayerFriendBossData)to_pb()(pb *db.PlayerFriendBoss){
	pb = &db.PlayerFriendBoss{}
	pb.MonsterPos = proto.Int32(this.MonsterPos)
	pb.MonsterId = proto.Int32(this.MonsterId)
	pb.MonsterHp = proto.Int32(this.MonsterHp)
	pb.MonsterMaxHp = proto.Int32(this.MonsterMaxHp)
	return
}
func (this* dbPlayerFriendBossData)clone_to(d *dbPlayerFriendBossData){
	d.MonsterPos = this.MonsterPos
	d.MonsterId = this.MonsterId
	d.MonsterHp = this.MonsterHp
	d.MonsterMaxHp = this.MonsterMaxHp
	return
}
type dbPlayerTaskCommonData struct{
	LastRefreshTime int32
}
func (this* dbPlayerTaskCommonData)from_pb(pb *db.PlayerTaskCommon){
	if pb == nil {
		return
	}
	this.LastRefreshTime = pb.GetLastRefreshTime()
	return
}
func (this* dbPlayerTaskCommonData)to_pb()(pb *db.PlayerTaskCommon){
	pb = &db.PlayerTaskCommon{}
	pb.LastRefreshTime = proto.Int32(this.LastRefreshTime)
	return
}
func (this* dbPlayerTaskCommonData)clone_to(d *dbPlayerTaskCommonData){
	d.LastRefreshTime = this.LastRefreshTime
	return
}
type dbPlayerTaskData struct{
	Id int32
	Value int32
	State int32
	Param int32
}
func (this* dbPlayerTaskData)from_pb(pb *db.PlayerTask){
	if pb == nil {
		return
	}
	this.Id = pb.GetId()
	this.Value = pb.GetValue()
	this.State = pb.GetState()
	this.Param = pb.GetParam()
	return
}
func (this* dbPlayerTaskData)to_pb()(pb *db.PlayerTask){
	pb = &db.PlayerTask{}
	pb.Id = proto.Int32(this.Id)
	pb.Value = proto.Int32(this.Value)
	pb.State = proto.Int32(this.State)
	pb.Param = proto.Int32(this.Param)
	return
}
func (this* dbPlayerTaskData)clone_to(d *dbPlayerTaskData){
	d.Id = this.Id
	d.Value = this.Value
	d.State = this.State
	d.Param = this.Param
	return
}
type dbPlayerFinishedTaskData struct{
	Id int32
}
func (this* dbPlayerFinishedTaskData)from_pb(pb *db.PlayerFinishedTask){
	if pb == nil {
		return
	}
	this.Id = pb.GetId()
	return
}
func (this* dbPlayerFinishedTaskData)to_pb()(pb *db.PlayerFinishedTask){
	pb = &db.PlayerFinishedTask{}
	pb.Id = proto.Int32(this.Id)
	return
}
func (this* dbPlayerFinishedTaskData)clone_to(d *dbPlayerFinishedTaskData){
	d.Id = this.Id
	return
}
type dbPlayerDailyTaskAllDailyData struct{
	CompleteTaskId int32
}
func (this* dbPlayerDailyTaskAllDailyData)from_pb(pb *db.PlayerDailyTaskAllDaily){
	if pb == nil {
		return
	}
	this.CompleteTaskId = pb.GetCompleteTaskId()
	return
}
func (this* dbPlayerDailyTaskAllDailyData)to_pb()(pb *db.PlayerDailyTaskAllDaily){
	pb = &db.PlayerDailyTaskAllDaily{}
	pb.CompleteTaskId = proto.Int32(this.CompleteTaskId)
	return
}
func (this* dbPlayerDailyTaskAllDailyData)clone_to(d *dbPlayerDailyTaskAllDailyData){
	d.CompleteTaskId = this.CompleteTaskId
	return
}
type dbPlayerExploreCommonData struct{
	LastRefreshTime int32
	CurrentId int32
}
func (this* dbPlayerExploreCommonData)from_pb(pb *db.PlayerExploreCommon){
	if pb == nil {
		return
	}
	this.LastRefreshTime = pb.GetLastRefreshTime()
	this.CurrentId = pb.GetCurrentId()
	return
}
func (this* dbPlayerExploreCommonData)to_pb()(pb *db.PlayerExploreCommon){
	pb = &db.PlayerExploreCommon{}
	pb.LastRefreshTime = proto.Int32(this.LastRefreshTime)
	pb.CurrentId = proto.Int32(this.CurrentId)
	return
}
func (this* dbPlayerExploreCommonData)clone_to(d *dbPlayerExploreCommonData){
	d.LastRefreshTime = this.LastRefreshTime
	d.CurrentId = this.CurrentId
	return
}
type dbPlayerExploreData struct{
	Id int32
	TaskId int32
	State int32
	RoleCampsCanSel []int32
	RoleTypesCanSel []int32
	RoleId4TaskTitle int32
	NameId4TaskTitle int32
	StartTime int32
	RoleIds []int32
	IsLock int32
	RandomRewards []int32
	RewardStageId int32
}
func (this* dbPlayerExploreData)from_pb(pb *db.PlayerExplore){
	if pb == nil {
		this.RoleCampsCanSel = make([]int32,0)
		this.RoleTypesCanSel = make([]int32,0)
		this.RoleIds = make([]int32,0)
		this.RandomRewards = make([]int32,0)
		return
	}
	this.Id = pb.GetId()
	this.TaskId = pb.GetTaskId()
	this.State = pb.GetState()
	this.RoleCampsCanSel = make([]int32,len(pb.GetRoleCampsCanSel()))
	for i, v := range pb.GetRoleCampsCanSel() {
		this.RoleCampsCanSel[i] = v
	}
	this.RoleTypesCanSel = make([]int32,len(pb.GetRoleTypesCanSel()))
	for i, v := range pb.GetRoleTypesCanSel() {
		this.RoleTypesCanSel[i] = v
	}
	this.RoleId4TaskTitle = pb.GetRoleId4TaskTitle()
	this.NameId4TaskTitle = pb.GetNameId4TaskTitle()
	this.StartTime = pb.GetStartTime()
	this.RoleIds = make([]int32,len(pb.GetRoleIds()))
	for i, v := range pb.GetRoleIds() {
		this.RoleIds[i] = v
	}
	this.IsLock = pb.GetIsLock()
	this.RandomRewards = make([]int32,len(pb.GetRandomRewards()))
	for i, v := range pb.GetRandomRewards() {
		this.RandomRewards[i] = v
	}
	this.RewardStageId = pb.GetRewardStageId()
	return
}
func (this* dbPlayerExploreData)to_pb()(pb *db.PlayerExplore){
	pb = &db.PlayerExplore{}
	pb.Id = proto.Int32(this.Id)
	pb.TaskId = proto.Int32(this.TaskId)
	pb.State = proto.Int32(this.State)
	pb.RoleCampsCanSel = make([]int32, len(this.RoleCampsCanSel))
	for i, v := range this.RoleCampsCanSel {
		pb.RoleCampsCanSel[i]=v
	}
	pb.RoleTypesCanSel = make([]int32, len(this.RoleTypesCanSel))
	for i, v := range this.RoleTypesCanSel {
		pb.RoleTypesCanSel[i]=v
	}
	pb.RoleId4TaskTitle = proto.Int32(this.RoleId4TaskTitle)
	pb.NameId4TaskTitle = proto.Int32(this.NameId4TaskTitle)
	pb.StartTime = proto.Int32(this.StartTime)
	pb.RoleIds = make([]int32, len(this.RoleIds))
	for i, v := range this.RoleIds {
		pb.RoleIds[i]=v
	}
	pb.IsLock = proto.Int32(this.IsLock)
	pb.RandomRewards = make([]int32, len(this.RandomRewards))
	for i, v := range this.RandomRewards {
		pb.RandomRewards[i]=v
	}
	pb.RewardStageId = proto.Int32(this.RewardStageId)
	return
}
func (this* dbPlayerExploreData)clone_to(d *dbPlayerExploreData){
	d.Id = this.Id
	d.TaskId = this.TaskId
	d.State = this.State
	d.RoleCampsCanSel = make([]int32, len(this.RoleCampsCanSel))
	for _ii, _vv := range this.RoleCampsCanSel {
		d.RoleCampsCanSel[_ii]=_vv
	}
	d.RoleTypesCanSel = make([]int32, len(this.RoleTypesCanSel))
	for _ii, _vv := range this.RoleTypesCanSel {
		d.RoleTypesCanSel[_ii]=_vv
	}
	d.RoleId4TaskTitle = this.RoleId4TaskTitle
	d.NameId4TaskTitle = this.NameId4TaskTitle
	d.StartTime = this.StartTime
	d.RoleIds = make([]int32, len(this.RoleIds))
	for _ii, _vv := range this.RoleIds {
		d.RoleIds[_ii]=_vv
	}
	d.IsLock = this.IsLock
	d.RandomRewards = make([]int32, len(this.RandomRewards))
	for _ii, _vv := range this.RandomRewards {
		d.RandomRewards[_ii]=_vv
	}
	d.RewardStageId = this.RewardStageId
	return
}
type dbPlayerExploreStoryData struct{
	TaskId int32
	State int32
	RoleCampsCanSel []int32
	RoleTypesCanSel []int32
	StartTime int32
	RoleIds []int32
	RandomRewards []int32
	RewardStageId int32
}
func (this* dbPlayerExploreStoryData)from_pb(pb *db.PlayerExploreStory){
	if pb == nil {
		this.RoleCampsCanSel = make([]int32,0)
		this.RoleTypesCanSel = make([]int32,0)
		this.RoleIds = make([]int32,0)
		this.RandomRewards = make([]int32,0)
		return
	}
	this.TaskId = pb.GetTaskId()
	this.State = pb.GetState()
	this.RoleCampsCanSel = make([]int32,len(pb.GetRoleCampsCanSel()))
	for i, v := range pb.GetRoleCampsCanSel() {
		this.RoleCampsCanSel[i] = v
	}
	this.RoleTypesCanSel = make([]int32,len(pb.GetRoleTypesCanSel()))
	for i, v := range pb.GetRoleTypesCanSel() {
		this.RoleTypesCanSel[i] = v
	}
	this.StartTime = pb.GetStartTime()
	this.RoleIds = make([]int32,len(pb.GetRoleIds()))
	for i, v := range pb.GetRoleIds() {
		this.RoleIds[i] = v
	}
	this.RandomRewards = make([]int32,len(pb.GetRandomRewards()))
	for i, v := range pb.GetRandomRewards() {
		this.RandomRewards[i] = v
	}
	this.RewardStageId = pb.GetRewardStageId()
	return
}
func (this* dbPlayerExploreStoryData)to_pb()(pb *db.PlayerExploreStory){
	pb = &db.PlayerExploreStory{}
	pb.TaskId = proto.Int32(this.TaskId)
	pb.State = proto.Int32(this.State)
	pb.RoleCampsCanSel = make([]int32, len(this.RoleCampsCanSel))
	for i, v := range this.RoleCampsCanSel {
		pb.RoleCampsCanSel[i]=v
	}
	pb.RoleTypesCanSel = make([]int32, len(this.RoleTypesCanSel))
	for i, v := range this.RoleTypesCanSel {
		pb.RoleTypesCanSel[i]=v
	}
	pb.StartTime = proto.Int32(this.StartTime)
	pb.RoleIds = make([]int32, len(this.RoleIds))
	for i, v := range this.RoleIds {
		pb.RoleIds[i]=v
	}
	pb.RandomRewards = make([]int32, len(this.RandomRewards))
	for i, v := range this.RandomRewards {
		pb.RandomRewards[i]=v
	}
	pb.RewardStageId = proto.Int32(this.RewardStageId)
	return
}
func (this* dbPlayerExploreStoryData)clone_to(d *dbPlayerExploreStoryData){
	d.TaskId = this.TaskId
	d.State = this.State
	d.RoleCampsCanSel = make([]int32, len(this.RoleCampsCanSel))
	for _ii, _vv := range this.RoleCampsCanSel {
		d.RoleCampsCanSel[_ii]=_vv
	}
	d.RoleTypesCanSel = make([]int32, len(this.RoleTypesCanSel))
	for _ii, _vv := range this.RoleTypesCanSel {
		d.RoleTypesCanSel[_ii]=_vv
	}
	d.StartTime = this.StartTime
	d.RoleIds = make([]int32, len(this.RoleIds))
	for _ii, _vv := range this.RoleIds {
		d.RoleIds[_ii]=_vv
	}
	d.RandomRewards = make([]int32, len(this.RandomRewards))
	for _ii, _vv := range this.RandomRewards {
		d.RandomRewards[_ii]=_vv
	}
	d.RewardStageId = this.RewardStageId
	return
}
type dbPlayerFriendChatUnreadIdData struct{
	FriendId int32
	MessageIds []int32
	CurrMessageId int32
}
func (this* dbPlayerFriendChatUnreadIdData)from_pb(pb *db.PlayerFriendChatUnreadId){
	if pb == nil {
		this.MessageIds = make([]int32,0)
		return
	}
	this.FriendId = pb.GetFriendId()
	this.MessageIds = make([]int32,len(pb.GetMessageIds()))
	for i, v := range pb.GetMessageIds() {
		this.MessageIds[i] = v
	}
	this.CurrMessageId = pb.GetCurrMessageId()
	return
}
func (this* dbPlayerFriendChatUnreadIdData)to_pb()(pb *db.PlayerFriendChatUnreadId){
	pb = &db.PlayerFriendChatUnreadId{}
	pb.FriendId = proto.Int32(this.FriendId)
	pb.MessageIds = make([]int32, len(this.MessageIds))
	for i, v := range this.MessageIds {
		pb.MessageIds[i]=v
	}
	pb.CurrMessageId = proto.Int32(this.CurrMessageId)
	return
}
func (this* dbPlayerFriendChatUnreadIdData)clone_to(d *dbPlayerFriendChatUnreadIdData){
	d.FriendId = this.FriendId
	d.MessageIds = make([]int32, len(this.MessageIds))
	for _ii, _vv := range this.MessageIds {
		d.MessageIds[_ii]=_vv
	}
	d.CurrMessageId = this.CurrMessageId
	return
}
type dbPlayerFriendChatUnreadMessageData struct{
	PlayerMessageId int64
	Message []byte
	SendTime int32
	IsRead int32
}
func (this* dbPlayerFriendChatUnreadMessageData)from_pb(pb *db.PlayerFriendChatUnreadMessage){
	if pb == nil {
		return
	}
	this.PlayerMessageId = pb.GetPlayerMessageId()
	this.Message = pb.GetMessage()
	this.SendTime = pb.GetSendTime()
	this.IsRead = pb.GetIsRead()
	return
}
func (this* dbPlayerFriendChatUnreadMessageData)to_pb()(pb *db.PlayerFriendChatUnreadMessage){
	pb = &db.PlayerFriendChatUnreadMessage{}
	pb.PlayerMessageId = proto.Int64(this.PlayerMessageId)
	pb.Message = this.Message
	pb.SendTime = proto.Int32(this.SendTime)
	pb.IsRead = proto.Int32(this.IsRead)
	return
}
func (this* dbPlayerFriendChatUnreadMessageData)clone_to(d *dbPlayerFriendChatUnreadMessageData){
	d.PlayerMessageId = this.PlayerMessageId
	d.Message = make([]byte, len(this.Message))
	for _ii, _vv := range this.Message {
		d.Message[_ii]=_vv
	}
	d.SendTime = this.SendTime
	d.IsRead = this.IsRead
	return
}
type dbPlayerHeadItemData struct{
	Id int32
}
func (this* dbPlayerHeadItemData)from_pb(pb *db.PlayerHeadItem){
	if pb == nil {
		return
	}
	this.Id = pb.GetId()
	return
}
func (this* dbPlayerHeadItemData)to_pb()(pb *db.PlayerHeadItem){
	pb = &db.PlayerHeadItem{}
	pb.Id = proto.Int32(this.Id)
	return
}
func (this* dbPlayerHeadItemData)clone_to(d *dbPlayerHeadItemData){
	d.Id = this.Id
	return
}
type dbPlayerSuitAwardData struct{
	Id int32
	AwardTime int32
}
func (this* dbPlayerSuitAwardData)from_pb(pb *db.PlayerSuitAward){
	if pb == nil {
		return
	}
	this.Id = pb.GetId()
	this.AwardTime = pb.GetAwardTime()
	return
}
func (this* dbPlayerSuitAwardData)to_pb()(pb *db.PlayerSuitAward){
	pb = &db.PlayerSuitAward{}
	pb.Id = proto.Int32(this.Id)
	pb.AwardTime = proto.Int32(this.AwardTime)
	return
}
func (this* dbPlayerSuitAwardData)clone_to(d *dbPlayerSuitAwardData){
	d.Id = this.Id
	d.AwardTime = this.AwardTime
	return
}
type dbPlayerChatData struct{
	Channel int32
	LastChatTime int32
	LastPullTime int32
	LastMsgIndex int32
}
func (this* dbPlayerChatData)from_pb(pb *db.PlayerChat){
	if pb == nil {
		return
	}
	this.Channel = pb.GetChannel()
	this.LastChatTime = pb.GetLastChatTime()
	this.LastPullTime = pb.GetLastPullTime()
	this.LastMsgIndex = pb.GetLastMsgIndex()
	return
}
func (this* dbPlayerChatData)to_pb()(pb *db.PlayerChat){
	pb = &db.PlayerChat{}
	pb.Channel = proto.Int32(this.Channel)
	pb.LastChatTime = proto.Int32(this.LastChatTime)
	pb.LastPullTime = proto.Int32(this.LastPullTime)
	pb.LastMsgIndex = proto.Int32(this.LastMsgIndex)
	return
}
func (this* dbPlayerChatData)clone_to(d *dbPlayerChatData){
	d.Channel = this.Channel
	d.LastChatTime = this.LastChatTime
	d.LastPullTime = this.LastPullTime
	d.LastMsgIndex = this.LastMsgIndex
	return
}
type dbPlayerAnouncementData struct{
	LastSendTime int32
}
func (this* dbPlayerAnouncementData)from_pb(pb *db.PlayerAnouncement){
	if pb == nil {
		return
	}
	this.LastSendTime = pb.GetLastSendTime()
	return
}
func (this* dbPlayerAnouncementData)to_pb()(pb *db.PlayerAnouncement){
	pb = &db.PlayerAnouncement{}
	pb.LastSendTime = proto.Int32(this.LastSendTime)
	return
}
func (this* dbPlayerAnouncementData)clone_to(d *dbPlayerAnouncementData){
	d.LastSendTime = this.LastSendTime
	return
}
type dbPlayerFirstDrawCardData struct{
	Id int32
	Drawed int32
}
func (this* dbPlayerFirstDrawCardData)from_pb(pb *db.PlayerFirstDrawCard){
	if pb == nil {
		return
	}
	this.Id = pb.GetId()
	this.Drawed = pb.GetDrawed()
	return
}
func (this* dbPlayerFirstDrawCardData)to_pb()(pb *db.PlayerFirstDrawCard){
	pb = &db.PlayerFirstDrawCard{}
	pb.Id = proto.Int32(this.Id)
	pb.Drawed = proto.Int32(this.Drawed)
	return
}
func (this* dbPlayerFirstDrawCardData)clone_to(d *dbPlayerFirstDrawCardData){
	d.Id = this.Id
	d.Drawed = this.Drawed
	return
}
type dbPlayerGuildData struct{
	Id int32
	JoinTime int32
	QuitTime int32
	SignTime int32
	Position int32
	DonateNum int32
	LastAskDonateTime int32
	LastDonateTime int32
}
func (this* dbPlayerGuildData)from_pb(pb *db.PlayerGuild){
	if pb == nil {
		return
	}
	this.Id = pb.GetId()
	this.JoinTime = pb.GetJoinTime()
	this.QuitTime = pb.GetQuitTime()
	this.SignTime = pb.GetSignTime()
	this.Position = pb.GetPosition()
	this.DonateNum = pb.GetDonateNum()
	this.LastAskDonateTime = pb.GetLastAskDonateTime()
	this.LastDonateTime = pb.GetLastDonateTime()
	return
}
func (this* dbPlayerGuildData)to_pb()(pb *db.PlayerGuild){
	pb = &db.PlayerGuild{}
	pb.Id = proto.Int32(this.Id)
	pb.JoinTime = proto.Int32(this.JoinTime)
	pb.QuitTime = proto.Int32(this.QuitTime)
	pb.SignTime = proto.Int32(this.SignTime)
	pb.Position = proto.Int32(this.Position)
	pb.DonateNum = proto.Int32(this.DonateNum)
	pb.LastAskDonateTime = proto.Int32(this.LastAskDonateTime)
	pb.LastDonateTime = proto.Int32(this.LastDonateTime)
	return
}
func (this* dbPlayerGuildData)clone_to(d *dbPlayerGuildData){
	d.Id = this.Id
	d.JoinTime = this.JoinTime
	d.QuitTime = this.QuitTime
	d.SignTime = this.SignTime
	d.Position = this.Position
	d.DonateNum = this.DonateNum
	d.LastAskDonateTime = this.LastAskDonateTime
	d.LastDonateTime = this.LastDonateTime
	return
}
type dbPlayerGuildStageData struct{
	RespawnNum int32
	RespawnState int32
	LastRefreshTime int32
}
func (this* dbPlayerGuildStageData)from_pb(pb *db.PlayerGuildStage){
	if pb == nil {
		return
	}
	this.RespawnNum = pb.GetRespawnNum()
	this.RespawnState = pb.GetRespawnState()
	this.LastRefreshTime = pb.GetLastRefreshTime()
	return
}
func (this* dbPlayerGuildStageData)to_pb()(pb *db.PlayerGuildStage){
	pb = &db.PlayerGuildStage{}
	pb.RespawnNum = proto.Int32(this.RespawnNum)
	pb.RespawnState = proto.Int32(this.RespawnState)
	pb.LastRefreshTime = proto.Int32(this.LastRefreshTime)
	return
}
func (this* dbPlayerGuildStageData)clone_to(d *dbPlayerGuildStageData){
	d.RespawnNum = this.RespawnNum
	d.RespawnState = this.RespawnState
	d.LastRefreshTime = this.LastRefreshTime
	return
}
type dbPlayerSignData struct{
	CurrGroup int32
	AwardIndex int32
	SignedIndex int32
	LastSignedTime int32
}
func (this* dbPlayerSignData)from_pb(pb *db.PlayerSign){
	if pb == nil {
		return
	}
	this.CurrGroup = pb.GetCurrGroup()
	this.AwardIndex = pb.GetAwardIndex()
	this.SignedIndex = pb.GetSignedIndex()
	this.LastSignedTime = pb.GetLastSignedTime()
	return
}
func (this* dbPlayerSignData)to_pb()(pb *db.PlayerSign){
	pb = &db.PlayerSign{}
	pb.CurrGroup = proto.Int32(this.CurrGroup)
	pb.AwardIndex = proto.Int32(this.AwardIndex)
	pb.SignedIndex = proto.Int32(this.SignedIndex)
	pb.LastSignedTime = proto.Int32(this.LastSignedTime)
	return
}
func (this* dbPlayerSignData)clone_to(d *dbPlayerSignData){
	d.CurrGroup = this.CurrGroup
	d.AwardIndex = this.AwardIndex
	d.SignedIndex = this.SignedIndex
	d.LastSignedTime = this.LastSignedTime
	return
}
type dbPlayerSevenDaysData struct{
	Awards []int32
	Days int32
}
func (this* dbPlayerSevenDaysData)from_pb(pb *db.PlayerSevenDays){
	if pb == nil {
		this.Awards = make([]int32,0)
		return
	}
	this.Awards = make([]int32,len(pb.GetAwards()))
	for i, v := range pb.GetAwards() {
		this.Awards[i] = v
	}
	this.Days = pb.GetDays()
	return
}
func (this* dbPlayerSevenDaysData)to_pb()(pb *db.PlayerSevenDays){
	pb = &db.PlayerSevenDays{}
	pb.Awards = make([]int32, len(this.Awards))
	for i, v := range this.Awards {
		pb.Awards[i]=v
	}
	pb.Days = proto.Int32(this.Days)
	return
}
func (this* dbPlayerSevenDaysData)clone_to(d *dbPlayerSevenDaysData){
	d.Awards = make([]int32, len(this.Awards))
	for _ii, _vv := range this.Awards {
		d.Awards[_ii]=_vv
	}
	d.Days = this.Days
	return
}
type dbPlayerPayCommonData struct{
	FirstPayState int32
}
func (this* dbPlayerPayCommonData)from_pb(pb *db.PlayerPayCommon){
	if pb == nil {
		return
	}
	this.FirstPayState = pb.GetFirstPayState()
	return
}
func (this* dbPlayerPayCommonData)to_pb()(pb *db.PlayerPayCommon){
	pb = &db.PlayerPayCommon{}
	pb.FirstPayState = proto.Int32(this.FirstPayState)
	return
}
func (this* dbPlayerPayCommonData)clone_to(d *dbPlayerPayCommonData){
	d.FirstPayState = this.FirstPayState
	return
}
type dbPlayerPayData struct{
	BundleId string
	LastPayedTime int32
	LastAwardTime int32
	SendMailNum int32
	ChargeNum int32
}
func (this* dbPlayerPayData)from_pb(pb *db.PlayerPay){
	if pb == nil {
		return
	}
	this.BundleId = pb.GetBundleId()
	this.LastPayedTime = pb.GetLastPayedTime()
	this.LastAwardTime = pb.GetLastAwardTime()
	this.SendMailNum = pb.GetSendMailNum()
	this.ChargeNum = pb.GetChargeNum()
	return
}
func (this* dbPlayerPayData)to_pb()(pb *db.PlayerPay){
	pb = &db.PlayerPay{}
	pb.BundleId = proto.String(this.BundleId)
	pb.LastPayedTime = proto.Int32(this.LastPayedTime)
	pb.LastAwardTime = proto.Int32(this.LastAwardTime)
	pb.SendMailNum = proto.Int32(this.SendMailNum)
	pb.ChargeNum = proto.Int32(this.ChargeNum)
	return
}
func (this* dbPlayerPayData)clone_to(d *dbPlayerPayData){
	d.BundleId = this.BundleId
	d.LastPayedTime = this.LastPayedTime
	d.LastAwardTime = this.LastAwardTime
	d.SendMailNum = this.SendMailNum
	d.ChargeNum = this.ChargeNum
	return
}
type dbPlayerGuideDataData struct{
	Data []byte
}
func (this* dbPlayerGuideDataData)from_pb(pb *db.PlayerGuideData){
	if pb == nil {
		return
	}
	this.Data = pb.GetData()
	return
}
func (this* dbPlayerGuideDataData)to_pb()(pb *db.PlayerGuideData){
	pb = &db.PlayerGuideData{}
	pb.Data = this.Data
	return
}
func (this* dbPlayerGuideDataData)clone_to(d *dbPlayerGuideDataData){
	d.Data = make([]byte, len(this.Data))
	for _ii, _vv := range this.Data {
		d.Data[_ii]=_vv
	}
	return
}
type dbPlayerActivityDataData struct{
	Id int32
	SubIds []int32
	SubValues []int32
	SubNum int32
}
func (this* dbPlayerActivityDataData)from_pb(pb *db.PlayerActivityData){
	if pb == nil {
		this.SubIds = make([]int32,0)
		this.SubValues = make([]int32,0)
		return
	}
	this.Id = pb.GetId()
	this.SubIds = make([]int32,len(pb.GetSubIds()))
	for i, v := range pb.GetSubIds() {
		this.SubIds[i] = v
	}
	this.SubValues = make([]int32,len(pb.GetSubValues()))
	for i, v := range pb.GetSubValues() {
		this.SubValues[i] = v
	}
	this.SubNum = pb.GetSubNum()
	return
}
func (this* dbPlayerActivityDataData)to_pb()(pb *db.PlayerActivityData){
	pb = &db.PlayerActivityData{}
	pb.Id = proto.Int32(this.Id)
	pb.SubIds = make([]int32, len(this.SubIds))
	for i, v := range this.SubIds {
		pb.SubIds[i]=v
	}
	pb.SubValues = make([]int32, len(this.SubValues))
	for i, v := range this.SubValues {
		pb.SubValues[i]=v
	}
	pb.SubNum = proto.Int32(this.SubNum)
	return
}
func (this* dbPlayerActivityDataData)clone_to(d *dbPlayerActivityDataData){
	d.Id = this.Id
	d.SubIds = make([]int32, len(this.SubIds))
	for _ii, _vv := range this.SubIds {
		d.SubIds[_ii]=_vv
	}
	d.SubValues = make([]int32, len(this.SubValues))
	for _ii, _vv := range this.SubValues {
		d.SubValues[_ii]=_vv
	}
	d.SubNum = this.SubNum
	return
}
type dbPlayerExpeditionDataData struct{
	RefreshTime int32
	CurrLevel int32
	PurifyPoints int32
}
func (this* dbPlayerExpeditionDataData)from_pb(pb *db.PlayerExpeditionData){
	if pb == nil {
		return
	}
	this.RefreshTime = pb.GetRefreshTime()
	this.CurrLevel = pb.GetCurrLevel()
	this.PurifyPoints = pb.GetPurifyPoints()
	return
}
func (this* dbPlayerExpeditionDataData)to_pb()(pb *db.PlayerExpeditionData){
	pb = &db.PlayerExpeditionData{}
	pb.RefreshTime = proto.Int32(this.RefreshTime)
	pb.CurrLevel = proto.Int32(this.CurrLevel)
	pb.PurifyPoints = proto.Int32(this.PurifyPoints)
	return
}
func (this* dbPlayerExpeditionDataData)clone_to(d *dbPlayerExpeditionDataData){
	d.RefreshTime = this.RefreshTime
	d.CurrLevel = this.CurrLevel
	d.PurifyPoints = this.PurifyPoints
	return
}
type dbPlayerExpeditionRoleData struct{
	Id int32
	HP int32
	Weak int32
	HpPercent int32
}
func (this* dbPlayerExpeditionRoleData)from_pb(pb *db.PlayerExpeditionRole){
	if pb == nil {
		return
	}
	this.Id = pb.GetId()
	this.HP = pb.GetHP()
	this.Weak = pb.GetWeak()
	this.HpPercent = pb.GetHpPercent()
	return
}
func (this* dbPlayerExpeditionRoleData)to_pb()(pb *db.PlayerExpeditionRole){
	pb = &db.PlayerExpeditionRole{}
	pb.Id = proto.Int32(this.Id)
	pb.HP = proto.Int32(this.HP)
	pb.Weak = proto.Int32(this.Weak)
	pb.HpPercent = proto.Int32(this.HpPercent)
	return
}
func (this* dbPlayerExpeditionRoleData)clone_to(d *dbPlayerExpeditionRoleData){
	d.Id = this.Id
	d.HP = this.HP
	d.Weak = this.Weak
	d.HpPercent = this.HpPercent
	return
}
type dbPlayerExpeditionLevelData struct{
	Level int32
	PlayerId int32
	Power int32
	GoldIncome int32
	ExpeditionGoldIncome int32
}
func (this* dbPlayerExpeditionLevelData)from_pb(pb *db.PlayerExpeditionLevel){
	if pb == nil {
		return
	}
	this.Level = pb.GetLevel()
	this.PlayerId = pb.GetPlayerId()
	this.Power = pb.GetPower()
	this.GoldIncome = pb.GetGoldIncome()
	this.ExpeditionGoldIncome = pb.GetExpeditionGoldIncome()
	return
}
func (this* dbPlayerExpeditionLevelData)to_pb()(pb *db.PlayerExpeditionLevel){
	pb = &db.PlayerExpeditionLevel{}
	pb.Level = proto.Int32(this.Level)
	pb.PlayerId = proto.Int32(this.PlayerId)
	pb.Power = proto.Int32(this.Power)
	pb.GoldIncome = proto.Int32(this.GoldIncome)
	pb.ExpeditionGoldIncome = proto.Int32(this.ExpeditionGoldIncome)
	return
}
func (this* dbPlayerExpeditionLevelData)clone_to(d *dbPlayerExpeditionLevelData){
	d.Level = this.Level
	d.PlayerId = this.PlayerId
	d.Power = this.Power
	d.GoldIncome = this.GoldIncome
	d.ExpeditionGoldIncome = this.ExpeditionGoldIncome
	return
}
type dbPlayerExpeditionLevelRoleData struct{
	Pos int32
	TableId int32
	Rank int32
	Level int32
	Equip []int32
	HP int32
	HpPercent int32
}
func (this* dbPlayerExpeditionLevelRoleData)from_pb(pb *db.PlayerExpeditionLevelRole){
	if pb == nil {
		this.Equip = make([]int32,0)
		return
	}
	this.Pos = pb.GetPos()
	this.TableId = pb.GetTableId()
	this.Rank = pb.GetRank()
	this.Level = pb.GetLevel()
	this.Equip = make([]int32,len(pb.GetEquip()))
	for i, v := range pb.GetEquip() {
		this.Equip[i] = v
	}
	this.HP = pb.GetHP()
	this.HpPercent = pb.GetHpPercent()
	return
}
func (this* dbPlayerExpeditionLevelRoleData)to_pb()(pb *db.PlayerExpeditionLevelRole){
	pb = &db.PlayerExpeditionLevelRole{}
	pb.Pos = proto.Int32(this.Pos)
	pb.TableId = proto.Int32(this.TableId)
	pb.Rank = proto.Int32(this.Rank)
	pb.Level = proto.Int32(this.Level)
	pb.Equip = make([]int32, len(this.Equip))
	for i, v := range this.Equip {
		pb.Equip[i]=v
	}
	pb.HP = proto.Int32(this.HP)
	pb.HpPercent = proto.Int32(this.HpPercent)
	return
}
func (this* dbPlayerExpeditionLevelRoleData)clone_to(d *dbPlayerExpeditionLevelRoleData){
	d.Pos = this.Pos
	d.TableId = this.TableId
	d.Rank = this.Rank
	d.Level = this.Level
	d.Equip = make([]int32, len(this.Equip))
	for _ii, _vv := range this.Equip {
		d.Equip[_ii]=_vv
	}
	d.HP = this.HP
	d.HpPercent = this.HpPercent
	return
}
type dbPlayerSysMailData struct{
	CurrId int32
}
func (this* dbPlayerSysMailData)from_pb(pb *db.PlayerSysMail){
	if pb == nil {
		return
	}
	this.CurrId = pb.GetCurrId()
	return
}
func (this* dbPlayerSysMailData)to_pb()(pb *db.PlayerSysMail){
	pb = &db.PlayerSysMail{}
	pb.CurrId = proto.Int32(this.CurrId)
	return
}
func (this* dbPlayerSysMailData)clone_to(d *dbPlayerSysMailData){
	d.CurrId = this.CurrId
	return
}
type dbPlayerArtifactData struct{
	Id int32
	Rank int32
	Level int32
}
func (this* dbPlayerArtifactData)from_pb(pb *db.PlayerArtifact){
	if pb == nil {
		return
	}
	this.Id = pb.GetId()
	this.Rank = pb.GetRank()
	this.Level = pb.GetLevel()
	return
}
func (this* dbPlayerArtifactData)to_pb()(pb *db.PlayerArtifact){
	pb = &db.PlayerArtifact{}
	pb.Id = proto.Int32(this.Id)
	pb.Rank = proto.Int32(this.Rank)
	pb.Level = proto.Int32(this.Level)
	return
}
func (this* dbPlayerArtifactData)clone_to(d *dbPlayerArtifactData){
	d.Id = this.Id
	d.Rank = this.Rank
	d.Level = this.Level
	return
}
type dbPlayerCarnivalCommonData struct{
	DayResetTime int32
}
func (this* dbPlayerCarnivalCommonData)from_pb(pb *db.PlayerCarnivalCommon){
	if pb == nil {
		return
	}
	this.DayResetTime = pb.GetDayResetTime()
	return
}
func (this* dbPlayerCarnivalCommonData)to_pb()(pb *db.PlayerCarnivalCommon){
	pb = &db.PlayerCarnivalCommon{}
	pb.DayResetTime = proto.Int32(this.DayResetTime)
	return
}
func (this* dbPlayerCarnivalCommonData)clone_to(d *dbPlayerCarnivalCommonData){
	d.DayResetTime = this.DayResetTime
	return
}
type dbPlayerCarnivalData struct{
	Id int32
	Value int32
	Value2 int32
}
func (this* dbPlayerCarnivalData)from_pb(pb *db.PlayerCarnival){
	if pb == nil {
		return
	}
	this.Id = pb.GetId()
	this.Value = pb.GetValue()
	this.Value2 = pb.GetValue2()
	return
}
func (this* dbPlayerCarnivalData)to_pb()(pb *db.PlayerCarnival){
	pb = &db.PlayerCarnival{}
	pb.Id = proto.Int32(this.Id)
	pb.Value = proto.Int32(this.Value)
	pb.Value2 = proto.Int32(this.Value2)
	return
}
func (this* dbPlayerCarnivalData)clone_to(d *dbPlayerCarnivalData){
	d.Id = this.Id
	d.Value = this.Value
	d.Value2 = this.Value2
	return
}
type dbPlayerInviteCodesData struct{
	Code string
}
func (this* dbPlayerInviteCodesData)from_pb(pb *db.PlayerInviteCodes){
	if pb == nil {
		return
	}
	this.Code = pb.GetCode()
	return
}
func (this* dbPlayerInviteCodesData)to_pb()(pb *db.PlayerInviteCodes){
	pb = &db.PlayerInviteCodes{}
	pb.Code = proto.String(this.Code)
	return
}
func (this* dbPlayerInviteCodesData)clone_to(d *dbPlayerInviteCodesData){
	d.Code = this.Code
	return
}
type dbBattleSaveDataData struct{
	Data []byte
}
func (this* dbBattleSaveDataData)from_pb(pb *db.BattleSaveData){
	if pb == nil {
		return
	}
	this.Data = pb.GetData()
	return
}
func (this* dbBattleSaveDataData)to_pb()(pb *db.BattleSaveData){
	pb = &db.BattleSaveData{}
	pb.Data = this.Data
	return
}
func (this* dbBattleSaveDataData)clone_to(d *dbBattleSaveDataData){
	d.Data = make([]byte, len(this.Data))
	for _ii, _vv := range this.Data {
		d.Data[_ii]=_vv
	}
	return
}
type dbTowerFightSaveDataData struct{
	Data []byte
}
func (this* dbTowerFightSaveDataData)from_pb(pb *db.TowerFightSaveData){
	if pb == nil {
		return
	}
	this.Data = pb.GetData()
	return
}
func (this* dbTowerFightSaveDataData)to_pb()(pb *db.TowerFightSaveData){
	pb = &db.TowerFightSaveData{}
	pb.Data = this.Data
	return
}
func (this* dbTowerFightSaveDataData)clone_to(d *dbTowerFightSaveDataData){
	d.Data = make([]byte, len(this.Data))
	for _ii, _vv := range this.Data {
		d.Data[_ii]=_vv
	}
	return
}
type dbArenaSeasonDataData struct{
	LastDayResetTime int32
	LastSeasonResetTime int32
}
func (this* dbArenaSeasonDataData)from_pb(pb *db.ArenaSeasonData){
	if pb == nil {
		return
	}
	this.LastDayResetTime = pb.GetLastDayResetTime()
	this.LastSeasonResetTime = pb.GetLastSeasonResetTime()
	return
}
func (this* dbArenaSeasonDataData)to_pb()(pb *db.ArenaSeasonData){
	pb = &db.ArenaSeasonData{}
	pb.LastDayResetTime = proto.Int32(this.LastDayResetTime)
	pb.LastSeasonResetTime = proto.Int32(this.LastSeasonResetTime)
	return
}
func (this* dbArenaSeasonDataData)clone_to(d *dbArenaSeasonDataData){
	d.LastDayResetTime = this.LastDayResetTime
	d.LastSeasonResetTime = this.LastSeasonResetTime
	return
}
type dbGuildMemberData struct{
	PlayerId int32
}
func (this* dbGuildMemberData)from_pb(pb *db.GuildMember){
	if pb == nil {
		return
	}
	this.PlayerId = pb.GetPlayerId()
	return
}
func (this* dbGuildMemberData)to_pb()(pb *db.GuildMember){
	pb = &db.GuildMember{}
	pb.PlayerId = proto.Int32(this.PlayerId)
	return
}
func (this* dbGuildMemberData)clone_to(d *dbGuildMemberData){
	d.PlayerId = this.PlayerId
	return
}
type dbGuildAskListData struct{
	PlayerId int32
}
func (this* dbGuildAskListData)from_pb(pb *db.GuildAskList){
	if pb == nil {
		return
	}
	this.PlayerId = pb.GetPlayerId()
	return
}
func (this* dbGuildAskListData)to_pb()(pb *db.GuildAskList){
	pb = &db.GuildAskList{}
	pb.PlayerId = proto.Int32(this.PlayerId)
	return
}
func (this* dbGuildAskListData)clone_to(d *dbGuildAskListData){
	d.PlayerId = this.PlayerId
	return
}
type dbGuildLogData struct{
	Id int32
	LogType int32
	PlayerId int32
	Time int32
}
func (this* dbGuildLogData)from_pb(pb *db.GuildLog){
	if pb == nil {
		return
	}
	this.Id = pb.GetId()
	this.LogType = pb.GetLogType()
	this.PlayerId = pb.GetPlayerId()
	this.Time = pb.GetTime()
	return
}
func (this* dbGuildLogData)to_pb()(pb *db.GuildLog){
	pb = &db.GuildLog{}
	pb.Id = proto.Int32(this.Id)
	pb.LogType = proto.Int32(this.LogType)
	pb.PlayerId = proto.Int32(this.PlayerId)
	pb.Time = proto.Int32(this.Time)
	return
}
func (this* dbGuildLogData)clone_to(d *dbGuildLogData){
	d.Id = this.Id
	d.LogType = this.LogType
	d.PlayerId = this.PlayerId
	d.Time = this.Time
	return
}
type dbGuildAskDonateData struct{
	PlayerId int32
	ItemId int32
	ItemNum int32
	AskTime int32
}
func (this* dbGuildAskDonateData)from_pb(pb *db.GuildAskDonate){
	if pb == nil {
		return
	}
	this.PlayerId = pb.GetPlayerId()
	this.ItemId = pb.GetItemId()
	this.ItemNum = pb.GetItemNum()
	this.AskTime = pb.GetAskTime()
	return
}
func (this* dbGuildAskDonateData)to_pb()(pb *db.GuildAskDonate){
	pb = &db.GuildAskDonate{}
	pb.PlayerId = proto.Int32(this.PlayerId)
	pb.ItemId = proto.Int32(this.ItemId)
	pb.ItemNum = proto.Int32(this.ItemNum)
	pb.AskTime = proto.Int32(this.AskTime)
	return
}
func (this* dbGuildAskDonateData)clone_to(d *dbGuildAskDonateData){
	d.PlayerId = this.PlayerId
	d.ItemId = this.ItemId
	d.ItemNum = this.ItemNum
	d.AskTime = this.AskTime
	return
}
type dbGuildStageData struct{
	BossId int32
	HpPercent int32
	BossPos int32
	BossHP int32
}
func (this* dbGuildStageData)from_pb(pb *db.GuildStage){
	if pb == nil {
		return
	}
	this.BossId = pb.GetBossId()
	this.HpPercent = pb.GetHpPercent()
	this.BossPos = pb.GetBossPos()
	this.BossHP = pb.GetBossHP()
	return
}
func (this* dbGuildStageData)to_pb()(pb *db.GuildStage){
	pb = &db.GuildStage{}
	pb.BossId = proto.Int32(this.BossId)
	pb.HpPercent = proto.Int32(this.HpPercent)
	pb.BossPos = proto.Int32(this.BossPos)
	pb.BossHP = proto.Int32(this.BossHP)
	return
}
func (this* dbGuildStageData)clone_to(d *dbGuildStageData){
	d.BossId = this.BossId
	d.HpPercent = this.HpPercent
	d.BossPos = this.BossPos
	d.BossHP = this.BossHP
	return
}
type dbGuildStageDamageLogData struct{
	AttackerId int32
	Damage int32
}
func (this* dbGuildStageDamageLogData)from_pb(pb *db.GuildStageDamageLog){
	if pb == nil {
		return
	}
	this.AttackerId = pb.GetAttackerId()
	this.Damage = pb.GetDamage()
	return
}
func (this* dbGuildStageDamageLogData)to_pb()(pb *db.GuildStageDamageLog){
	pb = &db.GuildStageDamageLog{}
	pb.AttackerId = proto.Int32(this.AttackerId)
	pb.Damage = proto.Int32(this.Damage)
	return
}
func (this* dbGuildStageDamageLogData)clone_to(d *dbGuildStageDamageLogData){
	d.AttackerId = this.AttackerId
	d.Damage = this.Damage
	return
}
type dbSysMailAttachedItemsData struct{
	ItemList []int32
}
func (this* dbSysMailAttachedItemsData)from_pb(pb *db.SysMailAttachedItems){
	if pb == nil {
		this.ItemList = make([]int32,0)
		return
	}
	this.ItemList = make([]int32,len(pb.GetItemList()))
	for i, v := range pb.GetItemList() {
		this.ItemList[i] = v
	}
	return
}
func (this* dbSysMailAttachedItemsData)to_pb()(pb *db.SysMailAttachedItems){
	pb = &db.SysMailAttachedItems{}
	pb.ItemList = make([]int32, len(this.ItemList))
	for i, v := range this.ItemList {
		pb.ItemList[i]=v
	}
	return
}
func (this* dbSysMailAttachedItemsData)clone_to(d *dbSysMailAttachedItemsData){
	d.ItemList = make([]int32, len(this.ItemList))
	for _ii, _vv := range this.ItemList {
		d.ItemList[_ii]=_vv
	}
	return
}

func (this *dbGlobalRow)GetCurrentPlayerId( )(r int32 ){
	this.m_lock.UnSafeRLock("dbGlobalRow.GetdbGlobalCurrentPlayerIdColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_CurrentPlayerId)
}
func (this *dbGlobalRow)SetCurrentPlayerId(v int32){
	this.m_lock.UnSafeLock("dbGlobalRow.SetdbGlobalCurrentPlayerIdColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_CurrentPlayerId=int32(v)
	this.m_CurrentPlayerId_changed=true
	return
}
func (this *dbGlobalRow)GetCurrentGuildId( )(r int32 ){
	this.m_lock.UnSafeRLock("dbGlobalRow.GetdbGlobalCurrentGuildIdColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_CurrentGuildId)
}
func (this *dbGlobalRow)SetCurrentGuildId(v int32){
	this.m_lock.UnSafeLock("dbGlobalRow.SetdbGlobalCurrentGuildIdColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_CurrentGuildId=int32(v)
	this.m_CurrentGuildId_changed=true
	return
}
type dbGlobalRow struct {
	m_table *dbGlobalTable
	m_lock       *RWMutex
	m_loaded  bool
	m_new     bool
	m_remove  bool
	m_touch      int32
	m_releasable bool
	m_valid   bool
	m_Id        int32
	m_CurrentPlayerId_changed bool
	m_CurrentPlayerId int32
	m_CurrentGuildId_changed bool
	m_CurrentGuildId int32
}
func new_dbGlobalRow(table *dbGlobalTable, Id int32) (r *dbGlobalRow) {
	this := &dbGlobalRow{}
	this.m_table = table
	this.m_Id = Id
	this.m_lock = NewRWMutex()
	this.m_CurrentPlayerId_changed=true
	this.m_CurrentGuildId_changed=true
	return this
}
func (this *dbGlobalRow) save_data(release bool) (err error, released bool, state int32, update_string string, args []interface{}) {
	this.m_lock.UnSafeLock("dbGlobalRow.save_data")
	defer this.m_lock.UnSafeUnlock()
	if this.m_new {
		db_args:=new_db_args(3)
		db_args.Push(this.m_Id)
		db_args.Push(this.m_CurrentPlayerId)
		db_args.Push(this.m_CurrentGuildId)
		args=db_args.GetArgs()
		state = 1
	} else {
		if this.m_CurrentPlayerId_changed||this.m_CurrentGuildId_changed{
			update_string = "UPDATE Global SET "
			db_args:=new_db_args(3)
			if this.m_CurrentPlayerId_changed{
				update_string+="CurrentPlayerId=?,"
				db_args.Push(this.m_CurrentPlayerId)
			}
			if this.m_CurrentGuildId_changed{
				update_string+="CurrentGuildId=?,"
				db_args.Push(this.m_CurrentGuildId)
			}
			update_string = strings.TrimRight(update_string, ", ")
			update_string+=" WHERE Id=?"
			db_args.Push(this.m_Id)
			args=db_args.GetArgs()
			state = 2
		}
	}
	this.m_new = false
	this.m_CurrentPlayerId_changed = false
	this.m_CurrentGuildId_changed = false
	if release && this.m_loaded {
		this.m_loaded = false
		released = true
	}
	return nil,released,state,update_string,args
}
func (this *dbGlobalRow) Save(release bool) (err error, d bool, released bool) {
	err,released, state, update_string, args := this.save_data(release)
	if err != nil {
		log.Error("save data failed")
		return err, false, false
	}
	if state == 0 {
		d = false
	} else if state == 1 {
		_, err = this.m_table.m_dbc.StmtExec(this.m_table.m_save_insert_stmt, args...)
		if err != nil {
			log.Error("INSERT Global exec failed %v ", this.m_Id)
			return err, false, released
		}
		d = true
	} else if state == 2 {
		_, err = this.m_table.m_dbc.Exec(update_string, args...)
		if err != nil {
			log.Error("UPDATE Global exec failed %v", this.m_Id)
			return err, false, released
		}
		d = true
	}
	return nil, d, released
}
type dbGlobalTable struct{
	m_dbc *DBC
	m_lock *RWMutex
	m_row *dbGlobalRow
	m_preload_select_stmt *sql.Stmt
	m_save_insert_stmt *sql.Stmt
}
func new_dbGlobalTable(dbc *DBC) (this *dbGlobalTable) {
	this = &dbGlobalTable{}
	this.m_dbc = dbc
	this.m_lock = NewRWMutex()
	return this
}
func (this *dbGlobalTable) check_create_table() (err error) {
	_, err = this.m_dbc.Exec("CREATE TABLE IF NOT EXISTS Global(Id int(11),PRIMARY KEY (Id))ENGINE=InnoDB ROW_FORMAT=DYNAMIC")
	if err != nil {
		log.Error("CREATE TABLE IF NOT EXISTS Global failed")
		return
	}
	rows, err := this.m_dbc.Query("SELECT COLUMN_NAME,ORDINAL_POSITION FROM information_schema.`COLUMNS` WHERE TABLE_SCHEMA=? AND TABLE_NAME='Global'", this.m_dbc.m_db_name)
	if err != nil {
		log.Error("SELECT information_schema failed")
		return
	}
	columns := make(map[string]int32)
	for rows.Next() {
		var column_name string
		var ordinal_position int32
		err = rows.Scan(&column_name, &ordinal_position)
		if err != nil {
			log.Error("scan information_schema row failed")
			return
		}
		if ordinal_position < 1 {
			log.Error("col ordinal out of range")
			continue
		}
		columns[column_name] = ordinal_position
	}
	_, hasCurrentPlayerId := columns["CurrentPlayerId"]
	if !hasCurrentPlayerId {
		_, err = this.m_dbc.Exec("ALTER TABLE Global ADD COLUMN CurrentPlayerId int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN CurrentPlayerId failed")
			return
		}
	}
	_, hasCurrentGuildId := columns["CurrentGuildId"]
	if !hasCurrentGuildId {
		_, err = this.m_dbc.Exec("ALTER TABLE Global ADD COLUMN CurrentGuildId int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN CurrentGuildId failed")
			return
		}
	}
	return
}
func (this *dbGlobalTable) prepare_preload_select_stmt() (err error) {
	this.m_preload_select_stmt,err=this.m_dbc.StmtPrepare("SELECT CurrentPlayerId,CurrentGuildId FROM Global WHERE Id=0")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbGlobalTable) prepare_save_insert_stmt()(err error){
	this.m_save_insert_stmt,err=this.m_dbc.StmtPrepare("INSERT INTO Global (Id,CurrentPlayerId,CurrentGuildId) VALUES (?,?,?)")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbGlobalTable) Init() (err error) {
	err=this.check_create_table()
	if err!=nil{
		log.Error("check_create_table failed")
		return
	}
	err=this.prepare_preload_select_stmt()
	if err!=nil{
		log.Error("prepare_preload_select_stmt failed")
		return
	}
	err=this.prepare_save_insert_stmt()
	if err!=nil{
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	return
}
func (this *dbGlobalTable) Preload() (err error) {
	r := this.m_dbc.StmtQueryRow(this.m_preload_select_stmt)
	var dCurrentPlayerId int32
	var dCurrentGuildId int32
	err = r.Scan(&dCurrentPlayerId,&dCurrentGuildId)
	if err!=nil{
		if err!=sql.ErrNoRows{
			log.Error("Scan failed")
			return
		}
	}else{
		row := new_dbGlobalRow(this,0)
		row.m_CurrentPlayerId=dCurrentPlayerId
		row.m_CurrentGuildId=dCurrentGuildId
		row.m_CurrentPlayerId_changed=false
		row.m_CurrentGuildId_changed=false
		row.m_valid = true
		row.m_loaded=true
		this.m_row=row
	}
	if this.m_row == nil {
		this.m_row = new_dbGlobalRow(this, 0)
		this.m_row.m_new = true
		this.m_row.m_valid = true
		err = this.Save(false)
		if err != nil {
			log.Error("save failed")
			return
		}
		this.m_row.m_loaded = true
	}
	return
}
func (this *dbGlobalTable) Save(quick bool) (err error) {
	if this.m_row==nil{
		return errors.New("row nil")
	}
	err, _, _ = this.m_row.Save(false)
	return err
}
func (this *dbGlobalTable) GetRow( ) (row *dbGlobalRow) {
	return this.m_row
}
func (this *dbPlayerRow)GetUniqueId( )(r string ){
	this.m_lock.UnSafeRLock("dbPlayerRow.GetdbPlayerUniqueIdColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_UniqueId)
}
func (this *dbPlayerRow)SetUniqueId(v string){
	this.m_lock.UnSafeLock("dbPlayerRow.SetdbPlayerUniqueIdColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_UniqueId=string(v)
	this.m_UniqueId_changed=true
	return
}
func (this *dbPlayerRow)GetAccount( )(r string ){
	this.m_lock.UnSafeRLock("dbPlayerRow.GetdbPlayerAccountColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_Account)
}
func (this *dbPlayerRow)SetAccount(v string){
	this.m_lock.UnSafeLock("dbPlayerRow.SetdbPlayerAccountColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Account=string(v)
	this.m_Account_changed=true
	return
}
func (this *dbPlayerRow)GetName( )(r string ){
	this.m_lock.UnSafeRLock("dbPlayerRow.GetdbPlayerNameColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_Name)
}
func (this *dbPlayerRow)SetName(v string){
	this.m_lock.UnSafeLock("dbPlayerRow.SetdbPlayerNameColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Name=string(v)
	this.m_Name_changed=true
	return
}
func (this *dbPlayerRow)GetToken( )(r string ){
	this.m_lock.UnSafeRLock("dbPlayerRow.GetdbPlayerTokenColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_Token)
}
func (this *dbPlayerRow)SetToken(v string){
	this.m_lock.UnSafeLock("dbPlayerRow.SetdbPlayerTokenColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Token=string(v)
	this.m_Token_changed=true
	return
}
func (this *dbPlayerRow)GetCurrReplyMsgNum( )(r int32 ){
	this.m_lock.UnSafeRLock("dbPlayerRow.GetdbPlayerCurrReplyMsgNumColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_CurrReplyMsgNum)
}
func (this *dbPlayerRow)SetCurrReplyMsgNum(v int32){
	this.m_lock.UnSafeLock("dbPlayerRow.SetdbPlayerCurrReplyMsgNumColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_CurrReplyMsgNum=int32(v)
	this.m_CurrReplyMsgNum_changed=true
	return
}
type dbPlayerInfoColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerInfoData
	m_changed bool
}
func (this *dbPlayerInfoColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerInfoData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerInfo{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerInfoData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerInfoColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerInfoColumn)Get( )(v *dbPlayerInfoData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerInfoColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerInfoData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerInfoColumn)Set(v dbPlayerInfoData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerInfoColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerInfoData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerInfoColumn)GetLvl( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerInfoColumn.GetLvl")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.Lvl
	return
}
func (this *dbPlayerInfoColumn)SetLvl(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerInfoColumn.SetLvl")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.Lvl = v
	this.m_changed = true
	return
}
func (this *dbPlayerInfoColumn)GetExp( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerInfoColumn.GetExp")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.Exp
	return
}
func (this *dbPlayerInfoColumn)SetExp(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerInfoColumn.SetExp")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.Exp = v
	this.m_changed = true
	return
}
func (this *dbPlayerInfoColumn)IncbyExp(v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerInfoColumn.IncbyExp")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.Exp += v
	this.m_changed = true
	return this.m_data.Exp
}
func (this *dbPlayerInfoColumn)GetCreateUnix( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerInfoColumn.GetCreateUnix")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.CreateUnix
	return
}
func (this *dbPlayerInfoColumn)SetCreateUnix(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerInfoColumn.SetCreateUnix")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.CreateUnix = v
	this.m_changed = true
	return
}
func (this *dbPlayerInfoColumn)GetGold( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerInfoColumn.GetGold")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.Gold
	return
}
func (this *dbPlayerInfoColumn)SetGold(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerInfoColumn.SetGold")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.Gold = v
	this.m_changed = true
	return
}
func (this *dbPlayerInfoColumn)IncbyGold(v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerInfoColumn.IncbyGold")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.Gold += v
	this.m_changed = true
	return this.m_data.Gold
}
func (this *dbPlayerInfoColumn)GetDiamond( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerInfoColumn.GetDiamond")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.Diamond
	return
}
func (this *dbPlayerInfoColumn)SetDiamond(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerInfoColumn.SetDiamond")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.Diamond = v
	this.m_changed = true
	return
}
func (this *dbPlayerInfoColumn)IncbyDiamond(v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerInfoColumn.IncbyDiamond")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.Diamond += v
	this.m_changed = true
	return this.m_data.Diamond
}
func (this *dbPlayerInfoColumn)GetLastLogout( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerInfoColumn.GetLastLogout")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LastLogout
	return
}
func (this *dbPlayerInfoColumn)SetLastLogout(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerInfoColumn.SetLastLogout")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LastLogout = v
	this.m_changed = true
	return
}
func (this *dbPlayerInfoColumn)GetLastLogin( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerInfoColumn.GetLastLogin")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LastLogin
	return
}
func (this *dbPlayerInfoColumn)SetLastLogin(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerInfoColumn.SetLastLogin")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LastLogin = v
	this.m_changed = true
	return
}
func (this *dbPlayerInfoColumn)GetVipLvl( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerInfoColumn.GetVipLvl")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.VipLvl
	return
}
func (this *dbPlayerInfoColumn)SetVipLvl(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerInfoColumn.SetVipLvl")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.VipLvl = v
	this.m_changed = true
	return
}
func (this *dbPlayerInfoColumn)GetHead( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerInfoColumn.GetHead")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.Head
	return
}
func (this *dbPlayerInfoColumn)SetHead(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerInfoColumn.SetHead")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.Head = v
	this.m_changed = true
	return
}
type dbPlayerGlobalColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerGlobalData
	m_changed bool
}
func (this *dbPlayerGlobalColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerGlobalData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerGlobal{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerGlobalData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerGlobalColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerGlobalColumn)Get( )(v *dbPlayerGlobalData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerGlobalColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerGlobalData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerGlobalColumn)Set(v dbPlayerGlobalData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerGlobalColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerGlobalData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerGlobalColumn)GetCurrentRoleId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerGlobalColumn.GetCurrentRoleId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.CurrentRoleId
	return
}
func (this *dbPlayerGlobalColumn)SetCurrentRoleId(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerGlobalColumn.SetCurrentRoleId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.CurrentRoleId = v
	this.m_changed = true
	return
}
func (this *dbPlayerGlobalColumn)IncbyCurrentRoleId(v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerGlobalColumn.IncbyCurrentRoleId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.CurrentRoleId += v
	this.m_changed = true
	return this.m_data.CurrentRoleId
}
func (this *dbPlayerRow)GetLevel( )(r int32 ){
	this.m_lock.UnSafeRLock("dbPlayerRow.GetdbPlayerLevelColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_Level)
}
func (this *dbPlayerRow)SetLevel(v int32){
	this.m_lock.UnSafeLock("dbPlayerRow.SetdbPlayerLevelColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Level=int32(v)
	this.m_Level_changed=true
	return
}
type dbPlayerItemColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerItemData
	m_changed bool
}
func (this *dbPlayerItemColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerItemList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerItemData{}
		d.from_pb(v)
		this.m_data[int32(d.Id)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerItemColumn)save( )(data []byte,err error){
	pb := &db.PlayerItemList{}
	pb.List=make([]*db.PlayerItem,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerItemColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerItemColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerItemColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerItemColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerItemColumn)GetAll()(list []dbPlayerItemData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerItemColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerItemData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerItemColumn)Get(id int32)(v *dbPlayerItemData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerItemColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerItemData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerItemColumn)Set(v dbPlayerItemData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerItemColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Id)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerItemColumn)Add(v *dbPlayerItemData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerItemColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Id)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	d:=&dbPlayerItemData{}
	v.clone_to(d)
	this.m_data[int32(v.Id)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerItemColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerItemColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerItemColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerItemColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerItemData)
	this.m_changed = true
	return
}
func (this *dbPlayerItemColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerItemColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerItemColumn)GetCount(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerItemColumn.GetCount")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Count
	return v,true
}
func (this *dbPlayerItemColumn)SetCount(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerItemColumn.SetCount")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Count = v
	this.m_changed = true
	return true
}
func (this *dbPlayerItemColumn)IncbyCount(id int32,v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerItemColumn.IncbyCount")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		d = &dbPlayerItemData{}
		this.m_data[id] = d
	}
	d.Count +=  v
	this.m_changed = true
	return d.Count
}
type dbPlayerRoleCommonColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerRoleCommonData
	m_changed bool
}
func (this *dbPlayerRoleCommonColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerRoleCommonData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerRoleCommon{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerRoleCommonData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerRoleCommonColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerRoleCommonColumn)Get( )(v *dbPlayerRoleCommonData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerRoleCommonColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerRoleCommonData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerRoleCommonColumn)Set(v dbPlayerRoleCommonData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerRoleCommonColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerRoleCommonData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerRoleCommonColumn)GetDisplaceRoleId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerRoleCommonColumn.GetDisplaceRoleId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.DisplaceRoleId
	return
}
func (this *dbPlayerRoleCommonColumn)SetDisplaceRoleId(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerRoleCommonColumn.SetDisplaceRoleId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.DisplaceRoleId = v
	this.m_changed = true
	return
}
func (this *dbPlayerRoleCommonColumn)GetDisplacedNewRoleTableId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerRoleCommonColumn.GetDisplacedNewRoleTableId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.DisplacedNewRoleTableId
	return
}
func (this *dbPlayerRoleCommonColumn)SetDisplacedNewRoleTableId(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerRoleCommonColumn.SetDisplacedNewRoleTableId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.DisplacedNewRoleTableId = v
	this.m_changed = true
	return
}
func (this *dbPlayerRoleCommonColumn)GetDisplaceGroupId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerRoleCommonColumn.GetDisplaceGroupId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.DisplaceGroupId
	return
}
func (this *dbPlayerRoleCommonColumn)SetDisplaceGroupId(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerRoleCommonColumn.SetDisplaceGroupId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.DisplaceGroupId = v
	this.m_changed = true
	return
}
func (this *dbPlayerRoleCommonColumn)GetPowerUpdateTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerRoleCommonColumn.GetPowerUpdateTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.PowerUpdateTime
	return
}
func (this *dbPlayerRoleCommonColumn)SetPowerUpdateTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerRoleCommonColumn.SetPowerUpdateTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.PowerUpdateTime = v
	this.m_changed = true
	return
}
type dbPlayerRoleColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerRoleData
	m_changed bool
}
func (this *dbPlayerRoleColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerRoleList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerRoleData{}
		d.from_pb(v)
		this.m_data[int32(d.Id)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerRoleColumn)save( )(data []byte,err error){
	pb := &db.PlayerRoleList{}
	pb.List=make([]*db.PlayerRole,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerRoleColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerRoleColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerRoleColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerRoleColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerRoleColumn)GetAll()(list []dbPlayerRoleData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerRoleColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerRoleData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerRoleColumn)Get(id int32)(v *dbPlayerRoleData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerRoleColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerRoleData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerRoleColumn)Set(v dbPlayerRoleData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerRoleColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Id)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerRoleColumn)Add(v *dbPlayerRoleData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerRoleColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Id)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	d:=&dbPlayerRoleData{}
	v.clone_to(d)
	this.m_data[int32(v.Id)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerRoleColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerRoleColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerRoleColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerRoleColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerRoleData)
	this.m_changed = true
	return
}
func (this *dbPlayerRoleColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerRoleColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerRoleColumn)GetTableId(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerRoleColumn.GetTableId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.TableId
	return v,true
}
func (this *dbPlayerRoleColumn)SetTableId(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerRoleColumn.SetTableId")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.TableId = v
	this.m_changed = true
	return true
}
func (this *dbPlayerRoleColumn)GetRank(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerRoleColumn.GetRank")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Rank
	return v,true
}
func (this *dbPlayerRoleColumn)SetRank(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerRoleColumn.SetRank")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Rank = v
	this.m_changed = true
	return true
}
func (this *dbPlayerRoleColumn)GetLevel(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerRoleColumn.GetLevel")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Level
	return v,true
}
func (this *dbPlayerRoleColumn)SetLevel(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerRoleColumn.SetLevel")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Level = v
	this.m_changed = true
	return true
}
func (this *dbPlayerRoleColumn)GetEquip(id int32)(v []int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerRoleColumn.GetEquip")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = make([]int32, len(d.Equip))
	for _ii, _vv := range d.Equip {
		v[_ii]=_vv
	}
	return v,true
}
func (this *dbPlayerRoleColumn)SetEquip(id int32,v []int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerRoleColumn.SetEquip")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Equip = make([]int32, len(v))
	for _ii, _vv := range v {
		d.Equip[_ii]=_vv
	}
	this.m_changed = true
	return true
}
func (this *dbPlayerRoleColumn)GetIsLock(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerRoleColumn.GetIsLock")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.IsLock
	return v,true
}
func (this *dbPlayerRoleColumn)SetIsLock(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerRoleColumn.SetIsLock")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.IsLock = v
	this.m_changed = true
	return true
}
func (this *dbPlayerRoleColumn)GetState(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerRoleColumn.GetState")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.State
	return v,true
}
func (this *dbPlayerRoleColumn)SetState(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerRoleColumn.SetState")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.State = v
	this.m_changed = true
	return true
}
type dbPlayerRoleHandbookColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerRoleHandbookData
	m_changed bool
}
func (this *dbPlayerRoleHandbookColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerRoleHandbookData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerRoleHandbook{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerRoleHandbookData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerRoleHandbookColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerRoleHandbookColumn)Get( )(v *dbPlayerRoleHandbookData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerRoleHandbookColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerRoleHandbookData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerRoleHandbookColumn)Set(v dbPlayerRoleHandbookData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerRoleHandbookColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerRoleHandbookData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerRoleHandbookColumn)GetRole( )(v []int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerRoleHandbookColumn.GetRole")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = make([]int32, len(this.m_data.Role))
	for _ii, _vv := range this.m_data.Role {
		v[_ii]=_vv
	}
	return
}
func (this *dbPlayerRoleHandbookColumn)SetRole(v []int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerRoleHandbookColumn.SetRole")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.Role = make([]int32, len(v))
	for _ii, _vv := range v {
		this.m_data.Role[_ii]=_vv
	}
	this.m_changed = true
	return
}
type dbPlayerBattleTeamColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerBattleTeamData
	m_changed bool
}
func (this *dbPlayerBattleTeamColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerBattleTeamData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerBattleTeam{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerBattleTeamData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerBattleTeamColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerBattleTeamColumn)Get( )(v *dbPlayerBattleTeamData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerBattleTeamColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerBattleTeamData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerBattleTeamColumn)Set(v dbPlayerBattleTeamData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerBattleTeamColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerBattleTeamData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerBattleTeamColumn)GetDefenseMembers( )(v []int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerBattleTeamColumn.GetDefenseMembers")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = make([]int32, len(this.m_data.DefenseMembers))
	for _ii, _vv := range this.m_data.DefenseMembers {
		v[_ii]=_vv
	}
	return
}
func (this *dbPlayerBattleTeamColumn)SetDefenseMembers(v []int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerBattleTeamColumn.SetDefenseMembers")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.DefenseMembers = make([]int32, len(v))
	for _ii, _vv := range v {
		this.m_data.DefenseMembers[_ii]=_vv
	}
	this.m_changed = true
	return
}
func (this *dbPlayerBattleTeamColumn)GetCampaignMembers( )(v []int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerBattleTeamColumn.GetCampaignMembers")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = make([]int32, len(this.m_data.CampaignMembers))
	for _ii, _vv := range this.m_data.CampaignMembers {
		v[_ii]=_vv
	}
	return
}
func (this *dbPlayerBattleTeamColumn)SetCampaignMembers(v []int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerBattleTeamColumn.SetCampaignMembers")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.CampaignMembers = make([]int32, len(v))
	for _ii, _vv := range v {
		this.m_data.CampaignMembers[_ii]=_vv
	}
	this.m_changed = true
	return
}
func (this *dbPlayerBattleTeamColumn)GetDefenseArtifactId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerBattleTeamColumn.GetDefenseArtifactId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.DefenseArtifactId
	return
}
func (this *dbPlayerBattleTeamColumn)SetDefenseArtifactId(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerBattleTeamColumn.SetDefenseArtifactId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.DefenseArtifactId = v
	this.m_changed = true
	return
}
func (this *dbPlayerBattleTeamColumn)GetCampaignArtifactId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerBattleTeamColumn.GetCampaignArtifactId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.CampaignArtifactId
	return
}
func (this *dbPlayerBattleTeamColumn)SetCampaignArtifactId(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerBattleTeamColumn.SetCampaignArtifactId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.CampaignArtifactId = v
	this.m_changed = true
	return
}
type dbPlayerCampaignCommonColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerCampaignCommonData
	m_changed bool
}
func (this *dbPlayerCampaignCommonColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerCampaignCommonData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerCampaignCommon{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerCampaignCommonData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerCampaignCommonColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerCampaignCommonColumn)Get( )(v *dbPlayerCampaignCommonData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignCommonColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerCampaignCommonData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerCampaignCommonColumn)Set(v dbPlayerCampaignCommonData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignCommonColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerCampaignCommonData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerCampaignCommonColumn)GetCurrentCampaignId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignCommonColumn.GetCurrentCampaignId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.CurrentCampaignId
	return
}
func (this *dbPlayerCampaignCommonColumn)SetCurrentCampaignId(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignCommonColumn.SetCurrentCampaignId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.CurrentCampaignId = v
	this.m_changed = true
	return
}
func (this *dbPlayerCampaignCommonColumn)GetHangupLastDropStaticIncomeTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignCommonColumn.GetHangupLastDropStaticIncomeTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.HangupLastDropStaticIncomeTime
	return
}
func (this *dbPlayerCampaignCommonColumn)SetHangupLastDropStaticIncomeTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignCommonColumn.SetHangupLastDropStaticIncomeTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.HangupLastDropStaticIncomeTime = v
	this.m_changed = true
	return
}
func (this *dbPlayerCampaignCommonColumn)GetHangupLastDropRandomIncomeTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignCommonColumn.GetHangupLastDropRandomIncomeTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.HangupLastDropRandomIncomeTime
	return
}
func (this *dbPlayerCampaignCommonColumn)SetHangupLastDropRandomIncomeTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignCommonColumn.SetHangupLastDropRandomIncomeTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.HangupLastDropRandomIncomeTime = v
	this.m_changed = true
	return
}
func (this *dbPlayerCampaignCommonColumn)GetHangupCampaignId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignCommonColumn.GetHangupCampaignId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.HangupCampaignId
	return
}
func (this *dbPlayerCampaignCommonColumn)SetHangupCampaignId(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignCommonColumn.SetHangupCampaignId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.HangupCampaignId = v
	this.m_changed = true
	return
}
func (this *dbPlayerCampaignCommonColumn)GetLastestPassedCampaignId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignCommonColumn.GetLastestPassedCampaignId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LastestPassedCampaignId
	return
}
func (this *dbPlayerCampaignCommonColumn)SetLastestPassedCampaignId(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignCommonColumn.SetLastestPassedCampaignId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LastestPassedCampaignId = v
	this.m_changed = true
	return
}
func (this *dbPlayerCampaignCommonColumn)GetRankSerialId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignCommonColumn.GetRankSerialId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.RankSerialId
	return
}
func (this *dbPlayerCampaignCommonColumn)SetRankSerialId(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignCommonColumn.SetRankSerialId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.RankSerialId = v
	this.m_changed = true
	return
}
func (this *dbPlayerCampaignCommonColumn)IncbyRankSerialId(v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignCommonColumn.IncbyRankSerialId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.RankSerialId += v
	this.m_changed = true
	return this.m_data.RankSerialId
}
func (this *dbPlayerCampaignCommonColumn)GetVipAccelNum( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignCommonColumn.GetVipAccelNum")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.VipAccelNum
	return
}
func (this *dbPlayerCampaignCommonColumn)SetVipAccelNum(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignCommonColumn.SetVipAccelNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.VipAccelNum = v
	this.m_changed = true
	return
}
func (this *dbPlayerCampaignCommonColumn)IncbyVipAccelNum(v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignCommonColumn.IncbyVipAccelNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.VipAccelNum += v
	this.m_changed = true
	return this.m_data.VipAccelNum
}
func (this *dbPlayerCampaignCommonColumn)GetVipAccelRefreshTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignCommonColumn.GetVipAccelRefreshTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.VipAccelRefreshTime
	return
}
func (this *dbPlayerCampaignCommonColumn)SetVipAccelRefreshTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignCommonColumn.SetVipAccelRefreshTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.VipAccelRefreshTime = v
	this.m_changed = true
	return
}
func (this *dbPlayerCampaignCommonColumn)IncbyVipAccelRefreshTime(v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignCommonColumn.IncbyVipAccelRefreshTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.VipAccelRefreshTime += v
	this.m_changed = true
	return this.m_data.VipAccelRefreshTime
}
func (this *dbPlayerCampaignCommonColumn)GetPassCampaginTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignCommonColumn.GetPassCampaginTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.PassCampaginTime
	return
}
func (this *dbPlayerCampaignCommonColumn)SetPassCampaginTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignCommonColumn.SetPassCampaginTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.PassCampaginTime = v
	this.m_changed = true
	return
}
type dbPlayerCampaignColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerCampaignData
	m_changed bool
}
func (this *dbPlayerCampaignColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerCampaignList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerCampaignData{}
		d.from_pb(v)
		this.m_data[int32(d.CampaignId)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerCampaignColumn)save( )(data []byte,err error){
	pb := &db.PlayerCampaignList{}
	pb.List=make([]*db.PlayerCampaign,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerCampaignColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerCampaignColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerCampaignColumn)GetAll()(list []dbPlayerCampaignData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerCampaignData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerCampaignColumn)Get(id int32)(v *dbPlayerCampaignData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerCampaignData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerCampaignColumn)Set(v dbPlayerCampaignData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.CampaignId)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.CampaignId)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerCampaignColumn)Add(v *dbPlayerCampaignData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.CampaignId)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.CampaignId)
		return false
	}
	d:=&dbPlayerCampaignData{}
	v.clone_to(d)
	this.m_data[int32(v.CampaignId)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerCampaignColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerCampaignColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerCampaignData)
	this.m_changed = true
	return
}
func (this *dbPlayerCampaignColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
type dbPlayerCampaignStaticIncomeColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerCampaignStaticIncomeData
	m_changed bool
}
func (this *dbPlayerCampaignStaticIncomeColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerCampaignStaticIncomeList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerCampaignStaticIncomeData{}
		d.from_pb(v)
		this.m_data[int32(d.ItemId)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerCampaignStaticIncomeColumn)save( )(data []byte,err error){
	pb := &db.PlayerCampaignStaticIncomeList{}
	pb.List=make([]*db.PlayerCampaignStaticIncome,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerCampaignStaticIncomeColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignStaticIncomeColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerCampaignStaticIncomeColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignStaticIncomeColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerCampaignStaticIncomeColumn)GetAll()(list []dbPlayerCampaignStaticIncomeData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignStaticIncomeColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerCampaignStaticIncomeData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerCampaignStaticIncomeColumn)Get(id int32)(v *dbPlayerCampaignStaticIncomeData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignStaticIncomeColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerCampaignStaticIncomeData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerCampaignStaticIncomeColumn)Set(v dbPlayerCampaignStaticIncomeData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignStaticIncomeColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.ItemId)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.ItemId)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerCampaignStaticIncomeColumn)Add(v *dbPlayerCampaignStaticIncomeData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignStaticIncomeColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.ItemId)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.ItemId)
		return false
	}
	d:=&dbPlayerCampaignStaticIncomeData{}
	v.clone_to(d)
	this.m_data[int32(v.ItemId)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerCampaignStaticIncomeColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignStaticIncomeColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerCampaignStaticIncomeColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignStaticIncomeColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerCampaignStaticIncomeData)
	this.m_changed = true
	return
}
func (this *dbPlayerCampaignStaticIncomeColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignStaticIncomeColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerCampaignStaticIncomeColumn)GetItemNum(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignStaticIncomeColumn.GetItemNum")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.ItemNum
	return v,true
}
func (this *dbPlayerCampaignStaticIncomeColumn)SetItemNum(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignStaticIncomeColumn.SetItemNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.ItemNum = v
	this.m_changed = true
	return true
}
func (this *dbPlayerCampaignStaticIncomeColumn)IncbyItemNum(id int32,v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignStaticIncomeColumn.IncbyItemNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		d = &dbPlayerCampaignStaticIncomeData{}
		this.m_data[id] = d
	}
	d.ItemNum +=  v
	this.m_changed = true
	return d.ItemNum
}
type dbPlayerCampaignRandomIncomeColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerCampaignRandomIncomeData
	m_changed bool
}
func (this *dbPlayerCampaignRandomIncomeColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerCampaignRandomIncomeList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerCampaignRandomIncomeData{}
		d.from_pb(v)
		this.m_data[int32(d.ItemId)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerCampaignRandomIncomeColumn)save( )(data []byte,err error){
	pb := &db.PlayerCampaignRandomIncomeList{}
	pb.List=make([]*db.PlayerCampaignRandomIncome,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerCampaignRandomIncomeColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignRandomIncomeColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerCampaignRandomIncomeColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignRandomIncomeColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerCampaignRandomIncomeColumn)GetAll()(list []dbPlayerCampaignRandomIncomeData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignRandomIncomeColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerCampaignRandomIncomeData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerCampaignRandomIncomeColumn)Get(id int32)(v *dbPlayerCampaignRandomIncomeData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignRandomIncomeColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerCampaignRandomIncomeData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerCampaignRandomIncomeColumn)Set(v dbPlayerCampaignRandomIncomeData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignRandomIncomeColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.ItemId)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.ItemId)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerCampaignRandomIncomeColumn)Add(v *dbPlayerCampaignRandomIncomeData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignRandomIncomeColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.ItemId)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.ItemId)
		return false
	}
	d:=&dbPlayerCampaignRandomIncomeData{}
	v.clone_to(d)
	this.m_data[int32(v.ItemId)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerCampaignRandomIncomeColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignRandomIncomeColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerCampaignRandomIncomeColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignRandomIncomeColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerCampaignRandomIncomeData)
	this.m_changed = true
	return
}
func (this *dbPlayerCampaignRandomIncomeColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignRandomIncomeColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerCampaignRandomIncomeColumn)GetItemNum(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCampaignRandomIncomeColumn.GetItemNum")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.ItemNum
	return v,true
}
func (this *dbPlayerCampaignRandomIncomeColumn)SetItemNum(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignRandomIncomeColumn.SetItemNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.ItemNum = v
	this.m_changed = true
	return true
}
func (this *dbPlayerCampaignRandomIncomeColumn)IncbyItemNum(id int32,v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCampaignRandomIncomeColumn.IncbyItemNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		d = &dbPlayerCampaignRandomIncomeData{}
		this.m_data[id] = d
	}
	d.ItemNum +=  v
	this.m_changed = true
	return d.ItemNum
}
type dbPlayerMailCommonColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerMailCommonData
	m_changed bool
}
func (this *dbPlayerMailCommonColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerMailCommonData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerMailCommon{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerMailCommonData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerMailCommonColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerMailCommonColumn)Get( )(v *dbPlayerMailCommonData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerMailCommonColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerMailCommonData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerMailCommonColumn)Set(v dbPlayerMailCommonData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerMailCommonColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerMailCommonData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerMailCommonColumn)GetCurrId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerMailCommonColumn.GetCurrId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.CurrId
	return
}
func (this *dbPlayerMailCommonColumn)SetCurrId(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerMailCommonColumn.SetCurrId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.CurrId = v
	this.m_changed = true
	return
}
func (this *dbPlayerMailCommonColumn)IncbyCurrId(v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerMailCommonColumn.IncbyCurrId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.CurrId += v
	this.m_changed = true
	return this.m_data.CurrId
}
func (this *dbPlayerMailCommonColumn)GetLastSendPlayerMailTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerMailCommonColumn.GetLastSendPlayerMailTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LastSendPlayerMailTime
	return
}
func (this *dbPlayerMailCommonColumn)SetLastSendPlayerMailTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerMailCommonColumn.SetLastSendPlayerMailTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LastSendPlayerMailTime = v
	this.m_changed = true
	return
}
type dbPlayerMailColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerMailData
	m_changed bool
}
func (this *dbPlayerMailColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerMailList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerMailData{}
		d.from_pb(v)
		this.m_data[int32(d.Id)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerMailColumn)save( )(data []byte,err error){
	pb := &db.PlayerMailList{}
	pb.List=make([]*db.PlayerMail,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerMailColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerMailColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerMailColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerMailColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerMailColumn)GetAll()(list []dbPlayerMailData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerMailColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerMailData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerMailColumn)Get(id int32)(v *dbPlayerMailData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerMailColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerMailData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerMailColumn)Set(v dbPlayerMailData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerMailColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Id)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerMailColumn)Add(v *dbPlayerMailData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerMailColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Id)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	d:=&dbPlayerMailData{}
	v.clone_to(d)
	this.m_data[int32(v.Id)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerMailColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerMailColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerMailColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerMailColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerMailData)
	this.m_changed = true
	return
}
func (this *dbPlayerMailColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerMailColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerMailColumn)GetType(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerMailColumn.GetType")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = int32(d.Type)
	return v,true
}
func (this *dbPlayerMailColumn)SetType(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerMailColumn.SetType")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Type = int8(v)
	this.m_changed = true
	return true
}
func (this *dbPlayerMailColumn)GetTitle(id int32)(v string ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerMailColumn.GetTitle")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Title
	return v,true
}
func (this *dbPlayerMailColumn)SetTitle(id int32,v string)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerMailColumn.SetTitle")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Title = v
	this.m_changed = true
	return true
}
func (this *dbPlayerMailColumn)GetContent(id int32)(v string ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerMailColumn.GetContent")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Content
	return v,true
}
func (this *dbPlayerMailColumn)SetContent(id int32,v string)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerMailColumn.SetContent")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Content = v
	this.m_changed = true
	return true
}
func (this *dbPlayerMailColumn)GetSendUnix(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerMailColumn.GetSendUnix")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.SendUnix
	return v,true
}
func (this *dbPlayerMailColumn)SetSendUnix(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerMailColumn.SetSendUnix")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.SendUnix = v
	this.m_changed = true
	return true
}
func (this *dbPlayerMailColumn)GetAttachItemIds(id int32)(v []int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerMailColumn.GetAttachItemIds")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = make([]int32, len(d.AttachItemIds))
	for _ii, _vv := range d.AttachItemIds {
		v[_ii]=_vv
	}
	return v,true
}
func (this *dbPlayerMailColumn)SetAttachItemIds(id int32,v []int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerMailColumn.SetAttachItemIds")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.AttachItemIds = make([]int32, len(v))
	for _ii, _vv := range v {
		d.AttachItemIds[_ii]=_vv
	}
	this.m_changed = true
	return true
}
func (this *dbPlayerMailColumn)GetAttachItemNums(id int32)(v []int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerMailColumn.GetAttachItemNums")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = make([]int32, len(d.AttachItemNums))
	for _ii, _vv := range d.AttachItemNums {
		v[_ii]=_vv
	}
	return v,true
}
func (this *dbPlayerMailColumn)SetAttachItemNums(id int32,v []int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerMailColumn.SetAttachItemNums")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.AttachItemNums = make([]int32, len(v))
	for _ii, _vv := range v {
		d.AttachItemNums[_ii]=_vv
	}
	this.m_changed = true
	return true
}
func (this *dbPlayerMailColumn)GetIsRead(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerMailColumn.GetIsRead")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.IsRead
	return v,true
}
func (this *dbPlayerMailColumn)SetIsRead(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerMailColumn.SetIsRead")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.IsRead = v
	this.m_changed = true
	return true
}
func (this *dbPlayerMailColumn)GetIsGetAttached(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerMailColumn.GetIsGetAttached")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.IsGetAttached
	return v,true
}
func (this *dbPlayerMailColumn)SetIsGetAttached(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerMailColumn.SetIsGetAttached")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.IsGetAttached = v
	this.m_changed = true
	return true
}
func (this *dbPlayerMailColumn)GetSenderId(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerMailColumn.GetSenderId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.SenderId
	return v,true
}
func (this *dbPlayerMailColumn)SetSenderId(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerMailColumn.SetSenderId")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.SenderId = v
	this.m_changed = true
	return true
}
func (this *dbPlayerMailColumn)GetSenderName(id int32)(v string ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerMailColumn.GetSenderName")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.SenderName
	return v,true
}
func (this *dbPlayerMailColumn)SetSenderName(id int32,v string)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerMailColumn.SetSenderName")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.SenderName = v
	this.m_changed = true
	return true
}
func (this *dbPlayerMailColumn)GetSubtype(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerMailColumn.GetSubtype")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Subtype
	return v,true
}
func (this *dbPlayerMailColumn)SetSubtype(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerMailColumn.SetSubtype")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Subtype = v
	this.m_changed = true
	return true
}
func (this *dbPlayerMailColumn)GetExtraValue(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerMailColumn.GetExtraValue")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.ExtraValue
	return v,true
}
func (this *dbPlayerMailColumn)SetExtraValue(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerMailColumn.SetExtraValue")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.ExtraValue = v
	this.m_changed = true
	return true
}
type dbPlayerBattleSaveColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerBattleSaveData
	m_changed bool
}
func (this *dbPlayerBattleSaveColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerBattleSaveList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerBattleSaveData{}
		d.from_pb(v)
		this.m_data[int32(d.Id)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerBattleSaveColumn)save( )(data []byte,err error){
	pb := &db.PlayerBattleSaveList{}
	pb.List=make([]*db.PlayerBattleSave,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerBattleSaveColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerBattleSaveColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerBattleSaveColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerBattleSaveColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerBattleSaveColumn)GetAll()(list []dbPlayerBattleSaveData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerBattleSaveColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerBattleSaveData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerBattleSaveColumn)Get(id int32)(v *dbPlayerBattleSaveData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerBattleSaveColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerBattleSaveData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerBattleSaveColumn)Set(v dbPlayerBattleSaveData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerBattleSaveColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Id)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerBattleSaveColumn)Add(v *dbPlayerBattleSaveData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerBattleSaveColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Id)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	d:=&dbPlayerBattleSaveData{}
	v.clone_to(d)
	this.m_data[int32(v.Id)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerBattleSaveColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerBattleSaveColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerBattleSaveColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerBattleSaveColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerBattleSaveData)
	this.m_changed = true
	return
}
func (this *dbPlayerBattleSaveColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerBattleSaveColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerBattleSaveColumn)GetSide(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerBattleSaveColumn.GetSide")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Side
	return v,true
}
func (this *dbPlayerBattleSaveColumn)SetSide(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerBattleSaveColumn.SetSide")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Side = v
	this.m_changed = true
	return true
}
func (this *dbPlayerBattleSaveColumn)GetSaveTime(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerBattleSaveColumn.GetSaveTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.SaveTime
	return v,true
}
func (this *dbPlayerBattleSaveColumn)SetSaveTime(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerBattleSaveColumn.SetSaveTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.SaveTime = v
	this.m_changed = true
	return true
}
type dbPlayerTalentColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerTalentData
	m_changed bool
}
func (this *dbPlayerTalentColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerTalentList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerTalentData{}
		d.from_pb(v)
		this.m_data[int32(d.Id)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerTalentColumn)save( )(data []byte,err error){
	pb := &db.PlayerTalentList{}
	pb.List=make([]*db.PlayerTalent,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerTalentColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTalentColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerTalentColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTalentColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerTalentColumn)GetAll()(list []dbPlayerTalentData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTalentColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerTalentData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerTalentColumn)Get(id int32)(v *dbPlayerTalentData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTalentColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerTalentData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerTalentColumn)Set(v dbPlayerTalentData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerTalentColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Id)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerTalentColumn)Add(v *dbPlayerTalentData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerTalentColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Id)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	d:=&dbPlayerTalentData{}
	v.clone_to(d)
	this.m_data[int32(v.Id)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerTalentColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerTalentColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerTalentColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerTalentColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerTalentData)
	this.m_changed = true
	return
}
func (this *dbPlayerTalentColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTalentColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerTalentColumn)GetLevel(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTalentColumn.GetLevel")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Level
	return v,true
}
func (this *dbPlayerTalentColumn)SetLevel(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerTalentColumn.SetLevel")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Level = v
	this.m_changed = true
	return true
}
type dbPlayerTowerCommonColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerTowerCommonData
	m_changed bool
}
func (this *dbPlayerTowerCommonColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerTowerCommonData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerTowerCommon{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerTowerCommonData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerTowerCommonColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerTowerCommonColumn)Get( )(v *dbPlayerTowerCommonData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTowerCommonColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerTowerCommonData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerTowerCommonColumn)Set(v dbPlayerTowerCommonData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerTowerCommonColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerTowerCommonData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerTowerCommonColumn)GetCurrId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTowerCommonColumn.GetCurrId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.CurrId
	return
}
func (this *dbPlayerTowerCommonColumn)SetCurrId(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerTowerCommonColumn.SetCurrId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.CurrId = v
	this.m_changed = true
	return
}
func (this *dbPlayerTowerCommonColumn)GetKeys( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTowerCommonColumn.GetKeys")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.Keys
	return
}
func (this *dbPlayerTowerCommonColumn)SetKeys(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerTowerCommonColumn.SetKeys")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.Keys = v
	this.m_changed = true
	return
}
func (this *dbPlayerTowerCommonColumn)GetLastGetNewKeyTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTowerCommonColumn.GetLastGetNewKeyTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LastGetNewKeyTime
	return
}
func (this *dbPlayerTowerCommonColumn)SetLastGetNewKeyTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerTowerCommonColumn.SetLastGetNewKeyTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LastGetNewKeyTime = v
	this.m_changed = true
	return
}
func (this *dbPlayerTowerCommonColumn)GetRankSerialId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTowerCommonColumn.GetRankSerialId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.RankSerialId
	return
}
func (this *dbPlayerTowerCommonColumn)SetRankSerialId(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerTowerCommonColumn.SetRankSerialId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.RankSerialId = v
	this.m_changed = true
	return
}
func (this *dbPlayerTowerCommonColumn)GetPassTowerTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTowerCommonColumn.GetPassTowerTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.PassTowerTime
	return
}
func (this *dbPlayerTowerCommonColumn)SetPassTowerTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerTowerCommonColumn.SetPassTowerTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.PassTowerTime = v
	this.m_changed = true
	return
}
type dbPlayerTowerColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerTowerData
	m_changed bool
}
func (this *dbPlayerTowerColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerTowerList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerTowerData{}
		d.from_pb(v)
		this.m_data[int32(d.Id)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerTowerColumn)save( )(data []byte,err error){
	pb := &db.PlayerTowerList{}
	pb.List=make([]*db.PlayerTower,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerTowerColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTowerColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerTowerColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTowerColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerTowerColumn)GetAll()(list []dbPlayerTowerData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTowerColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerTowerData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerTowerColumn)Get(id int32)(v *dbPlayerTowerData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTowerColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerTowerData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerTowerColumn)Set(v dbPlayerTowerData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerTowerColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Id)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerTowerColumn)Add(v *dbPlayerTowerData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerTowerColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Id)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	d:=&dbPlayerTowerData{}
	v.clone_to(d)
	this.m_data[int32(v.Id)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerTowerColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerTowerColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerTowerColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerTowerColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerTowerData)
	this.m_changed = true
	return
}
func (this *dbPlayerTowerColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTowerColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
type dbPlayerDrawColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerDrawData
	m_changed bool
}
func (this *dbPlayerDrawColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerDrawList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerDrawData{}
		d.from_pb(v)
		this.m_data[int32(d.Type)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerDrawColumn)save( )(data []byte,err error){
	pb := &db.PlayerDrawList{}
	pb.List=make([]*db.PlayerDraw,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerDrawColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerDrawColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerDrawColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerDrawColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerDrawColumn)GetAll()(list []dbPlayerDrawData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerDrawColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerDrawData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerDrawColumn)Get(id int32)(v *dbPlayerDrawData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerDrawColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerDrawData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerDrawColumn)Set(v dbPlayerDrawData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerDrawColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Type)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Type)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerDrawColumn)Add(v *dbPlayerDrawData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerDrawColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Type)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Type)
		return false
	}
	d:=&dbPlayerDrawData{}
	v.clone_to(d)
	this.m_data[int32(v.Type)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerDrawColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerDrawColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerDrawColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerDrawColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerDrawData)
	this.m_changed = true
	return
}
func (this *dbPlayerDrawColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerDrawColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerDrawColumn)GetLastDrawTime(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerDrawColumn.GetLastDrawTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.LastDrawTime
	return v,true
}
func (this *dbPlayerDrawColumn)SetLastDrawTime(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerDrawColumn.SetLastDrawTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.LastDrawTime = v
	this.m_changed = true
	return true
}
func (this *dbPlayerDrawColumn)GetNum(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerDrawColumn.GetNum")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Num
	return v,true
}
func (this *dbPlayerDrawColumn)SetNum(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerDrawColumn.SetNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Num = v
	this.m_changed = true
	return true
}
type dbPlayerGoldHandColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerGoldHandData
	m_changed bool
}
func (this *dbPlayerGoldHandColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerGoldHandData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerGoldHand{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerGoldHandData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerGoldHandColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerGoldHandColumn)Get( )(v *dbPlayerGoldHandData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerGoldHandColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerGoldHandData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerGoldHandColumn)Set(v dbPlayerGoldHandData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerGoldHandColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerGoldHandData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerGoldHandColumn)GetLastRefreshTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerGoldHandColumn.GetLastRefreshTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LastRefreshTime
	return
}
func (this *dbPlayerGoldHandColumn)SetLastRefreshTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerGoldHandColumn.SetLastRefreshTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LastRefreshTime = v
	this.m_changed = true
	return
}
func (this *dbPlayerGoldHandColumn)GetLeftNum( )(v []int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerGoldHandColumn.GetLeftNum")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = make([]int32, len(this.m_data.LeftNum))
	for _ii, _vv := range this.m_data.LeftNum {
		v[_ii]=_vv
	}
	return
}
func (this *dbPlayerGoldHandColumn)SetLeftNum(v []int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerGoldHandColumn.SetLeftNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LeftNum = make([]int32, len(v))
	for _ii, _vv := range v {
		this.m_data.LeftNum[_ii]=_vv
	}
	this.m_changed = true
	return
}
type dbPlayerShopColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerShopData
	m_changed bool
}
func (this *dbPlayerShopColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerShopList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerShopData{}
		d.from_pb(v)
		this.m_data[int32(d.Id)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerShopColumn)save( )(data []byte,err error){
	pb := &db.PlayerShopList{}
	pb.List=make([]*db.PlayerShop,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerShopColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerShopColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerShopColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerShopColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerShopColumn)GetAll()(list []dbPlayerShopData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerShopColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerShopData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerShopColumn)Get(id int32)(v *dbPlayerShopData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerShopColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerShopData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerShopColumn)Set(v dbPlayerShopData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerShopColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Id)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerShopColumn)Add(v *dbPlayerShopData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerShopColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Id)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	d:=&dbPlayerShopData{}
	v.clone_to(d)
	this.m_data[int32(v.Id)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerShopColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerShopColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerShopColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerShopColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerShopData)
	this.m_changed = true
	return
}
func (this *dbPlayerShopColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerShopColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerShopColumn)GetLastFreeRefreshTime(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerShopColumn.GetLastFreeRefreshTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.LastFreeRefreshTime
	return v,true
}
func (this *dbPlayerShopColumn)SetLastFreeRefreshTime(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerShopColumn.SetLastFreeRefreshTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.LastFreeRefreshTime = v
	this.m_changed = true
	return true
}
func (this *dbPlayerShopColumn)GetLastAutoRefreshTime(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerShopColumn.GetLastAutoRefreshTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.LastAutoRefreshTime
	return v,true
}
func (this *dbPlayerShopColumn)SetLastAutoRefreshTime(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerShopColumn.SetLastAutoRefreshTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.LastAutoRefreshTime = v
	this.m_changed = true
	return true
}
func (this *dbPlayerShopColumn)GetCurrAutoId(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerShopColumn.GetCurrAutoId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.CurrAutoId
	return v,true
}
func (this *dbPlayerShopColumn)SetCurrAutoId(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerShopColumn.SetCurrAutoId")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.CurrAutoId = v
	this.m_changed = true
	return true
}
func (this *dbPlayerShopColumn)IncbyCurrAutoId(id int32,v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerShopColumn.IncbyCurrAutoId")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		d = &dbPlayerShopData{}
		this.m_data[id] = d
	}
	d.CurrAutoId +=  v
	this.m_changed = true
	return d.CurrAutoId
}
type dbPlayerShopItemColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerShopItemData
	m_changed bool
}
func (this *dbPlayerShopItemColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerShopItemList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerShopItemData{}
		d.from_pb(v)
		this.m_data[int32(d.Id)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerShopItemColumn)save( )(data []byte,err error){
	pb := &db.PlayerShopItemList{}
	pb.List=make([]*db.PlayerShopItem,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerShopItemColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerShopItemColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerShopItemColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerShopItemColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerShopItemColumn)GetAll()(list []dbPlayerShopItemData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerShopItemColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerShopItemData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerShopItemColumn)Get(id int32)(v *dbPlayerShopItemData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerShopItemColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerShopItemData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerShopItemColumn)Set(v dbPlayerShopItemData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerShopItemColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Id)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerShopItemColumn)Add(v *dbPlayerShopItemData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerShopItemColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Id)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	d:=&dbPlayerShopItemData{}
	v.clone_to(d)
	this.m_data[int32(v.Id)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerShopItemColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerShopItemColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerShopItemColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerShopItemColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerShopItemData)
	this.m_changed = true
	return
}
func (this *dbPlayerShopItemColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerShopItemColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerShopItemColumn)GetShopItemId(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerShopItemColumn.GetShopItemId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.ShopItemId
	return v,true
}
func (this *dbPlayerShopItemColumn)SetShopItemId(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerShopItemColumn.SetShopItemId")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.ShopItemId = v
	this.m_changed = true
	return true
}
func (this *dbPlayerShopItemColumn)GetLeftNum(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerShopItemColumn.GetLeftNum")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.LeftNum
	return v,true
}
func (this *dbPlayerShopItemColumn)SetLeftNum(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerShopItemColumn.SetLeftNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.LeftNum = v
	this.m_changed = true
	return true
}
func (this *dbPlayerShopItemColumn)IncbyLeftNum(id int32,v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerShopItemColumn.IncbyLeftNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		d = &dbPlayerShopItemData{}
		this.m_data[id] = d
	}
	d.LeftNum +=  v
	this.m_changed = true
	return d.LeftNum
}
func (this *dbPlayerShopItemColumn)GetShopId(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerShopItemColumn.GetShopId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.ShopId
	return v,true
}
func (this *dbPlayerShopItemColumn)SetShopId(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerShopItemColumn.SetShopId")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.ShopId = v
	this.m_changed = true
	return true
}
func (this *dbPlayerShopItemColumn)GetBuyNum(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerShopItemColumn.GetBuyNum")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.BuyNum
	return v,true
}
func (this *dbPlayerShopItemColumn)SetBuyNum(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerShopItemColumn.SetBuyNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.BuyNum = v
	this.m_changed = true
	return true
}
func (this *dbPlayerShopItemColumn)IncbyBuyNum(id int32,v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerShopItemColumn.IncbyBuyNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		d = &dbPlayerShopItemData{}
		this.m_data[id] = d
	}
	d.BuyNum +=  v
	this.m_changed = true
	return d.BuyNum
}
type dbPlayerArenaColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerArenaData
	m_changed bool
}
func (this *dbPlayerArenaColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerArenaData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerArena{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerArenaData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerArenaColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerArenaColumn)Get( )(v *dbPlayerArenaData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerArenaColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerArenaData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerArenaColumn)Set(v dbPlayerArenaData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerArenaColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerArenaData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerArenaColumn)GetRepeatedWinNum( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerArenaColumn.GetRepeatedWinNum")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.RepeatedWinNum
	return
}
func (this *dbPlayerArenaColumn)SetRepeatedWinNum(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerArenaColumn.SetRepeatedWinNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.RepeatedWinNum = v
	this.m_changed = true
	return
}
func (this *dbPlayerArenaColumn)IncbyRepeatedWinNum(v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerArenaColumn.IncbyRepeatedWinNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.RepeatedWinNum += v
	this.m_changed = true
	return this.m_data.RepeatedWinNum
}
func (this *dbPlayerArenaColumn)GetRepeatedLoseNum( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerArenaColumn.GetRepeatedLoseNum")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.RepeatedLoseNum
	return
}
func (this *dbPlayerArenaColumn)SetRepeatedLoseNum(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerArenaColumn.SetRepeatedLoseNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.RepeatedLoseNum = v
	this.m_changed = true
	return
}
func (this *dbPlayerArenaColumn)IncbyRepeatedLoseNum(v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerArenaColumn.IncbyRepeatedLoseNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.RepeatedLoseNum += v
	this.m_changed = true
	return this.m_data.RepeatedLoseNum
}
func (this *dbPlayerArenaColumn)GetScore( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerArenaColumn.GetScore")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.Score
	return
}
func (this *dbPlayerArenaColumn)SetScore(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerArenaColumn.SetScore")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.Score = v
	this.m_changed = true
	return
}
func (this *dbPlayerArenaColumn)IncbyScore(v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerArenaColumn.IncbyScore")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.Score += v
	this.m_changed = true
	return this.m_data.Score
}
func (this *dbPlayerArenaColumn)GetUpdateScoreTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerArenaColumn.GetUpdateScoreTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.UpdateScoreTime
	return
}
func (this *dbPlayerArenaColumn)SetUpdateScoreTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerArenaColumn.SetUpdateScoreTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.UpdateScoreTime = v
	this.m_changed = true
	return
}
func (this *dbPlayerArenaColumn)GetMatchedPlayerId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerArenaColumn.GetMatchedPlayerId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.MatchedPlayerId
	return
}
func (this *dbPlayerArenaColumn)SetMatchedPlayerId(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerArenaColumn.SetMatchedPlayerId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.MatchedPlayerId = v
	this.m_changed = true
	return
}
func (this *dbPlayerArenaColumn)GetHistoryTopRank( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerArenaColumn.GetHistoryTopRank")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.HistoryTopRank
	return
}
func (this *dbPlayerArenaColumn)SetHistoryTopRank(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerArenaColumn.SetHistoryTopRank")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.HistoryTopRank = v
	this.m_changed = true
	return
}
func (this *dbPlayerArenaColumn)GetFirstGetTicket( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerArenaColumn.GetFirstGetTicket")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.FirstGetTicket
	return
}
func (this *dbPlayerArenaColumn)SetFirstGetTicket(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerArenaColumn.SetFirstGetTicket")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.FirstGetTicket = v
	this.m_changed = true
	return
}
func (this *dbPlayerArenaColumn)GetLastTicketsRefreshTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerArenaColumn.GetLastTicketsRefreshTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LastTicketsRefreshTime
	return
}
func (this *dbPlayerArenaColumn)SetLastTicketsRefreshTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerArenaColumn.SetLastTicketsRefreshTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LastTicketsRefreshTime = v
	this.m_changed = true
	return
}
func (this *dbPlayerArenaColumn)GetSerialId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerArenaColumn.GetSerialId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.SerialId
	return
}
func (this *dbPlayerArenaColumn)SetSerialId(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerArenaColumn.SetSerialId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.SerialId = v
	this.m_changed = true
	return
}
type dbPlayerEquipColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerEquipData
	m_changed bool
}
func (this *dbPlayerEquipColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerEquipData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerEquip{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerEquipData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerEquipColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerEquipColumn)Get( )(v *dbPlayerEquipData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerEquipColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerEquipData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerEquipColumn)Set(v dbPlayerEquipData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerEquipColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerEquipData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerEquipColumn)GetTmpSaveLeftSlotRoleId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerEquipColumn.GetTmpSaveLeftSlotRoleId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.TmpSaveLeftSlotRoleId
	return
}
func (this *dbPlayerEquipColumn)SetTmpSaveLeftSlotRoleId(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerEquipColumn.SetTmpSaveLeftSlotRoleId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.TmpSaveLeftSlotRoleId = v
	this.m_changed = true
	return
}
func (this *dbPlayerEquipColumn)GetTmpLeftSlotItemId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerEquipColumn.GetTmpLeftSlotItemId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.TmpLeftSlotItemId
	return
}
func (this *dbPlayerEquipColumn)SetTmpLeftSlotItemId(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerEquipColumn.SetTmpLeftSlotItemId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.TmpLeftSlotItemId = v
	this.m_changed = true
	return
}
type dbPlayerActiveStageCommonColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerActiveStageCommonData
	m_changed bool
}
func (this *dbPlayerActiveStageCommonColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerActiveStageCommonData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerActiveStageCommon{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerActiveStageCommonData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerActiveStageCommonColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerActiveStageCommonColumn)Get( )(v *dbPlayerActiveStageCommonData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerActiveStageCommonColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerActiveStageCommonData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerActiveStageCommonColumn)Set(v dbPlayerActiveStageCommonData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerActiveStageCommonColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerActiveStageCommonData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerActiveStageCommonColumn)GetLastRefreshTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerActiveStageCommonColumn.GetLastRefreshTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LastRefreshTime
	return
}
func (this *dbPlayerActiveStageCommonColumn)SetLastRefreshTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerActiveStageCommonColumn.SetLastRefreshTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LastRefreshTime = v
	this.m_changed = true
	return
}
func (this *dbPlayerActiveStageCommonColumn)GetGetPointsDay( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerActiveStageCommonColumn.GetGetPointsDay")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.GetPointsDay
	return
}
func (this *dbPlayerActiveStageCommonColumn)SetGetPointsDay(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerActiveStageCommonColumn.SetGetPointsDay")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.GetPointsDay = v
	this.m_changed = true
	return
}
func (this *dbPlayerActiveStageCommonColumn)IncbyGetPointsDay(v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerActiveStageCommonColumn.IncbyGetPointsDay")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.GetPointsDay += v
	this.m_changed = true
	return this.m_data.GetPointsDay
}
func (this *dbPlayerActiveStageCommonColumn)GetWithdrawPoints( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerActiveStageCommonColumn.GetWithdrawPoints")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.WithdrawPoints
	return
}
func (this *dbPlayerActiveStageCommonColumn)SetWithdrawPoints(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerActiveStageCommonColumn.SetWithdrawPoints")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.WithdrawPoints = v
	this.m_changed = true
	return
}
func (this *dbPlayerActiveStageCommonColumn)IncbyWithdrawPoints(v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerActiveStageCommonColumn.IncbyWithdrawPoints")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.WithdrawPoints += v
	this.m_changed = true
	return this.m_data.WithdrawPoints
}
type dbPlayerActiveStageColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerActiveStageData
	m_changed bool
}
func (this *dbPlayerActiveStageColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerActiveStageList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerActiveStageData{}
		d.from_pb(v)
		this.m_data[int32(d.Type)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerActiveStageColumn)save( )(data []byte,err error){
	pb := &db.PlayerActiveStageList{}
	pb.List=make([]*db.PlayerActiveStage,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerActiveStageColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerActiveStageColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerActiveStageColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerActiveStageColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerActiveStageColumn)GetAll()(list []dbPlayerActiveStageData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerActiveStageColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerActiveStageData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerActiveStageColumn)Get(id int32)(v *dbPlayerActiveStageData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerActiveStageColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerActiveStageData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerActiveStageColumn)Set(v dbPlayerActiveStageData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerActiveStageColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Type)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Type)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerActiveStageColumn)Add(v *dbPlayerActiveStageData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerActiveStageColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Type)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Type)
		return false
	}
	d:=&dbPlayerActiveStageData{}
	v.clone_to(d)
	this.m_data[int32(v.Type)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerActiveStageColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerActiveStageColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerActiveStageColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerActiveStageColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerActiveStageData)
	this.m_changed = true
	return
}
func (this *dbPlayerActiveStageColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerActiveStageColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerActiveStageColumn)GetCanChallengeNum(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerActiveStageColumn.GetCanChallengeNum")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.CanChallengeNum
	return v,true
}
func (this *dbPlayerActiveStageColumn)SetCanChallengeNum(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerActiveStageColumn.SetCanChallengeNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.CanChallengeNum = v
	this.m_changed = true
	return true
}
func (this *dbPlayerActiveStageColumn)IncbyCanChallengeNum(id int32,v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerActiveStageColumn.IncbyCanChallengeNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		d = &dbPlayerActiveStageData{}
		this.m_data[id] = d
	}
	d.CanChallengeNum +=  v
	this.m_changed = true
	return d.CanChallengeNum
}
func (this *dbPlayerActiveStageColumn)GetPurchasedNum(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerActiveStageColumn.GetPurchasedNum")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.PurchasedNum
	return v,true
}
func (this *dbPlayerActiveStageColumn)SetPurchasedNum(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerActiveStageColumn.SetPurchasedNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.PurchasedNum = v
	this.m_changed = true
	return true
}
func (this *dbPlayerActiveStageColumn)IncbyPurchasedNum(id int32,v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerActiveStageColumn.IncbyPurchasedNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		d = &dbPlayerActiveStageData{}
		this.m_data[id] = d
	}
	d.PurchasedNum +=  v
	this.m_changed = true
	return d.PurchasedNum
}
func (this *dbPlayerActiveStageColumn)GetBuyNum(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerActiveStageColumn.GetBuyNum")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.BuyNum
	return v,true
}
func (this *dbPlayerActiveStageColumn)SetBuyNum(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerActiveStageColumn.SetBuyNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.BuyNum = v
	this.m_changed = true
	return true
}
func (this *dbPlayerActiveStageColumn)IncbyBuyNum(id int32,v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerActiveStageColumn.IncbyBuyNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		d = &dbPlayerActiveStageData{}
		this.m_data[id] = d
	}
	d.BuyNum +=  v
	this.m_changed = true
	return d.BuyNum
}
type dbPlayerFriendCommonColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerFriendCommonData
	m_changed bool
}
func (this *dbPlayerFriendCommonColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerFriendCommonData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerFriendCommon{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerFriendCommonData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerFriendCommonColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerFriendCommonColumn)Get( )(v *dbPlayerFriendCommonData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendCommonColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerFriendCommonData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerFriendCommonColumn)Set(v dbPlayerFriendCommonData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendCommonColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerFriendCommonData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerFriendCommonColumn)GetLastRecommendTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendCommonColumn.GetLastRecommendTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LastRecommendTime
	return
}
func (this *dbPlayerFriendCommonColumn)SetLastRecommendTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendCommonColumn.SetLastRecommendTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LastRecommendTime = v
	this.m_changed = true
	return
}
func (this *dbPlayerFriendCommonColumn)GetLastBossRefreshTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendCommonColumn.GetLastBossRefreshTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LastBossRefreshTime
	return
}
func (this *dbPlayerFriendCommonColumn)SetLastBossRefreshTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendCommonColumn.SetLastBossRefreshTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LastBossRefreshTime = v
	this.m_changed = true
	return
}
func (this *dbPlayerFriendCommonColumn)GetFriendBossTableId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendCommonColumn.GetFriendBossTableId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.FriendBossTableId
	return
}
func (this *dbPlayerFriendCommonColumn)SetFriendBossTableId(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendCommonColumn.SetFriendBossTableId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.FriendBossTableId = v
	this.m_changed = true
	return
}
func (this *dbPlayerFriendCommonColumn)GetFriendBossHpPercent( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendCommonColumn.GetFriendBossHpPercent")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.FriendBossHpPercent
	return
}
func (this *dbPlayerFriendCommonColumn)SetFriendBossHpPercent(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendCommonColumn.SetFriendBossHpPercent")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.FriendBossHpPercent = v
	this.m_changed = true
	return
}
func (this *dbPlayerFriendCommonColumn)GetAttackBossPlayerList( )(v []int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendCommonColumn.GetAttackBossPlayerList")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = make([]int32, len(this.m_data.AttackBossPlayerList))
	for _ii, _vv := range this.m_data.AttackBossPlayerList {
		v[_ii]=_vv
	}
	return
}
func (this *dbPlayerFriendCommonColumn)SetAttackBossPlayerList(v []int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendCommonColumn.SetAttackBossPlayerList")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.AttackBossPlayerList = make([]int32, len(v))
	for _ii, _vv := range v {
		this.m_data.AttackBossPlayerList[_ii]=_vv
	}
	this.m_changed = true
	return
}
func (this *dbPlayerFriendCommonColumn)GetLastGetStaminaTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendCommonColumn.GetLastGetStaminaTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LastGetStaminaTime
	return
}
func (this *dbPlayerFriendCommonColumn)SetLastGetStaminaTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendCommonColumn.SetLastGetStaminaTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LastGetStaminaTime = v
	this.m_changed = true
	return
}
func (this *dbPlayerFriendCommonColumn)GetAssistRoleId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendCommonColumn.GetAssistRoleId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.AssistRoleId
	return
}
func (this *dbPlayerFriendCommonColumn)SetAssistRoleId(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendCommonColumn.SetAssistRoleId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.AssistRoleId = v
	this.m_changed = true
	return
}
func (this *dbPlayerFriendCommonColumn)GetLastGetPointsTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendCommonColumn.GetLastGetPointsTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LastGetPointsTime
	return
}
func (this *dbPlayerFriendCommonColumn)SetLastGetPointsTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendCommonColumn.SetLastGetPointsTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LastGetPointsTime = v
	this.m_changed = true
	return
}
func (this *dbPlayerFriendCommonColumn)GetGetPointsDay( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendCommonColumn.GetGetPointsDay")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.GetPointsDay
	return
}
func (this *dbPlayerFriendCommonColumn)SetGetPointsDay(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendCommonColumn.SetGetPointsDay")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.GetPointsDay = v
	this.m_changed = true
	return
}
func (this *dbPlayerFriendCommonColumn)IncbyGetPointsDay(v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendCommonColumn.IncbyGetPointsDay")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.GetPointsDay += v
	this.m_changed = true
	return this.m_data.GetPointsDay
}
func (this *dbPlayerFriendCommonColumn)GetSearchedBossNum( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendCommonColumn.GetSearchedBossNum")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.SearchedBossNum
	return
}
func (this *dbPlayerFriendCommonColumn)SetSearchedBossNum(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendCommonColumn.SetSearchedBossNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.SearchedBossNum = v
	this.m_changed = true
	return
}
func (this *dbPlayerFriendCommonColumn)IncbySearchedBossNum(v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendCommonColumn.IncbySearchedBossNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.SearchedBossNum += v
	this.m_changed = true
	return this.m_data.SearchedBossNum
}
func (this *dbPlayerFriendCommonColumn)GetLastSearchBossNumRefreshTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendCommonColumn.GetLastSearchBossNumRefreshTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LastSearchBossNumRefreshTime
	return
}
func (this *dbPlayerFriendCommonColumn)SetLastSearchBossNumRefreshTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendCommonColumn.SetLastSearchBossNumRefreshTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LastSearchBossNumRefreshTime = v
	this.m_changed = true
	return
}
type dbPlayerFriendColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerFriendData
	m_changed bool
}
func (this *dbPlayerFriendColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerFriendList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerFriendData{}
		d.from_pb(v)
		this.m_data[int32(d.PlayerId)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerFriendColumn)save( )(data []byte,err error){
	pb := &db.PlayerFriendList{}
	pb.List=make([]*db.PlayerFriend,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerFriendColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerFriendColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerFriendColumn)GetAll()(list []dbPlayerFriendData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerFriendData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerFriendColumn)Get(id int32)(v *dbPlayerFriendData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerFriendData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerFriendColumn)Set(v dbPlayerFriendData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.PlayerId)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.PlayerId)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerFriendColumn)Add(v *dbPlayerFriendData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.PlayerId)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.PlayerId)
		return false
	}
	d:=&dbPlayerFriendData{}
	v.clone_to(d)
	this.m_data[int32(v.PlayerId)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerFriendColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerFriendColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerFriendData)
	this.m_changed = true
	return
}
func (this *dbPlayerFriendColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerFriendColumn)GetLastGivePointsTime(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendColumn.GetLastGivePointsTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.LastGivePointsTime
	return v,true
}
func (this *dbPlayerFriendColumn)SetLastGivePointsTime(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendColumn.SetLastGivePointsTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.LastGivePointsTime = v
	this.m_changed = true
	return true
}
func (this *dbPlayerFriendColumn)GetGetPoints(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendColumn.GetGetPoints")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.GetPoints
	return v,true
}
func (this *dbPlayerFriendColumn)SetGetPoints(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendColumn.SetGetPoints")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.GetPoints = v
	this.m_changed = true
	return true
}
type dbPlayerFriendRecommendColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerFriendRecommendData
	m_changed bool
}
func (this *dbPlayerFriendRecommendColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerFriendRecommendList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerFriendRecommendData{}
		d.from_pb(v)
		this.m_data[int32(d.PlayerId)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerFriendRecommendColumn)save( )(data []byte,err error){
	pb := &db.PlayerFriendRecommendList{}
	pb.List=make([]*db.PlayerFriendRecommend,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerFriendRecommendColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendRecommendColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerFriendRecommendColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendRecommendColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerFriendRecommendColumn)GetAll()(list []dbPlayerFriendRecommendData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendRecommendColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerFriendRecommendData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerFriendRecommendColumn)Get(id int32)(v *dbPlayerFriendRecommendData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendRecommendColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerFriendRecommendData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerFriendRecommendColumn)Set(v dbPlayerFriendRecommendData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendRecommendColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.PlayerId)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.PlayerId)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerFriendRecommendColumn)Add(v *dbPlayerFriendRecommendData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendRecommendColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.PlayerId)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.PlayerId)
		return false
	}
	d:=&dbPlayerFriendRecommendData{}
	v.clone_to(d)
	this.m_data[int32(v.PlayerId)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerFriendRecommendColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendRecommendColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerFriendRecommendColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendRecommendColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerFriendRecommendData)
	this.m_changed = true
	return
}
func (this *dbPlayerFriendRecommendColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendRecommendColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
type dbPlayerFriendAskColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerFriendAskData
	m_changed bool
}
func (this *dbPlayerFriendAskColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerFriendAskList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerFriendAskData{}
		d.from_pb(v)
		this.m_data[int32(d.PlayerId)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerFriendAskColumn)save( )(data []byte,err error){
	pb := &db.PlayerFriendAskList{}
	pb.List=make([]*db.PlayerFriendAsk,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerFriendAskColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendAskColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerFriendAskColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendAskColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerFriendAskColumn)GetAll()(list []dbPlayerFriendAskData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendAskColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerFriendAskData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerFriendAskColumn)Get(id int32)(v *dbPlayerFriendAskData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendAskColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerFriendAskData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerFriendAskColumn)Set(v dbPlayerFriendAskData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendAskColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.PlayerId)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.PlayerId)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerFriendAskColumn)Add(v *dbPlayerFriendAskData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendAskColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.PlayerId)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.PlayerId)
		return false
	}
	d:=&dbPlayerFriendAskData{}
	v.clone_to(d)
	this.m_data[int32(v.PlayerId)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerFriendAskColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendAskColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerFriendAskColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendAskColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerFriendAskData)
	this.m_changed = true
	return
}
func (this *dbPlayerFriendAskColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendAskColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
type dbPlayerFriendBossColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerFriendBossData
	m_changed bool
}
func (this *dbPlayerFriendBossColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerFriendBossList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerFriendBossData{}
		d.from_pb(v)
		this.m_data[int32(d.MonsterPos)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerFriendBossColumn)save( )(data []byte,err error){
	pb := &db.PlayerFriendBossList{}
	pb.List=make([]*db.PlayerFriendBoss,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerFriendBossColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendBossColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerFriendBossColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendBossColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerFriendBossColumn)GetAll()(list []dbPlayerFriendBossData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendBossColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerFriendBossData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerFriendBossColumn)Get(id int32)(v *dbPlayerFriendBossData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendBossColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerFriendBossData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerFriendBossColumn)Set(v dbPlayerFriendBossData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendBossColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.MonsterPos)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.MonsterPos)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerFriendBossColumn)Add(v *dbPlayerFriendBossData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendBossColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.MonsterPos)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.MonsterPos)
		return false
	}
	d:=&dbPlayerFriendBossData{}
	v.clone_to(d)
	this.m_data[int32(v.MonsterPos)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerFriendBossColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendBossColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerFriendBossColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendBossColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerFriendBossData)
	this.m_changed = true
	return
}
func (this *dbPlayerFriendBossColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendBossColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerFriendBossColumn)GetMonsterId(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendBossColumn.GetMonsterId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.MonsterId
	return v,true
}
func (this *dbPlayerFriendBossColumn)SetMonsterId(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendBossColumn.SetMonsterId")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.MonsterId = v
	this.m_changed = true
	return true
}
func (this *dbPlayerFriendBossColumn)GetMonsterHp(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendBossColumn.GetMonsterHp")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.MonsterHp
	return v,true
}
func (this *dbPlayerFriendBossColumn)SetMonsterHp(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendBossColumn.SetMonsterHp")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.MonsterHp = v
	this.m_changed = true
	return true
}
func (this *dbPlayerFriendBossColumn)GetMonsterMaxHp(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendBossColumn.GetMonsterMaxHp")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.MonsterMaxHp
	return v,true
}
func (this *dbPlayerFriendBossColumn)SetMonsterMaxHp(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendBossColumn.SetMonsterMaxHp")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.MonsterMaxHp = v
	this.m_changed = true
	return true
}
type dbPlayerTaskCommonColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerTaskCommonData
	m_changed bool
}
func (this *dbPlayerTaskCommonColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerTaskCommonData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerTaskCommon{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerTaskCommonData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerTaskCommonColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerTaskCommonColumn)Get( )(v *dbPlayerTaskCommonData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTaskCommonColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerTaskCommonData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerTaskCommonColumn)Set(v dbPlayerTaskCommonData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerTaskCommonColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerTaskCommonData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerTaskCommonColumn)GetLastRefreshTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTaskCommonColumn.GetLastRefreshTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LastRefreshTime
	return
}
func (this *dbPlayerTaskCommonColumn)SetLastRefreshTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerTaskCommonColumn.SetLastRefreshTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LastRefreshTime = v
	this.m_changed = true
	return
}
type dbPlayerTaskColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerTaskData
	m_changed bool
}
func (this *dbPlayerTaskColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerTaskList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerTaskData{}
		d.from_pb(v)
		this.m_data[int32(d.Id)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerTaskColumn)save( )(data []byte,err error){
	pb := &db.PlayerTaskList{}
	pb.List=make([]*db.PlayerTask,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerTaskColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTaskColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerTaskColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTaskColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerTaskColumn)GetAll()(list []dbPlayerTaskData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTaskColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerTaskData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerTaskColumn)Get(id int32)(v *dbPlayerTaskData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTaskColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerTaskData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerTaskColumn)Set(v dbPlayerTaskData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerTaskColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Id)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerTaskColumn)Add(v *dbPlayerTaskData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerTaskColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Id)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	d:=&dbPlayerTaskData{}
	v.clone_to(d)
	this.m_data[int32(v.Id)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerTaskColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerTaskColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerTaskColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerTaskColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerTaskData)
	this.m_changed = true
	return
}
func (this *dbPlayerTaskColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTaskColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerTaskColumn)GetValue(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTaskColumn.GetValue")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Value
	return v,true
}
func (this *dbPlayerTaskColumn)SetValue(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerTaskColumn.SetValue")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Value = v
	this.m_changed = true
	return true
}
func (this *dbPlayerTaskColumn)IncbyValue(id int32,v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerTaskColumn.IncbyValue")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		d = &dbPlayerTaskData{}
		this.m_data[id] = d
	}
	d.Value +=  v
	this.m_changed = true
	return d.Value
}
func (this *dbPlayerTaskColumn)GetState(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTaskColumn.GetState")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.State
	return v,true
}
func (this *dbPlayerTaskColumn)SetState(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerTaskColumn.SetState")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.State = v
	this.m_changed = true
	return true
}
func (this *dbPlayerTaskColumn)GetParam(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerTaskColumn.GetParam")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Param
	return v,true
}
func (this *dbPlayerTaskColumn)SetParam(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerTaskColumn.SetParam")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Param = v
	this.m_changed = true
	return true
}
type dbPlayerFinishedTaskColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerFinishedTaskData
	m_changed bool
}
func (this *dbPlayerFinishedTaskColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerFinishedTaskList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerFinishedTaskData{}
		d.from_pb(v)
		this.m_data[int32(d.Id)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerFinishedTaskColumn)save( )(data []byte,err error){
	pb := &db.PlayerFinishedTaskList{}
	pb.List=make([]*db.PlayerFinishedTask,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerFinishedTaskColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFinishedTaskColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerFinishedTaskColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFinishedTaskColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerFinishedTaskColumn)GetAll()(list []dbPlayerFinishedTaskData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFinishedTaskColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerFinishedTaskData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerFinishedTaskColumn)Get(id int32)(v *dbPlayerFinishedTaskData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFinishedTaskColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerFinishedTaskData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerFinishedTaskColumn)Set(v dbPlayerFinishedTaskData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFinishedTaskColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Id)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerFinishedTaskColumn)Add(v *dbPlayerFinishedTaskData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFinishedTaskColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Id)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	d:=&dbPlayerFinishedTaskData{}
	v.clone_to(d)
	this.m_data[int32(v.Id)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerFinishedTaskColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerFinishedTaskColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerFinishedTaskColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerFinishedTaskColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerFinishedTaskData)
	this.m_changed = true
	return
}
func (this *dbPlayerFinishedTaskColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFinishedTaskColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
type dbPlayerDailyTaskAllDailyColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerDailyTaskAllDailyData
	m_changed bool
}
func (this *dbPlayerDailyTaskAllDailyColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerDailyTaskAllDailyList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerDailyTaskAllDailyData{}
		d.from_pb(v)
		this.m_data[int32(d.CompleteTaskId)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerDailyTaskAllDailyColumn)save( )(data []byte,err error){
	pb := &db.PlayerDailyTaskAllDailyList{}
	pb.List=make([]*db.PlayerDailyTaskAllDaily,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerDailyTaskAllDailyColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerDailyTaskAllDailyColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerDailyTaskAllDailyColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerDailyTaskAllDailyColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerDailyTaskAllDailyColumn)GetAll()(list []dbPlayerDailyTaskAllDailyData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerDailyTaskAllDailyColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerDailyTaskAllDailyData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerDailyTaskAllDailyColumn)Get(id int32)(v *dbPlayerDailyTaskAllDailyData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerDailyTaskAllDailyColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerDailyTaskAllDailyData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerDailyTaskAllDailyColumn)Set(v dbPlayerDailyTaskAllDailyData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerDailyTaskAllDailyColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.CompleteTaskId)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.CompleteTaskId)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerDailyTaskAllDailyColumn)Add(v *dbPlayerDailyTaskAllDailyData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerDailyTaskAllDailyColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.CompleteTaskId)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.CompleteTaskId)
		return false
	}
	d:=&dbPlayerDailyTaskAllDailyData{}
	v.clone_to(d)
	this.m_data[int32(v.CompleteTaskId)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerDailyTaskAllDailyColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerDailyTaskAllDailyColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerDailyTaskAllDailyColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerDailyTaskAllDailyColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerDailyTaskAllDailyData)
	this.m_changed = true
	return
}
func (this *dbPlayerDailyTaskAllDailyColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerDailyTaskAllDailyColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
type dbPlayerExploreCommonColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerExploreCommonData
	m_changed bool
}
func (this *dbPlayerExploreCommonColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerExploreCommonData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerExploreCommon{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerExploreCommonData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerExploreCommonColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerExploreCommonColumn)Get( )(v *dbPlayerExploreCommonData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreCommonColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerExploreCommonData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerExploreCommonColumn)Set(v dbPlayerExploreCommonData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreCommonColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerExploreCommonData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerExploreCommonColumn)GetLastRefreshTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreCommonColumn.GetLastRefreshTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LastRefreshTime
	return
}
func (this *dbPlayerExploreCommonColumn)SetLastRefreshTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreCommonColumn.SetLastRefreshTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LastRefreshTime = v
	this.m_changed = true
	return
}
func (this *dbPlayerExploreCommonColumn)GetCurrentId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreCommonColumn.GetCurrentId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.CurrentId
	return
}
func (this *dbPlayerExploreCommonColumn)SetCurrentId(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreCommonColumn.SetCurrentId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.CurrentId = v
	this.m_changed = true
	return
}
func (this *dbPlayerExploreCommonColumn)IncbyCurrentId(v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreCommonColumn.IncbyCurrentId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.CurrentId += v
	this.m_changed = true
	return this.m_data.CurrentId
}
type dbPlayerExploreColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerExploreData
	m_changed bool
}
func (this *dbPlayerExploreColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerExploreList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerExploreData{}
		d.from_pb(v)
		this.m_data[int32(d.Id)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerExploreColumn)save( )(data []byte,err error){
	pb := &db.PlayerExploreList{}
	pb.List=make([]*db.PlayerExplore,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerExploreColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerExploreColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerExploreColumn)GetAll()(list []dbPlayerExploreData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerExploreData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerExploreColumn)Get(id int32)(v *dbPlayerExploreData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerExploreData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerExploreColumn)Set(v dbPlayerExploreData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Id)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerExploreColumn)Add(v *dbPlayerExploreData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Id)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	d:=&dbPlayerExploreData{}
	v.clone_to(d)
	this.m_data[int32(v.Id)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerExploreColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerExploreColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerExploreData)
	this.m_changed = true
	return
}
func (this *dbPlayerExploreColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerExploreColumn)GetTaskId(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreColumn.GetTaskId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.TaskId
	return v,true
}
func (this *dbPlayerExploreColumn)SetTaskId(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreColumn.SetTaskId")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.TaskId = v
	this.m_changed = true
	return true
}
func (this *dbPlayerExploreColumn)GetState(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreColumn.GetState")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.State
	return v,true
}
func (this *dbPlayerExploreColumn)SetState(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreColumn.SetState")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.State = v
	this.m_changed = true
	return true
}
func (this *dbPlayerExploreColumn)GetRoleCampsCanSel(id int32)(v []int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreColumn.GetRoleCampsCanSel")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = make([]int32, len(d.RoleCampsCanSel))
	for _ii, _vv := range d.RoleCampsCanSel {
		v[_ii]=_vv
	}
	return v,true
}
func (this *dbPlayerExploreColumn)SetRoleCampsCanSel(id int32,v []int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreColumn.SetRoleCampsCanSel")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.RoleCampsCanSel = make([]int32, len(v))
	for _ii, _vv := range v {
		d.RoleCampsCanSel[_ii]=_vv
	}
	this.m_changed = true
	return true
}
func (this *dbPlayerExploreColumn)GetRoleTypesCanSel(id int32)(v []int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreColumn.GetRoleTypesCanSel")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = make([]int32, len(d.RoleTypesCanSel))
	for _ii, _vv := range d.RoleTypesCanSel {
		v[_ii]=_vv
	}
	return v,true
}
func (this *dbPlayerExploreColumn)SetRoleTypesCanSel(id int32,v []int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreColumn.SetRoleTypesCanSel")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.RoleTypesCanSel = make([]int32, len(v))
	for _ii, _vv := range v {
		d.RoleTypesCanSel[_ii]=_vv
	}
	this.m_changed = true
	return true
}
func (this *dbPlayerExploreColumn)GetRoleId4TaskTitle(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreColumn.GetRoleId4TaskTitle")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.RoleId4TaskTitle
	return v,true
}
func (this *dbPlayerExploreColumn)SetRoleId4TaskTitle(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreColumn.SetRoleId4TaskTitle")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.RoleId4TaskTitle = v
	this.m_changed = true
	return true
}
func (this *dbPlayerExploreColumn)GetNameId4TaskTitle(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreColumn.GetNameId4TaskTitle")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.NameId4TaskTitle
	return v,true
}
func (this *dbPlayerExploreColumn)SetNameId4TaskTitle(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreColumn.SetNameId4TaskTitle")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.NameId4TaskTitle = v
	this.m_changed = true
	return true
}
func (this *dbPlayerExploreColumn)GetStartTime(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreColumn.GetStartTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.StartTime
	return v,true
}
func (this *dbPlayerExploreColumn)SetStartTime(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreColumn.SetStartTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.StartTime = v
	this.m_changed = true
	return true
}
func (this *dbPlayerExploreColumn)GetRoleIds(id int32)(v []int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreColumn.GetRoleIds")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = make([]int32, len(d.RoleIds))
	for _ii, _vv := range d.RoleIds {
		v[_ii]=_vv
	}
	return v,true
}
func (this *dbPlayerExploreColumn)SetRoleIds(id int32,v []int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreColumn.SetRoleIds")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.RoleIds = make([]int32, len(v))
	for _ii, _vv := range v {
		d.RoleIds[_ii]=_vv
	}
	this.m_changed = true
	return true
}
func (this *dbPlayerExploreColumn)GetIsLock(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreColumn.GetIsLock")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.IsLock
	return v,true
}
func (this *dbPlayerExploreColumn)SetIsLock(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreColumn.SetIsLock")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.IsLock = v
	this.m_changed = true
	return true
}
func (this *dbPlayerExploreColumn)GetRandomRewards(id int32)(v []int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreColumn.GetRandomRewards")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = make([]int32, len(d.RandomRewards))
	for _ii, _vv := range d.RandomRewards {
		v[_ii]=_vv
	}
	return v,true
}
func (this *dbPlayerExploreColumn)SetRandomRewards(id int32,v []int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreColumn.SetRandomRewards")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.RandomRewards = make([]int32, len(v))
	for _ii, _vv := range v {
		d.RandomRewards[_ii]=_vv
	}
	this.m_changed = true
	return true
}
func (this *dbPlayerExploreColumn)GetRewardStageId(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreColumn.GetRewardStageId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.RewardStageId
	return v,true
}
func (this *dbPlayerExploreColumn)SetRewardStageId(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreColumn.SetRewardStageId")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.RewardStageId = v
	this.m_changed = true
	return true
}
type dbPlayerExploreStoryColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerExploreStoryData
	m_changed bool
}
func (this *dbPlayerExploreStoryColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerExploreStoryList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerExploreStoryData{}
		d.from_pb(v)
		this.m_data[int32(d.TaskId)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerExploreStoryColumn)save( )(data []byte,err error){
	pb := &db.PlayerExploreStoryList{}
	pb.List=make([]*db.PlayerExploreStory,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerExploreStoryColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreStoryColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerExploreStoryColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreStoryColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerExploreStoryColumn)GetAll()(list []dbPlayerExploreStoryData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreStoryColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerExploreStoryData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerExploreStoryColumn)Get(id int32)(v *dbPlayerExploreStoryData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreStoryColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerExploreStoryData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerExploreStoryColumn)Set(v dbPlayerExploreStoryData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreStoryColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.TaskId)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.TaskId)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerExploreStoryColumn)Add(v *dbPlayerExploreStoryData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreStoryColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.TaskId)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.TaskId)
		return false
	}
	d:=&dbPlayerExploreStoryData{}
	v.clone_to(d)
	this.m_data[int32(v.TaskId)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerExploreStoryColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreStoryColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerExploreStoryColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreStoryColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerExploreStoryData)
	this.m_changed = true
	return
}
func (this *dbPlayerExploreStoryColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreStoryColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerExploreStoryColumn)GetState(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreStoryColumn.GetState")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.State
	return v,true
}
func (this *dbPlayerExploreStoryColumn)SetState(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreStoryColumn.SetState")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.State = v
	this.m_changed = true
	return true
}
func (this *dbPlayerExploreStoryColumn)GetRoleCampsCanSel(id int32)(v []int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreStoryColumn.GetRoleCampsCanSel")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = make([]int32, len(d.RoleCampsCanSel))
	for _ii, _vv := range d.RoleCampsCanSel {
		v[_ii]=_vv
	}
	return v,true
}
func (this *dbPlayerExploreStoryColumn)SetRoleCampsCanSel(id int32,v []int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreStoryColumn.SetRoleCampsCanSel")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.RoleCampsCanSel = make([]int32, len(v))
	for _ii, _vv := range v {
		d.RoleCampsCanSel[_ii]=_vv
	}
	this.m_changed = true
	return true
}
func (this *dbPlayerExploreStoryColumn)GetRoleTypesCanSel(id int32)(v []int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreStoryColumn.GetRoleTypesCanSel")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = make([]int32, len(d.RoleTypesCanSel))
	for _ii, _vv := range d.RoleTypesCanSel {
		v[_ii]=_vv
	}
	return v,true
}
func (this *dbPlayerExploreStoryColumn)SetRoleTypesCanSel(id int32,v []int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreStoryColumn.SetRoleTypesCanSel")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.RoleTypesCanSel = make([]int32, len(v))
	for _ii, _vv := range v {
		d.RoleTypesCanSel[_ii]=_vv
	}
	this.m_changed = true
	return true
}
func (this *dbPlayerExploreStoryColumn)GetStartTime(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreStoryColumn.GetStartTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.StartTime
	return v,true
}
func (this *dbPlayerExploreStoryColumn)SetStartTime(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreStoryColumn.SetStartTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.StartTime = v
	this.m_changed = true
	return true
}
func (this *dbPlayerExploreStoryColumn)GetRoleIds(id int32)(v []int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreStoryColumn.GetRoleIds")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = make([]int32, len(d.RoleIds))
	for _ii, _vv := range d.RoleIds {
		v[_ii]=_vv
	}
	return v,true
}
func (this *dbPlayerExploreStoryColumn)SetRoleIds(id int32,v []int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreStoryColumn.SetRoleIds")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.RoleIds = make([]int32, len(v))
	for _ii, _vv := range v {
		d.RoleIds[_ii]=_vv
	}
	this.m_changed = true
	return true
}
func (this *dbPlayerExploreStoryColumn)GetRandomRewards(id int32)(v []int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreStoryColumn.GetRandomRewards")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = make([]int32, len(d.RandomRewards))
	for _ii, _vv := range d.RandomRewards {
		v[_ii]=_vv
	}
	return v,true
}
func (this *dbPlayerExploreStoryColumn)SetRandomRewards(id int32,v []int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreStoryColumn.SetRandomRewards")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.RandomRewards = make([]int32, len(v))
	for _ii, _vv := range v {
		d.RandomRewards[_ii]=_vv
	}
	this.m_changed = true
	return true
}
func (this *dbPlayerExploreStoryColumn)GetRewardStageId(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExploreStoryColumn.GetRewardStageId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.RewardStageId
	return v,true
}
func (this *dbPlayerExploreStoryColumn)SetRewardStageId(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExploreStoryColumn.SetRewardStageId")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.RewardStageId = v
	this.m_changed = true
	return true
}
type dbPlayerFriendChatUnreadIdColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerFriendChatUnreadIdData
	m_changed bool
}
func (this *dbPlayerFriendChatUnreadIdColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerFriendChatUnreadIdList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerFriendChatUnreadIdData{}
		d.from_pb(v)
		this.m_data[int32(d.FriendId)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerFriendChatUnreadIdColumn)save( )(data []byte,err error){
	pb := &db.PlayerFriendChatUnreadIdList{}
	pb.List=make([]*db.PlayerFriendChatUnreadId,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerFriendChatUnreadIdColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendChatUnreadIdColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerFriendChatUnreadIdColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendChatUnreadIdColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerFriendChatUnreadIdColumn)GetAll()(list []dbPlayerFriendChatUnreadIdData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendChatUnreadIdColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerFriendChatUnreadIdData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerFriendChatUnreadIdColumn)Get(id int32)(v *dbPlayerFriendChatUnreadIdData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendChatUnreadIdColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerFriendChatUnreadIdData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerFriendChatUnreadIdColumn)Set(v dbPlayerFriendChatUnreadIdData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendChatUnreadIdColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.FriendId)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.FriendId)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerFriendChatUnreadIdColumn)Add(v *dbPlayerFriendChatUnreadIdData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendChatUnreadIdColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.FriendId)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.FriendId)
		return false
	}
	d:=&dbPlayerFriendChatUnreadIdData{}
	v.clone_to(d)
	this.m_data[int32(v.FriendId)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerFriendChatUnreadIdColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendChatUnreadIdColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerFriendChatUnreadIdColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendChatUnreadIdColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerFriendChatUnreadIdData)
	this.m_changed = true
	return
}
func (this *dbPlayerFriendChatUnreadIdColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendChatUnreadIdColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerFriendChatUnreadIdColumn)GetMessageIds(id int32)(v []int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendChatUnreadIdColumn.GetMessageIds")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = make([]int32, len(d.MessageIds))
	for _ii, _vv := range d.MessageIds {
		v[_ii]=_vv
	}
	return v,true
}
func (this *dbPlayerFriendChatUnreadIdColumn)SetMessageIds(id int32,v []int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendChatUnreadIdColumn.SetMessageIds")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.MessageIds = make([]int32, len(v))
	for _ii, _vv := range v {
		d.MessageIds[_ii]=_vv
	}
	this.m_changed = true
	return true
}
func (this *dbPlayerFriendChatUnreadIdColumn)GetCurrMessageId(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendChatUnreadIdColumn.GetCurrMessageId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.CurrMessageId
	return v,true
}
func (this *dbPlayerFriendChatUnreadIdColumn)SetCurrMessageId(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendChatUnreadIdColumn.SetCurrMessageId")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.CurrMessageId = v
	this.m_changed = true
	return true
}
func (this *dbPlayerFriendChatUnreadIdColumn)IncbyCurrMessageId(id int32,v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendChatUnreadIdColumn.IncbyCurrMessageId")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		d = &dbPlayerFriendChatUnreadIdData{}
		this.m_data[id] = d
	}
	d.CurrMessageId +=  v
	this.m_changed = true
	return d.CurrMessageId
}
type dbPlayerFriendChatUnreadMessageColumn struct{
	m_row *dbPlayerRow
	m_data map[int64]*dbPlayerFriendChatUnreadMessageData
	m_changed bool
}
func (this *dbPlayerFriendChatUnreadMessageColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerFriendChatUnreadMessageList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerFriendChatUnreadMessageData{}
		d.from_pb(v)
		this.m_data[int64(d.PlayerMessageId)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerFriendChatUnreadMessageColumn)save( )(data []byte,err error){
	pb := &db.PlayerFriendChatUnreadMessageList{}
	pb.List=make([]*db.PlayerFriendChatUnreadMessage,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerFriendChatUnreadMessageColumn)HasIndex(id int64)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendChatUnreadMessageColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerFriendChatUnreadMessageColumn)GetAllIndex()(list []int64){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendChatUnreadMessageColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int64, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerFriendChatUnreadMessageColumn)GetAll()(list []dbPlayerFriendChatUnreadMessageData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendChatUnreadMessageColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerFriendChatUnreadMessageData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerFriendChatUnreadMessageColumn)Get(id int64)(v *dbPlayerFriendChatUnreadMessageData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendChatUnreadMessageColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerFriendChatUnreadMessageData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerFriendChatUnreadMessageColumn)Set(v dbPlayerFriendChatUnreadMessageData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendChatUnreadMessageColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int64(v.PlayerMessageId)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.PlayerMessageId)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerFriendChatUnreadMessageColumn)Add(v *dbPlayerFriendChatUnreadMessageData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendChatUnreadMessageColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int64(v.PlayerMessageId)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.PlayerMessageId)
		return false
	}
	d:=&dbPlayerFriendChatUnreadMessageData{}
	v.clone_to(d)
	this.m_data[int64(v.PlayerMessageId)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerFriendChatUnreadMessageColumn)Remove(id int64){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendChatUnreadMessageColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerFriendChatUnreadMessageColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendChatUnreadMessageColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int64]*dbPlayerFriendChatUnreadMessageData)
	this.m_changed = true
	return
}
func (this *dbPlayerFriendChatUnreadMessageColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendChatUnreadMessageColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerFriendChatUnreadMessageColumn)GetMessage(id int64)(v []byte,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendChatUnreadMessageColumn.GetMessage")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = make([]byte, len(d.Message))
	for _ii, _vv := range d.Message {
		v[_ii]=_vv
	}
	return v,true
}
func (this *dbPlayerFriendChatUnreadMessageColumn)SetMessage(id int64,v []byte)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendChatUnreadMessageColumn.SetMessage")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Message = make([]byte, len(v))
	for _ii, _vv := range v {
		d.Message[_ii]=_vv
	}
	this.m_changed = true
	return true
}
func (this *dbPlayerFriendChatUnreadMessageColumn)GetSendTime(id int64)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendChatUnreadMessageColumn.GetSendTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.SendTime
	return v,true
}
func (this *dbPlayerFriendChatUnreadMessageColumn)SetSendTime(id int64,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendChatUnreadMessageColumn.SetSendTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.SendTime = v
	this.m_changed = true
	return true
}
func (this *dbPlayerFriendChatUnreadMessageColumn)GetIsRead(id int64)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFriendChatUnreadMessageColumn.GetIsRead")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.IsRead
	return v,true
}
func (this *dbPlayerFriendChatUnreadMessageColumn)SetIsRead(id int64,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFriendChatUnreadMessageColumn.SetIsRead")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.IsRead = v
	this.m_changed = true
	return true
}
type dbPlayerHeadItemColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerHeadItemData
	m_changed bool
}
func (this *dbPlayerHeadItemColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerHeadItemList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerHeadItemData{}
		d.from_pb(v)
		this.m_data[int32(d.Id)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerHeadItemColumn)save( )(data []byte,err error){
	pb := &db.PlayerHeadItemList{}
	pb.List=make([]*db.PlayerHeadItem,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerHeadItemColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerHeadItemColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerHeadItemColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerHeadItemColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerHeadItemColumn)GetAll()(list []dbPlayerHeadItemData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerHeadItemColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerHeadItemData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerHeadItemColumn)Get(id int32)(v *dbPlayerHeadItemData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerHeadItemColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerHeadItemData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerHeadItemColumn)Set(v dbPlayerHeadItemData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerHeadItemColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Id)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerHeadItemColumn)Add(v *dbPlayerHeadItemData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerHeadItemColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Id)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	d:=&dbPlayerHeadItemData{}
	v.clone_to(d)
	this.m_data[int32(v.Id)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerHeadItemColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerHeadItemColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerHeadItemColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerHeadItemColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerHeadItemData)
	this.m_changed = true
	return
}
func (this *dbPlayerHeadItemColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerHeadItemColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
type dbPlayerSuitAwardColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerSuitAwardData
	m_changed bool
}
func (this *dbPlayerSuitAwardColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerSuitAwardList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerSuitAwardData{}
		d.from_pb(v)
		this.m_data[int32(d.Id)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerSuitAwardColumn)save( )(data []byte,err error){
	pb := &db.PlayerSuitAwardList{}
	pb.List=make([]*db.PlayerSuitAward,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerSuitAwardColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerSuitAwardColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerSuitAwardColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerSuitAwardColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerSuitAwardColumn)GetAll()(list []dbPlayerSuitAwardData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerSuitAwardColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerSuitAwardData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerSuitAwardColumn)Get(id int32)(v *dbPlayerSuitAwardData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerSuitAwardColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerSuitAwardData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerSuitAwardColumn)Set(v dbPlayerSuitAwardData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerSuitAwardColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Id)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerSuitAwardColumn)Add(v *dbPlayerSuitAwardData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerSuitAwardColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Id)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	d:=&dbPlayerSuitAwardData{}
	v.clone_to(d)
	this.m_data[int32(v.Id)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerSuitAwardColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerSuitAwardColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerSuitAwardColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerSuitAwardColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerSuitAwardData)
	this.m_changed = true
	return
}
func (this *dbPlayerSuitAwardColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerSuitAwardColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerSuitAwardColumn)GetAwardTime(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerSuitAwardColumn.GetAwardTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.AwardTime
	return v,true
}
func (this *dbPlayerSuitAwardColumn)SetAwardTime(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerSuitAwardColumn.SetAwardTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.AwardTime = v
	this.m_changed = true
	return true
}
type dbPlayerChatColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerChatData
	m_changed bool
}
func (this *dbPlayerChatColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerChatList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerChatData{}
		d.from_pb(v)
		this.m_data[int32(d.Channel)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerChatColumn)save( )(data []byte,err error){
	pb := &db.PlayerChatList{}
	pb.List=make([]*db.PlayerChat,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerChatColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerChatColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerChatColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerChatColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerChatColumn)GetAll()(list []dbPlayerChatData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerChatColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerChatData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerChatColumn)Get(id int32)(v *dbPlayerChatData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerChatColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerChatData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerChatColumn)Set(v dbPlayerChatData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerChatColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Channel)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Channel)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerChatColumn)Add(v *dbPlayerChatData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerChatColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Channel)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Channel)
		return false
	}
	d:=&dbPlayerChatData{}
	v.clone_to(d)
	this.m_data[int32(v.Channel)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerChatColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerChatColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerChatColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerChatColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerChatData)
	this.m_changed = true
	return
}
func (this *dbPlayerChatColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerChatColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerChatColumn)GetLastChatTime(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerChatColumn.GetLastChatTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.LastChatTime
	return v,true
}
func (this *dbPlayerChatColumn)SetLastChatTime(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerChatColumn.SetLastChatTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.LastChatTime = v
	this.m_changed = true
	return true
}
func (this *dbPlayerChatColumn)GetLastPullTime(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerChatColumn.GetLastPullTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.LastPullTime
	return v,true
}
func (this *dbPlayerChatColumn)SetLastPullTime(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerChatColumn.SetLastPullTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.LastPullTime = v
	this.m_changed = true
	return true
}
func (this *dbPlayerChatColumn)GetLastMsgIndex(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerChatColumn.GetLastMsgIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.LastMsgIndex
	return v,true
}
func (this *dbPlayerChatColumn)SetLastMsgIndex(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerChatColumn.SetLastMsgIndex")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.LastMsgIndex = v
	this.m_changed = true
	return true
}
type dbPlayerAnouncementColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerAnouncementData
	m_changed bool
}
func (this *dbPlayerAnouncementColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerAnouncementData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerAnouncement{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerAnouncementData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerAnouncementColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerAnouncementColumn)Get( )(v *dbPlayerAnouncementData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerAnouncementColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerAnouncementData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerAnouncementColumn)Set(v dbPlayerAnouncementData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerAnouncementColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerAnouncementData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerAnouncementColumn)GetLastSendTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerAnouncementColumn.GetLastSendTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LastSendTime
	return
}
func (this *dbPlayerAnouncementColumn)SetLastSendTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerAnouncementColumn.SetLastSendTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LastSendTime = v
	this.m_changed = true
	return
}
type dbPlayerFirstDrawCardColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerFirstDrawCardData
	m_changed bool
}
func (this *dbPlayerFirstDrawCardColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerFirstDrawCardList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerFirstDrawCardData{}
		d.from_pb(v)
		this.m_data[int32(d.Id)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerFirstDrawCardColumn)save( )(data []byte,err error){
	pb := &db.PlayerFirstDrawCardList{}
	pb.List=make([]*db.PlayerFirstDrawCard,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerFirstDrawCardColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFirstDrawCardColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerFirstDrawCardColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFirstDrawCardColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerFirstDrawCardColumn)GetAll()(list []dbPlayerFirstDrawCardData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFirstDrawCardColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerFirstDrawCardData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerFirstDrawCardColumn)Get(id int32)(v *dbPlayerFirstDrawCardData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFirstDrawCardColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerFirstDrawCardData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerFirstDrawCardColumn)Set(v dbPlayerFirstDrawCardData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFirstDrawCardColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Id)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerFirstDrawCardColumn)Add(v *dbPlayerFirstDrawCardData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFirstDrawCardColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Id)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	d:=&dbPlayerFirstDrawCardData{}
	v.clone_to(d)
	this.m_data[int32(v.Id)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerFirstDrawCardColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerFirstDrawCardColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerFirstDrawCardColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerFirstDrawCardColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerFirstDrawCardData)
	this.m_changed = true
	return
}
func (this *dbPlayerFirstDrawCardColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFirstDrawCardColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerFirstDrawCardColumn)GetDrawed(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerFirstDrawCardColumn.GetDrawed")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Drawed
	return v,true
}
func (this *dbPlayerFirstDrawCardColumn)SetDrawed(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerFirstDrawCardColumn.SetDrawed")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Drawed = v
	this.m_changed = true
	return true
}
type dbPlayerGuildColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerGuildData
	m_changed bool
}
func (this *dbPlayerGuildColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerGuildData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerGuild{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerGuildData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerGuildColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerGuildColumn)Get( )(v *dbPlayerGuildData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerGuildColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerGuildData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerGuildColumn)Set(v dbPlayerGuildData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerGuildColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerGuildData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerGuildColumn)GetId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerGuildColumn.GetId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.Id
	return
}
func (this *dbPlayerGuildColumn)SetId(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerGuildColumn.SetId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.Id = v
	this.m_changed = true
	return
}
func (this *dbPlayerGuildColumn)GetJoinTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerGuildColumn.GetJoinTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.JoinTime
	return
}
func (this *dbPlayerGuildColumn)SetJoinTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerGuildColumn.SetJoinTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.JoinTime = v
	this.m_changed = true
	return
}
func (this *dbPlayerGuildColumn)GetQuitTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerGuildColumn.GetQuitTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.QuitTime
	return
}
func (this *dbPlayerGuildColumn)SetQuitTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerGuildColumn.SetQuitTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.QuitTime = v
	this.m_changed = true
	return
}
func (this *dbPlayerGuildColumn)GetSignTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerGuildColumn.GetSignTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.SignTime
	return
}
func (this *dbPlayerGuildColumn)SetSignTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerGuildColumn.SetSignTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.SignTime = v
	this.m_changed = true
	return
}
func (this *dbPlayerGuildColumn)GetPosition( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerGuildColumn.GetPosition")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.Position
	return
}
func (this *dbPlayerGuildColumn)SetPosition(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerGuildColumn.SetPosition")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.Position = v
	this.m_changed = true
	return
}
func (this *dbPlayerGuildColumn)GetDonateNum( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerGuildColumn.GetDonateNum")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.DonateNum
	return
}
func (this *dbPlayerGuildColumn)SetDonateNum(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerGuildColumn.SetDonateNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.DonateNum = v
	this.m_changed = true
	return
}
func (this *dbPlayerGuildColumn)GetLastAskDonateTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerGuildColumn.GetLastAskDonateTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LastAskDonateTime
	return
}
func (this *dbPlayerGuildColumn)SetLastAskDonateTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerGuildColumn.SetLastAskDonateTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LastAskDonateTime = v
	this.m_changed = true
	return
}
func (this *dbPlayerGuildColumn)GetLastDonateTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerGuildColumn.GetLastDonateTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LastDonateTime
	return
}
func (this *dbPlayerGuildColumn)SetLastDonateTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerGuildColumn.SetLastDonateTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LastDonateTime = v
	this.m_changed = true
	return
}
type dbPlayerGuildStageColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerGuildStageData
	m_changed bool
}
func (this *dbPlayerGuildStageColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerGuildStageData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerGuildStage{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerGuildStageData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerGuildStageColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerGuildStageColumn)Get( )(v *dbPlayerGuildStageData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerGuildStageColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerGuildStageData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerGuildStageColumn)Set(v dbPlayerGuildStageData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerGuildStageColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerGuildStageData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerGuildStageColumn)GetRespawnNum( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerGuildStageColumn.GetRespawnNum")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.RespawnNum
	return
}
func (this *dbPlayerGuildStageColumn)SetRespawnNum(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerGuildStageColumn.SetRespawnNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.RespawnNum = v
	this.m_changed = true
	return
}
func (this *dbPlayerGuildStageColumn)IncbyRespawnNum(v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerGuildStageColumn.IncbyRespawnNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.RespawnNum += v
	this.m_changed = true
	return this.m_data.RespawnNum
}
func (this *dbPlayerGuildStageColumn)GetRespawnState( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerGuildStageColumn.GetRespawnState")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.RespawnState
	return
}
func (this *dbPlayerGuildStageColumn)SetRespawnState(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerGuildStageColumn.SetRespawnState")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.RespawnState = v
	this.m_changed = true
	return
}
func (this *dbPlayerGuildStageColumn)GetLastRefreshTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerGuildStageColumn.GetLastRefreshTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LastRefreshTime
	return
}
func (this *dbPlayerGuildStageColumn)SetLastRefreshTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerGuildStageColumn.SetLastRefreshTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LastRefreshTime = v
	this.m_changed = true
	return
}
type dbPlayerSignColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerSignData
	m_changed bool
}
func (this *dbPlayerSignColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerSignData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerSign{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerSignData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerSignColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerSignColumn)Get( )(v *dbPlayerSignData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerSignColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerSignData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerSignColumn)Set(v dbPlayerSignData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerSignColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerSignData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerSignColumn)GetCurrGroup( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerSignColumn.GetCurrGroup")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.CurrGroup
	return
}
func (this *dbPlayerSignColumn)SetCurrGroup(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerSignColumn.SetCurrGroup")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.CurrGroup = v
	this.m_changed = true
	return
}
func (this *dbPlayerSignColumn)GetAwardIndex( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerSignColumn.GetAwardIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.AwardIndex
	return
}
func (this *dbPlayerSignColumn)SetAwardIndex(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerSignColumn.SetAwardIndex")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.AwardIndex = v
	this.m_changed = true
	return
}
func (this *dbPlayerSignColumn)GetSignedIndex( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerSignColumn.GetSignedIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.SignedIndex
	return
}
func (this *dbPlayerSignColumn)SetSignedIndex(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerSignColumn.SetSignedIndex")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.SignedIndex = v
	this.m_changed = true
	return
}
func (this *dbPlayerSignColumn)GetLastSignedTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerSignColumn.GetLastSignedTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LastSignedTime
	return
}
func (this *dbPlayerSignColumn)SetLastSignedTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerSignColumn.SetLastSignedTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LastSignedTime = v
	this.m_changed = true
	return
}
type dbPlayerSevenDaysColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerSevenDaysData
	m_changed bool
}
func (this *dbPlayerSevenDaysColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerSevenDaysData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerSevenDays{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerSevenDaysData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerSevenDaysColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerSevenDaysColumn)Get( )(v *dbPlayerSevenDaysData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerSevenDaysColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerSevenDaysData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerSevenDaysColumn)Set(v dbPlayerSevenDaysData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerSevenDaysColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerSevenDaysData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerSevenDaysColumn)GetAwards( )(v []int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerSevenDaysColumn.GetAwards")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = make([]int32, len(this.m_data.Awards))
	for _ii, _vv := range this.m_data.Awards {
		v[_ii]=_vv
	}
	return
}
func (this *dbPlayerSevenDaysColumn)SetAwards(v []int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerSevenDaysColumn.SetAwards")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.Awards = make([]int32, len(v))
	for _ii, _vv := range v {
		this.m_data.Awards[_ii]=_vv
	}
	this.m_changed = true
	return
}
func (this *dbPlayerSevenDaysColumn)GetDays( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerSevenDaysColumn.GetDays")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.Days
	return
}
func (this *dbPlayerSevenDaysColumn)SetDays(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerSevenDaysColumn.SetDays")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.Days = v
	this.m_changed = true
	return
}
type dbPlayerPayCommonColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerPayCommonData
	m_changed bool
}
func (this *dbPlayerPayCommonColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerPayCommonData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerPayCommon{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerPayCommonData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerPayCommonColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerPayCommonColumn)Get( )(v *dbPlayerPayCommonData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerPayCommonColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerPayCommonData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerPayCommonColumn)Set(v dbPlayerPayCommonData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerPayCommonColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerPayCommonData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerPayCommonColumn)GetFirstPayState( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerPayCommonColumn.GetFirstPayState")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.FirstPayState
	return
}
func (this *dbPlayerPayCommonColumn)SetFirstPayState(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerPayCommonColumn.SetFirstPayState")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.FirstPayState = v
	this.m_changed = true
	return
}
type dbPlayerPayColumn struct{
	m_row *dbPlayerRow
	m_data map[string]*dbPlayerPayData
	m_changed bool
}
func (this *dbPlayerPayColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerPayList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerPayData{}
		d.from_pb(v)
		this.m_data[string(d.BundleId)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerPayColumn)save( )(data []byte,err error){
	pb := &db.PlayerPayList{}
	pb.List=make([]*db.PlayerPay,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerPayColumn)HasIndex(id string)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerPayColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerPayColumn)GetAllIndex()(list []string){
	this.m_row.m_lock.UnSafeRLock("dbPlayerPayColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]string, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerPayColumn)GetAll()(list []dbPlayerPayData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerPayColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerPayData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerPayColumn)Get(id string)(v *dbPlayerPayData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerPayColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerPayData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerPayColumn)Set(v dbPlayerPayData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerPayColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[string(v.BundleId)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.BundleId)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerPayColumn)Add(v *dbPlayerPayData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerPayColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[string(v.BundleId)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.BundleId)
		return false
	}
	d:=&dbPlayerPayData{}
	v.clone_to(d)
	this.m_data[string(v.BundleId)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerPayColumn)Remove(id string){
	this.m_row.m_lock.UnSafeLock("dbPlayerPayColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerPayColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerPayColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[string]*dbPlayerPayData)
	this.m_changed = true
	return
}
func (this *dbPlayerPayColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerPayColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerPayColumn)GetLastPayedTime(id string)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerPayColumn.GetLastPayedTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.LastPayedTime
	return v,true
}
func (this *dbPlayerPayColumn)SetLastPayedTime(id string,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerPayColumn.SetLastPayedTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.LastPayedTime = v
	this.m_changed = true
	return true
}
func (this *dbPlayerPayColumn)GetLastAwardTime(id string)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerPayColumn.GetLastAwardTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.LastAwardTime
	return v,true
}
func (this *dbPlayerPayColumn)SetLastAwardTime(id string,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerPayColumn.SetLastAwardTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.LastAwardTime = v
	this.m_changed = true
	return true
}
func (this *dbPlayerPayColumn)GetSendMailNum(id string)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerPayColumn.GetSendMailNum")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.SendMailNum
	return v,true
}
func (this *dbPlayerPayColumn)SetSendMailNum(id string,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerPayColumn.SetSendMailNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.SendMailNum = v
	this.m_changed = true
	return true
}
func (this *dbPlayerPayColumn)IncbySendMailNum(id string,v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerPayColumn.IncbySendMailNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		d = &dbPlayerPayData{}
		this.m_data[id] = d
	}
	d.SendMailNum +=  v
	this.m_changed = true
	return d.SendMailNum
}
func (this *dbPlayerPayColumn)GetChargeNum(id string)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerPayColumn.GetChargeNum")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.ChargeNum
	return v,true
}
func (this *dbPlayerPayColumn)SetChargeNum(id string,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerPayColumn.SetChargeNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.ChargeNum = v
	this.m_changed = true
	return true
}
func (this *dbPlayerPayColumn)IncbyChargeNum(id string,v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerPayColumn.IncbyChargeNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		d = &dbPlayerPayData{}
		this.m_data[id] = d
	}
	d.ChargeNum +=  v
	this.m_changed = true
	return d.ChargeNum
}
type dbPlayerGuideDataColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerGuideDataData
	m_changed bool
}
func (this *dbPlayerGuideDataColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerGuideDataData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerGuideData{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerGuideDataData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerGuideDataColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerGuideDataColumn)Get( )(v *dbPlayerGuideDataData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerGuideDataColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerGuideDataData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerGuideDataColumn)Set(v dbPlayerGuideDataData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerGuideDataColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerGuideDataData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerGuideDataColumn)GetData( )(v []byte){
	this.m_row.m_lock.UnSafeRLock("dbPlayerGuideDataColumn.GetData")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = make([]byte, len(this.m_data.Data))
	for _ii, _vv := range this.m_data.Data {
		v[_ii]=_vv
	}
	return
}
func (this *dbPlayerGuideDataColumn)SetData(v []byte){
	this.m_row.m_lock.UnSafeLock("dbPlayerGuideDataColumn.SetData")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.Data = make([]byte, len(v))
	for _ii, _vv := range v {
		this.m_data.Data[_ii]=_vv
	}
	this.m_changed = true
	return
}
type dbPlayerActivityDataColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerActivityDataData
	m_changed bool
}
func (this *dbPlayerActivityDataColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerActivityDataList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerActivityDataData{}
		d.from_pb(v)
		this.m_data[int32(d.Id)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerActivityDataColumn)save( )(data []byte,err error){
	pb := &db.PlayerActivityDataList{}
	pb.List=make([]*db.PlayerActivityData,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerActivityDataColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerActivityDataColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerActivityDataColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerActivityDataColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerActivityDataColumn)GetAll()(list []dbPlayerActivityDataData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerActivityDataColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerActivityDataData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerActivityDataColumn)Get(id int32)(v *dbPlayerActivityDataData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerActivityDataColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerActivityDataData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerActivityDataColumn)Set(v dbPlayerActivityDataData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerActivityDataColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Id)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerActivityDataColumn)Add(v *dbPlayerActivityDataData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerActivityDataColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Id)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	d:=&dbPlayerActivityDataData{}
	v.clone_to(d)
	this.m_data[int32(v.Id)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerActivityDataColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerActivityDataColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerActivityDataColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerActivityDataColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerActivityDataData)
	this.m_changed = true
	return
}
func (this *dbPlayerActivityDataColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerActivityDataColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerActivityDataColumn)GetSubIds(id int32)(v []int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerActivityDataColumn.GetSubIds")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = make([]int32, len(d.SubIds))
	for _ii, _vv := range d.SubIds {
		v[_ii]=_vv
	}
	return v,true
}
func (this *dbPlayerActivityDataColumn)SetSubIds(id int32,v []int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerActivityDataColumn.SetSubIds")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.SubIds = make([]int32, len(v))
	for _ii, _vv := range v {
		d.SubIds[_ii]=_vv
	}
	this.m_changed = true
	return true
}
func (this *dbPlayerActivityDataColumn)GetSubValues(id int32)(v []int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerActivityDataColumn.GetSubValues")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = make([]int32, len(d.SubValues))
	for _ii, _vv := range d.SubValues {
		v[_ii]=_vv
	}
	return v,true
}
func (this *dbPlayerActivityDataColumn)SetSubValues(id int32,v []int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerActivityDataColumn.SetSubValues")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.SubValues = make([]int32, len(v))
	for _ii, _vv := range v {
		d.SubValues[_ii]=_vv
	}
	this.m_changed = true
	return true
}
func (this *dbPlayerActivityDataColumn)GetSubNum(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerActivityDataColumn.GetSubNum")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.SubNum
	return v,true
}
func (this *dbPlayerActivityDataColumn)SetSubNum(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerActivityDataColumn.SetSubNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.SubNum = v
	this.m_changed = true
	return true
}
func (this *dbPlayerActivityDataColumn)IncbySubNum(id int32,v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerActivityDataColumn.IncbySubNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		d = &dbPlayerActivityDataData{}
		this.m_data[id] = d
	}
	d.SubNum +=  v
	this.m_changed = true
	return d.SubNum
}
type dbPlayerExpeditionDataColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerExpeditionDataData
	m_changed bool
}
func (this *dbPlayerExpeditionDataColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerExpeditionDataData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerExpeditionData{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerExpeditionDataData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerExpeditionDataColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerExpeditionDataColumn)Get( )(v *dbPlayerExpeditionDataData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionDataColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerExpeditionDataData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerExpeditionDataColumn)Set(v dbPlayerExpeditionDataData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionDataColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerExpeditionDataData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerExpeditionDataColumn)GetRefreshTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionDataColumn.GetRefreshTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.RefreshTime
	return
}
func (this *dbPlayerExpeditionDataColumn)SetRefreshTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionDataColumn.SetRefreshTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.RefreshTime = v
	this.m_changed = true
	return
}
func (this *dbPlayerExpeditionDataColumn)GetCurrLevel( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionDataColumn.GetCurrLevel")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.CurrLevel
	return
}
func (this *dbPlayerExpeditionDataColumn)SetCurrLevel(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionDataColumn.SetCurrLevel")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.CurrLevel = v
	this.m_changed = true
	return
}
func (this *dbPlayerExpeditionDataColumn)IncbyCurrLevel(v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionDataColumn.IncbyCurrLevel")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.CurrLevel += v
	this.m_changed = true
	return this.m_data.CurrLevel
}
func (this *dbPlayerExpeditionDataColumn)GetPurifyPoints( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionDataColumn.GetPurifyPoints")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.PurifyPoints
	return
}
func (this *dbPlayerExpeditionDataColumn)SetPurifyPoints(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionDataColumn.SetPurifyPoints")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.PurifyPoints = v
	this.m_changed = true
	return
}
func (this *dbPlayerExpeditionDataColumn)IncbyPurifyPoints(v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionDataColumn.IncbyPurifyPoints")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.PurifyPoints += v
	this.m_changed = true
	return this.m_data.PurifyPoints
}
type dbPlayerExpeditionRoleColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerExpeditionRoleData
	m_changed bool
}
func (this *dbPlayerExpeditionRoleColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerExpeditionRoleList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerExpeditionRoleData{}
		d.from_pb(v)
		this.m_data[int32(d.Id)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerExpeditionRoleColumn)save( )(data []byte,err error){
	pb := &db.PlayerExpeditionRoleList{}
	pb.List=make([]*db.PlayerExpeditionRole,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerExpeditionRoleColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionRoleColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerExpeditionRoleColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionRoleColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerExpeditionRoleColumn)GetAll()(list []dbPlayerExpeditionRoleData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionRoleColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerExpeditionRoleData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerExpeditionRoleColumn)Get(id int32)(v *dbPlayerExpeditionRoleData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionRoleColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerExpeditionRoleData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerExpeditionRoleColumn)Set(v dbPlayerExpeditionRoleData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionRoleColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Id)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerExpeditionRoleColumn)Add(v *dbPlayerExpeditionRoleData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionRoleColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Id)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	d:=&dbPlayerExpeditionRoleData{}
	v.clone_to(d)
	this.m_data[int32(v.Id)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerExpeditionRoleColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionRoleColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerExpeditionRoleColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionRoleColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerExpeditionRoleData)
	this.m_changed = true
	return
}
func (this *dbPlayerExpeditionRoleColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionRoleColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerExpeditionRoleColumn)GetHP(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionRoleColumn.GetHP")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.HP
	return v,true
}
func (this *dbPlayerExpeditionRoleColumn)SetHP(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionRoleColumn.SetHP")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.HP = v
	this.m_changed = true
	return true
}
func (this *dbPlayerExpeditionRoleColumn)GetWeak(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionRoleColumn.GetWeak")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Weak
	return v,true
}
func (this *dbPlayerExpeditionRoleColumn)SetWeak(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionRoleColumn.SetWeak")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Weak = v
	this.m_changed = true
	return true
}
func (this *dbPlayerExpeditionRoleColumn)GetHpPercent(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionRoleColumn.GetHpPercent")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.HpPercent
	return v,true
}
func (this *dbPlayerExpeditionRoleColumn)SetHpPercent(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionRoleColumn.SetHpPercent")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.HpPercent = v
	this.m_changed = true
	return true
}
type dbPlayerExpeditionLevelColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerExpeditionLevelData
	m_changed bool
}
func (this *dbPlayerExpeditionLevelColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerExpeditionLevelList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerExpeditionLevelData{}
		d.from_pb(v)
		this.m_data[int32(d.Level)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerExpeditionLevelColumn)save( )(data []byte,err error){
	pb := &db.PlayerExpeditionLevelList{}
	pb.List=make([]*db.PlayerExpeditionLevel,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerExpeditionLevelColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionLevelColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerExpeditionLevelColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionLevelColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerExpeditionLevelColumn)GetAll()(list []dbPlayerExpeditionLevelData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionLevelColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerExpeditionLevelData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerExpeditionLevelColumn)Get(id int32)(v *dbPlayerExpeditionLevelData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionLevelColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerExpeditionLevelData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerExpeditionLevelColumn)Set(v dbPlayerExpeditionLevelData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionLevelColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Level)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Level)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerExpeditionLevelColumn)Add(v *dbPlayerExpeditionLevelData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionLevelColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Level)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Level)
		return false
	}
	d:=&dbPlayerExpeditionLevelData{}
	v.clone_to(d)
	this.m_data[int32(v.Level)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerExpeditionLevelColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionLevelColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerExpeditionLevelColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionLevelColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerExpeditionLevelData)
	this.m_changed = true
	return
}
func (this *dbPlayerExpeditionLevelColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionLevelColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerExpeditionLevelColumn)GetPlayerId(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionLevelColumn.GetPlayerId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.PlayerId
	return v,true
}
func (this *dbPlayerExpeditionLevelColumn)SetPlayerId(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionLevelColumn.SetPlayerId")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.PlayerId = v
	this.m_changed = true
	return true
}
func (this *dbPlayerExpeditionLevelColumn)GetPower(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionLevelColumn.GetPower")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Power
	return v,true
}
func (this *dbPlayerExpeditionLevelColumn)SetPower(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionLevelColumn.SetPower")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Power = v
	this.m_changed = true
	return true
}
func (this *dbPlayerExpeditionLevelColumn)GetGoldIncome(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionLevelColumn.GetGoldIncome")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.GoldIncome
	return v,true
}
func (this *dbPlayerExpeditionLevelColumn)SetGoldIncome(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionLevelColumn.SetGoldIncome")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.GoldIncome = v
	this.m_changed = true
	return true
}
func (this *dbPlayerExpeditionLevelColumn)GetExpeditionGoldIncome(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionLevelColumn.GetExpeditionGoldIncome")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.ExpeditionGoldIncome
	return v,true
}
func (this *dbPlayerExpeditionLevelColumn)SetExpeditionGoldIncome(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionLevelColumn.SetExpeditionGoldIncome")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.ExpeditionGoldIncome = v
	this.m_changed = true
	return true
}
type dbPlayerExpeditionLevelRoleColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerExpeditionLevelRoleData
	m_changed bool
}
func (this *dbPlayerExpeditionLevelRoleColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerExpeditionLevelRoleList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerExpeditionLevelRoleData{}
		d.from_pb(v)
		this.m_data[int32(d.Pos)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerExpeditionLevelRoleColumn)save( )(data []byte,err error){
	pb := &db.PlayerExpeditionLevelRoleList{}
	pb.List=make([]*db.PlayerExpeditionLevelRole,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerExpeditionLevelRoleColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionLevelRoleColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerExpeditionLevelRoleColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionLevelRoleColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerExpeditionLevelRoleColumn)GetAll()(list []dbPlayerExpeditionLevelRoleData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionLevelRoleColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerExpeditionLevelRoleData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerExpeditionLevelRoleColumn)Get(id int32)(v *dbPlayerExpeditionLevelRoleData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionLevelRoleColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerExpeditionLevelRoleData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerExpeditionLevelRoleColumn)Set(v dbPlayerExpeditionLevelRoleData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionLevelRoleColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Pos)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Pos)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerExpeditionLevelRoleColumn)Add(v *dbPlayerExpeditionLevelRoleData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionLevelRoleColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Pos)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Pos)
		return false
	}
	d:=&dbPlayerExpeditionLevelRoleData{}
	v.clone_to(d)
	this.m_data[int32(v.Pos)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerExpeditionLevelRoleColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionLevelRoleColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerExpeditionLevelRoleColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionLevelRoleColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerExpeditionLevelRoleData)
	this.m_changed = true
	return
}
func (this *dbPlayerExpeditionLevelRoleColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionLevelRoleColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerExpeditionLevelRoleColumn)GetTableId(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionLevelRoleColumn.GetTableId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.TableId
	return v,true
}
func (this *dbPlayerExpeditionLevelRoleColumn)SetTableId(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionLevelRoleColumn.SetTableId")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.TableId = v
	this.m_changed = true
	return true
}
func (this *dbPlayerExpeditionLevelRoleColumn)GetRank(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionLevelRoleColumn.GetRank")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Rank
	return v,true
}
func (this *dbPlayerExpeditionLevelRoleColumn)SetRank(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionLevelRoleColumn.SetRank")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Rank = v
	this.m_changed = true
	return true
}
func (this *dbPlayerExpeditionLevelRoleColumn)GetLevel(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionLevelRoleColumn.GetLevel")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Level
	return v,true
}
func (this *dbPlayerExpeditionLevelRoleColumn)SetLevel(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionLevelRoleColumn.SetLevel")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Level = v
	this.m_changed = true
	return true
}
func (this *dbPlayerExpeditionLevelRoleColumn)GetEquip(id int32)(v []int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionLevelRoleColumn.GetEquip")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = make([]int32, len(d.Equip))
	for _ii, _vv := range d.Equip {
		v[_ii]=_vv
	}
	return v,true
}
func (this *dbPlayerExpeditionLevelRoleColumn)SetEquip(id int32,v []int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionLevelRoleColumn.SetEquip")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Equip = make([]int32, len(v))
	for _ii, _vv := range v {
		d.Equip[_ii]=_vv
	}
	this.m_changed = true
	return true
}
func (this *dbPlayerExpeditionLevelRoleColumn)GetHP(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionLevelRoleColumn.GetHP")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.HP
	return v,true
}
func (this *dbPlayerExpeditionLevelRoleColumn)SetHP(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionLevelRoleColumn.SetHP")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.HP = v
	this.m_changed = true
	return true
}
func (this *dbPlayerExpeditionLevelRoleColumn)GetHpPercent(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerExpeditionLevelRoleColumn.GetHpPercent")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.HpPercent
	return v,true
}
func (this *dbPlayerExpeditionLevelRoleColumn)SetHpPercent(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerExpeditionLevelRoleColumn.SetHpPercent")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.HpPercent = v
	this.m_changed = true
	return true
}
type dbPlayerSysMailColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerSysMailData
	m_changed bool
}
func (this *dbPlayerSysMailColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerSysMailData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerSysMail{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerSysMailData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerSysMailColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerSysMailColumn)Get( )(v *dbPlayerSysMailData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerSysMailColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerSysMailData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerSysMailColumn)Set(v dbPlayerSysMailData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerSysMailColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerSysMailData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerSysMailColumn)GetCurrId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerSysMailColumn.GetCurrId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.CurrId
	return
}
func (this *dbPlayerSysMailColumn)SetCurrId(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerSysMailColumn.SetCurrId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.CurrId = v
	this.m_changed = true
	return
}
type dbPlayerArtifactColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerArtifactData
	m_changed bool
}
func (this *dbPlayerArtifactColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerArtifactList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerArtifactData{}
		d.from_pb(v)
		this.m_data[int32(d.Id)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerArtifactColumn)save( )(data []byte,err error){
	pb := &db.PlayerArtifactList{}
	pb.List=make([]*db.PlayerArtifact,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerArtifactColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerArtifactColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerArtifactColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerArtifactColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerArtifactColumn)GetAll()(list []dbPlayerArtifactData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerArtifactColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerArtifactData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerArtifactColumn)Get(id int32)(v *dbPlayerArtifactData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerArtifactColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerArtifactData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerArtifactColumn)Set(v dbPlayerArtifactData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerArtifactColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Id)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerArtifactColumn)Add(v *dbPlayerArtifactData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerArtifactColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Id)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	d:=&dbPlayerArtifactData{}
	v.clone_to(d)
	this.m_data[int32(v.Id)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerArtifactColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerArtifactColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerArtifactColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerArtifactColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerArtifactData)
	this.m_changed = true
	return
}
func (this *dbPlayerArtifactColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerArtifactColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerArtifactColumn)GetRank(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerArtifactColumn.GetRank")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Rank
	return v,true
}
func (this *dbPlayerArtifactColumn)SetRank(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerArtifactColumn.SetRank")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Rank = v
	this.m_changed = true
	return true
}
func (this *dbPlayerArtifactColumn)IncbyRank(id int32,v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerArtifactColumn.IncbyRank")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		d = &dbPlayerArtifactData{}
		this.m_data[id] = d
	}
	d.Rank +=  v
	this.m_changed = true
	return d.Rank
}
func (this *dbPlayerArtifactColumn)GetLevel(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerArtifactColumn.GetLevel")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Level
	return v,true
}
func (this *dbPlayerArtifactColumn)SetLevel(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerArtifactColumn.SetLevel")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Level = v
	this.m_changed = true
	return true
}
func (this *dbPlayerArtifactColumn)IncbyLevel(id int32,v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerArtifactColumn.IncbyLevel")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		d = &dbPlayerArtifactData{}
		this.m_data[id] = d
	}
	d.Level +=  v
	this.m_changed = true
	return d.Level
}
type dbPlayerCarnivalCommonColumn struct{
	m_row *dbPlayerRow
	m_data *dbPlayerCarnivalCommonData
	m_changed bool
}
func (this *dbPlayerCarnivalCommonColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbPlayerCarnivalCommonData{}
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerCarnivalCommon{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_data = &dbPlayerCarnivalCommonData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbPlayerCarnivalCommonColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerCarnivalCommonColumn)Get( )(v *dbPlayerCarnivalCommonData ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCarnivalCommonColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbPlayerCarnivalCommonData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbPlayerCarnivalCommonColumn)Set(v dbPlayerCarnivalCommonData ){
	this.m_row.m_lock.UnSafeLock("dbPlayerCarnivalCommonColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbPlayerCarnivalCommonData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbPlayerCarnivalCommonColumn)GetDayResetTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCarnivalCommonColumn.GetDayResetTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.DayResetTime
	return
}
func (this *dbPlayerCarnivalCommonColumn)SetDayResetTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCarnivalCommonColumn.SetDayResetTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.DayResetTime = v
	this.m_changed = true
	return
}
type dbPlayerCarnivalColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerCarnivalData
	m_changed bool
}
func (this *dbPlayerCarnivalColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerCarnivalList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerCarnivalData{}
		d.from_pb(v)
		this.m_data[int32(d.Id)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerCarnivalColumn)save( )(data []byte,err error){
	pb := &db.PlayerCarnivalList{}
	pb.List=make([]*db.PlayerCarnival,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerCarnivalColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCarnivalColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerCarnivalColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCarnivalColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerCarnivalColumn)GetAll()(list []dbPlayerCarnivalData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCarnivalColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerCarnivalData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerCarnivalColumn)Get(id int32)(v *dbPlayerCarnivalData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCarnivalColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerCarnivalData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerCarnivalColumn)Set(v dbPlayerCarnivalData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerCarnivalColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Id)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerCarnivalColumn)Add(v *dbPlayerCarnivalData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerCarnivalColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.Id)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Id)
		return false
	}
	d:=&dbPlayerCarnivalData{}
	v.clone_to(d)
	this.m_data[int32(v.Id)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerCarnivalColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCarnivalColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerCarnivalColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerCarnivalColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerCarnivalData)
	this.m_changed = true
	return
}
func (this *dbPlayerCarnivalColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCarnivalColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerCarnivalColumn)GetValue(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCarnivalColumn.GetValue")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Value
	return v,true
}
func (this *dbPlayerCarnivalColumn)SetValue(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerCarnivalColumn.SetValue")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Value = v
	this.m_changed = true
	return true
}
func (this *dbPlayerCarnivalColumn)IncbyValue(id int32,v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCarnivalColumn.IncbyValue")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		d = &dbPlayerCarnivalData{}
		this.m_data[id] = d
	}
	d.Value +=  v
	this.m_changed = true
	return d.Value
}
func (this *dbPlayerCarnivalColumn)GetValue2(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCarnivalColumn.GetValue2")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Value2
	return v,true
}
func (this *dbPlayerCarnivalColumn)SetValue2(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerCarnivalColumn.SetValue2")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.Value2 = v
	this.m_changed = true
	return true
}
func (this *dbPlayerCarnivalColumn)IncbyValue2(id int32,v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCarnivalColumn.IncbyValue2")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		d = &dbPlayerCarnivalData{}
		this.m_data[id] = d
	}
	d.Value2 +=  v
	this.m_changed = true
	return d.Value2
}
type dbPlayerInviteCodesColumn struct{
	m_row *dbPlayerRow
	m_data map[string]*dbPlayerInviteCodesData
	m_changed bool
}
func (this *dbPlayerInviteCodesColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerInviteCodesList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerInviteCodesData{}
		d.from_pb(v)
		this.m_data[string(d.Code)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerInviteCodesColumn)save( )(data []byte,err error){
	pb := &db.PlayerInviteCodesList{}
	pb.List=make([]*db.PlayerInviteCodes,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetPlayerId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbPlayerInviteCodesColumn)HasIndex(id string)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerInviteCodesColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerInviteCodesColumn)GetAllIndex()(list []string){
	this.m_row.m_lock.UnSafeRLock("dbPlayerInviteCodesColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]string, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerInviteCodesColumn)GetAll()(list []dbPlayerInviteCodesData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerInviteCodesColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerInviteCodesData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerInviteCodesColumn)Get(id string)(v *dbPlayerInviteCodesData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerInviteCodesColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerInviteCodesData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerInviteCodesColumn)Set(v dbPlayerInviteCodesData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerInviteCodesColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[string(v.Code)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.Code)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerInviteCodesColumn)Add(v *dbPlayerInviteCodesData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerInviteCodesColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[string(v.Code)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.Code)
		return false
	}
	d:=&dbPlayerInviteCodesData{}
	v.clone_to(d)
	this.m_data[string(v.Code)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerInviteCodesColumn)Remove(id string){
	this.m_row.m_lock.UnSafeLock("dbPlayerInviteCodesColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerInviteCodesColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerInviteCodesColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[string]*dbPlayerInviteCodesData)
	this.m_changed = true
	return
}
func (this *dbPlayerInviteCodesColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerInviteCodesColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
type dbPlayerRow struct {
	m_table *dbPlayerTable
	m_lock       *RWMutex
	m_loaded  bool
	m_new     bool
	m_remove  bool
	m_touch      int32
	m_releasable bool
	m_valid   bool
	m_PlayerId        int32
	m_UniqueId_changed bool
	m_UniqueId string
	m_Account_changed bool
	m_Account string
	m_Name_changed bool
	m_Name string
	m_Token_changed bool
	m_Token string
	m_CurrReplyMsgNum_changed bool
	m_CurrReplyMsgNum int32
	Info dbPlayerInfoColumn
	Global dbPlayerGlobalColumn
	m_Level_changed bool
	m_Level int32
	Items dbPlayerItemColumn
	RoleCommon dbPlayerRoleCommonColumn
	Roles dbPlayerRoleColumn
	RoleHandbook dbPlayerRoleHandbookColumn
	BattleTeam dbPlayerBattleTeamColumn
	CampaignCommon dbPlayerCampaignCommonColumn
	Campaigns dbPlayerCampaignColumn
	CampaignStaticIncomes dbPlayerCampaignStaticIncomeColumn
	CampaignRandomIncomes dbPlayerCampaignRandomIncomeColumn
	MailCommon dbPlayerMailCommonColumn
	Mails dbPlayerMailColumn
	BattleSaves dbPlayerBattleSaveColumn
	Talents dbPlayerTalentColumn
	TowerCommon dbPlayerTowerCommonColumn
	Towers dbPlayerTowerColumn
	Draws dbPlayerDrawColumn
	GoldHand dbPlayerGoldHandColumn
	Shops dbPlayerShopColumn
	ShopItems dbPlayerShopItemColumn
	Arena dbPlayerArenaColumn
	Equip dbPlayerEquipColumn
	ActiveStageCommon dbPlayerActiveStageCommonColumn
	ActiveStages dbPlayerActiveStageColumn
	FriendCommon dbPlayerFriendCommonColumn
	Friends dbPlayerFriendColumn
	FriendRecommends dbPlayerFriendRecommendColumn
	FriendAsks dbPlayerFriendAskColumn
	FriendBosss dbPlayerFriendBossColumn
	TaskCommon dbPlayerTaskCommonColumn
	Tasks dbPlayerTaskColumn
	FinishedTasks dbPlayerFinishedTaskColumn
	DailyTaskAllDailys dbPlayerDailyTaskAllDailyColumn
	ExploreCommon dbPlayerExploreCommonColumn
	Explores dbPlayerExploreColumn
	ExploreStorys dbPlayerExploreStoryColumn
	FriendChatUnreadIds dbPlayerFriendChatUnreadIdColumn
	FriendChatUnreadMessages dbPlayerFriendChatUnreadMessageColumn
	HeadItems dbPlayerHeadItemColumn
	SuitAwards dbPlayerSuitAwardColumn
	Chats dbPlayerChatColumn
	Anouncement dbPlayerAnouncementColumn
	FirstDrawCards dbPlayerFirstDrawCardColumn
	Guild dbPlayerGuildColumn
	GuildStage dbPlayerGuildStageColumn
	Sign dbPlayerSignColumn
	SevenDays dbPlayerSevenDaysColumn
	PayCommon dbPlayerPayCommonColumn
	Pays dbPlayerPayColumn
	GuideData dbPlayerGuideDataColumn
	ActivityDatas dbPlayerActivityDataColumn
	ExpeditionData dbPlayerExpeditionDataColumn
	ExpeditionRoles dbPlayerExpeditionRoleColumn
	ExpeditionLevels dbPlayerExpeditionLevelColumn
	ExpeditionLevelRole0s dbPlayerExpeditionLevelRoleColumn
	ExpeditionLevelRole1s dbPlayerExpeditionLevelRoleColumn
	ExpeditionLevelRole2s dbPlayerExpeditionLevelRoleColumn
	ExpeditionLevelRole3s dbPlayerExpeditionLevelRoleColumn
	ExpeditionLevelRole4s dbPlayerExpeditionLevelRoleColumn
	ExpeditionLevelRole5s dbPlayerExpeditionLevelRoleColumn
	ExpeditionLevelRole6s dbPlayerExpeditionLevelRoleColumn
	ExpeditionLevelRole7s dbPlayerExpeditionLevelRoleColumn
	ExpeditionLevelRole8s dbPlayerExpeditionLevelRoleColumn
	ExpeditionLevelRole9s dbPlayerExpeditionLevelRoleColumn
	SysMail dbPlayerSysMailColumn
	Artifacts dbPlayerArtifactColumn
	CarnivalCommon dbPlayerCarnivalCommonColumn
	Carnivals dbPlayerCarnivalColumn
	InviteCodess dbPlayerInviteCodesColumn
}
func new_dbPlayerRow(table *dbPlayerTable, PlayerId int32) (r *dbPlayerRow) {
	this := &dbPlayerRow{}
	this.m_table = table
	this.m_PlayerId = PlayerId
	this.m_lock = NewRWMutex()
	this.m_UniqueId_changed=true
	this.m_Account_changed=true
	this.m_Name_changed=true
	this.m_Token_changed=true
	this.m_CurrReplyMsgNum_changed=true
	this.m_Level_changed=true
	this.Info.m_row=this
	this.Info.m_data=&dbPlayerInfoData{}
	this.Global.m_row=this
	this.Global.m_data=&dbPlayerGlobalData{}
	this.Items.m_row=this
	this.Items.m_data=make(map[int32]*dbPlayerItemData)
	this.RoleCommon.m_row=this
	this.RoleCommon.m_data=&dbPlayerRoleCommonData{}
	this.Roles.m_row=this
	this.Roles.m_data=make(map[int32]*dbPlayerRoleData)
	this.RoleHandbook.m_row=this
	this.RoleHandbook.m_data=&dbPlayerRoleHandbookData{}
	this.BattleTeam.m_row=this
	this.BattleTeam.m_data=&dbPlayerBattleTeamData{}
	this.CampaignCommon.m_row=this
	this.CampaignCommon.m_data=&dbPlayerCampaignCommonData{}
	this.Campaigns.m_row=this
	this.Campaigns.m_data=make(map[int32]*dbPlayerCampaignData)
	this.CampaignStaticIncomes.m_row=this
	this.CampaignStaticIncomes.m_data=make(map[int32]*dbPlayerCampaignStaticIncomeData)
	this.CampaignRandomIncomes.m_row=this
	this.CampaignRandomIncomes.m_data=make(map[int32]*dbPlayerCampaignRandomIncomeData)
	this.MailCommon.m_row=this
	this.MailCommon.m_data=&dbPlayerMailCommonData{}
	this.Mails.m_row=this
	this.Mails.m_data=make(map[int32]*dbPlayerMailData)
	this.BattleSaves.m_row=this
	this.BattleSaves.m_data=make(map[int32]*dbPlayerBattleSaveData)
	this.Talents.m_row=this
	this.Talents.m_data=make(map[int32]*dbPlayerTalentData)
	this.TowerCommon.m_row=this
	this.TowerCommon.m_data=&dbPlayerTowerCommonData{}
	this.Towers.m_row=this
	this.Towers.m_data=make(map[int32]*dbPlayerTowerData)
	this.Draws.m_row=this
	this.Draws.m_data=make(map[int32]*dbPlayerDrawData)
	this.GoldHand.m_row=this
	this.GoldHand.m_data=&dbPlayerGoldHandData{}
	this.Shops.m_row=this
	this.Shops.m_data=make(map[int32]*dbPlayerShopData)
	this.ShopItems.m_row=this
	this.ShopItems.m_data=make(map[int32]*dbPlayerShopItemData)
	this.Arena.m_row=this
	this.Arena.m_data=&dbPlayerArenaData{}
	this.Equip.m_row=this
	this.Equip.m_data=&dbPlayerEquipData{}
	this.ActiveStageCommon.m_row=this
	this.ActiveStageCommon.m_data=&dbPlayerActiveStageCommonData{}
	this.ActiveStages.m_row=this
	this.ActiveStages.m_data=make(map[int32]*dbPlayerActiveStageData)
	this.FriendCommon.m_row=this
	this.FriendCommon.m_data=&dbPlayerFriendCommonData{}
	this.Friends.m_row=this
	this.Friends.m_data=make(map[int32]*dbPlayerFriendData)
	this.FriendRecommends.m_row=this
	this.FriendRecommends.m_data=make(map[int32]*dbPlayerFriendRecommendData)
	this.FriendAsks.m_row=this
	this.FriendAsks.m_data=make(map[int32]*dbPlayerFriendAskData)
	this.FriendBosss.m_row=this
	this.FriendBosss.m_data=make(map[int32]*dbPlayerFriendBossData)
	this.TaskCommon.m_row=this
	this.TaskCommon.m_data=&dbPlayerTaskCommonData{}
	this.Tasks.m_row=this
	this.Tasks.m_data=make(map[int32]*dbPlayerTaskData)
	this.FinishedTasks.m_row=this
	this.FinishedTasks.m_data=make(map[int32]*dbPlayerFinishedTaskData)
	this.DailyTaskAllDailys.m_row=this
	this.DailyTaskAllDailys.m_data=make(map[int32]*dbPlayerDailyTaskAllDailyData)
	this.ExploreCommon.m_row=this
	this.ExploreCommon.m_data=&dbPlayerExploreCommonData{}
	this.Explores.m_row=this
	this.Explores.m_data=make(map[int32]*dbPlayerExploreData)
	this.ExploreStorys.m_row=this
	this.ExploreStorys.m_data=make(map[int32]*dbPlayerExploreStoryData)
	this.FriendChatUnreadIds.m_row=this
	this.FriendChatUnreadIds.m_data=make(map[int32]*dbPlayerFriendChatUnreadIdData)
	this.FriendChatUnreadMessages.m_row=this
	this.FriendChatUnreadMessages.m_data=make(map[int64]*dbPlayerFriendChatUnreadMessageData)
	this.HeadItems.m_row=this
	this.HeadItems.m_data=make(map[int32]*dbPlayerHeadItemData)
	this.SuitAwards.m_row=this
	this.SuitAwards.m_data=make(map[int32]*dbPlayerSuitAwardData)
	this.Chats.m_row=this
	this.Chats.m_data=make(map[int32]*dbPlayerChatData)
	this.Anouncement.m_row=this
	this.Anouncement.m_data=&dbPlayerAnouncementData{}
	this.FirstDrawCards.m_row=this
	this.FirstDrawCards.m_data=make(map[int32]*dbPlayerFirstDrawCardData)
	this.Guild.m_row=this
	this.Guild.m_data=&dbPlayerGuildData{}
	this.GuildStage.m_row=this
	this.GuildStage.m_data=&dbPlayerGuildStageData{}
	this.Sign.m_row=this
	this.Sign.m_data=&dbPlayerSignData{}
	this.SevenDays.m_row=this
	this.SevenDays.m_data=&dbPlayerSevenDaysData{}
	this.PayCommon.m_row=this
	this.PayCommon.m_data=&dbPlayerPayCommonData{}
	this.Pays.m_row=this
	this.Pays.m_data=make(map[string]*dbPlayerPayData)
	this.GuideData.m_row=this
	this.GuideData.m_data=&dbPlayerGuideDataData{}
	this.ActivityDatas.m_row=this
	this.ActivityDatas.m_data=make(map[int32]*dbPlayerActivityDataData)
	this.ExpeditionData.m_row=this
	this.ExpeditionData.m_data=&dbPlayerExpeditionDataData{}
	this.ExpeditionRoles.m_row=this
	this.ExpeditionRoles.m_data=make(map[int32]*dbPlayerExpeditionRoleData)
	this.ExpeditionLevels.m_row=this
	this.ExpeditionLevels.m_data=make(map[int32]*dbPlayerExpeditionLevelData)
	this.ExpeditionLevelRole0s.m_row=this
	this.ExpeditionLevelRole0s.m_data=make(map[int32]*dbPlayerExpeditionLevelRoleData)
	this.ExpeditionLevelRole1s.m_row=this
	this.ExpeditionLevelRole1s.m_data=make(map[int32]*dbPlayerExpeditionLevelRoleData)
	this.ExpeditionLevelRole2s.m_row=this
	this.ExpeditionLevelRole2s.m_data=make(map[int32]*dbPlayerExpeditionLevelRoleData)
	this.ExpeditionLevelRole3s.m_row=this
	this.ExpeditionLevelRole3s.m_data=make(map[int32]*dbPlayerExpeditionLevelRoleData)
	this.ExpeditionLevelRole4s.m_row=this
	this.ExpeditionLevelRole4s.m_data=make(map[int32]*dbPlayerExpeditionLevelRoleData)
	this.ExpeditionLevelRole5s.m_row=this
	this.ExpeditionLevelRole5s.m_data=make(map[int32]*dbPlayerExpeditionLevelRoleData)
	this.ExpeditionLevelRole6s.m_row=this
	this.ExpeditionLevelRole6s.m_data=make(map[int32]*dbPlayerExpeditionLevelRoleData)
	this.ExpeditionLevelRole7s.m_row=this
	this.ExpeditionLevelRole7s.m_data=make(map[int32]*dbPlayerExpeditionLevelRoleData)
	this.ExpeditionLevelRole8s.m_row=this
	this.ExpeditionLevelRole8s.m_data=make(map[int32]*dbPlayerExpeditionLevelRoleData)
	this.ExpeditionLevelRole9s.m_row=this
	this.ExpeditionLevelRole9s.m_data=make(map[int32]*dbPlayerExpeditionLevelRoleData)
	this.SysMail.m_row=this
	this.SysMail.m_data=&dbPlayerSysMailData{}
	this.Artifacts.m_row=this
	this.Artifacts.m_data=make(map[int32]*dbPlayerArtifactData)
	this.CarnivalCommon.m_row=this
	this.CarnivalCommon.m_data=&dbPlayerCarnivalCommonData{}
	this.Carnivals.m_row=this
	this.Carnivals.m_data=make(map[int32]*dbPlayerCarnivalData)
	this.InviteCodess.m_row=this
	this.InviteCodess.m_data=make(map[string]*dbPlayerInviteCodesData)
	return this
}
func (this *dbPlayerRow) GetPlayerId() (r int32) {
	return this.m_PlayerId
}
func (this *dbPlayerRow) save_data(release bool) (err error, released bool, state int32, update_string string, args []interface{}) {
	this.m_lock.UnSafeLock("dbPlayerRow.save_data")
	defer this.m_lock.UnSafeUnlock()
	if this.m_new {
		db_args:=new_db_args(77)
		db_args.Push(this.m_PlayerId)
		db_args.Push(this.m_UniqueId)
		db_args.Push(this.m_Account)
		db_args.Push(this.m_Name)
		db_args.Push(this.m_Token)
		db_args.Push(this.m_CurrReplyMsgNum)
		dInfo,db_err:=this.Info.save()
		if db_err!=nil{
			log.Error("insert save Info failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dInfo)
		dGlobal,db_err:=this.Global.save()
		if db_err!=nil{
			log.Error("insert save Global failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dGlobal)
		db_args.Push(this.m_Level)
		dItems,db_err:=this.Items.save()
		if db_err!=nil{
			log.Error("insert save Item failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dItems)
		dRoleCommon,db_err:=this.RoleCommon.save()
		if db_err!=nil{
			log.Error("insert save RoleCommon failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dRoleCommon)
		dRoles,db_err:=this.Roles.save()
		if db_err!=nil{
			log.Error("insert save Role failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dRoles)
		dRoleHandbook,db_err:=this.RoleHandbook.save()
		if db_err!=nil{
			log.Error("insert save RoleHandbook failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dRoleHandbook)
		dBattleTeam,db_err:=this.BattleTeam.save()
		if db_err!=nil{
			log.Error("insert save BattleTeam failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dBattleTeam)
		dCampaignCommon,db_err:=this.CampaignCommon.save()
		if db_err!=nil{
			log.Error("insert save CampaignCommon failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dCampaignCommon)
		dCampaigns,db_err:=this.Campaigns.save()
		if db_err!=nil{
			log.Error("insert save Campaign failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dCampaigns)
		dCampaignStaticIncomes,db_err:=this.CampaignStaticIncomes.save()
		if db_err!=nil{
			log.Error("insert save CampaignStaticIncome failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dCampaignStaticIncomes)
		dCampaignRandomIncomes,db_err:=this.CampaignRandomIncomes.save()
		if db_err!=nil{
			log.Error("insert save CampaignRandomIncome failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dCampaignRandomIncomes)
		dMailCommon,db_err:=this.MailCommon.save()
		if db_err!=nil{
			log.Error("insert save MailCommon failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dMailCommon)
		dMails,db_err:=this.Mails.save()
		if db_err!=nil{
			log.Error("insert save Mail failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dMails)
		dBattleSaves,db_err:=this.BattleSaves.save()
		if db_err!=nil{
			log.Error("insert save BattleSave failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dBattleSaves)
		dTalents,db_err:=this.Talents.save()
		if db_err!=nil{
			log.Error("insert save Talent failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dTalents)
		dTowerCommon,db_err:=this.TowerCommon.save()
		if db_err!=nil{
			log.Error("insert save TowerCommon failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dTowerCommon)
		dTowers,db_err:=this.Towers.save()
		if db_err!=nil{
			log.Error("insert save Tower failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dTowers)
		dDraws,db_err:=this.Draws.save()
		if db_err!=nil{
			log.Error("insert save Draw failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dDraws)
		dGoldHand,db_err:=this.GoldHand.save()
		if db_err!=nil{
			log.Error("insert save GoldHand failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dGoldHand)
		dShops,db_err:=this.Shops.save()
		if db_err!=nil{
			log.Error("insert save Shop failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dShops)
		dShopItems,db_err:=this.ShopItems.save()
		if db_err!=nil{
			log.Error("insert save ShopItem failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dShopItems)
		dArena,db_err:=this.Arena.save()
		if db_err!=nil{
			log.Error("insert save Arena failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dArena)
		dEquip,db_err:=this.Equip.save()
		if db_err!=nil{
			log.Error("insert save Equip failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dEquip)
		dActiveStageCommon,db_err:=this.ActiveStageCommon.save()
		if db_err!=nil{
			log.Error("insert save ActiveStageCommon failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dActiveStageCommon)
		dActiveStages,db_err:=this.ActiveStages.save()
		if db_err!=nil{
			log.Error("insert save ActiveStage failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dActiveStages)
		dFriendCommon,db_err:=this.FriendCommon.save()
		if db_err!=nil{
			log.Error("insert save FriendCommon failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dFriendCommon)
		dFriends,db_err:=this.Friends.save()
		if db_err!=nil{
			log.Error("insert save Friend failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dFriends)
		dFriendRecommends,db_err:=this.FriendRecommends.save()
		if db_err!=nil{
			log.Error("insert save FriendRecommend failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dFriendRecommends)
		dFriendAsks,db_err:=this.FriendAsks.save()
		if db_err!=nil{
			log.Error("insert save FriendAsk failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dFriendAsks)
		dFriendBosss,db_err:=this.FriendBosss.save()
		if db_err!=nil{
			log.Error("insert save FriendBoss failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dFriendBosss)
		dTaskCommon,db_err:=this.TaskCommon.save()
		if db_err!=nil{
			log.Error("insert save TaskCommon failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dTaskCommon)
		dTasks,db_err:=this.Tasks.save()
		if db_err!=nil{
			log.Error("insert save Task failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dTasks)
		dFinishedTasks,db_err:=this.FinishedTasks.save()
		if db_err!=nil{
			log.Error("insert save FinishedTask failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dFinishedTasks)
		dDailyTaskAllDailys,db_err:=this.DailyTaskAllDailys.save()
		if db_err!=nil{
			log.Error("insert save DailyTaskAllDaily failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dDailyTaskAllDailys)
		dExploreCommon,db_err:=this.ExploreCommon.save()
		if db_err!=nil{
			log.Error("insert save ExploreCommon failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dExploreCommon)
		dExplores,db_err:=this.Explores.save()
		if db_err!=nil{
			log.Error("insert save Explore failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dExplores)
		dExploreStorys,db_err:=this.ExploreStorys.save()
		if db_err!=nil{
			log.Error("insert save ExploreStory failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dExploreStorys)
		dFriendChatUnreadIds,db_err:=this.FriendChatUnreadIds.save()
		if db_err!=nil{
			log.Error("insert save FriendChatUnreadId failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dFriendChatUnreadIds)
		dFriendChatUnreadMessages,db_err:=this.FriendChatUnreadMessages.save()
		if db_err!=nil{
			log.Error("insert save FriendChatUnreadMessage failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dFriendChatUnreadMessages)
		dHeadItems,db_err:=this.HeadItems.save()
		if db_err!=nil{
			log.Error("insert save HeadItem failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dHeadItems)
		dSuitAwards,db_err:=this.SuitAwards.save()
		if db_err!=nil{
			log.Error("insert save SuitAward failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dSuitAwards)
		dChats,db_err:=this.Chats.save()
		if db_err!=nil{
			log.Error("insert save Chat failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dChats)
		dAnouncement,db_err:=this.Anouncement.save()
		if db_err!=nil{
			log.Error("insert save Anouncement failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dAnouncement)
		dFirstDrawCards,db_err:=this.FirstDrawCards.save()
		if db_err!=nil{
			log.Error("insert save FirstDrawCard failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dFirstDrawCards)
		dGuild,db_err:=this.Guild.save()
		if db_err!=nil{
			log.Error("insert save Guild failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dGuild)
		dGuildStage,db_err:=this.GuildStage.save()
		if db_err!=nil{
			log.Error("insert save GuildStage failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dGuildStage)
		dSign,db_err:=this.Sign.save()
		if db_err!=nil{
			log.Error("insert save Sign failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dSign)
		dSevenDays,db_err:=this.SevenDays.save()
		if db_err!=nil{
			log.Error("insert save SevenDays failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dSevenDays)
		dPayCommon,db_err:=this.PayCommon.save()
		if db_err!=nil{
			log.Error("insert save PayCommon failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dPayCommon)
		dPays,db_err:=this.Pays.save()
		if db_err!=nil{
			log.Error("insert save Pay failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dPays)
		dGuideData,db_err:=this.GuideData.save()
		if db_err!=nil{
			log.Error("insert save GuideData failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dGuideData)
		dActivityDatas,db_err:=this.ActivityDatas.save()
		if db_err!=nil{
			log.Error("insert save ActivityData failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dActivityDatas)
		dExpeditionData,db_err:=this.ExpeditionData.save()
		if db_err!=nil{
			log.Error("insert save ExpeditionData failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dExpeditionData)
		dExpeditionRoles,db_err:=this.ExpeditionRoles.save()
		if db_err!=nil{
			log.Error("insert save ExpeditionRole failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dExpeditionRoles)
		dExpeditionLevels,db_err:=this.ExpeditionLevels.save()
		if db_err!=nil{
			log.Error("insert save ExpeditionLevel failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dExpeditionLevels)
		dExpeditionLevelRole0s,db_err:=this.ExpeditionLevelRole0s.save()
		if db_err!=nil{
			log.Error("insert save ExpeditionLevelRole0 failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dExpeditionLevelRole0s)
		dExpeditionLevelRole1s,db_err:=this.ExpeditionLevelRole1s.save()
		if db_err!=nil{
			log.Error("insert save ExpeditionLevelRole1 failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dExpeditionLevelRole1s)
		dExpeditionLevelRole2s,db_err:=this.ExpeditionLevelRole2s.save()
		if db_err!=nil{
			log.Error("insert save ExpeditionLevelRole2 failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dExpeditionLevelRole2s)
		dExpeditionLevelRole3s,db_err:=this.ExpeditionLevelRole3s.save()
		if db_err!=nil{
			log.Error("insert save ExpeditionLevelRole3 failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dExpeditionLevelRole3s)
		dExpeditionLevelRole4s,db_err:=this.ExpeditionLevelRole4s.save()
		if db_err!=nil{
			log.Error("insert save ExpeditionLevelRole4 failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dExpeditionLevelRole4s)
		dExpeditionLevelRole5s,db_err:=this.ExpeditionLevelRole5s.save()
		if db_err!=nil{
			log.Error("insert save ExpeditionLevelRole5 failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dExpeditionLevelRole5s)
		dExpeditionLevelRole6s,db_err:=this.ExpeditionLevelRole6s.save()
		if db_err!=nil{
			log.Error("insert save ExpeditionLevelRole6 failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dExpeditionLevelRole6s)
		dExpeditionLevelRole7s,db_err:=this.ExpeditionLevelRole7s.save()
		if db_err!=nil{
			log.Error("insert save ExpeditionLevelRole7 failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dExpeditionLevelRole7s)
		dExpeditionLevelRole8s,db_err:=this.ExpeditionLevelRole8s.save()
		if db_err!=nil{
			log.Error("insert save ExpeditionLevelRole8 failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dExpeditionLevelRole8s)
		dExpeditionLevelRole9s,db_err:=this.ExpeditionLevelRole9s.save()
		if db_err!=nil{
			log.Error("insert save ExpeditionLevelRole9 failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dExpeditionLevelRole9s)
		dSysMail,db_err:=this.SysMail.save()
		if db_err!=nil{
			log.Error("insert save SysMail failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dSysMail)
		dArtifacts,db_err:=this.Artifacts.save()
		if db_err!=nil{
			log.Error("insert save Artifact failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dArtifacts)
		dCarnivalCommon,db_err:=this.CarnivalCommon.save()
		if db_err!=nil{
			log.Error("insert save CarnivalCommon failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dCarnivalCommon)
		dCarnivals,db_err:=this.Carnivals.save()
		if db_err!=nil{
			log.Error("insert save Carnival failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dCarnivals)
		dInviteCodess,db_err:=this.InviteCodess.save()
		if db_err!=nil{
			log.Error("insert save InviteCodes failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dInviteCodess)
		args=db_args.GetArgs()
		state = 1
	} else {
		if this.m_UniqueId_changed||this.m_Account_changed||this.m_Name_changed||this.m_Token_changed||this.m_CurrReplyMsgNum_changed||this.Info.m_changed||this.Global.m_changed||this.m_Level_changed||this.Items.m_changed||this.RoleCommon.m_changed||this.Roles.m_changed||this.RoleHandbook.m_changed||this.BattleTeam.m_changed||this.CampaignCommon.m_changed||this.Campaigns.m_changed||this.CampaignStaticIncomes.m_changed||this.CampaignRandomIncomes.m_changed||this.MailCommon.m_changed||this.Mails.m_changed||this.BattleSaves.m_changed||this.Talents.m_changed||this.TowerCommon.m_changed||this.Towers.m_changed||this.Draws.m_changed||this.GoldHand.m_changed||this.Shops.m_changed||this.ShopItems.m_changed||this.Arena.m_changed||this.Equip.m_changed||this.ActiveStageCommon.m_changed||this.ActiveStages.m_changed||this.FriendCommon.m_changed||this.Friends.m_changed||this.FriendRecommends.m_changed||this.FriendAsks.m_changed||this.FriendBosss.m_changed||this.TaskCommon.m_changed||this.Tasks.m_changed||this.FinishedTasks.m_changed||this.DailyTaskAllDailys.m_changed||this.ExploreCommon.m_changed||this.Explores.m_changed||this.ExploreStorys.m_changed||this.FriendChatUnreadIds.m_changed||this.FriendChatUnreadMessages.m_changed||this.HeadItems.m_changed||this.SuitAwards.m_changed||this.Chats.m_changed||this.Anouncement.m_changed||this.FirstDrawCards.m_changed||this.Guild.m_changed||this.GuildStage.m_changed||this.Sign.m_changed||this.SevenDays.m_changed||this.PayCommon.m_changed||this.Pays.m_changed||this.GuideData.m_changed||this.ActivityDatas.m_changed||this.ExpeditionData.m_changed||this.ExpeditionRoles.m_changed||this.ExpeditionLevels.m_changed||this.ExpeditionLevelRole0s.m_changed||this.ExpeditionLevelRole1s.m_changed||this.ExpeditionLevelRole2s.m_changed||this.ExpeditionLevelRole3s.m_changed||this.ExpeditionLevelRole4s.m_changed||this.ExpeditionLevelRole5s.m_changed||this.ExpeditionLevelRole6s.m_changed||this.ExpeditionLevelRole7s.m_changed||this.ExpeditionLevelRole8s.m_changed||this.ExpeditionLevelRole9s.m_changed||this.SysMail.m_changed||this.Artifacts.m_changed||this.CarnivalCommon.m_changed||this.Carnivals.m_changed||this.InviteCodess.m_changed{
			update_string = "UPDATE Players SET "
			db_args:=new_db_args(77)
			if this.m_UniqueId_changed{
				update_string+="UniqueId=?,"
				db_args.Push(this.m_UniqueId)
			}
			if this.m_Account_changed{
				update_string+="Account=?,"
				db_args.Push(this.m_Account)
			}
			if this.m_Name_changed{
				update_string+="Name=?,"
				db_args.Push(this.m_Name)
			}
			if this.m_Token_changed{
				update_string+="Token=?,"
				db_args.Push(this.m_Token)
			}
			if this.m_CurrReplyMsgNum_changed{
				update_string+="CurrReplyMsgNum=?,"
				db_args.Push(this.m_CurrReplyMsgNum)
			}
			if this.Info.m_changed{
				update_string+="Info=?,"
				dInfo,err:=this.Info.save()
				if err!=nil{
					log.Error("update save Info failed")
					return err,false,0,"",nil
				}
				db_args.Push(dInfo)
			}
			if this.Global.m_changed{
				update_string+="Global=?,"
				dGlobal,err:=this.Global.save()
				if err!=nil{
					log.Error("update save Global failed")
					return err,false,0,"",nil
				}
				db_args.Push(dGlobal)
			}
			if this.m_Level_changed{
				update_string+="Level=?,"
				db_args.Push(this.m_Level)
			}
			if this.Items.m_changed{
				update_string+="Items=?,"
				dItems,err:=this.Items.save()
				if err!=nil{
					log.Error("insert save Item failed")
					return err,false,0,"",nil
				}
				db_args.Push(dItems)
			}
			if this.RoleCommon.m_changed{
				update_string+="RoleCommon=?,"
				dRoleCommon,err:=this.RoleCommon.save()
				if err!=nil{
					log.Error("update save RoleCommon failed")
					return err,false,0,"",nil
				}
				db_args.Push(dRoleCommon)
			}
			if this.Roles.m_changed{
				update_string+="Roles=?,"
				dRoles,err:=this.Roles.save()
				if err!=nil{
					log.Error("insert save Role failed")
					return err,false,0,"",nil
				}
				db_args.Push(dRoles)
			}
			if this.RoleHandbook.m_changed{
				update_string+="RoleHandbook=?,"
				dRoleHandbook,err:=this.RoleHandbook.save()
				if err!=nil{
					log.Error("update save RoleHandbook failed")
					return err,false,0,"",nil
				}
				db_args.Push(dRoleHandbook)
			}
			if this.BattleTeam.m_changed{
				update_string+="BattleTeam=?,"
				dBattleTeam,err:=this.BattleTeam.save()
				if err!=nil{
					log.Error("update save BattleTeam failed")
					return err,false,0,"",nil
				}
				db_args.Push(dBattleTeam)
			}
			if this.CampaignCommon.m_changed{
				update_string+="CampaignCommon=?,"
				dCampaignCommon,err:=this.CampaignCommon.save()
				if err!=nil{
					log.Error("update save CampaignCommon failed")
					return err,false,0,"",nil
				}
				db_args.Push(dCampaignCommon)
			}
			if this.Campaigns.m_changed{
				update_string+="Campaigns=?,"
				dCampaigns,err:=this.Campaigns.save()
				if err!=nil{
					log.Error("insert save Campaign failed")
					return err,false,0,"",nil
				}
				db_args.Push(dCampaigns)
			}
			if this.CampaignStaticIncomes.m_changed{
				update_string+="CampaignStaticIncomes=?,"
				dCampaignStaticIncomes,err:=this.CampaignStaticIncomes.save()
				if err!=nil{
					log.Error("insert save CampaignStaticIncome failed")
					return err,false,0,"",nil
				}
				db_args.Push(dCampaignStaticIncomes)
			}
			if this.CampaignRandomIncomes.m_changed{
				update_string+="CampaignRandomIncomes=?,"
				dCampaignRandomIncomes,err:=this.CampaignRandomIncomes.save()
				if err!=nil{
					log.Error("insert save CampaignRandomIncome failed")
					return err,false,0,"",nil
				}
				db_args.Push(dCampaignRandomIncomes)
			}
			if this.MailCommon.m_changed{
				update_string+="MailCommon=?,"
				dMailCommon,err:=this.MailCommon.save()
				if err!=nil{
					log.Error("update save MailCommon failed")
					return err,false,0,"",nil
				}
				db_args.Push(dMailCommon)
			}
			if this.Mails.m_changed{
				update_string+="Mails=?,"
				dMails,err:=this.Mails.save()
				if err!=nil{
					log.Error("insert save Mail failed")
					return err,false,0,"",nil
				}
				db_args.Push(dMails)
			}
			if this.BattleSaves.m_changed{
				update_string+="BattleSaves=?,"
				dBattleSaves,err:=this.BattleSaves.save()
				if err!=nil{
					log.Error("insert save BattleSave failed")
					return err,false,0,"",nil
				}
				db_args.Push(dBattleSaves)
			}
			if this.Talents.m_changed{
				update_string+="Talents=?,"
				dTalents,err:=this.Talents.save()
				if err!=nil{
					log.Error("insert save Talent failed")
					return err,false,0,"",nil
				}
				db_args.Push(dTalents)
			}
			if this.TowerCommon.m_changed{
				update_string+="TowerCommon=?,"
				dTowerCommon,err:=this.TowerCommon.save()
				if err!=nil{
					log.Error("update save TowerCommon failed")
					return err,false,0,"",nil
				}
				db_args.Push(dTowerCommon)
			}
			if this.Towers.m_changed{
				update_string+="Towers=?,"
				dTowers,err:=this.Towers.save()
				if err!=nil{
					log.Error("insert save Tower failed")
					return err,false,0,"",nil
				}
				db_args.Push(dTowers)
			}
			if this.Draws.m_changed{
				update_string+="Draws=?,"
				dDraws,err:=this.Draws.save()
				if err!=nil{
					log.Error("insert save Draw failed")
					return err,false,0,"",nil
				}
				db_args.Push(dDraws)
			}
			if this.GoldHand.m_changed{
				update_string+="GoldHand=?,"
				dGoldHand,err:=this.GoldHand.save()
				if err!=nil{
					log.Error("update save GoldHand failed")
					return err,false,0,"",nil
				}
				db_args.Push(dGoldHand)
			}
			if this.Shops.m_changed{
				update_string+="Shops=?,"
				dShops,err:=this.Shops.save()
				if err!=nil{
					log.Error("insert save Shop failed")
					return err,false,0,"",nil
				}
				db_args.Push(dShops)
			}
			if this.ShopItems.m_changed{
				update_string+="ShopItems=?,"
				dShopItems,err:=this.ShopItems.save()
				if err!=nil{
					log.Error("insert save ShopItem failed")
					return err,false,0,"",nil
				}
				db_args.Push(dShopItems)
			}
			if this.Arena.m_changed{
				update_string+="Arena=?,"
				dArena,err:=this.Arena.save()
				if err!=nil{
					log.Error("update save Arena failed")
					return err,false,0,"",nil
				}
				db_args.Push(dArena)
			}
			if this.Equip.m_changed{
				update_string+="Equip=?,"
				dEquip,err:=this.Equip.save()
				if err!=nil{
					log.Error("update save Equip failed")
					return err,false,0,"",nil
				}
				db_args.Push(dEquip)
			}
			if this.ActiveStageCommon.m_changed{
				update_string+="ActiveStageCommon=?,"
				dActiveStageCommon,err:=this.ActiveStageCommon.save()
				if err!=nil{
					log.Error("update save ActiveStageCommon failed")
					return err,false,0,"",nil
				}
				db_args.Push(dActiveStageCommon)
			}
			if this.ActiveStages.m_changed{
				update_string+="ActiveStages=?,"
				dActiveStages,err:=this.ActiveStages.save()
				if err!=nil{
					log.Error("insert save ActiveStage failed")
					return err,false,0,"",nil
				}
				db_args.Push(dActiveStages)
			}
			if this.FriendCommon.m_changed{
				update_string+="FriendCommon=?,"
				dFriendCommon,err:=this.FriendCommon.save()
				if err!=nil{
					log.Error("update save FriendCommon failed")
					return err,false,0,"",nil
				}
				db_args.Push(dFriendCommon)
			}
			if this.Friends.m_changed{
				update_string+="Friends=?,"
				dFriends,err:=this.Friends.save()
				if err!=nil{
					log.Error("insert save Friend failed")
					return err,false,0,"",nil
				}
				db_args.Push(dFriends)
			}
			if this.FriendRecommends.m_changed{
				update_string+="FriendRecommends=?,"
				dFriendRecommends,err:=this.FriendRecommends.save()
				if err!=nil{
					log.Error("insert save FriendRecommend failed")
					return err,false,0,"",nil
				}
				db_args.Push(dFriendRecommends)
			}
			if this.FriendAsks.m_changed{
				update_string+="FriendAsks=?,"
				dFriendAsks,err:=this.FriendAsks.save()
				if err!=nil{
					log.Error("insert save FriendAsk failed")
					return err,false,0,"",nil
				}
				db_args.Push(dFriendAsks)
			}
			if this.FriendBosss.m_changed{
				update_string+="FriendBosss=?,"
				dFriendBosss,err:=this.FriendBosss.save()
				if err!=nil{
					log.Error("insert save FriendBoss failed")
					return err,false,0,"",nil
				}
				db_args.Push(dFriendBosss)
			}
			if this.TaskCommon.m_changed{
				update_string+="TaskCommon=?,"
				dTaskCommon,err:=this.TaskCommon.save()
				if err!=nil{
					log.Error("update save TaskCommon failed")
					return err,false,0,"",nil
				}
				db_args.Push(dTaskCommon)
			}
			if this.Tasks.m_changed{
				update_string+="Tasks=?,"
				dTasks,err:=this.Tasks.save()
				if err!=nil{
					log.Error("insert save Task failed")
					return err,false,0,"",nil
				}
				db_args.Push(dTasks)
			}
			if this.FinishedTasks.m_changed{
				update_string+="FinishedTasks=?,"
				dFinishedTasks,err:=this.FinishedTasks.save()
				if err!=nil{
					log.Error("insert save FinishedTask failed")
					return err,false,0,"",nil
				}
				db_args.Push(dFinishedTasks)
			}
			if this.DailyTaskAllDailys.m_changed{
				update_string+="DailyTaskAllDailys=?,"
				dDailyTaskAllDailys,err:=this.DailyTaskAllDailys.save()
				if err!=nil{
					log.Error("insert save DailyTaskAllDaily failed")
					return err,false,0,"",nil
				}
				db_args.Push(dDailyTaskAllDailys)
			}
			if this.ExploreCommon.m_changed{
				update_string+="ExploreCommon=?,"
				dExploreCommon,err:=this.ExploreCommon.save()
				if err!=nil{
					log.Error("update save ExploreCommon failed")
					return err,false,0,"",nil
				}
				db_args.Push(dExploreCommon)
			}
			if this.Explores.m_changed{
				update_string+="Explores=?,"
				dExplores,err:=this.Explores.save()
				if err!=nil{
					log.Error("insert save Explore failed")
					return err,false,0,"",nil
				}
				db_args.Push(dExplores)
			}
			if this.ExploreStorys.m_changed{
				update_string+="ExploreStorys=?,"
				dExploreStorys,err:=this.ExploreStorys.save()
				if err!=nil{
					log.Error("insert save ExploreStory failed")
					return err,false,0,"",nil
				}
				db_args.Push(dExploreStorys)
			}
			if this.FriendChatUnreadIds.m_changed{
				update_string+="FriendChatUnreadIds=?,"
				dFriendChatUnreadIds,err:=this.FriendChatUnreadIds.save()
				if err!=nil{
					log.Error("insert save FriendChatUnreadId failed")
					return err,false,0,"",nil
				}
				db_args.Push(dFriendChatUnreadIds)
			}
			if this.FriendChatUnreadMessages.m_changed{
				update_string+="FriendChatUnreadMessages=?,"
				dFriendChatUnreadMessages,err:=this.FriendChatUnreadMessages.save()
				if err!=nil{
					log.Error("insert save FriendChatUnreadMessage failed")
					return err,false,0,"",nil
				}
				db_args.Push(dFriendChatUnreadMessages)
			}
			if this.HeadItems.m_changed{
				update_string+="HeadItems=?,"
				dHeadItems,err:=this.HeadItems.save()
				if err!=nil{
					log.Error("insert save HeadItem failed")
					return err,false,0,"",nil
				}
				db_args.Push(dHeadItems)
			}
			if this.SuitAwards.m_changed{
				update_string+="SuitAwards=?,"
				dSuitAwards,err:=this.SuitAwards.save()
				if err!=nil{
					log.Error("insert save SuitAward failed")
					return err,false,0,"",nil
				}
				db_args.Push(dSuitAwards)
			}
			if this.Chats.m_changed{
				update_string+="Chats=?,"
				dChats,err:=this.Chats.save()
				if err!=nil{
					log.Error("insert save Chat failed")
					return err,false,0,"",nil
				}
				db_args.Push(dChats)
			}
			if this.Anouncement.m_changed{
				update_string+="Anouncement=?,"
				dAnouncement,err:=this.Anouncement.save()
				if err!=nil{
					log.Error("update save Anouncement failed")
					return err,false,0,"",nil
				}
				db_args.Push(dAnouncement)
			}
			if this.FirstDrawCards.m_changed{
				update_string+="FirstDrawCards=?,"
				dFirstDrawCards,err:=this.FirstDrawCards.save()
				if err!=nil{
					log.Error("insert save FirstDrawCard failed")
					return err,false,0,"",nil
				}
				db_args.Push(dFirstDrawCards)
			}
			if this.Guild.m_changed{
				update_string+="Guild=?,"
				dGuild,err:=this.Guild.save()
				if err!=nil{
					log.Error("update save Guild failed")
					return err,false,0,"",nil
				}
				db_args.Push(dGuild)
			}
			if this.GuildStage.m_changed{
				update_string+="GuildStage=?,"
				dGuildStage,err:=this.GuildStage.save()
				if err!=nil{
					log.Error("update save GuildStage failed")
					return err,false,0,"",nil
				}
				db_args.Push(dGuildStage)
			}
			if this.Sign.m_changed{
				update_string+="Sign=?,"
				dSign,err:=this.Sign.save()
				if err!=nil{
					log.Error("update save Sign failed")
					return err,false,0,"",nil
				}
				db_args.Push(dSign)
			}
			if this.SevenDays.m_changed{
				update_string+="SevenDays=?,"
				dSevenDays,err:=this.SevenDays.save()
				if err!=nil{
					log.Error("update save SevenDays failed")
					return err,false,0,"",nil
				}
				db_args.Push(dSevenDays)
			}
			if this.PayCommon.m_changed{
				update_string+="PayCommon=?,"
				dPayCommon,err:=this.PayCommon.save()
				if err!=nil{
					log.Error("update save PayCommon failed")
					return err,false,0,"",nil
				}
				db_args.Push(dPayCommon)
			}
			if this.Pays.m_changed{
				update_string+="Pays=?,"
				dPays,err:=this.Pays.save()
				if err!=nil{
					log.Error("insert save Pay failed")
					return err,false,0,"",nil
				}
				db_args.Push(dPays)
			}
			if this.GuideData.m_changed{
				update_string+="GuideData=?,"
				dGuideData,err:=this.GuideData.save()
				if err!=nil{
					log.Error("update save GuideData failed")
					return err,false,0,"",nil
				}
				db_args.Push(dGuideData)
			}
			if this.ActivityDatas.m_changed{
				update_string+="ActivityDatas=?,"
				dActivityDatas,err:=this.ActivityDatas.save()
				if err!=nil{
					log.Error("insert save ActivityData failed")
					return err,false,0,"",nil
				}
				db_args.Push(dActivityDatas)
			}
			if this.ExpeditionData.m_changed{
				update_string+="ExpeditionData=?,"
				dExpeditionData,err:=this.ExpeditionData.save()
				if err!=nil{
					log.Error("update save ExpeditionData failed")
					return err,false,0,"",nil
				}
				db_args.Push(dExpeditionData)
			}
			if this.ExpeditionRoles.m_changed{
				update_string+="ExpeditionRoles=?,"
				dExpeditionRoles,err:=this.ExpeditionRoles.save()
				if err!=nil{
					log.Error("insert save ExpeditionRole failed")
					return err,false,0,"",nil
				}
				db_args.Push(dExpeditionRoles)
			}
			if this.ExpeditionLevels.m_changed{
				update_string+="ExpeditionLevels=?,"
				dExpeditionLevels,err:=this.ExpeditionLevels.save()
				if err!=nil{
					log.Error("insert save ExpeditionLevel failed")
					return err,false,0,"",nil
				}
				db_args.Push(dExpeditionLevels)
			}
			if this.ExpeditionLevelRole0s.m_changed{
				update_string+="ExpeditionLevelRole0s=?,"
				dExpeditionLevelRole0s,err:=this.ExpeditionLevelRole0s.save()
				if err!=nil{
					log.Error("insert save ExpeditionLevelRole0 failed")
					return err,false,0,"",nil
				}
				db_args.Push(dExpeditionLevelRole0s)
			}
			if this.ExpeditionLevelRole1s.m_changed{
				update_string+="ExpeditionLevelRole1s=?,"
				dExpeditionLevelRole1s,err:=this.ExpeditionLevelRole1s.save()
				if err!=nil{
					log.Error("insert save ExpeditionLevelRole1 failed")
					return err,false,0,"",nil
				}
				db_args.Push(dExpeditionLevelRole1s)
			}
			if this.ExpeditionLevelRole2s.m_changed{
				update_string+="ExpeditionLevelRole2s=?,"
				dExpeditionLevelRole2s,err:=this.ExpeditionLevelRole2s.save()
				if err!=nil{
					log.Error("insert save ExpeditionLevelRole2 failed")
					return err,false,0,"",nil
				}
				db_args.Push(dExpeditionLevelRole2s)
			}
			if this.ExpeditionLevelRole3s.m_changed{
				update_string+="ExpeditionLevelRole3s=?,"
				dExpeditionLevelRole3s,err:=this.ExpeditionLevelRole3s.save()
				if err!=nil{
					log.Error("insert save ExpeditionLevelRole3 failed")
					return err,false,0,"",nil
				}
				db_args.Push(dExpeditionLevelRole3s)
			}
			if this.ExpeditionLevelRole4s.m_changed{
				update_string+="ExpeditionLevelRole4s=?,"
				dExpeditionLevelRole4s,err:=this.ExpeditionLevelRole4s.save()
				if err!=nil{
					log.Error("insert save ExpeditionLevelRole4 failed")
					return err,false,0,"",nil
				}
				db_args.Push(dExpeditionLevelRole4s)
			}
			if this.ExpeditionLevelRole5s.m_changed{
				update_string+="ExpeditionLevelRole5s=?,"
				dExpeditionLevelRole5s,err:=this.ExpeditionLevelRole5s.save()
				if err!=nil{
					log.Error("insert save ExpeditionLevelRole5 failed")
					return err,false,0,"",nil
				}
				db_args.Push(dExpeditionLevelRole5s)
			}
			if this.ExpeditionLevelRole6s.m_changed{
				update_string+="ExpeditionLevelRole6s=?,"
				dExpeditionLevelRole6s,err:=this.ExpeditionLevelRole6s.save()
				if err!=nil{
					log.Error("insert save ExpeditionLevelRole6 failed")
					return err,false,0,"",nil
				}
				db_args.Push(dExpeditionLevelRole6s)
			}
			if this.ExpeditionLevelRole7s.m_changed{
				update_string+="ExpeditionLevelRole7s=?,"
				dExpeditionLevelRole7s,err:=this.ExpeditionLevelRole7s.save()
				if err!=nil{
					log.Error("insert save ExpeditionLevelRole7 failed")
					return err,false,0,"",nil
				}
				db_args.Push(dExpeditionLevelRole7s)
			}
			if this.ExpeditionLevelRole8s.m_changed{
				update_string+="ExpeditionLevelRole8s=?,"
				dExpeditionLevelRole8s,err:=this.ExpeditionLevelRole8s.save()
				if err!=nil{
					log.Error("insert save ExpeditionLevelRole8 failed")
					return err,false,0,"",nil
				}
				db_args.Push(dExpeditionLevelRole8s)
			}
			if this.ExpeditionLevelRole9s.m_changed{
				update_string+="ExpeditionLevelRole9s=?,"
				dExpeditionLevelRole9s,err:=this.ExpeditionLevelRole9s.save()
				if err!=nil{
					log.Error("insert save ExpeditionLevelRole9 failed")
					return err,false,0,"",nil
				}
				db_args.Push(dExpeditionLevelRole9s)
			}
			if this.SysMail.m_changed{
				update_string+="SysMail=?,"
				dSysMail,err:=this.SysMail.save()
				if err!=nil{
					log.Error("update save SysMail failed")
					return err,false,0,"",nil
				}
				db_args.Push(dSysMail)
			}
			if this.Artifacts.m_changed{
				update_string+="Artifacts=?,"
				dArtifacts,err:=this.Artifacts.save()
				if err!=nil{
					log.Error("insert save Artifact failed")
					return err,false,0,"",nil
				}
				db_args.Push(dArtifacts)
			}
			if this.CarnivalCommon.m_changed{
				update_string+="CarnivalCommon=?,"
				dCarnivalCommon,err:=this.CarnivalCommon.save()
				if err!=nil{
					log.Error("update save CarnivalCommon failed")
					return err,false,0,"",nil
				}
				db_args.Push(dCarnivalCommon)
			}
			if this.Carnivals.m_changed{
				update_string+="Carnivals=?,"
				dCarnivals,err:=this.Carnivals.save()
				if err!=nil{
					log.Error("insert save Carnival failed")
					return err,false,0,"",nil
				}
				db_args.Push(dCarnivals)
			}
			if this.InviteCodess.m_changed{
				update_string+="InviteCodess=?,"
				dInviteCodess,err:=this.InviteCodess.save()
				if err!=nil{
					log.Error("insert save InviteCodes failed")
					return err,false,0,"",nil
				}
				db_args.Push(dInviteCodess)
			}
			update_string = strings.TrimRight(update_string, ", ")
			update_string+=" WHERE PlayerId=?"
			db_args.Push(this.m_PlayerId)
			args=db_args.GetArgs()
			state = 2
		}
	}
	this.m_new = false
	this.m_UniqueId_changed = false
	this.m_Account_changed = false
	this.m_Name_changed = false
	this.m_Token_changed = false
	this.m_CurrReplyMsgNum_changed = false
	this.Info.m_changed = false
	this.Global.m_changed = false
	this.m_Level_changed = false
	this.Items.m_changed = false
	this.RoleCommon.m_changed = false
	this.Roles.m_changed = false
	this.RoleHandbook.m_changed = false
	this.BattleTeam.m_changed = false
	this.CampaignCommon.m_changed = false
	this.Campaigns.m_changed = false
	this.CampaignStaticIncomes.m_changed = false
	this.CampaignRandomIncomes.m_changed = false
	this.MailCommon.m_changed = false
	this.Mails.m_changed = false
	this.BattleSaves.m_changed = false
	this.Talents.m_changed = false
	this.TowerCommon.m_changed = false
	this.Towers.m_changed = false
	this.Draws.m_changed = false
	this.GoldHand.m_changed = false
	this.Shops.m_changed = false
	this.ShopItems.m_changed = false
	this.Arena.m_changed = false
	this.Equip.m_changed = false
	this.ActiveStageCommon.m_changed = false
	this.ActiveStages.m_changed = false
	this.FriendCommon.m_changed = false
	this.Friends.m_changed = false
	this.FriendRecommends.m_changed = false
	this.FriendAsks.m_changed = false
	this.FriendBosss.m_changed = false
	this.TaskCommon.m_changed = false
	this.Tasks.m_changed = false
	this.FinishedTasks.m_changed = false
	this.DailyTaskAllDailys.m_changed = false
	this.ExploreCommon.m_changed = false
	this.Explores.m_changed = false
	this.ExploreStorys.m_changed = false
	this.FriendChatUnreadIds.m_changed = false
	this.FriendChatUnreadMessages.m_changed = false
	this.HeadItems.m_changed = false
	this.SuitAwards.m_changed = false
	this.Chats.m_changed = false
	this.Anouncement.m_changed = false
	this.FirstDrawCards.m_changed = false
	this.Guild.m_changed = false
	this.GuildStage.m_changed = false
	this.Sign.m_changed = false
	this.SevenDays.m_changed = false
	this.PayCommon.m_changed = false
	this.Pays.m_changed = false
	this.GuideData.m_changed = false
	this.ActivityDatas.m_changed = false
	this.ExpeditionData.m_changed = false
	this.ExpeditionRoles.m_changed = false
	this.ExpeditionLevels.m_changed = false
	this.ExpeditionLevelRole0s.m_changed = false
	this.ExpeditionLevelRole1s.m_changed = false
	this.ExpeditionLevelRole2s.m_changed = false
	this.ExpeditionLevelRole3s.m_changed = false
	this.ExpeditionLevelRole4s.m_changed = false
	this.ExpeditionLevelRole5s.m_changed = false
	this.ExpeditionLevelRole6s.m_changed = false
	this.ExpeditionLevelRole7s.m_changed = false
	this.ExpeditionLevelRole8s.m_changed = false
	this.ExpeditionLevelRole9s.m_changed = false
	this.SysMail.m_changed = false
	this.Artifacts.m_changed = false
	this.CarnivalCommon.m_changed = false
	this.Carnivals.m_changed = false
	this.InviteCodess.m_changed = false
	if release && this.m_loaded {
		atomic.AddInt32(&this.m_table.m_gc_n, -1)
		this.m_loaded = false
		released = true
	}
	return nil,released,state,update_string,args
}
func (this *dbPlayerRow) Save(release bool) (err error, d bool, released bool) {
	err,released, state, update_string, args := this.save_data(release)
	if err != nil {
		log.Error("save data failed")
		return err, false, false
	}
	if state == 0 {
		d = false
	} else if state == 1 {
		_, err = this.m_table.m_dbc.StmtExec(this.m_table.m_save_insert_stmt, args...)
		if err != nil {
			log.Error("INSERT Players exec failed %v ", this.m_PlayerId)
			return err, false, released
		}
		d = true
	} else if state == 2 {
		_, err = this.m_table.m_dbc.Exec(update_string, args...)
		if err != nil {
			log.Error("UPDATE Players exec failed %v", this.m_PlayerId)
			return err, false, released
		}
		d = true
	}
	return nil, d, released
}
func (this *dbPlayerRow) Touch(releasable bool) {
	this.m_touch = int32(time.Now().Unix())
	this.m_releasable = releasable
}
type dbPlayerRowSort struct {
	rows []*dbPlayerRow
}
func (this *dbPlayerRowSort) Len() (length int) {
	return len(this.rows)
}
func (this *dbPlayerRowSort) Less(i int, j int) (less bool) {
	return this.rows[i].m_touch < this.rows[j].m_touch
}
func (this *dbPlayerRowSort) Swap(i int, j int) {
	temp := this.rows[i]
	this.rows[i] = this.rows[j]
	this.rows[j] = temp
}
type dbPlayerTable struct{
	m_dbc *DBC
	m_lock *RWMutex
	m_rows map[int32]*dbPlayerRow
	m_new_rows map[int32]*dbPlayerRow
	m_removed_rows map[int32]*dbPlayerRow
	m_gc_n int32
	m_gcing int32
	m_pool_size int32
	m_preload_select_stmt *sql.Stmt
	m_preload_max_id int32
	m_save_insert_stmt *sql.Stmt
	m_delete_stmt *sql.Stmt
}
func new_dbPlayerTable(dbc *DBC) (this *dbPlayerTable) {
	this = &dbPlayerTable{}
	this.m_dbc = dbc
	this.m_lock = NewRWMutex()
	this.m_rows = make(map[int32]*dbPlayerRow)
	this.m_new_rows = make(map[int32]*dbPlayerRow)
	this.m_removed_rows = make(map[int32]*dbPlayerRow)
	return this
}
func (this *dbPlayerTable) check_create_table() (err error) {
	_, err = this.m_dbc.Exec("CREATE TABLE IF NOT EXISTS Players(PlayerId int(11),PRIMARY KEY (PlayerId))ENGINE=InnoDB ROW_FORMAT=DYNAMIC")
	if err != nil {
		log.Error("CREATE TABLE IF NOT EXISTS Players failed")
		return
	}
	rows, err := this.m_dbc.Query("SELECT COLUMN_NAME,ORDINAL_POSITION FROM information_schema.`COLUMNS` WHERE TABLE_SCHEMA=? AND TABLE_NAME='Players'", this.m_dbc.m_db_name)
	if err != nil {
		log.Error("SELECT information_schema failed")
		return
	}
	columns := make(map[string]int32)
	for rows.Next() {
		var column_name string
		var ordinal_position int32
		err = rows.Scan(&column_name, &ordinal_position)
		if err != nil {
			log.Error("scan information_schema row failed")
			return
		}
		if ordinal_position < 1 {
			log.Error("col ordinal out of range")
			continue
		}
		columns[column_name] = ordinal_position
	}
	_, hasUniqueId := columns["UniqueId"]
	if !hasUniqueId {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN UniqueId varchar(256) DEFAULT ''")
		if err != nil {
			log.Error("ADD COLUMN UniqueId failed")
			return
		}
	}
	_, hasAccount := columns["Account"]
	if !hasAccount {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Account varchar(256)")
		if err != nil {
			log.Error("ADD COLUMN Account failed")
			return
		}
	}
	_, hasName := columns["Name"]
	if !hasName {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Name varchar(256)")
		if err != nil {
			log.Error("ADD COLUMN Name failed")
			return
		}
	}
	_, hasToken := columns["Token"]
	if !hasToken {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Token varchar(256) DEFAULT ''")
		if err != nil {
			log.Error("ADD COLUMN Token failed")
			return
		}
	}
	_, hasCurrReplyMsgNum := columns["CurrReplyMsgNum"]
	if !hasCurrReplyMsgNum {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN CurrReplyMsgNum int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN CurrReplyMsgNum failed")
			return
		}
	}
	_, hasInfo := columns["Info"]
	if !hasInfo {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Info LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Info failed")
			return
		}
	}
	_, hasGlobal := columns["Global"]
	if !hasGlobal {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Global LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Global failed")
			return
		}
	}
	_, hasLevel := columns["Level"]
	if !hasLevel {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Level int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN Level failed")
			return
		}
	}
	_, hasItem := columns["Items"]
	if !hasItem {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Items LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Items failed")
			return
		}
	}
	_, hasRoleCommon := columns["RoleCommon"]
	if !hasRoleCommon {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN RoleCommon LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN RoleCommon failed")
			return
		}
	}
	_, hasRole := columns["Roles"]
	if !hasRole {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Roles LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Roles failed")
			return
		}
	}
	_, hasRoleHandbook := columns["RoleHandbook"]
	if !hasRoleHandbook {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN RoleHandbook LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN RoleHandbook failed")
			return
		}
	}
	_, hasBattleTeam := columns["BattleTeam"]
	if !hasBattleTeam {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN BattleTeam LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN BattleTeam failed")
			return
		}
	}
	_, hasCampaignCommon := columns["CampaignCommon"]
	if !hasCampaignCommon {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN CampaignCommon LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN CampaignCommon failed")
			return
		}
	}
	_, hasCampaign := columns["Campaigns"]
	if !hasCampaign {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Campaigns LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Campaigns failed")
			return
		}
	}
	_, hasCampaignStaticIncome := columns["CampaignStaticIncomes"]
	if !hasCampaignStaticIncome {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN CampaignStaticIncomes LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN CampaignStaticIncomes failed")
			return
		}
	}
	_, hasCampaignRandomIncome := columns["CampaignRandomIncomes"]
	if !hasCampaignRandomIncome {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN CampaignRandomIncomes LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN CampaignRandomIncomes failed")
			return
		}
	}
	_, hasMailCommon := columns["MailCommon"]
	if !hasMailCommon {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN MailCommon LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN MailCommon failed")
			return
		}
	}
	_, hasMail := columns["Mails"]
	if !hasMail {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Mails LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Mails failed")
			return
		}
	}
	_, hasBattleSave := columns["BattleSaves"]
	if !hasBattleSave {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN BattleSaves LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN BattleSaves failed")
			return
		}
	}
	_, hasTalent := columns["Talents"]
	if !hasTalent {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Talents LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Talents failed")
			return
		}
	}
	_, hasTowerCommon := columns["TowerCommon"]
	if !hasTowerCommon {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN TowerCommon LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN TowerCommon failed")
			return
		}
	}
	_, hasTower := columns["Towers"]
	if !hasTower {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Towers LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Towers failed")
			return
		}
	}
	_, hasDraw := columns["Draws"]
	if !hasDraw {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Draws LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Draws failed")
			return
		}
	}
	_, hasGoldHand := columns["GoldHand"]
	if !hasGoldHand {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN GoldHand LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN GoldHand failed")
			return
		}
	}
	_, hasShop := columns["Shops"]
	if !hasShop {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Shops LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Shops failed")
			return
		}
	}
	_, hasShopItem := columns["ShopItems"]
	if !hasShopItem {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN ShopItems LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN ShopItems failed")
			return
		}
	}
	_, hasArena := columns["Arena"]
	if !hasArena {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Arena LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Arena failed")
			return
		}
	}
	_, hasEquip := columns["Equip"]
	if !hasEquip {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Equip LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Equip failed")
			return
		}
	}
	_, hasActiveStageCommon := columns["ActiveStageCommon"]
	if !hasActiveStageCommon {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN ActiveStageCommon LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN ActiveStageCommon failed")
			return
		}
	}
	_, hasActiveStage := columns["ActiveStages"]
	if !hasActiveStage {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN ActiveStages LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN ActiveStages failed")
			return
		}
	}
	_, hasFriendCommon := columns["FriendCommon"]
	if !hasFriendCommon {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN FriendCommon LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN FriendCommon failed")
			return
		}
	}
	_, hasFriend := columns["Friends"]
	if !hasFriend {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Friends LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Friends failed")
			return
		}
	}
	_, hasFriendRecommend := columns["FriendRecommends"]
	if !hasFriendRecommend {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN FriendRecommends LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN FriendRecommends failed")
			return
		}
	}
	_, hasFriendAsk := columns["FriendAsks"]
	if !hasFriendAsk {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN FriendAsks LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN FriendAsks failed")
			return
		}
	}
	_, hasFriendBoss := columns["FriendBosss"]
	if !hasFriendBoss {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN FriendBosss LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN FriendBosss failed")
			return
		}
	}
	_, hasTaskCommon := columns["TaskCommon"]
	if !hasTaskCommon {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN TaskCommon LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN TaskCommon failed")
			return
		}
	}
	_, hasTask := columns["Tasks"]
	if !hasTask {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Tasks LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Tasks failed")
			return
		}
	}
	_, hasFinishedTask := columns["FinishedTasks"]
	if !hasFinishedTask {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN FinishedTasks LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN FinishedTasks failed")
			return
		}
	}
	_, hasDailyTaskAllDaily := columns["DailyTaskAllDailys"]
	if !hasDailyTaskAllDaily {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN DailyTaskAllDailys LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN DailyTaskAllDailys failed")
			return
		}
	}
	_, hasExploreCommon := columns["ExploreCommon"]
	if !hasExploreCommon {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN ExploreCommon LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN ExploreCommon failed")
			return
		}
	}
	_, hasExplore := columns["Explores"]
	if !hasExplore {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Explores LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Explores failed")
			return
		}
	}
	_, hasExploreStory := columns["ExploreStorys"]
	if !hasExploreStory {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN ExploreStorys LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN ExploreStorys failed")
			return
		}
	}
	_, hasFriendChatUnreadId := columns["FriendChatUnreadIds"]
	if !hasFriendChatUnreadId {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN FriendChatUnreadIds LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN FriendChatUnreadIds failed")
			return
		}
	}
	_, hasFriendChatUnreadMessage := columns["FriendChatUnreadMessages"]
	if !hasFriendChatUnreadMessage {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN FriendChatUnreadMessages LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN FriendChatUnreadMessages failed")
			return
		}
	}
	_, hasHeadItem := columns["HeadItems"]
	if !hasHeadItem {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN HeadItems LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN HeadItems failed")
			return
		}
	}
	_, hasSuitAward := columns["SuitAwards"]
	if !hasSuitAward {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN SuitAwards LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN SuitAwards failed")
			return
		}
	}
	_, hasChat := columns["Chats"]
	if !hasChat {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Chats LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Chats failed")
			return
		}
	}
	_, hasAnouncement := columns["Anouncement"]
	if !hasAnouncement {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Anouncement LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Anouncement failed")
			return
		}
	}
	_, hasFirstDrawCard := columns["FirstDrawCards"]
	if !hasFirstDrawCard {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN FirstDrawCards LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN FirstDrawCards failed")
			return
		}
	}
	_, hasGuild := columns["Guild"]
	if !hasGuild {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Guild LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Guild failed")
			return
		}
	}
	_, hasGuildStage := columns["GuildStage"]
	if !hasGuildStage {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN GuildStage LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN GuildStage failed")
			return
		}
	}
	_, hasSign := columns["Sign"]
	if !hasSign {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Sign LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Sign failed")
			return
		}
	}
	_, hasSevenDays := columns["SevenDays"]
	if !hasSevenDays {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN SevenDays LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN SevenDays failed")
			return
		}
	}
	_, hasPayCommon := columns["PayCommon"]
	if !hasPayCommon {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN PayCommon LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN PayCommon failed")
			return
		}
	}
	_, hasPay := columns["Pays"]
	if !hasPay {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Pays LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Pays failed")
			return
		}
	}
	_, hasGuideData := columns["GuideData"]
	if !hasGuideData {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN GuideData LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN GuideData failed")
			return
		}
	}
	_, hasActivityData := columns["ActivityDatas"]
	if !hasActivityData {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN ActivityDatas LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN ActivityDatas failed")
			return
		}
	}
	_, hasExpeditionData := columns["ExpeditionData"]
	if !hasExpeditionData {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN ExpeditionData LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN ExpeditionData failed")
			return
		}
	}
	_, hasExpeditionRole := columns["ExpeditionRoles"]
	if !hasExpeditionRole {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN ExpeditionRoles LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN ExpeditionRoles failed")
			return
		}
	}
	_, hasExpeditionLevel := columns["ExpeditionLevels"]
	if !hasExpeditionLevel {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN ExpeditionLevels LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN ExpeditionLevels failed")
			return
		}
	}
	_, hasExpeditionLevelRole0 := columns["ExpeditionLevelRole0s"]
	if !hasExpeditionLevelRole0 {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN ExpeditionLevelRole0s LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN ExpeditionLevelRole0s failed")
			return
		}
	}
	_, hasExpeditionLevelRole1 := columns["ExpeditionLevelRole1s"]
	if !hasExpeditionLevelRole1 {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN ExpeditionLevelRole1s LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN ExpeditionLevelRole1s failed")
			return
		}
	}
	_, hasExpeditionLevelRole2 := columns["ExpeditionLevelRole2s"]
	if !hasExpeditionLevelRole2 {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN ExpeditionLevelRole2s LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN ExpeditionLevelRole2s failed")
			return
		}
	}
	_, hasExpeditionLevelRole3 := columns["ExpeditionLevelRole3s"]
	if !hasExpeditionLevelRole3 {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN ExpeditionLevelRole3s LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN ExpeditionLevelRole3s failed")
			return
		}
	}
	_, hasExpeditionLevelRole4 := columns["ExpeditionLevelRole4s"]
	if !hasExpeditionLevelRole4 {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN ExpeditionLevelRole4s LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN ExpeditionLevelRole4s failed")
			return
		}
	}
	_, hasExpeditionLevelRole5 := columns["ExpeditionLevelRole5s"]
	if !hasExpeditionLevelRole5 {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN ExpeditionLevelRole5s LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN ExpeditionLevelRole5s failed")
			return
		}
	}
	_, hasExpeditionLevelRole6 := columns["ExpeditionLevelRole6s"]
	if !hasExpeditionLevelRole6 {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN ExpeditionLevelRole6s LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN ExpeditionLevelRole6s failed")
			return
		}
	}
	_, hasExpeditionLevelRole7 := columns["ExpeditionLevelRole7s"]
	if !hasExpeditionLevelRole7 {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN ExpeditionLevelRole7s LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN ExpeditionLevelRole7s failed")
			return
		}
	}
	_, hasExpeditionLevelRole8 := columns["ExpeditionLevelRole8s"]
	if !hasExpeditionLevelRole8 {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN ExpeditionLevelRole8s LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN ExpeditionLevelRole8s failed")
			return
		}
	}
	_, hasExpeditionLevelRole9 := columns["ExpeditionLevelRole9s"]
	if !hasExpeditionLevelRole9 {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN ExpeditionLevelRole9s LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN ExpeditionLevelRole9s failed")
			return
		}
	}
	_, hasSysMail := columns["SysMail"]
	if !hasSysMail {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN SysMail LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN SysMail failed")
			return
		}
	}
	_, hasArtifact := columns["Artifacts"]
	if !hasArtifact {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Artifacts LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Artifacts failed")
			return
		}
	}
	_, hasCarnivalCommon := columns["CarnivalCommon"]
	if !hasCarnivalCommon {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN CarnivalCommon LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN CarnivalCommon failed")
			return
		}
	}
	_, hasCarnival := columns["Carnivals"]
	if !hasCarnival {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Carnivals LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Carnivals failed")
			return
		}
	}
	_, hasInviteCodes := columns["InviteCodess"]
	if !hasInviteCodes {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN InviteCodess LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN InviteCodess failed")
			return
		}
	}
	return
}
func (this *dbPlayerTable) prepare_preload_select_stmt() (err error) {
	this.m_preload_select_stmt,err=this.m_dbc.StmtPrepare("SELECT PlayerId,UniqueId,Account,Name,Token,CurrReplyMsgNum,Info,Global,Level,Items,RoleCommon,Roles,RoleHandbook,BattleTeam,CampaignCommon,Campaigns,CampaignStaticIncomes,CampaignRandomIncomes,MailCommon,Mails,BattleSaves,Talents,TowerCommon,Towers,Draws,GoldHand,Shops,ShopItems,Arena,Equip,ActiveStageCommon,ActiveStages,FriendCommon,Friends,FriendRecommends,FriendAsks,FriendBosss,TaskCommon,Tasks,FinishedTasks,DailyTaskAllDailys,ExploreCommon,Explores,ExploreStorys,FriendChatUnreadIds,FriendChatUnreadMessages,HeadItems,SuitAwards,Chats,Anouncement,FirstDrawCards,Guild,GuildStage,Sign,SevenDays,PayCommon,Pays,GuideData,ActivityDatas,ExpeditionData,ExpeditionRoles,ExpeditionLevels,ExpeditionLevelRole0s,ExpeditionLevelRole1s,ExpeditionLevelRole2s,ExpeditionLevelRole3s,ExpeditionLevelRole4s,ExpeditionLevelRole5s,ExpeditionLevelRole6s,ExpeditionLevelRole7s,ExpeditionLevelRole8s,ExpeditionLevelRole9s,SysMail,Artifacts,CarnivalCommon,Carnivals,InviteCodess FROM Players")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbPlayerTable) prepare_save_insert_stmt()(err error){
	this.m_save_insert_stmt,err=this.m_dbc.StmtPrepare("INSERT INTO Players (PlayerId,UniqueId,Account,Name,Token,CurrReplyMsgNum,Info,Global,Level,Items,RoleCommon,Roles,RoleHandbook,BattleTeam,CampaignCommon,Campaigns,CampaignStaticIncomes,CampaignRandomIncomes,MailCommon,Mails,BattleSaves,Talents,TowerCommon,Towers,Draws,GoldHand,Shops,ShopItems,Arena,Equip,ActiveStageCommon,ActiveStages,FriendCommon,Friends,FriendRecommends,FriendAsks,FriendBosss,TaskCommon,Tasks,FinishedTasks,DailyTaskAllDailys,ExploreCommon,Explores,ExploreStorys,FriendChatUnreadIds,FriendChatUnreadMessages,HeadItems,SuitAwards,Chats,Anouncement,FirstDrawCards,Guild,GuildStage,Sign,SevenDays,PayCommon,Pays,GuideData,ActivityDatas,ExpeditionData,ExpeditionRoles,ExpeditionLevels,ExpeditionLevelRole0s,ExpeditionLevelRole1s,ExpeditionLevelRole2s,ExpeditionLevelRole3s,ExpeditionLevelRole4s,ExpeditionLevelRole5s,ExpeditionLevelRole6s,ExpeditionLevelRole7s,ExpeditionLevelRole8s,ExpeditionLevelRole9s,SysMail,Artifacts,CarnivalCommon,Carnivals,InviteCodess) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbPlayerTable) prepare_delete_stmt() (err error) {
	this.m_delete_stmt,err=this.m_dbc.StmtPrepare("DELETE FROM Players WHERE PlayerId=?")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbPlayerTable) Init() (err error) {
	err=this.check_create_table()
	if err!=nil{
		log.Error("check_create_table failed")
		return
	}
	err=this.prepare_preload_select_stmt()
	if err!=nil{
		log.Error("prepare_preload_select_stmt failed")
		return
	}
	err=this.prepare_save_insert_stmt()
	if err!=nil{
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	err=this.prepare_delete_stmt()
	if err!=nil{
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	return
}
func (this *dbPlayerTable) Preload() (err error) {
	r, err := this.m_dbc.StmtQuery(this.m_preload_select_stmt)
	if err != nil {
		log.Error("SELECT")
		return
	}
	var PlayerId int32
	var dUniqueId string
	var dAccount string
	var dName string
	var dToken string
	var dCurrReplyMsgNum int32
	var dInfo []byte
	var dGlobal []byte
	var dLevel int32
	var dItems []byte
	var dRoleCommon []byte
	var dRoles []byte
	var dRoleHandbook []byte
	var dBattleTeam []byte
	var dCampaignCommon []byte
	var dCampaigns []byte
	var dCampaignStaticIncomes []byte
	var dCampaignRandomIncomes []byte
	var dMailCommon []byte
	var dMails []byte
	var dBattleSaves []byte
	var dTalents []byte
	var dTowerCommon []byte
	var dTowers []byte
	var dDraws []byte
	var dGoldHand []byte
	var dShops []byte
	var dShopItems []byte
	var dArena []byte
	var dEquip []byte
	var dActiveStageCommon []byte
	var dActiveStages []byte
	var dFriendCommon []byte
	var dFriends []byte
	var dFriendRecommends []byte
	var dFriendAsks []byte
	var dFriendBosss []byte
	var dTaskCommon []byte
	var dTasks []byte
	var dFinishedTasks []byte
	var dDailyTaskAllDailys []byte
	var dExploreCommon []byte
	var dExplores []byte
	var dExploreStorys []byte
	var dFriendChatUnreadIds []byte
	var dFriendChatUnreadMessages []byte
	var dHeadItems []byte
	var dSuitAwards []byte
	var dChats []byte
	var dAnouncement []byte
	var dFirstDrawCards []byte
	var dGuild []byte
	var dGuildStage []byte
	var dSign []byte
	var dSevenDays []byte
	var dPayCommon []byte
	var dPays []byte
	var dGuideData []byte
	var dActivityDatas []byte
	var dExpeditionData []byte
	var dExpeditionRoles []byte
	var dExpeditionLevels []byte
	var dExpeditionLevelRole0s []byte
	var dExpeditionLevelRole1s []byte
	var dExpeditionLevelRole2s []byte
	var dExpeditionLevelRole3s []byte
	var dExpeditionLevelRole4s []byte
	var dExpeditionLevelRole5s []byte
	var dExpeditionLevelRole6s []byte
	var dExpeditionLevelRole7s []byte
	var dExpeditionLevelRole8s []byte
	var dExpeditionLevelRole9s []byte
	var dSysMail []byte
	var dArtifacts []byte
	var dCarnivalCommon []byte
	var dCarnivals []byte
	var dInviteCodess []byte
		this.m_preload_max_id = 0
	for r.Next() {
		err = r.Scan(&PlayerId,&dUniqueId,&dAccount,&dName,&dToken,&dCurrReplyMsgNum,&dInfo,&dGlobal,&dLevel,&dItems,&dRoleCommon,&dRoles,&dRoleHandbook,&dBattleTeam,&dCampaignCommon,&dCampaigns,&dCampaignStaticIncomes,&dCampaignRandomIncomes,&dMailCommon,&dMails,&dBattleSaves,&dTalents,&dTowerCommon,&dTowers,&dDraws,&dGoldHand,&dShops,&dShopItems,&dArena,&dEquip,&dActiveStageCommon,&dActiveStages,&dFriendCommon,&dFriends,&dFriendRecommends,&dFriendAsks,&dFriendBosss,&dTaskCommon,&dTasks,&dFinishedTasks,&dDailyTaskAllDailys,&dExploreCommon,&dExplores,&dExploreStorys,&dFriendChatUnreadIds,&dFriendChatUnreadMessages,&dHeadItems,&dSuitAwards,&dChats,&dAnouncement,&dFirstDrawCards,&dGuild,&dGuildStage,&dSign,&dSevenDays,&dPayCommon,&dPays,&dGuideData,&dActivityDatas,&dExpeditionData,&dExpeditionRoles,&dExpeditionLevels,&dExpeditionLevelRole0s,&dExpeditionLevelRole1s,&dExpeditionLevelRole2s,&dExpeditionLevelRole3s,&dExpeditionLevelRole4s,&dExpeditionLevelRole5s,&dExpeditionLevelRole6s,&dExpeditionLevelRole7s,&dExpeditionLevelRole8s,&dExpeditionLevelRole9s,&dSysMail,&dArtifacts,&dCarnivalCommon,&dCarnivals,&dInviteCodess)
		if err != nil {
			log.Error("Scan err[%v]", err.Error())
			return
		}
		if PlayerId>this.m_preload_max_id{
			this.m_preload_max_id =PlayerId
		}
		row := new_dbPlayerRow(this,PlayerId)
		row.m_UniqueId=dUniqueId
		row.m_Account=dAccount
		row.m_Name=dName
		row.m_Token=dToken
		row.m_CurrReplyMsgNum=dCurrReplyMsgNum
		err = row.Info.load(dInfo)
		if err != nil {
			log.Error("Info %v", PlayerId)
			return
		}
		err = row.Global.load(dGlobal)
		if err != nil {
			log.Error("Global %v", PlayerId)
			return
		}
		row.m_Level=dLevel
		err = row.Items.load(dItems)
		if err != nil {
			log.Error("Items %v", PlayerId)
			return
		}
		err = row.RoleCommon.load(dRoleCommon)
		if err != nil {
			log.Error("RoleCommon %v", PlayerId)
			return
		}
		err = row.Roles.load(dRoles)
		if err != nil {
			log.Error("Roles %v", PlayerId)
			return
		}
		err = row.RoleHandbook.load(dRoleHandbook)
		if err != nil {
			log.Error("RoleHandbook %v", PlayerId)
			return
		}
		err = row.BattleTeam.load(dBattleTeam)
		if err != nil {
			log.Error("BattleTeam %v", PlayerId)
			return
		}
		err = row.CampaignCommon.load(dCampaignCommon)
		if err != nil {
			log.Error("CampaignCommon %v", PlayerId)
			return
		}
		err = row.Campaigns.load(dCampaigns)
		if err != nil {
			log.Error("Campaigns %v", PlayerId)
			return
		}
		err = row.CampaignStaticIncomes.load(dCampaignStaticIncomes)
		if err != nil {
			log.Error("CampaignStaticIncomes %v", PlayerId)
			return
		}
		err = row.CampaignRandomIncomes.load(dCampaignRandomIncomes)
		if err != nil {
			log.Error("CampaignRandomIncomes %v", PlayerId)
			return
		}
		err = row.MailCommon.load(dMailCommon)
		if err != nil {
			log.Error("MailCommon %v", PlayerId)
			return
		}
		err = row.Mails.load(dMails)
		if err != nil {
			log.Error("Mails %v", PlayerId)
			return
		}
		err = row.BattleSaves.load(dBattleSaves)
		if err != nil {
			log.Error("BattleSaves %v", PlayerId)
			return
		}
		err = row.Talents.load(dTalents)
		if err != nil {
			log.Error("Talents %v", PlayerId)
			return
		}
		err = row.TowerCommon.load(dTowerCommon)
		if err != nil {
			log.Error("TowerCommon %v", PlayerId)
			return
		}
		err = row.Towers.load(dTowers)
		if err != nil {
			log.Error("Towers %v", PlayerId)
			return
		}
		err = row.Draws.load(dDraws)
		if err != nil {
			log.Error("Draws %v", PlayerId)
			return
		}
		err = row.GoldHand.load(dGoldHand)
		if err != nil {
			log.Error("GoldHand %v", PlayerId)
			return
		}
		err = row.Shops.load(dShops)
		if err != nil {
			log.Error("Shops %v", PlayerId)
			return
		}
		err = row.ShopItems.load(dShopItems)
		if err != nil {
			log.Error("ShopItems %v", PlayerId)
			return
		}
		err = row.Arena.load(dArena)
		if err != nil {
			log.Error("Arena %v", PlayerId)
			return
		}
		err = row.Equip.load(dEquip)
		if err != nil {
			log.Error("Equip %v", PlayerId)
			return
		}
		err = row.ActiveStageCommon.load(dActiveStageCommon)
		if err != nil {
			log.Error("ActiveStageCommon %v", PlayerId)
			return
		}
		err = row.ActiveStages.load(dActiveStages)
		if err != nil {
			log.Error("ActiveStages %v", PlayerId)
			return
		}
		err = row.FriendCommon.load(dFriendCommon)
		if err != nil {
			log.Error("FriendCommon %v", PlayerId)
			return
		}
		err = row.Friends.load(dFriends)
		if err != nil {
			log.Error("Friends %v", PlayerId)
			return
		}
		err = row.FriendRecommends.load(dFriendRecommends)
		if err != nil {
			log.Error("FriendRecommends %v", PlayerId)
			return
		}
		err = row.FriendAsks.load(dFriendAsks)
		if err != nil {
			log.Error("FriendAsks %v", PlayerId)
			return
		}
		err = row.FriendBosss.load(dFriendBosss)
		if err != nil {
			log.Error("FriendBosss %v", PlayerId)
			return
		}
		err = row.TaskCommon.load(dTaskCommon)
		if err != nil {
			log.Error("TaskCommon %v", PlayerId)
			return
		}
		err = row.Tasks.load(dTasks)
		if err != nil {
			log.Error("Tasks %v", PlayerId)
			return
		}
		err = row.FinishedTasks.load(dFinishedTasks)
		if err != nil {
			log.Error("FinishedTasks %v", PlayerId)
			return
		}
		err = row.DailyTaskAllDailys.load(dDailyTaskAllDailys)
		if err != nil {
			log.Error("DailyTaskAllDailys %v", PlayerId)
			return
		}
		err = row.ExploreCommon.load(dExploreCommon)
		if err != nil {
			log.Error("ExploreCommon %v", PlayerId)
			return
		}
		err = row.Explores.load(dExplores)
		if err != nil {
			log.Error("Explores %v", PlayerId)
			return
		}
		err = row.ExploreStorys.load(dExploreStorys)
		if err != nil {
			log.Error("ExploreStorys %v", PlayerId)
			return
		}
		err = row.FriendChatUnreadIds.load(dFriendChatUnreadIds)
		if err != nil {
			log.Error("FriendChatUnreadIds %v", PlayerId)
			return
		}
		err = row.FriendChatUnreadMessages.load(dFriendChatUnreadMessages)
		if err != nil {
			log.Error("FriendChatUnreadMessages %v", PlayerId)
			return
		}
		err = row.HeadItems.load(dHeadItems)
		if err != nil {
			log.Error("HeadItems %v", PlayerId)
			return
		}
		err = row.SuitAwards.load(dSuitAwards)
		if err != nil {
			log.Error("SuitAwards %v", PlayerId)
			return
		}
		err = row.Chats.load(dChats)
		if err != nil {
			log.Error("Chats %v", PlayerId)
			return
		}
		err = row.Anouncement.load(dAnouncement)
		if err != nil {
			log.Error("Anouncement %v", PlayerId)
			return
		}
		err = row.FirstDrawCards.load(dFirstDrawCards)
		if err != nil {
			log.Error("FirstDrawCards %v", PlayerId)
			return
		}
		err = row.Guild.load(dGuild)
		if err != nil {
			log.Error("Guild %v", PlayerId)
			return
		}
		err = row.GuildStage.load(dGuildStage)
		if err != nil {
			log.Error("GuildStage %v", PlayerId)
			return
		}
		err = row.Sign.load(dSign)
		if err != nil {
			log.Error("Sign %v", PlayerId)
			return
		}
		err = row.SevenDays.load(dSevenDays)
		if err != nil {
			log.Error("SevenDays %v", PlayerId)
			return
		}
		err = row.PayCommon.load(dPayCommon)
		if err != nil {
			log.Error("PayCommon %v", PlayerId)
			return
		}
		err = row.Pays.load(dPays)
		if err != nil {
			log.Error("Pays %v", PlayerId)
			return
		}
		err = row.GuideData.load(dGuideData)
		if err != nil {
			log.Error("GuideData %v", PlayerId)
			return
		}
		err = row.ActivityDatas.load(dActivityDatas)
		if err != nil {
			log.Error("ActivityDatas %v", PlayerId)
			return
		}
		err = row.ExpeditionData.load(dExpeditionData)
		if err != nil {
			log.Error("ExpeditionData %v", PlayerId)
			return
		}
		err = row.ExpeditionRoles.load(dExpeditionRoles)
		if err != nil {
			log.Error("ExpeditionRoles %v", PlayerId)
			return
		}
		err = row.ExpeditionLevels.load(dExpeditionLevels)
		if err != nil {
			log.Error("ExpeditionLevels %v", PlayerId)
			return
		}
		err = row.ExpeditionLevelRole0s.load(dExpeditionLevelRole0s)
		if err != nil {
			log.Error("ExpeditionLevelRole0s %v", PlayerId)
			return
		}
		err = row.ExpeditionLevelRole1s.load(dExpeditionLevelRole1s)
		if err != nil {
			log.Error("ExpeditionLevelRole1s %v", PlayerId)
			return
		}
		err = row.ExpeditionLevelRole2s.load(dExpeditionLevelRole2s)
		if err != nil {
			log.Error("ExpeditionLevelRole2s %v", PlayerId)
			return
		}
		err = row.ExpeditionLevelRole3s.load(dExpeditionLevelRole3s)
		if err != nil {
			log.Error("ExpeditionLevelRole3s %v", PlayerId)
			return
		}
		err = row.ExpeditionLevelRole4s.load(dExpeditionLevelRole4s)
		if err != nil {
			log.Error("ExpeditionLevelRole4s %v", PlayerId)
			return
		}
		err = row.ExpeditionLevelRole5s.load(dExpeditionLevelRole5s)
		if err != nil {
			log.Error("ExpeditionLevelRole5s %v", PlayerId)
			return
		}
		err = row.ExpeditionLevelRole6s.load(dExpeditionLevelRole6s)
		if err != nil {
			log.Error("ExpeditionLevelRole6s %v", PlayerId)
			return
		}
		err = row.ExpeditionLevelRole7s.load(dExpeditionLevelRole7s)
		if err != nil {
			log.Error("ExpeditionLevelRole7s %v", PlayerId)
			return
		}
		err = row.ExpeditionLevelRole8s.load(dExpeditionLevelRole8s)
		if err != nil {
			log.Error("ExpeditionLevelRole8s %v", PlayerId)
			return
		}
		err = row.ExpeditionLevelRole9s.load(dExpeditionLevelRole9s)
		if err != nil {
			log.Error("ExpeditionLevelRole9s %v", PlayerId)
			return
		}
		err = row.SysMail.load(dSysMail)
		if err != nil {
			log.Error("SysMail %v", PlayerId)
			return
		}
		err = row.Artifacts.load(dArtifacts)
		if err != nil {
			log.Error("Artifacts %v", PlayerId)
			return
		}
		err = row.CarnivalCommon.load(dCarnivalCommon)
		if err != nil {
			log.Error("CarnivalCommon %v", PlayerId)
			return
		}
		err = row.Carnivals.load(dCarnivals)
		if err != nil {
			log.Error("Carnivals %v", PlayerId)
			return
		}
		err = row.InviteCodess.load(dInviteCodess)
		if err != nil {
			log.Error("InviteCodess %v", PlayerId)
			return
		}
		row.m_UniqueId_changed=false
		row.m_Account_changed=false
		row.m_Name_changed=false
		row.m_Token_changed=false
		row.m_CurrReplyMsgNum_changed=false
		row.m_Level_changed=false
		row.m_valid = true
		this.m_rows[PlayerId]=row
	}
	return
}
func (this *dbPlayerTable) GetPreloadedMaxId() (max_id int32) {
	return this.m_preload_max_id
}
func (this *dbPlayerTable) fetch_rows(rows map[int32]*dbPlayerRow) (r map[int32]*dbPlayerRow) {
	this.m_lock.UnSafeLock("dbPlayerTable.fetch_rows")
	defer this.m_lock.UnSafeUnlock()
	r = make(map[int32]*dbPlayerRow)
	for i, v := range rows {
		r[i] = v
	}
	return r
}
func (this *dbPlayerTable) fetch_new_rows() (new_rows map[int32]*dbPlayerRow) {
	this.m_lock.UnSafeLock("dbPlayerTable.fetch_new_rows")
	defer this.m_lock.UnSafeUnlock()
	new_rows = make(map[int32]*dbPlayerRow)
	for i, v := range this.m_new_rows {
		_, has := this.m_rows[i]
		if has {
			log.Error("rows already has new rows %v", i)
			continue
		}
		this.m_rows[i] = v
		new_rows[i] = v
	}
	for i, _ := range new_rows {
		delete(this.m_new_rows, i)
	}
	return
}
func (this *dbPlayerTable) save_rows(rows map[int32]*dbPlayerRow, quick bool) {
	for _, v := range rows {
		if this.m_dbc.m_quit && !quick {
			return
		}
		err, delay, _ := v.Save(false)
		if err != nil {
			log.Error("save failed %v", err)
		}
		if this.m_dbc.m_quit && !quick {
			return
		}
		if delay&&!quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
}
func (this *dbPlayerTable) Save(quick bool) (err error){
	removed_rows := this.fetch_rows(this.m_removed_rows)
	for _, v := range removed_rows {
		_, err := this.m_dbc.StmtExec(this.m_delete_stmt, v.GetPlayerId())
		if err != nil {
			log.Error("exec delete stmt failed %v", err)
		}
		v.m_valid = false
		if !quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
	this.m_removed_rows = make(map[int32]*dbPlayerRow)
	rows := this.fetch_rows(this.m_rows)
	this.save_rows(rows, quick)
	new_rows := this.fetch_new_rows()
	this.save_rows(new_rows, quick)
	return
}
func (this *dbPlayerTable) AddRow(PlayerId int32) (row *dbPlayerRow) {
	this.m_lock.UnSafeLock("dbPlayerTable.AddRow")
	defer this.m_lock.UnSafeUnlock()
	row = new_dbPlayerRow(this,PlayerId)
	row.m_new = true
	row.m_loaded = true
	row.m_valid = true
	_, has := this.m_new_rows[PlayerId]
	if has{
		log.Error("已经存在 %v", PlayerId)
		return nil
	}
	this.m_new_rows[PlayerId] = row
	atomic.AddInt32(&this.m_gc_n,1)
	return row
}
func (this *dbPlayerTable) RemoveRow(PlayerId int32) {
	this.m_lock.UnSafeLock("dbPlayerTable.RemoveRow")
	defer this.m_lock.UnSafeUnlock()
	row := this.m_rows[PlayerId]
	if row != nil {
		row.m_remove = true
		delete(this.m_rows, PlayerId)
		rm_row := this.m_removed_rows[PlayerId]
		if rm_row != nil {
			log.Error("rows and removed rows both has %v", PlayerId)
		}
		this.m_removed_rows[PlayerId] = row
		_, has_new := this.m_new_rows[PlayerId]
		if has_new {
			delete(this.m_new_rows, PlayerId)
			log.Error("rows and new_rows both has %v", PlayerId)
		}
	} else {
		row = this.m_removed_rows[PlayerId]
		if row == nil {
			_, has_new := this.m_new_rows[PlayerId]
			if has_new {
				delete(this.m_new_rows, PlayerId)
			} else {
				log.Error("row not exist %v", PlayerId)
			}
		} else {
			log.Error("already removed %v", PlayerId)
			_, has_new := this.m_new_rows[PlayerId]
			if has_new {
				delete(this.m_new_rows, PlayerId)
				log.Error("removed rows and new_rows both has %v", PlayerId)
			}
		}
	}
}
func (this *dbPlayerTable) GetRow(PlayerId int32) (row *dbPlayerRow) {
	this.m_lock.UnSafeRLock("dbPlayerTable.GetRow")
	defer this.m_lock.UnSafeRUnlock()
	row = this.m_rows[PlayerId]
	if row == nil {
		row = this.m_new_rows[PlayerId]
	}
	return row
}
type dbBattleSaveDataColumn struct{
	m_row *dbBattleSaveRow
	m_data *dbBattleSaveDataData
	m_changed bool
}
func (this *dbBattleSaveDataColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbBattleSaveDataData{}
		this.m_changed = false
		return nil
	}
	pb := &db.BattleSaveData{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetId())
		return
	}
	this.m_data = &dbBattleSaveDataData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbBattleSaveDataColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbBattleSaveDataColumn)Get( )(v *dbBattleSaveDataData ){
	this.m_row.m_lock.UnSafeRLock("dbBattleSaveDataColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbBattleSaveDataData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbBattleSaveDataColumn)Set(v dbBattleSaveDataData ){
	this.m_row.m_lock.UnSafeLock("dbBattleSaveDataColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbBattleSaveDataData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbBattleSaveDataColumn)GetData( )(v []byte){
	this.m_row.m_lock.UnSafeRLock("dbBattleSaveDataColumn.GetData")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = make([]byte, len(this.m_data.Data))
	for _ii, _vv := range this.m_data.Data {
		v[_ii]=_vv
	}
	return
}
func (this *dbBattleSaveDataColumn)SetData(v []byte){
	this.m_row.m_lock.UnSafeLock("dbBattleSaveDataColumn.SetData")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.Data = make([]byte, len(v))
	for _ii, _vv := range v {
		this.m_data.Data[_ii]=_vv
	}
	this.m_changed = true
	return
}
func (this *dbBattleSaveRow)GetSaveTime( )(r int32 ){
	this.m_lock.UnSafeRLock("dbBattleSaveRow.GetdbBattleSaveSaveTimeColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_SaveTime)
}
func (this *dbBattleSaveRow)SetSaveTime(v int32){
	this.m_lock.UnSafeLock("dbBattleSaveRow.SetdbBattleSaveSaveTimeColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_SaveTime=int32(v)
	this.m_SaveTime_changed=true
	return
}
func (this *dbBattleSaveRow)GetAttacker( )(r int32 ){
	this.m_lock.UnSafeRLock("dbBattleSaveRow.GetdbBattleSaveAttackerColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_Attacker)
}
func (this *dbBattleSaveRow)SetAttacker(v int32){
	this.m_lock.UnSafeLock("dbBattleSaveRow.SetdbBattleSaveAttackerColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Attacker=int32(v)
	this.m_Attacker_changed=true
	return
}
func (this *dbBattleSaveRow)GetDefenser( )(r int32 ){
	this.m_lock.UnSafeRLock("dbBattleSaveRow.GetdbBattleSaveDefenserColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_Defenser)
}
func (this *dbBattleSaveRow)SetDefenser(v int32){
	this.m_lock.UnSafeLock("dbBattleSaveRow.SetdbBattleSaveDefenserColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Defenser=int32(v)
	this.m_Defenser_changed=true
	return
}
func (this *dbBattleSaveRow)GetDeleteState( )(r int32 ){
	this.m_lock.UnSafeRLock("dbBattleSaveRow.GetdbBattleSaveDeleteStateColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_DeleteState)
}
func (this *dbBattleSaveRow)SetDeleteState(v int32){
	this.m_lock.UnSafeLock("dbBattleSaveRow.SetdbBattleSaveDeleteStateColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_DeleteState=int32(v)
	this.m_DeleteState_changed=true
	return
}
func (this *dbBattleSaveRow)GetIsWin( )(r int32 ){
	this.m_lock.UnSafeRLock("dbBattleSaveRow.GetdbBattleSaveIsWinColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_IsWin)
}
func (this *dbBattleSaveRow)SetIsWin(v int32){
	this.m_lock.UnSafeLock("dbBattleSaveRow.SetdbBattleSaveIsWinColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_IsWin=int32(v)
	this.m_IsWin_changed=true
	return
}
func (this *dbBattleSaveRow)GetAddScore( )(r int32 ){
	this.m_lock.UnSafeRLock("dbBattleSaveRow.GetdbBattleSaveAddScoreColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_AddScore)
}
func (this *dbBattleSaveRow)SetAddScore(v int32){
	this.m_lock.UnSafeLock("dbBattleSaveRow.SetdbBattleSaveAddScoreColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_AddScore=int32(v)
	this.m_AddScore_changed=true
	return
}
type dbBattleSaveRow struct {
	m_table *dbBattleSaveTable
	m_lock       *RWMutex
	m_loaded  bool
	m_new     bool
	m_remove  bool
	m_touch      int32
	m_releasable bool
	m_valid   bool
	m_Id        int32
	Data dbBattleSaveDataColumn
	m_SaveTime_changed bool
	m_SaveTime int32
	m_Attacker_changed bool
	m_Attacker int32
	m_Defenser_changed bool
	m_Defenser int32
	m_DeleteState_changed bool
	m_DeleteState int32
	m_IsWin_changed bool
	m_IsWin int32
	m_AddScore_changed bool
	m_AddScore int32
}
func new_dbBattleSaveRow(table *dbBattleSaveTable, Id int32) (r *dbBattleSaveRow) {
	this := &dbBattleSaveRow{}
	this.m_table = table
	this.m_Id = Id
	this.m_lock = NewRWMutex()
	this.m_SaveTime_changed=true
	this.m_Attacker_changed=true
	this.m_Defenser_changed=true
	this.m_DeleteState_changed=true
	this.m_IsWin_changed=true
	this.m_AddScore_changed=true
	this.Data.m_row=this
	this.Data.m_data=&dbBattleSaveDataData{}
	return this
}
func (this *dbBattleSaveRow) GetId() (r int32) {
	return this.m_Id
}
func (this *dbBattleSaveRow) save_data(release bool) (err error, released bool, state int32, update_string string, args []interface{}) {
	this.m_lock.UnSafeLock("dbBattleSaveRow.save_data")
	defer this.m_lock.UnSafeUnlock()
	if this.m_new {
		db_args:=new_db_args(8)
		db_args.Push(this.m_Id)
		dData,db_err:=this.Data.save()
		if db_err!=nil{
			log.Error("insert save Data failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dData)
		db_args.Push(this.m_SaveTime)
		db_args.Push(this.m_Attacker)
		db_args.Push(this.m_Defenser)
		db_args.Push(this.m_DeleteState)
		db_args.Push(this.m_IsWin)
		db_args.Push(this.m_AddScore)
		args=db_args.GetArgs()
		state = 1
	} else {
		if this.Data.m_changed||this.m_SaveTime_changed||this.m_Attacker_changed||this.m_Defenser_changed||this.m_DeleteState_changed||this.m_IsWin_changed||this.m_AddScore_changed{
			update_string = "UPDATE BattleSaves SET "
			db_args:=new_db_args(8)
			if this.Data.m_changed{
				update_string+="Data=?,"
				dData,err:=this.Data.save()
				if err!=nil{
					log.Error("update save Data failed")
					return err,false,0,"",nil
				}
				db_args.Push(dData)
			}
			if this.m_SaveTime_changed{
				update_string+="SaveTime=?,"
				db_args.Push(this.m_SaveTime)
			}
			if this.m_Attacker_changed{
				update_string+="Attacker=?,"
				db_args.Push(this.m_Attacker)
			}
			if this.m_Defenser_changed{
				update_string+="Defenser=?,"
				db_args.Push(this.m_Defenser)
			}
			if this.m_DeleteState_changed{
				update_string+="DeleteState=?,"
				db_args.Push(this.m_DeleteState)
			}
			if this.m_IsWin_changed{
				update_string+="IsWin=?,"
				db_args.Push(this.m_IsWin)
			}
			if this.m_AddScore_changed{
				update_string+="AddScore=?,"
				db_args.Push(this.m_AddScore)
			}
			update_string = strings.TrimRight(update_string, ", ")
			update_string+=" WHERE Id=?"
			db_args.Push(this.m_Id)
			args=db_args.GetArgs()
			state = 2
		}
	}
	this.m_new = false
	this.Data.m_changed = false
	this.m_SaveTime_changed = false
	this.m_Attacker_changed = false
	this.m_Defenser_changed = false
	this.m_DeleteState_changed = false
	this.m_IsWin_changed = false
	this.m_AddScore_changed = false
	if release && this.m_loaded {
		atomic.AddInt32(&this.m_table.m_gc_n, -1)
		this.m_loaded = false
		released = true
	}
	return nil,released,state,update_string,args
}
func (this *dbBattleSaveRow) Save(release bool) (err error, d bool, released bool) {
	err,released, state, update_string, args := this.save_data(release)
	if err != nil {
		log.Error("save data failed")
		return err, false, false
	}
	if state == 0 {
		d = false
	} else if state == 1 {
		_, err = this.m_table.m_dbc.StmtExec(this.m_table.m_save_insert_stmt, args...)
		if err != nil {
			log.Error("INSERT BattleSaves exec failed %v ", this.m_Id)
			return err, false, released
		}
		d = true
	} else if state == 2 {
		_, err = this.m_table.m_dbc.Exec(update_string, args...)
		if err != nil {
			log.Error("UPDATE BattleSaves exec failed %v", this.m_Id)
			return err, false, released
		}
		d = true
	}
	return nil, d, released
}
func (this *dbBattleSaveRow) Touch(releasable bool) {
	this.m_touch = int32(time.Now().Unix())
	this.m_releasable = releasable
}
type dbBattleSaveRowSort struct {
	rows []*dbBattleSaveRow
}
func (this *dbBattleSaveRowSort) Len() (length int) {
	return len(this.rows)
}
func (this *dbBattleSaveRowSort) Less(i int, j int) (less bool) {
	return this.rows[i].m_touch < this.rows[j].m_touch
}
func (this *dbBattleSaveRowSort) Swap(i int, j int) {
	temp := this.rows[i]
	this.rows[i] = this.rows[j]
	this.rows[j] = temp
}
type dbBattleSaveTable struct{
	m_dbc *DBC
	m_lock *RWMutex
	m_rows map[int32]*dbBattleSaveRow
	m_new_rows map[int32]*dbBattleSaveRow
	m_removed_rows map[int32]*dbBattleSaveRow
	m_gc_n int32
	m_gcing int32
	m_pool_size int32
	m_preload_select_stmt *sql.Stmt
	m_preload_max_id int32
	m_save_insert_stmt *sql.Stmt
	m_delete_stmt *sql.Stmt
	m_max_id int32
	m_max_id_changed bool
}
func new_dbBattleSaveTable(dbc *DBC) (this *dbBattleSaveTable) {
	this = &dbBattleSaveTable{}
	this.m_dbc = dbc
	this.m_lock = NewRWMutex()
	this.m_rows = make(map[int32]*dbBattleSaveRow)
	this.m_new_rows = make(map[int32]*dbBattleSaveRow)
	this.m_removed_rows = make(map[int32]*dbBattleSaveRow)
	return this
}
func (this *dbBattleSaveTable) check_create_table() (err error) {
	_, err = this.m_dbc.Exec("CREATE TABLE IF NOT EXISTS BattleSavesMaxId(PlaceHolder int(11),MaxId int(11),PRIMARY KEY (PlaceHolder))ENGINE=InnoDB ROW_FORMAT=DYNAMIC")
	if err != nil {
		log.Error("CREATE TABLE IF NOT EXISTS BattleSavesMaxId failed")
		return
	}
	r := this.m_dbc.QueryRow("SELECT Count(*) FROM BattleSavesMaxId WHERE PlaceHolder=0")
	if r != nil {
		var count int32
		err = r.Scan(&count)
		if err != nil {
			log.Error("scan count failed")
			return
		}
		if count == 0 {
		_, err = this.m_dbc.Exec("INSERT INTO BattleSavesMaxId (PlaceHolder,MaxId) VALUES (0,0)")
			if err != nil {
				log.Error("INSERTBattleSavesMaxId failed")
				return
			}
		}
	}
	_, err = this.m_dbc.Exec("CREATE TABLE IF NOT EXISTS BattleSaves(Id int(11),PRIMARY KEY (Id))ENGINE=InnoDB ROW_FORMAT=DYNAMIC")
	if err != nil {
		log.Error("CREATE TABLE IF NOT EXISTS BattleSaves failed")
		return
	}
	rows, err := this.m_dbc.Query("SELECT COLUMN_NAME,ORDINAL_POSITION FROM information_schema.`COLUMNS` WHERE TABLE_SCHEMA=? AND TABLE_NAME='BattleSaves'", this.m_dbc.m_db_name)
	if err != nil {
		log.Error("SELECT information_schema failed")
		return
	}
	columns := make(map[string]int32)
	for rows.Next() {
		var column_name string
		var ordinal_position int32
		err = rows.Scan(&column_name, &ordinal_position)
		if err != nil {
			log.Error("scan information_schema row failed")
			return
		}
		if ordinal_position < 1 {
			log.Error("col ordinal out of range")
			continue
		}
		columns[column_name] = ordinal_position
	}
	_, hasData := columns["Data"]
	if !hasData {
		_, err = this.m_dbc.Exec("ALTER TABLE BattleSaves ADD COLUMN Data LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Data failed")
			return
		}
	}
	_, hasSaveTime := columns["SaveTime"]
	if !hasSaveTime {
		_, err = this.m_dbc.Exec("ALTER TABLE BattleSaves ADD COLUMN SaveTime int(11)")
		if err != nil {
			log.Error("ADD COLUMN SaveTime failed")
			return
		}
	}
	_, hasAttacker := columns["Attacker"]
	if !hasAttacker {
		_, err = this.m_dbc.Exec("ALTER TABLE BattleSaves ADD COLUMN Attacker int(11)")
		if err != nil {
			log.Error("ADD COLUMN Attacker failed")
			return
		}
	}
	_, hasDefenser := columns["Defenser"]
	if !hasDefenser {
		_, err = this.m_dbc.Exec("ALTER TABLE BattleSaves ADD COLUMN Defenser int(11)")
		if err != nil {
			log.Error("ADD COLUMN Defenser failed")
			return
		}
	}
	_, hasDeleteState := columns["DeleteState"]
	if !hasDeleteState {
		_, err = this.m_dbc.Exec("ALTER TABLE BattleSaves ADD COLUMN DeleteState int(11)")
		if err != nil {
			log.Error("ADD COLUMN DeleteState failed")
			return
		}
	}
	_, hasIsWin := columns["IsWin"]
	if !hasIsWin {
		_, err = this.m_dbc.Exec("ALTER TABLE BattleSaves ADD COLUMN IsWin int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN IsWin failed")
			return
		}
	}
	_, hasAddScore := columns["AddScore"]
	if !hasAddScore {
		_, err = this.m_dbc.Exec("ALTER TABLE BattleSaves ADD COLUMN AddScore int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN AddScore failed")
			return
		}
	}
	return
}
func (this *dbBattleSaveTable) prepare_preload_select_stmt() (err error) {
	this.m_preload_select_stmt,err=this.m_dbc.StmtPrepare("SELECT Id,Data,SaveTime,Attacker,Defenser,DeleteState,IsWin,AddScore FROM BattleSaves")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbBattleSaveTable) prepare_save_insert_stmt()(err error){
	this.m_save_insert_stmt,err=this.m_dbc.StmtPrepare("INSERT INTO BattleSaves (Id,Data,SaveTime,Attacker,Defenser,DeleteState,IsWin,AddScore) VALUES (?,?,?,?,?,?,?,?)")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbBattleSaveTable) prepare_delete_stmt() (err error) {
	this.m_delete_stmt,err=this.m_dbc.StmtPrepare("DELETE FROM BattleSaves WHERE Id=?")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbBattleSaveTable) Init() (err error) {
	err=this.check_create_table()
	if err!=nil{
		log.Error("check_create_table failed")
		return
	}
	err=this.prepare_preload_select_stmt()
	if err!=nil{
		log.Error("prepare_preload_select_stmt failed")
		return
	}
	err=this.prepare_save_insert_stmt()
	if err!=nil{
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	err=this.prepare_delete_stmt()
	if err!=nil{
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	return
}
func (this *dbBattleSaveTable) Preload() (err error) {
	r_max_id := this.m_dbc.QueryRow("SELECT MaxId FROM BattleSavesMaxId WHERE PLACEHOLDER=0")
	if r_max_id != nil {
		err = r_max_id.Scan(&this.m_max_id)
		if err != nil {
			log.Error("scan max id failed")
			return
		}
	}
	r, err := this.m_dbc.StmtQuery(this.m_preload_select_stmt)
	if err != nil {
		log.Error("SELECT")
		return
	}
	var Id int32
	var dData []byte
	var dSaveTime int32
	var dAttacker int32
	var dDefenser int32
	var dDeleteState int32
	var dIsWin int32
	var dAddScore int32
	for r.Next() {
		err = r.Scan(&Id,&dData,&dSaveTime,&dAttacker,&dDefenser,&dDeleteState,&dIsWin,&dAddScore)
		if err != nil {
			log.Error("Scan err[%v]", err.Error())
			return
		}
		if Id>this.m_max_id{
			log.Error("max id ext")
			this.m_max_id = Id
			this.m_max_id_changed = true
		}
		row := new_dbBattleSaveRow(this,Id)
		err = row.Data.load(dData)
		if err != nil {
			log.Error("Data %v", Id)
			return
		}
		row.m_SaveTime=dSaveTime
		row.m_Attacker=dAttacker
		row.m_Defenser=dDefenser
		row.m_DeleteState=dDeleteState
		row.m_IsWin=dIsWin
		row.m_AddScore=dAddScore
		row.m_SaveTime_changed=false
		row.m_Attacker_changed=false
		row.m_Defenser_changed=false
		row.m_DeleteState_changed=false
		row.m_IsWin_changed=false
		row.m_AddScore_changed=false
		row.m_valid = true
		this.m_rows[Id]=row
	}
	return
}
func (this *dbBattleSaveTable) GetPreloadedMaxId() (max_id int32) {
	return this.m_preload_max_id
}
func (this *dbBattleSaveTable) fetch_rows(rows map[int32]*dbBattleSaveRow) (r map[int32]*dbBattleSaveRow) {
	this.m_lock.UnSafeLock("dbBattleSaveTable.fetch_rows")
	defer this.m_lock.UnSafeUnlock()
	r = make(map[int32]*dbBattleSaveRow)
	for i, v := range rows {
		r[i] = v
	}
	return r
}
func (this *dbBattleSaveTable) fetch_new_rows() (new_rows map[int32]*dbBattleSaveRow) {
	this.m_lock.UnSafeLock("dbBattleSaveTable.fetch_new_rows")
	defer this.m_lock.UnSafeUnlock()
	new_rows = make(map[int32]*dbBattleSaveRow)
	for i, v := range this.m_new_rows {
		_, has := this.m_rows[i]
		if has {
			log.Error("rows already has new rows %v", i)
			continue
		}
		this.m_rows[i] = v
		new_rows[i] = v
	}
	for i, _ := range new_rows {
		delete(this.m_new_rows, i)
	}
	return
}
func (this *dbBattleSaveTable) save_rows(rows map[int32]*dbBattleSaveRow, quick bool) {
	for _, v := range rows {
		if this.m_dbc.m_quit && !quick {
			return
		}
		err, delay, _ := v.Save(false)
		if err != nil {
			log.Error("save failed %v", err)
		}
		if this.m_dbc.m_quit && !quick {
			return
		}
		if delay&&!quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
}
func (this *dbBattleSaveTable) Save(quick bool) (err error){
	if this.m_max_id_changed {
		max_id := atomic.LoadInt32(&this.m_max_id)
		_, err := this.m_dbc.Exec("UPDATE BattleSavesMaxId SET MaxId=?", max_id)
		if err != nil {
			log.Error("save max id failed %v", err)
		}
	}
	removed_rows := this.fetch_rows(this.m_removed_rows)
	for _, v := range removed_rows {
		_, err := this.m_dbc.StmtExec(this.m_delete_stmt, v.GetId())
		if err != nil {
			log.Error("exec delete stmt failed %v", err)
		}
		v.m_valid = false
		if !quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
	this.m_removed_rows = make(map[int32]*dbBattleSaveRow)
	rows := this.fetch_rows(this.m_rows)
	this.save_rows(rows, quick)
	new_rows := this.fetch_new_rows()
	this.save_rows(new_rows, quick)
	return
}
func (this *dbBattleSaveTable) AddRow() (row *dbBattleSaveRow) {
	this.m_lock.UnSafeLock("dbBattleSaveTable.AddRow")
	defer this.m_lock.UnSafeUnlock()
	Id := atomic.AddInt32(&this.m_max_id, 1)
	this.m_max_id_changed = true
	row = new_dbBattleSaveRow(this,Id)
	row.m_new = true
	row.m_loaded = true
	row.m_valid = true
	this.m_new_rows[Id] = row
	atomic.AddInt32(&this.m_gc_n,1)
	return row
}
func (this *dbBattleSaveTable) RemoveRow(Id int32) {
	this.m_lock.UnSafeLock("dbBattleSaveTable.RemoveRow")
	defer this.m_lock.UnSafeUnlock()
	row := this.m_rows[Id]
	if row != nil {
		row.m_remove = true
		delete(this.m_rows, Id)
		rm_row := this.m_removed_rows[Id]
		if rm_row != nil {
			log.Error("rows and removed rows both has %v", Id)
		}
		this.m_removed_rows[Id] = row
		_, has_new := this.m_new_rows[Id]
		if has_new {
			delete(this.m_new_rows, Id)
			log.Error("rows and new_rows both has %v", Id)
		}
	} else {
		row = this.m_removed_rows[Id]
		if row == nil {
			_, has_new := this.m_new_rows[Id]
			if has_new {
				delete(this.m_new_rows, Id)
			} else {
				log.Error("row not exist %v", Id)
			}
		} else {
			log.Error("already removed %v", Id)
			_, has_new := this.m_new_rows[Id]
			if has_new {
				delete(this.m_new_rows, Id)
				log.Error("removed rows and new_rows both has %v", Id)
			}
		}
	}
}
func (this *dbBattleSaveTable) GetRow(Id int32) (row *dbBattleSaveRow) {
	this.m_lock.UnSafeRLock("dbBattleSaveTable.GetRow")
	defer this.m_lock.UnSafeRUnlock()
	row = this.m_rows[Id]
	if row == nil {
		row = this.m_new_rows[Id]
	}
	return row
}
type dbTowerFightSaveDataColumn struct{
	m_row *dbTowerFightSaveRow
	m_data *dbTowerFightSaveDataData
	m_changed bool
}
func (this *dbTowerFightSaveDataColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbTowerFightSaveDataData{}
		this.m_changed = false
		return nil
	}
	pb := &db.TowerFightSaveData{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetTowerFightId())
		return
	}
	this.m_data = &dbTowerFightSaveDataData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbTowerFightSaveDataColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetTowerFightId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbTowerFightSaveDataColumn)Get( )(v *dbTowerFightSaveDataData ){
	this.m_row.m_lock.UnSafeRLock("dbTowerFightSaveDataColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbTowerFightSaveDataData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbTowerFightSaveDataColumn)Set(v dbTowerFightSaveDataData ){
	this.m_row.m_lock.UnSafeLock("dbTowerFightSaveDataColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbTowerFightSaveDataData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbTowerFightSaveDataColumn)GetData( )(v []byte){
	this.m_row.m_lock.UnSafeRLock("dbTowerFightSaveDataColumn.GetData")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = make([]byte, len(this.m_data.Data))
	for _ii, _vv := range this.m_data.Data {
		v[_ii]=_vv
	}
	return
}
func (this *dbTowerFightSaveDataColumn)SetData(v []byte){
	this.m_row.m_lock.UnSafeLock("dbTowerFightSaveDataColumn.SetData")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.Data = make([]byte, len(v))
	for _ii, _vv := range v {
		this.m_data.Data[_ii]=_vv
	}
	this.m_changed = true
	return
}
func (this *dbTowerFightSaveRow)GetSaveTime( )(r int32 ){
	this.m_lock.UnSafeRLock("dbTowerFightSaveRow.GetdbTowerFightSaveSaveTimeColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_SaveTime)
}
func (this *dbTowerFightSaveRow)SetSaveTime(v int32){
	this.m_lock.UnSafeLock("dbTowerFightSaveRow.SetdbTowerFightSaveSaveTimeColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_SaveTime=int32(v)
	this.m_SaveTime_changed=true
	return
}
func (this *dbTowerFightSaveRow)GetAttacker( )(r int32 ){
	this.m_lock.UnSafeRLock("dbTowerFightSaveRow.GetdbTowerFightSaveAttackerColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_Attacker)
}
func (this *dbTowerFightSaveRow)SetAttacker(v int32){
	this.m_lock.UnSafeLock("dbTowerFightSaveRow.SetdbTowerFightSaveAttackerColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Attacker=int32(v)
	this.m_Attacker_changed=true
	return
}
func (this *dbTowerFightSaveRow)GetTowerId( )(r int32 ){
	this.m_lock.UnSafeRLock("dbTowerFightSaveRow.GetdbTowerFightSaveTowerIdColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_TowerId)
}
func (this *dbTowerFightSaveRow)SetTowerId(v int32){
	this.m_lock.UnSafeLock("dbTowerFightSaveRow.SetdbTowerFightSaveTowerIdColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_TowerId=int32(v)
	this.m_TowerId_changed=true
	return
}
type dbTowerFightSaveRow struct {
	m_table *dbTowerFightSaveTable
	m_lock       *RWMutex
	m_loaded  bool
	m_new     bool
	m_remove  bool
	m_touch      int32
	m_releasable bool
	m_valid   bool
	m_TowerFightId        int32
	Data dbTowerFightSaveDataColumn
	m_SaveTime_changed bool
	m_SaveTime int32
	m_Attacker_changed bool
	m_Attacker int32
	m_TowerId_changed bool
	m_TowerId int32
}
func new_dbTowerFightSaveRow(table *dbTowerFightSaveTable, TowerFightId int32) (r *dbTowerFightSaveRow) {
	this := &dbTowerFightSaveRow{}
	this.m_table = table
	this.m_TowerFightId = TowerFightId
	this.m_lock = NewRWMutex()
	this.m_SaveTime_changed=true
	this.m_Attacker_changed=true
	this.m_TowerId_changed=true
	this.Data.m_row=this
	this.Data.m_data=&dbTowerFightSaveDataData{}
	return this
}
func (this *dbTowerFightSaveRow) GetTowerFightId() (r int32) {
	return this.m_TowerFightId
}
func (this *dbTowerFightSaveRow) save_data(release bool) (err error, released bool, state int32, update_string string, args []interface{}) {
	this.m_lock.UnSafeLock("dbTowerFightSaveRow.save_data")
	defer this.m_lock.UnSafeUnlock()
	if this.m_new {
		db_args:=new_db_args(5)
		db_args.Push(this.m_TowerFightId)
		dData,db_err:=this.Data.save()
		if db_err!=nil{
			log.Error("insert save Data failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dData)
		db_args.Push(this.m_SaveTime)
		db_args.Push(this.m_Attacker)
		db_args.Push(this.m_TowerId)
		args=db_args.GetArgs()
		state = 1
	} else {
		if this.Data.m_changed||this.m_SaveTime_changed||this.m_Attacker_changed||this.m_TowerId_changed{
			update_string = "UPDATE TowerFightSaves SET "
			db_args:=new_db_args(5)
			if this.Data.m_changed{
				update_string+="Data=?,"
				dData,err:=this.Data.save()
				if err!=nil{
					log.Error("update save Data failed")
					return err,false,0,"",nil
				}
				db_args.Push(dData)
			}
			if this.m_SaveTime_changed{
				update_string+="SaveTime=?,"
				db_args.Push(this.m_SaveTime)
			}
			if this.m_Attacker_changed{
				update_string+="Attacker=?,"
				db_args.Push(this.m_Attacker)
			}
			if this.m_TowerId_changed{
				update_string+="TowerId=?,"
				db_args.Push(this.m_TowerId)
			}
			update_string = strings.TrimRight(update_string, ", ")
			update_string+=" WHERE TowerFightId=?"
			db_args.Push(this.m_TowerFightId)
			args=db_args.GetArgs()
			state = 2
		}
	}
	this.m_new = false
	this.Data.m_changed = false
	this.m_SaveTime_changed = false
	this.m_Attacker_changed = false
	this.m_TowerId_changed = false
	if release && this.m_loaded {
		atomic.AddInt32(&this.m_table.m_gc_n, -1)
		this.m_loaded = false
		released = true
	}
	return nil,released,state,update_string,args
}
func (this *dbTowerFightSaveRow) Save(release bool) (err error, d bool, released bool) {
	err,released, state, update_string, args := this.save_data(release)
	if err != nil {
		log.Error("save data failed")
		return err, false, false
	}
	if state == 0 {
		d = false
	} else if state == 1 {
		_, err = this.m_table.m_dbc.StmtExec(this.m_table.m_save_insert_stmt, args...)
		if err != nil {
			log.Error("INSERT TowerFightSaves exec failed %v ", this.m_TowerFightId)
			return err, false, released
		}
		d = true
	} else if state == 2 {
		_, err = this.m_table.m_dbc.Exec(update_string, args...)
		if err != nil {
			log.Error("UPDATE TowerFightSaves exec failed %v", this.m_TowerFightId)
			return err, false, released
		}
		d = true
	}
	return nil, d, released
}
func (this *dbTowerFightSaveRow) Touch(releasable bool) {
	this.m_touch = int32(time.Now().Unix())
	this.m_releasable = releasable
}
type dbTowerFightSaveRowSort struct {
	rows []*dbTowerFightSaveRow
}
func (this *dbTowerFightSaveRowSort) Len() (length int) {
	return len(this.rows)
}
func (this *dbTowerFightSaveRowSort) Less(i int, j int) (less bool) {
	return this.rows[i].m_touch < this.rows[j].m_touch
}
func (this *dbTowerFightSaveRowSort) Swap(i int, j int) {
	temp := this.rows[i]
	this.rows[i] = this.rows[j]
	this.rows[j] = temp
}
type dbTowerFightSaveTable struct{
	m_dbc *DBC
	m_lock *RWMutex
	m_rows map[int32]*dbTowerFightSaveRow
	m_new_rows map[int32]*dbTowerFightSaveRow
	m_removed_rows map[int32]*dbTowerFightSaveRow
	m_gc_n int32
	m_gcing int32
	m_pool_size int32
	m_preload_select_stmt *sql.Stmt
	m_preload_max_id int32
	m_save_insert_stmt *sql.Stmt
	m_delete_stmt *sql.Stmt
}
func new_dbTowerFightSaveTable(dbc *DBC) (this *dbTowerFightSaveTable) {
	this = &dbTowerFightSaveTable{}
	this.m_dbc = dbc
	this.m_lock = NewRWMutex()
	this.m_rows = make(map[int32]*dbTowerFightSaveRow)
	this.m_new_rows = make(map[int32]*dbTowerFightSaveRow)
	this.m_removed_rows = make(map[int32]*dbTowerFightSaveRow)
	return this
}
func (this *dbTowerFightSaveTable) check_create_table() (err error) {
	_, err = this.m_dbc.Exec("CREATE TABLE IF NOT EXISTS TowerFightSaves(TowerFightId int(11),PRIMARY KEY (TowerFightId))ENGINE=InnoDB ROW_FORMAT=DYNAMIC")
	if err != nil {
		log.Error("CREATE TABLE IF NOT EXISTS TowerFightSaves failed")
		return
	}
	rows, err := this.m_dbc.Query("SELECT COLUMN_NAME,ORDINAL_POSITION FROM information_schema.`COLUMNS` WHERE TABLE_SCHEMA=? AND TABLE_NAME='TowerFightSaves'", this.m_dbc.m_db_name)
	if err != nil {
		log.Error("SELECT information_schema failed")
		return
	}
	columns := make(map[string]int32)
	for rows.Next() {
		var column_name string
		var ordinal_position int32
		err = rows.Scan(&column_name, &ordinal_position)
		if err != nil {
			log.Error("scan information_schema row failed")
			return
		}
		if ordinal_position < 1 {
			log.Error("col ordinal out of range")
			continue
		}
		columns[column_name] = ordinal_position
	}
	_, hasData := columns["Data"]
	if !hasData {
		_, err = this.m_dbc.Exec("ALTER TABLE TowerFightSaves ADD COLUMN Data LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Data failed")
			return
		}
	}
	_, hasSaveTime := columns["SaveTime"]
	if !hasSaveTime {
		_, err = this.m_dbc.Exec("ALTER TABLE TowerFightSaves ADD COLUMN SaveTime int(11)")
		if err != nil {
			log.Error("ADD COLUMN SaveTime failed")
			return
		}
	}
	_, hasAttacker := columns["Attacker"]
	if !hasAttacker {
		_, err = this.m_dbc.Exec("ALTER TABLE TowerFightSaves ADD COLUMN Attacker int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN Attacker failed")
			return
		}
	}
	_, hasTowerId := columns["TowerId"]
	if !hasTowerId {
		_, err = this.m_dbc.Exec("ALTER TABLE TowerFightSaves ADD COLUMN TowerId int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN TowerId failed")
			return
		}
	}
	return
}
func (this *dbTowerFightSaveTable) prepare_preload_select_stmt() (err error) {
	this.m_preload_select_stmt,err=this.m_dbc.StmtPrepare("SELECT TowerFightId,Data,SaveTime,Attacker,TowerId FROM TowerFightSaves")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbTowerFightSaveTable) prepare_save_insert_stmt()(err error){
	this.m_save_insert_stmt,err=this.m_dbc.StmtPrepare("INSERT INTO TowerFightSaves (TowerFightId,Data,SaveTime,Attacker,TowerId) VALUES (?,?,?,?,?)")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbTowerFightSaveTable) prepare_delete_stmt() (err error) {
	this.m_delete_stmt,err=this.m_dbc.StmtPrepare("DELETE FROM TowerFightSaves WHERE TowerFightId=?")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbTowerFightSaveTable) Init() (err error) {
	err=this.check_create_table()
	if err!=nil{
		log.Error("check_create_table failed")
		return
	}
	err=this.prepare_preload_select_stmt()
	if err!=nil{
		log.Error("prepare_preload_select_stmt failed")
		return
	}
	err=this.prepare_save_insert_stmt()
	if err!=nil{
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	err=this.prepare_delete_stmt()
	if err!=nil{
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	return
}
func (this *dbTowerFightSaveTable) Preload() (err error) {
	r, err := this.m_dbc.StmtQuery(this.m_preload_select_stmt)
	if err != nil {
		log.Error("SELECT")
		return
	}
	var TowerFightId int32
	var dData []byte
	var dSaveTime int32
	var dAttacker int32
	var dTowerId int32
		this.m_preload_max_id = 0
	for r.Next() {
		err = r.Scan(&TowerFightId,&dData,&dSaveTime,&dAttacker,&dTowerId)
		if err != nil {
			log.Error("Scan err[%v]", err.Error())
			return
		}
		if TowerFightId>this.m_preload_max_id{
			this.m_preload_max_id =TowerFightId
		}
		row := new_dbTowerFightSaveRow(this,TowerFightId)
		err = row.Data.load(dData)
		if err != nil {
			log.Error("Data %v", TowerFightId)
			return
		}
		row.m_SaveTime=dSaveTime
		row.m_Attacker=dAttacker
		row.m_TowerId=dTowerId
		row.m_SaveTime_changed=false
		row.m_Attacker_changed=false
		row.m_TowerId_changed=false
		row.m_valid = true
		this.m_rows[TowerFightId]=row
	}
	return
}
func (this *dbTowerFightSaveTable) GetPreloadedMaxId() (max_id int32) {
	return this.m_preload_max_id
}
func (this *dbTowerFightSaveTable) fetch_rows(rows map[int32]*dbTowerFightSaveRow) (r map[int32]*dbTowerFightSaveRow) {
	this.m_lock.UnSafeLock("dbTowerFightSaveTable.fetch_rows")
	defer this.m_lock.UnSafeUnlock()
	r = make(map[int32]*dbTowerFightSaveRow)
	for i, v := range rows {
		r[i] = v
	}
	return r
}
func (this *dbTowerFightSaveTable) fetch_new_rows() (new_rows map[int32]*dbTowerFightSaveRow) {
	this.m_lock.UnSafeLock("dbTowerFightSaveTable.fetch_new_rows")
	defer this.m_lock.UnSafeUnlock()
	new_rows = make(map[int32]*dbTowerFightSaveRow)
	for i, v := range this.m_new_rows {
		_, has := this.m_rows[i]
		if has {
			log.Error("rows already has new rows %v", i)
			continue
		}
		this.m_rows[i] = v
		new_rows[i] = v
	}
	for i, _ := range new_rows {
		delete(this.m_new_rows, i)
	}
	return
}
func (this *dbTowerFightSaveTable) save_rows(rows map[int32]*dbTowerFightSaveRow, quick bool) {
	for _, v := range rows {
		if this.m_dbc.m_quit && !quick {
			return
		}
		err, delay, _ := v.Save(false)
		if err != nil {
			log.Error("save failed %v", err)
		}
		if this.m_dbc.m_quit && !quick {
			return
		}
		if delay&&!quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
}
func (this *dbTowerFightSaveTable) Save(quick bool) (err error){
	removed_rows := this.fetch_rows(this.m_removed_rows)
	for _, v := range removed_rows {
		_, err := this.m_dbc.StmtExec(this.m_delete_stmt, v.GetTowerFightId())
		if err != nil {
			log.Error("exec delete stmt failed %v", err)
		}
		v.m_valid = false
		if !quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
	this.m_removed_rows = make(map[int32]*dbTowerFightSaveRow)
	rows := this.fetch_rows(this.m_rows)
	this.save_rows(rows, quick)
	new_rows := this.fetch_new_rows()
	this.save_rows(new_rows, quick)
	return
}
func (this *dbTowerFightSaveTable) AddRow(TowerFightId int32) (row *dbTowerFightSaveRow) {
	this.m_lock.UnSafeLock("dbTowerFightSaveTable.AddRow")
	defer this.m_lock.UnSafeUnlock()
	row = new_dbTowerFightSaveRow(this,TowerFightId)
	row.m_new = true
	row.m_loaded = true
	row.m_valid = true
	_, has := this.m_new_rows[TowerFightId]
	if has{
		log.Error("已经存在 %v", TowerFightId)
		return nil
	}
	this.m_new_rows[TowerFightId] = row
	atomic.AddInt32(&this.m_gc_n,1)
	return row
}
func (this *dbTowerFightSaveTable) RemoveRow(TowerFightId int32) {
	this.m_lock.UnSafeLock("dbTowerFightSaveTable.RemoveRow")
	defer this.m_lock.UnSafeUnlock()
	row := this.m_rows[TowerFightId]
	if row != nil {
		row.m_remove = true
		delete(this.m_rows, TowerFightId)
		rm_row := this.m_removed_rows[TowerFightId]
		if rm_row != nil {
			log.Error("rows and removed rows both has %v", TowerFightId)
		}
		this.m_removed_rows[TowerFightId] = row
		_, has_new := this.m_new_rows[TowerFightId]
		if has_new {
			delete(this.m_new_rows, TowerFightId)
			log.Error("rows and new_rows both has %v", TowerFightId)
		}
	} else {
		row = this.m_removed_rows[TowerFightId]
		if row == nil {
			_, has_new := this.m_new_rows[TowerFightId]
			if has_new {
				delete(this.m_new_rows, TowerFightId)
			} else {
				log.Error("row not exist %v", TowerFightId)
			}
		} else {
			log.Error("already removed %v", TowerFightId)
			_, has_new := this.m_new_rows[TowerFightId]
			if has_new {
				delete(this.m_new_rows, TowerFightId)
				log.Error("removed rows and new_rows both has %v", TowerFightId)
			}
		}
	}
}
func (this *dbTowerFightSaveTable) GetRow(TowerFightId int32) (row *dbTowerFightSaveRow) {
	this.m_lock.UnSafeRLock("dbTowerFightSaveTable.GetRow")
	defer this.m_lock.UnSafeRUnlock()
	row = this.m_rows[TowerFightId]
	if row == nil {
		row = this.m_new_rows[TowerFightId]
	}
	return row
}
type dbArenaSeasonDataColumn struct{
	m_row *dbArenaSeasonRow
	m_data *dbArenaSeasonDataData
	m_changed bool
}
func (this *dbArenaSeasonDataColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbArenaSeasonDataData{}
		this.m_changed = false
		return nil
	}
	pb := &db.ArenaSeasonData{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal ")
		return
	}
	this.m_data = &dbArenaSeasonDataData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbArenaSeasonDataColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Unmarshal ")
		return
	}
	this.m_changed = false
	return
}
func (this *dbArenaSeasonDataColumn)Get( )(v *dbArenaSeasonDataData ){
	this.m_row.m_lock.UnSafeRLock("dbArenaSeasonDataColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbArenaSeasonDataData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbArenaSeasonDataColumn)Set(v dbArenaSeasonDataData ){
	this.m_row.m_lock.UnSafeLock("dbArenaSeasonDataColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbArenaSeasonDataData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbArenaSeasonDataColumn)GetLastDayResetTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbArenaSeasonDataColumn.GetLastDayResetTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LastDayResetTime
	return
}
func (this *dbArenaSeasonDataColumn)SetLastDayResetTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbArenaSeasonDataColumn.SetLastDayResetTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LastDayResetTime = v
	this.m_changed = true
	return
}
func (this *dbArenaSeasonDataColumn)GetLastSeasonResetTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbArenaSeasonDataColumn.GetLastSeasonResetTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LastSeasonResetTime
	return
}
func (this *dbArenaSeasonDataColumn)SetLastSeasonResetTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbArenaSeasonDataColumn.SetLastSeasonResetTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LastSeasonResetTime = v
	this.m_changed = true
	return
}
type dbArenaSeasonRow struct {
	m_table *dbArenaSeasonTable
	m_lock       *RWMutex
	m_loaded  bool
	m_new     bool
	m_remove  bool
	m_touch      int32
	m_releasable bool
	m_valid   bool
	m_Id        int32
	Data dbArenaSeasonDataColumn
}
func new_dbArenaSeasonRow(table *dbArenaSeasonTable, Id int32) (r *dbArenaSeasonRow) {
	this := &dbArenaSeasonRow{}
	this.m_table = table
	this.m_Id = Id
	this.m_lock = NewRWMutex()
	this.Data.m_row=this
	this.Data.m_data=&dbArenaSeasonDataData{}
	return this
}
func (this *dbArenaSeasonRow) save_data(release bool) (err error, released bool, state int32, update_string string, args []interface{}) {
	this.m_lock.UnSafeLock("dbArenaSeasonRow.save_data")
	defer this.m_lock.UnSafeUnlock()
	if this.m_new {
		db_args:=new_db_args(2)
		db_args.Push(this.m_Id)
		dData,db_err:=this.Data.save()
		if db_err!=nil{
			log.Error("insert save Data failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dData)
		args=db_args.GetArgs()
		state = 1
	} else {
		if this.Data.m_changed{
			update_string = "UPDATE ArenaSeason SET "
			db_args:=new_db_args(2)
			if this.Data.m_changed{
				update_string+="Data=?,"
				dData,err:=this.Data.save()
				if err!=nil{
					log.Error("update save Data failed")
					return err,false,0,"",nil
				}
				db_args.Push(dData)
			}
			update_string = strings.TrimRight(update_string, ", ")
			update_string+=" WHERE Id=?"
			db_args.Push(this.m_Id)
			args=db_args.GetArgs()
			state = 2
		}
	}
	this.m_new = false
	this.Data.m_changed = false
	if release && this.m_loaded {
		this.m_loaded = false
		released = true
	}
	return nil,released,state,update_string,args
}
func (this *dbArenaSeasonRow) Save(release bool) (err error, d bool, released bool) {
	err,released, state, update_string, args := this.save_data(release)
	if err != nil {
		log.Error("save data failed")
		return err, false, false
	}
	if state == 0 {
		d = false
	} else if state == 1 {
		_, err = this.m_table.m_dbc.StmtExec(this.m_table.m_save_insert_stmt, args...)
		if err != nil {
			log.Error("INSERT ArenaSeason exec failed %v ", this.m_Id)
			return err, false, released
		}
		d = true
	} else if state == 2 {
		_, err = this.m_table.m_dbc.Exec(update_string, args...)
		if err != nil {
			log.Error("UPDATE ArenaSeason exec failed %v", this.m_Id)
			return err, false, released
		}
		d = true
	}
	return nil, d, released
}
type dbArenaSeasonTable struct{
	m_dbc *DBC
	m_lock *RWMutex
	m_row *dbArenaSeasonRow
	m_preload_select_stmt *sql.Stmt
	m_save_insert_stmt *sql.Stmt
}
func new_dbArenaSeasonTable(dbc *DBC) (this *dbArenaSeasonTable) {
	this = &dbArenaSeasonTable{}
	this.m_dbc = dbc
	this.m_lock = NewRWMutex()
	return this
}
func (this *dbArenaSeasonTable) check_create_table() (err error) {
	_, err = this.m_dbc.Exec("CREATE TABLE IF NOT EXISTS ArenaSeason(Id int(11),PRIMARY KEY (Id))ENGINE=InnoDB ROW_FORMAT=DYNAMIC")
	if err != nil {
		log.Error("CREATE TABLE IF NOT EXISTS ArenaSeason failed")
		return
	}
	rows, err := this.m_dbc.Query("SELECT COLUMN_NAME,ORDINAL_POSITION FROM information_schema.`COLUMNS` WHERE TABLE_SCHEMA=? AND TABLE_NAME='ArenaSeason'", this.m_dbc.m_db_name)
	if err != nil {
		log.Error("SELECT information_schema failed")
		return
	}
	columns := make(map[string]int32)
	for rows.Next() {
		var column_name string
		var ordinal_position int32
		err = rows.Scan(&column_name, &ordinal_position)
		if err != nil {
			log.Error("scan information_schema row failed")
			return
		}
		if ordinal_position < 1 {
			log.Error("col ordinal out of range")
			continue
		}
		columns[column_name] = ordinal_position
	}
	_, hasData := columns["Data"]
	if !hasData {
		_, err = this.m_dbc.Exec("ALTER TABLE ArenaSeason ADD COLUMN Data LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Data failed")
			return
		}
	}
	return
}
func (this *dbArenaSeasonTable) prepare_preload_select_stmt() (err error) {
	this.m_preload_select_stmt,err=this.m_dbc.StmtPrepare("SELECT Data FROM ArenaSeason WHERE Id=0")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbArenaSeasonTable) prepare_save_insert_stmt()(err error){
	this.m_save_insert_stmt,err=this.m_dbc.StmtPrepare("INSERT INTO ArenaSeason (Id,Data) VALUES (?,?)")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbArenaSeasonTable) Init() (err error) {
	err=this.check_create_table()
	if err!=nil{
		log.Error("check_create_table failed")
		return
	}
	err=this.prepare_preload_select_stmt()
	if err!=nil{
		log.Error("prepare_preload_select_stmt failed")
		return
	}
	err=this.prepare_save_insert_stmt()
	if err!=nil{
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	return
}
func (this *dbArenaSeasonTable) Preload() (err error) {
	r := this.m_dbc.StmtQueryRow(this.m_preload_select_stmt)
	var dData []byte
	err = r.Scan(&dData)
	if err!=nil{
		if err!=sql.ErrNoRows{
			log.Error("Scan failed")
			return
		}
	}else{
		row := new_dbArenaSeasonRow(this,0)
		err = row.Data.load(dData)
		if err != nil {
			log.Error("Data ")
			return
		}
		row.m_valid = true
		row.m_loaded=true
		this.m_row=row
	}
	if this.m_row == nil {
		this.m_row = new_dbArenaSeasonRow(this, 0)
		this.m_row.m_new = true
		this.m_row.m_valid = true
		err = this.Save(false)
		if err != nil {
			log.Error("save failed")
			return
		}
		this.m_row.m_loaded = true
	}
	return
}
func (this *dbArenaSeasonTable) Save(quick bool) (err error) {
	if this.m_row==nil{
		return errors.New("row nil")
	}
	err, _, _ = this.m_row.Save(false)
	return err
}
func (this *dbArenaSeasonTable) GetRow( ) (row *dbArenaSeasonRow) {
	return this.m_row
}
func (this *dbGuildRow)GetName( )(r string ){
	this.m_lock.UnSafeRLock("dbGuildRow.GetdbGuildNameColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_Name)
}
func (this *dbGuildRow)SetName(v string){
	this.m_lock.UnSafeLock("dbGuildRow.SetdbGuildNameColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Name=string(v)
	this.m_Name_changed=true
	return
}
func (this *dbGuildRow)GetCreater( )(r int32 ){
	this.m_lock.UnSafeRLock("dbGuildRow.GetdbGuildCreaterColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_Creater)
}
func (this *dbGuildRow)SetCreater(v int32){
	this.m_lock.UnSafeLock("dbGuildRow.SetdbGuildCreaterColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Creater=int32(v)
	this.m_Creater_changed=true
	return
}
func (this *dbGuildRow)GetCreateTime( )(r int32 ){
	this.m_lock.UnSafeRLock("dbGuildRow.GetdbGuildCreateTimeColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_CreateTime)
}
func (this *dbGuildRow)SetCreateTime(v int32){
	this.m_lock.UnSafeLock("dbGuildRow.SetdbGuildCreateTimeColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_CreateTime=int32(v)
	this.m_CreateTime_changed=true
	return
}
func (this *dbGuildRow)GetDismissTime( )(r int32 ){
	this.m_lock.UnSafeRLock("dbGuildRow.GetdbGuildDismissTimeColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_DismissTime)
}
func (this *dbGuildRow)SetDismissTime(v int32){
	this.m_lock.UnSafeLock("dbGuildRow.SetdbGuildDismissTimeColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_DismissTime=int32(v)
	this.m_DismissTime_changed=true
	return
}
func (this *dbGuildRow)GetLogo( )(r int32 ){
	this.m_lock.UnSafeRLock("dbGuildRow.GetdbGuildLogoColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_Logo)
}
func (this *dbGuildRow)SetLogo(v int32){
	this.m_lock.UnSafeLock("dbGuildRow.SetdbGuildLogoColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Logo=int32(v)
	this.m_Logo_changed=true
	return
}
func (this *dbGuildRow)GetLevel( )(r int32 ){
	this.m_lock.UnSafeRLock("dbGuildRow.GetdbGuildLevelColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_Level)
}
func (this *dbGuildRow)SetLevel(v int32){
	this.m_lock.UnSafeLock("dbGuildRow.SetdbGuildLevelColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Level=int32(v)
	this.m_Level_changed=true
	return
}
func (this *dbGuildRow)GetExp( )(r int32 ){
	this.m_lock.UnSafeRLock("dbGuildRow.GetdbGuildExpColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_Exp)
}
func (this *dbGuildRow)SetExp(v int32){
	this.m_lock.UnSafeLock("dbGuildRow.SetdbGuildExpColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Exp=int32(v)
	this.m_Exp_changed=true
	return
}
func (this *dbGuildRow)GetExistType( )(r int32 ){
	this.m_lock.UnSafeRLock("dbGuildRow.GetdbGuildExistTypeColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_ExistType)
}
func (this *dbGuildRow)SetExistType(v int32){
	this.m_lock.UnSafeLock("dbGuildRow.SetdbGuildExistTypeColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_ExistType=int32(v)
	this.m_ExistType_changed=true
	return
}
func (this *dbGuildRow)GetAnouncement( )(r string ){
	this.m_lock.UnSafeRLock("dbGuildRow.GetdbGuildAnouncementColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_Anouncement)
}
func (this *dbGuildRow)SetAnouncement(v string){
	this.m_lock.UnSafeLock("dbGuildRow.SetdbGuildAnouncementColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Anouncement=string(v)
	this.m_Anouncement_changed=true
	return
}
func (this *dbGuildRow)GetPresident( )(r int32 ){
	this.m_lock.UnSafeRLock("dbGuildRow.GetdbGuildPresidentColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_President)
}
func (this *dbGuildRow)SetPresident(v int32){
	this.m_lock.UnSafeLock("dbGuildRow.SetdbGuildPresidentColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_President=int32(v)
	this.m_President_changed=true
	return
}
type dbGuildMemberColumn struct{
	m_row *dbGuildRow
	m_data map[int32]*dbGuildMemberData
	m_changed bool
}
func (this *dbGuildMemberColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.GuildMemberList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetId())
		return
	}
	for _, v := range pb.List {
		d := &dbGuildMemberData{}
		d.from_pb(v)
		this.m_data[int32(d.PlayerId)] = d
	}
	this.m_changed = false
	return
}
func (this *dbGuildMemberColumn)save( )(data []byte,err error){
	pb := &db.GuildMemberList{}
	pb.List=make([]*db.GuildMember,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbGuildMemberColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbGuildMemberColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbGuildMemberColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbGuildMemberColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbGuildMemberColumn)GetAll()(list []dbGuildMemberData){
	this.m_row.m_lock.UnSafeRLock("dbGuildMemberColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbGuildMemberData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbGuildMemberColumn)Get(id int32)(v *dbGuildMemberData){
	this.m_row.m_lock.UnSafeRLock("dbGuildMemberColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbGuildMemberData{}
	d.clone_to(v)
	return
}
func (this *dbGuildMemberColumn)Set(v dbGuildMemberData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbGuildMemberColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.PlayerId)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetId(), v.PlayerId)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbGuildMemberColumn)Add(v *dbGuildMemberData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbGuildMemberColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.PlayerId)]
	if has {
		log.Error("already added %v %v",this.m_row.GetId(), v.PlayerId)
		return false
	}
	d:=&dbGuildMemberData{}
	v.clone_to(d)
	this.m_data[int32(v.PlayerId)]=d
	this.m_changed = true
	return true
}
func (this *dbGuildMemberColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbGuildMemberColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbGuildMemberColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbGuildMemberColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbGuildMemberData)
	this.m_changed = true
	return
}
func (this *dbGuildMemberColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbGuildMemberColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
type dbGuildAskListColumn struct{
	m_row *dbGuildRow
	m_data map[int32]*dbGuildAskListData
	m_changed bool
}
func (this *dbGuildAskListColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.GuildAskListList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetId())
		return
	}
	for _, v := range pb.List {
		d := &dbGuildAskListData{}
		d.from_pb(v)
		this.m_data[int32(d.PlayerId)] = d
	}
	this.m_changed = false
	return
}
func (this *dbGuildAskListColumn)save( )(data []byte,err error){
	pb := &db.GuildAskListList{}
	pb.List=make([]*db.GuildAskList,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbGuildAskListColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbGuildAskListColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbGuildAskListColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbGuildAskListColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbGuildAskListColumn)GetAll()(list []dbGuildAskListData){
	this.m_row.m_lock.UnSafeRLock("dbGuildAskListColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbGuildAskListData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbGuildAskListColumn)Get(id int32)(v *dbGuildAskListData){
	this.m_row.m_lock.UnSafeRLock("dbGuildAskListColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbGuildAskListData{}
	d.clone_to(v)
	return
}
func (this *dbGuildAskListColumn)Set(v dbGuildAskListData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbGuildAskListColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.PlayerId)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetId(), v.PlayerId)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbGuildAskListColumn)Add(v *dbGuildAskListData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbGuildAskListColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.PlayerId)]
	if has {
		log.Error("already added %v %v",this.m_row.GetId(), v.PlayerId)
		return false
	}
	d:=&dbGuildAskListData{}
	v.clone_to(d)
	this.m_data[int32(v.PlayerId)]=d
	this.m_changed = true
	return true
}
func (this *dbGuildAskListColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbGuildAskListColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbGuildAskListColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbGuildAskListColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbGuildAskListData)
	this.m_changed = true
	return
}
func (this *dbGuildAskListColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbGuildAskListColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbGuildRow)GetLastDonateRefreshTime( )(r int32 ){
	this.m_lock.UnSafeRLock("dbGuildRow.GetdbGuildLastDonateRefreshTimeColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_LastDonateRefreshTime)
}
func (this *dbGuildRow)SetLastDonateRefreshTime(v int32){
	this.m_lock.UnSafeLock("dbGuildRow.SetdbGuildLastDonateRefreshTimeColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_LastDonateRefreshTime=int32(v)
	this.m_LastDonateRefreshTime_changed=true
	return
}
type dbGuildLogColumn struct{
	m_row *dbGuildRow
	m_data map[int32]*dbGuildLogData
	m_max_id int32
	m_changed bool
}
func (this *dbGuildLogColumn)load(max_id int32, data []byte)(err error){
	this.m_max_id=max_id
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.GuildLogList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetId())
		return
	}
	for _, v := range pb.List {
		d := &dbGuildLogData{}
		d.from_pb(v)
		this.m_data[int32(d.Id)] = d
	}
	this.m_changed = false
	return
}
func (this *dbGuildLogColumn)save( )(max_id int32,data []byte,err error){
	max_id=this.m_max_id

	pb := &db.GuildLogList{}
	pb.List=make([]*db.GuildLog,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbGuildLogColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbGuildLogColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbGuildLogColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbGuildLogColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbGuildLogColumn)GetAll()(list []dbGuildLogData){
	this.m_row.m_lock.UnSafeRLock("dbGuildLogColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbGuildLogData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbGuildLogColumn)Get(id int32)(v *dbGuildLogData){
	this.m_row.m_lock.UnSafeRLock("dbGuildLogColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbGuildLogData{}
	d.clone_to(v)
	return
}
func (this *dbGuildLogColumn)Set(v dbGuildLogData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbGuildLogColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.Id)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetId(), v.Id)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbGuildLogColumn)Add(v *dbGuildLogData)(id int32){
	this.m_row.m_lock.UnSafeLock("dbGuildLogColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_max_id++
	id=this.m_max_id
	v.Id=id
	d:=&dbGuildLogData{}
	v.clone_to(d)
	this.m_data[v.Id]=d
	this.m_changed = true
	return
}
func (this *dbGuildLogColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbGuildLogColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbGuildLogColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbGuildLogColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbGuildLogData)
	this.m_changed = true
	return
}
func (this *dbGuildLogColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbGuildLogColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbGuildLogColumn)GetLogType(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbGuildLogColumn.GetLogType")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.LogType
	return v,true
}
func (this *dbGuildLogColumn)SetLogType(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbGuildLogColumn.SetLogType")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetId(), id)
		return
	}
	d.LogType = v
	this.m_changed = true
	return true
}
func (this *dbGuildLogColumn)GetPlayerId(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbGuildLogColumn.GetPlayerId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.PlayerId
	return v,true
}
func (this *dbGuildLogColumn)SetPlayerId(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbGuildLogColumn.SetPlayerId")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetId(), id)
		return
	}
	d.PlayerId = v
	this.m_changed = true
	return true
}
func (this *dbGuildLogColumn)GetTime(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbGuildLogColumn.GetTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Time
	return v,true
}
func (this *dbGuildLogColumn)SetTime(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbGuildLogColumn.SetTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetId(), id)
		return
	}
	d.Time = v
	this.m_changed = true
	return true
}
func (this *dbGuildRow)GetLastRecruitTime( )(r int32 ){
	this.m_lock.UnSafeRLock("dbGuildRow.GetdbGuildLastRecruitTimeColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_LastRecruitTime)
}
func (this *dbGuildRow)SetLastRecruitTime(v int32){
	this.m_lock.UnSafeLock("dbGuildRow.SetdbGuildLastRecruitTimeColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_LastRecruitTime=int32(v)
	this.m_LastRecruitTime_changed=true
	return
}
type dbGuildAskDonateColumn struct{
	m_row *dbGuildRow
	m_data map[int32]*dbGuildAskDonateData
	m_changed bool
}
func (this *dbGuildAskDonateColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.GuildAskDonateList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetId())
		return
	}
	for _, v := range pb.List {
		d := &dbGuildAskDonateData{}
		d.from_pb(v)
		this.m_data[int32(d.PlayerId)] = d
	}
	this.m_changed = false
	return
}
func (this *dbGuildAskDonateColumn)save( )(data []byte,err error){
	pb := &db.GuildAskDonateList{}
	pb.List=make([]*db.GuildAskDonate,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbGuildAskDonateColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbGuildAskDonateColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbGuildAskDonateColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbGuildAskDonateColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbGuildAskDonateColumn)GetAll()(list []dbGuildAskDonateData){
	this.m_row.m_lock.UnSafeRLock("dbGuildAskDonateColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbGuildAskDonateData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbGuildAskDonateColumn)Get(id int32)(v *dbGuildAskDonateData){
	this.m_row.m_lock.UnSafeRLock("dbGuildAskDonateColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbGuildAskDonateData{}
	d.clone_to(v)
	return
}
func (this *dbGuildAskDonateColumn)Set(v dbGuildAskDonateData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbGuildAskDonateColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.PlayerId)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetId(), v.PlayerId)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbGuildAskDonateColumn)Add(v *dbGuildAskDonateData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbGuildAskDonateColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.PlayerId)]
	if has {
		log.Error("already added %v %v",this.m_row.GetId(), v.PlayerId)
		return false
	}
	d:=&dbGuildAskDonateData{}
	v.clone_to(d)
	this.m_data[int32(v.PlayerId)]=d
	this.m_changed = true
	return true
}
func (this *dbGuildAskDonateColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbGuildAskDonateColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbGuildAskDonateColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbGuildAskDonateColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbGuildAskDonateData)
	this.m_changed = true
	return
}
func (this *dbGuildAskDonateColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbGuildAskDonateColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbGuildAskDonateColumn)GetItemId(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbGuildAskDonateColumn.GetItemId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.ItemId
	return v,true
}
func (this *dbGuildAskDonateColumn)SetItemId(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbGuildAskDonateColumn.SetItemId")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetId(), id)
		return
	}
	d.ItemId = v
	this.m_changed = true
	return true
}
func (this *dbGuildAskDonateColumn)GetItemNum(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbGuildAskDonateColumn.GetItemNum")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.ItemNum
	return v,true
}
func (this *dbGuildAskDonateColumn)SetItemNum(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbGuildAskDonateColumn.SetItemNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetId(), id)
		return
	}
	d.ItemNum = v
	this.m_changed = true
	return true
}
func (this *dbGuildAskDonateColumn)GetAskTime(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbGuildAskDonateColumn.GetAskTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.AskTime
	return v,true
}
func (this *dbGuildAskDonateColumn)SetAskTime(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbGuildAskDonateColumn.SetAskTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetId(), id)
		return
	}
	d.AskTime = v
	this.m_changed = true
	return true
}
type dbGuildStageColumn struct{
	m_row *dbGuildRow
	m_data *dbGuildStageData
	m_changed bool
}
func (this *dbGuildStageColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbGuildStageData{}
		this.m_changed = false
		return nil
	}
	pb := &db.GuildStage{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetId())
		return
	}
	this.m_data = &dbGuildStageData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbGuildStageColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbGuildStageColumn)Get( )(v *dbGuildStageData ){
	this.m_row.m_lock.UnSafeRLock("dbGuildStageColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbGuildStageData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbGuildStageColumn)Set(v dbGuildStageData ){
	this.m_row.m_lock.UnSafeLock("dbGuildStageColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbGuildStageData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbGuildStageColumn)GetBossId( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbGuildStageColumn.GetBossId")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.BossId
	return
}
func (this *dbGuildStageColumn)SetBossId(v int32){
	this.m_row.m_lock.UnSafeLock("dbGuildStageColumn.SetBossId")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.BossId = v
	this.m_changed = true
	return
}
func (this *dbGuildStageColumn)GetHpPercent( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbGuildStageColumn.GetHpPercent")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.HpPercent
	return
}
func (this *dbGuildStageColumn)SetHpPercent(v int32){
	this.m_row.m_lock.UnSafeLock("dbGuildStageColumn.SetHpPercent")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.HpPercent = v
	this.m_changed = true
	return
}
func (this *dbGuildStageColumn)GetBossPos( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbGuildStageColumn.GetBossPos")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.BossPos
	return
}
func (this *dbGuildStageColumn)SetBossPos(v int32){
	this.m_row.m_lock.UnSafeLock("dbGuildStageColumn.SetBossPos")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.BossPos = v
	this.m_changed = true
	return
}
func (this *dbGuildStageColumn)GetBossHP( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbGuildStageColumn.GetBossHP")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.BossHP
	return
}
func (this *dbGuildStageColumn)SetBossHP(v int32){
	this.m_row.m_lock.UnSafeLock("dbGuildStageColumn.SetBossHP")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.BossHP = v
	this.m_changed = true
	return
}
func (this *dbGuildRow)GetLastStageResetTime( )(r int32 ){
	this.m_lock.UnSafeRLock("dbGuildRow.GetdbGuildLastStageResetTimeColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_LastStageResetTime)
}
func (this *dbGuildRow)SetLastStageResetTime(v int32){
	this.m_lock.UnSafeLock("dbGuildRow.SetdbGuildLastStageResetTimeColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_LastStageResetTime=int32(v)
	this.m_LastStageResetTime_changed=true
	return
}
type dbGuildRow struct {
	m_table *dbGuildTable
	m_lock       *RWMutex
	m_loaded  bool
	m_new     bool
	m_remove  bool
	m_touch      int32
	m_releasable bool
	m_valid   bool
	m_Id        int32
	m_Name_changed bool
	m_Name string
	m_Creater_changed bool
	m_Creater int32
	m_CreateTime_changed bool
	m_CreateTime int32
	m_DismissTime_changed bool
	m_DismissTime int32
	m_Logo_changed bool
	m_Logo int32
	m_Level_changed bool
	m_Level int32
	m_Exp_changed bool
	m_Exp int32
	m_ExistType_changed bool
	m_ExistType int32
	m_Anouncement_changed bool
	m_Anouncement string
	m_President_changed bool
	m_President int32
	Members dbGuildMemberColumn
	AskLists dbGuildAskListColumn
	m_LastDonateRefreshTime_changed bool
	m_LastDonateRefreshTime int32
	Logs dbGuildLogColumn
	m_LastRecruitTime_changed bool
	m_LastRecruitTime int32
	AskDonates dbGuildAskDonateColumn
	Stage dbGuildStageColumn
	m_LastStageResetTime_changed bool
	m_LastStageResetTime int32
}
func new_dbGuildRow(table *dbGuildTable, Id int32) (r *dbGuildRow) {
	this := &dbGuildRow{}
	this.m_table = table
	this.m_Id = Id
	this.m_lock = NewRWMutex()
	this.m_Name_changed=true
	this.m_Creater_changed=true
	this.m_CreateTime_changed=true
	this.m_DismissTime_changed=true
	this.m_Logo_changed=true
	this.m_Level_changed=true
	this.m_Exp_changed=true
	this.m_ExistType_changed=true
	this.m_Anouncement_changed=true
	this.m_President_changed=true
	this.m_LastDonateRefreshTime_changed=true
	this.m_LastRecruitTime_changed=true
	this.m_LastStageResetTime_changed=true
	this.Members.m_row=this
	this.Members.m_data=make(map[int32]*dbGuildMemberData)
	this.AskLists.m_row=this
	this.AskLists.m_data=make(map[int32]*dbGuildAskListData)
	this.Logs.m_row=this
	this.Logs.m_data=make(map[int32]*dbGuildLogData)
	this.AskDonates.m_row=this
	this.AskDonates.m_data=make(map[int32]*dbGuildAskDonateData)
	this.Stage.m_row=this
	this.Stage.m_data=&dbGuildStageData{}
	return this
}
func (this *dbGuildRow) GetId() (r int32) {
	return this.m_Id
}
func (this *dbGuildRow) save_data(release bool) (err error, released bool, state int32, update_string string, args []interface{}) {
	this.m_lock.UnSafeLock("dbGuildRow.save_data")
	defer this.m_lock.UnSafeUnlock()
	if this.m_new {
		db_args:=new_db_args(20)
		db_args.Push(this.m_Id)
		db_args.Push(this.m_Name)
		db_args.Push(this.m_Creater)
		db_args.Push(this.m_CreateTime)
		db_args.Push(this.m_DismissTime)
		db_args.Push(this.m_Logo)
		db_args.Push(this.m_Level)
		db_args.Push(this.m_Exp)
		db_args.Push(this.m_ExistType)
		db_args.Push(this.m_Anouncement)
		db_args.Push(this.m_President)
		dMembers,db_err:=this.Members.save()
		if db_err!=nil{
			log.Error("insert save Member failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dMembers)
		dAskLists,db_err:=this.AskLists.save()
		if db_err!=nil{
			log.Error("insert save AskList failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dAskLists)
		db_args.Push(this.m_LastDonateRefreshTime)
		dMaxLogId,dLogs,db_err:=this.Logs.save()
		if db_err!=nil{
			log.Error("insert save Log failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dMaxLogId)
		db_args.Push(dLogs)
		db_args.Push(this.m_LastRecruitTime)
		dAskDonates,db_err:=this.AskDonates.save()
		if db_err!=nil{
			log.Error("insert save AskDonate failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dAskDonates)
		dStage,db_err:=this.Stage.save()
		if db_err!=nil{
			log.Error("insert save Stage failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dStage)
		db_args.Push(this.m_LastStageResetTime)
		args=db_args.GetArgs()
		state = 1
	} else {
		if this.m_Name_changed||this.m_Creater_changed||this.m_CreateTime_changed||this.m_DismissTime_changed||this.m_Logo_changed||this.m_Level_changed||this.m_Exp_changed||this.m_ExistType_changed||this.m_Anouncement_changed||this.m_President_changed||this.Members.m_changed||this.AskLists.m_changed||this.m_LastDonateRefreshTime_changed||this.Logs.m_changed||this.m_LastRecruitTime_changed||this.AskDonates.m_changed||this.Stage.m_changed||this.m_LastStageResetTime_changed{
			update_string = "UPDATE Guilds SET "
			db_args:=new_db_args(20)
			if this.m_Name_changed{
				update_string+="Name=?,"
				db_args.Push(this.m_Name)
			}
			if this.m_Creater_changed{
				update_string+="Creater=?,"
				db_args.Push(this.m_Creater)
			}
			if this.m_CreateTime_changed{
				update_string+="CreateTime=?,"
				db_args.Push(this.m_CreateTime)
			}
			if this.m_DismissTime_changed{
				update_string+="DismissTime=?,"
				db_args.Push(this.m_DismissTime)
			}
			if this.m_Logo_changed{
				update_string+="Logo=?,"
				db_args.Push(this.m_Logo)
			}
			if this.m_Level_changed{
				update_string+="Level=?,"
				db_args.Push(this.m_Level)
			}
			if this.m_Exp_changed{
				update_string+="Exp=?,"
				db_args.Push(this.m_Exp)
			}
			if this.m_ExistType_changed{
				update_string+="ExistType=?,"
				db_args.Push(this.m_ExistType)
			}
			if this.m_Anouncement_changed{
				update_string+="Anouncement=?,"
				db_args.Push(this.m_Anouncement)
			}
			if this.m_President_changed{
				update_string+="President=?,"
				db_args.Push(this.m_President)
			}
			if this.Members.m_changed{
				update_string+="Members=?,"
				dMembers,err:=this.Members.save()
				if err!=nil{
					log.Error("insert save Member failed")
					return err,false,0,"",nil
				}
				db_args.Push(dMembers)
			}
			if this.AskLists.m_changed{
				update_string+="AskLists=?,"
				dAskLists,err:=this.AskLists.save()
				if err!=nil{
					log.Error("insert save AskList failed")
					return err,false,0,"",nil
				}
				db_args.Push(dAskLists)
			}
			if this.m_LastDonateRefreshTime_changed{
				update_string+="LastDonateRefreshTime=?,"
				db_args.Push(this.m_LastDonateRefreshTime)
			}
			if this.Logs.m_changed{
				update_string+="MaxLogId=?,"
				update_string+="Logs=?,"
				dMaxLogId,dLogs,err:=this.Logs.save()
				if err!=nil{
					log.Error("insert save Log failed")
					return err,false,0,"",nil
				}
				db_args.Push(dMaxLogId)
				db_args.Push(dLogs)
			}
			if this.m_LastRecruitTime_changed{
				update_string+="LastRecruitTime=?,"
				db_args.Push(this.m_LastRecruitTime)
			}
			if this.AskDonates.m_changed{
				update_string+="AskDonates=?,"
				dAskDonates,err:=this.AskDonates.save()
				if err!=nil{
					log.Error("insert save AskDonate failed")
					return err,false,0,"",nil
				}
				db_args.Push(dAskDonates)
			}
			if this.Stage.m_changed{
				update_string+="Stage=?,"
				dStage,err:=this.Stage.save()
				if err!=nil{
					log.Error("update save Stage failed")
					return err,false,0,"",nil
				}
				db_args.Push(dStage)
			}
			if this.m_LastStageResetTime_changed{
				update_string+="LastStageResetTime=?,"
				db_args.Push(this.m_LastStageResetTime)
			}
			update_string = strings.TrimRight(update_string, ", ")
			update_string+=" WHERE Id=?"
			db_args.Push(this.m_Id)
			args=db_args.GetArgs()
			state = 2
		}
	}
	this.m_new = false
	this.m_Name_changed = false
	this.m_Creater_changed = false
	this.m_CreateTime_changed = false
	this.m_DismissTime_changed = false
	this.m_Logo_changed = false
	this.m_Level_changed = false
	this.m_Exp_changed = false
	this.m_ExistType_changed = false
	this.m_Anouncement_changed = false
	this.m_President_changed = false
	this.Members.m_changed = false
	this.AskLists.m_changed = false
	this.m_LastDonateRefreshTime_changed = false
	this.Logs.m_changed = false
	this.m_LastRecruitTime_changed = false
	this.AskDonates.m_changed = false
	this.Stage.m_changed = false
	this.m_LastStageResetTime_changed = false
	if release && this.m_loaded {
		atomic.AddInt32(&this.m_table.m_gc_n, -1)
		this.m_loaded = false
		released = true
	}
	return nil,released,state,update_string,args
}
func (this *dbGuildRow) Save(release bool) (err error, d bool, released bool) {
	err,released, state, update_string, args := this.save_data(release)
	if err != nil {
		log.Error("save data failed")
		return err, false, false
	}
	if state == 0 {
		d = false
	} else if state == 1 {
		_, err = this.m_table.m_dbc.StmtExec(this.m_table.m_save_insert_stmt, args...)
		if err != nil {
			log.Error("INSERT Guilds exec failed %v ", this.m_Id)
			return err, false, released
		}
		d = true
	} else if state == 2 {
		_, err = this.m_table.m_dbc.Exec(update_string, args...)
		if err != nil {
			log.Error("UPDATE Guilds exec failed %v", this.m_Id)
			return err, false, released
		}
		d = true
	}
	return nil, d, released
}
func (this *dbGuildRow) Touch(releasable bool) {
	this.m_touch = int32(time.Now().Unix())
	this.m_releasable = releasable
}
type dbGuildRowSort struct {
	rows []*dbGuildRow
}
func (this *dbGuildRowSort) Len() (length int) {
	return len(this.rows)
}
func (this *dbGuildRowSort) Less(i int, j int) (less bool) {
	return this.rows[i].m_touch < this.rows[j].m_touch
}
func (this *dbGuildRowSort) Swap(i int, j int) {
	temp := this.rows[i]
	this.rows[i] = this.rows[j]
	this.rows[j] = temp
}
type dbGuildTable struct{
	m_dbc *DBC
	m_lock *RWMutex
	m_rows map[int32]*dbGuildRow
	m_new_rows map[int32]*dbGuildRow
	m_removed_rows map[int32]*dbGuildRow
	m_gc_n int32
	m_gcing int32
	m_pool_size int32
	m_preload_select_stmt *sql.Stmt
	m_preload_max_id int32
	m_save_insert_stmt *sql.Stmt
	m_delete_stmt *sql.Stmt
}
func new_dbGuildTable(dbc *DBC) (this *dbGuildTable) {
	this = &dbGuildTable{}
	this.m_dbc = dbc
	this.m_lock = NewRWMutex()
	this.m_rows = make(map[int32]*dbGuildRow)
	this.m_new_rows = make(map[int32]*dbGuildRow)
	this.m_removed_rows = make(map[int32]*dbGuildRow)
	return this
}
func (this *dbGuildTable) check_create_table() (err error) {
	_, err = this.m_dbc.Exec("CREATE TABLE IF NOT EXISTS Guilds(Id int(11),PRIMARY KEY (Id))ENGINE=InnoDB ROW_FORMAT=DYNAMIC")
	if err != nil {
		log.Error("CREATE TABLE IF NOT EXISTS Guilds failed")
		return
	}
	rows, err := this.m_dbc.Query("SELECT COLUMN_NAME,ORDINAL_POSITION FROM information_schema.`COLUMNS` WHERE TABLE_SCHEMA=? AND TABLE_NAME='Guilds'", this.m_dbc.m_db_name)
	if err != nil {
		log.Error("SELECT information_schema failed")
		return
	}
	columns := make(map[string]int32)
	for rows.Next() {
		var column_name string
		var ordinal_position int32
		err = rows.Scan(&column_name, &ordinal_position)
		if err != nil {
			log.Error("scan information_schema row failed")
			return
		}
		if ordinal_position < 1 {
			log.Error("col ordinal out of range")
			continue
		}
		columns[column_name] = ordinal_position
	}
	_, hasName := columns["Name"]
	if !hasName {
		_, err = this.m_dbc.Exec("ALTER TABLE Guilds ADD COLUMN Name varchar(256) DEFAULT ''")
		if err != nil {
			log.Error("ADD COLUMN Name failed")
			return
		}
	}
	_, hasCreater := columns["Creater"]
	if !hasCreater {
		_, err = this.m_dbc.Exec("ALTER TABLE Guilds ADD COLUMN Creater int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN Creater failed")
			return
		}
	}
	_, hasCreateTime := columns["CreateTime"]
	if !hasCreateTime {
		_, err = this.m_dbc.Exec("ALTER TABLE Guilds ADD COLUMN CreateTime int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN CreateTime failed")
			return
		}
	}
	_, hasDismissTime := columns["DismissTime"]
	if !hasDismissTime {
		_, err = this.m_dbc.Exec("ALTER TABLE Guilds ADD COLUMN DismissTime int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN DismissTime failed")
			return
		}
	}
	_, hasLogo := columns["Logo"]
	if !hasLogo {
		_, err = this.m_dbc.Exec("ALTER TABLE Guilds ADD COLUMN Logo int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN Logo failed")
			return
		}
	}
	_, hasLevel := columns["Level"]
	if !hasLevel {
		_, err = this.m_dbc.Exec("ALTER TABLE Guilds ADD COLUMN Level int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN Level failed")
			return
		}
	}
	_, hasExp := columns["Exp"]
	if !hasExp {
		_, err = this.m_dbc.Exec("ALTER TABLE Guilds ADD COLUMN Exp int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN Exp failed")
			return
		}
	}
	_, hasExistType := columns["ExistType"]
	if !hasExistType {
		_, err = this.m_dbc.Exec("ALTER TABLE Guilds ADD COLUMN ExistType int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN ExistType failed")
			return
		}
	}
	_, hasAnouncement := columns["Anouncement"]
	if !hasAnouncement {
		_, err = this.m_dbc.Exec("ALTER TABLE Guilds ADD COLUMN Anouncement varchar(256) DEFAULT ''")
		if err != nil {
			log.Error("ADD COLUMN Anouncement failed")
			return
		}
	}
	_, hasPresident := columns["President"]
	if !hasPresident {
		_, err = this.m_dbc.Exec("ALTER TABLE Guilds ADD COLUMN President int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN President failed")
			return
		}
	}
	_, hasMember := columns["Members"]
	if !hasMember {
		_, err = this.m_dbc.Exec("ALTER TABLE Guilds ADD COLUMN Members LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Members failed")
			return
		}
	}
	_, hasAskList := columns["AskLists"]
	if !hasAskList {
		_, err = this.m_dbc.Exec("ALTER TABLE Guilds ADD COLUMN AskLists LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN AskLists failed")
			return
		}
	}
	_, hasLastDonateRefreshTime := columns["LastDonateRefreshTime"]
	if !hasLastDonateRefreshTime {
		_, err = this.m_dbc.Exec("ALTER TABLE Guilds ADD COLUMN LastDonateRefreshTime int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN LastDonateRefreshTime failed")
			return
		}
	}
	_, hasMaxLog := columns["MaxLogId"]
	if !hasMaxLog {
		_, err = this.m_dbc.Exec("ALTER TABLE Guilds ADD COLUMN MaxLogId int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN map index MaxLogId failed")
			return
		}
	}
	_, hasLog := columns["Logs"]
	if !hasLog {
		_, err = this.m_dbc.Exec("ALTER TABLE Guilds ADD COLUMN Logs LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Logs failed")
			return
		}
	}
	_, hasLastRecruitTime := columns["LastRecruitTime"]
	if !hasLastRecruitTime {
		_, err = this.m_dbc.Exec("ALTER TABLE Guilds ADD COLUMN LastRecruitTime int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN LastRecruitTime failed")
			return
		}
	}
	_, hasAskDonate := columns["AskDonates"]
	if !hasAskDonate {
		_, err = this.m_dbc.Exec("ALTER TABLE Guilds ADD COLUMN AskDonates LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN AskDonates failed")
			return
		}
	}
	_, hasStage := columns["Stage"]
	if !hasStage {
		_, err = this.m_dbc.Exec("ALTER TABLE Guilds ADD COLUMN Stage LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Stage failed")
			return
		}
	}
	_, hasLastStageResetTime := columns["LastStageResetTime"]
	if !hasLastStageResetTime {
		_, err = this.m_dbc.Exec("ALTER TABLE Guilds ADD COLUMN LastStageResetTime int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN LastStageResetTime failed")
			return
		}
	}
	return
}
func (this *dbGuildTable) prepare_preload_select_stmt() (err error) {
	this.m_preload_select_stmt,err=this.m_dbc.StmtPrepare("SELECT Id,Name,Creater,CreateTime,DismissTime,Logo,Level,Exp,ExistType,Anouncement,President,Members,AskLists,LastDonateRefreshTime,MaxLogId,Logs,LastRecruitTime,AskDonates,Stage,LastStageResetTime FROM Guilds")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbGuildTable) prepare_save_insert_stmt()(err error){
	this.m_save_insert_stmt,err=this.m_dbc.StmtPrepare("INSERT INTO Guilds (Id,Name,Creater,CreateTime,DismissTime,Logo,Level,Exp,ExistType,Anouncement,President,Members,AskLists,LastDonateRefreshTime,MaxLogId,Logs,LastRecruitTime,AskDonates,Stage,LastStageResetTime) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbGuildTable) prepare_delete_stmt() (err error) {
	this.m_delete_stmt,err=this.m_dbc.StmtPrepare("DELETE FROM Guilds WHERE Id=?")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbGuildTable) Init() (err error) {
	err=this.check_create_table()
	if err!=nil{
		log.Error("check_create_table failed")
		return
	}
	err=this.prepare_preload_select_stmt()
	if err!=nil{
		log.Error("prepare_preload_select_stmt failed")
		return
	}
	err=this.prepare_save_insert_stmt()
	if err!=nil{
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	err=this.prepare_delete_stmt()
	if err!=nil{
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	return
}
func (this *dbGuildTable) Preload() (err error) {
	r, err := this.m_dbc.StmtQuery(this.m_preload_select_stmt)
	if err != nil {
		log.Error("SELECT")
		return
	}
	var Id int32
	var dName string
	var dCreater int32
	var dCreateTime int32
	var dDismissTime int32
	var dLogo int32
	var dLevel int32
	var dExp int32
	var dExistType int32
	var dAnouncement string
	var dPresident int32
	var dMembers []byte
	var dAskLists []byte
	var dLastDonateRefreshTime int32
	var dMaxLogId int32
	var dLogs []byte
	var dLastRecruitTime int32
	var dAskDonates []byte
	var dStage []byte
	var dLastStageResetTime int32
		this.m_preload_max_id = 0
	for r.Next() {
		err = r.Scan(&Id,&dName,&dCreater,&dCreateTime,&dDismissTime,&dLogo,&dLevel,&dExp,&dExistType,&dAnouncement,&dPresident,&dMembers,&dAskLists,&dLastDonateRefreshTime,&dMaxLogId,&dLogs,&dLastRecruitTime,&dAskDonates,&dStage,&dLastStageResetTime)
		if err != nil {
			log.Error("Scan err[%v]", err.Error())
			return
		}
		if Id>this.m_preload_max_id{
			this.m_preload_max_id =Id
		}
		row := new_dbGuildRow(this,Id)
		row.m_Name=dName
		row.m_Creater=dCreater
		row.m_CreateTime=dCreateTime
		row.m_DismissTime=dDismissTime
		row.m_Logo=dLogo
		row.m_Level=dLevel
		row.m_Exp=dExp
		row.m_ExistType=dExistType
		row.m_Anouncement=dAnouncement
		row.m_President=dPresident
		err = row.Members.load(dMembers)
		if err != nil {
			log.Error("Members %v", Id)
			return
		}
		err = row.AskLists.load(dAskLists)
		if err != nil {
			log.Error("AskLists %v", Id)
			return
		}
		row.m_LastDonateRefreshTime=dLastDonateRefreshTime
		err = row.Logs.load(dMaxLogId,dLogs)
		if err != nil {
			log.Error("Logs %v", Id)
			return
		}
		row.m_LastRecruitTime=dLastRecruitTime
		err = row.AskDonates.load(dAskDonates)
		if err != nil {
			log.Error("AskDonates %v", Id)
			return
		}
		err = row.Stage.load(dStage)
		if err != nil {
			log.Error("Stage %v", Id)
			return
		}
		row.m_LastStageResetTime=dLastStageResetTime
		row.m_Name_changed=false
		row.m_Creater_changed=false
		row.m_CreateTime_changed=false
		row.m_DismissTime_changed=false
		row.m_Logo_changed=false
		row.m_Level_changed=false
		row.m_Exp_changed=false
		row.m_ExistType_changed=false
		row.m_Anouncement_changed=false
		row.m_President_changed=false
		row.m_LastDonateRefreshTime_changed=false
		row.m_LastRecruitTime_changed=false
		row.m_LastStageResetTime_changed=false
		row.m_valid = true
		this.m_rows[Id]=row
	}
	return
}
func (this *dbGuildTable) GetPreloadedMaxId() (max_id int32) {
	return this.m_preload_max_id
}
func (this *dbGuildTable) fetch_rows(rows map[int32]*dbGuildRow) (r map[int32]*dbGuildRow) {
	this.m_lock.UnSafeLock("dbGuildTable.fetch_rows")
	defer this.m_lock.UnSafeUnlock()
	r = make(map[int32]*dbGuildRow)
	for i, v := range rows {
		r[i] = v
	}
	return r
}
func (this *dbGuildTable) fetch_new_rows() (new_rows map[int32]*dbGuildRow) {
	this.m_lock.UnSafeLock("dbGuildTable.fetch_new_rows")
	defer this.m_lock.UnSafeUnlock()
	new_rows = make(map[int32]*dbGuildRow)
	for i, v := range this.m_new_rows {
		_, has := this.m_rows[i]
		if has {
			log.Error("rows already has new rows %v", i)
			continue
		}
		this.m_rows[i] = v
		new_rows[i] = v
	}
	for i, _ := range new_rows {
		delete(this.m_new_rows, i)
	}
	return
}
func (this *dbGuildTable) save_rows(rows map[int32]*dbGuildRow, quick bool) {
	for _, v := range rows {
		if this.m_dbc.m_quit && !quick {
			return
		}
		err, delay, _ := v.Save(false)
		if err != nil {
			log.Error("save failed %v", err)
		}
		if this.m_dbc.m_quit && !quick {
			return
		}
		if delay&&!quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
}
func (this *dbGuildTable) Save(quick bool) (err error){
	removed_rows := this.fetch_rows(this.m_removed_rows)
	for _, v := range removed_rows {
		_, err := this.m_dbc.StmtExec(this.m_delete_stmt, v.GetId())
		if err != nil {
			log.Error("exec delete stmt failed %v", err)
		}
		v.m_valid = false
		if !quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
	this.m_removed_rows = make(map[int32]*dbGuildRow)
	rows := this.fetch_rows(this.m_rows)
	this.save_rows(rows, quick)
	new_rows := this.fetch_new_rows()
	this.save_rows(new_rows, quick)
	return
}
func (this *dbGuildTable) AddRow(Id int32) (row *dbGuildRow) {
	this.m_lock.UnSafeLock("dbGuildTable.AddRow")
	defer this.m_lock.UnSafeUnlock()
	row = new_dbGuildRow(this,Id)
	row.m_new = true
	row.m_loaded = true
	row.m_valid = true
	_, has := this.m_new_rows[Id]
	if has{
		log.Error("已经存在 %v", Id)
		return nil
	}
	this.m_new_rows[Id] = row
	atomic.AddInt32(&this.m_gc_n,1)
	return row
}
func (this *dbGuildTable) RemoveRow(Id int32) {
	this.m_lock.UnSafeLock("dbGuildTable.RemoveRow")
	defer this.m_lock.UnSafeUnlock()
	row := this.m_rows[Id]
	if row != nil {
		row.m_remove = true
		delete(this.m_rows, Id)
		rm_row := this.m_removed_rows[Id]
		if rm_row != nil {
			log.Error("rows and removed rows both has %v", Id)
		}
		this.m_removed_rows[Id] = row
		_, has_new := this.m_new_rows[Id]
		if has_new {
			delete(this.m_new_rows, Id)
			log.Error("rows and new_rows both has %v", Id)
		}
	} else {
		row = this.m_removed_rows[Id]
		if row == nil {
			_, has_new := this.m_new_rows[Id]
			if has_new {
				delete(this.m_new_rows, Id)
			} else {
				log.Error("row not exist %v", Id)
			}
		} else {
			log.Error("already removed %v", Id)
			_, has_new := this.m_new_rows[Id]
			if has_new {
				delete(this.m_new_rows, Id)
				log.Error("removed rows and new_rows both has %v", Id)
			}
		}
	}
}
func (this *dbGuildTable) GetRow(Id int32) (row *dbGuildRow) {
	this.m_lock.UnSafeRLock("dbGuildTable.GetRow")
	defer this.m_lock.UnSafeRUnlock()
	row = this.m_rows[Id]
	if row == nil {
		row = this.m_new_rows[Id]
	}
	return row
}
type dbGuildStageDamageLogColumn struct{
	m_row *dbGuildStageRow
	m_data map[int32]*dbGuildStageDamageLogData
	m_changed bool
}
func (this *dbGuildStageDamageLogColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.GuildStageDamageLogList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetId())
		return
	}
	for _, v := range pb.List {
		d := &dbGuildStageDamageLogData{}
		d.from_pb(v)
		this.m_data[int32(d.AttackerId)] = d
	}
	this.m_changed = false
	return
}
func (this *dbGuildStageDamageLogColumn)save( )(data []byte,err error){
	pb := &db.GuildStageDamageLogList{}
	pb.List=make([]*db.GuildStageDamageLog,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbGuildStageDamageLogColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbGuildStageDamageLogColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbGuildStageDamageLogColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbGuildStageDamageLogColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbGuildStageDamageLogColumn)GetAll()(list []dbGuildStageDamageLogData){
	this.m_row.m_lock.UnSafeRLock("dbGuildStageDamageLogColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbGuildStageDamageLogData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbGuildStageDamageLogColumn)Get(id int32)(v *dbGuildStageDamageLogData){
	this.m_row.m_lock.UnSafeRLock("dbGuildStageDamageLogColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbGuildStageDamageLogData{}
	d.clone_to(v)
	return
}
func (this *dbGuildStageDamageLogColumn)Set(v dbGuildStageDamageLogData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbGuildStageDamageLogColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.AttackerId)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetId(), v.AttackerId)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbGuildStageDamageLogColumn)Add(v *dbGuildStageDamageLogData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbGuildStageDamageLogColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.AttackerId)]
	if has {
		log.Error("already added %v %v",this.m_row.GetId(), v.AttackerId)
		return false
	}
	d:=&dbGuildStageDamageLogData{}
	v.clone_to(d)
	this.m_data[int32(v.AttackerId)]=d
	this.m_changed = true
	return true
}
func (this *dbGuildStageDamageLogColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbGuildStageDamageLogColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbGuildStageDamageLogColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbGuildStageDamageLogColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbGuildStageDamageLogData)
	this.m_changed = true
	return
}
func (this *dbGuildStageDamageLogColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbGuildStageDamageLogColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbGuildStageDamageLogColumn)GetDamage(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbGuildStageDamageLogColumn.GetDamage")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Damage
	return v,true
}
func (this *dbGuildStageDamageLogColumn)SetDamage(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbGuildStageDamageLogColumn.SetDamage")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetId(), id)
		return
	}
	d.Damage = v
	this.m_changed = true
	return true
}
type dbGuildStageRow struct {
	m_table *dbGuildStageTable
	m_lock       *RWMutex
	m_loaded  bool
	m_new     bool
	m_remove  bool
	m_touch      int32
	m_releasable bool
	m_valid   bool
	m_Id        int64
	DamageLogs dbGuildStageDamageLogColumn
}
func new_dbGuildStageRow(table *dbGuildStageTable, Id int64) (r *dbGuildStageRow) {
	this := &dbGuildStageRow{}
	this.m_table = table
	this.m_Id = Id
	this.m_lock = NewRWMutex()
	this.DamageLogs.m_row=this
	this.DamageLogs.m_data=make(map[int32]*dbGuildStageDamageLogData)
	return this
}
func (this *dbGuildStageRow) GetId() (r int64) {
	return this.m_Id
}
func (this *dbGuildStageRow) save_data(release bool) (err error, released bool, state int32, update_string string, args []interface{}) {
	this.m_lock.UnSafeLock("dbGuildStageRow.save_data")
	defer this.m_lock.UnSafeUnlock()
	if this.m_new {
		db_args:=new_db_args(2)
		db_args.Push(this.m_Id)
		dDamageLogs,db_err:=this.DamageLogs.save()
		if db_err!=nil{
			log.Error("insert save DamageLog failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dDamageLogs)
		args=db_args.GetArgs()
		state = 1
	} else {
		if this.DamageLogs.m_changed{
			update_string = "UPDATE GuildStages SET "
			db_args:=new_db_args(2)
			if this.DamageLogs.m_changed{
				update_string+="DamageLogs=?,"
				dDamageLogs,err:=this.DamageLogs.save()
				if err!=nil{
					log.Error("insert save DamageLog failed")
					return err,false,0,"",nil
				}
				db_args.Push(dDamageLogs)
			}
			update_string = strings.TrimRight(update_string, ", ")
			update_string+=" WHERE Id=?"
			db_args.Push(this.m_Id)
			args=db_args.GetArgs()
			state = 2
		}
	}
	this.m_new = false
	this.DamageLogs.m_changed = false
	if release && this.m_loaded {
		atomic.AddInt32(&this.m_table.m_gc_n, -1)
		this.m_loaded = false
		released = true
	}
	return nil,released,state,update_string,args
}
func (this *dbGuildStageRow) Save(release bool) (err error, d bool, released bool) {
	err,released, state, update_string, args := this.save_data(release)
	if err != nil {
		log.Error("save data failed")
		return err, false, false
	}
	if state == 0 {
		d = false
	} else if state == 1 {
		_, err = this.m_table.m_dbc.StmtExec(this.m_table.m_save_insert_stmt, args...)
		if err != nil {
			log.Error("INSERT GuildStages exec failed %v ", this.m_Id)
			return err, false, released
		}
		d = true
	} else if state == 2 {
		_, err = this.m_table.m_dbc.Exec(update_string, args...)
		if err != nil {
			log.Error("UPDATE GuildStages exec failed %v", this.m_Id)
			return err, false, released
		}
		d = true
	}
	return nil, d, released
}
func (this *dbGuildStageRow) Touch(releasable bool) {
	this.m_touch = int32(time.Now().Unix())
	this.m_releasable = releasable
}
type dbGuildStageRowSort struct {
	rows []*dbGuildStageRow
}
func (this *dbGuildStageRowSort) Len() (length int) {
	return len(this.rows)
}
func (this *dbGuildStageRowSort) Less(i int, j int) (less bool) {
	return this.rows[i].m_touch < this.rows[j].m_touch
}
func (this *dbGuildStageRowSort) Swap(i int, j int) {
	temp := this.rows[i]
	this.rows[i] = this.rows[j]
	this.rows[j] = temp
}
type dbGuildStageTable struct{
	m_dbc *DBC
	m_lock *RWMutex
	m_rows map[int64]*dbGuildStageRow
	m_new_rows map[int64]*dbGuildStageRow
	m_removed_rows map[int64]*dbGuildStageRow
	m_gc_n int32
	m_gcing int32
	m_pool_size int32
	m_preload_select_stmt *sql.Stmt
	m_preload_max_id int64
	m_save_insert_stmt *sql.Stmt
	m_delete_stmt *sql.Stmt
}
func new_dbGuildStageTable(dbc *DBC) (this *dbGuildStageTable) {
	this = &dbGuildStageTable{}
	this.m_dbc = dbc
	this.m_lock = NewRWMutex()
	this.m_rows = make(map[int64]*dbGuildStageRow)
	this.m_new_rows = make(map[int64]*dbGuildStageRow)
	this.m_removed_rows = make(map[int64]*dbGuildStageRow)
	return this
}
func (this *dbGuildStageTable) check_create_table() (err error) {
	_, err = this.m_dbc.Exec("CREATE TABLE IF NOT EXISTS GuildStages(Id bigint(20),PRIMARY KEY (Id))ENGINE=InnoDB ROW_FORMAT=DYNAMIC")
	if err != nil {
		log.Error("CREATE TABLE IF NOT EXISTS GuildStages failed")
		return
	}
	rows, err := this.m_dbc.Query("SELECT COLUMN_NAME,ORDINAL_POSITION FROM information_schema.`COLUMNS` WHERE TABLE_SCHEMA=? AND TABLE_NAME='GuildStages'", this.m_dbc.m_db_name)
	if err != nil {
		log.Error("SELECT information_schema failed")
		return
	}
	columns := make(map[string]int32)
	for rows.Next() {
		var column_name string
		var ordinal_position int32
		err = rows.Scan(&column_name, &ordinal_position)
		if err != nil {
			log.Error("scan information_schema row failed")
			return
		}
		if ordinal_position < 1 {
			log.Error("col ordinal out of range")
			continue
		}
		columns[column_name] = ordinal_position
	}
	_, hasDamageLog := columns["DamageLogs"]
	if !hasDamageLog {
		_, err = this.m_dbc.Exec("ALTER TABLE GuildStages ADD COLUMN DamageLogs LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN DamageLogs failed")
			return
		}
	}
	return
}
func (this *dbGuildStageTable) prepare_preload_select_stmt() (err error) {
	this.m_preload_select_stmt,err=this.m_dbc.StmtPrepare("SELECT Id,DamageLogs FROM GuildStages")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbGuildStageTable) prepare_save_insert_stmt()(err error){
	this.m_save_insert_stmt,err=this.m_dbc.StmtPrepare("INSERT INTO GuildStages (Id,DamageLogs) VALUES (?,?)")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbGuildStageTable) prepare_delete_stmt() (err error) {
	this.m_delete_stmt,err=this.m_dbc.StmtPrepare("DELETE FROM GuildStages WHERE Id=?")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbGuildStageTable) Init() (err error) {
	err=this.check_create_table()
	if err!=nil{
		log.Error("check_create_table failed")
		return
	}
	err=this.prepare_preload_select_stmt()
	if err!=nil{
		log.Error("prepare_preload_select_stmt failed")
		return
	}
	err=this.prepare_save_insert_stmt()
	if err!=nil{
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	err=this.prepare_delete_stmt()
	if err!=nil{
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	return
}
func (this *dbGuildStageTable) Preload() (err error) {
	r, err := this.m_dbc.StmtQuery(this.m_preload_select_stmt)
	if err != nil {
		log.Error("SELECT")
		return
	}
	var Id int64
	var dDamageLogs []byte
		this.m_preload_max_id = 0
	for r.Next() {
		err = r.Scan(&Id,&dDamageLogs)
		if err != nil {
			log.Error("Scan err[%v]", err.Error())
			return
		}
		if Id>this.m_preload_max_id{
			this.m_preload_max_id =Id
		}
		row := new_dbGuildStageRow(this,Id)
		err = row.DamageLogs.load(dDamageLogs)
		if err != nil {
			log.Error("DamageLogs %v", Id)
			return
		}
		row.m_valid = true
		this.m_rows[Id]=row
	}
	return
}
func (this *dbGuildStageTable) GetPreloadedMaxId() (max_id int64) {
	return this.m_preload_max_id
}
func (this *dbGuildStageTable) fetch_rows(rows map[int64]*dbGuildStageRow) (r map[int64]*dbGuildStageRow) {
	this.m_lock.UnSafeLock("dbGuildStageTable.fetch_rows")
	defer this.m_lock.UnSafeUnlock()
	r = make(map[int64]*dbGuildStageRow)
	for i, v := range rows {
		r[i] = v
	}
	return r
}
func (this *dbGuildStageTable) fetch_new_rows() (new_rows map[int64]*dbGuildStageRow) {
	this.m_lock.UnSafeLock("dbGuildStageTable.fetch_new_rows")
	defer this.m_lock.UnSafeUnlock()
	new_rows = make(map[int64]*dbGuildStageRow)
	for i, v := range this.m_new_rows {
		_, has := this.m_rows[i]
		if has {
			log.Error("rows already has new rows %v", i)
			continue
		}
		this.m_rows[i] = v
		new_rows[i] = v
	}
	for i, _ := range new_rows {
		delete(this.m_new_rows, i)
	}
	return
}
func (this *dbGuildStageTable) save_rows(rows map[int64]*dbGuildStageRow, quick bool) {
	for _, v := range rows {
		if this.m_dbc.m_quit && !quick {
			return
		}
		err, delay, _ := v.Save(false)
		if err != nil {
			log.Error("save failed %v", err)
		}
		if this.m_dbc.m_quit && !quick {
			return
		}
		if delay&&!quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
}
func (this *dbGuildStageTable) Save(quick bool) (err error){
	removed_rows := this.fetch_rows(this.m_removed_rows)
	for _, v := range removed_rows {
		_, err := this.m_dbc.StmtExec(this.m_delete_stmt, v.GetId())
		if err != nil {
			log.Error("exec delete stmt failed %v", err)
		}
		v.m_valid = false
		if !quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
	this.m_removed_rows = make(map[int64]*dbGuildStageRow)
	rows := this.fetch_rows(this.m_rows)
	this.save_rows(rows, quick)
	new_rows := this.fetch_new_rows()
	this.save_rows(new_rows, quick)
	return
}
func (this *dbGuildStageTable) AddRow(Id int64) (row *dbGuildStageRow) {
	this.m_lock.UnSafeLock("dbGuildStageTable.AddRow")
	defer this.m_lock.UnSafeUnlock()
	row = new_dbGuildStageRow(this,Id)
	row.m_new = true
	row.m_loaded = true
	row.m_valid = true
	_, has := this.m_new_rows[Id]
	if has{
		log.Error("已经存在 %v", Id)
		return nil
	}
	this.m_new_rows[Id] = row
	atomic.AddInt32(&this.m_gc_n,1)
	return row
}
func (this *dbGuildStageTable) RemoveRow(Id int64) {
	this.m_lock.UnSafeLock("dbGuildStageTable.RemoveRow")
	defer this.m_lock.UnSafeUnlock()
	row := this.m_rows[Id]
	if row != nil {
		row.m_remove = true
		delete(this.m_rows, Id)
		rm_row := this.m_removed_rows[Id]
		if rm_row != nil {
			log.Error("rows and removed rows both has %v", Id)
		}
		this.m_removed_rows[Id] = row
		_, has_new := this.m_new_rows[Id]
		if has_new {
			delete(this.m_new_rows, Id)
			log.Error("rows and new_rows both has %v", Id)
		}
	} else {
		row = this.m_removed_rows[Id]
		if row == nil {
			_, has_new := this.m_new_rows[Id]
			if has_new {
				delete(this.m_new_rows, Id)
			} else {
				log.Error("row not exist %v", Id)
			}
		} else {
			log.Error("already removed %v", Id)
			_, has_new := this.m_new_rows[Id]
			if has_new {
				delete(this.m_new_rows, Id)
				log.Error("removed rows and new_rows both has %v", Id)
			}
		}
	}
}
func (this *dbGuildStageTable) GetRow(Id int64) (row *dbGuildStageRow) {
	this.m_lock.UnSafeRLock("dbGuildStageTable.GetRow")
	defer this.m_lock.UnSafeRUnlock()
	row = this.m_rows[Id]
	if row == nil {
		row = this.m_new_rows[Id]
	}
	return row
}
func (this *dbActivitysToDeleteRow)GetStartTime( )(r int32 ){
	this.m_lock.UnSafeRLock("dbActivitysToDeleteRow.GetdbActivitysToDeleteStartTimeColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_StartTime)
}
func (this *dbActivitysToDeleteRow)SetStartTime(v int32){
	this.m_lock.UnSafeLock("dbActivitysToDeleteRow.SetdbActivitysToDeleteStartTimeColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_StartTime=int32(v)
	this.m_StartTime_changed=true
	return
}
func (this *dbActivitysToDeleteRow)GetEndTime( )(r int32 ){
	this.m_lock.UnSafeRLock("dbActivitysToDeleteRow.GetdbActivitysToDeleteEndTimeColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_EndTime)
}
func (this *dbActivitysToDeleteRow)SetEndTime(v int32){
	this.m_lock.UnSafeLock("dbActivitysToDeleteRow.SetdbActivitysToDeleteEndTimeColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_EndTime=int32(v)
	this.m_EndTime_changed=true
	return
}
type dbActivitysToDeleteRow struct {
	m_table *dbActivitysToDeleteTable
	m_lock       *RWMutex
	m_loaded  bool
	m_new     bool
	m_remove  bool
	m_touch      int32
	m_releasable bool
	m_valid   bool
	m_Id        int32
	m_StartTime_changed bool
	m_StartTime int32
	m_EndTime_changed bool
	m_EndTime int32
}
func new_dbActivitysToDeleteRow(table *dbActivitysToDeleteTable, Id int32) (r *dbActivitysToDeleteRow) {
	this := &dbActivitysToDeleteRow{}
	this.m_table = table
	this.m_Id = Id
	this.m_lock = NewRWMutex()
	this.m_StartTime_changed=true
	this.m_EndTime_changed=true
	return this
}
func (this *dbActivitysToDeleteRow) GetId() (r int32) {
	return this.m_Id
}
func (this *dbActivitysToDeleteRow) save_data(release bool) (err error, released bool, state int32, update_string string, args []interface{}) {
	this.m_lock.UnSafeLock("dbActivitysToDeleteRow.save_data")
	defer this.m_lock.UnSafeUnlock()
	if this.m_new {
		db_args:=new_db_args(3)
		db_args.Push(this.m_Id)
		db_args.Push(this.m_StartTime)
		db_args.Push(this.m_EndTime)
		args=db_args.GetArgs()
		state = 1
	} else {
		if this.m_StartTime_changed||this.m_EndTime_changed{
			update_string = "UPDATE ActivitysToDeletes SET "
			db_args:=new_db_args(3)
			if this.m_StartTime_changed{
				update_string+="StartTime=?,"
				db_args.Push(this.m_StartTime)
			}
			if this.m_EndTime_changed{
				update_string+="EndTime=?,"
				db_args.Push(this.m_EndTime)
			}
			update_string = strings.TrimRight(update_string, ", ")
			update_string+=" WHERE Id=?"
			db_args.Push(this.m_Id)
			args=db_args.GetArgs()
			state = 2
		}
	}
	this.m_new = false
	this.m_StartTime_changed = false
	this.m_EndTime_changed = false
	if release && this.m_loaded {
		atomic.AddInt32(&this.m_table.m_gc_n, -1)
		this.m_loaded = false
		released = true
	}
	return nil,released,state,update_string,args
}
func (this *dbActivitysToDeleteRow) Save(release bool) (err error, d bool, released bool) {
	err,released, state, update_string, args := this.save_data(release)
	if err != nil {
		log.Error("save data failed")
		return err, false, false
	}
	if state == 0 {
		d = false
	} else if state == 1 {
		_, err = this.m_table.m_dbc.StmtExec(this.m_table.m_save_insert_stmt, args...)
		if err != nil {
			log.Error("INSERT ActivitysToDeletes exec failed %v ", this.m_Id)
			return err, false, released
		}
		d = true
	} else if state == 2 {
		_, err = this.m_table.m_dbc.Exec(update_string, args...)
		if err != nil {
			log.Error("UPDATE ActivitysToDeletes exec failed %v", this.m_Id)
			return err, false, released
		}
		d = true
	}
	return nil, d, released
}
func (this *dbActivitysToDeleteRow) Touch(releasable bool) {
	this.m_touch = int32(time.Now().Unix())
	this.m_releasable = releasable
}
type dbActivitysToDeleteRowSort struct {
	rows []*dbActivitysToDeleteRow
}
func (this *dbActivitysToDeleteRowSort) Len() (length int) {
	return len(this.rows)
}
func (this *dbActivitysToDeleteRowSort) Less(i int, j int) (less bool) {
	return this.rows[i].m_touch < this.rows[j].m_touch
}
func (this *dbActivitysToDeleteRowSort) Swap(i int, j int) {
	temp := this.rows[i]
	this.rows[i] = this.rows[j]
	this.rows[j] = temp
}
type dbActivitysToDeleteTable struct{
	m_dbc *DBC
	m_lock *RWMutex
	m_rows map[int32]*dbActivitysToDeleteRow
	m_new_rows map[int32]*dbActivitysToDeleteRow
	m_removed_rows map[int32]*dbActivitysToDeleteRow
	m_gc_n int32
	m_gcing int32
	m_pool_size int32
	m_preload_select_stmt *sql.Stmt
	m_preload_max_id int32
	m_save_insert_stmt *sql.Stmt
	m_delete_stmt *sql.Stmt
}
func new_dbActivitysToDeleteTable(dbc *DBC) (this *dbActivitysToDeleteTable) {
	this = &dbActivitysToDeleteTable{}
	this.m_dbc = dbc
	this.m_lock = NewRWMutex()
	this.m_rows = make(map[int32]*dbActivitysToDeleteRow)
	this.m_new_rows = make(map[int32]*dbActivitysToDeleteRow)
	this.m_removed_rows = make(map[int32]*dbActivitysToDeleteRow)
	return this
}
func (this *dbActivitysToDeleteTable) check_create_table() (err error) {
	_, err = this.m_dbc.Exec("CREATE TABLE IF NOT EXISTS ActivitysToDeletes(Id int(11),PRIMARY KEY (Id))ENGINE=InnoDB ROW_FORMAT=DYNAMIC")
	if err != nil {
		log.Error("CREATE TABLE IF NOT EXISTS ActivitysToDeletes failed")
		return
	}
	rows, err := this.m_dbc.Query("SELECT COLUMN_NAME,ORDINAL_POSITION FROM information_schema.`COLUMNS` WHERE TABLE_SCHEMA=? AND TABLE_NAME='ActivitysToDeletes'", this.m_dbc.m_db_name)
	if err != nil {
		log.Error("SELECT information_schema failed")
		return
	}
	columns := make(map[string]int32)
	for rows.Next() {
		var column_name string
		var ordinal_position int32
		err = rows.Scan(&column_name, &ordinal_position)
		if err != nil {
			log.Error("scan information_schema row failed")
			return
		}
		if ordinal_position < 1 {
			log.Error("col ordinal out of range")
			continue
		}
		columns[column_name] = ordinal_position
	}
	_, hasStartTime := columns["StartTime"]
	if !hasStartTime {
		_, err = this.m_dbc.Exec("ALTER TABLE ActivitysToDeletes ADD COLUMN StartTime int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN StartTime failed")
			return
		}
	}
	_, hasEndTime := columns["EndTime"]
	if !hasEndTime {
		_, err = this.m_dbc.Exec("ALTER TABLE ActivitysToDeletes ADD COLUMN EndTime int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN EndTime failed")
			return
		}
	}
	return
}
func (this *dbActivitysToDeleteTable) prepare_preload_select_stmt() (err error) {
	this.m_preload_select_stmt,err=this.m_dbc.StmtPrepare("SELECT Id,StartTime,EndTime FROM ActivitysToDeletes")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbActivitysToDeleteTable) prepare_save_insert_stmt()(err error){
	this.m_save_insert_stmt,err=this.m_dbc.StmtPrepare("INSERT INTO ActivitysToDeletes (Id,StartTime,EndTime) VALUES (?,?,?)")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbActivitysToDeleteTable) prepare_delete_stmt() (err error) {
	this.m_delete_stmt,err=this.m_dbc.StmtPrepare("DELETE FROM ActivitysToDeletes WHERE Id=?")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbActivitysToDeleteTable) Init() (err error) {
	err=this.check_create_table()
	if err!=nil{
		log.Error("check_create_table failed")
		return
	}
	err=this.prepare_preload_select_stmt()
	if err!=nil{
		log.Error("prepare_preload_select_stmt failed")
		return
	}
	err=this.prepare_save_insert_stmt()
	if err!=nil{
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	err=this.prepare_delete_stmt()
	if err!=nil{
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	return
}
func (this *dbActivitysToDeleteTable) Preload() (err error) {
	r, err := this.m_dbc.StmtQuery(this.m_preload_select_stmt)
	if err != nil {
		log.Error("SELECT")
		return
	}
	var Id int32
	var dStartTime int32
	var dEndTime int32
		this.m_preload_max_id = 0
	for r.Next() {
		err = r.Scan(&Id,&dStartTime,&dEndTime)
		if err != nil {
			log.Error("Scan err[%v]", err.Error())
			return
		}
		if Id>this.m_preload_max_id{
			this.m_preload_max_id =Id
		}
		row := new_dbActivitysToDeleteRow(this,Id)
		row.m_StartTime=dStartTime
		row.m_EndTime=dEndTime
		row.m_StartTime_changed=false
		row.m_EndTime_changed=false
		row.m_valid = true
		this.m_rows[Id]=row
	}
	return
}
func (this *dbActivitysToDeleteTable) GetPreloadedMaxId() (max_id int32) {
	return this.m_preload_max_id
}
func (this *dbActivitysToDeleteTable) fetch_rows(rows map[int32]*dbActivitysToDeleteRow) (r map[int32]*dbActivitysToDeleteRow) {
	this.m_lock.UnSafeLock("dbActivitysToDeleteTable.fetch_rows")
	defer this.m_lock.UnSafeUnlock()
	r = make(map[int32]*dbActivitysToDeleteRow)
	for i, v := range rows {
		r[i] = v
	}
	return r
}
func (this *dbActivitysToDeleteTable) fetch_new_rows() (new_rows map[int32]*dbActivitysToDeleteRow) {
	this.m_lock.UnSafeLock("dbActivitysToDeleteTable.fetch_new_rows")
	defer this.m_lock.UnSafeUnlock()
	new_rows = make(map[int32]*dbActivitysToDeleteRow)
	for i, v := range this.m_new_rows {
		_, has := this.m_rows[i]
		if has {
			log.Error("rows already has new rows %v", i)
			continue
		}
		this.m_rows[i] = v
		new_rows[i] = v
	}
	for i, _ := range new_rows {
		delete(this.m_new_rows, i)
	}
	return
}
func (this *dbActivitysToDeleteTable) save_rows(rows map[int32]*dbActivitysToDeleteRow, quick bool) {
	for _, v := range rows {
		if this.m_dbc.m_quit && !quick {
			return
		}
		err, delay, _ := v.Save(false)
		if err != nil {
			log.Error("save failed %v", err)
		}
		if this.m_dbc.m_quit && !quick {
			return
		}
		if delay&&!quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
}
func (this *dbActivitysToDeleteTable) Save(quick bool) (err error){
	removed_rows := this.fetch_rows(this.m_removed_rows)
	for _, v := range removed_rows {
		_, err := this.m_dbc.StmtExec(this.m_delete_stmt, v.GetId())
		if err != nil {
			log.Error("exec delete stmt failed %v", err)
		}
		v.m_valid = false
		if !quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
	this.m_removed_rows = make(map[int32]*dbActivitysToDeleteRow)
	rows := this.fetch_rows(this.m_rows)
	this.save_rows(rows, quick)
	new_rows := this.fetch_new_rows()
	this.save_rows(new_rows, quick)
	return
}
func (this *dbActivitysToDeleteTable) AddRow(Id int32) (row *dbActivitysToDeleteRow) {
	this.m_lock.UnSafeLock("dbActivitysToDeleteTable.AddRow")
	defer this.m_lock.UnSafeUnlock()
	row = new_dbActivitysToDeleteRow(this,Id)
	row.m_new = true
	row.m_loaded = true
	row.m_valid = true
	_, has := this.m_new_rows[Id]
	if has{
		log.Error("已经存在 %v", Id)
		return nil
	}
	this.m_new_rows[Id] = row
	atomic.AddInt32(&this.m_gc_n,1)
	return row
}
func (this *dbActivitysToDeleteTable) RemoveRow(Id int32) {
	this.m_lock.UnSafeLock("dbActivitysToDeleteTable.RemoveRow")
	defer this.m_lock.UnSafeUnlock()
	row := this.m_rows[Id]
	if row != nil {
		row.m_remove = true
		delete(this.m_rows, Id)
		rm_row := this.m_removed_rows[Id]
		if rm_row != nil {
			log.Error("rows and removed rows both has %v", Id)
		}
		this.m_removed_rows[Id] = row
		_, has_new := this.m_new_rows[Id]
		if has_new {
			delete(this.m_new_rows, Id)
			log.Error("rows and new_rows both has %v", Id)
		}
	} else {
		row = this.m_removed_rows[Id]
		if row == nil {
			_, has_new := this.m_new_rows[Id]
			if has_new {
				delete(this.m_new_rows, Id)
			} else {
				log.Error("row not exist %v", Id)
			}
		} else {
			log.Error("already removed %v", Id)
			_, has_new := this.m_new_rows[Id]
			if has_new {
				delete(this.m_new_rows, Id)
				log.Error("removed rows and new_rows both has %v", Id)
			}
		}
	}
}
func (this *dbActivitysToDeleteTable) GetRow(Id int32) (row *dbActivitysToDeleteRow) {
	this.m_lock.UnSafeRLock("dbActivitysToDeleteTable.GetRow")
	defer this.m_lock.UnSafeRUnlock()
	row = this.m_rows[Id]
	if row == nil {
		row = this.m_new_rows[Id]
	}
	return row
}
func (this *dbSysMailCommonRow)GetCurrMailId( )(r int32 ){
	this.m_lock.UnSafeRLock("dbSysMailCommonRow.GetdbSysMailCommonCurrMailIdColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_CurrMailId)
}
func (this *dbSysMailCommonRow)SetCurrMailId(v int32){
	this.m_lock.UnSafeLock("dbSysMailCommonRow.SetdbSysMailCommonCurrMailIdColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_CurrMailId=int32(v)
	this.m_CurrMailId_changed=true
	return
}
type dbSysMailCommonRow struct {
	m_table *dbSysMailCommonTable
	m_lock       *RWMutex
	m_loaded  bool
	m_new     bool
	m_remove  bool
	m_touch      int32
	m_releasable bool
	m_valid   bool
	m_Id        int32
	m_CurrMailId_changed bool
	m_CurrMailId int32
}
func new_dbSysMailCommonRow(table *dbSysMailCommonTable, Id int32) (r *dbSysMailCommonRow) {
	this := &dbSysMailCommonRow{}
	this.m_table = table
	this.m_Id = Id
	this.m_lock = NewRWMutex()
	this.m_CurrMailId_changed=true
	return this
}
func (this *dbSysMailCommonRow) save_data(release bool) (err error, released bool, state int32, update_string string, args []interface{}) {
	this.m_lock.UnSafeLock("dbSysMailCommonRow.save_data")
	defer this.m_lock.UnSafeUnlock()
	if this.m_new {
		db_args:=new_db_args(2)
		db_args.Push(this.m_Id)
		db_args.Push(this.m_CurrMailId)
		args=db_args.GetArgs()
		state = 1
	} else {
		if this.m_CurrMailId_changed{
			update_string = "UPDATE SysMailCommon SET "
			db_args:=new_db_args(2)
			if this.m_CurrMailId_changed{
				update_string+="CurrMailId=?,"
				db_args.Push(this.m_CurrMailId)
			}
			update_string = strings.TrimRight(update_string, ", ")
			update_string+=" WHERE Id=?"
			db_args.Push(this.m_Id)
			args=db_args.GetArgs()
			state = 2
		}
	}
	this.m_new = false
	this.m_CurrMailId_changed = false
	if release && this.m_loaded {
		this.m_loaded = false
		released = true
	}
	return nil,released,state,update_string,args
}
func (this *dbSysMailCommonRow) Save(release bool) (err error, d bool, released bool) {
	err,released, state, update_string, args := this.save_data(release)
	if err != nil {
		log.Error("save data failed")
		return err, false, false
	}
	if state == 0 {
		d = false
	} else if state == 1 {
		_, err = this.m_table.m_dbc.StmtExec(this.m_table.m_save_insert_stmt, args...)
		if err != nil {
			log.Error("INSERT SysMailCommon exec failed %v ", this.m_Id)
			return err, false, released
		}
		d = true
	} else if state == 2 {
		_, err = this.m_table.m_dbc.Exec(update_string, args...)
		if err != nil {
			log.Error("UPDATE SysMailCommon exec failed %v", this.m_Id)
			return err, false, released
		}
		d = true
	}
	return nil, d, released
}
type dbSysMailCommonTable struct{
	m_dbc *DBC
	m_lock *RWMutex
	m_row *dbSysMailCommonRow
	m_preload_select_stmt *sql.Stmt
	m_save_insert_stmt *sql.Stmt
}
func new_dbSysMailCommonTable(dbc *DBC) (this *dbSysMailCommonTable) {
	this = &dbSysMailCommonTable{}
	this.m_dbc = dbc
	this.m_lock = NewRWMutex()
	return this
}
func (this *dbSysMailCommonTable) check_create_table() (err error) {
	_, err = this.m_dbc.Exec("CREATE TABLE IF NOT EXISTS SysMailCommon(Id int(11),PRIMARY KEY (Id))ENGINE=InnoDB ROW_FORMAT=DYNAMIC")
	if err != nil {
		log.Error("CREATE TABLE IF NOT EXISTS SysMailCommon failed")
		return
	}
	rows, err := this.m_dbc.Query("SELECT COLUMN_NAME,ORDINAL_POSITION FROM information_schema.`COLUMNS` WHERE TABLE_SCHEMA=? AND TABLE_NAME='SysMailCommon'", this.m_dbc.m_db_name)
	if err != nil {
		log.Error("SELECT information_schema failed")
		return
	}
	columns := make(map[string]int32)
	for rows.Next() {
		var column_name string
		var ordinal_position int32
		err = rows.Scan(&column_name, &ordinal_position)
		if err != nil {
			log.Error("scan information_schema row failed")
			return
		}
		if ordinal_position < 1 {
			log.Error("col ordinal out of range")
			continue
		}
		columns[column_name] = ordinal_position
	}
	_, hasCurrMailId := columns["CurrMailId"]
	if !hasCurrMailId {
		_, err = this.m_dbc.Exec("ALTER TABLE SysMailCommon ADD COLUMN CurrMailId int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN CurrMailId failed")
			return
		}
	}
	return
}
func (this *dbSysMailCommonTable) prepare_preload_select_stmt() (err error) {
	this.m_preload_select_stmt,err=this.m_dbc.StmtPrepare("SELECT CurrMailId FROM SysMailCommon WHERE Id=0")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbSysMailCommonTable) prepare_save_insert_stmt()(err error){
	this.m_save_insert_stmt,err=this.m_dbc.StmtPrepare("INSERT INTO SysMailCommon (Id,CurrMailId) VALUES (?,?)")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbSysMailCommonTable) Init() (err error) {
	err=this.check_create_table()
	if err!=nil{
		log.Error("check_create_table failed")
		return
	}
	err=this.prepare_preload_select_stmt()
	if err!=nil{
		log.Error("prepare_preload_select_stmt failed")
		return
	}
	err=this.prepare_save_insert_stmt()
	if err!=nil{
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	return
}
func (this *dbSysMailCommonTable) Preload() (err error) {
	r := this.m_dbc.StmtQueryRow(this.m_preload_select_stmt)
	var dCurrMailId int32
	err = r.Scan(&dCurrMailId)
	if err!=nil{
		if err!=sql.ErrNoRows{
			log.Error("Scan failed")
			return
		}
	}else{
		row := new_dbSysMailCommonRow(this,0)
		row.m_CurrMailId=dCurrMailId
		row.m_CurrMailId_changed=false
		row.m_valid = true
		row.m_loaded=true
		this.m_row=row
	}
	if this.m_row == nil {
		this.m_row = new_dbSysMailCommonRow(this, 0)
		this.m_row.m_new = true
		this.m_row.m_valid = true
		err = this.Save(false)
		if err != nil {
			log.Error("save failed")
			return
		}
		this.m_row.m_loaded = true
	}
	return
}
func (this *dbSysMailCommonTable) Save(quick bool) (err error) {
	if this.m_row==nil{
		return errors.New("row nil")
	}
	err, _, _ = this.m_row.Save(false)
	return err
}
func (this *dbSysMailCommonTable) GetRow( ) (row *dbSysMailCommonRow) {
	return this.m_row
}
func (this *dbSysMailRow)GetTableId( )(r int32 ){
	this.m_lock.UnSafeRLock("dbSysMailRow.GetdbSysMailTableIdColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_TableId)
}
func (this *dbSysMailRow)SetTableId(v int32){
	this.m_lock.UnSafeLock("dbSysMailRow.SetdbSysMailTableIdColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_TableId=int32(v)
	this.m_TableId_changed=true
	return
}
type dbSysMailAttachedItemsColumn struct{
	m_row *dbSysMailRow
	m_data *dbSysMailAttachedItemsData
	m_changed bool
}
func (this *dbSysMailAttachedItemsColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_data = &dbSysMailAttachedItemsData{}
		this.m_changed = false
		return nil
	}
	pb := &db.SysMailAttachedItems{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetId())
		return
	}
	this.m_data = &dbSysMailAttachedItemsData{}
	this.m_data.from_pb(pb)
	this.m_changed = false
	return
}
func (this *dbSysMailAttachedItemsColumn)save( )(data []byte,err error){
	pb:=this.m_data.to_pb()
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbSysMailAttachedItemsColumn)Get( )(v *dbSysMailAttachedItemsData ){
	this.m_row.m_lock.UnSafeRLock("dbSysMailAttachedItemsColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v=&dbSysMailAttachedItemsData{}
	this.m_data.clone_to(v)
	return
}
func (this *dbSysMailAttachedItemsColumn)Set(v dbSysMailAttachedItemsData ){
	this.m_row.m_lock.UnSafeLock("dbSysMailAttachedItemsColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=&dbSysMailAttachedItemsData{}
	v.clone_to(this.m_data)
	this.m_changed=true
	return
}
func (this *dbSysMailAttachedItemsColumn)GetItemList( )(v []int32 ){
	this.m_row.m_lock.UnSafeRLock("dbSysMailAttachedItemsColumn.GetItemList")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = make([]int32, len(this.m_data.ItemList))
	for _ii, _vv := range this.m_data.ItemList {
		v[_ii]=_vv
	}
	return
}
func (this *dbSysMailAttachedItemsColumn)SetItemList(v []int32){
	this.m_row.m_lock.UnSafeLock("dbSysMailAttachedItemsColumn.SetItemList")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.ItemList = make([]int32, len(v))
	for _ii, _vv := range v {
		this.m_data.ItemList[_ii]=_vv
	}
	this.m_changed = true
	return
}
func (this *dbSysMailRow)GetSendTime( )(r int32 ){
	this.m_lock.UnSafeRLock("dbSysMailRow.GetdbSysMailSendTimeColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_SendTime)
}
func (this *dbSysMailRow)SetSendTime(v int32){
	this.m_lock.UnSafeLock("dbSysMailRow.SetdbSysMailSendTimeColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_SendTime=int32(v)
	this.m_SendTime_changed=true
	return
}
type dbSysMailRow struct {
	m_table *dbSysMailTable
	m_lock       *RWMutex
	m_loaded  bool
	m_new     bool
	m_remove  bool
	m_touch      int32
	m_releasable bool
	m_valid   bool
	m_Id        int32
	m_TableId_changed bool
	m_TableId int32
	AttachedItems dbSysMailAttachedItemsColumn
	m_SendTime_changed bool
	m_SendTime int32
}
func new_dbSysMailRow(table *dbSysMailTable, Id int32) (r *dbSysMailRow) {
	this := &dbSysMailRow{}
	this.m_table = table
	this.m_Id = Id
	this.m_lock = NewRWMutex()
	this.m_TableId_changed=true
	this.m_SendTime_changed=true
	this.AttachedItems.m_row=this
	this.AttachedItems.m_data=&dbSysMailAttachedItemsData{}
	return this
}
func (this *dbSysMailRow) GetId() (r int32) {
	return this.m_Id
}
func (this *dbSysMailRow) save_data(release bool) (err error, released bool, state int32, update_string string, args []interface{}) {
	this.m_lock.UnSafeLock("dbSysMailRow.save_data")
	defer this.m_lock.UnSafeUnlock()
	if this.m_new {
		db_args:=new_db_args(4)
		db_args.Push(this.m_Id)
		db_args.Push(this.m_TableId)
		dAttachedItems,db_err:=this.AttachedItems.save()
		if db_err!=nil{
			log.Error("insert save AttachedItems failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dAttachedItems)
		db_args.Push(this.m_SendTime)
		args=db_args.GetArgs()
		state = 1
	} else {
		if this.m_TableId_changed||this.AttachedItems.m_changed||this.m_SendTime_changed{
			update_string = "UPDATE SysMails SET "
			db_args:=new_db_args(4)
			if this.m_TableId_changed{
				update_string+="TableId=?,"
				db_args.Push(this.m_TableId)
			}
			if this.AttachedItems.m_changed{
				update_string+="AttachedItems=?,"
				dAttachedItems,err:=this.AttachedItems.save()
				if err!=nil{
					log.Error("update save AttachedItems failed")
					return err,false,0,"",nil
				}
				db_args.Push(dAttachedItems)
			}
			if this.m_SendTime_changed{
				update_string+="SendTime=?,"
				db_args.Push(this.m_SendTime)
			}
			update_string = strings.TrimRight(update_string, ", ")
			update_string+=" WHERE Id=?"
			db_args.Push(this.m_Id)
			args=db_args.GetArgs()
			state = 2
		}
	}
	this.m_new = false
	this.m_TableId_changed = false
	this.AttachedItems.m_changed = false
	this.m_SendTime_changed = false
	if release && this.m_loaded {
		atomic.AddInt32(&this.m_table.m_gc_n, -1)
		this.m_loaded = false
		released = true
	}
	return nil,released,state,update_string,args
}
func (this *dbSysMailRow) Save(release bool) (err error, d bool, released bool) {
	err,released, state, update_string, args := this.save_data(release)
	if err != nil {
		log.Error("save data failed")
		return err, false, false
	}
	if state == 0 {
		d = false
	} else if state == 1 {
		_, err = this.m_table.m_dbc.StmtExec(this.m_table.m_save_insert_stmt, args...)
		if err != nil {
			log.Error("INSERT SysMails exec failed %v ", this.m_Id)
			return err, false, released
		}
		d = true
	} else if state == 2 {
		_, err = this.m_table.m_dbc.Exec(update_string, args...)
		if err != nil {
			log.Error("UPDATE SysMails exec failed %v", this.m_Id)
			return err, false, released
		}
		d = true
	}
	return nil, d, released
}
func (this *dbSysMailRow) Touch(releasable bool) {
	this.m_touch = int32(time.Now().Unix())
	this.m_releasable = releasable
}
type dbSysMailRowSort struct {
	rows []*dbSysMailRow
}
func (this *dbSysMailRowSort) Len() (length int) {
	return len(this.rows)
}
func (this *dbSysMailRowSort) Less(i int, j int) (less bool) {
	return this.rows[i].m_touch < this.rows[j].m_touch
}
func (this *dbSysMailRowSort) Swap(i int, j int) {
	temp := this.rows[i]
	this.rows[i] = this.rows[j]
	this.rows[j] = temp
}
type dbSysMailTable struct{
	m_dbc *DBC
	m_lock *RWMutex
	m_rows map[int32]*dbSysMailRow
	m_new_rows map[int32]*dbSysMailRow
	m_removed_rows map[int32]*dbSysMailRow
	m_gc_n int32
	m_gcing int32
	m_pool_size int32
	m_preload_select_stmt *sql.Stmt
	m_preload_max_id int32
	m_save_insert_stmt *sql.Stmt
	m_delete_stmt *sql.Stmt
	m_max_id int32
	m_max_id_changed bool
}
func new_dbSysMailTable(dbc *DBC) (this *dbSysMailTable) {
	this = &dbSysMailTable{}
	this.m_dbc = dbc
	this.m_lock = NewRWMutex()
	this.m_rows = make(map[int32]*dbSysMailRow)
	this.m_new_rows = make(map[int32]*dbSysMailRow)
	this.m_removed_rows = make(map[int32]*dbSysMailRow)
	return this
}
func (this *dbSysMailTable) check_create_table() (err error) {
	_, err = this.m_dbc.Exec("CREATE TABLE IF NOT EXISTS SysMailsMaxId(PlaceHolder int(11),MaxId int(11),PRIMARY KEY (PlaceHolder))ENGINE=InnoDB ROW_FORMAT=DYNAMIC")
	if err != nil {
		log.Error("CREATE TABLE IF NOT EXISTS SysMailsMaxId failed")
		return
	}
	r := this.m_dbc.QueryRow("SELECT Count(*) FROM SysMailsMaxId WHERE PlaceHolder=0")
	if r != nil {
		var count int32
		err = r.Scan(&count)
		if err != nil {
			log.Error("scan count failed")
			return
		}
		if count == 0 {
		_, err = this.m_dbc.Exec("INSERT INTO SysMailsMaxId (PlaceHolder,MaxId) VALUES (0,0)")
			if err != nil {
				log.Error("INSERTSysMailsMaxId failed")
				return
			}
		}
	}
	_, err = this.m_dbc.Exec("CREATE TABLE IF NOT EXISTS SysMails(Id int(11),PRIMARY KEY (Id))ENGINE=InnoDB ROW_FORMAT=DYNAMIC")
	if err != nil {
		log.Error("CREATE TABLE IF NOT EXISTS SysMails failed")
		return
	}
	rows, err := this.m_dbc.Query("SELECT COLUMN_NAME,ORDINAL_POSITION FROM information_schema.`COLUMNS` WHERE TABLE_SCHEMA=? AND TABLE_NAME='SysMails'", this.m_dbc.m_db_name)
	if err != nil {
		log.Error("SELECT information_schema failed")
		return
	}
	columns := make(map[string]int32)
	for rows.Next() {
		var column_name string
		var ordinal_position int32
		err = rows.Scan(&column_name, &ordinal_position)
		if err != nil {
			log.Error("scan information_schema row failed")
			return
		}
		if ordinal_position < 1 {
			log.Error("col ordinal out of range")
			continue
		}
		columns[column_name] = ordinal_position
	}
	_, hasTableId := columns["TableId"]
	if !hasTableId {
		_, err = this.m_dbc.Exec("ALTER TABLE SysMails ADD COLUMN TableId int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN TableId failed")
			return
		}
	}
	_, hasAttachedItems := columns["AttachedItems"]
	if !hasAttachedItems {
		_, err = this.m_dbc.Exec("ALTER TABLE SysMails ADD COLUMN AttachedItems LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN AttachedItems failed")
			return
		}
	}
	_, hasSendTime := columns["SendTime"]
	if !hasSendTime {
		_, err = this.m_dbc.Exec("ALTER TABLE SysMails ADD COLUMN SendTime int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN SendTime failed")
			return
		}
	}
	return
}
func (this *dbSysMailTable) prepare_preload_select_stmt() (err error) {
	this.m_preload_select_stmt,err=this.m_dbc.StmtPrepare("SELECT Id,TableId,AttachedItems,SendTime FROM SysMails")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbSysMailTable) prepare_save_insert_stmt()(err error){
	this.m_save_insert_stmt,err=this.m_dbc.StmtPrepare("INSERT INTO SysMails (Id,TableId,AttachedItems,SendTime) VALUES (?,?,?,?)")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbSysMailTable) prepare_delete_stmt() (err error) {
	this.m_delete_stmt,err=this.m_dbc.StmtPrepare("DELETE FROM SysMails WHERE Id=?")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbSysMailTable) Init() (err error) {
	err=this.check_create_table()
	if err!=nil{
		log.Error("check_create_table failed")
		return
	}
	err=this.prepare_preload_select_stmt()
	if err!=nil{
		log.Error("prepare_preload_select_stmt failed")
		return
	}
	err=this.prepare_save_insert_stmt()
	if err!=nil{
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	err=this.prepare_delete_stmt()
	if err!=nil{
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	return
}
func (this *dbSysMailTable) Preload() (err error) {
	r_max_id := this.m_dbc.QueryRow("SELECT MaxId FROM SysMailsMaxId WHERE PLACEHOLDER=0")
	if r_max_id != nil {
		err = r_max_id.Scan(&this.m_max_id)
		if err != nil {
			log.Error("scan max id failed")
			return
		}
	}
	r, err := this.m_dbc.StmtQuery(this.m_preload_select_stmt)
	if err != nil {
		log.Error("SELECT")
		return
	}
	var Id int32
	var dTableId int32
	var dAttachedItems []byte
	var dSendTime int32
	for r.Next() {
		err = r.Scan(&Id,&dTableId,&dAttachedItems,&dSendTime)
		if err != nil {
			log.Error("Scan err[%v]", err.Error())
			return
		}
		if Id>this.m_max_id{
			log.Error("max id ext")
			this.m_max_id = Id
			this.m_max_id_changed = true
		}
		row := new_dbSysMailRow(this,Id)
		row.m_TableId=dTableId
		err = row.AttachedItems.load(dAttachedItems)
		if err != nil {
			log.Error("AttachedItems %v", Id)
			return
		}
		row.m_SendTime=dSendTime
		row.m_TableId_changed=false
		row.m_SendTime_changed=false
		row.m_valid = true
		this.m_rows[Id]=row
	}
	return
}
func (this *dbSysMailTable) GetPreloadedMaxId() (max_id int32) {
	return this.m_preload_max_id
}
func (this *dbSysMailTable) fetch_rows(rows map[int32]*dbSysMailRow) (r map[int32]*dbSysMailRow) {
	this.m_lock.UnSafeLock("dbSysMailTable.fetch_rows")
	defer this.m_lock.UnSafeUnlock()
	r = make(map[int32]*dbSysMailRow)
	for i, v := range rows {
		r[i] = v
	}
	return r
}
func (this *dbSysMailTable) fetch_new_rows() (new_rows map[int32]*dbSysMailRow) {
	this.m_lock.UnSafeLock("dbSysMailTable.fetch_new_rows")
	defer this.m_lock.UnSafeUnlock()
	new_rows = make(map[int32]*dbSysMailRow)
	for i, v := range this.m_new_rows {
		_, has := this.m_rows[i]
		if has {
			log.Error("rows already has new rows %v", i)
			continue
		}
		this.m_rows[i] = v
		new_rows[i] = v
	}
	for i, _ := range new_rows {
		delete(this.m_new_rows, i)
	}
	return
}
func (this *dbSysMailTable) save_rows(rows map[int32]*dbSysMailRow, quick bool) {
	for _, v := range rows {
		if this.m_dbc.m_quit && !quick {
			return
		}
		err, delay, _ := v.Save(false)
		if err != nil {
			log.Error("save failed %v", err)
		}
		if this.m_dbc.m_quit && !quick {
			return
		}
		if delay&&!quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
}
func (this *dbSysMailTable) Save(quick bool) (err error){
	if this.m_max_id_changed {
		max_id := atomic.LoadInt32(&this.m_max_id)
		_, err := this.m_dbc.Exec("UPDATE SysMailsMaxId SET MaxId=?", max_id)
		if err != nil {
			log.Error("save max id failed %v", err)
		}
	}
	removed_rows := this.fetch_rows(this.m_removed_rows)
	for _, v := range removed_rows {
		_, err := this.m_dbc.StmtExec(this.m_delete_stmt, v.GetId())
		if err != nil {
			log.Error("exec delete stmt failed %v", err)
		}
		v.m_valid = false
		if !quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
	this.m_removed_rows = make(map[int32]*dbSysMailRow)
	rows := this.fetch_rows(this.m_rows)
	this.save_rows(rows, quick)
	new_rows := this.fetch_new_rows()
	this.save_rows(new_rows, quick)
	return
}
func (this *dbSysMailTable) AddRow() (row *dbSysMailRow) {
	this.m_lock.UnSafeLock("dbSysMailTable.AddRow")
	defer this.m_lock.UnSafeUnlock()
	Id := atomic.AddInt32(&this.m_max_id, 1)
	this.m_max_id_changed = true
	row = new_dbSysMailRow(this,Id)
	row.m_new = true
	row.m_loaded = true
	row.m_valid = true
	this.m_new_rows[Id] = row
	atomic.AddInt32(&this.m_gc_n,1)
	return row
}
func (this *dbSysMailTable) RemoveRow(Id int32) {
	this.m_lock.UnSafeLock("dbSysMailTable.RemoveRow")
	defer this.m_lock.UnSafeUnlock()
	row := this.m_rows[Id]
	if row != nil {
		row.m_remove = true
		delete(this.m_rows, Id)
		rm_row := this.m_removed_rows[Id]
		if rm_row != nil {
			log.Error("rows and removed rows both has %v", Id)
		}
		this.m_removed_rows[Id] = row
		_, has_new := this.m_new_rows[Id]
		if has_new {
			delete(this.m_new_rows, Id)
			log.Error("rows and new_rows both has %v", Id)
		}
	} else {
		row = this.m_removed_rows[Id]
		if row == nil {
			_, has_new := this.m_new_rows[Id]
			if has_new {
				delete(this.m_new_rows, Id)
			} else {
				log.Error("row not exist %v", Id)
			}
		} else {
			log.Error("already removed %v", Id)
			_, has_new := this.m_new_rows[Id]
			if has_new {
				delete(this.m_new_rows, Id)
				log.Error("removed rows and new_rows both has %v", Id)
			}
		}
	}
}
func (this *dbSysMailTable) GetRow(Id int32) (row *dbSysMailRow) {
	this.m_lock.UnSafeRLock("dbSysMailTable.GetRow")
	defer this.m_lock.UnSafeRUnlock()
	row = this.m_rows[Id]
	if row == nil {
		row = this.m_new_rows[Id]
	}
	return row
}
func (this *dbBanPlayerRow)GetStartTime( )(r int32 ){
	this.m_lock.UnSafeRLock("dbBanPlayerRow.GetdbBanPlayerStartTimeColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_StartTime)
}
func (this *dbBanPlayerRow)SetStartTime(v int32){
	this.m_lock.UnSafeLock("dbBanPlayerRow.SetdbBanPlayerStartTimeColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_StartTime=int32(v)
	this.m_StartTime_changed=true
	return
}
func (this *dbBanPlayerRow)GetStartTimeStr( )(r string ){
	this.m_lock.UnSafeRLock("dbBanPlayerRow.GetdbBanPlayerStartTimeStrColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_StartTimeStr)
}
func (this *dbBanPlayerRow)SetStartTimeStr(v string){
	this.m_lock.UnSafeLock("dbBanPlayerRow.SetdbBanPlayerStartTimeStrColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_StartTimeStr=string(v)
	this.m_StartTimeStr_changed=true
	return
}
func (this *dbBanPlayerRow)GetDuration( )(r int32 ){
	this.m_lock.UnSafeRLock("dbBanPlayerRow.GetdbBanPlayerDurationColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_Duration)
}
func (this *dbBanPlayerRow)SetDuration(v int32){
	this.m_lock.UnSafeLock("dbBanPlayerRow.SetdbBanPlayerDurationColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Duration=int32(v)
	this.m_Duration_changed=true
	return
}
func (this *dbBanPlayerRow)GetPlayerId( )(r int32 ){
	this.m_lock.UnSafeRLock("dbBanPlayerRow.GetdbBanPlayerPlayerIdColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_PlayerId)
}
func (this *dbBanPlayerRow)SetPlayerId(v int32){
	this.m_lock.UnSafeLock("dbBanPlayerRow.SetdbBanPlayerPlayerIdColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_PlayerId=int32(v)
	this.m_PlayerId_changed=true
	return
}
func (this *dbBanPlayerRow)GetAccount( )(r string ){
	this.m_lock.UnSafeRLock("dbBanPlayerRow.GetdbBanPlayerAccountColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_Account)
}
func (this *dbBanPlayerRow)SetAccount(v string){
	this.m_lock.UnSafeLock("dbBanPlayerRow.SetdbBanPlayerAccountColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Account=string(v)
	this.m_Account_changed=true
	return
}
type dbBanPlayerRow struct {
	m_table *dbBanPlayerTable
	m_lock       *RWMutex
	m_loaded  bool
	m_new     bool
	m_remove  bool
	m_touch      int32
	m_releasable bool
	m_valid   bool
	m_UniqueId        string
	m_StartTime_changed bool
	m_StartTime int32
	m_StartTimeStr_changed bool
	m_StartTimeStr string
	m_Duration_changed bool
	m_Duration int32
	m_PlayerId_changed bool
	m_PlayerId int32
	m_Account_changed bool
	m_Account string
}
func new_dbBanPlayerRow(table *dbBanPlayerTable, UniqueId string) (r *dbBanPlayerRow) {
	this := &dbBanPlayerRow{}
	this.m_table = table
	this.m_UniqueId = UniqueId
	this.m_lock = NewRWMutex()
	this.m_StartTime_changed=true
	this.m_StartTimeStr_changed=true
	this.m_Duration_changed=true
	this.m_PlayerId_changed=true
	this.m_Account_changed=true
	return this
}
func (this *dbBanPlayerRow) GetUniqueId() (r string) {
	return this.m_UniqueId
}
func (this *dbBanPlayerRow) save_data(release bool) (err error, released bool, state int32, update_string string, args []interface{}) {
	this.m_lock.UnSafeLock("dbBanPlayerRow.save_data")
	defer this.m_lock.UnSafeUnlock()
	if this.m_new {
		db_args:=new_db_args(6)
		db_args.Push(this.m_UniqueId)
		db_args.Push(this.m_StartTime)
		db_args.Push(this.m_StartTimeStr)
		db_args.Push(this.m_Duration)
		db_args.Push(this.m_PlayerId)
		db_args.Push(this.m_Account)
		args=db_args.GetArgs()
		state = 1
	} else {
		if this.m_StartTime_changed||this.m_StartTimeStr_changed||this.m_Duration_changed||this.m_PlayerId_changed||this.m_Account_changed{
			update_string = "UPDATE BanPlayers SET "
			db_args:=new_db_args(6)
			if this.m_StartTime_changed{
				update_string+="StartTime=?,"
				db_args.Push(this.m_StartTime)
			}
			if this.m_StartTimeStr_changed{
				update_string+="StartTimeStr=?,"
				db_args.Push(this.m_StartTimeStr)
			}
			if this.m_Duration_changed{
				update_string+="Duration=?,"
				db_args.Push(this.m_Duration)
			}
			if this.m_PlayerId_changed{
				update_string+="PlayerId=?,"
				db_args.Push(this.m_PlayerId)
			}
			if this.m_Account_changed{
				update_string+="Account=?,"
				db_args.Push(this.m_Account)
			}
			update_string = strings.TrimRight(update_string, ", ")
			update_string+=" WHERE UniqueId=?"
			db_args.Push(this.m_UniqueId)
			args=db_args.GetArgs()
			state = 2
		}
	}
	this.m_new = false
	this.m_StartTime_changed = false
	this.m_StartTimeStr_changed = false
	this.m_Duration_changed = false
	this.m_PlayerId_changed = false
	this.m_Account_changed = false
	if release && this.m_loaded {
		atomic.AddInt32(&this.m_table.m_gc_n, -1)
		this.m_loaded = false
		released = true
	}
	return nil,released,state,update_string,args
}
func (this *dbBanPlayerRow) Save(release bool) (err error, d bool, released bool) {
	err,released, state, update_string, args := this.save_data(release)
	if err != nil {
		log.Error("save data failed")
		return err, false, false
	}
	if state == 0 {
		d = false
	} else if state == 1 {
		_, err = this.m_table.m_dbc.StmtExec(this.m_table.m_save_insert_stmt, args...)
		if err != nil {
			log.Error("INSERT BanPlayers exec failed %v ", this.m_UniqueId)
			return err, false, released
		}
		d = true
	} else if state == 2 {
		_, err = this.m_table.m_dbc.Exec(update_string, args...)
		if err != nil {
			log.Error("UPDATE BanPlayers exec failed %v", this.m_UniqueId)
			return err, false, released
		}
		d = true
	}
	return nil, d, released
}
func (this *dbBanPlayerRow) Touch(releasable bool) {
	this.m_touch = int32(time.Now().Unix())
	this.m_releasable = releasable
}
type dbBanPlayerRowSort struct {
	rows []*dbBanPlayerRow
}
func (this *dbBanPlayerRowSort) Len() (length int) {
	return len(this.rows)
}
func (this *dbBanPlayerRowSort) Less(i int, j int) (less bool) {
	return this.rows[i].m_touch < this.rows[j].m_touch
}
func (this *dbBanPlayerRowSort) Swap(i int, j int) {
	temp := this.rows[i]
	this.rows[i] = this.rows[j]
	this.rows[j] = temp
}
type dbBanPlayerTable struct{
	m_dbc *DBC
	m_lock *RWMutex
	m_rows map[string]*dbBanPlayerRow
	m_new_rows map[string]*dbBanPlayerRow
	m_removed_rows map[string]*dbBanPlayerRow
	m_gc_n int32
	m_gcing int32
	m_pool_size int32
	m_preload_select_stmt *sql.Stmt
	m_preload_max_id int32
	m_save_insert_stmt *sql.Stmt
	m_delete_stmt *sql.Stmt
}
func new_dbBanPlayerTable(dbc *DBC) (this *dbBanPlayerTable) {
	this = &dbBanPlayerTable{}
	this.m_dbc = dbc
	this.m_lock = NewRWMutex()
	this.m_rows = make(map[string]*dbBanPlayerRow)
	this.m_new_rows = make(map[string]*dbBanPlayerRow)
	this.m_removed_rows = make(map[string]*dbBanPlayerRow)
	return this
}
func (this *dbBanPlayerTable) check_create_table() (err error) {
	_, err = this.m_dbc.Exec("CREATE TABLE IF NOT EXISTS BanPlayers(UniqueId varchar(64),PRIMARY KEY (UniqueId))ENGINE=InnoDB ROW_FORMAT=DYNAMIC")
	if err != nil {
		log.Error("CREATE TABLE IF NOT EXISTS BanPlayers failed")
		return
	}
	rows, err := this.m_dbc.Query("SELECT COLUMN_NAME,ORDINAL_POSITION FROM information_schema.`COLUMNS` WHERE TABLE_SCHEMA=? AND TABLE_NAME='BanPlayers'", this.m_dbc.m_db_name)
	if err != nil {
		log.Error("SELECT information_schema failed")
		return
	}
	columns := make(map[string]int32)
	for rows.Next() {
		var column_name string
		var ordinal_position int32
		err = rows.Scan(&column_name, &ordinal_position)
		if err != nil {
			log.Error("scan information_schema row failed")
			return
		}
		if ordinal_position < 1 {
			log.Error("col ordinal out of range")
			continue
		}
		columns[column_name] = ordinal_position
	}
	_, hasStartTime := columns["StartTime"]
	if !hasStartTime {
		_, err = this.m_dbc.Exec("ALTER TABLE BanPlayers ADD COLUMN StartTime int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN StartTime failed")
			return
		}
	}
	_, hasStartTimeStr := columns["StartTimeStr"]
	if !hasStartTimeStr {
		_, err = this.m_dbc.Exec("ALTER TABLE BanPlayers ADD COLUMN StartTimeStr varchar(256) DEFAULT ''")
		if err != nil {
			log.Error("ADD COLUMN StartTimeStr failed")
			return
		}
	}
	_, hasDuration := columns["Duration"]
	if !hasDuration {
		_, err = this.m_dbc.Exec("ALTER TABLE BanPlayers ADD COLUMN Duration int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN Duration failed")
			return
		}
	}
	_, hasPlayerId := columns["PlayerId"]
	if !hasPlayerId {
		_, err = this.m_dbc.Exec("ALTER TABLE BanPlayers ADD COLUMN PlayerId int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN PlayerId failed")
			return
		}
	}
	_, hasAccount := columns["Account"]
	if !hasAccount {
		_, err = this.m_dbc.Exec("ALTER TABLE BanPlayers ADD COLUMN Account varchar(256) DEFAULT ''")
		if err != nil {
			log.Error("ADD COLUMN Account failed")
			return
		}
	}
	return
}
func (this *dbBanPlayerTable) prepare_preload_select_stmt() (err error) {
	this.m_preload_select_stmt,err=this.m_dbc.StmtPrepare("SELECT UniqueId,StartTime,StartTimeStr,Duration,PlayerId,Account FROM BanPlayers")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbBanPlayerTable) prepare_save_insert_stmt()(err error){
	this.m_save_insert_stmt,err=this.m_dbc.StmtPrepare("INSERT INTO BanPlayers (UniqueId,StartTime,StartTimeStr,Duration,PlayerId,Account) VALUES (?,?,?,?,?,?)")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbBanPlayerTable) prepare_delete_stmt() (err error) {
	this.m_delete_stmt,err=this.m_dbc.StmtPrepare("DELETE FROM BanPlayers WHERE UniqueId=?")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbBanPlayerTable) Init() (err error) {
	err=this.check_create_table()
	if err!=nil{
		log.Error("check_create_table failed")
		return
	}
	err=this.prepare_preload_select_stmt()
	if err!=nil{
		log.Error("prepare_preload_select_stmt failed")
		return
	}
	err=this.prepare_save_insert_stmt()
	if err!=nil{
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	err=this.prepare_delete_stmt()
	if err!=nil{
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	return
}
func (this *dbBanPlayerTable) Preload() (err error) {
	r, err := this.m_dbc.StmtQuery(this.m_preload_select_stmt)
	if err != nil {
		log.Error("SELECT")
		return
	}
	var UniqueId string
	var dStartTime int32
	var dStartTimeStr string
	var dDuration int32
	var dPlayerId int32
	var dAccount string
	for r.Next() {
		err = r.Scan(&UniqueId,&dStartTime,&dStartTimeStr,&dDuration,&dPlayerId,&dAccount)
		if err != nil {
			log.Error("Scan err[%v]", err.Error())
			return
		}
		row := new_dbBanPlayerRow(this,UniqueId)
		row.m_StartTime=dStartTime
		row.m_StartTimeStr=dStartTimeStr
		row.m_Duration=dDuration
		row.m_PlayerId=dPlayerId
		row.m_Account=dAccount
		row.m_StartTime_changed=false
		row.m_StartTimeStr_changed=false
		row.m_Duration_changed=false
		row.m_PlayerId_changed=false
		row.m_Account_changed=false
		row.m_valid = true
		this.m_rows[UniqueId]=row
	}
	return
}
func (this *dbBanPlayerTable) GetPreloadedMaxId() (max_id int32) {
	return this.m_preload_max_id
}
func (this *dbBanPlayerTable) fetch_rows(rows map[string]*dbBanPlayerRow) (r map[string]*dbBanPlayerRow) {
	this.m_lock.UnSafeLock("dbBanPlayerTable.fetch_rows")
	defer this.m_lock.UnSafeUnlock()
	r = make(map[string]*dbBanPlayerRow)
	for i, v := range rows {
		r[i] = v
	}
	return r
}
func (this *dbBanPlayerTable) fetch_new_rows() (new_rows map[string]*dbBanPlayerRow) {
	this.m_lock.UnSafeLock("dbBanPlayerTable.fetch_new_rows")
	defer this.m_lock.UnSafeUnlock()
	new_rows = make(map[string]*dbBanPlayerRow)
	for i, v := range this.m_new_rows {
		_, has := this.m_rows[i]
		if has {
			log.Error("rows already has new rows %v", i)
			continue
		}
		this.m_rows[i] = v
		new_rows[i] = v
	}
	for i, _ := range new_rows {
		delete(this.m_new_rows, i)
	}
	return
}
func (this *dbBanPlayerTable) save_rows(rows map[string]*dbBanPlayerRow, quick bool) {
	for _, v := range rows {
		if this.m_dbc.m_quit && !quick {
			return
		}
		err, delay, _ := v.Save(false)
		if err != nil {
			log.Error("save failed %v", err)
		}
		if this.m_dbc.m_quit && !quick {
			return
		}
		if delay&&!quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
}
func (this *dbBanPlayerTable) Save(quick bool) (err error){
	removed_rows := this.fetch_rows(this.m_removed_rows)
	for _, v := range removed_rows {
		_, err := this.m_dbc.StmtExec(this.m_delete_stmt, v.GetUniqueId())
		if err != nil {
			log.Error("exec delete stmt failed %v", err)
		}
		v.m_valid = false
		if !quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
	this.m_removed_rows = make(map[string]*dbBanPlayerRow)
	rows := this.fetch_rows(this.m_rows)
	this.save_rows(rows, quick)
	new_rows := this.fetch_new_rows()
	this.save_rows(new_rows, quick)
	return
}
func (this *dbBanPlayerTable) AddRow(UniqueId string) (row *dbBanPlayerRow) {
	this.m_lock.UnSafeLock("dbBanPlayerTable.AddRow")
	defer this.m_lock.UnSafeUnlock()
	row = new_dbBanPlayerRow(this,UniqueId)
	row.m_new = true
	row.m_loaded = true
	row.m_valid = true
	_, has := this.m_new_rows[UniqueId]
	if has{
		log.Error("已经存在 %v", UniqueId)
		return nil
	}
	this.m_new_rows[UniqueId] = row
	atomic.AddInt32(&this.m_gc_n,1)
	return row
}
func (this *dbBanPlayerTable) RemoveRow(UniqueId string) {
	this.m_lock.UnSafeLock("dbBanPlayerTable.RemoveRow")
	defer this.m_lock.UnSafeUnlock()
	row := this.m_rows[UniqueId]
	if row != nil {
		row.m_remove = true
		delete(this.m_rows, UniqueId)
		rm_row := this.m_removed_rows[UniqueId]
		if rm_row != nil {
			log.Error("rows and removed rows both has %v", UniqueId)
		}
		this.m_removed_rows[UniqueId] = row
		_, has_new := this.m_new_rows[UniqueId]
		if has_new {
			delete(this.m_new_rows, UniqueId)
			log.Error("rows and new_rows both has %v", UniqueId)
		}
	} else {
		row = this.m_removed_rows[UniqueId]
		if row == nil {
			_, has_new := this.m_new_rows[UniqueId]
			if has_new {
				delete(this.m_new_rows, UniqueId)
			} else {
				log.Error("row not exist %v", UniqueId)
			}
		} else {
			log.Error("already removed %v", UniqueId)
			_, has_new := this.m_new_rows[UniqueId]
			if has_new {
				delete(this.m_new_rows, UniqueId)
				log.Error("removed rows and new_rows both has %v", UniqueId)
			}
		}
	}
}
func (this *dbBanPlayerTable) GetRow(UniqueId string) (row *dbBanPlayerRow) {
	this.m_lock.UnSafeRLock("dbBanPlayerTable.GetRow")
	defer this.m_lock.UnSafeRUnlock()
	row = this.m_rows[UniqueId]
	if row == nil {
		row = this.m_new_rows[UniqueId]
	}
	return row
}
func (this *dbCarnivalRow)GetRound( )(r int32 ){
	this.m_lock.UnSafeRLock("dbCarnivalRow.GetdbCarnivalRoundColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_Round)
}
func (this *dbCarnivalRow)SetRound(v int32){
	this.m_lock.UnSafeLock("dbCarnivalRow.SetdbCarnivalRoundColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Round=int32(v)
	this.m_Round_changed=true
	return
}
type dbCarnivalRow struct {
	m_table *dbCarnivalTable
	m_lock       *RWMutex
	m_loaded  bool
	m_new     bool
	m_remove  bool
	m_touch      int32
	m_releasable bool
	m_valid   bool
	m_Id        int32
	m_Round_changed bool
	m_Round int32
}
func new_dbCarnivalRow(table *dbCarnivalTable, Id int32) (r *dbCarnivalRow) {
	this := &dbCarnivalRow{}
	this.m_table = table
	this.m_Id = Id
	this.m_lock = NewRWMutex()
	this.m_Round_changed=true
	return this
}
func (this *dbCarnivalRow) save_data(release bool) (err error, released bool, state int32, update_string string, args []interface{}) {
	this.m_lock.UnSafeLock("dbCarnivalRow.save_data")
	defer this.m_lock.UnSafeUnlock()
	if this.m_new {
		db_args:=new_db_args(2)
		db_args.Push(this.m_Id)
		db_args.Push(this.m_Round)
		args=db_args.GetArgs()
		state = 1
	} else {
		if this.m_Round_changed{
			update_string = "UPDATE Carnival SET "
			db_args:=new_db_args(2)
			if this.m_Round_changed{
				update_string+="Round=?,"
				db_args.Push(this.m_Round)
			}
			update_string = strings.TrimRight(update_string, ", ")
			update_string+=" WHERE Id=?"
			db_args.Push(this.m_Id)
			args=db_args.GetArgs()
			state = 2
		}
	}
	this.m_new = false
	this.m_Round_changed = false
	if release && this.m_loaded {
		this.m_loaded = false
		released = true
	}
	return nil,released,state,update_string,args
}
func (this *dbCarnivalRow) Save(release bool) (err error, d bool, released bool) {
	err,released, state, update_string, args := this.save_data(release)
	if err != nil {
		log.Error("save data failed")
		return err, false, false
	}
	if state == 0 {
		d = false
	} else if state == 1 {
		_, err = this.m_table.m_dbc.StmtExec(this.m_table.m_save_insert_stmt, args...)
		if err != nil {
			log.Error("INSERT Carnival exec failed %v ", this.m_Id)
			return err, false, released
		}
		d = true
	} else if state == 2 {
		_, err = this.m_table.m_dbc.Exec(update_string, args...)
		if err != nil {
			log.Error("UPDATE Carnival exec failed %v", this.m_Id)
			return err, false, released
		}
		d = true
	}
	return nil, d, released
}
type dbCarnivalTable struct{
	m_dbc *DBC
	m_lock *RWMutex
	m_row *dbCarnivalRow
	m_preload_select_stmt *sql.Stmt
	m_save_insert_stmt *sql.Stmt
}
func new_dbCarnivalTable(dbc *DBC) (this *dbCarnivalTable) {
	this = &dbCarnivalTable{}
	this.m_dbc = dbc
	this.m_lock = NewRWMutex()
	return this
}
func (this *dbCarnivalTable) check_create_table() (err error) {
	_, err = this.m_dbc.Exec("CREATE TABLE IF NOT EXISTS Carnival(Id int(11),PRIMARY KEY (Id))ENGINE=InnoDB ROW_FORMAT=DYNAMIC")
	if err != nil {
		log.Error("CREATE TABLE IF NOT EXISTS Carnival failed")
		return
	}
	rows, err := this.m_dbc.Query("SELECT COLUMN_NAME,ORDINAL_POSITION FROM information_schema.`COLUMNS` WHERE TABLE_SCHEMA=? AND TABLE_NAME='Carnival'", this.m_dbc.m_db_name)
	if err != nil {
		log.Error("SELECT information_schema failed")
		return
	}
	columns := make(map[string]int32)
	for rows.Next() {
		var column_name string
		var ordinal_position int32
		err = rows.Scan(&column_name, &ordinal_position)
		if err != nil {
			log.Error("scan information_schema row failed")
			return
		}
		if ordinal_position < 1 {
			log.Error("col ordinal out of range")
			continue
		}
		columns[column_name] = ordinal_position
	}
	_, hasRound := columns["Round"]
	if !hasRound {
		_, err = this.m_dbc.Exec("ALTER TABLE Carnival ADD COLUMN Round int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN Round failed")
			return
		}
	}
	return
}
func (this *dbCarnivalTable) prepare_preload_select_stmt() (err error) {
	this.m_preload_select_stmt,err=this.m_dbc.StmtPrepare("SELECT Round FROM Carnival WHERE Id=0")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbCarnivalTable) prepare_save_insert_stmt()(err error){
	this.m_save_insert_stmt,err=this.m_dbc.StmtPrepare("INSERT INTO Carnival (Id,Round) VALUES (?,?)")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbCarnivalTable) Init() (err error) {
	err=this.check_create_table()
	if err!=nil{
		log.Error("check_create_table failed")
		return
	}
	err=this.prepare_preload_select_stmt()
	if err!=nil{
		log.Error("prepare_preload_select_stmt failed")
		return
	}
	err=this.prepare_save_insert_stmt()
	if err!=nil{
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	return
}
func (this *dbCarnivalTable) Preload() (err error) {
	r := this.m_dbc.StmtQueryRow(this.m_preload_select_stmt)
	var dRound int32
	err = r.Scan(&dRound)
	if err!=nil{
		if err!=sql.ErrNoRows{
			log.Error("Scan failed")
			return
		}
	}else{
		row := new_dbCarnivalRow(this,0)
		row.m_Round=dRound
		row.m_Round_changed=false
		row.m_valid = true
		row.m_loaded=true
		this.m_row=row
	}
	if this.m_row == nil {
		this.m_row = new_dbCarnivalRow(this, 0)
		this.m_row.m_new = true
		this.m_row.m_valid = true
		err = this.Save(false)
		if err != nil {
			log.Error("save failed")
			return
		}
		this.m_row.m_loaded = true
	}
	return
}
func (this *dbCarnivalTable) Save(quick bool) (err error) {
	if this.m_row==nil{
		return errors.New("row nil")
	}
	err, _, _ = this.m_row.Save(false)
	return err
}
func (this *dbCarnivalTable) GetRow( ) (row *dbCarnivalRow) {
	return this.m_row
}
func (this *dbOtherServerPlayerRow)GetAccount( )(r string ){
	this.m_lock.UnSafeRLock("dbOtherServerPlayerRow.GetdbOtherServerPlayerAccountColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_Account)
}
func (this *dbOtherServerPlayerRow)SetAccount(v string){
	this.m_lock.UnSafeLock("dbOtherServerPlayerRow.SetdbOtherServerPlayerAccountColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Account=string(v)
	this.m_Account_changed=true
	return
}
func (this *dbOtherServerPlayerRow)GetName( )(r string ){
	this.m_lock.UnSafeRLock("dbOtherServerPlayerRow.GetdbOtherServerPlayerNameColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_Name)
}
func (this *dbOtherServerPlayerRow)SetName(v string){
	this.m_lock.UnSafeLock("dbOtherServerPlayerRow.SetdbOtherServerPlayerNameColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Name=string(v)
	this.m_Name_changed=true
	return
}
func (this *dbOtherServerPlayerRow)GetLevel( )(r int32 ){
	this.m_lock.UnSafeRLock("dbOtherServerPlayerRow.GetdbOtherServerPlayerLevelColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_Level)
}
func (this *dbOtherServerPlayerRow)SetLevel(v int32){
	this.m_lock.UnSafeLock("dbOtherServerPlayerRow.SetdbOtherServerPlayerLevelColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Level=int32(v)
	this.m_Level_changed=true
	return
}
func (this *dbOtherServerPlayerRow)GetHead( )(r string ){
	this.m_lock.UnSafeRLock("dbOtherServerPlayerRow.GetdbOtherServerPlayerHeadColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_Head)
}
func (this *dbOtherServerPlayerRow)SetHead(v string){
	this.m_lock.UnSafeLock("dbOtherServerPlayerRow.SetdbOtherServerPlayerHeadColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Head=string(v)
	this.m_Head_changed=true
	return
}
type dbOtherServerPlayerRow struct {
	m_table *dbOtherServerPlayerTable
	m_lock       *RWMutex
	m_loaded  bool
	m_new     bool
	m_remove  bool
	m_touch      int32
	m_releasable bool
	m_valid   bool
	m_PlayerId        int32
	m_Account_changed bool
	m_Account string
	m_Name_changed bool
	m_Name string
	m_Level_changed bool
	m_Level int32
	m_Head_changed bool
	m_Head string
}
func new_dbOtherServerPlayerRow(table *dbOtherServerPlayerTable, PlayerId int32) (r *dbOtherServerPlayerRow) {
	this := &dbOtherServerPlayerRow{}
	this.m_table = table
	this.m_PlayerId = PlayerId
	this.m_lock = NewRWMutex()
	this.m_Account_changed=true
	this.m_Name_changed=true
	this.m_Level_changed=true
	this.m_Head_changed=true
	return this
}
func (this *dbOtherServerPlayerRow) GetPlayerId() (r int32) {
	return this.m_PlayerId
}
func (this *dbOtherServerPlayerRow) save_data(release bool) (err error, released bool, state int32, update_string string, args []interface{}) {
	this.m_lock.UnSafeLock("dbOtherServerPlayerRow.save_data")
	defer this.m_lock.UnSafeUnlock()
	if this.m_new {
		db_args:=new_db_args(5)
		db_args.Push(this.m_PlayerId)
		db_args.Push(this.m_Account)
		db_args.Push(this.m_Name)
		db_args.Push(this.m_Level)
		db_args.Push(this.m_Head)
		args=db_args.GetArgs()
		state = 1
	} else {
		if this.m_Account_changed||this.m_Name_changed||this.m_Level_changed||this.m_Head_changed{
			update_string = "UPDATE OtherServerPlayers SET "
			db_args:=new_db_args(5)
			if this.m_Account_changed{
				update_string+="Account=?,"
				db_args.Push(this.m_Account)
			}
			if this.m_Name_changed{
				update_string+="Name=?,"
				db_args.Push(this.m_Name)
			}
			if this.m_Level_changed{
				update_string+="Level=?,"
				db_args.Push(this.m_Level)
			}
			if this.m_Head_changed{
				update_string+="Head=?,"
				db_args.Push(this.m_Head)
			}
			update_string = strings.TrimRight(update_string, ", ")
			update_string+=" WHERE PlayerId=?"
			db_args.Push(this.m_PlayerId)
			args=db_args.GetArgs()
			state = 2
		}
	}
	this.m_new = false
	this.m_Account_changed = false
	this.m_Name_changed = false
	this.m_Level_changed = false
	this.m_Head_changed = false
	if release && this.m_loaded {
		atomic.AddInt32(&this.m_table.m_gc_n, -1)
		this.m_loaded = false
		released = true
	}
	return nil,released,state,update_string,args
}
func (this *dbOtherServerPlayerRow) Save(release bool) (err error, d bool, released bool) {
	err,released, state, update_string, args := this.save_data(release)
	if err != nil {
		log.Error("save data failed")
		return err, false, false
	}
	if state == 0 {
		d = false
	} else if state == 1 {
		_, err = this.m_table.m_dbc.StmtExec(this.m_table.m_save_insert_stmt, args...)
		if err != nil {
			log.Error("INSERT OtherServerPlayers exec failed %v ", this.m_PlayerId)
			return err, false, released
		}
		d = true
	} else if state == 2 {
		_, err = this.m_table.m_dbc.Exec(update_string, args...)
		if err != nil {
			log.Error("UPDATE OtherServerPlayers exec failed %v", this.m_PlayerId)
			return err, false, released
		}
		d = true
	}
	return nil, d, released
}
func (this *dbOtherServerPlayerRow) Touch(releasable bool) {
	this.m_touch = int32(time.Now().Unix())
	this.m_releasable = releasable
}
type dbOtherServerPlayerRowSort struct {
	rows []*dbOtherServerPlayerRow
}
func (this *dbOtherServerPlayerRowSort) Len() (length int) {
	return len(this.rows)
}
func (this *dbOtherServerPlayerRowSort) Less(i int, j int) (less bool) {
	return this.rows[i].m_touch < this.rows[j].m_touch
}
func (this *dbOtherServerPlayerRowSort) Swap(i int, j int) {
	temp := this.rows[i]
	this.rows[i] = this.rows[j]
	this.rows[j] = temp
}
type dbOtherServerPlayerTable struct{
	m_dbc *DBC
	m_lock *RWMutex
	m_rows map[int32]*dbOtherServerPlayerRow
	m_new_rows map[int32]*dbOtherServerPlayerRow
	m_removed_rows map[int32]*dbOtherServerPlayerRow
	m_gc_n int32
	m_gcing int32
	m_pool_size int32
	m_preload_select_stmt *sql.Stmt
	m_preload_max_id int32
	m_save_insert_stmt *sql.Stmt
	m_delete_stmt *sql.Stmt
}
func new_dbOtherServerPlayerTable(dbc *DBC) (this *dbOtherServerPlayerTable) {
	this = &dbOtherServerPlayerTable{}
	this.m_dbc = dbc
	this.m_lock = NewRWMutex()
	this.m_rows = make(map[int32]*dbOtherServerPlayerRow)
	this.m_new_rows = make(map[int32]*dbOtherServerPlayerRow)
	this.m_removed_rows = make(map[int32]*dbOtherServerPlayerRow)
	return this
}
func (this *dbOtherServerPlayerTable) check_create_table() (err error) {
	_, err = this.m_dbc.Exec("CREATE TABLE IF NOT EXISTS OtherServerPlayers(PlayerId int(11),PRIMARY KEY (PlayerId))ENGINE=InnoDB ROW_FORMAT=DYNAMIC")
	if err != nil {
		log.Error("CREATE TABLE IF NOT EXISTS OtherServerPlayers failed")
		return
	}
	rows, err := this.m_dbc.Query("SELECT COLUMN_NAME,ORDINAL_POSITION FROM information_schema.`COLUMNS` WHERE TABLE_SCHEMA=? AND TABLE_NAME='OtherServerPlayers'", this.m_dbc.m_db_name)
	if err != nil {
		log.Error("SELECT information_schema failed")
		return
	}
	columns := make(map[string]int32)
	for rows.Next() {
		var column_name string
		var ordinal_position int32
		err = rows.Scan(&column_name, &ordinal_position)
		if err != nil {
			log.Error("scan information_schema row failed")
			return
		}
		if ordinal_position < 1 {
			log.Error("col ordinal out of range")
			continue
		}
		columns[column_name] = ordinal_position
	}
	_, hasAccount := columns["Account"]
	if !hasAccount {
		_, err = this.m_dbc.Exec("ALTER TABLE OtherServerPlayers ADD COLUMN Account varchar(256)")
		if err != nil {
			log.Error("ADD COLUMN Account failed")
			return
		}
	}
	_, hasName := columns["Name"]
	if !hasName {
		_, err = this.m_dbc.Exec("ALTER TABLE OtherServerPlayers ADD COLUMN Name varchar(256)")
		if err != nil {
			log.Error("ADD COLUMN Name failed")
			return
		}
	}
	_, hasLevel := columns["Level"]
	if !hasLevel {
		_, err = this.m_dbc.Exec("ALTER TABLE OtherServerPlayers ADD COLUMN Level int(11)")
		if err != nil {
			log.Error("ADD COLUMN Level failed")
			return
		}
	}
	_, hasHead := columns["Head"]
	if !hasHead {
		_, err = this.m_dbc.Exec("ALTER TABLE OtherServerPlayers ADD COLUMN Head varchar(256)")
		if err != nil {
			log.Error("ADD COLUMN Head failed")
			return
		}
	}
	return
}
func (this *dbOtherServerPlayerTable) prepare_preload_select_stmt() (err error) {
	this.m_preload_select_stmt,err=this.m_dbc.StmtPrepare("SELECT PlayerId,Account,Name,Level,Head FROM OtherServerPlayers")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbOtherServerPlayerTable) prepare_save_insert_stmt()(err error){
	this.m_save_insert_stmt,err=this.m_dbc.StmtPrepare("INSERT INTO OtherServerPlayers (PlayerId,Account,Name,Level,Head) VALUES (?,?,?,?,?)")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbOtherServerPlayerTable) prepare_delete_stmt() (err error) {
	this.m_delete_stmt,err=this.m_dbc.StmtPrepare("DELETE FROM OtherServerPlayers WHERE PlayerId=?")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbOtherServerPlayerTable) Init() (err error) {
	err=this.check_create_table()
	if err!=nil{
		log.Error("check_create_table failed")
		return
	}
	err=this.prepare_preload_select_stmt()
	if err!=nil{
		log.Error("prepare_preload_select_stmt failed")
		return
	}
	err=this.prepare_save_insert_stmt()
	if err!=nil{
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	err=this.prepare_delete_stmt()
	if err!=nil{
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	return
}
func (this *dbOtherServerPlayerTable) Preload() (err error) {
	r, err := this.m_dbc.StmtQuery(this.m_preload_select_stmt)
	if err != nil {
		log.Error("SELECT")
		return
	}
	var PlayerId int32
	var dAccount string
	var dName string
	var dLevel int32
	var dHead string
		this.m_preload_max_id = 0
	for r.Next() {
		err = r.Scan(&PlayerId,&dAccount,&dName,&dLevel,&dHead)
		if err != nil {
			log.Error("Scan err[%v]", err.Error())
			return
		}
		if PlayerId>this.m_preload_max_id{
			this.m_preload_max_id =PlayerId
		}
		row := new_dbOtherServerPlayerRow(this,PlayerId)
		row.m_Account=dAccount
		row.m_Name=dName
		row.m_Level=dLevel
		row.m_Head=dHead
		row.m_Account_changed=false
		row.m_Name_changed=false
		row.m_Level_changed=false
		row.m_Head_changed=false
		row.m_valid = true
		this.m_rows[PlayerId]=row
	}
	return
}
func (this *dbOtherServerPlayerTable) GetPreloadedMaxId() (max_id int32) {
	return this.m_preload_max_id
}
func (this *dbOtherServerPlayerTable) fetch_rows(rows map[int32]*dbOtherServerPlayerRow) (r map[int32]*dbOtherServerPlayerRow) {
	this.m_lock.UnSafeLock("dbOtherServerPlayerTable.fetch_rows")
	defer this.m_lock.UnSafeUnlock()
	r = make(map[int32]*dbOtherServerPlayerRow)
	for i, v := range rows {
		r[i] = v
	}
	return r
}
func (this *dbOtherServerPlayerTable) fetch_new_rows() (new_rows map[int32]*dbOtherServerPlayerRow) {
	this.m_lock.UnSafeLock("dbOtherServerPlayerTable.fetch_new_rows")
	defer this.m_lock.UnSafeUnlock()
	new_rows = make(map[int32]*dbOtherServerPlayerRow)
	for i, v := range this.m_new_rows {
		_, has := this.m_rows[i]
		if has {
			log.Error("rows already has new rows %v", i)
			continue
		}
		this.m_rows[i] = v
		new_rows[i] = v
	}
	for i, _ := range new_rows {
		delete(this.m_new_rows, i)
	}
	return
}
func (this *dbOtherServerPlayerTable) save_rows(rows map[int32]*dbOtherServerPlayerRow, quick bool) {
	for _, v := range rows {
		if this.m_dbc.m_quit && !quick {
			return
		}
		err, delay, _ := v.Save(false)
		if err != nil {
			log.Error("save failed %v", err)
		}
		if this.m_dbc.m_quit && !quick {
			return
		}
		if delay&&!quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
}
func (this *dbOtherServerPlayerTable) Save(quick bool) (err error){
	removed_rows := this.fetch_rows(this.m_removed_rows)
	for _, v := range removed_rows {
		_, err := this.m_dbc.StmtExec(this.m_delete_stmt, v.GetPlayerId())
		if err != nil {
			log.Error("exec delete stmt failed %v", err)
		}
		v.m_valid = false
		if !quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
	this.m_removed_rows = make(map[int32]*dbOtherServerPlayerRow)
	rows := this.fetch_rows(this.m_rows)
	this.save_rows(rows, quick)
	new_rows := this.fetch_new_rows()
	this.save_rows(new_rows, quick)
	return
}
func (this *dbOtherServerPlayerTable) AddRow(PlayerId int32) (row *dbOtherServerPlayerRow) {
	this.m_lock.UnSafeLock("dbOtherServerPlayerTable.AddRow")
	defer this.m_lock.UnSafeUnlock()
	row = new_dbOtherServerPlayerRow(this,PlayerId)
	row.m_new = true
	row.m_loaded = true
	row.m_valid = true
	_, has := this.m_new_rows[PlayerId]
	if has{
		log.Error("已经存在 %v", PlayerId)
		return nil
	}
	this.m_new_rows[PlayerId] = row
	atomic.AddInt32(&this.m_gc_n,1)
	return row
}
func (this *dbOtherServerPlayerTable) RemoveRow(PlayerId int32) {
	this.m_lock.UnSafeLock("dbOtherServerPlayerTable.RemoveRow")
	defer this.m_lock.UnSafeUnlock()
	row := this.m_rows[PlayerId]
	if row != nil {
		row.m_remove = true
		delete(this.m_rows, PlayerId)
		rm_row := this.m_removed_rows[PlayerId]
		if rm_row != nil {
			log.Error("rows and removed rows both has %v", PlayerId)
		}
		this.m_removed_rows[PlayerId] = row
		_, has_new := this.m_new_rows[PlayerId]
		if has_new {
			delete(this.m_new_rows, PlayerId)
			log.Error("rows and new_rows both has %v", PlayerId)
		}
	} else {
		row = this.m_removed_rows[PlayerId]
		if row == nil {
			_, has_new := this.m_new_rows[PlayerId]
			if has_new {
				delete(this.m_new_rows, PlayerId)
			} else {
				log.Error("row not exist %v", PlayerId)
			}
		} else {
			log.Error("already removed %v", PlayerId)
			_, has_new := this.m_new_rows[PlayerId]
			if has_new {
				delete(this.m_new_rows, PlayerId)
				log.Error("removed rows and new_rows both has %v", PlayerId)
			}
		}
	}
}
func (this *dbOtherServerPlayerTable) GetRow(PlayerId int32) (row *dbOtherServerPlayerRow) {
	this.m_lock.UnSafeRLock("dbOtherServerPlayerTable.GetRow")
	defer this.m_lock.UnSafeRUnlock()
	row = this.m_rows[PlayerId]
	if row == nil {
		row = this.m_new_rows[PlayerId]
	}
	return row
}

type DBC struct {
	m_db_name            string
	m_db                 *sql.DB
	m_db_lock            *Mutex
	m_initialized        bool
	m_quit               bool
	m_shutdown_completed bool
	m_shutdown_lock      *Mutex
	m_db_last_copy_time	int32
	m_db_copy_path		string
	m_db_addr			string
	m_db_account			string
	m_db_password		string
	Global *dbGlobalTable
	Players *dbPlayerTable
	BattleSaves *dbBattleSaveTable
	TowerFightSaves *dbTowerFightSaveTable
	ArenaSeason *dbArenaSeasonTable
	Guilds *dbGuildTable
	GuildStages *dbGuildStageTable
	ActivitysToDeletes *dbActivitysToDeleteTable
	SysMailCommon *dbSysMailCommonTable
	SysMails *dbSysMailTable
	BanPlayers *dbBanPlayerTable
	Carnival *dbCarnivalTable
	OtherServerPlayers *dbOtherServerPlayerTable
}
func (this *DBC)init_tables()(err error){
	this.Global = new_dbGlobalTable(this)
	err = this.Global.Init()
	if err != nil {
		log.Error("init Global table failed")
		return
	}
	this.Players = new_dbPlayerTable(this)
	err = this.Players.Init()
	if err != nil {
		log.Error("init Players table failed")
		return
	}
	this.BattleSaves = new_dbBattleSaveTable(this)
	err = this.BattleSaves.Init()
	if err != nil {
		log.Error("init BattleSaves table failed")
		return
	}
	this.TowerFightSaves = new_dbTowerFightSaveTable(this)
	err = this.TowerFightSaves.Init()
	if err != nil {
		log.Error("init TowerFightSaves table failed")
		return
	}
	this.ArenaSeason = new_dbArenaSeasonTable(this)
	err = this.ArenaSeason.Init()
	if err != nil {
		log.Error("init ArenaSeason table failed")
		return
	}
	this.Guilds = new_dbGuildTable(this)
	err = this.Guilds.Init()
	if err != nil {
		log.Error("init Guilds table failed")
		return
	}
	this.GuildStages = new_dbGuildStageTable(this)
	err = this.GuildStages.Init()
	if err != nil {
		log.Error("init GuildStages table failed")
		return
	}
	this.ActivitysToDeletes = new_dbActivitysToDeleteTable(this)
	err = this.ActivitysToDeletes.Init()
	if err != nil {
		log.Error("init ActivitysToDeletes table failed")
		return
	}
	this.SysMailCommon = new_dbSysMailCommonTable(this)
	err = this.SysMailCommon.Init()
	if err != nil {
		log.Error("init SysMailCommon table failed")
		return
	}
	this.SysMails = new_dbSysMailTable(this)
	err = this.SysMails.Init()
	if err != nil {
		log.Error("init SysMails table failed")
		return
	}
	this.BanPlayers = new_dbBanPlayerTable(this)
	err = this.BanPlayers.Init()
	if err != nil {
		log.Error("init BanPlayers table failed")
		return
	}
	this.Carnival = new_dbCarnivalTable(this)
	err = this.Carnival.Init()
	if err != nil {
		log.Error("init Carnival table failed")
		return
	}
	this.OtherServerPlayers = new_dbOtherServerPlayerTable(this)
	err = this.OtherServerPlayers.Init()
	if err != nil {
		log.Error("init OtherServerPlayers table failed")
		return
	}
	return
}
func (this *DBC)Preload()(err error){
	err = this.Global.Preload()
	if err != nil {
		log.Error("preload Global table failed")
		return
	}else{
		log.Info("preload Global table succeed !")
	}
	err = this.Players.Preload()
	if err != nil {
		log.Error("preload Players table failed")
		return
	}else{
		log.Info("preload Players table succeed !")
	}
	err = this.BattleSaves.Preload()
	if err != nil {
		log.Error("preload BattleSaves table failed")
		return
	}else{
		log.Info("preload BattleSaves table succeed !")
	}
	err = this.TowerFightSaves.Preload()
	if err != nil {
		log.Error("preload TowerFightSaves table failed")
		return
	}else{
		log.Info("preload TowerFightSaves table succeed !")
	}
	err = this.ArenaSeason.Preload()
	if err != nil {
		log.Error("preload ArenaSeason table failed")
		return
	}else{
		log.Info("preload ArenaSeason table succeed !")
	}
	err = this.Guilds.Preload()
	if err != nil {
		log.Error("preload Guilds table failed")
		return
	}else{
		log.Info("preload Guilds table succeed !")
	}
	err = this.GuildStages.Preload()
	if err != nil {
		log.Error("preload GuildStages table failed")
		return
	}else{
		log.Info("preload GuildStages table succeed !")
	}
	err = this.ActivitysToDeletes.Preload()
	if err != nil {
		log.Error("preload ActivitysToDeletes table failed")
		return
	}else{
		log.Info("preload ActivitysToDeletes table succeed !")
	}
	err = this.SysMailCommon.Preload()
	if err != nil {
		log.Error("preload SysMailCommon table failed")
		return
	}else{
		log.Info("preload SysMailCommon table succeed !")
	}
	err = this.SysMails.Preload()
	if err != nil {
		log.Error("preload SysMails table failed")
		return
	}else{
		log.Info("preload SysMails table succeed !")
	}
	err = this.BanPlayers.Preload()
	if err != nil {
		log.Error("preload BanPlayers table failed")
		return
	}else{
		log.Info("preload BanPlayers table succeed !")
	}
	err = this.Carnival.Preload()
	if err != nil {
		log.Error("preload Carnival table failed")
		return
	}else{
		log.Info("preload Carnival table succeed !")
	}
	err = this.OtherServerPlayers.Preload()
	if err != nil {
		log.Error("preload OtherServerPlayers table failed")
		return
	}else{
		log.Info("preload OtherServerPlayers table succeed !")
	}
	err = this.on_preload()
	if err != nil {
		log.Error("on_preload failed")
		return
	}
	err = this.Save(true)
	if err != nil {
		log.Error("save on preload failed")
		return
	}
	return
}
func (this *DBC)Save(quick bool)(err error){
	err = this.Global.Save(quick)
	if err != nil {
		log.Error("save Global table failed")
		return
	}
	err = this.Players.Save(quick)
	if err != nil {
		log.Error("save Players table failed")
		return
	}
	err = this.BattleSaves.Save(quick)
	if err != nil {
		log.Error("save BattleSaves table failed")
		return
	}
	err = this.TowerFightSaves.Save(quick)
	if err != nil {
		log.Error("save TowerFightSaves table failed")
		return
	}
	err = this.ArenaSeason.Save(quick)
	if err != nil {
		log.Error("save ArenaSeason table failed")
		return
	}
	err = this.Guilds.Save(quick)
	if err != nil {
		log.Error("save Guilds table failed")
		return
	}
	err = this.GuildStages.Save(quick)
	if err != nil {
		log.Error("save GuildStages table failed")
		return
	}
	err = this.ActivitysToDeletes.Save(quick)
	if err != nil {
		log.Error("save ActivitysToDeletes table failed")
		return
	}
	err = this.SysMailCommon.Save(quick)
	if err != nil {
		log.Error("save SysMailCommon table failed")
		return
	}
	err = this.SysMails.Save(quick)
	if err != nil {
		log.Error("save SysMails table failed")
		return
	}
	err = this.BanPlayers.Save(quick)
	if err != nil {
		log.Error("save BanPlayers table failed")
		return
	}
	err = this.Carnival.Save(quick)
	if err != nil {
		log.Error("save Carnival table failed")
		return
	}
	err = this.OtherServerPlayers.Save(quick)
	if err != nil {
		log.Error("save OtherServerPlayers table failed")
		return
	}
	return
}
