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

	"github.com/panjf2000/gnet"
	"github.com/panjf2000/gnet/pkg/pool/goroutine"
	"github.com/spf13/cast"
	"go.uber.org/atomic"
)

var (
	clients   sync.Map
	conns     sync.Map
	connId2Id sync.Map
	connDelay sync.Map
	p         = goroutine.Default()
)

type Delay struct {
	startTime time.Time
	delay     time.Duration
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
	stop                      sync.Once
	startTime                 time.Time
	dataSize                  *atomic.Int64
	stopTime                  time.Time
	ready                     *atomic.Bool
	readyChan                 chan struct{}
}

func (c *Client) SetReady() {
	if c.ready.Load() {
		return
	}
	pkg.Debug("设置 %s ready", c.id)
	c.ready.Store(true)
	c.readyChan <- struct{}{}
}

func (c *Client) SetWait() {
	pkg.Debug("设置 %s wait", c.id)
	c.ready.Store(false)
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

func (c *Client) Init(req protocol.Request, defaultPoolAddress, clientId string) error {
	c.id = req.MinerId
	c.ready = atomic.NewBool(true)
	c.readyChan = make(chan struct{})
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
		gnet.WithReusePort(true),
		gnet.WithReuseAddr(true),
		gnet.WithCodec(protocol.NewProtocol(secretKey, true)),
		gnet.WithTicker(true),
	)
}

func (ps *Server) OnClosed(c gnet.Conn, _ error) (action gnet.Action) {
	clientId, ok := connId2Id.Load(c.RemoteAddr().String())
	if !ok {
		return gnet.None
	}
	v, ok := conns.Load(clientId)
	if !ok {
		return gnet.None
	}
	cd := v.(*ClientDispatch)
	cd.DelConn(ps.getConnId(cast.ToString(clientId), c))
	if cd.ConnCount() == 0 {
		conns.Delete(clientId)
	}
	connId2Id.Delete(c.RemoteAddr().String())
	return gnet.None
}

func (ps *Server) Tick() (delay time.Duration, action gnet.Action) {
	var exist = make(map[string]struct{})
	var clientMap = make(map[string][]string)
	clients.Range(func(key, value interface{}) bool {
		c := value.(*Client)
		if c.closed.Load() {
			return true
		}
		clientMap[c.clientId] = append(clientMap[c.clientId], cast.ToString(key))
		return true
	})

	connId2Id.Range(func(key, value interface{}) bool {
		v, ok := conns.Load(cast.ToString(value))
		if !ok {
			return true
		}
		cd := v.(*ClientDispatch)
		if cd.ConnCount() == 0 {
			conns.Delete(value)
			connId2Id.Delete(key)
			return true
		}
		cd.conns.Range(func(key, value1 interface{}) bool {
			req := new(protocol.Request).SetType(protocol.PING).
				SetClientId(strings.Split(cast.ToString(key), "-")[0])
			if _, ok := exist[cast.ToString(value)]; !ok {
				v, _ := connDelay.Load(value)
				if v == nil {
					v = Delay{}
				}
				connDelay.Store(value, Delay{
					startTime: time.Now(),
					delay:     v.(Delay).delay,
				})
				if _, ok = clientMap[cast.ToString(value)]; ok {
					req.SetData([]byte(strings.Join(clientMap[cast.ToString(value)], ",")))
				}
			}

			data, _ := protocol.Decode2Byte(req.End())
			if err := value1.(*Conn).Conn.AsyncWrite(data); err != nil {
				cd.DelConn(cast.ToString(key))
				return true
			}
			exist[cast.ToString(value)] = struct{}{}
			return false
		})
		return true
	})
	delay = time.Second * 20
	return
}

func (ps *Server) SendToClient(req protocol.Request, maxTry int, clientId, _ string) error {
	return pkg.Try(func() bool {
		v, ok := conns.Load(clientId)
		if !ok {
			time.Sleep(time.Second)
			return false
		}
		cd := v.(*ClientDispatch)

		conn := cd.GetConn()
		if conn == nil {
			pkg.Warn("%s 没有可用的连接", clientId)
			time.Sleep(time.Second)
			return false
		}
		data, _ := protocol.Decode2Byte(req)
		pkg.Debug("server -> client %s", req)
		if err := conn.AsyncWrite(data); err != nil {
			pkg.Warn("server data to client error: %v", err)
			return false
		}
		return true
	}, maxTry)
}

