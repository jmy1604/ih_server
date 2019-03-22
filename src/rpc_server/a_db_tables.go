package main

import (
	_ "github.com/go-sql-driver/mysql"
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"ih_server/libs/log"
	"math/rand"
	"os"
	"os/exec"
	_ "ih_server/proto/gen_go/db_rpc"
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
		log.Trace("db存数据花费时长: %v", time.Now().Sub(begin).Nanoseconds())

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


func (this *dbApplePayRow)GetBundleId( )(r string ){
	this.m_lock.UnSafeRLock("dbApplePayRow.GetdbApplePayBundleIdColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_BundleId)
}
func (this *dbApplePayRow)SetBundleId(v string){
	this.m_lock.UnSafeLock("dbApplePayRow.SetdbApplePayBundleIdColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_BundleId=string(v)
	this.m_BundleId_changed=true
	return
}
func (this *dbApplePayRow)GetAccount( )(r string ){
	this.m_lock.UnSafeRLock("dbApplePayRow.GetdbApplePayAccountColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_Account)
}
func (this *dbApplePayRow)SetAccount(v string){
	this.m_lock.UnSafeLock("dbApplePayRow.SetdbApplePayAccountColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Account=string(v)
	this.m_Account_changed=true
	return
}
func (this *dbApplePayRow)GetPlayerId( )(r int32 ){
	this.m_lock.UnSafeRLock("dbApplePayRow.GetdbApplePayPlayerIdColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_PlayerId)
}
func (this *dbApplePayRow)SetPlayerId(v int32){
	this.m_lock.UnSafeLock("dbApplePayRow.SetdbApplePayPlayerIdColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_PlayerId=int32(v)
	this.m_PlayerId_changed=true
	return
}
func (this *dbApplePayRow)GetPayTime( )(r int32 ){
	this.m_lock.UnSafeRLock("dbApplePayRow.GetdbApplePayPayTimeColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_PayTime)
}
func (this *dbApplePayRow)SetPayTime(v int32){
	this.m_lock.UnSafeLock("dbApplePayRow.SetdbApplePayPayTimeColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_PayTime=int32(v)
	this.m_PayTime_changed=true
	return
}
func (this *dbApplePayRow)GetPayTimeStr( )(r string ){
	this.m_lock.UnSafeRLock("dbApplePayRow.GetdbApplePayPayTimeStrColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_PayTimeStr)
}
func (this *dbApplePayRow)SetPayTimeStr(v string){
	this.m_lock.UnSafeLock("dbApplePayRow.SetdbApplePayPayTimeStrColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_PayTimeStr=string(v)
	this.m_PayTimeStr_changed=true
	return
}
type dbApplePayRow struct {
	m_table *dbApplePayTable
	m_lock       *RWMutex
	m_loaded  bool
	m_new     bool
	m_remove  bool
	m_touch      int32
	m_releasable bool
	m_valid   bool
	m_OrderId        string
	m_BundleId_changed bool
	m_BundleId string
	m_Account_changed bool
	m_Account string
	m_PlayerId_changed bool
	m_PlayerId int32
	m_PayTime_changed bool
	m_PayTime int32
	m_PayTimeStr_changed bool
	m_PayTimeStr string
}
func new_dbApplePayRow(table *dbApplePayTable, OrderId string) (r *dbApplePayRow) {
	this := &dbApplePayRow{}
	this.m_table = table
	this.m_OrderId = OrderId
	this.m_lock = NewRWMutex()
	this.m_BundleId_changed=true
	this.m_Account_changed=true
	this.m_PlayerId_changed=true
	this.m_PayTime_changed=true
	this.m_PayTimeStr_changed=true
	return this
}
func (this *dbApplePayRow) GetOrderId() (r string) {
	return this.m_OrderId
}
func (this *dbApplePayRow) save_data(release bool) (err error, released bool, state int32, update_string string, args []interface{}) {
	this.m_lock.UnSafeLock("dbApplePayRow.save_data")
	defer this.m_lock.UnSafeUnlock()
	if this.m_new {
		db_args:=new_db_args(6)
		db_args.Push(this.m_OrderId)
		db_args.Push(this.m_BundleId)
		db_args.Push(this.m_Account)
		db_args.Push(this.m_PlayerId)
		db_args.Push(this.m_PayTime)
		db_args.Push(this.m_PayTimeStr)
		args=db_args.GetArgs()
		state = 1
	} else {
		if this.m_BundleId_changed||this.m_Account_changed||this.m_PlayerId_changed||this.m_PayTime_changed||this.m_PayTimeStr_changed{
			update_string = "UPDATE ApplePays SET "
			db_args:=new_db_args(6)
			if this.m_BundleId_changed{
				update_string+="BundleId=?,"
				db_args.Push(this.m_BundleId)
			}
			if this.m_Account_changed{
				update_string+="Account=?,"
				db_args.Push(this.m_Account)
			}
			if this.m_PlayerId_changed{
				update_string+="PlayerId=?,"
				db_args.Push(this.m_PlayerId)
			}
			if this.m_PayTime_changed{
				update_string+="PayTime=?,"
				db_args.Push(this.m_PayTime)
			}
			if this.m_PayTimeStr_changed{
				update_string+="PayTimeStr=?,"
				db_args.Push(this.m_PayTimeStr)
			}
			update_string = strings.TrimRight(update_string, ", ")
			update_string+=" WHERE OrderId=?"
			db_args.Push(this.m_OrderId)
			args=db_args.GetArgs()
			state = 2
		}
	}
	this.m_new = false
	this.m_BundleId_changed = false
	this.m_Account_changed = false
	this.m_PlayerId_changed = false
	this.m_PayTime_changed = false
	this.m_PayTimeStr_changed = false
	if release && this.m_loaded {
		atomic.AddInt32(&this.m_table.m_gc_n, -1)
		this.m_loaded = false
		released = true
	}
	return nil,released,state,update_string,args
}
func (this *dbApplePayRow) Save(release bool) (err error, d bool, released bool) {
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
			log.Error("INSERT ApplePays exec failed %v ", this.m_OrderId)
			return err, false, released
		}
		d = true
	} else if state == 2 {
		_, err = this.m_table.m_dbc.Exec(update_string, args...)
		if err != nil {
			log.Error("UPDATE ApplePays exec failed %v", this.m_OrderId)
			return err, false, released
		}
		d = true
	}
	return nil, d, released
}
func (this *dbApplePayRow) Touch(releasable bool) {
	this.m_touch = int32(time.Now().Unix())
	this.m_releasable = releasable
}
type dbApplePayRowSort struct {
	rows []*dbApplePayRow
}
func (this *dbApplePayRowSort) Len() (length int) {
	return len(this.rows)
}
func (this *dbApplePayRowSort) Less(i int, j int) (less bool) {
	return this.rows[i].m_touch < this.rows[j].m_touch
}
func (this *dbApplePayRowSort) Swap(i int, j int) {
	temp := this.rows[i]
	this.rows[i] = this.rows[j]
	this.rows[j] = temp
}
type dbApplePayTable struct{
	m_dbc *DBC
	m_lock *RWMutex
	m_rows map[string]*dbApplePayRow
	m_new_rows map[string]*dbApplePayRow
	m_removed_rows map[string]*dbApplePayRow
	m_gc_n int32
	m_gcing int32
	m_pool_size int32
	m_preload_select_stmt *sql.Stmt
	m_preload_max_id int32
	m_save_insert_stmt *sql.Stmt
	m_delete_stmt *sql.Stmt
}
func new_dbApplePayTable(dbc *DBC) (this *dbApplePayTable) {
	this = &dbApplePayTable{}
	this.m_dbc = dbc
	this.m_lock = NewRWMutex()
	this.m_rows = make(map[string]*dbApplePayRow)
	this.m_new_rows = make(map[string]*dbApplePayRow)
	this.m_removed_rows = make(map[string]*dbApplePayRow)
	return this
}
func (this *dbApplePayTable) check_create_table() (err error) {
	_, err = this.m_dbc.Exec("CREATE TABLE IF NOT EXISTS ApplePays(OrderId varchar(32),PRIMARY KEY (OrderId))ENGINE=InnoDB ROW_FORMAT=DYNAMIC")
	if err != nil {
		log.Error("CREATE TABLE IF NOT EXISTS ApplePays failed")
		return
	}
	rows, err := this.m_dbc.Query("SELECT COLUMN_NAME,ORDINAL_POSITION FROM information_schema.`COLUMNS` WHERE TABLE_SCHEMA=? AND TABLE_NAME='ApplePays'", this.m_dbc.m_db_name)
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
	_, hasBundleId := columns["BundleId"]
	if !hasBundleId {
		_, err = this.m_dbc.Exec("ALTER TABLE ApplePays ADD COLUMN BundleId varchar(256)")
		if err != nil {
			log.Error("ADD COLUMN BundleId failed")
			return
		}
	}
	_, hasAccount := columns["Account"]
	if !hasAccount {
		_, err = this.m_dbc.Exec("ALTER TABLE ApplePays ADD COLUMN Account varchar(256)")
		if err != nil {
			log.Error("ADD COLUMN Account failed")
			return
		}
	}
	_, hasPlayerId := columns["PlayerId"]
	if !hasPlayerId {
		_, err = this.m_dbc.Exec("ALTER TABLE ApplePays ADD COLUMN PlayerId int(11)")
		if err != nil {
			log.Error("ADD COLUMN PlayerId failed")
			return
		}
	}
	_, hasPayTime := columns["PayTime"]
	if !hasPayTime {
		_, err = this.m_dbc.Exec("ALTER TABLE ApplePays ADD COLUMN PayTime int(11)")
		if err != nil {
			log.Error("ADD COLUMN PayTime failed")
			return
		}
	}
	_, hasPayTimeStr := columns["PayTimeStr"]
	if !hasPayTimeStr {
		_, err = this.m_dbc.Exec("ALTER TABLE ApplePays ADD COLUMN PayTimeStr varchar(256)")
		if err != nil {
			log.Error("ADD COLUMN PayTimeStr failed")
			return
		}
	}
	return
}
func (this *dbApplePayTable) prepare_preload_select_stmt() (err error) {
	this.m_preload_select_stmt,err=this.m_dbc.StmtPrepare("SELECT OrderId,BundleId,Account,PlayerId,PayTime,PayTimeStr FROM ApplePays")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbApplePayTable) prepare_save_insert_stmt()(err error){
	this.m_save_insert_stmt,err=this.m_dbc.StmtPrepare("INSERT INTO ApplePays (OrderId,BundleId,Account,PlayerId,PayTime,PayTimeStr) VALUES (?,?,?,?,?,?)")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbApplePayTable) prepare_delete_stmt() (err error) {
	this.m_delete_stmt,err=this.m_dbc.StmtPrepare("DELETE FROM ApplePays WHERE OrderId=?")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbApplePayTable) Init() (err error) {
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
func (this *dbApplePayTable) Preload() (err error) {
	r, err := this.m_dbc.StmtQuery(this.m_preload_select_stmt)
	if err != nil {
		log.Error("SELECT")
		return
	}
	var OrderId string
	var dBundleId string
	var dAccount string
	var dPlayerId int32
	var dPayTime int32
	var dPayTimeStr string
	for r.Next() {
		err = r.Scan(&OrderId,&dBundleId,&dAccount,&dPlayerId,&dPayTime,&dPayTimeStr)
		if err != nil {
			log.Error("Scan err[%v]", err.Error())
			return
		}
		row := new_dbApplePayRow(this,OrderId)
		row.m_BundleId=dBundleId
		row.m_Account=dAccount
		row.m_PlayerId=dPlayerId
		row.m_PayTime=dPayTime
		row.m_PayTimeStr=dPayTimeStr
		row.m_BundleId_changed=false
		row.m_Account_changed=false
		row.m_PlayerId_changed=false
		row.m_PayTime_changed=false
		row.m_PayTimeStr_changed=false
		row.m_valid = true
		this.m_rows[OrderId]=row
	}
	return
}
func (this *dbApplePayTable) GetPreloadedMaxId() (max_id int32) {
	return this.m_preload_max_id
}
func (this *dbApplePayTable) fetch_rows(rows map[string]*dbApplePayRow) (r map[string]*dbApplePayRow) {
	this.m_lock.UnSafeLock("dbApplePayTable.fetch_rows")
	defer this.m_lock.UnSafeUnlock()
	r = make(map[string]*dbApplePayRow)
	for i, v := range rows {
		r[i] = v
	}
	return r
}
func (this *dbApplePayTable) fetch_new_rows() (new_rows map[string]*dbApplePayRow) {
	this.m_lock.UnSafeLock("dbApplePayTable.fetch_new_rows")
	defer this.m_lock.UnSafeUnlock()
	new_rows = make(map[string]*dbApplePayRow)
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
func (this *dbApplePayTable) save_rows(rows map[string]*dbApplePayRow, quick bool) {
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
func (this *dbApplePayTable) Save(quick bool) (err error){
	removed_rows := this.fetch_rows(this.m_removed_rows)
	for _, v := range removed_rows {
		_, err := this.m_dbc.StmtExec(this.m_delete_stmt, v.GetOrderId())
		if err != nil {
			log.Error("exec delete stmt failed %v", err)
		}
		v.m_valid = false
		if !quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
	this.m_removed_rows = make(map[string]*dbApplePayRow)
	rows := this.fetch_rows(this.m_rows)
	this.save_rows(rows, quick)
	new_rows := this.fetch_new_rows()
	this.save_rows(new_rows, quick)
	return
}
func (this *dbApplePayTable) AddRow(OrderId string) (row *dbApplePayRow) {
	this.m_lock.UnSafeLock("dbApplePayTable.AddRow")
	defer this.m_lock.UnSafeUnlock()
	row = new_dbApplePayRow(this,OrderId)
	row.m_new = true
	row.m_loaded = true
	row.m_valid = true
	_, has := this.m_new_rows[OrderId]
	if has{
		log.Error("已经存在 %v", OrderId)
		return nil
	}
	this.m_new_rows[OrderId] = row
	atomic.AddInt32(&this.m_gc_n,1)
	return row
}
func (this *dbApplePayTable) RemoveRow(OrderId string) {
	this.m_lock.UnSafeLock("dbApplePayTable.RemoveRow")
	defer this.m_lock.UnSafeUnlock()
	row := this.m_rows[OrderId]
	if row != nil {
		row.m_remove = true
		delete(this.m_rows, OrderId)
		rm_row := this.m_removed_rows[OrderId]
		if rm_row != nil {
			log.Error("rows and removed rows both has %v", OrderId)
		}
		this.m_removed_rows[OrderId] = row
		_, has_new := this.m_new_rows[OrderId]
		if has_new {
			delete(this.m_new_rows, OrderId)
			log.Error("rows and new_rows both has %v", OrderId)
		}
	} else {
		row = this.m_removed_rows[OrderId]
		if row == nil {
			_, has_new := this.m_new_rows[OrderId]
			if has_new {
				delete(this.m_new_rows, OrderId)
			} else {
				log.Error("row not exist %v", OrderId)
			}
		} else {
			log.Error("already removed %v", OrderId)
			_, has_new := this.m_new_rows[OrderId]
			if has_new {
				delete(this.m_new_rows, OrderId)
				log.Error("removed rows and new_rows both has %v", OrderId)
			}
		}
	}
}
func (this *dbApplePayTable) GetRow(OrderId string) (row *dbApplePayRow) {
	this.m_lock.UnSafeRLock("dbApplePayTable.GetRow")
	defer this.m_lock.UnSafeRUnlock()
	row = this.m_rows[OrderId]
	if row == nil {
		row = this.m_new_rows[OrderId]
	}
	return row
}
func (this *dbGooglePayRow)GetBundleId( )(r string ){
	this.m_lock.UnSafeRLock("dbGooglePayRow.GetdbGooglePayBundleIdColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_BundleId)
}
func (this *dbGooglePayRow)SetBundleId(v string){
	this.m_lock.UnSafeLock("dbGooglePayRow.SetdbGooglePayBundleIdColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_BundleId=string(v)
	this.m_BundleId_changed=true
	return
}
func (this *dbGooglePayRow)GetAccount( )(r string ){
	this.m_lock.UnSafeRLock("dbGooglePayRow.GetdbGooglePayAccountColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_Account)
}
func (this *dbGooglePayRow)SetAccount(v string){
	this.m_lock.UnSafeLock("dbGooglePayRow.SetdbGooglePayAccountColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Account=string(v)
	this.m_Account_changed=true
	return
}
func (this *dbGooglePayRow)GetPlayerId( )(r int32 ){
	this.m_lock.UnSafeRLock("dbGooglePayRow.GetdbGooglePayPlayerIdColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_PlayerId)
}
func (this *dbGooglePayRow)SetPlayerId(v int32){
	this.m_lock.UnSafeLock("dbGooglePayRow.SetdbGooglePayPlayerIdColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_PlayerId=int32(v)
	this.m_PlayerId_changed=true
	return
}
func (this *dbGooglePayRow)GetPayTime( )(r int32 ){
	this.m_lock.UnSafeRLock("dbGooglePayRow.GetdbGooglePayPayTimeColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_PayTime)
}
func (this *dbGooglePayRow)SetPayTime(v int32){
	this.m_lock.UnSafeLock("dbGooglePayRow.SetdbGooglePayPayTimeColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_PayTime=int32(v)
	this.m_PayTime_changed=true
	return
}
func (this *dbGooglePayRow)GetPayTimeStr( )(r string ){
	this.m_lock.UnSafeRLock("dbGooglePayRow.GetdbGooglePayPayTimeStrColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_PayTimeStr)
}
func (this *dbGooglePayRow)SetPayTimeStr(v string){
	this.m_lock.UnSafeLock("dbGooglePayRow.SetdbGooglePayPayTimeStrColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_PayTimeStr=string(v)
	this.m_PayTimeStr_changed=true
	return
}
type dbGooglePayRow struct {
	m_table *dbGooglePayTable
	m_lock       *RWMutex
	m_loaded  bool
	m_new     bool
	m_remove  bool
	m_touch      int32
	m_releasable bool
	m_valid   bool
	m_OrderId        string
	m_BundleId_changed bool
	m_BundleId string
	m_Account_changed bool
	m_Account string
	m_PlayerId_changed bool
	m_PlayerId int32
	m_PayTime_changed bool
	m_PayTime int32
	m_PayTimeStr_changed bool
	m_PayTimeStr string
}
func new_dbGooglePayRow(table *dbGooglePayTable, OrderId string) (r *dbGooglePayRow) {
	this := &dbGooglePayRow{}
	this.m_table = table
	this.m_OrderId = OrderId
	this.m_lock = NewRWMutex()
	this.m_BundleId_changed=true
	this.m_Account_changed=true
	this.m_PlayerId_changed=true
	this.m_PayTime_changed=true
	this.m_PayTimeStr_changed=true
	return this
}
func (this *dbGooglePayRow) GetOrderId() (r string) {
	return this.m_OrderId
}
func (this *dbGooglePayRow) save_data(release bool) (err error, released bool, state int32, update_string string, args []interface{}) {
	this.m_lock.UnSafeLock("dbGooglePayRow.save_data")
	defer this.m_lock.UnSafeUnlock()
	if this.m_new {
		db_args:=new_db_args(6)
		db_args.Push(this.m_OrderId)
		db_args.Push(this.m_BundleId)
		db_args.Push(this.m_Account)
		db_args.Push(this.m_PlayerId)
		db_args.Push(this.m_PayTime)
		db_args.Push(this.m_PayTimeStr)
		args=db_args.GetArgs()
		state = 1
	} else {
		if this.m_BundleId_changed||this.m_Account_changed||this.m_PlayerId_changed||this.m_PayTime_changed||this.m_PayTimeStr_changed{
			update_string = "UPDATE GooglePays SET "
			db_args:=new_db_args(6)
			if this.m_BundleId_changed{
				update_string+="BundleId=?,"
				db_args.Push(this.m_BundleId)
			}
			if this.m_Account_changed{
				update_string+="Account=?,"
				db_args.Push(this.m_Account)
			}
			if this.m_PlayerId_changed{
				update_string+="PlayerId=?,"
				db_args.Push(this.m_PlayerId)
			}
			if this.m_PayTime_changed{
				update_string+="PayTime=?,"
				db_args.Push(this.m_PayTime)
			}
			if this.m_PayTimeStr_changed{
				update_string+="PayTimeStr=?,"
				db_args.Push(this.m_PayTimeStr)
			}
			update_string = strings.TrimRight(update_string, ", ")
			update_string+=" WHERE OrderId=?"
			db_args.Push(this.m_OrderId)
			args=db_args.GetArgs()
			state = 2
		}
	}
	this.m_new = false
	this.m_BundleId_changed = false
	this.m_Account_changed = false
	this.m_PlayerId_changed = false
	this.m_PayTime_changed = false
	this.m_PayTimeStr_changed = false
	if release && this.m_loaded {
		atomic.AddInt32(&this.m_table.m_gc_n, -1)
		this.m_loaded = false
		released = true
	}
	return nil,released,state,update_string,args
}
func (this *dbGooglePayRow) Save(release bool) (err error, d bool, released bool) {
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
			log.Error("INSERT GooglePays exec failed %v ", this.m_OrderId)
			return err, false, released
		}
		d = true
	} else if state == 2 {
		_, err = this.m_table.m_dbc.Exec(update_string, args...)
		if err != nil {
			log.Error("UPDATE GooglePays exec failed %v", this.m_OrderId)
			return err, false, released
		}
		d = true
	}
	return nil, d, released
}
func (this *dbGooglePayRow) Touch(releasable bool) {
	this.m_touch = int32(time.Now().Unix())
	this.m_releasable = releasable
}
type dbGooglePayRowSort struct {
	rows []*dbGooglePayRow
}
func (this *dbGooglePayRowSort) Len() (length int) {
	return len(this.rows)
}
func (this *dbGooglePayRowSort) Less(i int, j int) (less bool) {
	return this.rows[i].m_touch < this.rows[j].m_touch
}
func (this *dbGooglePayRowSort) Swap(i int, j int) {
	temp := this.rows[i]
	this.rows[i] = this.rows[j]
	this.rows[j] = temp
}
type dbGooglePayTable struct{
	m_dbc *DBC
	m_lock *RWMutex
	m_rows map[string]*dbGooglePayRow
	m_new_rows map[string]*dbGooglePayRow
	m_removed_rows map[string]*dbGooglePayRow
	m_gc_n int32
	m_gcing int32
	m_pool_size int32
	m_preload_select_stmt *sql.Stmt
	m_preload_max_id int32
	m_save_insert_stmt *sql.Stmt
	m_delete_stmt *sql.Stmt
}
func new_dbGooglePayTable(dbc *DBC) (this *dbGooglePayTable) {
	this = &dbGooglePayTable{}
	this.m_dbc = dbc
	this.m_lock = NewRWMutex()
	this.m_rows = make(map[string]*dbGooglePayRow)
	this.m_new_rows = make(map[string]*dbGooglePayRow)
	this.m_removed_rows = make(map[string]*dbGooglePayRow)
	return this
}
func (this *dbGooglePayTable) check_create_table() (err error) {
	_, err = this.m_dbc.Exec("CREATE TABLE IF NOT EXISTS GooglePays(OrderId varchar(32),PRIMARY KEY (OrderId))ENGINE=InnoDB ROW_FORMAT=DYNAMIC")
	if err != nil {
		log.Error("CREATE TABLE IF NOT EXISTS GooglePays failed")
		return
	}
	rows, err := this.m_dbc.Query("SELECT COLUMN_NAME,ORDINAL_POSITION FROM information_schema.`COLUMNS` WHERE TABLE_SCHEMA=? AND TABLE_NAME='GooglePays'", this.m_dbc.m_db_name)
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
	_, hasBundleId := columns["BundleId"]
	if !hasBundleId {
		_, err = this.m_dbc.Exec("ALTER TABLE GooglePays ADD COLUMN BundleId varchar(256)")
		if err != nil {
			log.Error("ADD COLUMN BundleId failed")
			return
		}
	}
	_, hasAccount := columns["Account"]
	if !hasAccount {
		_, err = this.m_dbc.Exec("ALTER TABLE GooglePays ADD COLUMN Account varchar(256)")
		if err != nil {
			log.Error("ADD COLUMN Account failed")
			return
		}
	}
	_, hasPlayerId := columns["PlayerId"]
	if !hasPlayerId {
		_, err = this.m_dbc.Exec("ALTER TABLE GooglePays ADD COLUMN PlayerId int(11)")
		if err != nil {
			log.Error("ADD COLUMN PlayerId failed")
			return
		}
	}
	_, hasPayTime := columns["PayTime"]
	if !hasPayTime {
		_, err = this.m_dbc.Exec("ALTER TABLE GooglePays ADD COLUMN PayTime int(11)")
		if err != nil {
			log.Error("ADD COLUMN PayTime failed")
			return
		}
	}
	_, hasPayTimeStr := columns["PayTimeStr"]
	if !hasPayTimeStr {
		_, err = this.m_dbc.Exec("ALTER TABLE GooglePays ADD COLUMN PayTimeStr varchar(256)")
		if err != nil {
			log.Error("ADD COLUMN PayTimeStr failed")
			return
		}
	}
	return
}
func (this *dbGooglePayTable) prepare_preload_select_stmt() (err error) {
	this.m_preload_select_stmt,err=this.m_dbc.StmtPrepare("SELECT OrderId,BundleId,Account,PlayerId,PayTime,PayTimeStr FROM GooglePays")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbGooglePayTable) prepare_save_insert_stmt()(err error){
	this.m_save_insert_stmt,err=this.m_dbc.StmtPrepare("INSERT INTO GooglePays (OrderId,BundleId,Account,PlayerId,PayTime,PayTimeStr) VALUES (?,?,?,?,?,?)")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbGooglePayTable) prepare_delete_stmt() (err error) {
	this.m_delete_stmt,err=this.m_dbc.StmtPrepare("DELETE FROM GooglePays WHERE OrderId=?")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbGooglePayTable) Init() (err error) {
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
func (this *dbGooglePayTable) Preload() (err error) {
	r, err := this.m_dbc.StmtQuery(this.m_preload_select_stmt)
	if err != nil {
		log.Error("SELECT")
		return
	}
	var OrderId string
	var dBundleId string
	var dAccount string
	var dPlayerId int32
	var dPayTime int32
	var dPayTimeStr string
	for r.Next() {
		err = r.Scan(&OrderId,&dBundleId,&dAccount,&dPlayerId,&dPayTime,&dPayTimeStr)
		if err != nil {
			log.Error("Scan err[%v]", err.Error())
			return
		}
		row := new_dbGooglePayRow(this,OrderId)
		row.m_BundleId=dBundleId
		row.m_Account=dAccount
		row.m_PlayerId=dPlayerId
		row.m_PayTime=dPayTime
		row.m_PayTimeStr=dPayTimeStr
		row.m_BundleId_changed=false
		row.m_Account_changed=false
		row.m_PlayerId_changed=false
		row.m_PayTime_changed=false
		row.m_PayTimeStr_changed=false
		row.m_valid = true
		this.m_rows[OrderId]=row
	}
	return
}
func (this *dbGooglePayTable) GetPreloadedMaxId() (max_id int32) {
	return this.m_preload_max_id
}
func (this *dbGooglePayTable) fetch_rows(rows map[string]*dbGooglePayRow) (r map[string]*dbGooglePayRow) {
	this.m_lock.UnSafeLock("dbGooglePayTable.fetch_rows")
	defer this.m_lock.UnSafeUnlock()
	r = make(map[string]*dbGooglePayRow)
	for i, v := range rows {
		r[i] = v
	}
	return r
}
func (this *dbGooglePayTable) fetch_new_rows() (new_rows map[string]*dbGooglePayRow) {
	this.m_lock.UnSafeLock("dbGooglePayTable.fetch_new_rows")
	defer this.m_lock.UnSafeUnlock()
	new_rows = make(map[string]*dbGooglePayRow)
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
func (this *dbGooglePayTable) save_rows(rows map[string]*dbGooglePayRow, quick bool) {
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
func (this *dbGooglePayTable) Save(quick bool) (err error){
	removed_rows := this.fetch_rows(this.m_removed_rows)
	for _, v := range removed_rows {
		_, err := this.m_dbc.StmtExec(this.m_delete_stmt, v.GetOrderId())
		if err != nil {
			log.Error("exec delete stmt failed %v", err)
		}
		v.m_valid = false
		if !quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
	this.m_removed_rows = make(map[string]*dbGooglePayRow)
	rows := this.fetch_rows(this.m_rows)
	this.save_rows(rows, quick)
	new_rows := this.fetch_new_rows()
	this.save_rows(new_rows, quick)
	return
}
func (this *dbGooglePayTable) AddRow(OrderId string) (row *dbGooglePayRow) {
	this.m_lock.UnSafeLock("dbGooglePayTable.AddRow")
	defer this.m_lock.UnSafeUnlock()
	row = new_dbGooglePayRow(this,OrderId)
	row.m_new = true
	row.m_loaded = true
	row.m_valid = true
	_, has := this.m_new_rows[OrderId]
	if has{
		log.Error("已经存在 %v", OrderId)
		return nil
	}
	this.m_new_rows[OrderId] = row
	atomic.AddInt32(&this.m_gc_n,1)
	return row
}
func (this *dbGooglePayTable) RemoveRow(OrderId string) {
	this.m_lock.UnSafeLock("dbGooglePayTable.RemoveRow")
	defer this.m_lock.UnSafeUnlock()
	row := this.m_rows[OrderId]
	if row != nil {
		row.m_remove = true
		delete(this.m_rows, OrderId)
		rm_row := this.m_removed_rows[OrderId]
		if rm_row != nil {
			log.Error("rows and removed rows both has %v", OrderId)
		}
		this.m_removed_rows[OrderId] = row
		_, has_new := this.m_new_rows[OrderId]
		if has_new {
			delete(this.m_new_rows, OrderId)
			log.Error("rows and new_rows both has %v", OrderId)
		}
	} else {
		row = this.m_removed_rows[OrderId]
		if row == nil {
			_, has_new := this.m_new_rows[OrderId]
			if has_new {
				delete(this.m_new_rows, OrderId)
			} else {
				log.Error("row not exist %v", OrderId)
			}
		} else {
			log.Error("already removed %v", OrderId)
			_, has_new := this.m_new_rows[OrderId]
			if has_new {
				delete(this.m_new_rows, OrderId)
				log.Error("removed rows and new_rows both has %v", OrderId)
			}
		}
	}
}
func (this *dbGooglePayTable) GetRow(OrderId string) (row *dbGooglePayRow) {
	this.m_lock.UnSafeRLock("dbGooglePayTable.GetRow")
	defer this.m_lock.UnSafeRUnlock()
	row = this.m_rows[OrderId]
	if row == nil {
		row = this.m_new_rows[OrderId]
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
	ApplePays *dbApplePayTable
	GooglePays *dbGooglePayTable
}
func (this *DBC)init_tables()(err error){
	this.ApplePays = new_dbApplePayTable(this)
	err = this.ApplePays.Init()
	if err != nil {
		log.Error("init ApplePays table failed")
		return
	}
	this.GooglePays = new_dbGooglePayTable(this)
	err = this.GooglePays.Init()
	if err != nil {
		log.Error("init GooglePays table failed")
		return
	}
	return
}
func (this *DBC)Preload()(err error){
	err = this.ApplePays.Preload()
	if err != nil {
		log.Error("preload ApplePays table failed")
		return
	}else{
		log.Info("preload ApplePays table succeed !")
	}
	err = this.GooglePays.Preload()
	if err != nil {
		log.Error("preload GooglePays table failed")
		return
	}else{
		log.Info("preload GooglePays table succeed !")
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
	err = this.ApplePays.Save(quick)
	if err != nil {
		log.Error("save ApplePays table failed")
		return
	}
	err = this.GooglePays.Save(quick)
	if err != nil {
		log.Error("save GooglePays table failed")
		return
	}
	return
}
