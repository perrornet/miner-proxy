package proxy

import (
	"bytes"
	"crypto/tls"
	"github.com/dustin/go-humanize"
	"io"
	"log"
	"miner-proxy/pkg"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Proxy - Manages a Proxy connection, piping data between local and remote.
type Proxy struct {
	sentBytes     uint64
	receivedBytes uint64
	laddr, raddr  *net.TCPAddr
	lconn, rconn  io.ReadWriteCloser
	erred         bool
	errsig        chan bool
	tlsUnwrapp    bool
	tlsAddress    string

	Matcher  func([]byte)
	Replacer func([]byte) []byte

	// Settings
	Nagles    bool
	Log       pkg.Logger
	OutputHex bool
	SecretKey string
	IsClient  bool
}

var (
	totalSize uint64
)

// New - Create a new Proxy instance. Takes over local connection passed in,
// and closes it when finished.
func New(lconn *net.TCPConn, laddr, raddr *net.TCPAddr) *Proxy {
	return &Proxy{
		lconn:  lconn,
		laddr:  laddr,
		raddr:  raddr,
		erred:  false,
		errsig: make(chan bool),
		Log:    pkg.NullLogger{},
	}
}

// NewTLSUnwrapped - Create a new Proxy instance with a remote TLS server for
// which we want to unwrap the TLS to be able to connect without encryption
// locally
func NewTLSUnwrapped(lconn *net.TCPConn, laddr, raddr *net.TCPAddr, addr string) *Proxy {
	p := New(lconn, laddr, raddr)
	p.tlsUnwrapp = true
	p.tlsAddress = addr
	return p
}

type setNoDelayer interface {
	SetNoDelay(bool) error
}

var once sync.Once

func (p Proxy) TimerPrint() {
	once.Do(func() {
		t := time.Now()
		for range time.Tick(time.Second * 30) {
			total := atomic.LoadUint64(&totalSize)
			log.Printf("从 %s 至现在总计加密转发 %s 数据\n", t.Format("2006-01-02 15:04:05"), humanize.Bytes(total))
		}
	})

}

// Start - open connection to remote and start proxying data.
func (p *Proxy) Start() {
	defer p.lconn.Close()
	go p.TimerPrint()
	var err error
	//connect to remote
	if p.tlsUnwrapp {
		p.rconn, err = tls.Dial("tcp", p.tlsAddress, nil)
	} else {
		p.rconn, err = net.DialTCP("tcp", nil, p.raddr)
	}
	if err != nil {
		p.Log.Warn("Remote connection failed: %s", err)
		return
	}
	defer p.rconn.Close()

	//nagles?
	if p.Nagles {
		if conn, ok := p.lconn.(setNoDelayer); ok {
			conn.SetNoDelay(true)
		}
		if conn, ok := p.rconn.(setNoDelayer); ok {
			conn.SetNoDelay(true)
		}
	}

	//display both ends
	p.Log.Info("Opened %s >>> %s", p.laddr.String(), p.raddr.String())

	//bidirectional copy
	go p.pipe(p.lconn, p.rconn, true)
	go p.pipe(p.rconn, p.lconn, false)

	//wait for close...
	<-p.errsig
	p.Log.Info("Closed (%d bytes sent, %d bytes recieved)", p.sentBytes, p.receivedBytes)
}

func (p *Proxy) err(s string, err error) {
	if p.erred {
		return
	}
	if err != io.EOF {
		p.Log.Warn(s, err)
	}
	p.errsig <- true
	p.erred = true
}

func (p *Proxy) pipe(src, dst io.ReadWriter, sendServer bool) {
	islocal := src == p.lconn

	var dataDirection string
	if islocal {
		dataDirection = ">>> %d bytes sent%s"
	} else {
		dataDirection = "<<< %d bytes recieved%s"
	}

	var byteFormat string
	if p.OutputHex {
		byteFormat = "%x"
	} else {
		byteFormat = "%s"
	}

	//directional copy (64k buffer)
	buff := make([]byte, 0xffff)
	for {
		n, err := src.Read(buff)
		if err != nil {
			p.err("Read failed '%s'\n", err)
			return
		}
		b := buff[:n]

		//execute match
		if p.Matcher != nil {
			p.Matcher(b)
		}

		//execute replace
		if p.Replacer != nil {
			b = p.Replacer(b)
		}

		//show output
		p.Log.Debug(dataDirection, n, "")
		p.Log.Trace(byteFormat, b)
		if p.SecretKey != "" {

			if bytes.HasPrefix(b, []byte("start-proxy")) {
				b, err = pkg.AesDecrypt(bytes.TrimLeft(b, "start-proxy"), []byte(p.SecretKey))
				p.Log.Debug("解密后数据包: %s", strings.TrimSpace(string(b)))
			}

			if p.IsClient && sendServer { // 如果是客户端并且发送到服务端的数据全加密
				p.Log.Debug("发送到服务端数据包, 稍后加密数据: %s", strings.TrimSpace(string(b)))
				b, err = pkg.AesEncrypt(b, []byte(p.SecretKey))
				b = append([]byte("start-proxy"), b...)

			}

			if !p.IsClient && !sendServer {
				p.Log.Debug("发送到客户端数据包, 稍后加密数据: %s", strings.TrimSpace(string(b)))
				b, err = pkg.AesEncrypt(b, []byte(p.SecretKey))
				b = append([]byte("start-proxy"), b...)
			}

			if err != nil {
				p.err("Encryption or decryption\n\n failed '%s'\n", err)
				return
			}
		}

		atomic.AddUint64(&totalSize, uint64(len(b)))
		n, err = dst.Write(b)
		if err != nil {
			p.err("Write failed '%s'\n", err)
			return
		}
		if islocal {
			p.sentBytes += uint64(n)
		} else {
			p.receivedBytes += uint64(n)
		}
	}
}
