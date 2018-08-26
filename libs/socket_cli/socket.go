package socket

import (
	"bytes"
	"compress/flate"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"ih_server/libs/log"
	"ih_server/libs/timer"
	"io"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"ih_server/third_party/code.google.com.protobuf/proto"
)

type E_DISCONNECT_REASON int32

const (
	cRECV_LEN        = 4096
	cMSG_BUFFER_LEN  = 8192
	cRECV_BUFFER_LEN = 4096
	cMSG_SEND_LEN    = 32768
	MSG_HEAD_LEN     = 6 // 整个消息头长度
	MSG_HEAD_SUB_LEN = 3 // 消息头除去长度之后的长度
	MSG_TAIL_LEN     = 2

	E_DISCONNECT_REASON_NONE                    E_DISCONNECT_REASON = 0
	E_DISCONNECT_REASON_INTERNAL_ERROR          E_DISCONNECT_REASON = 1
	E_DISCONNECT_REASON_SERVER_SHUTDOWN         E_DISCONNECT_REASON = 2
	E_DISCONNECT_REASON_NET_ERROR               E_DISCONNECT_REASON = 3
	E_DISCONNECT_REASON_OTHER_PLACE_LOGIN       E_DISCONNECT_REASON = 4
	E_DISCONNECT_REASON_PACKET_MALFORMED        E_DISCONNECT_REASON = 11
	E_DISCONNECT_REASON_CLIENT_DISCONNECT       E_DISCONNECT_REASON = 16
	E_DISCONNECT_REASON_PACKET_HANDLE_EXCEPTION E_DISCONNECT_REASON = 19
	E_DISCONNECT_REASON_FORCE_CLOSED_CLIENT     E_DISCONNECT_REASON = 20
)

type MessageItem struct {
	type_id uint16
	data    []byte
}

type RecentGroupInfo struct {
	RecentGroups []*MessageGroup
	CurGroupIdx  int32
}

type MessageGroup struct {
	length int32
	seq    uint8
	first  *MessageItem
	last   *MessageItem
}

type TcpConn struct {
	addr              string
	node              *Node
	c                 *net.TCPConn
	is_server         bool
	closing           bool
	closed            bool
	self_closing      bool
	recv_chan         chan []byte
	disc_chan         chan E_DISCONNECT_REASON
	send_chan         chan *MessageItem
	send_chan_closed  bool
	send_lock         *sync.Mutex
	handing           bool
	send_group        *MessageGroup
	start_time        time.Time
	last_message_time time.Time
	bytes_sended      int64
	bytes_recved      int64

	w_msg_hd []byte

	// tls
	tls_c net.Conn

	T             int64
	I             interface{}
	State         interface{}
	RecvSeq       int32
	LastSendGroup *MessageGroup
}

func new_conn(node *Node, conn *net.TCPConn, is_server bool) *TcpConn {
	c := TcpConn{}
	c.node = node
	c.c = conn
	c.is_server = is_server
	c.send_chan = make(chan *MessageItem, 256)
	c.recv_chan = make(chan []byte, 256)
	c.disc_chan = make(chan E_DISCONNECT_REASON, 128)
	c.addr = conn.RemoteAddr().String()
	c.send_lock = &sync.Mutex{}
	c.start_time = time.Now()
	c.w_msg_hd = make([]byte, 7)

	return &c
}

func tls_new_conn(node *Node, conn net.Conn, is_server bool) *TcpConn {
	c := TcpConn{}
	c.node = node
	c.tls_c = conn
	c.is_server = is_server
	c.send_chan = make(chan *MessageItem, 256)
	c.recv_chan = make(chan []byte, 256)
	c.disc_chan = make(chan E_DISCONNECT_REASON, 128)
	c.addr = conn.RemoteAddr().String()
	c.send_lock = &sync.Mutex{}
	c.start_time = time.Now()
	c.w_msg_hd = make([]byte, 7)

	return &c
}

func (this *TcpConn) GetAddr() (addr string) {
	return this.addr
}

func (this *TcpConn) IsServer() (ok bool) {
	return this.is_server
}

func (this *TcpConn) push_timeout(r bool, w bool) {
	c := this.GetConn()
	if c == nil {
		return
	}

	if r {
		if this.node.recv_timeout > 0 {
			c.SetReadDeadline(time.Now().Add(this.node.recv_timeout))
		}
	}
	if w {
		if this.node.send_timeout > 0 {
			c.SetWriteDeadline(time.Now().Add(this.node.send_timeout))
		}
	}
}

