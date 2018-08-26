package main

import (
	"database/sql"
	"errors"
	"ih_server/libs/log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DEFAULT_PLAYER_TABLE_COUNT = 5000
)

// ----------------------------------------------------------------------------

func (this *AccountDBC) StmtPrepare(s string) (r *sql.Stmt, e error) {
	this.m_db_lock.Lock("DBC.StmtPrepare")
	defer this.m_db_lock.Unlock()
	return this.m_db.Prepare(s)
}
func (this *AccountDBC) StmtExec(stmt *sql.Stmt, args ...interface{}) (r sql.Result, err error) {
	this.m_db_lock.Lock("DBC.StmtExec")
	defer this.m_db_lock.Unlock()
	return stmt.Exec(args...)
}
func (this *AccountDBC) StmtQuery(stmt *sql.Stmt, args ...interface{}) (r *sql.Rows, err error) {
	this.m_db_lock.Lock("DBC.StmtQuery")
	defer this.m_db_lock.Unlock()
	return stmt.Query(args...)
}
func (this *AccountDBC) StmtQueryRow(stmt *sql.Stmt, args ...interface{}) (r *sql.Row) {
	this.m_db_lock.Lock("DBC.StmtQueryRow")
	defer this.m_db_lock.Unlock()
	return stmt.QueryRow(args...)
}
func (this *AccountDBC) Query(s string, args ...interface{}) (r *sql.Rows, e error) {
	this.m_db_lock.Lock("DBC.Query")
	defer this.m_db_lock.Unlock()
	return this.m_db.Query(s, args...)
}
func (this *AccountDBC) QueryRow(s string, args ...interface{}) (r *sql.Row) {
	this.m_db_lock.Lock("DBC.QueryRow")
	defer this.m_db_lock.Unlock()
	return this.m_db.QueryRow(s, args...)
}
func (this *AccountDBC) Exec(s string, args ...interface{}) (r sql.Result, e error) {
	this.m_db_lock.Lock("DBC.Exec")
	defer this.m_db_lock.Unlock()
	return this.m_db.Exec(s, args...)
}
func (this *AccountDBC) Conn(name string, addr string, acc string, pwd string, db_copy_path string) (err error) {
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

func (this *AccountDBC) Loop() {
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
func (this *AccountDBC) Shutdown() {
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

type AccountDBC struct {
	m_db_name            string
	m_db                 *sql.DB
	m_db_lock            *Mutex
	m_initialized        bool
	m_quit               bool
	m_shutdown_completed bool
	m_shutdown_lock      *Mutex
	m_db_last_copy_time  int32
	m_db_copy_path       string
	m_db_addr            string
	m_db_account         string
	m_db_password        string
	PlayerIdMax          *dbPlayerIdMaxTable
	AccountsMgr          *dbAccountsManager
}

func (this *AccountDBC) init_tables() (err error) {
	this.PlayerIdMax = new_dbPlayerIdMaxTable(this)
	err = this.PlayerIdMax.Init()
	if err != nil {
		log.Error("AccountDBC init PlayerIdMax table failed !")
		return
	}

	this.AccountsMgr = &dbAccountsManager{}
	err = this.AccountsMgr.Init(this)
	if err != nil {
		log.Error("AccountDBC init AccountsMgr failed !")
		return
	}

	return
}
func (this *AccountDBC) Preload() (err error) {
	err = this.PlayerIdMax.Preload()
	if err != nil {
		log.Error("preload PlayerIdMax table failed")
		return
	} else {
		log.Info("preload PlayerIdMax table succeed !")
	}

	tmp_row := this.PlayerIdMax.GetRow()
	if nil == tmp_row {
		log.Error("AccountDBC Preload PlayerIdMax GetRow nil !")
		return
	}

	err = this.AccountsMgr.PreLoad(this, tmp_row.GetPlayerIdMax())
	if nil != err {
		log.Error("AccountDBC Preload AccountsMgr error [%s]", err.Error())
		return
	}

	return
}
func (this *AccountDBC) Save(quick bool) (err error) {
	err = this.PlayerIdMax.Save(quick)
	if err != nil {
		log.Error("save PlayerIdMax table failed")
		return
	}

	err = this.AccountsMgr.Save(quick)
	if err != nil {
		log.Error("save AccountsMgr table failed")
		return
	}
	return
}

// ----------------------------------------------------------------------------

func (this *dbPlayerIdMaxRow) GetPlayerIdMax() (r int32) {
	this.m_lock.UnSafeRLock("dbPlayerIdMaxRow.GetPlayerIdMax")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_PlayerIdMax)
}
func (this *dbPlayerIdMaxRow) SetPlayerIdMax(v int32) {
	this.m_lock.UnSafeLock("dbPlayerIdMaxRow.SetPlayerIdMax")
	defer this.m_lock.UnSafeUnlock()
	this.m_PlayerIdMax = int32(v)
	this.m_PlayerIdMax_changed = true
	return
}

func (this *dbPlayerIdMaxRow) IncPlayerIdMax(val int32) int32 {
	this.m_lock.UnSafeLock("dbPlayerIdMaxRow.IncPlayerIdMax")
	defer this.m_lock.UnSafeUnlock()
	this.m_PlayerIdMax += val
	this.m_PlayerIdMax_changed = true
	return this.m_PlayerIdMax
}

type dbPlayerIdMaxRow struct {
	m_table               *dbPlayerIdMaxTable
	m_lock                *RWMutex
	m_loaded              bool
	m_new                 bool
	m_remove              bool
	m_touch               int32
	m_releasable          bool
	m_valid               bool
	m_PlaceHolder         int32
	m_PlayerIdMax_changed bool
	m_PlayerIdMax         int32
}

func new_dbPlayerIdMaxRow(table *dbPlayerIdMaxTable, PlaceHolder int32) (r *dbPlayerIdMaxRow) {
	this := &dbPlayerIdMaxRow{}
	this.m_table = table
	this.m_PlaceHolder = PlaceHolder
	this.m_lock = NewRWMutex()
	this.m_PlayerIdMax_changed = true
	return this
}
func (this *dbPlayerIdMaxRow) save_data(release bool) (err error, released bool, state int32, update_string string, args []interface{}) {
	this.m_lock.UnSafeLock("dbPlayerIdMaxRow.save_data")
	defer this.m_lock.UnSafeUnlock()
	if this.m_new {
		db_args := new_db_args(2)
		db_args.Push(this.m_PlaceHolder)
		db_args.Push(this.m_PlayerIdMax)
		args = db_args.GetArgs()
		state = 1
	} else {
		if this.m_PlayerIdMax_changed {
			update_string = "UPDATE PlayerIdMax SET "
			db_args := new_db_args(2)
			if this.m_PlayerIdMax_changed {
				update_string += "PlayerIdMax=?,"
				db_args.Push(this.m_PlayerIdMax)
			}
			update_string = strings.TrimRight(update_string, ", ")
			update_string += " WHERE PlaceHolder=?"
			db_args.Push(this.m_PlaceHolder)
			args = db_args.GetArgs()
			state = 2
		}
	}
	this.m_new = false
	this.m_PlayerIdMax_changed = false
	if release && this.m_loaded {
		this.m_loaded = false
		released = true
	}
	return nil, released, state, update_string, args
}
func (this *dbPlayerIdMaxRow) Save(release bool) (err error, d bool, released bool) {
	err, released, state, update_string, args := this.save_data(release)
	if err != nil {
		log.Error("save data failed")
		return err, false, false
	}
	if state == 0 {
		d = false
	} else if state == 1 {
		_, err = this.m_table.m_dbc.StmtExec(this.m_table.m_save_insert_stmt, args...)
		if err != nil {
			log.Error("INSERT PlayerIdMax exec failed %v ", this.m_PlaceHolder)
			return err, false, released
		}
		d = true
	} else if state == 2 {
		_, err = this.m_table.m_dbc.Exec(update_string, args...)
		if err != nil {
			log.Error("UPDATE PlayerIdMax exec failed %v", this.m_PlaceHolder)
			return err, false, released
		}
		d = true
	}
	return nil, d, released
}

type dbPlayerIdMaxTable struct {
	m_dbc                 *AccountDBC
	m_lock                *RWMutex
	m_row                 *dbPlayerIdMaxRow
	m_preload_select_stmt *sql.Stmt
	m_save_insert_stmt    *sql.Stmt
}

func new_dbPlayerIdMaxTable(dbc *AccountDBC) (this *dbPlayerIdMaxTable) {
	this = &dbPlayerIdMaxTable{}
	this.m_dbc = dbc
	this.m_lock = NewRWMutex()
	return this
}
func (this *dbPlayerIdMaxTable) check_create_table() (err error) {
	_, err = this.m_dbc.Exec("CREATE TABLE IF NOT EXISTS PlayerIdMax(PlaceHolder int(11),PRIMARY KEY (PlaceHolder))ENGINE=InnoDB ROW_FORMAT=DYNAMIC")
	if err != nil {
		log.Error("CREATE TABLE IF NOT EXISTS PlayerIdMax failed")
		return
	}
	rows, err := this.m_dbc.Query("SELECT COLUMN_NAME,ORDINAL_POSITION FROM information_schema.`COLUMNS` WHERE TABLE_SCHEMA=? AND TABLE_NAME='PlayerIdMax'", this.m_dbc.m_db_name)
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
	_, hasPlayerIdMax := columns["PlayerIdMax"]
	if !hasPlayerIdMax {
		_, err = this.m_dbc.Exec("ALTER TABLE PlayerIdMax ADD COLUMN PlayerIdMax int(11) DEFAULT 0")
		if err != nil {
			log.Error("ADD COLUMN PlayerIdMax failed")
			return
		}
	}
	return
}
func (this *dbPlayerIdMaxTable) prepare_preload_select_stmt() (err error) {
	this.m_preload_select_stmt, err = this.m_dbc.StmtPrepare("SELECT PlayerIdMax FROM PlayerIdMax WHERE PlaceHolder=0")
	if err != nil {
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbPlayerIdMaxTable) prepare_save_insert_stmt() (err error) {
	this.m_save_insert_stmt, err = this.m_dbc.StmtPrepare("INSERT INTO PlayerIdMax (PlaceHolder,PlayerIdMax) VALUES (?,?)")
	if err != nil {
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbPlayerIdMaxTable) Init() (err error) {
	err = this.check_create_table()
	if err != nil {
		log.Error("check_create_table failed")
		return
	}
	err = this.prepare_preload_select_stmt()
	if err != nil {
		log.Error("prepare_preload_select_stmt failed")
		return
	}
	err = this.prepare_save_insert_stmt()
	if err != nil {
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	return
}
func (this *dbPlayerIdMaxTable) Preload() (err error) {
	r := this.m_dbc.StmtQueryRow(this.m_preload_select_stmt)
	var dPlayerIdMax int32
	err = r.Scan(&dPlayerIdMax)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Error("Scan failed")
			return
		}
	} else {
		row := new_dbPlayerIdMaxRow(this, 0)
		row.m_PlayerIdMax = dPlayerIdMax
		row.m_PlayerIdMax_changed = false
		row.m_valid = true
		row.m_loaded = true
		this.m_row = row
	}
	if this.m_row == nil {
		this.m_row = new_dbPlayerIdMaxRow(this, 0)
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
func (this *dbPlayerIdMaxTable) Save(quick bool) (err error) {
	if this.m_row == nil {
		return errors.New("row nil")
	}
	err, _, _ = this.m_row.Save(false)
	return err
}
func (this *dbPlayerIdMaxTable) GetRow() (row *dbPlayerIdMaxRow) {
	return this.m_row
}

// ----------------------------------------------------------------------------

func (this *dbAccountRow) GetPlayerId() (r int32) {
	this.m_lock.UnSafeRLock("dbAccountRow.GetdbAccountPlayerIdColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_PlayerId)
}
func (this *dbAccountRow) SetPlayerId(v int32) {
	this.m_lock.UnSafeLock("dbAccountRow.SetdbAccountPlayerIdColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_PlayerId = int32(v)
	this.m_PlayerId_changed = true
	return
}
func (this *dbAccountRow) GetCreateTime() (r int32) {
	this.m_lock.UnSafeRLock("dbAccountRow.GetdbAccountCreateTimeColumn")
	defer this.m_lock.UnSafeRUnlock()
	return int32(this.m_CreateTime)
}
func (this *dbAccountRow) SetCreateTime(v int32) {
	this.m_lock.UnSafeLock("dbAccountRow.SetdbAccountCreateTimeColumn")
	defer this.m_lock.UnSafeUnlock()
	this.m_CreateTime = int32(v)
	this.m_CreateTime_changed = true
	return
}

type dbAccountRow struct {
	m_table_name         string
	m_table              *dbAccountTable
	m_lock               *RWMutex
	m_loaded             bool
	m_new                bool
	m_remove             bool
	m_touch              int32
	m_releasable         bool
	m_valid              bool
	m_Account            string
	m_PlayerId_changed   bool
	m_PlayerId           int32
	m_CreateTime_changed bool
	m_CreateTime         int32
	m_update_pre_str     string
}

func new_dbAccountRow(table *dbAccountTable, Account, table_name string) (r *dbAccountRow) {
	this := &dbAccountRow{}
	this.m_table_name = table_name
	this.m_table = table
	this.m_Account = Account
	this.m_lock = NewRWMutex()
	this.m_PlayerId_changed = true
	this.m_CreateTime_changed = true
	this.m_update_pre_str = "UPDATE " + table_name + " SET "
	return this
}
func (this *dbAccountRow) GetAccount() (r string) {
	return this.m_Account
}
func (this *dbAccountRow) save_data(release bool) (err error, released bool, state int32, update_string string, args []interface{}) {
	this.m_lock.UnSafeLock("dbAccountRow.save_data")
	defer this.m_lock.UnSafeUnlock()
	if this.m_new {
		db_args := new_db_args(3)
		db_args.Push(this.m_Account)
		db_args.Push(this.m_PlayerId)
		db_args.Push(this.m_CreateTime)
		args = db_args.GetArgs()
		state = 1
	} else {
		if this.m_PlayerId_changed || this.m_CreateTime_changed {
			update_string = this.m_update_pre_str
			db_args := new_db_args(3)
			if this.m_PlayerId_changed {
				update_string += "PlayerId=?,"
				db_args.Push(this.m_PlayerId)
			}
			if this.m_CreateTime_changed {
				update_string += "CreateTime=?,"
				db_args.Push(this.m_CreateTime)
			}
			update_string = strings.TrimRight(update_string, ", ")
			update_string += " WHERE Account=?"
			db_args.Push(this.m_Account)
			args = db_args.GetArgs()
			state = 2
		}
	}
	this.m_new = false
	this.m_PlayerId_changed = false
	this.m_CreateTime_changed = false
	if release && this.m_loaded {
		atomic.AddInt32(&this.m_table.m_gc_n, -1)
		this.m_loaded = false
		released = true
	}
	return nil, released, state, update_string, args
}
func (this *dbAccountRow) Save(release bool) (err error, d bool, released bool) {
	err, released, state, update_string, args := this.save_data(release)
	if err != nil {
		log.Error("save data failed")
		return err, false, false
	}
	if state == 0 {
		d = false
	} else if state == 1 {
		_, err = this.m_table.m_dbc.StmtExec(this.m_table.m_save_insert_stmt, args...)
		if err != nil {
			log.Error("INSERT %s exec failed %v ", this.m_table_name, this.m_Account)
			return err, false, released
		}
		d = true
	} else if state == 2 {
		_, err = this.m_table.m_dbc.Exec(update_string, args...)
		if err != nil {
			log.Error("UPDATE Accounts exec failed %v", this.m_Account)
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

// ---------------------------------------------------------------------------

type dbAccountTable struct {
	m_dbc                 *AccountDBC
	m_lock                *RWMutex
	m_rows                map[string]*dbAccountRow
	m_new_rows            map[string]*dbAccountRow
	m_removed_rows        map[string]*dbAccountRow
	m_gc_n                int32
	m_gcing               int32
	m_pool_size           int32
	m_preload_select_stmt *sql.Stmt
	m_preload_max_id      int32
	m_save_insert_stmt    *sql.Stmt
	m_delete_stmt         *sql.Stmt
	m_table_name          string
}

func new_dbAccountTable(dbc *AccountDBC) (this *dbAccountTable) {
	this = &dbAccountTable{}
	this.m_dbc = dbc
	this.m_lock = NewRWMutex()
	this.m_rows = make(map[string]*dbAccountRow)
	this.m_new_rows = make(map[string]*dbAccountRow)
	this.m_removed_rows = make(map[string]*dbAccountRow)
	return this
}

func (this *dbAccountTable) check_create_table_by_name(table_name string) (err error) {
	sql_str := "CREATE TABLE IF NOT EXISTS " + table_name + "(Account varchar(45),PRIMARY KEY (Account))ENGINE=InnoDB ROW_FORMAT=DYNAMIC"
	_, err = this.m_dbc.Exec(sql_str)
	if err != nil {
		log.Error("CREATE TABLE IF NOT EXISTS %s failed", table_name)
		return
	}
	rows, err := this.m_dbc.Query("SELECT COLUMN_NAME,ORDINAL_POSITION FROM information_schema.`COLUMNS` WHERE TABLE_SCHEMA=? AND TABLE_NAME='"+table_name+"'", this.m_dbc.m_db_name)
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
	_, hasPlayerId := columns["PlayerId"]
	if !hasPlayerId {
		_, err = this.m_dbc.Exec("ALTER TABLE " + table_name + " ADD COLUMN PlayerId int(11)")
		if err != nil {
			log.Error("ADD COLUMN PlayerId failed")
			return
		}
	}
	_, hasCreateTime := columns["CreateTime"]
	if !hasCreateTime {
		_, err = this.m_dbc.Exec("ALTER TABLE " + table_name + " ADD COLUMN CreateTime int(11)")
		if err != nil {
			log.Error("ADD COLUMN CreateTime failed")
			return
		}
	}
	return
}
func (this *dbAccountTable) prepare_preload_select_stmt(table_name string) (err error) {
	this.m_preload_select_stmt, err = this.m_dbc.StmtPrepare("SELECT Account,PlayerId,CreateTime FROM " + table_name)
	if err != nil {
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbAccountTable) prepare_save_insert_stmt(table_name string) (err error) {
	this.m_save_insert_stmt, err = this.m_dbc.StmtPrepare("INSERT INTO " + table_name + " (Account,PlayerId,CreateTime) VALUES (?,?,?)")
	if err != nil {
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbAccountTable) prepare_delete_stmt(table_name string) (err error) {
	this.m_delete_stmt, err = this.m_dbc.StmtPrepare("DELETE FROM " + table_name + " WHERE Account=?")
	if err != nil {
		log.Error("prepare failed")
		return
	}
	return
}
func (this *dbAccountTable) Init(table_name string) (err error) {
	this.m_table_name = table_name
	err = this.check_create_table_by_name(table_name)
	if err != nil {
		log.Error("check_create_table failed")
		return
	}
	err = this.prepare_preload_select_stmt(table_name)
	if err != nil {
		log.Error("prepare_preload_select_stmt failed")
		return
	}
	err = this.prepare_save_insert_stmt(table_name)
	if err != nil {
		log.Error("prepare_save_insert_stmt failed")
		return
	}
	err = this.prepare_delete_stmt(table_name)
	if err != nil {
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
	var Account string
	var dPlayerId int32
	var dCreateTime int32
	for r.Next() {
		err = r.Scan(&Account, &dPlayerId, &dCreateTime)
		if err != nil {
			log.Error("Scan")
			return
		}
		row := new_dbAccountRow(this, Account, this.m_table_name)
		row.m_PlayerId = dPlayerId
		row.m_CreateTime = dCreateTime
		row.m_PlayerId_changed = false
		row.m_CreateTime_changed = false
		row.m_valid = true
		this.m_rows[Account] = row
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
		if delay && !quick {
			time.Sleep(time.Millisecond * 5)
		}
	}
}
func (this *dbAccountTable) Save(quick bool) (err error) {
	removed_rows := this.fetch_rows(this.m_removed_rows)
	for _, v := range removed_rows {
		_, err := this.m_dbc.StmtExec(this.m_delete_stmt, v.GetAccount())
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
func (this *dbAccountTable) AddRow(Account string) (row *dbAccountRow) {
	this.m_lock.UnSafeLock("dbAccountTable.AddRow")
	defer this.m_lock.UnSafeUnlock()
	row = new_dbAccountRow(this, Account, this.m_table_name)
	row.m_new = true
	row.m_loaded = true
	row.m_valid = true
	_, has := this.m_new_rows[Account]
	if has {
		log.Error("已经存在 %v", Account)
		return nil
	}
	this.m_new_rows[Account] = row
	atomic.AddInt32(&this.m_gc_n, 1)
	return row
}
func (this *dbAccountTable) RemoveRow(Account string) {
	this.m_lock.UnSafeLock("dbAccountTable.RemoveRow")
	defer this.m_lock.UnSafeUnlock()
	row := this.m_rows[Account]
	if row != nil {
		row.m_remove = true
		delete(this.m_rows, Account)
		rm_row := this.m_removed_rows[Account]
		if rm_row != nil {
			log.Error("rows and removed rows both has %v", Account)
		}
		this.m_removed_rows[Account] = row
		_, has_new := this.m_new_rows[Account]
		if has_new {
			delete(this.m_new_rows, Account)
			log.Error("rows and new_rows both has %v", Account)
		}
	} else {
		row = this.m_removed_rows[Account]
		if row == nil {
			_, has_new := this.m_new_rows[Account]
			if has_new {
				delete(this.m_new_rows, Account)
			} else {
				log.Error("row not exist %v", Account)
			}
		} else {
			log.Error("already removed %v", Account)
			_, has_new := this.m_new_rows[Account]
			if has_new {
				delete(this.m_new_rows, Account)
				log.Error("removed rows and new_rows both has %v", Account)
			}
		}
	}
}
func (this *dbAccountTable) GetRow(Account string) (row *dbAccountRow) {
	this.m_lock.UnSafeRLock("dbAccountTable.GetRow")
	defer this.m_lock.UnSafeRUnlock()
	row = this.m_rows[Account]
	if row == nil {
		row = this.m_new_rows[Account]
	}
	return row
}

// ----------------------------------------------------------------------------

type dbAccountsManager struct {
	cur_table       *dbAccountTable
	cur_idx_max     int32
	next_table      *dbAccountTable
	gete_table_lock *sync.RWMutex
	dbc             *AccountDBC

	pid2acc      map[int32]string
	acc2pid      map[string]int32
	acc2pid_lock *sync.RWMutex

	cur_pass_id          int32
	id2passaccounts      map[int32]*dbAccountTable
	id2passaccounts_lock *sync.RWMutex

	playeridxmax_row *dbPlayerIdMaxRow

	b_gen_next      bool
	b_gen_next_lock *sync.RWMutex
}

func (this *dbAccountsManager) Init(tmp_dbc *AccountDBC) (err error) {
	this.dbc = tmp_dbc
	this.cur_table = new_dbAccountTable(this.dbc)
	this.next_table = new_dbAccountTable(this.dbc)

	this.pid2acc = make(map[int32]string)
	this.acc2pid = make(map[string]int32)
	this.acc2pid_lock = &sync.RWMutex{}

	this.id2passaccounts = make(map[int32]*dbAccountTable)
	this.id2passaccounts_lock = &sync.RWMutex{}

	this.b_gen_next_lock = &sync.RWMutex{}

	return nil
}

func (this *dbAccountsManager) PreLoad(dbc *AccountDBC, cur_max_pid int32) error {

	this.playeridxmax_row = this.dbc.PlayerIdMax.GetRow()

	cur_max_pid--
	if cur_max_pid < 0 {
		cur_max_pid = 0
	}
	cur_table_count := cur_max_pid/DEFAULT_PLAYER_TABLE_COUNT + 1
	var err error
	for idx := int32(1); idx <= cur_table_count; idx++ {

		err = this.cur_table.Init("Accounts" + strconv.Itoa(int(idx)))
		if nil != err {
			log.Error("dbAccountsManager Init %s failed %s", this.cur_table.m_table_name, err.Error())
			return err
		}

		err = this.cur_table.Preload()
		if nil != err {
			log.Error("dbAccountsManager Preload %s failed %s", this.cur_table.m_table_name, err.Error())
			return err
		}

		for tmp_acc, tmp_mp_val := range this.cur_table.m_rows {
			this.acc2pid[tmp_acc] = tmp_mp_val.GetPlayerId()
			this.pid2acc[tmp_mp_val.GetPlayerId()] = tmp_acc
		}
	}

	this.cur_idx_max = DEFAULT_PLAYER_TABLE_COUNT * cur_table_count

	err = this.next_table.Init("Accounts" + strconv.Itoa(int(cur_table_count+1)))
	if nil != err {
		log.Error("dbAccountsManager next_table Init failed !")
		return err
	}

	err = this.next_table.Preload()
	if nil != err {
		log.Error("dbAccountsManager next_table Preload failed !")
		return err
	}

	return nil
}

func (this *dbAccountsManager) add_to_pass_tables(tmp_table *dbAccountTable) {
	this.id2passaccounts_lock.Lock()
	defer this.id2passaccounts_lock.Unlock()

	this.cur_pass_id++
	this.id2passaccounts[this.cur_pass_id] = tmp_table

	return
}

func (this *dbAccountsManager) pop_all_pass_tables() (ret_tables map[int32]*dbAccountTable) {
	this.id2passaccounts_lock.Lock()
	defer this.id2passaccounts_lock.Unlock()
	if len(this.id2passaccounts) > 0 {
		ret_tables = this.id2passaccounts
		this.id2passaccounts = make(map[int32]*dbAccountTable)
		this.cur_pass_id = 0
	}

	return
}

func (this *dbAccountsManager) GetCurAccountPid(acc string) int32 {
	this.acc2pid_lock.Lock()
	defer this.acc2pid_lock.Unlock()

	return this.acc2pid[acc]
}

func (this *dbAccountsManager) set_next_table(tmp_table *dbAccountTable) {
	this.acc2pid_lock.Lock()
	defer this.acc2pid_lock.Unlock()

	this.next_table = tmp_table
}

func (this *dbAccountsManager) try_lock_gen_next_table() bool {
	this.b_gen_next_lock.Lock()
	defer this.b_gen_next_lock.Unlock()

	if !this.b_gen_next {
		this.b_gen_next = true
	}

	return true
}

func (this *dbAccountsManager) set_gen_next_table(val bool) {
	this.b_gen_next_lock.Lock()
	defer this.b_gen_next_lock.Unlock()

	this.b_gen_next = val
}

func (this *dbAccountsManager) async_gen_new_next_table(cur_max_pid int32) {
	if !this.try_lock_gen_next_table() {
		return
	}

	defer this.set_gen_next_table(false)

	tmp_table := new_dbAccountTable(this.dbc)
	cur_max_pid--
	if cur_max_pid < 0 {
		cur_max_pid = 0
	}

	new_next_table_idx := cur_max_pid/DEFAULT_PLAYER_TABLE_COUNT + 2
	err := tmp_table.Init("Accounts" + strconv.Itoa(int(new_next_table_idx)))
	if nil != err {
		log.Error("dbAccountsManager gen_new_next_table Init failed !")
		return
	}

	err = tmp_table.Preload()
	if nil != err {
		log.Error("dbAccountsManager gen_new_next_table Preload failed !")
		return
	}

	this.next_table = tmp_table

	return
}

func (this *dbAccountsManager) gen_new_next_table(cur_max_pid int32) {
	if !this.try_lock_gen_next_table() {
		return
	}

	defer this.set_gen_next_table(false)

	tmp_table := new_dbAccountTable(this.dbc)
	cur_max_pid--
	if cur_max_pid < 0 {
		cur_max_pid = 0
	}

	new_next_table_idx := cur_max_pid/DEFAULT_PLAYER_TABLE_COUNT + 2
	err := tmp_table.Init("Accounts" + strconv.Itoa(int(new_next_table_idx)))
	if nil != err {
		log.Error("dbAccountsManager gen_new_next_table Init failed !")
		return
	}

	err = tmp_table.Preload()
	if nil != err {
		log.Error("dbAccountsManager gen_new_next_table Preload failed !")
		return
	}

	this.set_next_table(tmp_table)

	return
}

func (this *dbAccountsManager) GetAccByPid(pid int32) string {
	this.acc2pid_lock.RLock()
	defer this.acc2pid_lock.RUnlock()

	acc, ok := this.pid2acc[pid]
	if !ok {
		return ""
	}

	return acc
}

func (this *dbAccountsManager) TryGetAccountPid(acc string) int32 {
	this.acc2pid_lock.Lock()
	defer this.acc2pid_lock.Unlock()

	cur_pid := this.acc2pid[acc]
	if cur_pid <= 0 {
		cur_pid = this.playeridxmax_row.GetPlayerIdMax() + 1
	} else {
		return cur_pid
	}

	if cur_pid > this.cur_idx_max {
		if nil == this.next_table {
			return -1
		}

		this.cur_idx_max += DEFAULT_PLAYER_TABLE_COUNT
		this.add_to_pass_tables(this.cur_table)
		this.cur_table = this.next_table
		this.next_table = nil

		this.async_gen_new_next_table(cur_pid)
		//this.gen_new_next_table(cur_pid)
	}

	tmp_row := this.cur_table.AddRow(acc)
	if nil == tmp_row {
		return -1
	}

	this.playeridxmax_row.IncPlayerIdMax(1)
	tmp_row.SetCreateTime(int32(time.Now().Unix()))
	tmp_row.SetPlayerId(cur_pid)
	this.acc2pid[acc] = cur_pid

	return cur_pid
}

func (this *dbAccountsManager) Save(quick bool) (err error) {
	pass_tables := this.pop_all_pass_tables()
	if nil != pass_tables {
		for _, tmp_table := range pass_tables {
			if nil == tmp_table {
				continue
			}
			tmp_table.Save(quick)
		}
	}

	this.cur_table.Save(quick)

	return
}

// ----------------------------------------------------------------------------
