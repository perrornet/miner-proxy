package server

import (
	"crypto/sha1"
	"io"
	"math/rand"
	"net"
	"time"

	"github.com/go-ozzo/ozzo-log"
	kcp "github.com/xtaci/kcp-go/v5"
	"github.com/xtaci/kcptun/generic"
	"github.com/xtaci/smux"
	"go.uber.org/atomic"
	"golang.org/x/crypto/pbkdf2"
)

const (
	// SALT is use for pbkdf2 key expansion
	SALT = "miner-proxy"
	// maximum supported smux version
	maxSmuxVer = 2
	// stream copy buffer size
	bufSize = 4096
)

type Server struct {
	l, r    string
	key     string
	logFile string
	crypt   string
	log     *log.Logger
	stop    *atomic.Bool
}

func NewServer(l, r, key, logFile, crypt string) *Server {
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
	return &Server{
		l: l, r: r, key: key, log: logger, logFile: logFile, crypt: crypt, stop: atomic.NewBool(false),
	}
}

func (s *Server) handleMux(conn net.Conn) {

	// stream multiplex
	smuxConfig := smux.DefaultConfig()
	smuxConfig.Version = 1
	smuxConfig.MaxReceiveBuffer = 4194304
	smuxConfig.MaxStreamBuffer = 2097152
	smuxConfig.KeepAliveInterval = 600 * time.Second
	smuxConfig.KeepAliveTimeout = 900 * time.Second

	mux, err := smux.Server(conn, smuxConfig)
	if err != nil {
		s.log.Error(err.Error())
		return
	}
	defer mux.Close()

	for !s.stop.Load() {
		stream, err := mux.AcceptStream()
		if err != nil {
			return
		}

		go func(p1 *smux.Stream) {
			var p2 net.Conn
			var err error
			p2, err = net.Dial("tcp", s.r)
			if err != nil {
				s.log.Error(err.Error())
				p1.Close()
				return
			}
			s.handleClient(p1, p2)
		}(stream)
	}
}

func (s *Server) handleClient(p1 *smux.Stream, p2 net.Conn) {

	defer p1.Close()
	defer p2.Close()

	s.log.Debug("新的客户端 %s 连接被代理到 %s (%d)", p1.RemoteAddr(), s.r, p1.ID())
	defer s.log.Debug("关闭  %s 到 %s 的代理连接 (%d)", p1.RemoteAddr(), s.r, p1.ID())

	// start tunnel & wait for tunnel termination
	streamCopy := func(dst io.Writer, src io.ReadCloser) {
		if _, err := generic.Copy(dst, src); err != nil {
			if err == smux.ErrInvalidProtocol {
				s.log.Debug("协议发生错误, 请确定服务端也是本程序")
			}
		}
		p1.Close()
		p2.Close()
	}

	go streamCopy(p2, p1)
	streamCopy(p1, p2)
}

type timedSession struct {
	session    *smux.Session
	expiryDate time.Time
}

func (s *Server) Run() {
	defer s.Close()
	rand.Seed(int64(time.Now().Nanosecond()))

	addr, err := net.ResolveTCPAddr("tcp", s.l)
	if err != nil {
		s.log.Error("解析监听端口失败")
		return
	}

	pass := pbkdf2.Key([]byte(s.key), []byte(SALT), 4096, 32, sha1.New)
	s.log.Debug("监听 %s 成功, 开始处理密钥...", addr)
	var block kcp.BlockCrypt
	switch s.crypt {
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
		s.crypt = "aes"
		block, _ = kcp.NewAESBlockCrypt(pass)
	}

	s.log.Debug("开始处理数据...")
	loop := func(lis *kcp.Listener) {
		_ = lis.SetDSCP(0)
		_ = lis.SetReadBuffer(4194304)
		_ = lis.SetWriteBuffer(4194304)

		for !s.stop.Load() {

			conn, err := lis.AcceptKCP()
			if err != nil {
				continue
			}
			conn.SetStreamMode(true)
			conn.SetWriteDelay(false)
			conn.SetNoDelay(0, 30, 2, 1)
			conn.SetMtu(958)
			conn.SetWindowSize(1024, 1024)
			conn.SetACKNoDelay(true)
			go s.handleMux(generic.NewCompStream(conn))
		}
	}

	// udp stack
	lis, err := kcp.ListenWithOptions(s.l, block, 10, 3)
	if err != nil {
		s.log.Error(err.Error())
		return
	}
	loop(lis)
}

func (c *Server) Close() {
	c.log.Debug("关闭服务器...")
	c.log.Close()
	c.stop.Store(true)
	return
}
