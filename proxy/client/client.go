package client

import (
	"fmt"
	"miner-proxy/pkg"
	"miner-proxy/pkg/cache"
	"miner-proxy/proxy/protocol"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/segmentio/ksuid"
	"github.com/spf13/cast"
	"go.uber.org/atomic"
)

var (
	clients sync.Map
	// key=client id value=*ServerManage
	serverManage sync.Map
	localIPv4    = pkg.LocalIPv4s()
)

func InitServerManage(maxConn int, secretKey, serverAddress, clientId, pool string) error {
	s, err := NewServerManage(maxConn, secretKey, serverAddress, clientId, pool)
	if err != nil {
		return err
	}

	go func() {
		for {
			s.m.RLock()
			size := len(s.connIds)
			s.m.RUnlock()
			for i := 0; i < maxConn-size; i++ {
				server := s.NewServer(ksuid.New().String())
				if server == nil {
					pkg.Warn("connection to server failed")
				}
			}
			time.Sleep(time.Second * 5)
		}
	}()
	serverManage.Store(clientId, s)
	return nil
}

type ServerManage struct {
	secretKey, serverAddress, clientId, pool string
	maxConn                                  int
	m                                        sync.RWMutex
	conns                                    sync.Map
	connIds                                  []string
	index                                    *atomic.Int64
}

func NewServerManage(maxConn int, secretKey, serverAddress, clientId, pool string) (*ServerManage, error) {
	s := &ServerManage{
		secretKey: secretKey, serverAddress: serverAddress,
		maxConn: maxConn, index: atomic.NewInt64(0),
		clientId: clientId,
		pool:     pool,
	}
	for i := 0; i < maxConn; i++ {
		server := s.NewServer(ksuid.New().String())
		if server == nil {
			return nil, errors.New("connection to server error")
		}
	}
	return s, nil
}

func (s *ServerManage) DelServerConn(key string) {
	s.m.Lock()
	defer s.m.Unlock()
	s.conns.Delete(key)
	var conns []string
	for index, v := range s.connIds {
		if v == key {
			continue
		}
		conns = append(conns, s.connIds[index])
	}
	s.connIds = conns
	return
}

func (s *ServerManage) SetServerConn(key string, server *Server) {
	s.m.Lock()
	defer s.m.Unlock()
	s.conns.Store(key, server)
	s.connIds = append(s.connIds, key)
	return
}

func (s *ServerManage) GetServer() *Server {
	s.m.RLock()
	connSize := len(s.connIds)
	if connSize == 0 {
		s.m.RUnlock()
		return nil
	}
	index := s.index.Add(1) % int64(connSize)
	key := s.connIds[index]
	s.m.RUnlock()
	v, ok := s.conns.Load(key)
	if !ok {
		return nil
	}
	server := v.(*Server)
	if server == nil || server.close.Load() { // 连接
		s.DelServerConn(key)
		key = ksuid.New().String()
		if server = s.NewServer(key); server == nil {
			return nil
		}
		s.SetServerConn(key, server)
	}
	return server
}

func (s *ServerManage) NewServer(id string) *Server {
	conn, err := net.DialTimeout("tcp", s.serverAddress, time.Second*3)
	if err != nil {
		return nil
	}
	server := &Server{
		id:      id,
		address: s.serverAddress,
		conn:    conn,
		close:   atomic.NewBool(false),
	}

	fc := protocol.NewGoframeProtocol(s.secretKey, true, server.conn)
	var miners []string
	clients.Range(func(key, value interface{}) bool {
		miners = append(miners, cast.ToString(key))
		return true
	})
	req := protocol.Request{
		ClientId: s.clientId,
		Type:     protocol.INIT,
		Data:     []byte(fmt.Sprintf("%s|%s|%s", s.pool, strings.Join(miners, ","), localIPv4)),
	}

	data, _ := protocol.Decode2Byte(req)
	pkg.Debug("client -> server %s", req)
	if err := fc.WriteFrame(data); err != nil {
		return nil
	}

	go func(server *Server) {
		defer server.Close()
		defer s.DelServerConn(id)
		fc := protocol.NewGoframeProtocol(s.secretKey, true, server.conn)
		for !server.close.Load() {
			data, err := fc.ReadFrame()
			if err != nil {
				return
			}
			req, err := protocol.Encode2Request(data)
			if err != nil {
				return
			}
			pkg.Debug("client <- server %s", req)
			switch req.Type {
			case protocol.PING, protocol.PONG:
				var needClose []string
				for _, minerId := range strings.Split(string(req.Data), ",") {
					if minerId == "" {
						continue
					}
					if _, ok := clients.Load(minerId); !ok { // 发送删除
						needClose = append(needClose, minerId)
					}
				}

				req := protocol.Request{
					ClientId: s.clientId,
					Type:     protocol.PONG,
					Data:     []byte(strings.Join(needClose, ",")),
				}
				data, _ := protocol.Decode2Byte(req)
				if err := fc.WriteFrame(data); err != nil {
					return
				}
				continue
			case protocol.INIT:
				continue
			case protocol.CLOSE:
				for _, v := range pkg.String2Array(string(req.Data), ",") {
					value, ok := clients.Load(v)
					if !ok {
						continue
					}
					pkg.Debug("server send mandate close connection")
					value.(*Client).Close()
				}
			}
			v, ok := clients.Load(req.MinerId)
			if !ok {
				pkg.Debug("没有找到 %s miner id", req.MinerId)
				continue
			}
			v.(*Client).input <- req
		}
	}(server)

	s.conns.Store(id, server)

	s.m.RLock()
	defer s.m.RUnlock()
	s.connIds = append(s.connIds, id)
	return server
}