func (ps *Server) getOrCreateClient(req protocol.Request) (*Client, error) {
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
	if err := c.Init(req, ps.PoolAddress, req.ClientId); err != nil {
		return nil, err
	}
	clients.Store(req.MinerId, c)
	_ = ps.pool.Submit(c.pool.Start)
	_ = ps.pool.Submit(func() {
		defer c.Close()
		t := time.NewTicker(time.Second * 5)
		defer t.Stop()
		for !c.closed.Load() {
			if !c.Wait(time.Second * 10) {
				pkg.Warn("等待 client %s ack 超时", c.id)
				return
			}
			select {
			case data, ok := <-c.output:
				if !ok {
					_ = ps.SendToClient(protocol.Request{
						ClientId: c.clientId,
						MinerId:  c.id,
						Type:     protocol.CLOSE,
					}, 1, c.clientId, c.id)
					return
				}
				req := protocol.Request{Type: protocol.DATA, MinerId: c.id, Data: data,
					ClientId: c.clientId}
				data, _ = protocol.Decode2Byte(req)
				if err := ps.SendToClient(req, 10, c.clientId, c.id); err != nil {
					pkg.Warn("try 10 times write to client failed")
					return
				}
				c.SetWait()
				c.dataSize.Add(int64(len(data)))
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

func (ps *Server) ping(req protocol.Request, _ gnet.Conn) (out []byte, action gnet.Action) {
	v, ok := connDelay.Load(req.ClientId)
	if !ok {
		return nil, gnet.None
	}
	if v.(Delay).delay.Seconds() <= 0 {
		connDelay.Store(req.ClientId, Delay{
			delay: time.Since(v.(Delay).startTime),
		})
	}
	for _, v := range strings.Split(string(req.Data), ",") {
		if v == "" {
			continue
		}
		pkg.Debug("删除过时的矿机id: %s", v)
		client, ok := ps.getClient(v)
		if !ok {
			return nil, gnet.None
		}
		client.Close()
	}
	return nil, gnet.None
}

func (ps *Server) login(req protocol.Request, _ gnet.Conn) (out []byte, action gnet.Action) {
	_, err := ps.getOrCreateClient(req)
	if err != nil {
		data, _ := protocol.Decode2Byte(protocol.Request{Type: protocol.ERROR, MinerId: req.MinerId, Data: []byte(err.Error())})
		return data, gnet.None
	}
	req = protocol.CopyRequest(req)
	pkg.Debug("server -> client %s", req)
	data, _ := protocol.Decode2Byte(req)
	return data, gnet.None
}

func (ps *Server) proxy(req protocol.Request, _ gnet.Conn) (out []byte, action gnet.Action) {
	defer func() {
		if err := recover(); err != nil {
			if strings.Contains(cast.ToString(err), "send on closed channel") {
				return
			}
			out, _ = protocol.Decode2Byte(protocol.Request{Type: protocol.ACK,
				MinerId: req.MinerId})
		}
	}()
	client, ok := ps.getClient(req.MinerId)
	if !ok {
		data, _ := protocol.Decode2Byte(protocol.Request{Type: protocol.ERROR,
			MinerId: req.MinerId, Data: []byte("need login")})
		return data, gnet.None
	}
	client.input <- req.Data
	client.dataSize.Add(int64(len(req.Data)))
	req = protocol.Request{Type: protocol.ACK,
		MinerId: req.MinerId}
	data, _ := protocol.Decode2Byte(req)
	pkg.Debug("server -> client %s", req)
	return data, gnet.None
}

func (ps *Server) getConnId(clientId string, c gnet.Conn) string {
	return fmt.Sprintf("%s_%s", clientId, c.RemoteAddr().String())
}

func (ps *Server) init(req protocol.Request, c gnet.Conn) (out []byte, action gnet.Action) {
	info := strings.Split(string(req.Data), "|")
	if len(info) < 3 {
		return nil, gnet.Close
	}
	v, _ := conns.LoadOrStore(req.ClientId, NewClientDispatch(req.ClientId, info[0], info[2]))
	cd := v.(*ClientDispatch)

	cd.SetConn(ps.getConnId(req.ClientId, c), c)
	connId2Id.Store(c.RemoteAddr().String(), req.ClientId)
	var closeMiner []string
	for _, miner := range pkg.String2Array(info[1], ",") {
		if _, ok := clients.Load(miner); !ok {
			closeMiner = append(closeMiner, miner)
		}
	}
	if len(closeMiner) != 0 {
		data, _ := protocol.Decode2Byte(protocol.Request{
			ClientId: cd.ClientId,
			Type:     protocol.CLOSE,
			Data:     []byte(strings.Join(closeMiner, ",")),
		})
		return data, gnet.None
	}
	data, _ := protocol.Decode2Byte(protocol.Request{
		ClientId: cd.ClientId,
		Type:     req.Type,
	})
	pkg.Debug("server -> client %s", req)
	cd.conns.Range(func(key, value interface{}) bool {
		return true
	})
	return data, gnet.None
}

func (ps *Server) React(frame []byte, c gnet.Conn) (out []byte, action gnet.Action) {
	defer pkg.Recover(true)
	req, err := protocol.Encode2Request(frame)
	if err != nil {
		pkg.Warn("Encode2Request error %s", err)
		return nil, gnet.Close
	}
	pkg.Debug("server <- client %s", req.String())
	switch req.Type {
	case protocol.INIT:
		return ps.init(req, c)
	case protocol.DATA:
		return ps.proxy(req, c)
	case protocol.LOGIN:
		return ps.login(req, c)
	case protocol.PING, protocol.PONG:
		return ps.ping(req, c)
	case protocol.CLOSE:
		client, ok := ps.getClient(req.MinerId)
		if !ok {
			return nil, gnet.None
		}
		client.Close()
		return nil, gnet.None
	case protocol.ACK:
		client, ok := ps.getClient(req.MinerId)
		if !ok {
			return nil, gnet.None
		}
		client.SetReady()
		return nil, gnet.None
	}
	return nil, gnet.Close
}
