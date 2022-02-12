package client

import (
	"fmt"
	"miner-proxy/pkg"
	"miner-proxy/proxy/protocol"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/jmcvetta/randutil"
	"github.com/pkg/errors"
	"github.com/segmentio/ksuid"
	"github.com/spf13/cast"
	"go.uber.org/atomic"
)

var (
	servers      sync.Map
	clients      sync.Map
	serverManage *ServerManage
	localIPv4    = pkg.LocalIPv4s()
)

func InitServerManage(maxConn int, secretKey, serverAddress string) error {
	s, err := NewServerManage(maxConn, secretKey, serverAddress)
	if err != nil {
		return err
	}
	server := s.GetServer()
	if server == nil {
		return errors.New("connection to server error")
	}

	fc := protocol.NewGoframeProtocol(s.secretKey, true, server.conn)
	req := protocol.Request{
		ClientId: pkg.CLIENTID,
		Type:     protocol.INIT,
	}
	data, _ := protocol.Decode2Byte(req)
	if err := fc.WriteFrame(data); err != nil {
		return errors.Wrap(err, "send init data error")
	}

	go func() {
		for {
			var count int
			servers.Range(func(key, value interface{}) bool {
				count++
				return true
			})
			for i := 0; i < maxConn-count; i++ {
				server := s.NewServer(fmt.Sprintf("%s_%d", s.serverAddress, i))
				if server == nil {
					pkg.Warn("connection to server failed")
				}
			}
			time.Sleep(time.Second * 5)
		}
	}()
	serverManage = s
	return nil
}

type ServerManage struct {
	secretKey, serverAddress string
	maxConn                  int
}

func NewServerManage(maxConn int, secretKey, serverAddress string) (*ServerManage, error) {
	s := &ServerManage{secretKey: secretKey, serverAddress: serverAddress, maxConn: maxConn}
	for i := 0; i < maxConn; i++ {
		server := s.NewServer(fmt.Sprintf("%s_%d", s.serverAddress, i))
		if server == nil {
			return nil, errors.New("connection to server error")
		}
	}
	return s, nil
}

func (s *ServerManage) GetServer() *Server {
	var result []*Server
	servers.Range(func(key, value interface{}) bool {
		if value.(*Server).close.Load() {
			// 重新建立
			server := s.NewServer(cast.ToString(key))
			if server == nil {
				return true
			}
			value = server
			servers.Store(key, server)
		}
		result = append(result, value.(*Server))
		return true
	})
	if len(result) == 0 {
		server := s.NewServer(ksuid.New().String())
		if server == nil {
			return nil
		}
		return server
	}
	index, _ := randutil.IntRange(0, len(result))
	return result[index]
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

	go func(server *Server) {
		defer server.Close()
		fc := protocol.NewGoframeProtocol(s.secretKey, true, server.conn)
		for !server.close.Load() {
			data, err := fc.ReadFrame()
			if err != nil {
				pkg.Debug("server closed")
				return
			}
			req, err := protocol.Encode2Request(data)
			if err != nil {
				return
			}

			switch req.Type {
			case protocol.PING, protocol.PONG:
				req := protocol.Request{
					ClientId: pkg.CLIENTID,
					Type:     protocol.PONG,
				}
				data, _ := protocol.Decode2Byte(req)
				if err := fc.WriteFrame(data); err != nil {
					return
				}
				continue
			case protocol.INIT, protocol.REGISTER:
				continue
			}
			v, ok := clients.Load(req.MinerId)
			if !ok {
				continue
			}

			v.(*Client).input <- req
		}
	}(server)
	fc := protocol.NewGoframeProtocol(s.secretKey, true, server.conn)
	req, _ := protocol.Decode2Byte(protocol.Request{
		ClientId: pkg.CLIENTID,
		Type:     protocol.REGISTER,
	})
	_ = fc.WriteFrame(req)
	servers.Store(id, server)
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
		// 从server中删除
		servers.Delete(s.id)
	})
}

type Client struct {
	id, ip, serverAddress, secretKey, poolAddress string
	lconn                                         net.Conn
	input                                         chan protocol.Request
	closed                                        *atomic.Bool
	ready                                         *atomic.Bool
	seq                                           *atomic.Int64
	stop                                          sync.Once
}

