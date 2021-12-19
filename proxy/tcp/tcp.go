package tcp

import (
	"bytes"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/jmcvetta/randutil"
	"io"
	"log"
	"miner-proxy/pkg"
	"miner-proxy/proxy"
	"miner-proxy/proxy/encryption"
	"net"
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
	tlsAddress    string

	Matcher  func([]byte)
	Replacer func([]byte) []byte

	// Settings
	Nagles               bool
	Log                  pkg.Logger
	OutputHex            bool
	SecretKey            string
	IsClient             bool
	SendRemoteAddr       string
	UseSendConfusionData bool
}

var (
	totalSize   uint64
	onlineCount int64
)

// New - Create a new Proxy instance. Takes over local connection passed in,
// and closes it when finished.
func NewTcp(lconn *net.TCPConn, laddr, raddr *net.TCPAddr) *Proxy {
	return &Proxy{
		lconn:  lconn,
		laddr:  laddr,
		raddr:  raddr,
		erred:  false,
		errsig: make(chan bool),
		Log:    pkg.NullLogger{},
	}
}

var (
	once        sync.Once
	startTime   = time.Now()
	randomStart = []byte("random-proxy")
	// 启动时随机生成
	randomPingData [][]byte
)

func InitTcpData() {
	for i := 0; i < 1000; i++ {
		dataLength, _ := randutil.IntRange(10, 102)
		var temp = make([]byte, dataLength)
		for l := 0; l < dataLength; l++ {
			char, _ := randutil.IntRange(0, 255)
			temp[l] = uint8(char)
		}
		randomPingData = append(randomPingData, temp)
	}
	go once.Do(func() {
		t := time.Now()
		for range time.Tick(time.Second * 30) {
			total := atomic.LoadUint64(&totalSize)
			log.Printf("从 %s 至现在总计加密转发 %s 数据; 平均转发速度 %s/秒; 当前在线 %d 个客户端 \n",
				t.Format("2006-01-02 15:04:05"),
				humanize.Bytes(total),
				humanize.Bytes(uint64(float64(total)/time.Since(startTime).Seconds())),
				atomic.LoadInt64(&onlineCount),
			)
		}
	})
}

