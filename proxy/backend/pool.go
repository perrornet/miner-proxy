package backend

import (
	"io"
	"miner-proxy/pkg"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"go.uber.org/atomic"
)

type PoolConn struct {
	stop   sync.Once
	addr   string
	conn   net.Conn
	input  <-chan []byte
	output chan<- []byte
	closed *atomic.Bool
}

func NewPoolConn(addr string, input <-chan []byte, output chan<- []byte) (*PoolConn, error) {
	p := &PoolConn{
		addr:   addr,
		input:  input,
		output: output,
		closed: atomic.NewBool(false),
	}
	if err := p.init(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *PoolConn) init() error {
	if p.conn != nil {
		return nil
	}
	if p.closed.Load() {
		return nil
	}
	if p.input == nil || p.output == nil {
		return errors.New("input or output not make")
	}
	conn, err := net.DialTimeout("tcp", p.addr, time.Second*10)
	if err != nil {
		return errors.Wrapf(err, "dial mine pool %s error", p.addr)
	}
	p.conn = conn
	return nil
}

func (p *PoolConn) Close() {
	p.stop.Do(func() {
		if p.conn != nil {
			_ = p.conn.Close()
		}
		if p.output != nil {
			close(p.output)
		}
	})
	p.closed.Store(true)
}

func (p *PoolConn) IsClosed() bool {
	return p.closed.Load()
}

func (p *PoolConn) Address() string {
	return p.addr
}

func (p *PoolConn) Start() {
	defer p.Close()

	go func() {
		defer p.Close()
		defer func() {
			if err := recover(); err != nil {
				if strings.Contains(cast.ToString(err), "send on closed channel") {
					return
				}
			}
		}()
		for !p.IsClosed() {
			data := make([]byte, 1024)
			n, err := p.conn.Read(data)
			switch err {
			case nil:
			case io.EOF:
				return
			default:
				pkg.Warn("read data from miner pool error %s", err)
				return
			}
			p.output <- data[:n]
		}
	}()

	for !p.IsClosed() {
		data, isOpen := <-p.input
		if !isOpen {
			break
		}
		if _, err := p.conn.Write(data); err != nil {
			pkg.Debug("write data to miner pool error: %s", err)
			return
		}
	}
	return
}
