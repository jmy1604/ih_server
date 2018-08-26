package main

import (
	"github.com/golang/protobuf/proto"
	"database/sql"
	"errors"
	"fmt"
	"ih_server/libs/log"
	"math/rand"
	"os"
	"ih_server/proto/gen_go/db_center"
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
		/*
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
		*/
		
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

type dbIdNumData struct{
	Id int32
	Num int32
}
func (this* dbIdNumData)from_pb(pb *db.IdNum){
	if pb == nil {
		return
	}
	this.Id = pb.GetId()
	this.Num = pb.GetNum()
	return
}
func (this* dbIdNumData)to_pb()(pb *db.IdNum){
	pb = &db.IdNum{}
	pb.Id = proto.Int32(this.Id)
	pb.Num = proto.Int32(this.Num)
	return
}
func (this* dbIdNumData)clone_to(d *dbIdNumData){
	d.Id = this.Id
	d.Num = this.Num
	return
}
type dbServerRewardRewardInfoData struct{
	RewardId int32
	Items []dbIdNumData
	Channel string
	EndUnix int32
	Content string
}
func (this* dbServerRewardRewardInfoData)from_pb(pb *db.ServerRewardRewardInfo){
	if pb == nil {
		this.Items = make([]dbIdNumData,0)
		return
	}
	this.RewardId = pb.GetRewardId()
	this.Items = make([]dbIdNumData,len(pb.GetItems()))
	for i, v := range pb.GetItems() {
		this.Items[i].from_pb(v)
	}
	this.Channel = pb.GetChannel()
	this.EndUnix = pb.GetEndUnix()
	this.Content = pb.GetContent()
	return
}
func (this* dbServerRewardRewardInfoData)to_pb()(pb *db.ServerRewardRewardInfo){
	pb = &db.ServerRewardRewardInfo{}
	pb.RewardId = proto.Int32(this.RewardId)
	pb.Items = make([]*db.IdNum, len(this.Items))
	for i, v := range this.Items {
		pb.Items[i]=v.to_pb()
	}
	pb.Channel = proto.String(this.Channel)
	pb.EndUnix = proto.Int32(this.EndUnix)
	pb.Content = proto.String(this.Content)
	return
}
func (this* dbServerRewardRewardInfoData)clone_to(d *dbServerRewardRewardInfoData){
	d.RewardId = this.RewardId
	d.Items = make([]dbIdNumData, len(this.Items))
	for _ii, _vv := range this.Items {
		_vv.clone_to(&d.Items[_ii])
	}
	d.Channel = this.Channel
	d.EndUnix = this.EndUnix
	d.Content = this.Content
	return
}

func (this *dbServerRewardRow)GetNextRewardId( )(r int32 ){
	this.m_lock.UnSafeRLock("dbServerRewardRow.GetdbServerRewardNextRewardIdColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_NextRewardId)
}
func (this *dbServerRewardRow)SetNextRewardId(v int32){
	this.m_lock.UnSafeLock("dbServerRewardRow.SetdbServerRewardNextRewardIdColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_NextRewardId=int32(v)
	this.m_NextRewardId_changed=true
	return
}
type dbServerRewardRewardInfoColumn struct{
	m_row *dbServerRewardRow
	m_data map[int32]*dbServerRewardRewardInfoData
	m_changed bool
}
func (this *dbServerRewardRewardInfoColumn)load(data []byte)(err error){
	if data == nil || len(data) == 0 {
		this.m_changed = false
		return nil
	}
	pb := &db.ServerRewardRewardInfoList{}
	err = proto.Unmarshal(data, pb)
	if err != nil {
		log.Error("Unmarshal %v", this.m_row.GetKeyId())
		return
	}
	for _, v := range pb.List {
		d := &dbServerRewardRewardInfoData{}
		d.from_pb(v)
		this.m_data[int32(d.RewardId)] = d
	}
	this.m_changed = false
	return
}
func (this *dbServerRewardRewardInfoColumn)save( )(data []byte,err error){
	pb := &db.ServerRewardRewardInfoList{}
	pb.List=make([]*db.ServerRewardRewardInfo,len(this.m_data))
	i:=0
	for _, v := range this.m_data {
		pb.List[i] = v.to_pb()
		i++
	}
	data, err = proto.Marshal(pb)
	if err != nil {
		log.Error("Marshal %v", this.m_row.GetKeyId())
		return
	}
	this.m_changed = false
	return
}
func (this *dbServerRewardRewardInfoColumn)HasIndex(id int32)(has bool){
	this.m_row.m_lock.UnSafeRLock("dbServerRewardRewardInfoColumn.HasIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	_, has = this.m_data[id]
	return
}
func (this *dbServerRewardRewardInfoColumn)GetAllIndex()(list []int32){
	this.m_row.m_lock.UnSafeRLock("dbServerRewardRewardInfoColumn.GetAllIndex")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]int32, len(this.m_data))
	i := 0
	for k, _ := range this.m_data {
		list[i] = k
		i++
	}
	return
}
func (this *dbServerRewardRewardInfoColumn)GetAll()(list []dbServerRewardRewardInfoData){
	this.m_row.m_lock.UnSafeRLock("dbServerRewardRewardInfoColumn.GetAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	list = make([]dbServerRewardRewardInfoData, len(this.m_data))
	i := 0
	for _, v := range this.m_data {
		v.clone_to(&list[i])
		i++
	}
	return
}
func (this *dbServerRewardRewardInfoColumn)Get(id int32)(v *dbServerRewardRewardInfoData){
	this.m_row.m_lock.UnSafeRLock("dbServerRewardRewardInfoColumn.Get")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return nil
	}
	v=&dbServerRewardRewardInfoData{}
	d.clone_to(v)
	return
}
func (this *dbServerRewardRewardInfoColumn)Set(v dbServerRewardRewardInfoData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbServerRewardRewardInfoColumn.Set")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[int32(v.RewardId)]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetKeyId(), v.RewardId)
		return false
	}
	v.clone_to(d)
	this.m_changed = true
	return true
}
func (this *dbServerRewardRewardInfoColumn)Add(v *dbServerRewardRewardInfoData)(ok bool){
	this.m_row.m_lock.UnSafeLock("dbServerRewardRewardInfoColumn.Add")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[int32(v.RewardId)]
	if has {
		log.Error("already added %v %v",this.m_row.GetKeyId(), v.RewardId)
		return false
	}
	d:=&dbServerRewardRewardInfoData{}
	v.clone_to(d)
	this.m_data[int32(v.RewardId)]=d
	this.m_changed = true
	return true
}
func (this *dbServerRewardRewardInfoColumn)Remove(id int32){
	this.m_row.m_lock.UnSafeLock("dbServerRewardRewardInfoColumn.Remove")
	defer this.m_row.m_lock.UnSafeUnlock()
	_, has := this.m_data[id]
	if has {
		delete(this.m_data,id)
	}
	this.m_changed = true
	return
}
func (this *dbServerRewardRewardInfoColumn)Clear(){
	this.m_row.m_lock.UnSafeLock("dbServerRewardRewardInfoColumn.Clear")
	defer this.m_row.m_lock.UnSafeUnlock()
	this.m_data=make(map[int32]*dbServerRewardRewardInfoData)
	this.m_changed = true
	return
}
func (this *dbServerRewardRewardInfoColumn)NumAll()(n int32){
	this.m_row.m_lock.UnSafeRLock("dbServerRewardRewardInfoColumn.NumAll")
	defer this.m_row.m_lock.UnSafeRUnlock()
	return int32(len(this.m_data))
}
func (this *dbServerRewardRewardInfoColumn)GetItems(id int32)(v []dbIdNumData,has bool ){
	this.m_row.m_lock.UnSafeRLock("dbServerRewardRewardInfoColumn.GetItems")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = make([]dbIdNumData, len(d.Items))
	for _ii, _vv := range d.Items {
		_vv.clone_to(&v[_ii])
	}
	return v,true
}
func (this *dbServerRewardRewardInfoColumn)SetItems(id int32,v []dbIdNumData)(has bool){
	this.m_row.m_lock.UnSafeLock("dbServerRewardRewardInfoColumn.SetItems")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetKeyId(), id)
		return
	}
	d.Items = make([]dbIdNumData, len(v))
	for _ii, _vv := range v {
		_vv.clone_to(&d.Items[_ii])
	}
	this.m_changed = true
	return true
}
func (this *dbServerRewardRewardInfoColumn)GetChannel(id int32)(v string ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbServerRewardRewardInfoColumn.GetChannel")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Channel
	return v,true
}
func (this *dbServerRewardRewardInfoColumn)SetChannel(id int32,v string)(has bool){
	this.m_row.m_lock.UnSafeLock("dbServerRewardRewardInfoColumn.SetChannel")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetKeyId(), id)
		return
	}
	d.Channel = v
	this.m_changed = true
	return true
}
func (this *dbServerRewardRewardInfoColumn)GetEndUnix(id int32)(v int32 ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbServerRewardRewardInfoColumn.GetEndUnix")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.EndUnix
	return v,true
}
func (this *dbServerRewardRewardInfoColumn)SetEndUnix(id int32,v int32)(has bool){
	this.m_row.m_lock.UnSafeLock("dbServerRewardRewardInfoColumn.SetEndUnix")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetKeyId(), id)
		return
	}
	d.EndUnix = v
	this.m_changed = true
	return true
}
func (this *dbServerRewardRewardInfoColumn)GetContent(id int32)(v string ,has bool){
	this.m_row.m_lock.UnSafeRLock("dbServerRewardRewardInfoColumn.GetContent")
	defer this.m_row.m_lock.UnSafeRUnlock()
	d := this.m_data[id]
	if d==nil{
		return
	}
	v = d.Content
	return v,true
}
func (this *dbServerRewardRewardInfoColumn)SetContent(id int32,v string)(has bool){
	this.m_row.m_lock.UnSafeLock("dbServerRewardRewardInfoColumn.SetContent")
	defer this.m_row.m_lock.UnSafeUnlock()
	d := this.m_data[id]
	if d==nil{
		log.Error("not exist %v %v",this.m_row.GetKeyId(), id)
		return
	}
	d.Content = v
	this.m_changed = true
	return true
}
type dbServerRewardRow struct {
	m_table *dbServerRewardTable
	m_lock       *RWMutex
	m_loaded  bool
	m_new     bool
	m_remove  bool
	m_touch      int32
	m_releasable bool
	m_valid   bool
	m_KeyId        int32
	m_NextRewardId_changed bool
	m_NextRewardId int32
	RewardInfos dbServerRewardRewardInfoColumn
}
func new_dbServerRewardRow(table *dbServerRewardTable, KeyId int32) (r *dbServerRewardRow) {
	this := &dbServerRewardRow{}
	this.m_table = table
	this.m_KeyId = KeyId
	this.m_lock = NewRWMutex()
	this.m_NextRewardId_changed=true
	this.RewardInfos.m_row=this
	this.RewardInfos.m_data=make(map[int32]*dbServerRewardRewardInfoData)
	return this
}
func (this *dbServerRewardRow) GetKeyId() (r int32) {
	return this.m_KeyId
}
func (this *dbServerRewardRow) save_data(release bool) (err error, released bool, state int32, update_string string, args []interface{}) {
	this.m_lock.UnSafeLock("dbServerRewardRow.save_data")
	defer this.m_lock.UnSafeUnlock()
	if this.m_new {
		db_args:=new_db_args(3)
		db_args.Push(this.m_KeyId)
		db_args.Push(this.m_NextRewardId)
		dRewardInfos,db_err:=this.RewardInfos.save()
		if db_err!=nil{
			log.Error("insert save RewardInfo failed")
			return db_err,false,0,"",nil
		}
		db_args.Push(dRewardInfos)
		args=db_args.GetArgs()
		state = 1
	} else {
		if this.m_NextRewardId_changed||this.RewardInfos.m_changed{
			update_string = "UPDATE ServerRewards SET "
			db_args:=new_db_args(3)
			if this.m_NextRewardId_changed{
				update_string+="NextRewardId=?,"
				db_args.Push(this.m_NextRewardId)
			}
			if this.RewardInfos.m_changed{
				update_string+="RewardInfos=?,"
				dRewardInfos,err:=this.RewardInfos.save()
				if err!=nil{
					log.Error("insert save RewardInfo failed")
					return err,false,0,"",nil
				}
				db_args.Push(dRewardInfos)
			}
			update_string = strings.TrimRight(update_string, ", ")
			update_string+=" WHERE KeyId=?"
			db_args.Push(this.m_KeyId)
			args=db_args.GetArgs()
			state = 2
		}
	}
	this.m_new = false
	this.m_NextRewardId_changed = false
	this.RewardInfos.m_changed = false
	if release && this.m_loaded {
		atomic.AddInt32(&this.m_table.m_gc_n, -1)
		this.m_loaded = false
		released = true
	}
	return nil,released,state,update_string,args
}
func (this *dbServerRewardRow) Save(release bool) (err error, d bool, released bool) {
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
			log.Error("INSERT ServerRewards exec failed %v ", this.m_KeyId)
			return err, false, released
		}
		d = true
	} else if state == 2 {
		_, err = this.m_table.m_dbc.Exec(update_string, args...)
		if err != nil {
			log.Error("UPDATE ServerRewards exec failed %v", this.m_KeyId)
			return err, false, released
		}
		d = true
	}
	return nil, d, released
}
func (this *dbServerRewardRow) Touch(releasable bool) {
	this.m_touch = int32(time.Now().Unix())
	this.m_releasable = releasable
}
type dbServerRewardRowSort struct {
	rows []*dbServerRewardRow
}
func (this *dbServerRewardRowSort) Len() (length int) {
	return len(this.rows)
}
func (this *dbServerRewardRowSort) Less(i int, j int) (less bool) {
	return this.rows[i].m_touch < this.rows[j].m_touch
}
func (this *dbServerRewardRowSort) Swap(i int, j int) {
	temp := this.rows[i]
	this.rows[i] = this.rows[j]
	this.rows[j] = temp
}
type dbServerRewardTable struct{
	m_dbc *DBC
	m_lock *RWMutex
	m_rows map[int32]*dbServerRewardRow
	m_new_rows map[int32]*dbServerRewardRow
	m_removed_rows map[int32]*dbServerRewardRow
	m_gc_n int32
	m_gcing int32
	m_pool_size int32
	m_preload_select_stmt *sql.Stmt
	m_preload_max_id int32
	m_save_insert_stmt *sql.Stmt
	m_delete_stmt *sql.Stmt
}
func new_dbServerRewardTable(dbc *DBC) (this *dbServerRewardTable) {
	this = &dbServerRewardTable{}
	this.m_dbc = dbc
	this.m_lock = NewRWMutex()
	this.m_rows = make(map[int32]*dbServerRewardRow)
	this.m_new_rows = make(map[int32]*dbServerRewardRow)
	this.m_removed_rows = make(map[int32]*dbServerRewardRow)
	return this
}
func (this *dbServerRewardTable) check_create_table() (err error) {
	_, err = this.m_dbc.Exec("CREATE TABLE IF NOT EXISTS ServerRewards(KeyId int(11),PRIMARY KEY (KeyId))ENGINE=InnoDB ROW_FORMAT=DYNAMIC")
	if err != nil {
		log.Error("CREATE TABLE IF NOT EXISTS ServerRewards failed")
		return
	}
	rows, err := this.m_dbc.Query("SELECT COLUMN_NAME,ORDINAL_POSITION FROM information_schema.`COLUMNS` WHERE TABLE_SCHEMA=? AND TABLE_NAME='ServerRewards'", this.m_dbc.m_db_name)
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
	_, hasNextRewardId := columns["NextRewardId"]
	if !hasNextRewardId {
		_, err = this.m_dbc.Exec("ALTER TABLE ServerRewards ADD COLUMN NextRewardId int(11)")
		if err != nil {
			log.Error("ADD COLUMN NextRewardId failed")
			return
		}
	}
	_, hasRewardInfo := columns["RewardInfos"]
	if !hasRewardInfo {
		_, err = this.m_dbc.Exec("ALTER TABLE ServerRewards ADD COLUMN RewardInfos LONGBLOB")
		if err != nil {
			log.Error("ADD COLUMN RewardInfos failed")
			return
		}
	}
	return
}
func (this *dbServerRewardTable) prepare_preload_select_stmt() (err error) {
	this.m_preload_select_stmt,err=this.m_dbc.StmtPrepare("SELECT KeyId,NextRewardId,RewardInfos FROM ServerRewards")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbServerRewardTable) prepare_save_insert_stmt()(err error){
	this.m_save_insert_stmt,err=this.m_dbc.StmtPrepare("INSERT INTO ServerRewards (KeyId,NextRewardId,RewardInfos) VALUES (?,?,?)")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbServerRewardTable) prepare_delete_stmt() (err error) {
	this.m_delete_stmt,err=this.m_dbc.StmtPrepare("DELETE FROM ServerRewards WHERE KeyId=?")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbServerRewardTable) Init() (err error) {
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
func (this *dbServerRewardTable) Preload() (err error) {
	r, err := this.m_dbc.StmtQuery(this.m_preload_select_stmt)
	if err != nil {
		log.Error("SELECT")
		return
	}
	var KeyId int32
	var dNextRewardId int32
	var dRewardInfos []byte
		this.m_preload_max_id = 0
	for r.Next() {
		err = r.Scan(&KeyId,&dNextRewardId,&dRewardInfos)
		if err != nil {
			log.Error("Scan err[%v]", err.Error())
			return
		}
		if KeyId>this.m_preload_max_id{
			this.m_preload_max_id =KeyId
		}
		row := new_dbServerRewardRow(this,KeyId)
		row.m_NextRewardId=dNextRewardId
		err = row.RewardInfos.load(dRewardInfos)
		if err != nil {
			log.Error("RewardInfos %v", KeyId)
			return
		}
		row.m_NextRewardId_changed=false
		row.m_valid = true
		this.m_rows[KeyId]=row
	}
	return
}
func (this *dbServerRewardTable) GetPreloadedMaxId() (max_id int32) {
	return this.m_preload_max_id
}
func (this *dbServerRewardTable) fetch_rows(rows map[int32]*dbServerRewardRow) (r map[int32]*dbServerRewardRow) {
	this.m_lock.UnSafeLock("dbServerRewardTable.fetch_rows")
	defer this.m_lock.UnSafeUnlock()
	r = make(map[int32]*dbServerRewardRow)
	for i, v := range rows {
		r[i] = v
	}
	return r
}
func (this *dbServerRewardTable) fetch_new_rows() (new_rows map[int32]*dbServerRewardRow) {
	this.m_lock.UnSafeLock("dbServerRewardTable.fetch_new_rows")
	defer this.m_lock.UnSafeUnlock()
	new_rows = make(map[int32]*dbServerRewardRow)
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
func (this *dbServerRewardTable) save_rows(rows map[int32]*dbServerRewardRow, quick bool) {
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
func (this *dbServerRewardTable) Save(quick bool) (err error){
	removed_rows := this.fetch_rows(this.m_removed_rows)
	for _, v := range removed_rows {
		_, err := this.m_dbc.StmtExec(this.m_delete_stmt, v.GetKeyId())
		if err != nil {
			log.Error("exec delete stmt failed %v", err)
		}
		v.m_valid = false
		if !quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
	this.m_removed_rows = make(map[int32]*dbServerRewardRow)
	rows := this.fetch_rows(this.m_rows)
	this.save_rows(rows, quick)
	new_rows := this.fetch_new_rows()
	this.save_rows(new_rows, quick)
	return
}
func (this *dbServerRewardTable) AddRow(KeyId int32) (row *dbServerRewardRow) {
	this.m_lock.UnSafeLock("dbServerRewardTable.AddRow")
	defer this.m_lock.UnSafeUnlock()
	row = new_dbServerRewardRow(this,KeyId)
	row.m_new = true
	row.m_loaded = true
	row.m_valid = true
	_, has := this.m_new_rows[KeyId]
	if has{
		log.Error("已经存在 %v", KeyId)
		return nil
	}
	this.m_new_rows[KeyId] = row
	atomic.AddInt32(&this.m_gc_n,1)
	return row
}
func (this *dbServerRewardTable) RemoveRow(KeyId int32) {
	this.m_lock.UnSafeLock("dbServerRewardTable.RemoveRow")
	defer this.m_lock.UnSafeUnlock()
	row := this.m_rows[KeyId]
	if row != nil {
		row.m_remove = true
		delete(this.m_rows, KeyId)
		rm_row := this.m_removed_rows[KeyId]
		if rm_row != nil {
			log.Error("rows and removed rows both has %v", KeyId)
		}
		this.m_removed_rows[KeyId] = row
		_, has_new := this.m_new_rows[KeyId]
		if has_new {
			delete(this.m_new_rows, KeyId)
			log.Error("rows and new_rows both has %v", KeyId)
		}
	} else {
		row = this.m_removed_rows[KeyId]
		if row == nil {
			_, has_new := this.m_new_rows[KeyId]
			if has_new {
				delete(this.m_new_rows, KeyId)
			} else {
				log.Error("row not exist %v", KeyId)
			}
		} else {
			log.Error("already removed %v", KeyId)
			_, has_new := this.m_new_rows[KeyId]
			if has_new {
				delete(this.m_new_rows, KeyId)
				log.Error("removed rows and new_rows both has %v", KeyId)
			}
		}
	}
}
func (this *dbServerRewardTable) GetRow(KeyId int32) (row *dbServerRewardRow) {
	this.m_lock.UnSafeRLock("dbServerRewardTable.GetRow")
	defer this.m_lock.UnSafeRUnlock()
	row = this.m_rows[KeyId]
	if row == nil {
		row = this.m_new_rows[KeyId]
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
	ServerRewards *dbServerRewardTable
}
func (this *DBC)init_tables()(err error){
	this.ServerRewards = new_dbServerRewardTable(this)
	err = this.ServerRewards.Init()
	if err != nil {
		log.Error("init ServerRewards table failed")
		return
	}
	return
}
func (this *DBC)Preload()(err error){
	err = this.ServerRewards.Preload()
	if err != nil {
		log.Error("preload ServerRewards table failed")
		return
	}else{
		log.Info("preload ServerRewards table succeed !")
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
	err = this.ServerRewards.Save(quick)
	if err != nil {
		log.Error("save ServerRewards table failed")
		return
	}
	return
}