type Server struct {
	conn        net.Conn
	close       *atomic.Bool
	stop        sync.Once
	id, address string
}

func (s *Server) Close() {
	s.stop.Do(func() {
		s.close.Store(true)
		if s.conn != nil {
			_ = s.conn.Close()
		}
	})
}

type Client struct {
	ClientId string
	// id MinerId
	id, ip, serverAddress, secretKey, poolAddress string
	lconn                                         net.Conn
	input                                         chan protocol.Request
	closed                                        *atomic.Bool
	lastSendReq                                   protocol.Request
	ready                                         *atomic.Bool
	readyChan                                     chan struct{}
	stop                                          sync.Once
	seq                                           *atomic.Int64
}

func newClient(ip string, serverAddress string, secretKey string, poolAddress string, conn net.Conn, clientId string) {
	defer pkg.Recover(true)
	if strings.Contains(ip, "127.0.0.1") && localIPv4 != "" {
		ip = localIPv4
	}
	client := &Client{
		secretKey:     secretKey,
		serverAddress: serverAddress,
		ClientId:      clientId,
		ip:            ip,
		lconn:         conn,
		input:         make(chan protocol.Request),
		ready:         atomic.NewBool(true),
		readyChan:     make(chan struct{}),
		closed:        atomic.NewBool(false),
		id:            ksuid.New().String(),
		poolAddress:   poolAddress,
		seq:           atomic.NewInt64(0),
	}
	defer func() {
		client.Close()
	}()

	clients.Store(client.id, client)
	if err := client.Login(); err != nil {
		pkg.Warn("login to server failed %s", err)
		return
	}
	client.Run()
	return
}

func (c *Client) IsSend(req protocol.Request) bool {
	key := fmt.Sprintf("c_send_req:%d:%s:%s", req.Seq, req.Hash, req.ClientId)
	if _, ok := cache.Client.Get(key); ok {
		return true
	}
	return false
}

func (c *Client) SetSend(req protocol.Request) {
	key := fmt.Sprintf("c_send_req:%d:%s:%s", req.Seq, req.Hash, req.ClientId)
	cache.Client.SetDefault(key, "")
}

func (c *Client) Close() {
	c.stop.Do(func() {
		c.closed.Store(true)
		if c.lconn != nil {
			_ = c.lconn.Close()
		}
		clients.Delete(c.id)
	})
}

func (c *Client) SendToServer(req protocol.Request, maxTry int, secretKey string) error {
	value, ok := serverManage.Load(c.ClientId)
	if !ok {
		return errors.Errorf("not found %s server connection", c.ClientId)
	}
	sm := value.(*ServerManage)
	return pkg.Try(func() bool {
		s := sm.GetServer()
		if s == nil {
			s = sm.NewServer(ksuid.New().String())
		}
		if s == nil {
			pkg.Warn("没有server连接可用!也无法新建连接到server端, 检查网络是否畅通, 1S 后重试")
			time.Sleep(time.Second)
			return false
		}
		fc := protocol.NewGoframeProtocol(secretKey, true, s.conn)
		sendData, err := protocol.Decode2Byte(req)
		if err != nil {
			time.Sleep(time.Second)
			return false
		}
		pkg.Debug("client -> server %s", req)
		if err := fc.WriteFrame(sendData); err != nil {
			return false
		}
		return true
	}, maxTry)
}