func (this *TcpConn) err(err error, desc string) {
	if !strings.Contains(err.Error(), "use of closed network connection") &&
		!strings.Contains(err.Error(), "An existing connection was forcibly closed by the remote host") &&
		!strings.Contains(err.Error(), "An established connection was aborted by the software in your host machine") &&
		!strings.Contains(err.Error(), "connected party did not properly respond after a period of time") {
		log.Error("[%v][玩家%v][%v][%v][%v]", this.addr, this.T, err, desc, reflect.TypeOf(err))
	}
	log.Trace("[%v][玩家%v][%v][%v][%v]", this.addr, this.T, err, desc, reflect.TypeOf(err))
}

func (this *TcpConn) release() {
	defer func() {
		this.closed = true
	}()

	c := this.GetConn()
	if c == nil {
		return
	}

	defer this.node.remove_conn(this)

	c.Close()
	close(this.disc_chan)
	close(this.recv_chan)
	if !this.send_chan_closed {
		this.send_chan_closed = true
		close(this.send_chan)
	}

	log.Trace("[%v][玩家%v]连接已释放 <总共发送 %v 字节><总共接收 %v 字节>", this.addr, this.T, this.bytes_sended, this.bytes_recved)
}

func (this *TcpConn) event_disc(reason E_DISCONNECT_REASON) {
	defer func() {
		if err := recover(); err != nil {
			//nothing
		}
	}()

	if this.closed {
		return
	}

	log.Trace("event_disc reason %v", reason)

	this.closing = true
	if !this.send_chan_closed {
		close(this.send_chan)
		this.send_chan_closed = true
	}
	this.disc_chan <- reason
}

func (this *TcpConn) Send(msg proto.Message) {
	if msg == nil {
		log.Error("[%v][玩家%v] 消息为空", this.addr, this.T)
		this.event_disc(E_DISCONNECT_REASON_INTERNAL_ERROR)
		return
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		log.Error("[%v][玩家%v]序列化数据失败 %v %v", this.addr, this.T, err, msg.String())
		this.event_disc(E_DISCONNECT_REASON_INTERNAL_ERROR)
		return
	}

	length := int32(len(data))
	this.bytes_sended += int64(length)

	log.Debug("发送[%v][玩家%v][%v][%v][%v][%v字节][%v字节] %v ",
		this.addr, this.T, length, this.bytes_sended, "{"+msg.MessageTypeName()+":{"+msg.String()+"}}")

	this.send_data(msg.MessageTypeId(), data)
}

func (this *TcpConn) send_data(type_id uint16, data []byte) {
	defer func() {
		if err := recover(); err != nil {
			log.Error("[%v][玩家%v]%v send_data", this.addr, this.T, this.node.get_message_name(type_id))
			log.Stack(err)
		}
	}()

	this.send_lock.Lock()
	defer this.send_lock.Unlock()

	if this.closing || this.closed {
		return
	}

	length := int32(len(data)) + 7
	if length > cMSG_SEND_LEN-4 {
		log.Error("msg length too long %v %v", this.node.get_message_name(type_id), length)
		return
	}

	item := &MessageItem{}
	item.type_id = type_id
	item.data = data
	this.event_send(item)
}

func (this *TcpConn) event_send(item *MessageItem) {
	defer func() {
		if err := recover(); err != nil {
			//nothing
		}
	}()

	if this.closed || this.closing {
		return
	}

	this.send_chan <- item
}

type Handler func(c *TcpConn, m proto.Message)

type AckHandler func(c *TcpConn, seq uint8)

type handler_info struct {
	t reflect.Type
	h Handler
}

