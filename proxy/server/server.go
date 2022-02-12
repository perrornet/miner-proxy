package server

import (
	"errors"
	"fmt"
	"miner-proxy/pkg"
	"miner-proxy/proxy/backend"
	"miner-proxy/proxy/protocol"
	"strings"
	"sync"
	"time"

	"github.com/jmcvetta/randutil"
	"github.com/panjf2000/gnet"
	"github.com/panjf2000/gnet/pkg/pool/goroutine"
	"github.com/spf13/cast"
	"go.uber.org/atomic"
)

var (
	clients     sync.Map
	clientConn  sync.Map
	clientDelay sync.Map
	p           = goroutine.Default()
)

type Delay struct {
	startTime time.Time
	endTime   time.Time
	delay     time.Duration
}

func getClientConn(clientId string) gnet.Conn {
	var result []interface{}
	clientConn.Range(func(key, value interface{}) bool {
		k := cast.ToString(key)
		if !strings.HasPrefix(k, clientId) {
			return true
		}
		result = append(result, value)
		return true
	})
	if len(result) == 0 {
		return nil
	}
	index, _ := randutil.IntRange(0, len(result))
	return result[index].(gnet.Conn)
}

func setClientConn(clientId string, c gnet.Conn) {
	clientConn.Store(getClientConnKey(clientId, c), c)
}

func getClientConnKey(clientId string, c gnet.Conn) string {
	return fmt.Sprintf("%s-%s", clientId, c.RemoteAddr().String())
}

func delClientConn(c gnet.Conn) {
	ip := c.RemoteAddr().String()
	clientConn.Range(func(key, value interface{}) bool {
		k := cast.ToString(key)
		if !strings.HasSuffix(k, ip) {
			return true
		}
		clientConn.Delete(key)
		clientDelay.Delete(key)
		return false
	})
}

type Server struct {
	*gnet.EventServer
	pool        *goroutine.Pool
	PoolAddress string
}

type Client struct {
	id, address, ip, clientId string
	pool                      *backend.PoolConn
	input                     chan []byte
	output                    chan []byte
	closed                    *atomic.Bool
	seq                       *atomic.Int64
	stop                      sync.Once
	startTime                 time.Time
	dataSize                  *atomic.Int64
	stopTime                  time.Time
}

func (c *Client) Init(req protocol.Request, _ gnet.Conn, defaultPoolAddress, clientId string) error {
	c.id = req.MinerId
	lr, err := protocol.Encode2LoginRequest(req.Data)
	if err != nil {
		return err
	}
	c.ip = lr.MinerIp
	c.clientId = clientId
	c.address = lr.PoolAddress
	if c.address == "" {
		c.address = defaultPoolAddress
	}
	c.input = make(chan []byte)
	c.output = make(chan []byte)
	c.startTime = time.Now()
	c.seq = atomic.NewInt64(0)
	c.stopTime = time.Time{}
	c.dataSize = atomic.NewInt64(0)
	c.closed = atomic.NewBool(false)
	p, err := backend.NewPoolConn(c.address, c.input, c.output)
	if err != nil {
		return err
	}
	c.pool = p
	return nil
}

func (c *Client) Close() {
	c.stop.Do(func() {

		c.closed.Store(true)
		if c.input != nil {
			close(c.input)
		}

		if c.pool != nil {
			c.pool.Close()
		}
		c.stopTime = time.Now()
	})
}

func NewServer(address, secretKey, PoolAddress string) error {
	s := &Server{pool: p, PoolAddress: PoolAddress}
	return gnet.Serve(s, "tcp://"+address,
		gnet.WithMulticore(true),
		gnet.WithCodec(protocol.NewProtocol(secretKey, true)),
		gnet.WithTicker(true),
	)
}

func (ps *Server) OnClosed(c gnet.Conn, _ error) (action gnet.Action) {
	pkg.Debug("close connect %s", c.RemoteAddr().String())
	delClientConn(c)
	return gnet.None
}

func (ps *Server) Tick() (delay time.Duration, action gnet.Action) {
	var exist = make(map[string]struct{})
	clientConn.Range(func(key, value interface{}) bool {
		c := value.(gnet.Conn)
		req := new(protocol.Request).SetType(protocol.PING).
			SetClientId(strings.Split(cast.ToString(key), "-")[0]).End()
		data, _ := protocol.Decode2Byte(req)
		if _, ok := exist[req.ClientId]; !ok {
			clientDelay.Store(req.ClientId, &Delay{
				startTime: time.Now(),
				delay:     0,
			})
			exist[req.ClientId] = struct{}{}
		}

		if err := c.AsyncWrite(data); err != nil {
			pkg.Warn("ping client failed %s", err)
			delClientConn(c)
			clientDelay.Delete(req.ClientId)
			return true
		}
		return true
	})

	n, _ := randutil.IntRange(10, 60)
	delay = time.Second * time.Duration(n)
	return
}