func newClient(ip string, serverAddress string, secretKey string, poolAddress string, conn net.Conn) {
	if strings.Contains(ip, "127.0.0.1") && localIPv4 != "" {
		ip = localIPv4
	}
	client := &Client{
		secretKey:     secretKey,
		serverAddress: serverAddress,
		ip:            ip,
		lconn:         conn,
		input:         make(chan protocol.Request),
		ready:         atomic.NewBool(true),
		closed:        atomic.NewBool(false),
		id:            ksuid.New().String(),
		poolAddress:   poolAddress,
		seq:           atomic.NewInt64(0),
	}
	defer client.Close()
	clients.Store(client.id, client)
	if err := client.Login(); err != nil {
		pkg.Warn("login to server failed %s", err)
		return
	}
	client.Run()
	return
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
	return pkg.Try(func() bool {
		s := serverManage.GetServer()
		if s == nil {
			time.Sleep(time.Second) // 最多重试10次
			return false
		}
		req.Seq = c.seq.Add(1)
		fc := protocol.NewGoframeProtocol(secretKey, true, s.conn)
		sendData, err := protocol.Decode2Byte(req)
		if err != nil {
			return false
		}
		pkg.Debug("send data to server %s %s", s.conn.RemoteAddr().String(), req.String())
		if err := fc.WriteFrame(sendData); err != nil {
			return false
		}
		c.SetWait()
		return true
	}, maxTry)
}

func (c *Client) SendCloseToServer(secretKey string) {
	req := &protocol.Request{
		ClientId: pkg.CLIENTID,
		MinerId:  c.id,
		Type:     protocol.CLOSE,
	}
	_ = c.SendToServer(req.End(), 1, secretKey)
}

func (c *Client) SendDataToServer(data []byte, secretKey string) error {
	req := (&protocol.Request{
		MinerId:  c.id,
		ClientId: pkg.CLIENTID,
		Type:     protocol.DATA,
		Data:     data,
	}).End()
	return c.SendToServer(req, 10, secretKey)
}

func (c *Client) Login() error {
	req := protocol.Request{
		Seq:      c.seq.Add(1),
		MsgId:    ksuid.New().String(),
		ClientId: pkg.CLIENTID,
		MinerId:  c.id,
		Type:     protocol.LOGIN,
		Data: protocol.DecodeLoginRequest2Byte(protocol.LoginRequest{
			PoolAddress: c.poolAddress,
			MinerIp:     c.ip,
		}),
	}
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
				return
			}
			switch req.Type {
			case protocol.ERROR, protocol.CLOSE:
				pkg.Error("server error or pool closed. client id %s; error %s", req.MinerId, req.Data)
				return
			case protocol.LOGIN:
				c.SetReady()
				continue
			}
			c.SetReady()
			pkg.Debug("read data from server %s %s", c.serverAddress, req)
			if _, err := c.lconn.Write(req.Data); err != nil {
				return
			}
		case <-t.C:
			continue
		}
	}
}

func (c *Client) Run() {
	defer c.Close()

	go c.readServerData()

	for !c.closed.Load() { // 从矿机从读取数据
		if !c.Wait(10) {
			return
		}
		data := make([]byte, 1024)
		n, err := c.lconn.Read(data)
		if err != nil {
			c.SendCloseToServer(c.secretKey)
			return
		}
		if err := c.SendDataToServer(data[:n], c.secretKey); err != nil {
			pkg.Debug("send data to server error: %s try 10 times", err)
			return
		}
	}
}

func (c *Client) SetWait() {
	c.ready.Store(false)
}

func (c *Client) Wait(timeout int) bool {
	var count int
	for count < timeout {
		if c.ready.Load() {
			return true
		}
		count++
		time.Sleep(time.Second)
	}
	return false
}

func (c *Client) SetReady() {
	c.ready.Store(true)
}

func RunClient(address, secretKey, serverAddress, poolAddress string) error {
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
			serverAddress, secretKey, poolAddress, conn)
	}
}