func (this *TcpConn) send_loop() {
	b_remote_closed := false
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
		}

		log.Trace("send loop quit %v", this.addr)
		if !b_remote_closed {
			this.event_disc(E_DISCONNECT_REASON_NET_ERROR)
		} else {
			this.event_disc(E_DISCONNECT_REASON_FORCE_CLOSED_CLIENT)
		}
	}()

	c := this.GetConn()
	if c == nil {
		log.Trace("net connection disconnected, get conn failed")
		return
	}

	var h [MSG_HEAD_LEN]byte
	for {
		select {
		case item, ok := <-this.send_chan:
			{
				if this.closing {
					log.Trace("send loop quit this.closing 1 %v", this.addr)
					return
				}

				if !ok {
					log.Trace("send loop quit chan not ok %v", this.addr)
					return
				}

				this.push_timeout(false, true)
				if false { // d.length > 2048
					var b bytes.Buffer
					w, err := flate.NewWriter(&b, 1)
					if err != nil {
						this.err(err, "flate.NewWriter failed")
						return
					}
					_, err = w.Write(item.data)
					if err != nil {
						this.err(err, "write_msgs flate.Write failed")
						return
					}
					err = w.Close()
					if err != nil {
						this.err(err, "flate.Close failed")
						return
					}
					data := b.Bytes()

					length := len(data) + MSG_TAIL_LEN
					h[0] = byte(length)
					h[1] = byte(length >> 8)
					h[2] = byte(length >> 16)
					h[3] = 1
					h[4] = byte(item.type_id)
					h[5] = byte(item.type_id >> 8)
					_, err = c.Write(h[:])
					if err != nil {
						b_remote_closed = true
						this.err(err, "write compress header failed")
						return
					}
					_, err = c.Write(data)
					if err != nil {
						b_remote_closed = true
						this.err(err, "write compress body failed")
						return
					}

					/*
						_, err = c.Write(msg_tail[:])
						if nil != err {
							b_remote_closed = true
							this.err(err, "write tail failed")
							return
						}
					*/
				} else {
					length := len(item.data) + MSG_TAIL_LEN

					h[0] = byte(length)
					h[1] = byte(length >> 8)
					h[2] = byte(length >> 16)
					h[3] = 0
					h[4] = byte(item.type_id)
					h[5] = byte(item.type_id >> 8)

					_, err := c.Write(h[:])
					if err != nil {
						b_remote_closed = true
						this.err(err, "write header failed")
						return
					}

					_, err = c.Write(item.data)
					if err != nil {
						b_remote_closed = true
						this.err(err, "write body failed")
						return
					}

					/*
						_, err = c.Write(msg_tail[:])
						if nil != err {
							b_remote_closed = true
							this.err(err, "write tail failed")
							return
						}
					*/
				}

				if this.closing {
					log.Trace("Send Loop Quit this.closing 2 !")
					return
				}

				time.Sleep(time.Millisecond * 5)
			}
		}
	}
}

func (this *TcpConn) event_recv(data []byte) {
	defer func() {
		if err := recover(); err != nil {
			//nothing
		}
	}()

	if this.closed || this.closing {
		return
	}

	this.recv_chan <- data
}

func (this *TcpConn) on_recv(d []byte) {
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)
			this.event_disc(E_DISCONNECT_REASON_PACKET_HANDLE_EXCEPTION)
		}
	}()

	this.last_message_time = time.Now()

	length := int32(len(d))
	this.bytes_recved += int64(length)
	if length < MSG_HEAD_SUB_LEN {
		log.Error("[%v][玩家%v]长度小于MSG_HEAD_SUB_LEN", this.addr, this.T)
		this.event_disc(E_DISCONNECT_REASON_PACKET_MALFORMED)
		return
	}

	ctrl := d[0]
	compressed := (ctrl&0x01 != 0)

	type_id := uint16(0)
	type_id += uint16(d[1]) + (uint16(d[2]) << 8)

	var data []byte
	if compressed {
		b := new(bytes.Buffer)
		reader := bytes.NewReader(d[MSG_HEAD_SUB_LEN:length])
		r := flate.NewReader(reader)
		_, err := io.Copy(b, r)
		if err != nil {
			defer r.Close()
			log.Error("[%v][玩家%v]decompress copy failed %v", this.addr, this.T, err)
			this.event_disc(E_DISCONNECT_REASON_PACKET_MALFORMED)
			return
		}
		err = r.Close()
		if err != nil {
			log.Error("[%v][玩家%v]flate Close failed %v", this.addr, this.T, err)
			this.event_disc(E_DISCONNECT_REASON_PACKET_MALFORMED)
			return
		}
		data = b.Bytes()
	} else {
		data = d[MSG_HEAD_SUB_LEN:length]
	}

	//log.Error("OnRecv msg %v %v %v %v", type_id, d, data)

	p := this.node.handler_map[type_id]
	if p.t == nil {
		log.Error("[%v][玩家%v]消息句柄为空 %v", this.addr, this.T, this.node.get_message_name(type_id))
		//this.event_disc(E_DISCONNECT_REASON_PACKET_MALFORMED)
		return
	}

	msg := reflect.New(p.t).Interface().(proto.Message)
	err := proto.Unmarshal(data[:], msg)
	if err != nil {
		log.Error("[%v][玩家%v]反序列化失败 %v data:%v err%s msgtypename(%s)", this.addr, this.T, this.node.get_message_name(type_id), data, err.Error(), msg.MessageTypeName())
		this.event_disc(E_DISCONNECT_REASON_PACKET_MALFORMED)
		return
	}

	log.Debug("接收[%v][玩家%v][%v][%v字节][%v字节]%v",
		this.addr,
		this.T,
		len(d),
		this.bytes_recved,
		"{"+msg.MessageTypeName()+":{"+msg.String()+"}}")

	begin := time.Now()

	if this.is_server {
		this.handing = true
		p.h(this, msg)
		this.handing = false
	} else {
		p.h(this, msg)
	}

	time_cost := time.Now().Sub(begin).Seconds()
	if time_cost > 3 {
		log.Trace("[时间过长 %v][%v][玩家%v]消息%v", time_cost, this.addr, this.T, this.node.get_message_name(type_id))
	} else {
		//log.Trace("[时间%v][%v][玩家%v]消息%v", time_cost, this.addr, this.T, this.node.get_message_name(type_id))
	}

	return
}

