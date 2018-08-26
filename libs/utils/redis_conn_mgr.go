package utils

type RedisConnMgr struct {
	conn_arr []*RedisConn
}

func (this *RedisConnMgr) Init(max_conn int) bool {
	if max_conn <= 0 {
		return false
	}
	this.conn_arr = make([]*RedisConn, max_conn)
	for i := 0; i < max_conn; i++ {
		this.conn_arr[i] = &RedisConn{}
	}
	return true
}

func (this *RedisConnMgr) InitWithLock(max_conn int) bool {
	if max_conn <= 0 {
		return false
	}
	this.conn_arr = make([]*RedisConn, max_conn)
	for i := 0; i < max_conn; i++ {
		this.conn_arr[i] = &RedisConn{}
		this.conn_arr[i].InitWithLock()
	}
	return true
}

func (this *RedisConnMgr) AllConnect(addr string) bool {
	b := false
	i := 0
	for ; i < len(this.conn_arr); i++ {
		if !this.conn_arr[i].Connect(addr) {
			b = true
			break
		}
	}
	if b {
		for n := 0; n <= i; n++ {
			this.conn_arr[n].Clear()
		}
	}
	return true
}

func (this *RedisConnMgr) GetConn(index int) *RedisConn {
	if this.conn_arr == nil || index < 0 || index >= len(this.conn_arr) {
		return nil
	}
	return this.conn_arr[index]
}

func (this *RedisConnMgr) DisconnectAll() {
	if this.conn_arr == nil {
		return
	}
	for i := 0; i < len(this.conn_arr); i++ {
		this.conn_arr[i].Close()
	}
}

func (this *RedisConnMgr) Clear() {
	this.DisconnectAll()
}
