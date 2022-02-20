package server

import (
	"sync"
	"time"

	"github.com/panjf2000/gnet"
	"go.uber.org/atomic"
)

type ClientDispatch struct {
	m          sync.RWMutex
	remoteAddr string
	pool       string
	conns      sync.Map
	connIds    []string
	index      *atomic.Int64
	ClientId   string
	startTime  time.Time
}

type Conn struct {
	gnet.Conn
	Id string
}

func NewClientDispatch(clientId, pool, remoteAddr string) *ClientDispatch {
	return &ClientDispatch{
		index:      atomic.NewInt64(0),
		ClientId:   clientId,
		pool:       pool,
		remoteAddr: remoteAddr,
		startTime:  time.Now(),
	}
}

func (c *ClientDispatch) GetConn() *Conn {
	c.m.RLock()
	defer c.m.RUnlock()
	if len(c.connIds) == 0 {
		return nil
	}
	index := c.index.Add(1) % int64(len(c.connIds))
	v, ok := c.conns.Load(c.connIds[index])
	if !ok {
		return nil
	}
	return v.(*Conn)
}

func (c *ClientDispatch) SetConn(id string, conn gnet.Conn) {
	c.conns.Store(id, &Conn{
		Conn: conn,
		Id:   id,
	})
	c.m.Lock()
	defer c.m.Unlock()
	c.connIds = append(c.connIds, id)
}

func (c *ClientDispatch) DelConn(id string) {
	c.conns.Delete(id)
	c.m.Lock()
	defer c.m.Unlock()
	var conns []string
	for index, v := range c.connIds {
		if v != id {
			conns = append(conns, c.connIds[index])
			continue
		}
	}
	c.connIds = conns
}

func (c *ClientDispatch) ConnCount() int {
	c.m.RLock()
	defer c.m.RUnlock()
	return len(c.connIds)
}