func (this *TcpConn) recv_loop() {
	bforeclosed := false
	reason := E_DISCONNECT_REASON_NET_ERROR
	defer func() {
		if err := recover(); err != nil {
			log.Stack(err)

		}

		log.Trace("recv loop quit %v", this.addr)

		if bforeclosed {
			this.event_disc(E_DISCONNECT_REASON_FORCE_CLOSED_CLIENT)
		} else {
			this.event_disc(reason)
		}

	}()

	c := this.GetConn()
	if c == nil {
		return
	}

	l := uint32(0)
	extra_len := int(MSG_HEAD_SUB_LEN - MSG_TAIL_LEN)
	mb := bytes.NewBuffer(make([]byte, 0, cMSG_BUFFER_LEN))
	rb := make([]byte, cRECV_BUFFER_LEN)
	for {
		this.push_timeout(true, false)
		if this.closing {
			bforeclosed = true
			log.Trace("recv loop quit when closing 1")
			break
		}

		rl, err := c.Read(rb)

		if this.closing {
			bforeclosed = true
			log.Trace("recv loop quit when closing 2")
			break
		}

		if io.EOF == err {
			log.Error("[%v][玩家%v]另一端关闭了连接", this.addr, this.T)
			reason = E_DISCONNECT_REASON_CLIENT_DISCONNECT
			return
		}

		if nil != err {
			net_err := err.(net.Error)
			if !net_err.Timeout() {
				bforeclosed = true
				log.Error("recv_loop Read error(%s) !", err.Error())
				//this.err(err, "读取数据失败")
				return
			}
		}

		//log.Info("收到客户端数据%d: %v", rl, rb[:rl])
		mb.Write(rb[:rl])

		if 0 == l && mb.Len() >= 3 {
			b, err := mb.ReadByte()
			if nil != err {
				this.err(err, "读取长度失败1")
				return
			}
			l += uint32(b)

			b, err = mb.ReadByte()
			if nil != err {
				this.err(err, "读取长度失败2")
				return
			}
			l += uint32(b) << 8

			b, err = mb.ReadByte()
			if nil != err {
				this.err(err, "读取长度失败3")
				return
			}
			l += uint32(b) << 16

			if l > cRECV_LEN || l < 2 {
				this.err(errors.New("消息长度不合法 "+strconv.Itoa(int(l))), "")
				return
			}

			//log.Info("读取到消息长度", l)
		}

		if l > 0 && mb.Len() >= int(l)+extra_len {

			n := mb.Next(int(l) + extra_len)
			d := make([]byte, len(n))
			for i := int(0); i < len(n); i++ {
				d[i] = n[i]
			}

			if this.closing || this.closed {
				log.Trace("recv loop quit when closing")
				return
			}

			this.event_recv(d)

			l = 0

			time.Sleep(time.Millisecond * 100)
		} else {
			//log.Info("需要更多的数据", l, mb.Len(), int(l)+extra_len)
			//break
		}
	}
}