func (c *Client) SendCloseToServer(secretKey string) {
	req := &protocol.Request{
		ClientId: c.ClientId,
		MinerId:  c.id,
		Type:     protocol.CLOSE,
	}
	_ = c.SendToServer(req.End(), 1, secretKey)
	pkg.Debug("client -> server %s", req)
}

func (c *Client) SendDataToServer(data []byte, secretKey string) error {
	req := protocol.Request{
		MinerId:  c.id,
		ClientId: c.ClientId,
		Type:     protocol.DATA,
		Data:     data,
		Seq:      c.seq.Inc(),
	}

	c.SetWait(req)
	return c.SendToServer(req, 10, secretKey)
}

func (c *Client) Login() error {
	req := protocol.Request{
		ClientId: c.ClientId,
		MinerId:  c.id,
		Type:     protocol.LOGIN,
		Data: protocol.DecodeLoginRequest2Byte(protocol.LoginRequest{
			PoolAddress: c.poolAddress,
			MinerIp:     c.ip,
		}),
	}
	c.SetWait(req)
	return c.SendToServer(req, 3, c.secretKey)
}

func (c *Client) readServerData() {
	defer c.Close()
	t := time.NewTicker(time.Second * 5)
	defer t.Stop()
	for !c.closed.Load() {
		select {
		case req, ok := <-c.input:
			if !ok {
				pkg.Debug("服务端关闭了连接")
				return
			}
			switch req.Type {
			case protocol.ERROR, protocol.CLOSE:
				pkg.Debug("server send mandate close connection")
				return
			case protocol.LOGIN, protocol.ACK:
				c.SetReady()
				continue
			}
			if !c.IsSend(req) {
				if _, err := c.lconn.Write(req.Data); err != nil {
					pkg.Warn("write miner error: %s. close connection", err)
					return
				}
				c.SetSend(req)
			}

			if err := c.SendToServer(protocol.Request{
				ClientId: c.ClientId,
				MinerId:  c.id,
				Type:     protocol.ACK,
			}, 2, c.secretKey); err != nil {
				pkg.Error("send ACK to server error: %v close connection", err)
				return
			}
		case <-t.C:
			continue
		}
	}
}

func (c *Client) SendTryLastRequest() {
	if len(c.lastSendReq.Data) != 0 {
		pkg.Debug("client -> server try %s", c.lastSendReq)
		_ = c.SendToServer(c.lastSendReq, 1, c.secretKey)
	}
}

func (c *Client) Run() {
	defer c.Close()
	go c.readServerData()

	var count int
	for !c.closed.Load() { // 从矿机从读取数据
		if !c.Wait(3 * time.Second) {
			if count < 3 {
				c.SendTryLastRequest()
				continue
			}
			pkg.Warn("%s %s 等待ack超时. close connection", c.ip, c.id)
			return
		}
		count = 0
		data := make([]byte, 1024)
		n, err := c.lconn.Read(data)
		if err != nil {
			pkg.Warn("miner close connection error: %v. close connection", err)
			c.SendCloseToServer(c.secretKey)
			return
		}

		if err := c.SendDataToServer(data[:n], c.secretKey); err != nil {
			pkg.Error("send data to server error: %s try 10 times. close connection", err)
			return
		}
	}
}

func (c *Client) SetReady() {
	if c.ready.Load() {
		return
	}
	c.lastSendReq = protocol.Request{}
	c.ready.Store(true)
	c.readyChan <- struct{}{}
	pkg.Debug("设置 %s ready", c.id)
}

func (c *Client) SetWait(req protocol.Request) {
	pkg.Debug("设置 %s wait", c.id)
	c.ready.Store(false)
	c.lastSendReq = req
}

func (c *Client) Wait(timeout time.Duration) bool {
	if c.ready.Load() {
		return true
	}
	t := time.NewTicker(timeout)
	defer t.Stop()
	select {
	case <-c.readyChan:
		return true
	case <-t.C:
		return false
	}
}

func RunClient(address, secretKey, serverAddress, poolAddress, clientId string) error {
	s, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	for {
		conn, err := s.Accept()
		if err != nil {
			continue
		}
		pkg.Debug("nwe connect from mine %s", conn.RemoteAddr().String())
		go newClient(
			strings.Split(conn.RemoteAddr().String(), ":")[0],
			serverAddress, secretKey, poolAddress, conn, clientId)
	}
}