// Start - open connection to remote and start proxying data.
func (p *Proxy) Start() {
	defer pkg.Recover(true)
	defer p.lconn.Close()
	defer func() {
		atomic.AddInt64(&onlineCount, -1)
	}()
	atomic.AddInt64(&onlineCount, 1)
	conn, err := p.connRemote()
	if err != nil {
		p.err("Remote connection failed: %s", err)
		return
	}
	p.rconn = conn
	defer p.rconn.Close()

	//bidirectional copy
	go p.pipe(p.lconn, p.rconn)
	go p.pipe(p.rconn, p.lconn)

	if !p.IsClient { // 由于挖矿的特性, 只需要在服务端向客户端 发送随机数据
		go p.SendRandomData(p.lconn)
	}

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

// ReadByPlaintextSendEncryption 读取明文, 发送加密数据
func (p *Proxy) ReadByPlaintextSendEncryption(reader io.Reader, writer io.Writer) error {
	data := make([]byte, 1024)
	n, err := reader.Read(data)
	if err != nil {
		return err
	}
	data = data[:n]
	t := time.Now()
	EnData, err := encryption.EncryptionData(data, p.UseSendConfusionData, p.SecretKey)
	if err != nil {
		return err
	}
	p.Log.Debug("plaintext(%s) -%s> encryption(%s)", humanize.Bytes(uint64(len(data))), time.Since(t), humanize.Bytes(uint64(len(EnData))))
	atomic.AddUint64(&totalSize, uint64(len(EnData)))
	return proxy.NewPackage(EnData).Pack(writer)
}

// ReadEncryptionSendPlaintext 读取加密数据, 发送明文
func (p *Proxy) ReadEncryptionSendPlaintext(reader io.Reader, writer io.Writer) error {
	var err error
	readErr := new(proxy.Package).Read(reader, func(pck proxy.Package) {
		t := time.Now()
		deData, err := encryption.DecryptData(pck.Data, p.UseSendConfusionData, p.SecretKey)
		if err != nil {
			p.err("DecryptData error %s", err)
		}
		if bytes.HasPrefix(deData, randomStart) {
			p.Log.Debug("get random confusion data %s", humanize.Bytes(uint64(len(deData))))
			return
		}
		p.Log.Debug("encryption(%s) -%s> plaintext(%s)", humanize.Bytes(uint64(len(pck.Data))), time.Since(t), humanize.Bytes(uint64(len(deData))))
		atomic.AddUint64(&totalSize, uint64(len(pck.Data)))
		_, err = writer.Write(deData)
		if err != nil {
			p.err("write data to server or client error", err)
			return
		}
	})
	if readErr != nil {
		return readErr
	}
	return err
}

func (p *Proxy) SendRandomData(dst io.Writer) {
	sleepTime, _ := randutil.IntRange(3, 15)
	for {
		time.Sleep(time.Second * time.Duration(sleepTime))

		// 写入随机数据
		index, _ := randutil.IntRange(0, len(randomPingData))
		data, err := encryption.EncryptionData(append(randomStart, randomPingData[index]...), p.UseSendConfusionData, p.SecretKey)
		if err != nil {
			return
		}
		p.Log.Debug("write random confusion data to client, data size %s", humanize.Bytes(uint64(len(data))))
		if err := proxy.NewPackage(data).Pack(dst); err != nil {
			return
		}
	}
}

func (p *Proxy) connRemote() (net.Conn, error) {
	if p.IsClient {
		conn, err := net.Dial("tcp", p.raddr.String())
		if err != nil {
			return nil, err
		}
		p.Log.Info("Opened %s >>> %s", conn.LocalAddr().String(), conn.RemoteAddr())
		if p.SendRemoteAddr == "" {
			return conn, nil
		}
		EnData, err := encryption.EncryptionData([]byte(fmt.Sprintf("remote_address%s", p.SendRemoteAddr)),
			p.UseSendConfusionData, p.SecretKey)
		if err != nil {
			return nil, err
		}
		p.Log.Debug("send remote address %s to server", p.SendRemoteAddr)
		atomic.AddUint64(&totalSize, uint64(len(EnData)))
		return conn, proxy.NewPackage(EnData).Pack(conn)
	}

	// 服务端读取远端数据
	var conn net.Conn
	var err error
	err = new(proxy.Package).Read(p.lconn, func(pck proxy.Package) {
		deData, err := encryption.DecryptData(pck.Data, p.UseSendConfusionData, p.SecretKey)
		if err != nil {
			p.err("decryptData error %s", err)
		}
		if bytes.HasPrefix(deData, randomStart) {
			p.Log.Debug("get random confusion data %s", humanize.Bytes(uint64(len(deData))))
			return
		}
		if bytes.HasPrefix(deData, []byte("remote_address")) {
			deData = bytes.Replace(deData, []byte("remote_address"), nil, 1)
			p.Log.Debug("use client send remote address %s", deData)
			tcpAdder, err := net.ResolveTCPAddr("tcp", string(deData))
			if err != nil {
				p.err("connection client send remote address error ", err)
				return
			}
			p.raddr = tcpAdder
			conn, err = net.Dial("tcp", p.raddr.String())
			if err != nil {
				p.err("connection client send remote address  error ", err)
				return
			}
			if v, ok := p.Log.(*pkg.ColorLogger); ok {
				v.Prefix = fmt.Sprintf("connection %s(client send) >>", p.raddr.String())
			}
			p.Log.Info("Opened %s >>> %s", p.laddr.String(), tcpAdder.String())
			return
		}

		// 直接使用服务端指定的远程地址
		conn, err = net.Dial("tcp", p.raddr.String())
		if err != nil {
			p.err("connection remote address  error ", err)
			return
		}
		p.Log.Info("Opened %s >>> %s", conn.LocalAddr().String(), conn.RemoteAddr().String())
		_, err = conn.Write(deData)
		if err != nil {
			p.err("write data to server error", err)
		}
		return
	})

	return conn, err
}

func (p *Proxy) pipe(src, dst io.ReadWriter) {
	defer pkg.Recover(true)
	islocal := src == p.lconn
	var f func(reader io.Reader, writer io.Writer) error
	var name string
	switch {
	case p.IsClient == islocal:
		name = "read plaintext data send encryption data"
		f = p.ReadByPlaintextSendEncryption
	default:
		name = "read encryption data send plaintext data"
		f = p.ReadEncryptionSendPlaintext
	}
	name = fmt.Sprintf("%s error ", name) + "%s"
	for {
		err := f(src, dst)
		if err != nil {
			p.err(name, err)
			return
		}
	}
}