func (this *TcpConn) main_loop() {
	var disc_reason E_DISCONNECT_REASON
	defer func() {
		defer func() {
			if err := recover(); err != nil {
				log.Stack(err)
			}
		}()

		defer this.release()

		if err := recover(); err != nil {
			log.Stack(err)
		}

		log.Trace("main_loop quit %v %v", this.T, disc_reason)

		if this.node.callback != nil {
			this.node.callback.OnDisconnect(this, disc_reason)
		}

		log.Trace("main loop quit completed")
	}()

	t := timer.NewTickTimer(this.node.update_interval_ms)
	t.Start()
	defer t.Stop()

	for {
		select {
		case d, ok := <-this.recv_chan:
			{
				if !ok {
					disc_reason = E_DISCONNECT_REASON_INTERNAL_ERROR
					return
				}

				this.on_recv(d)

				if this.self_closing {
					begin := time.Now()
					for {
						select {
						case reason, ok := <-this.disc_chan:
							{
								log.Trace("self disc chan %v", ok)
								if !ok {
									disc_reason = E_DISCONNECT_REASON_INTERNAL_ERROR
									return
								}

								disc_reason = reason

								return
							}
						default:
							{
								//nothing
							}
						}

						time.Sleep(100)
						if dur := time.Now().Sub(begin).Seconds(); dur > 3 {
							log.Trace("wait client close timeout %v", dur)
							return
						}
					}
				}
			}
		case reason, ok := <-this.disc_chan:
			{
				if !ok {
					disc_reason = E_DISCONNECT_REASON_INTERNAL_ERROR
					return
				}

				disc_reason = reason

				return
			}
		case d, ok := <-t.Chan:
			{
				if !ok {
					disc_reason = E_DISCONNECT_REASON_INTERNAL_ERROR
					return
				}

				if this.node.callback != nil {
					this.node.callback.OnUpdate(this, d)
				}
			}
		}
	}
}

func (this *TcpConn) Close(reason E_DISCONNECT_REASON) {
	if this == nil {
		return
	}

	log.Trace("[%v][玩家%v]关闭连接", this.addr, this.T)

	if this.handing {
		log.Trace("self closing")
		this.self_closing = true
		return
	} else {
		this.event_disc(reason)
	}

	begin := time.Now()
	for {
		if dur := time.Now().Sub(begin).Seconds(); dur > 2 {
			log.Trace("等待断开超时 %v", dur)
			break
		}

		if this.closing {
			log.Trace("wait remote close closing break")
			break
		}

		if this.closed {
			log.Trace("wait remote close closed break")
			return
		}

		time.Sleep(time.Millisecond * 100)
	}

	if !this.closing && !this.closed {
		log.Trace("关闭连接 event_disc")
		this.event_disc(reason)
	}

	begin = time.Now()
	logged := false
	for {
		if this.closed {
			return
		}

		time.Sleep(time.Millisecond * 100)

		if dur := time.Now().Sub(begin).Seconds(); dur > 5 {
			if !logged {
				logged = true
				log.Error("关闭连接超时 %v %v %v", this.addr, this.T, dur)
			}
		}
	}
}

func (this *TcpConn) IsClosing() (closing bool) {
	return this.closing
}

func (this *TcpConn) GetStartTime() (t time.Time) {
	return this.start_time
}

func (this *TcpConn) GetLastMessageTime() (t time.Time) {
	return this.last_message_time
}

func (this *TcpConn) GetConn() (conn net.Conn) {
	if this.c != nil {
		conn = this.c
	} else {
		conn = this.tls_c
	}
	return
}

type ICallback interface {
	OnAccept(c *TcpConn)
	OnConnect(c *TcpConn)
	OnUpdate(c *TcpConn, t timer.TickTime)
	OnDisconnect(c *TcpConn, reason E_DISCONNECT_REASON)
}

type Node struct {
	addr               *net.TCPAddr
	max_conn           int32
	callback           ICallback
	recv_timeout       time.Duration
	send_timeout       time.Duration
	update_interval_ms int32
	listener           *net.TCPListener
	client             *TcpConn
	quit               bool
	type_names         map[uint16]string
	handler_map        map[uint16]handler_info
	ack_handler        AckHandler
	conn_map           map[*TcpConn]int32
	conn_map_lock      *sync.RWMutex
	shutdown_lock      *sync.Mutex
	initialized        bool

	// tls
	tls_listener net.Listener
	tls_config   *tls.Config

	I interface{}
}

