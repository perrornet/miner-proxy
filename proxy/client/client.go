package client

import (
	"crypto/sha1"
	"io"
	"math/rand"
	"net"
	"time"

	"github.com/go-ozzo/ozzo-log"
	"go.uber.org/atomic"
	"golang.org/x/crypto/pbkdf2"

	"github.com/pkg/errors"
	kcp "github.com/xtaci/kcp-go/v5"
	"github.com/xtaci/kcptun/generic"
	"github.com/xtaci/smux"
)

const (
	SALT = "miner-proxy"
)

type Client struct {
	l       string
	r       string
	key     string
	logFile string
	crypt   string
	log     *log.Logger
	stop    *atomic.Bool
}

func NewClient(l, r, key, logFile, crypt string) *Client {
	logger := log.NewLogger()

	t2 := &log.FileTarget{
		Filter:      &log.Filter{MaxLevel: log.LevelDebug},
		Rotate:      true,
		BackupCount: 10,
		MaxBytes:    100 * 1024 * 1024,
		FileName:    logFile,
	}

	logger.MaxLevel = log.LevelDebug
	logger.Targets = append(logger.Targets, t2, log.NewConsoleTarget())
	logger.Open()
	return &Client{
		l: l, r: r, key: key, log: logger, logFile: logFile, crypt: crypt, stop: atomic.NewBool(false),
	}
}

func (c *Client) Close() {
	c.log.Close()
	c.stop.Store(true)
	return
}

func (c *Client) handleClient(session *smux.Session, p1 net.Conn) {
	defer p1.Close()
	p2, err := session.OpenStream()
	if err != nil {
		c.log.Debug("创建新的流失败: %s", err)
		return
	}
	defer p2.Close()
	c.log.Debug("新的客户端 %s 连接被代理到 %s (%d)", p1.RemoteAddr(), c.r, p2.ID())
	defer c.log.Debug("关闭  %s 到 %s 的代理连接 (%d)", p1.RemoteAddr(), c.r, p2.ID())

	streamCopy := func(dst io.Writer, src io.ReadCloser) {
		if _, err := generic.Copy(dst, src); err != nil {
			// report protocol error
			if err == smux.ErrInvalidProtocol {
				c.log.Debug("协议发生错误, 请确定服务端也是本程序")
			}
		}
		p1.Close()
		p2.Close()
	}

	go streamCopy(p1, p2)
	streamCopy(p2, p1)
}

type timedSession struct {
	session    *smux.Session
	expiryDate time.Time
}

func (c *Client) Run() {
	rand.Seed(int64(time.Now().Nanosecond()))
	c.log.Debug("开始处理监听端口...")
	addr, err := net.ResolveTCPAddr("tcp", c.l)
	if err != nil {
		c.log.Error("解析监听端口失败")
		return
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		c.log.Error("监听端口失败, 请更换端口")
		return
	}
	c.log.Debug("监听 %s 成功, 开始处理密钥...", addr)
	pass := pbkdf2.Key([]byte(c.key), []byte(SALT), 4096, 32, sha1.New)
	var block kcp.BlockCrypt
	switch c.crypt {
	case "sm4":
		block, _ = kcp.NewSM4BlockCrypt(pass[:16])
	case "tea":
		block, _ = kcp.NewTEABlockCrypt(pass[:16])
	case "xor":
		block, _ = kcp.NewSimpleXORBlockCrypt(pass)
	case "none":
		block, _ = kcp.NewNoneBlockCrypt(pass)
	case "aes-128":
		block, _ = kcp.NewAESBlockCrypt(pass[:16])
	case "aes-192":
		block, _ = kcp.NewAESBlockCrypt(pass[:24])
	case "blowfish":
		block, _ = kcp.NewBlowfishBlockCrypt(pass)
	case "twofish":
		block, _ = kcp.NewTwofishBlockCrypt(pass)
	case "cast5":
		block, _ = kcp.NewCast5BlockCrypt(pass[:16])
	case "3des":
		block, _ = kcp.NewTripleDESBlockCrypt(pass[:24])
	case "xtea":
		block, _ = kcp.NewXTEABlockCrypt(pass[:16])
	case "salsa20":
		block, _ = kcp.NewSalsa20BlockCrypt(pass)
	default:
		c.crypt = "aes"
		block, _ = kcp.NewAESBlockCrypt(pass)
	}

	createConn := func() (*smux.Session, error) {
		kcpconn, err := dial(c.r, block)
		if err != nil {
			return nil, errors.Wrap(err, "dial()")
		}
		kcpconn.SetStreamMode(true)
		kcpconn.SetWriteDelay(false)
		kcpconn.SetNoDelay(0, 30, 2, 1)
		kcpconn.SetWindowSize(1024, 1024)
		kcpconn.SetMtu(958)
		kcpconn.SetACKNoDelay(true)

		_ = kcpconn.SetReadBuffer(4194304)
		_ = kcpconn.SetWriteBuffer(4194304)
		smuxConfig := smux.DefaultConfig()
		smuxConfig.Version = 1
		smuxConfig.MaxReceiveBuffer = 4194304
		smuxConfig.MaxStreamBuffer = 2097152
		smuxConfig.KeepAliveInterval = 10 * time.Second

		if err := smux.VerifyConfig(smuxConfig); err != nil {
			return nil, errors.Wrap(err, "验证配置错误!")
		}

		session, err := smux.Client(generic.NewCompStream(kcpconn), smuxConfig)
		if err != nil {
			return nil, errors.Wrap(err, "创建连接失败")
		}
		return session, nil
	}

	waitConn := func() *smux.Session {
		for {
			session, err := createConn()
			if err != nil {
				c.log.Error(err.Error())
				time.Sleep(time.Second)
				continue
			}
			return session
		}
	}

	chScavenger := make(chan timedSession, 128)
	go c.scavenger(chScavenger)

	muxes := make([]timedSession, 1)

	rr := uint16(0)
	c.log.Debug("开始处理数据...")
	for !c.stop.Load() {
		p1, err := listener.AcceptTCP()
		if err != nil {
			c.log.Error(err.Error())
			continue
		}
		idx := rr % 1
		// do auto expiration && reconnection
		if muxes[idx].session == nil || muxes[idx].session.IsClosed() || time.Now().After(muxes[idx].expiryDate) {
			muxes[idx].session = waitConn()
			muxes[idx].expiryDate = time.Now().Add(600 * time.Second)
			chScavenger <- muxes[idx]
		}
		go c.handleClient(muxes[idx].session, p1)
		rr++
	}
}

func (c *Client) scavenger(ch chan timedSession) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var sessionList []timedSession
	for !c.stop.Load() {
		select {
		case item := <-ch:
			sessionList = append(sessionList, timedSession{
				item.session,
				item.expiryDate.Add(600 * time.Second)})
		case <-ticker.C:
			if len(sessionList) == 0 {
				continue
			}

			var newList []timedSession
			for k := range sessionList {
				session := sessionList[k]
				if session.session.IsClosed() {
					continue
				}
				if time.Now().After(session.expiryDate) {
					session.session.Close()
					continue
				}
				newList = append(newList, sessionList[k])
			}
			sessionList = newList
		}
	}
}
