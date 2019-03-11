package main

import (
	_ "github.com/golang/protobuf/proto"
	_ "github.com/go-sql-driver/mysql"
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"ih_server/libs/log"
	"math/rand"
	"os"
	"os/exec"
	_ "ih_server/proto/gen_go/db_login"
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
		
			now_time_hour := int32(time.Now().Hour())
			if now_time_hour-24 >= this.m_db_last_copy_time {
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


func (this *dbAccountRow)GetUniqueId( )(r string ){
	this.m_lock.UnSafeRLock("dbAccountRow.GetdbAccountUniqueIdColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_UniqueId)
}
func (this *dbAccountRow)SetUniqueId(v string){
	this.m_lock.UnSafeLock("dbAccountRow.SetdbAccountUniqueIdColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_UniqueId=string(v)
	this.m_UniqueId_changed=true
	return
}
func (this *dbAccountRow)GetPassword( )(r string ){
	this.m_lock.UnSafeRLock("dbAccountRow.GetdbAccountPasswordColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_Password)
}
func (this *dbAccountRow)SetPassword(v string){
	this.m_lock.UnSafeLock("dbAccountRow.SetdbAccountPasswordColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Password=string(v)
	this.m_Password_changed=true
	return
}
func (this *dbAccountRow)GetRegisterTime( )(r int32 ){
	this.m_lock.UnSafeRLock("dbAccountRow.GetdbAccountRegisterTimeColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_RegisterTime)
}
func (this *dbAccountRow)SetRegisterTime(v int32){
	this.m_lock.UnSafeLock("dbAccountRow.SetdbAccountRegisterTimeColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_RegisterTime=int32(v)
	this.m_RegisterTime_changed=true
	return
}
func (this *dbAccountRow)GetChannel( )(r string ){
	this.m_lock.UnSafeRLock("dbAccountRow.GetdbAccountChannelColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_Channel)
}
func (this *dbAccountRow)SetChannel(v string){
	this.m_lock.UnSafeLock("dbAccountRow.SetdbAccountChannelColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Channel=string(v)
	this.m_Channel_changed=true
	return
}
func (this *dbAccountRow)GetToken( )(r string ){
	this.m_lock.UnSafeRLock("dbAccountRow.GetdbAccountTokenColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_Token)
}
func (this *dbAccountRow)SetToken(v string){
	this.m_lock.UnSafeLock("dbAccountRow.SetdbAccountTokenColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_Token=string(v)
	this.m_Token_changed=true
	return
}
func (this *dbAccountRow)GetLastGetAccountPlayerListTime( )(r int32 ){
	this.m_lock.UnSafeRLock("dbAccountRow.GetdbAccountLastGetAccountPlayerListTimeColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_LastGetAccountPlayerListTime)
}
func (this *dbAccountRow)SetLastGetAccountPlayerListTime(v int32){
	this.m_lock.UnSafeLock("dbAccountRow.SetdbAccountLastGetAccountPlayerListTimeColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_LastGetAccountPlayerListTime=int32(v)
	this.m_LastGetAccountPlayerListTime_changed=true
	return
}
func (this *dbAccountRow)GetLastSelectServerId( )(r int32 ){
	this.m_lock.UnSafeRLock("dbAccountRow.GetdbAccountLastSelectServerIdColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_LastSelectServerId)
}
func (this *dbAccountRow)SetLastSelectServerId(v int32){
	this.m_lock.UnSafeLock("dbAccountRow.SetdbAccountLastSelectServerIdColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_LastSelectServerId=int32(v)
	this.m_LastSelectServerId_changed=true
	return
}
func (this *dbAccountRow)GetLastSelectIOSServerId( )(r int32 ){
	this.m_lock.UnSafeRLock("dbAccountRow.GetdbAccountLastSelectIOSServerIdColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_LastSelectIOSServerId)
}
func (this *dbAccountRow)SetLastSelectIOSServerId(v int32){
	this.m_lock.UnSafeLock("dbAccountRow.SetdbAccountLastSelectIOSServerIdColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_LastSelectIOSServerId=int32(v)
	this.m_LastSelectIOSServerId_changed=true
	return
}
func (this *dbAccountRow)GetBindNewAccount( )(r string ){
	this.m_lock.UnSafeRLock("dbAccountRow.GetdbAccountBindNewAccountColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_BindNewAccount)
}
func (this *dbAccountRow)SetBindNewAccount(v string){
	this.m_lock.UnSafeLock("dbAccountRow.SetdbAccountBindNewAccountColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_BindNewAccount=string(v)
	this.m_BindNewAccount_changed=true
	return
}
func (this *dbAccountRow)GetOldAccount( )(r string ){
	this.m_lock.UnSafeRLock("dbAccountRow.GetdbAccountOldAccountColumn")
	defer this.m_lock.UnSafeRUnlock()
	return string(this.m_OldAccount)
}
func (this *dbAccountRow)SetOldAccount(v string){
	this.m_lock.UnSafeLock("dbAccountRow.SetdbAccountOldAccountColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_OldAccount=string(v)
	this.m_OldAccount_changed=true
	return
}
type dbAccountRow struct {
	m_table *dbAccountTable
	m_lock       *RWMutex
	m_loaded  bool
	m_new     bool
	m_remove  bool
	m_touch      int32
	m_releasable bool
	m_valid   bool
	m_AccountId        string
	m_UniqueId_changed bool
	m_UniqueId string
	m_Password_changed bool
	m_Password string
	m_RegisterTime_changed bool
	m_RegisterTime int32
	m_Channel_changed bool
	m_Channel string
	m_Token_changed bool
	m_Token string
	m_LastGetAccountPlayerListTime_changed bool
	m_LastGetAccountPlayerListTime int32
	m_LastSelectServerId_changed bool
	m_LastSelectServerId int32
	m_LastSelectIOSServerId_changed bool
	m_LastSelectIOSServerId int32
	m_BindNewAccount_changed bool
	m_BindNewAccount string
	m_OldAccount_changed bool
	m_OldAccount string
}
func new_dbAccountRow(table *dbAccountTable, AccountId string) (r *dbAccountRow) {
	this := &dbAccountRow{}
	this.m_table = table
	this.m_AccountId = AccountId
	this.m_lock = NewRWMutex()
	this.m_UniqueId_changed=true
	this.m_Password_changed=true
	this.m_RegisterTime_changed=true
	this.m_Channel_changed=true
	this.m_Token_changed=true
	this.m_LastGetAccountPlayerListTime_changed=true
	this.m_LastSelectServerId_changed=true
	this.m_LastSelectIOSServerId_changed=true
	this.m_BindNewAccount_changed=true
	this.m_OldAccount_changed=true
	return this
}
func (this *dbAccountRow) GetAccountId() (r string) {
	return this.m_AccountId
}
func (this *dbAccountRow) save_data(release bool) (err error, released bool, state int32, update_string string, args []interface{}) {
	this.m_lock.UnSafeLock("dbAccountRow.save_data")
	defer this.m_lock.UnSafeUnlock()
	if this.m_new {
		db_args:=new_db_args(11)
		db_args.Push(this.m_AccountId)
		db_args.Push(this.m_UniqueId)
		db_args.Push(this.m_Password)
		db_args.Push(this.m_RegisterTime)
		db_args.Push(this.m_Channel)
		db_args.Push(this.m_Token)
		db_args.Push(this.m_LastGetAccountPlayerListTime)
		db_args.Push(this.m_LastSelectServerId)
		db_args.Push(this.m_LastSelectIOSServerId)
		db_args.Push(this.m_BindNewAccount)
		db_args.Push(this.m_OldAccount)
		args=db_args.GetArgs()
		state = 1
	} else {
		if this.m_UniqueId_changed||this.m_Password_changed||this.m_RegisterTime_changed||this.m_Channel_changed||this.m_Token_changed||this.m_LastGetAccountPlayerListTime_changed||this.m_LastSelectServerId_changed||this.m_LastSelectIOSServerId_changed||this.m_BindNewAccount_changed||this.m_OldAccount_changed{
			update_string = "UPDATE Accounts SET "
			db_args:=new_db_args(11)
			if this.m_UniqueId_changed{
				update_string+="UniqueId=?,"
				db_args.Push(this.m_UniqueId)
			}
			if this.m_Password_changed{
				update_string+="Password=?,"
				db_args.Push(this.m_Password)
			}
			if this.m_RegisterTime_changed{
				update_string+="RegisterTime=?,"
				db_args.Push(this.m_RegisterTime)
			}
			if this.m_Channel_changed{
				update_string+="Channel=?,"
				db_args.Push(this.m_Channel)
			}
			if this.m_Token_changed{
				update_string+="Token=?,"
				db_args.Push(this.m_Token)
			}
			if this.m_LastGetAccountPlayerListTime_changed{
				update_string+="LastGetAccountPlayerListTime=?,"
				db_args.Push(this.m_LastGetAccountPlayerListTime)
			}
			if this.m_LastSelectServerId_changed{
				update_string+="LastSelectServerId=?,"
				db_args.Push(this.m_LastSelectServerId)
			}
			if this.m_LastSelectIOSServerId_changed{
				update_string+="LastSelectIOSServerId=?,"
				db_args.Push(this.m_LastSelectIOSServerId)
			}
			if this.m_BindNewAccount_changed{
				update_string+="BindNewAccount=?,"
				db_args.Push(this.m_BindNewAccount)
			}
			if this.m_OldAccount_changed{
				update_string+="OldAccount=?,"
				db_args.Push(this.m_OldAccount)
			}
			update_string = strings.TrimRight(update_string, ", ")
			update_string+=" WHERE AccountId=?"
			db_args.Push(this.m_AccountId)
			args=db_args.GetArgs()
			state = 2
		}
	}
	this.m_new = false
	this.m_UniqueId_changed = false
	this.m_Password_changed = false
	this.m_RegisterTime_changed = false
	this.m_Channel_changed = false
	this.m_Token_changed = false
	this.m_LastGetAccountPlayerListTime_changed = false
	this.m_LastSelectServerId_changed = false
	this.m_LastSelectIOSServerId_changed = false
	this.m_BindNewAccount_changed = false
	this.m_OldAccount_changed = false
	if release && this.m_loaded {
		atomic.AddInt32(&this.m_table.m_gc_n, -1)
		this.m_loaded = false
		released = true
	}
	return nil,released,state,update_string,args
}
func (this *dbAccountRow) Save(release bool) (err error, d bool, released bool) {
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
			log.Error("INSERT Accounts exec failed %v ", this.m_AccountId)
			return err, false, released
		}
		d = true
	} else if state == 2 {
		_, err = this.m_table.m_dbc.Exec(update_string, args...)
		if err != nil {
			log.Error("UPDATE Accounts exec failed %v", this.m_AccountId)
			return err, false, released
		}
		d = true
	}
	return nil, d, released
}
func (this *dbAccountRow) Touch(releasable bool) {
	this.m_touch = int32(time.Now().Unix())
	this.m_releasable = releasable
}
type dbAccountRowSort struct {
	rows []*dbAccountRow
}
func (this *dbAccountRowSort) Len() (length int) {
	return len(this.rows)
}
func (this *dbAccountRowSort) Less(i int, j int) (less bool) {
	return this.rows[i].m_touch < this.rows[j].m_touch
}
func (this *dbAccountRowSort) Swap(i int, j int) {
	temp := this.rows[i]
	this.rows[i] = this.rows[j]
	this.rows[j] = temp
}
type dbAccountTable struct{
	m_dbc *DBC
	m_lock *RWMutex
	m_rows map[string]*dbAccountRow
	m_new_rows map[string]*dbAccountRow
	m_removed_rows map[string]*dbAccountRow
	m_gc_n int32
	m_gcing int32
	m_pool_size int32
	m_preload_select_stmt *sql.Stmt
	m_preload_max_id int32
	m_save_insert_stmt *sql.Stmt
	m_delete_stmt *sql.Stmt
}
func new_dbAccountTable(dbc *DBC) (this *dbAccountTable) {
	this = &dbAccountTable{}
	this.m_dbc = dbc
	this.m_lock = NewRWMutex()
	this.m_rows = make(map[string]*dbAccountRow)
	this.m_new_rows = make(map[string]*dbAccountRow)
	this.m_removed_rows = make(map[string]*dbAccountRow)
	return this
}
func (this *dbAccountTable) check_create_table() (err error) {
	_, err = this.m_dbc.Exec("CREATE TABLE IF NOT EXISTS Accounts(AccountId varchar(64),PRIMARY KEY (AccountId))ENGINE=InnoDB ROW_FORMAT=DYNAMIC")
	if err != nil {
		log.Error("CREATE TABLE IF NOT EXISTS Accounts failed")
		return
	}
	rows, err := this.m_dbc.Query("SELECT COLUMN_NAME,ORDINAL_POSITION FROM information_schema.`COLUMNS` WHERE TABLE_SCHEMA=? AND TABLE_NAME='Accounts'", this.m_dbc.m_db_name)
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
		_, err = this.m_dbc.Exec("ALTER TABLE Accounts ADD COLUMN UniqueId varchar(45) DEFAULT ''")
		if err != nil {
			log.Error("ADD COLUMN UniqueId failed")
			return
		}
	}
	_, hasPassword := columns["Password"]
	if !hasPassword {
		_, err = this.m_dbc.Exec("ALTER TABLE Accounts ADD COLUMN Password varchar(45) DEFAULT ''")
		if err != nil {
			log.Error("ADD COLUMN Password failed")
			return
		}
	}
	_, hasRegisterTime := columns["RegisterTime"]
	if !hasRegisterTime {
		_, err = this.m_dbc.Exec("ALTER TABLE Accounts ADD COLUMN RegisterTime int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN RegisterTime failed")
			return
		}
	}
	_, hasChannel := columns["Channel"]
	if !hasChannel {
		_, err = this.m_dbc.Exec("ALTER TABLE Accounts ADD COLUMN Channel varchar(45) DEFAULT ''")
		if err != nil {
			log.Error("ADD COLUMN Channel failed")
			return
		}
	}
	_, hasToken := columns["Token"]
	if !hasToken {
		_, err = this.m_dbc.Exec("ALTER TABLE Accounts ADD COLUMN Token varchar(45) DEFAULT ''")
		if err != nil {
			log.Error("ADD COLUMN Token failed")
			return
		}
	}
	_, hasLastGetAccountPlayerListTime := columns["LastGetAccountPlayerListTime"]
	if !hasLastGetAccountPlayerListTime {
		_, err = this.m_dbc.Exec("ALTER TABLE Accounts ADD COLUMN LastGetAccountPlayerListTime int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN LastGetAccountPlayerListTime failed")
			return
		}
	}
	_, hasLastSelectServerId := columns["LastSelectServerId"]
	if !hasLastSelectServerId {
		_, err = this.m_dbc.Exec("ALTER TABLE Accounts ADD COLUMN LastSelectServerId int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN LastSelectServerId failed")
			return
		}
	}
	_, hasLastSelectIOSServerId := columns["LastSelectIOSServerId"]
	if !hasLastSelectIOSServerId {
		_, err = this.m_dbc.Exec("ALTER TABLE Accounts ADD COLUMN LastSelectIOSServerId int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN LastSelectIOSServerId failed")
			return
		}
	}
	_, hasBindNewAccount := columns["BindNewAccount"]
	if !hasBindNewAccount {
		_, err = this.m_dbc.Exec("ALTER TABLE Accounts ADD COLUMN BindNewAccount varchar(45) DEFAULT '0'")
		if err != nil {
			log.Error("ADD COLUMN BindNewAccount failed")
			return
		}
	}
	_, hasOldAccount := columns["OldAccount"]
	if !hasOldAccount {
		_, err = this.m_dbc.Exec("ALTER TABLE Accounts ADD COLUMN OldAccount varchar(45) DEFAULT '0'")
		if err != nil {
			log.Error("ADD COLUMN OldAccount failed")
			return
		}
	}
	return
}
func (this *dbAccountTable) prepare_preload_select_stmt() (err error) {
	this.m_preload_select_stmt,err=this.m_dbc.StmtPrepare("SELECT AccountId,UniqueId,Password,RegisterTime,Channel,Token,LastGetAccountPlayerListTime,LastSelectServerId,LastSelectIOSServerId,BindNewAccount,OldAccount FROM Accounts")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbAccountTable) prepare_save_insert_stmt()(err error){
	this.m_save_insert_stmt,err=this.m_dbc.StmtPrepare("INSERT INTO Accounts (AccountId,UniqueId,Password,RegisterTime,Channel,Token,LastGetAccountPlayerListTime,LastSelectServerId,LastSelectIOSServerId,BindNewAccount,OldAccount) VALUES (?,?,?,?,?,?,?,?,?,?,?)")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbAccountTable) prepare_delete_stmt() (err error) {
	this.m_delete_stmt,err=this.m_dbc.StmtPrepare("DELETE FROM Accounts WHERE AccountId=?")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbAccountTable) Init() (err error) {
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
func (this *dbAccountTable) Preload() (err error) {
	r, err := this.m_dbc.StmtQuery(this.m_preload_select_stmt)
	if err != nil {
		log.Error("SELECT")
		return
	}
	var AccountId string
	var dUniqueId string
	var dPassword string
	var dRegisterTime int32
	var dChannel string
	var dToken string
	var dLastGetAccountPlayerListTime int32
	var dLastSelectServerId int32
	var dLastSelectIOSServerId int32
	var dBindNewAccount string
	var dOldAccount string
	for r.Next() {
		err = r.Scan(&AccountId,&dUniqueId,&dPassword,&dRegisterTime,&dChannel,&dToken,&dLastGetAccountPlayerListTime,&dLastSelectServerId,&dLastSelectIOSServerId,&dBindNewAccount,&dOldAccount)
		if err != nil {
			log.Error("Scan err[%v]", err.Error())
			return
		}
		row := new_dbAccountRow(this,AccountId)
		row.m_UniqueId=dUniqueId
		row.m_Password=dPassword
		row.m_RegisterTime=dRegisterTime
		row.m_Channel=dChannel
		row.m_Token=dToken
		row.m_LastGetAccountPlayerListTime=dLastGetAccountPlayerListTime
		row.m_LastSelectServerId=dLastSelectServerId
		row.m_LastSelectIOSServerId=dLastSelectIOSServerId
		row.m_BindNewAccount=dBindNewAccount
		row.m_OldAccount=dOldAccount
		row.m_UniqueId_changed=false
		row.m_Password_changed=false
		row.m_RegisterTime_changed=false
		row.m_Channel_changed=false
		row.m_Token_changed=false
		row.m_LastGetAccountPlayerListTime_changed=false
		row.m_LastSelectServerId_changed=false
		row.m_LastSelectIOSServerId_changed=false
		row.m_BindNewAccount_changed=false
		row.m_OldAccount_changed=false
		row.m_valid = true
		this.m_rows[AccountId]=row
	}
	return
}
func (this *dbAccountTable) GetPreloadedMaxId() (max_id int32) {
	return this.m_preload_max_id
}
func (this *dbAccountTable) fetch_rows(rows map[string]*dbAccountRow) (r map[string]*dbAccountRow) {
	this.m_lock.UnSafeLock("dbAccountTable.fetch_rows")
	defer this.m_lock.UnSafeUnlock()
	r = make(map[string]*dbAccountRow)
	for i, v := range rows {
		r[i] = v
	}
	return r
}
func (this *dbAccountTable) fetch_new_rows() (new_rows map[string]*dbAccountRow) {
	this.m_lock.UnSafeLock("dbAccountTable.fetch_new_rows")
	defer this.m_lock.UnSafeUnlock()
	new_rows = make(map[string]*dbAccountRow)
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
func (this *dbAccountTable) save_rows(rows map[string]*dbAccountRow, quick bool) {
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
func (this *dbAccountTable) Save(quick bool) (err error){
	removed_rows := this.fetch_rows(this.m_removed_rows)
	for _, v := range removed_rows {
		_, err := this.m_dbc.StmtExec(this.m_delete_stmt, v.GetAccountId())
		if err != nil {
			log.Error("exec delete stmt failed %v", err)
		}
		v.m_valid = false
		if !quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
	this.m_removed_rows = make(map[string]*dbAccountRow)
	rows := this.fetch_rows(this.m_rows)
	this.save_rows(rows, quick)
	new_rows := this.fetch_new_rows()
	this.save_rows(new_rows, quick)
	return
}
func (this *dbAccountTable) AddRow(AccountId string) (row *dbAccountRow) {
	this.m_lock.UnSafeLock("dbAccountTable.AddRow")
	defer this.m_lock.UnSafeUnlock()
	row = new_dbAccountRow(this,AccountId)
	row.m_new = true
	row.m_loaded = true
	row.m_valid = true
	_, has := this.m_new_rows[AccountId]
	if has{
		log.Error("已经存在 %v", AccountId)
		return nil
	}
	this.m_new_rows[AccountId] = row
	atomic.AddInt32(&this.m_gc_n,1)
	return row
}
func (this *dbAccountTable) RemoveRow(AccountId string) {
	this.m_lock.UnSafeLock("dbAccountTable.RemoveRow")
	defer this.m_lock.UnSafeUnlock()
	row := this.m_rows[AccountId]
	if row != nil {
		row.m_remove = true
		delete(this.m_rows, AccountId)
		rm_row := this.m_removed_rows[AccountId]
		if rm_row != nil {
			log.Error("rows and removed rows both has %v", AccountId)
		}
		this.m_removed_rows[AccountId] = row
		_, has_new := this.m_new_rows[AccountId]
		if has_new {
			delete(this.m_new_rows, AccountId)
			log.Error("rows and new_rows both has %v", AccountId)
		}
	} else {
		row = this.m_removed_rows[AccountId]
		if row == nil {
			_, has_new := this.m_new_rows[AccountId]
			if has_new {
				delete(this.m_new_rows, AccountId)
			} else {
				log.Error("row not exist %v", AccountId)
			}
		} else {
			log.Error("already removed %v", AccountId)
			_, has_new := this.m_new_rows[AccountId]
			if has_new {
				delete(this.m_new_rows, AccountId)
				log.Error("removed rows and new_rows both has %v", AccountId)
			}
		}
	}
}
func (this *dbAccountTable) GetRow(AccountId string) (row *dbAccountRow) {
	this.m_lock.UnSafeRLock("dbAccountTable.GetRow")
	defer this.m_lock.UnSafeRUnlock()
	row = this.m_rows[AccountId]
	if row == nil {
		row = this.m_new_rows[AccountId]
	}
	return row
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
	m_Account_changed bool
	m_Account string
	m_PlayerId_changed bool
	m_PlayerId int32
	m_StartTime_changed bool
	m_StartTime int32
	m_StartTimeStr_changed bool
	m_StartTimeStr string
	m_Duration_changed bool
	m_Duration int32
}
func new_dbBanPlayerRow(table *dbBanPlayerTable, UniqueId string) (r *dbBanPlayerRow) {
	this := &dbBanPlayerRow{}
	this.m_table = table
	this.m_UniqueId = UniqueId
	this.m_lock = NewRWMutex()
	this.m_Account_changed=true
	this.m_PlayerId_changed=true
	this.m_StartTime_changed=true
	this.m_StartTimeStr_changed=true
	this.m_Duration_changed=true
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
		db_args.Push(this.m_Account)
		db_args.Push(this.m_PlayerId)
		db_args.Push(this.m_StartTime)
		db_args.Push(this.m_StartTimeStr)
		db_args.Push(this.m_Duration)
		args=db_args.GetArgs()
		state = 1
	} else {
		if this.m_Account_changed||this.m_PlayerId_changed||this.m_StartTime_changed||this.m_StartTimeStr_changed||this.m_Duration_changed{
			update_string = "UPDATE BanPlayers SET "
			db_args:=new_db_args(6)
			if this.m_Account_changed{
				update_string+="Account=?,"
				db_args.Push(this.m_Account)
			}
			if this.m_PlayerId_changed{
				update_string+="PlayerId=?,"
				db_args.Push(this.m_PlayerId)
			}
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
			update_string = strings.TrimRight(update_string, ", ")
			update_string+=" WHERE UniqueId=?"
			db_args.Push(this.m_UniqueId)
			args=db_args.GetArgs()
			state = 2
		}
	}
	this.m_new = false
	this.m_Account_changed = false
	this.m_PlayerId_changed = false
	this.m_StartTime_changed = false
	this.m_StartTimeStr_changed = false
	this.m_Duration_changed = false
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
	_, hasAccount := columns["Account"]
	if !hasAccount {
		_, err = this.m_dbc.Exec("ALTER TABLE BanPlayers ADD COLUMN Account varchar(45) DEFAULT ''")
		if err != nil {
			log.Error("ADD COLUMN Account failed")
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
		_, err = this.m_dbc.Exec("ALTER TABLE BanPlayers ADD COLUMN StartTimeStr varchar(45) DEFAULT ''")
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
	return
}
func (this *dbBanPlayerTable) prepare_preload_select_stmt() (err error) {
	this.m_preload_select_stmt,err=this.m_dbc.StmtPrepare("SELECT UniqueId,Account,PlayerId,StartTime,StartTimeStr,Duration FROM BanPlayers")
	if err!=nil{
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbBanPlayerTable) prepare_save_insert_stmt()(err error){
	this.m_save_insert_stmt,err=this.m_dbc.StmtPrepare("INSERT INTO BanPlayers (UniqueId,Account,PlayerId,StartTime,StartTimeStr,Duration) VALUES (?,?,?,?,?,?)")
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
	var dAccount string
	var dPlayerId int32
	var dStartTime int32
	var dStartTimeStr string
	var dDuration int32
	for r.Next() {
		err = r.Scan(&UniqueId,&dAccount,&dPlayerId,&dStartTime,&dStartTimeStr,&dDuration)
		if err != nil {
			log.Error("Scan err[%v]", err.Error())
			return
		}
		row := new_dbBanPlayerRow(this,UniqueId)
		row.m_Account=dAccount
		row.m_PlayerId=dPlayerId
		row.m_StartTime=dStartTime
		row.m_StartTimeStr=dStartTimeStr
		row.m_Duration=dDuration
		row.m_Account_changed=false
		row.m_PlayerId_changed=false
		row.m_StartTime_changed=false
		row.m_StartTimeStr_changed=false
		row.m_Duration_changed=false
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
	Accounts *dbAccountTable
	BanPlayers *dbBanPlayerTable
}
func (this *DBC)init_tables()(err error){
	this.Accounts = new_dbAccountTable(this)
	err = this.Accounts.Init()
	if err != nil {
		log.Error("init Accounts table failed")
		return
	}
	this.BanPlayers = new_dbBanPlayerTable(this)
	err = this.BanPlayers.Init()
	if err != nil {
		log.Error("init BanPlayers table failed")
		return
	}
	return
}
func (this *DBC)Preload()(err error){
	err = this.Accounts.Preload()
	if err != nil {
		log.Error("preload Accounts table failed")
		return
	}else{
		log.Info("preload Accounts table succeed !")
	}
	err = this.BanPlayers.Preload()
	if err != nil {
		log.Error("preload BanPlayers table failed")
		return
	}else{
		log.Info("preload BanPlayers table succeed !")
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
	err = this.Accounts.Save(quick)
	if err != nil {
		log.Error("save Accounts table failed")
		return
	}
	err = this.BanPlayers.Save(quick)
	if err != nil {
		log.Error("save BanPlayers table failed")
		return
	}
	return
}