func NewNode(cb ICallback, recv_timeout time.Duration, send_timeout time.Duration, update_interval_ms int32, type_names map[uint16]string) (this *Node) {
	this = &Node{}
	this.callback = cb
	this.recv_timeout = recv_timeout * time.Millisecond
	this.send_timeout = send_timeout * time.Millisecond
	this.update_interval_ms = update_interval_ms
	this.handler_map = make(map[uint16]handler_info)
	this.conn_map = make(map[*TcpConn]int32)
	this.conn_map_lock = &sync.RWMutex{}
	this.shutdown_lock = &sync.Mutex{}
	this.initialized = true
	this.type_names = make(map[uint16]string)
	for i, v := range type_names {
		this.type_names[i] = v
	}

	return this
}

func (this *Node) UseTls(cert_file string, key_file string) (err error) {
	cert, err := tls.LoadX509KeyPair(cert_file, key_file)
	if err != nil {
		return
	}

	this.tls_config = &tls.Config{
		//RootCAs: pool,
		//InsecureSkipVerify: true,
		//ClientAuth: tls.RequireAndVerifyClientCert,
		Certificates:             []tls.Certificate{cert},
		CipherSuites:             []uint16{tls.TLS_RSA_WITH_RC4_128_SHA},
		PreferServerCipherSuites: true,
	}

	now := time.Now()
	this.tls_config.Time = func() time.Time { return now }
	this.tls_config.Rand = rand.Reader

	return
}

func (this *Node) UseTlsClient() {
	this.tls_config = &tls.Config{
		InsecureSkipVerify: true,
	}

	now := time.Now()
	this.tls_config.Time = func() time.Time { return now }
	this.tls_config.Rand = rand.Reader
}

func (this *Node) add_conn(c *TcpConn) {
	this.conn_map_lock.Lock()
	defer this.conn_map_lock.Unlock()

	this.conn_map[c] = 0
}

func (this *Node) remove_conn(c *TcpConn) {
	this.conn_map_lock.Lock()
	defer this.conn_map_lock.Unlock()

	delete(this.conn_map, c)
}

func (this *Node) ConnCount() (n int32) {
	this.conn_map_lock.RLock()
	defer this.conn_map_lock.RUnlock()

	return int32(len(this.conn_map))
}

func (this *Node) get_all_conn() (conn_map map[*TcpConn]int32) {
	this.conn_map_lock.RLock()
	defer this.conn_map_lock.RUnlock()

	conn_map = make(map[*TcpConn]int32)
	for i, v := range this.conn_map {
		conn_map[i] = v
	}

	return
}

func (this *Node) SetHandler(type_id uint16, typ reflect.Type, h Handler) {
	_, has := this.handler_map[type_id]
	if has {
		log.Error("[%v]消息处理函数已设置,将被替换 %v %v", this.addr, type_id, this.get_message_name(type_id))
	}
	this.handler_map[type_id] = handler_info{typ, h}
}

func (this *Node) GetHandler(type_id uint16) (h Handler) {
	hi, has := this.handler_map[type_id]
	if !has {
		return nil
	}

	return hi.h
}

func (this *Node) get_message_name(type_id uint16) (name string) {
	if this.type_names == nil {
		return ""
	}

	name, ok := this.type_names[type_id]
	if ok {
		return name
	}

	return ""
}

func (this *Node) SetAckHandler(h AckHandler) {
	if this.ack_handler != nil {
		log.Error("[%v]ack句柄已经设置,将被替换", this.addr)
	}
	this.ack_handler = h
}