func (ps *Server) SendToClient(req protocol.Request, maxTry int, client *Client) error {
	return pkg.Try(func() bool {
		conn := getClientConn(client.clientId)
		if conn == nil {
			return false
		}
		data, _ := protocol.Decode2Byte(req.End())
		_ = conn.AsyncWrite(data)
		return true
	}, maxTry)
}

func (ps *Server) getOrCreateClient(req protocol.Request, ctx gnet.Conn) (*Client, error) {
	client, ok := clients.Load(req.MinerId)
	if ok {
		if !client.(*Client).closed.Load() {
			return client.(*Client), nil
		}
	}
	if req.Type != protocol.LOGIN {
		return nil, errors.New("need login")
	}
	c := new(Client)
	if err := c.Init(req, ctx, ps.PoolAddress, req.ClientId); err != nil {
		return nil, err
	}
	clients.Store(req.MinerId, c)
	_ = ps.pool.Submit(c.pool.Start)
	_ = ps.pool.Submit(func() {
		defer c.Close()
		t := time.NewTicker(time.Second * 5)
		defer t.Stop()
		for !c.closed.Load() {
			select {
			case data, ok := <-c.output:
				if !ok {
					_ = ps.SendToClient(protocol.Request{
						ClientId: c.clientId,
						MinerId:  c.id,
						Type:     protocol.CLOSE,
					}, 1, c)
					return
				}
				req := protocol.Request{Type: protocol.DATA, MinerId: c.id, Data: data,
					ClientId: c.clientId, Seq: c.seq.Add(1)}

				data, _ = protocol.Decode2Byte(req.End())
				if err := ps.SendToClient(req, 10, c); err != nil {
					pkg.Warn("try 10 times write to client failed")
					return
				}
				c.dataSize.Add(int64(len(data)))
				TOTALSIZE.Add(int64(len(data)))
			case <-t.C:
			}
		}
	})

	return c, nil
}

func (ps *Server) getClient(MinerId string) (*Client, bool) {
	client, ok := clients.Load(MinerId)
	if !ok {
		return nil, false
	}
	c := client.(*Client)
	if c.closed.Load() {
		return nil, false
	}
	return c, true
}

func (ps *Server) readPing(req protocol.Request, _ gnet.Conn) (out []byte, action gnet.Action) {
	v, ok := clientDelay.Load(req.ClientId)
	if !ok {
		return nil, gnet.None
	}
	if v.(*Delay).endTime.IsZero() {
		v.(*Delay).endTime = time.Now()
		clientDelay.Store(req.ClientId, v)
	}
	return nil, gnet.None
}

func (ps *Server) login(req protocol.Request, c gnet.Conn) (out []byte, action gnet.Action) {
	client, err := ps.getOrCreateClient(req, c)
	if err != nil {
		data, _ := protocol.Decode2Byte(protocol.Request{Type: protocol.ERROR, MinerId: req.MinerId, Data: []byte(err.Error())})
		return data, gnet.None
	}
	req = protocol.CopyRequest(req, client.seq.Add(1))
	pkg.Debug("send response %s", req)
	data, _ := protocol.Decode2Byte(req)
	return data, gnet.None
}

func (ps *Server) proxy(req protocol.Request, _ gnet.Conn) (out []byte, action gnet.Action) {
	client, ok := ps.getClient(req.MinerId)
	if !ok {
		data, _ := protocol.Decode2Byte(protocol.Request{Type: protocol.ERROR,
			MinerId: req.MinerId, Data: []byte("need login")})
		return data, gnet.None
	}
	client.input <- req.Data
	client.dataSize.Add(int64(len(req.Data)))
	TOTALSIZE.Add(int64(len(req.Data)))
	req = protocol.CopyRequest(req, client.seq.Add(1))
	data, _ := protocol.Decode2Byte(req)
	return data, gnet.None
}

func (ps *Server) init(req protocol.Request, _ gnet.Conn) (out []byte, action gnet.Action) {
	// 清理所有的client
	clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)
		if client.clientId == req.ClientId {
			clients.Delete(key)
		}
		return true
	})
	data, _ := protocol.Decode2Byte(protocol.CopyRequest(req, 0))
	return data, gnet.None
}

func (ps *Server) React(frame []byte, c gnet.Conn) (out []byte, action gnet.Action) {
	req, err := protocol.Encode2Request(frame)
	if err != nil {
		pkg.Debug("Encode2Request error %s", err)
		return nil, gnet.Close
	}
	if req.Type != protocol.PING && req.Type != protocol.PONG {
		pkg.Debug("read data from client %s %s", c.RemoteAddr().String(), req.String())
	}
	setClientConn(req.ClientId, c)
	switch req.Type {
	case protocol.INIT:
		return ps.init(req, c)
	case protocol.DATA:
		return ps.proxy(req, c)
	case protocol.LOGIN:
		return ps.login(req, c)
	case protocol.PING, protocol.PONG:
		return ps.readPing(req, c)
	case protocol.CLOSE:
		client, ok := ps.getClient(req.MinerId)
		if !ok {
			return nil, gnet.None
		}
		client.Close()
		return nil, gnet.None
	case protocol.REGISTER:
		return nil, gnet.None
	}
	return nil, gnet.Close
}
