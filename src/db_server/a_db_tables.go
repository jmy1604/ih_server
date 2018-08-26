package main

import (
	"3p/code.google.com.protobuf/proto"
	_ "3p/mysql"
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"libs/log"
	"math/rand"
	"os"
	"os/exec"
	"public_message/gen_go/db_dbsvr"
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

type dbPlayerInfoData struct{
	MatchScore int32
	Coin int32
	Diamond int32
	DayMatchRewardNum int32
	LayMatchRewardTime int32
	CurUseCardTeam int32
}
func (this* dbPlayerInfoData)from_pb(pb *db.PlayerInfo){
	if pb == nil {
		return
	}
	this.MatchScore = pb.GetMatchScore()
	this.Coin = pb.GetCoin()
	this.Diamond = pb.GetDiamond()
	this.DayMatchRewardNum = pb.GetDayMatchRewardNum()
	this.LayMatchRewardTime = pb.GetLayMatchRewardTime()
	this.CurUseCardTeam = pb.GetCurUseCardTeam()
	return
}
func (this* dbPlayerInfoData)to_pb()(pb *db.PlayerInfo){
	pb = &db.PlayerInfo{}
	pb.MatchScore = proto.Int32(this.MatchScore)
	pb.Coin = proto.Int32(this.Coin)
	pb.Diamond = proto.Int32(this.Diamond)
	pb.DayMatchRewardNum = proto.Int32(this.DayMatchRewardNum)
	pb.LayMatchRewardTime = proto.Int32(this.LayMatchRewardTime)
	pb.CurUseCardTeam = proto.Int32(this.CurUseCardTeam)
	return
}
func (this* dbPlayerInfoData)clone_to(d *dbPlayerInfoData){
	d.MatchScore = this.MatchScore
	d.Coin = this.Coin
	d.Diamond = this.Diamond
	d.DayMatchRewardNum = this.DayMatchRewardNum
	d.LayMatchRewardTime = this.LayMatchRewardTime
	d.CurUseCardTeam = this.CurUseCardTeam
	return
}
type dbPlayerCardData struct{
	ConfigId int32
	CardCount int32
}
func (this* dbPlayerCardData)from_pb(pb *db.PlayerCard){
	if pb == nil {
		return
	}
	this.ConfigId = pb.GetConfigId()
	this.CardCount = pb.GetCardCount()
	return
}
func (this* dbPlayerCardData)to_pb()(pb *db.PlayerCard){
	pb = &db.PlayerCard{}
	pb.ConfigId = proto.Int32(this.ConfigId)
	pb.CardCount = proto.Int32(this.CardCount)
	return
}
func (this* dbPlayerCardData)clone_to(d *dbPlayerCardData){
	d.ConfigId = this.ConfigId
	d.CardCount = this.CardCount
	return
}
type dbPlayerCardTeamData struct{
	TeamId int32
	CardCfgIds []int32
}
func (this* dbPlayerCardTeamData)from_pb(pb *db.PlayerCardTeam){
	if pb == nil {
		this.CardCfgIds = make([]int32,0)
		return
	}
	this.TeamId = pb.GetTeamId()
	this.CardCfgIds = make([]int32,len(pb.GetCardCfgIds()))
	for i, v := range pb.GetCardCfgIds() {
		this.CardCfgIds[i] = v
	}
	return
}
func (this* dbPlayerCardTeamData)to_pb()(pb *db.PlayerCardTeam){
	pb = &db.PlayerCardTeam{}
	pb.TeamId = proto.Int32(this.TeamId)
	pb.CardCfgIds = make([]int32, len(this.CardCfgIds))
	for i, v := range this.CardCfgIds {
		pb.CardCfgIds[i]=v
	}
	return
}
func (this* dbPlayerCardTeamData)clone_to(d *dbPlayerCardTeamData){
	d.TeamId = this.TeamId
	d.CardCfgIds = make([]int32, len(this.CardCfgIds))
	for _ii, _vv := range this.CardCfgIds {
		d.CardCfgIds[_ii]=_vv
	}
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
func (this *dbPlayerInfoColumn)GetMatchScore( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerInfoColumn.GetMatchScore")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.MatchScore
	return
}
func (this *dbPlayerInfoColumn)SetMatchScore(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerInfoColumn.SetMatchScore")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.MatchScore = v
	this.m_changed = true
	return
}
func (this *dbPlayerInfoColumn)IncbyMatchScore(v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerInfoColumn.IncbyMatchScore")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.MatchScore += v
	this.m_changed = true
	return this.m_data.MatchScore
}
func (this *dbPlayerInfoColumn)GetCoin( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerInfoColumn.GetCoin")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.Coin
	return
}
func (this *dbPlayerInfoColumn)SetCoin(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerInfoColumn.SetCoin")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.Coin = v
	this.m_changed = true
	return
}
func (this *dbPlayerInfoColumn)IncbyCoin(v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerInfoColumn.IncbyCoin")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.Coin += v
	this.m_changed = true
	return this.m_data.Coin
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
func (this *dbPlayerInfoColumn)GetDayMatchRewardNum( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerInfoColumn.GetDayMatchRewardNum")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.DayMatchRewardNum
	return
}
func (this *dbPlayerInfoColumn)SetDayMatchRewardNum(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerInfoColumn.SetDayMatchRewardNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.DayMatchRewardNum = v
	this.m_changed = true
	return
}
func (this *dbPlayerInfoColumn)IncbyDayMatchRewardNum(v int32)(r int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerInfoColumn.IncbyDayMatchRewardNum")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.DayMatchRewardNum += v
	this.m_changed = true
	return this.m_data.DayMatchRewardNum
}
func (this *dbPlayerInfoColumn)GetLayMatchRewardTime( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerInfoColumn.GetLayMatchRewardTime")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.LayMatchRewardTime
	return
}
func (this *dbPlayerInfoColumn)SetLayMatchRewardTime(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerInfoColumn.SetLayMatchRewardTime")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.LayMatchRewardTime = v
	this.m_changed = true
	return
}
func (this *dbPlayerInfoColumn)GetCurUseCardTeam( )(v int32 ){
	this.m_row.m_lock.UnSafeRLock("dbPlayerInfoColumn.GetCurUseCardTeam")
	defer this.m_row.m_lock.UnSafeRUnlock()
	v = this.m_data.CurUseCardTeam
	return
}
func (this *dbPlayerInfoColumn)SetCurUseCardTeam(v int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerInfoColumn.SetCurUseCardTeam")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data.CurUseCardTeam = v
	this.m_changed = true
	return
}
type dbPlayerCardColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerCardData
	m_changed bool
}
func (this *dbPlayerCardColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerCardList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerCardData{}
		d.from_pb(v)
		this.m_data[int32(d.ConfigId)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerCardColumn)save( )(data []byte,err error){
	pb := &db.PlayerCardList{}
	pb.List=make([]*db.PlayerCard,len(this.m_data))
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
func (this *dbPlayerCardColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCardColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerCardColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCardColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerCardColumn)GetAll()(list []dbPlayerCardData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCardColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerCardData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerCardColumn)Get(id int32)(v *dbPlayerCardData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCardColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerCardData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerCardColumn)Set(v dbPlayerCardData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerCardColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.ConfigId)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.ConfigId)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerCardColumn)Add(v *dbPlayerCardData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerCardColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.ConfigId)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.ConfigId)
		return false
	}
	d:=&dbPlayerCardData{}
	v.clone_to(d)
	this.m_data[int32(v.ConfigId)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerCardColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCardColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerCardColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerCardColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerCardData)
	this.m_changed = true
	return
}
func (this *dbPlayerCardColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCardColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerCardColumn)GetCardCount(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCardColumn.GetCardCount")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.CardCount
	return v,true
}
func (this *dbPlayerCardColumn)SetCardCount(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerCardColumn.SetCardCount")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.CardCount = v
	this.m_changed = true
	return true
}
type dbPlayerCardTeamColumn struct{
	m_row *dbPlayerRow
	m_data map[int32]*dbPlayerCardTeamData
	m_changed bool
}
func (this *dbPlayerCardTeamColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.PlayerCardTeamList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetPlayerId())
		return
	}
	for _, v := range pb.List {
		d := &dbPlayerCardTeamData{}
		d.from_pb(v)
		this.m_data[int32(d.TeamId)] = d
	}
	this.m_changed = false
	return
}
func (this *dbPlayerCardTeamColumn)save( )(data []byte,err error){
	pb := &db.PlayerCardTeamList{}
	pb.List=make([]*db.PlayerCardTeam,len(this.m_data))
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
func (this *dbPlayerCardTeamColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCardTeamColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbPlayerCardTeamColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCardTeamColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbPlayerCardTeamColumn)GetAll()(list []dbPlayerCardTeamData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCardTeamColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbPlayerCardTeamData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbPlayerCardTeamColumn)Get(id int32)(v *dbPlayerCardTeamData){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCardTeamColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbPlayerCardTeamData{}
	d.clone_to(v)
	return
}
func (this *dbPlayerCardTeamColumn)Set(v dbPlayerCardTeamData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerCardTeamColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.TeamId)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), v.TeamId)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbPlayerCardTeamColumn)Add(v *dbPlayerCardTeamData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerCardTeamColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.TeamId)]
	if has {
		log.Error("already added %v %v",this.m_row.GetPlayerId(), v.TeamId)
		return false
	}
	d:=&dbPlayerCardTeamData{}
	v.clone_to(d)
	this.m_data[int32(v.TeamId)]=d
	this.m_changed = true
	return true
}
func (this *dbPlayerCardTeamColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbPlayerCardTeamColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbPlayerCardTeamColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbPlayerCardTeamColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbPlayerCardTeamData)
	this.m_changed = true
	return
}
func (this *dbPlayerCardTeamColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCardTeamColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbPlayerCardTeamColumn)GetCardCfgIds(id int32)(v []int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbPlayerCardTeamColumn.GetCardCfgIds")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = make([]int32, len(d.CardCfgIds))
	for _ii, _vv := range d.CardCfgIds {
		v[_ii]=_vv
	}
	return v,true
}
func (this *dbPlayerCardTeamColumn)SetCardCfgIds(id int32,v []int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbPlayerCardTeamColumn.SetCardCfgIds")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetPlayerId(), id)
		return
	}
	d.CardCfgIds = make([]int32, len(v))
	for _ii, _vv := range v {
		d.CardCfgIds[_ii]=_vv
	}
	this.m_changed = true
	return true
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
	Info dbPlayerInfoColumn
	Cards dbPlayerCardColumn
	CardTeams dbPlayerCardTeamColumn
}
func new_dbPlayerRow(table *dbPlayerTable, PlayerId int32) (r *dbPlayerRow) {
	this := &dbPlayerRow{}
	this.m_table = table
	this.m_PlayerId = PlayerId
	this.m_lock = NewRWMutex()
	this.Info.m_row=this
	this.Info.m_data=&dbPlayerInfoData{}
	this.Cards.m_row=this
	this.Cards.m_data=make(map[int32]*dbPlayerCardData)
	this.CardTeams.m_row=this
	this.CardTeams.m_data=make(map[int32]*dbPlayerCardTeamData)
	return this
}
func (this *dbPlayerRow) GetPlayerId() (r int32) {
	return this.m_PlayerId
}
func (this *dbPlayerRow) save_data(release bool) (err error, released bool, state int32, update_string string, args []interface{}) {
	this.m_lock.UnSafeLock("dbPlayerRow.save_data")
	defer this.m_lock.UnSafeUnlock()
	if this.m_new {
		db_args:=new_db_args(4)
		db_args.Push(this.m_PlayerId)
		dInfo,db_err:=this.Info.save()
		if db_err!=nil{
			log.Error("insert save Info failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dInfo)
		dCards,db_err:=this.Cards.save()
		if db_err!=nil{
			log.Error("insert save Card failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dCards)
		dCardTeams,db_err:=this.CardTeams.save()
		if db_err!=nil{
			log.Error("insert save CardTeam failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dCardTeams)
		args=db_args.GetArgs()
		state = 1
	} else {
		if this.Info.m_changed||this.Cards.m_changed||this.CardTeams.m_changed{
			update_string = "UPDATE Players SET "
			db_args:=new_db_args(4)
			if this.Info.m_changed{
				update_string+="Info=?,"
				dInfo,err:=this.Info.save()
				if err!=nil{
					log.Error("update save Info failed")
					return err,false,0,"",nil
				}
				db_args.Push(dInfo)
			}
			if this.Cards.m_changed{
				update_string+="Cards=?,"
				dCards,err:=this.Cards.save()
				if err!=nil{
					log.Error("insert save Card failed")
					return err,false,0,"",nil
				}
				db_args.Push(dCards)
			}
			if this.CardTeams.m_changed{
				update_string+="CardTeams=?,"
				dCardTeams,err:=this.CardTeams.save()
				if err!=nil{
					log.Error("insert save CardTeam failed")
					return err,false,0,"",nil
				}
				db_args.Push(dCardTeams)
			}
			update_string = strings.TrimRight(update_string, ", ")
			update_string+=" WHERE PlayerId=?"
			db_args.Push(this.m_PlayerId)
			args=db_args.GetArgs()
			state = 2
		}
	}
	this.m_new = false
	this.Info.m_changed = false
	this.Cards.m_changed = false
	this.CardTeams.m_changed = false
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
	_, hasInfo := columns["Info"]
	if !hasInfo {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Info LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Info failed")
			return
		}
	}
	_, hasCard := columns["Cards"]
	if !hasCard {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN Cards LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN Cards failed")
			return
		}
	}
	_, hasCardTeam := columns["CardTeams"]
	if !hasCardTeam {
		_, err = this.m_dbc.Exec("ALTER TABLE Players ADD COLUMN CardTeams LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN CardTeams failed")
			return
		}
	}
	return
}
func (this *dbPlayerTable) prepare_preload_select_stmt() (err error) {
	this.m_preload_select_stmt,err=this.m_dbc.StmtPrepare("SELECT PlayerId,Info,Cards,CardTeams FROM Players")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbPlayerTable) prepare_save_insert_stmt()(err error){
	this.m_save_insert_stmt,err=this.m_dbc.StmtPrepare("INSERT INTO Players (PlayerId,Info,Cards,CardTeams) VALUES (?,?,?,?)")
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
	var dInfo []byte
	var dCards []byte
	var dCardTeams []byte
		this.m_preload_max_id = 0
	for r.Next() {
		err = r.Scan(&PlayerId,&dInfo,&dCards,&dCardTeams)
		if err != nil {
			log.Error("Scan")
			return
		}
		if PlayerId>this.m_preload_max_id{
			this.m_preload_max_id =PlayerId
		}
		row := new_dbPlayerRow(this,PlayerId)
		err = row.Info.load(dInfo)
		if err != nil {
			log.Error("Info %v", PlayerId)
			return
		}
		err = row.Cards.load(dCards)
		if err != nil {
			log.Error("Cards %v", PlayerId)
			return
		}
		err = row.CardTeams.load(dCardTeams)
		if err != nil {
			log.Error("CardTeams %v", PlayerId)
			return
		}
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
	Players *dbPlayerTable
}
func (this *DBC)init_tables()(err error){
	this.Players = new_dbPlayerTable(this)
	err = this.Players.Init()
	if err != nil {
		log.Error("init Players table failed")
		return
	}
	return
}
func (this *DBC)Preload()(err error){
	err = this.Players.Preload()
	if err != nil {
		log.Error("preload Players table failed")
		return
	}else{
		log.Info("preload Players table succeed !")
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
	err = this.Players.Save(quick)
	if err != nil {
		log.Error("save Players table failed")
		return
	}
	return
}
