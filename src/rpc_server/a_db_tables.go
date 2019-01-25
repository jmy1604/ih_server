package main

import (
	_ "ih_server/third_party/mysql"
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"ih_server/libs/log"
	"math/rand"
	"os"
	"os/exec"
	_ "ih_server/proto/gen_go/db_rpc"
	"sort"
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
		log.Trace("db存数据花费时长: %v", time.Now().Sub(begin).Nanoseconds())

		now_time_hour := int32(time.Now().Hour())
		if now_time_hour != this.m_db_last_copy_time {
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
			this.m_db_last_copy_time = now_time_hour
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


func (this *dbPlayerImportantDataRow)GetAccount( )(r string ){
	this.m_lock.UnSafeRLock("dbPlayerImportantDataRow.GetdbPlayerImportantDataAccountColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_Account)
}
func (this *dbPlayerImportantDataRow)SetAccount(v string){
	this.m_lock.UnSafeLock("dbPlayerImportantDataRow.SetdbPlayerImportantDataAccountColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Account=string(v)
	this.m_Account_changed=true
	return
}
func (this *dbPlayerImportantDataRow)GetUniqueId( )(r string ){
	this.m_lock.UnSafeRLock("dbPlayerImportantDataRow.GetdbPlayerImportantDataUniqueIdColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_UniqueId)
}
func (this *dbPlayerImportantDataRow)SetUniqueId(v string){
	this.m_lock.UnSafeLock("dbPlayerImportantDataRow.SetdbPlayerImportantDataUniqueIdColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_UniqueId=string(v)
	this.m_UniqueId_changed=true
	return
}
func (this *dbPlayerImportantDataRow)GetName( )(r string ){
	this.m_lock.UnSafeRLock("dbPlayerImportantDataRow.GetdbPlayerImportantDataNameColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_Name)
}
func (this *dbPlayerImportantDataRow)SetName(v string){
	this.m_lock.UnSafeLock("dbPlayerImportantDataRow.SetdbPlayerImportantDataNameColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Name=string(v)
	this.m_Name_changed=true
	return
}
func (this *dbPlayerImportantDataRow)GetLevel( )(r int32 ){
	this.m_lock.UnSafeRLock("dbPlayerImportantDataRow.GetdbPlayerImportantDataLevelColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_Level)
}
func (this *dbPlayerImportantDataRow)SetLevel(v int32){
	this.m_lock.UnSafeLock("dbPlayerImportantDataRow.SetdbPlayerImportantDataLevelColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Level=int32(v)
	this.m_Level_changed=true
	return
}
func (this *dbPlayerImportantDataRow)GetVipLevel( )(r int32 ){
	this.m_lock.UnSafeRLock("dbPlayerImportantDataRow.GetdbPlayerImportantDataVipLevelColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_VipLevel)
}
func (this *dbPlayerImportantDataRow)SetVipLevel(v int32){
	this.m_lock.UnSafeLock("dbPlayerImportantDataRow.SetdbPlayerImportantDataVipLevelColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_VipLevel=int32(v)
	this.m_VipLevel_changed=true
	return
}
func (this *dbPlayerImportantDataRow)GetGold( )(r int32 ){
	this.m_lock.UnSafeRLock("dbPlayerImportantDataRow.GetdbPlayerImportantDataGoldColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_Gold)
}
func (this *dbPlayerImportantDataRow)SetGold(v int32){
	this.m_lock.UnSafeLock("dbPlayerImportantDataRow.SetdbPlayerImportantDataGoldColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Gold=int32(v)
	this.m_Gold_changed=true
	return
}
func (this *dbPlayerImportantDataRow)GetDiamond( )(r int32 ){
	this.m_lock.UnSafeRLock("dbPlayerImportantDataRow.GetdbPlayerImportantDataDiamondColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_Diamond)
}
func (this *dbPlayerImportantDataRow)SetDiamond(v int32){
	this.m_lock.UnSafeLock("dbPlayerImportantDataRow.SetdbPlayerImportantDataDiamondColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Diamond=int32(v)
	this.m_Diamond_changed=true
	return
}
func (this *dbPlayerImportantDataRow)GetGuildId( )(r int32 ){
	this.m_lock.UnSafeRLock("dbPlayerImportantDataRow.GetdbPlayerImportantDataGuildIdColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_GuildId)
}
func (this *dbPlayerImportantDataRow)SetGuildId(v int32){
	this.m_lock.UnSafeLock("dbPlayerImportantDataRow.SetdbPlayerImportantDataGuildIdColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_GuildId=int32(v)
	this.m_GuildId_changed=true
	return
}
func (this *dbPlayerImportantDataRow)GetGuildName( )(r string ){
	this.m_lock.UnSafeRLock("dbPlayerImportantDataRow.GetdbPlayerImportantDataGuildNameColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_GuildName)
}
func (this *dbPlayerImportantDataRow)SetGuildName(v string){
	this.m_lock.UnSafeLock("dbPlayerImportantDataRow.SetdbPlayerImportantDataGuildNameColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_GuildName=string(v)
	this.m_GuildName_changed=true
	return
}
func (this *dbPlayerImportantDataRow)GetPassedCampaignId( )(r int32 ){
	this.m_lock.UnSafeRLock("dbPlayerImportantDataRow.GetdbPlayerImportantDataPassedCampaignIdColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_PassedCampaignId)
}
func (this *dbPlayerImportantDataRow)SetPassedCampaignId(v int32){
	this.m_lock.UnSafeLock("dbPlayerImportantDataRow.SetdbPlayerImportantDataPassedCampaignIdColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_PassedCampaignId=int32(v)
	this.m_PassedCampaignId_changed=true
	return
}
func (this *dbPlayerImportantDataRow)GetHungupCampaignId( )(r int32 ){
	this.m_lock.UnSafeRLock("dbPlayerImportantDataRow.GetdbPlayerImportantDataHungupCampaignIdColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_HungupCampaignId)
}
func (this *dbPlayerImportantDataRow)SetHungupCampaignId(v int32){
	this.m_lock.UnSafeLock("dbPlayerImportantDataRow.SetdbPlayerImportantDataHungupCampaignIdColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_HungupCampaignId=int32(v)
	this.m_HungupCampaignId_changed=true
	return
}
func (this *dbPlayerImportantDataRow)GetArenaScore( )(r int32 ){
	this.m_lock.UnSafeRLock("dbPlayerImportantDataRow.GetdbPlayerImportantDataArenaScoreColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_ArenaScore)
}
func (this *dbPlayerImportantDataRow)SetArenaScore(v int32){
	this.m_lock.UnSafeLock("dbPlayerImportantDataRow.SetdbPlayerImportantDataArenaScoreColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_ArenaScore=int32(v)
	this.m_ArenaScore_changed=true
	return
}
func (this *dbPlayerImportantDataRow)GetTalentPoint( )(r int32 ){
	this.m_lock.UnSafeRLock("dbPlayerImportantDataRow.GetdbPlayerImportantDataTalentPointColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_TalentPoint)
}
func (this *dbPlayerImportantDataRow)SetTalentPoint(v int32){
	this.m_lock.UnSafeLock("dbPlayerImportantDataRow.SetdbPlayerImportantDataTalentPointColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_TalentPoint=int32(v)
	this.m_TalentPoint_changed=true
	return
}
func (this *dbPlayerImportantDataRow)GetTowerId( )(r int32 ){
	this.m_lock.UnSafeRLock("dbPlayerImportantDataRow.GetdbPlayerImportantDataTowerIdColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_TowerId)
}
func (this *dbPlayerImportantDataRow)SetTowerId(v int32){
	this.m_lock.UnSafeLock("dbPlayerImportantDataRow.SetdbPlayerImportantDataTowerIdColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_TowerId=int32(v)
	this.m_TowerId_changed=true
	return
}
func (this *dbPlayerImportantDataRow)GetSignIn( )(r int32 ){
	this.m_lock.UnSafeRLock("dbPlayerImportantDataRow.GetdbPlayerImportantDataSignInColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_SignIn)
}
func (this *dbPlayerImportantDataRow)SetSignIn(v int32){
	this.m_lock.UnSafeLock("dbPlayerImportantDataRow.SetdbPlayerImportantDataSignInColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_SignIn=int32(v)
	this.m_SignIn_changed=true
	return
}
type dbPlayerImportantDataRow struct {
	m_table *dbPlayerImportantDataTable
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
	m_UniqueId_changed bool
	m_UniqueId string
	m_Name_changed bool
	m_Name string
	m_Level_changed bool
	m_Level int32
	m_VipLevel_changed bool
	m_VipLevel int32
	m_Gold_changed bool
	m_Gold int32
	m_Diamond_changed bool
	m_Diamond int32
	m_GuildId_changed bool
	m_GuildId int32
	m_GuildName_changed bool
	m_GuildName string
	m_PassedCampaignId_changed bool
	m_PassedCampaignId int32
	m_HungupCampaignId_changed bool
	m_HungupCampaignId int32
	m_ArenaScore_changed bool
	m_ArenaScore int32
	m_TalentPoint_changed bool
	m_TalentPoint int32
	m_TowerId_changed bool
	m_TowerId int32
	m_SignIn_changed bool
	m_SignIn int32
}
func new_dbPlayerImportantDataRow(table *dbPlayerImportantDataTable, PlayerId int32) (r *dbPlayerImportantDataRow) {
	this := &dbPlayerImportantDataRow{}
	this.m_table = table
	this.m_PlayerId = PlayerId
	this.m_lock = NewRWMutex()
	this.m_Account_changed=true
	this.m_UniqueId_changed=true
	this.m_Name_changed=true
	this.m_Level_changed=true
	this.m_VipLevel_changed=true
	this.m_Gold_changed=true
	this.m_Diamond_changed=true
	this.m_GuildId_changed=true
	this.m_GuildName_changed=true
	this.m_PassedCampaignId_changed=true
	this.m_HungupCampaignId_changed=true
	this.m_ArenaScore_changed=true
	this.m_TalentPoint_changed=true
	this.m_TowerId_changed=true
	this.m_SignIn_changed=true
	return this
}
func (this *dbPlayerImportantDataRow) GetPlayerId() (r int32) {
	return this.m_PlayerId
}
func (this *dbPlayerImportantDataRow) Load() (err error) {
	this.m_table.GC()
	this.m_lock.UnSafeLock("dbPlayerImportantDataRow.Load")
	defer this.m_lock.UnSafeUnlock()
	if this.m_loaded {
		return
	}
	var dAccount string
	var dUniqueId string
	var dName string
	var dLevel int32
	var dVipLevel int32
	var dGold int32
	var dDiamond int32
	var dGuildId int32
	var dGuildName string
	var dPassedCampaignId int32
	var dHungupCampaignId int32
	var dArenaScore int32
	var dTalentPoint int32
	var dTowerId int32
	var dSignIn int32
	r := this.m_table.m_dbc.StmtQueryRow(this.m_table.m_load_select_stmt, this.m_PlayerId)
	err = r.Scan(&dAccount,&dUniqueId,&dName,&dLevel,&dVipLevel,&dGold,&dDiamond,&dGuildId,&dGuildName,&dPassedCampaignId,&dHungupCampaignId,&dArenaScore,&dTalentPoint,&dTowerId,&dSignIn)
	if err != nil {
		log.Error("Scan err[%v]", err.Error())
		return
	}
		this.m_Account=dAccount
		this.m_UniqueId=dUniqueId
		this.m_Name=dName
		this.m_Level=dLevel
		this.m_VipLevel=dVipLevel
		this.m_Gold=dGold
		this.m_Diamond=dDiamond
		this.m_GuildId=dGuildId
		this.m_GuildName=dGuildName
		this.m_PassedCampaignId=dPassedCampaignId
		this.m_HungupCampaignId=dHungupCampaignId
		this.m_ArenaScore=dArenaScore
		this.m_TalentPoint=dTalentPoint
		this.m_TowerId=dTowerId
		this.m_SignIn=dSignIn
	this.m_loaded=true
	this.m_Account_changed=false
	this.m_UniqueId_changed=false
	this.m_Name_changed=false
	this.m_Level_changed=false
	this.m_VipLevel_changed=false
	this.m_Gold_changed=false
	this.m_Diamond_changed=false
	this.m_GuildId_changed=false
	this.m_GuildName_changed=false
	this.m_PassedCampaignId_changed=false
	this.m_HungupCampaignId_changed=false
	this.m_ArenaScore_changed=false
	this.m_TalentPoint_changed=false
	this.m_TowerId_changed=false
	this.m_SignIn_changed=false
	this.Touch(false)
	atomic.AddInt32(&this.m_table.m_gc_n,1)
	return
}
func (this *dbPlayerImportantDataRow) save_data(release bool) (err error, released bool, state int32, update_string string, args []interface{}) {
	this.m_lock.UnSafeLock("dbPlayerImportantDataRow.save_data")
	defer this.m_lock.UnSafeUnlock()
	if this.m_new {
		db_args:=new_db_args(16)
		db_args.Push(this.m_PlayerId)
		db_args.Push(this.m_Account)
		db_args.Push(this.m_UniqueId)
		db_args.Push(this.m_Name)
		db_args.Push(this.m_Level)
		db_args.Push(this.m_VipLevel)
		db_args.Push(this.m_Gold)
		db_args.Push(this.m_Diamond)
		db_args.Push(this.m_GuildId)
		db_args.Push(this.m_GuildName)
		db_args.Push(this.m_PassedCampaignId)
		db_args.Push(this.m_HungupCampaignId)
		db_args.Push(this.m_ArenaScore)
		db_args.Push(this.m_TalentPoint)
		db_args.Push(this.m_TowerId)
		db_args.Push(this.m_SignIn)
		args=db_args.GetArgs()
		state = 1
	} else {
		if this.m_Account_changed||this.m_UniqueId_changed||this.m_Name_changed||this.m_Level_changed||this.m_VipLevel_changed||this.m_Gold_changed||this.m_Diamond_changed||this.m_GuildId_changed||this.m_GuildName_changed||this.m_PassedCampaignId_changed||this.m_HungupCampaignId_changed||this.m_ArenaScore_changed||this.m_TalentPoint_changed||this.m_TowerId_changed||this.m_SignIn_changed{
			update_string = "UPDATE PlayerImportantDatas SET "
			db_args:=new_db_args(16)
			if this.m_Account_changed{
				update_string+="Account=?,"
				db_args.Push(this.m_Account)
			}
			if this.m_UniqueId_changed{
				update_string+="UniqueId=?,"
				db_args.Push(this.m_UniqueId)
			}
			if this.m_Name_changed{
				update_string+="Name=?,"
				db_args.Push(this.m_Name)
			}
			if this.m_Level_changed{
				update_string+="Level=?,"
				db_args.Push(this.m_Level)
			}
			if this.m_VipLevel_changed{
				update_string+="VipLevel=?,"
				db_args.Push(this.m_VipLevel)
			}
			if this.m_Gold_changed{
				update_string+="Gold=?,"
				db_args.Push(this.m_Gold)
			}
			if this.m_Diamond_changed{
				update_string+="Diamond=?,"
				db_args.Push(this.m_Diamond)
			}
			if this.m_GuildId_changed{
				update_string+="GuildId=?,"
				db_args.Push(this.m_GuildId)
			}
			if this.m_GuildName_changed{
				update_string+="GuildName=?,"
				db_args.Push(this.m_GuildName)
			}
			if this.m_PassedCampaignId_changed{
				update_string+="PassedCampaignId=?,"
				db_args.Push(this.m_PassedCampaignId)
			}
			if this.m_HungupCampaignId_changed{
				update_string+="HungupCampaignId=?,"
				db_args.Push(this.m_HungupCampaignId)
			}
			if this.m_ArenaScore_changed{
				update_string+="ArenaScore=?,"
				db_args.Push(this.m_ArenaScore)
			}
			if this.m_TalentPoint_changed{
				update_string+="TalentPoint=?,"
				db_args.Push(this.m_TalentPoint)
			}
			if this.m_TowerId_changed{
				update_string+="TowerId=?,"
				db_args.Push(this.m_TowerId)
			}
			if this.m_SignIn_changed{
				update_string+="SignIn=?,"
				db_args.Push(this.m_SignIn)
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
	this.m_UniqueId_changed = false
	this.m_Name_changed = false
	this.m_Level_changed = false
	this.m_VipLevel_changed = false
	this.m_Gold_changed = false
	this.m_Diamond_changed = false
	this.m_GuildId_changed = false
	this.m_GuildName_changed = false
	this.m_PassedCampaignId_changed = false
	this.m_HungupCampaignId_changed = false
	this.m_ArenaScore_changed = false
	this.m_TalentPoint_changed = false
	this.m_TowerId_changed = false
	this.m_SignIn_changed = false
	if release && this.m_loaded {
		atomic.AddInt32(&this.m_table.m_gc_n, -1)
		this.m_loaded = false
		released = true
	}
	return nil,released,state,update_string,args
}
func (this *dbPlayerImportantDataRow) Save(release bool) (err error, d bool, released bool) {
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
			log.Error("INSERT PlayerImportantDatas exec failed %v ", this.m_PlayerId)
			return err, false, released
		}
		d = true
	} else if state == 2 {
		_, err = this.m_table.m_dbc.Exec(update_string, args...)
		if err != nil {
			log.Error("UPDATE PlayerImportantDatas exec failed %v", this.m_PlayerId)
			return err, false, released
		}
		d = true
	}
	return nil, d, released
}
func (this *dbPlayerImportantDataRow) Touch(releasable bool) {
	this.m_touch = int32(time.Now().Unix())
	this.m_releasable = releasable
}
type dbPlayerImportantDataRowSort struct {
	rows []*dbPlayerImportantDataRow
}
func (this *dbPlayerImportantDataRowSort) Len() (length int) {
	return len(this.rows)
}
func (this *dbPlayerImportantDataRowSort) Less(i int, j int) (less bool) {
	return this.rows[i].m_touch < this.rows[j].m_touch
}
func (this *dbPlayerImportantDataRowSort) Swap(i int, j int) {
	temp := this.rows[i]
	this.rows[i] = this.rows[j]
	this.rows[j] = temp
}
type dbPlayerImportantDataTable struct{
	m_dbc *DBC
	m_lock *RWMutex
	m_rows map[int32]*dbPlayerImportantDataRow
	m_new_rows map[int32]*dbPlayerImportantDataRow
	m_removed_rows map[int32]*dbPlayerImportantDataRow
	m_gc_n int32
	m_gcing int32
	m_pool_size int32
	m_preload_select_stmt *sql.Stmt
	m_preload_max_id int32
	m_load_select_stmt *sql.Stmt
	m_save_insert_stmt *sql.Stmt
	m_delete_stmt *sql.Stmt
}
func new_dbPlayerImportantDataTable(dbc *DBC) (this *dbPlayerImportantDataTable) {
	this = &dbPlayerImportantDataTable{}
	this.m_dbc = dbc
	this.m_lock = NewRWMutex()
	this.m_rows = make(map[int32]*dbPlayerImportantDataRow)
	this.m_new_rows = make(map[int32]*dbPlayerImportantDataRow)
	this.m_removed_rows = make(map[int32]*dbPlayerImportantDataRow)
	return this
}
func (this *dbPlayerImportantDataTable) check_create_table() (err error) {
	_, err = this.m_dbc.Exec("CREATE TABLE IF NOT EXISTS PlayerImportantDatas(PlayerId int(11),PRIMARY KEY (PlayerId))ENGINE=InnoDB ROW_FORMAT=DYNAMIC")
	if err != nil {
		log.Error("CREATE TABLE IF NOT EXISTS PlayerImportantDatas failed")
		return
	}
	rows, err := this.m_dbc.Query("SELECT COLUMN_NAME,ORDINAL_POSITION FROM information_schema.`COLUMNS` WHERE TABLE_SCHEMA=? AND TABLE_NAME='PlayerImportantDatas'", this.m_dbc.m_db_name)
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
		_, err = this.m_dbc.Exec("ALTER TABLE PlayerImportantDatas ADD COLUMN Account varchar(45)")
		if err != nil {
			log.Error("ADD COLUMN Account failed")
			return
		}
	}
	_, hasUniqueId := columns["UniqueId"]
	if !hasUniqueId {
		_, err = this.m_dbc.Exec("ALTER TABLE PlayerImportantDatas ADD COLUMN UniqueId varchar(45)")
		if err != nil {
			log.Error("ADD COLUMN UniqueId failed")
			return
		}
	}
	_, hasName := columns["Name"]
	if !hasName {
		_, err = this.m_dbc.Exec("ALTER TABLE PlayerImportantDatas ADD COLUMN Name varchar(45)")
		if err != nil {
			log.Error("ADD COLUMN Name failed")
			return
		}
	}
	_, hasLevel := columns["Level"]
	if !hasLevel {
		_, err = this.m_dbc.Exec("ALTER TABLE PlayerImportantDatas ADD COLUMN Level int(11)")
		if err != nil {
			log.Error("ADD COLUMN Level failed")
			return
		}
	}
	_, hasVipLevel := columns["VipLevel"]
	if !hasVipLevel {
		_, err = this.m_dbc.Exec("ALTER TABLE PlayerImportantDatas ADD COLUMN VipLevel int(11)")
		if err != nil {
			log.Error("ADD COLUMN VipLevel failed")
			return
		}
	}
	_, hasGold := columns["Gold"]
	if !hasGold {
		_, err = this.m_dbc.Exec("ALTER TABLE PlayerImportantDatas ADD COLUMN Gold int(11)")
		if err != nil {
			log.Error("ADD COLUMN Gold failed")
			return
		}
	}
	_, hasDiamond := columns["Diamond"]
	if !hasDiamond {
		_, err = this.m_dbc.Exec("ALTER TABLE PlayerImportantDatas ADD COLUMN Diamond int(11)")
		if err != nil {
			log.Error("ADD COLUMN Diamond failed")
			return
		}
	}
	_, hasGuildId := columns["GuildId"]
	if !hasGuildId {
		_, err = this.m_dbc.Exec("ALTER TABLE PlayerImportantDatas ADD COLUMN GuildId int(11)")
		if err != nil {
			log.Error("ADD COLUMN GuildId failed")
			return
		}
	}
	_, hasGuildName := columns["GuildName"]
	if !hasGuildName {
		_, err = this.m_dbc.Exec("ALTER TABLE PlayerImportantDatas ADD COLUMN GuildName varchar(45)")
		if err != nil {
			log.Error("ADD COLUMN GuildName failed")
			return
		}
	}
	_, hasPassedCampaignId := columns["PassedCampaignId"]
	if !hasPassedCampaignId {
		_, err = this.m_dbc.Exec("ALTER TABLE PlayerImportantDatas ADD COLUMN PassedCampaignId int(11)")
		if err != nil {
			log.Error("ADD COLUMN PassedCampaignId failed")
			return
		}
	}
	_, hasHungupCampaignId := columns["HungupCampaignId"]
	if !hasHungupCampaignId {
		_, err = this.m_dbc.Exec("ALTER TABLE PlayerImportantDatas ADD COLUMN HungupCampaignId int(11)")
		if err != nil {
			log.Error("ADD COLUMN HungupCampaignId failed")
			return
		}
	}
	_, hasArenaScore := columns["ArenaScore"]
	if !hasArenaScore {
		_, err = this.m_dbc.Exec("ALTER TABLE PlayerImportantDatas ADD COLUMN ArenaScore int(11)")
		if err != nil {
			log.Error("ADD COLUMN ArenaScore failed")
			return
		}
	}
	_, hasTalentPoint := columns["TalentPoint"]
	if !hasTalentPoint {
		_, err = this.m_dbc.Exec("ALTER TABLE PlayerImportantDatas ADD COLUMN TalentPoint int(11)")
		if err != nil {
			log.Error("ADD COLUMN TalentPoint failed")
			return
		}
	}
	_, hasTowerId := columns["TowerId"]
	if !hasTowerId {
		_, err = this.m_dbc.Exec("ALTER TABLE PlayerImportantDatas ADD COLUMN TowerId int(11)")
		if err != nil {
			log.Error("ADD COLUMN TowerId failed")
			return
		}
	}
	_, hasSignIn := columns["SignIn"]
	if !hasSignIn {
		_, err = this.m_dbc.Exec("ALTER TABLE PlayerImportantDatas ADD COLUMN SignIn int(11)")
		if err != nil {
			log.Error("ADD COLUMN SignIn failed")
			return
		}
	}
	return
}
func (this *dbPlayerImportantDataTable) prepare_preload_select_stmt() (err error) {
	this.m_preload_select_stmt,err=this.m_dbc.StmtPrepare("SELECT PlayerId FROM PlayerImportantDatas")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbPlayerImportantDataTable) prepare_load_select_stmt() (err error) {
	this.m_load_select_stmt,err=this.m_dbc.StmtPrepare("SELECT Account,UniqueId,Name,Level,VipLevel,Gold,Diamond,GuildId,GuildName,PassedCampaignId,HungupCampaignId,ArenaScore,TalentPoint,TowerId,SignIn FROM PlayerImportantDatas WHERE PlayerId=?")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbPlayerImportantDataTable) prepare_save_insert_stmt()(err error){
	this.m_save_insert_stmt,err=this.m_dbc.StmtPrepare("INSERT INTO PlayerImportantDatas (PlayerId,Account,UniqueId,Name,Level,VipLevel,Gold,Diamond,GuildId,GuildName,PassedCampaignId,HungupCampaignId,ArenaScore,TalentPoint,TowerId,SignIn) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbPlayerImportantDataTable) prepare_delete_stmt() (err error) {
	this.m_delete_stmt,err=this.m_dbc.StmtPrepare("DELETE FROM PlayerImportantDatas WHERE PlayerId=?")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbPlayerImportantDataTable) Init() (err error) {
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
	err=this.prepare_load_select_stmt()
	if err!=nil{
		log.Error("prepare_load_select_stmt failed")
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
func (this *dbPlayerImportantDataTable) Preload() (err error) {
	r, err := this.m_dbc.StmtQuery(this.m_preload_select_stmt)
	if err != nil {
		log.Error("SELECT")
		return
	}
	var PlayerId int32
		this.m_preload_max_id = 0
	for r.Next() {
		err = r.Scan(&PlayerId)
		if err != nil {
			log.Error("Scan err[%v]", err.Error())
			return
		}
		if PlayerId>this.m_preload_max_id{
			this.m_preload_max_id =PlayerId
		}
		row := new_dbPlayerImportantDataRow(this,PlayerId)
		row.m_valid = true
		this.m_rows[PlayerId]=row
	}
	return
}
func (this *dbPlayerImportantDataTable) GetPreloadedMaxId() (max_id int32) {
	return this.m_preload_max_id
}
func (this *dbPlayerImportantDataTable) fetch_rows(rows map[int32]*dbPlayerImportantDataRow) (r map[int32]*dbPlayerImportantDataRow) {
	this.m_lock.UnSafeLock("dbPlayerImportantDataTable.fetch_rows")
	defer this.m_lock.UnSafeUnlock()
	r = make(map[int32]*dbPlayerImportantDataRow)
	for i, v := range rows {
		r[i] = v
	}
	return r
}
func (this *dbPlayerImportantDataTable) fetch_new_rows() (new_rows map[int32]*dbPlayerImportantDataRow) {
	this.m_lock.UnSafeLock("dbPlayerImportantDataTable.fetch_new_rows")
	defer this.m_lock.UnSafeUnlock()
	new_rows = make(map[int32]*dbPlayerImportantDataRow)
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
func (this *dbPlayerImportantDataTable) save_rows(rows map[int32]*dbPlayerImportantDataRow, quick bool) {
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
func (this *dbPlayerImportantDataTable) Save(quick bool) (err error){
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
	this.m_removed_rows = make(map[int32]*dbPlayerImportantDataRow)
	rows := this.fetch_rows(this.m_rows)
	this.save_rows(rows, quick)
	new_rows := this.fetch_new_rows()
	this.save_rows(new_rows, quick)
	return
}
func (this *dbPlayerImportantDataTable) AddRow(PlayerId int32) (row *dbPlayerImportantDataRow) {
	this.GC()
	this.m_lock.UnSafeLock("dbPlayerImportantDataTable.AddRow")
	defer this.m_lock.UnSafeUnlock()
	row = new_dbPlayerImportantDataRow(this,PlayerId)
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
func (this *dbPlayerImportantDataTable) RemoveRow(PlayerId int32) {
	this.m_lock.UnSafeLock("dbPlayerImportantDataTable.RemoveRow")
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
func (this *dbPlayerImportantDataTable) GetRow(PlayerId int32) (row *dbPlayerImportantDataRow) {
	this.m_lock.UnSafeRLock("dbPlayerImportantDataTable.GetRow")
	defer this.m_lock.UnSafeRUnlock()
	row = this.m_rows[PlayerId]
	if row == nil {
		row = this.m_new_rows[PlayerId]
	}
	return row
}
func (this *dbPlayerImportantDataTable) SetPoolSize(n int32) {
	this.m_pool_size = n
}
func (this *dbPlayerImportantDataTable) GC() {
	if this.m_pool_size<=0{
		return
	}
	if !atomic.CompareAndSwapInt32(&this.m_gcing, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&this.m_gcing, 0)
	n := atomic.LoadInt32(&this.m_gc_n)
	if float32(n) < float32(this.m_pool_size)*1.2 {
		return
	}
	max := (n - this.m_pool_size) / 2
	arr := dbPlayerImportantDataRowSort{}
	rows := this.fetch_rows(this.m_rows)
	arr.rows = make([]*dbPlayerImportantDataRow, len(rows))
	index := 0
	for _, v := range rows {
		arr.rows[index] = v
		index++
	}
	sort.Sort(&arr)
	count := int32(0)
	for _, v := range arr.rows {
		err, _, released := v.Save(true)
		if err != nil {
			log.Error("release failed %v", err)
			continue
		}
		if released {
			count++
			if count > max {
				return
			}
		}
	}
	return
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
	PlayerImportantDatas *dbPlayerImportantDataTable
}
func (this *DBC)init_tables()(err error){
	this.PlayerImportantDatas = new_dbPlayerImportantDataTable(this)
	err = this.PlayerImportantDatas.Init()
	if err != nil {
		log.Error("init PlayerImportantDatas table failed")
		return
	}
	return
}
func (this *DBC)Preload()(err error){
	err = this.PlayerImportantDatas.Preload()
	if err != nil {
		log.Error("preload PlayerImportantDatas table failed")
		return
	}else{
		log.Info("preload PlayerImportantDatas table succeed !")
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
	err = this.PlayerImportantDatas.Save(quick)
	if err != nil {
		log.Error("save PlayerImportantDatas table failed")
		return
	}
	return
}