func (this *Node) Listen(server_addr string, max_conn int32) (err error) {
	addr, err := net.ResolveTCPAddr("tcp", server_addr)
	if err != nil {
		return
	}

	this.addr = addr
	this.max_conn = max_conn

	if this.tls_config == nil {
		var l *net.TCPListener
		l, err = net.ListenTCP(this.addr.Network(), this.addr)
		if err != nil {
			return
		}
		this.listener = l
	} else {
		var tls_l net.Listener
		tls_l, err = tls.Listen("tcp", server_addr, this.tls_config)
		if err != nil {
			return
		}
		this.tls_listener = tls_l
	}

	var delay time.Duration
	var conn *net.TCPConn
	var tls_conn net.Conn
	log.Trace("[%v]服务器监听中", this.addr)
	for !this.quit {
		log.Trace("[%v]等待新的连接", this.addr)
		if this.tls_config == nil {
			conn, err = this.listener.AcceptTCP()
		} else {
			tls_conn, err = this.tls_listener.Accept()
		}
		if err != nil {
			if this.quit {
				return nil
			}
			if net_err, ok := err.(net.Error); ok && net_err.Temporary() {
				if delay == 0 {
					delay = 5 * time.Millisecond
				} else {
					delay *= 2
				}
				if max := 1 * time.Second; delay > max {
					delay = max
				}
				log.Trace("Accept error: %v; retrying in %v", err, delay)
				time.Sleep(delay)
				continue
			}
			return err
		}

		delay = 0

		if this.quit {
			break
		}

		if this.tls_config == nil {
			log.Trace("[%v]客户端连接成功 %v", this.addr, conn.RemoteAddr())
			if this.max_conn > 0 {
				if this.ConnCount() > this.max_conn {
					log.Trace("[%v]已达到最大连接数 %v", this.addr, this.max_conn)
					conn.Close()
					continue
				}
			}
		} else {
			log.Trace("[%v]客户端连接成功 %v", this.addr, tls_conn.RemoteAddr())
			if this.max_conn > 0 {
				if this.ConnCount() > this.max_conn {
					log.Trace("[%v]已达到最大连接数 %v", this.addr, this.max_conn)
					tls_conn.Close()
					continue
				}
			}
		}

		var c *TcpConn
		if this.tls_config == nil {
			conn.SetKeepAlive(true)
			conn.SetKeepAlivePeriod(time.Minute * 2)
			c = new_conn(this, conn, true)
		} else {
			c = tls_new_conn(this, tls_conn, true)
		}

		this.add_conn(c)

		go c.send_loop()
		go c.recv_loop()
		if this.callback != nil {
			this.callback.OnAccept(c)
		}
		go c.main_loop()
	}

	return nil
}

func (this *Node) Connect(server_addr string, timeout time.Duration) (err error) {
	addr, err := net.ResolveTCPAddr("tcp", server_addr)
	if err != nil {
		return
	}
	this.addr = addr
	this.send_timeout = timeout

	var conn *net.TCPConn
	var tls_conn net.Conn
	if this.tls_config == nil {
		conn, err = net.DialTCP(this.addr.Network(), nil, this.addr)
	} else {
		tls_conn, err = tls.Dial("tcp", server_addr, this.tls_config)
	}

	if err != nil {
		return
	}

	log.Trace("[%v]连接成功", server_addr)

	var c *TcpConn
	if this.tls_config == nil {
		c = new_conn(this, conn, false)
	} else {
		c = tls_new_conn(this, tls_conn, false)
	}

	c.I = this.I
	this.client = c
	go c.send_loop()
	go c.recv_loop()
	if this.callback != nil {
		this.callback.OnConnect(c)
	}
	go c.main_loop()

	return
}

func (this *Node) GetClient() (c *TcpConn) {
	return this.client
}

func (this *Node) Shutdown() {
	log.Trace("[%v]关闭网络服务", this.addr)
	if !this.initialized {
		return
	}

	this.shutdown_lock.Lock()
	defer this.shutdown_lock.Unlock()

	if this.quit {
		return
	}
	this.quit = true

	begin := time.Now()

	if this.tls_config == nil {
		if this.listener != nil {
			this.listener.Close()
		}
	} else {
		if this.tls_listener != nil {
			this.tls_listener.Close()
		}
	}

	conn_map := this.get_all_conn()
	log.Trace("[%v]总共 %v 个连接需要关闭", this.addr, len(conn_map))
	for k, _ := range conn_map {
		go k.Close(E_DISCONNECT_REASON_SERVER_SHUTDOWN)
	}

	for {
		conn_count := this.ConnCount()
		if conn_count == 0 {
			break
		}

		time.Sleep(time.Millisecond * 100)
	}

	log.Trace("[%v]关闭网络服务耗时 %v 秒", this.addr, time.Now().Sub(begin).Seconds())
}

func (this *Node) StopListen() {
	if this.tls_config == nil {
		if this.listener != nil {
			this.listener.Close()
		}
	} else {
		if this.tls_listener != nil {
			this.tls_listener.Close()
		}
	}
}
