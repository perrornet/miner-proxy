package proxy

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"miner-proxy/pkg"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jmcvetta/randutil"
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

var (
	once      sync.Once
	startTime = time.Now()
)

func init() {
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

var (
	proxyStart = []byte{87, 62, 64, 57, 136, 6, 18, 50, 118, 135, 214, 247}
	proxyEnd   = []byte{93, 124, 242, 154, 241, 48, 161, 242, 209, 90, 73, 163}
	// proxyJustConfusionStart 只是混淆数据才回使用的开头
	//proxyJustConfusionStart = []byte{113,158,190,157,204,56,4,142,189,85,168,56}
	proxyConfusionStart = []byte{178, 254, 235, 166, 15, 61, 52, 198, 83, 207, 6, 83, 183, 115, 50, 58, 110, 6, 13, 60, 143, 242, 254, 143}
	proxyConfusionEnd   = []byte{114, 44, 203, 23, 55, 50, 148, 231, 241, 154, 112, 180, 115, 126, 148, 149, 180, 55, 115, 242, 98, 119, 170, 249}
	randomStart         = []byte("random-proxy")
	// 启动时随机生成
	randomPingData [][]byte
)

func init() {
	for i := 0; i < 1000; i++ {
		dataLength, _ := randutil.IntRange(10, 102)
		var temp = make([]byte, dataLength)
		for l := 0; l < dataLength; l++ {
			char, _ := randutil.IntRange(0, 255)
			temp[l] = uint8(char)
		}
		randomPingData = append(randomPingData, temp)
	}
}

// separateConfusionData 分离混淆的数据
func (p *Proxy) separateConfusionData(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	if !p.UseSendConfusionData {
		return data
	}
	var result = make([]byte, 0, len(data)/2)
	for index, v := range data {
		if index%2 == 0 {
			continue
		}
		result = append(result, v)
	}
	return result
}

// buildConfusionData 构建混淆数据
// 从 10 - 135中随机一个数字作为本次随机数据的长度 N
// 循环 N 次, 每次从 1 - 255 中随机一个数字作为本次随机数据
// 最后在头部加入 proxyConfusionStart 尾部加入 proxyConfusionStart
func (p *Proxy) buildConfusionData() []byte {
	number, _ := randutil.IntRange(10, 135)
	var data = make([]byte, number)
	for i := 0; i < number; i++ {
		index, _ := randutil.IntRange(1, 255)
		data[i] = uint8(index)
	}
	data = append(data, proxyConfusionEnd...)
	return append(proxyConfusionStart, data...)
}

// EncryptionData 构建需要发送的加密数据
// 先使用 SecretKey aes 加密 data 如果 UseSendConfusionData 等于 true
// 那么将会每25个字符插入 buildConfusionData 生成的随机字符
func (p *Proxy) EncryptionData(data []byte) ([]byte, error) {
	if p.UseSendConfusionData { // 插入随机混淆数据
		confusionData := p.buildConfusionData()
		confusionData = confusionData[len(proxyConfusionStart) : len(confusionData)-len(proxyConfusionEnd)]
		var result []byte
		for _, v := range data {
			result = append(result, confusionData[0])
			confusionData = append(confusionData[1:], confusionData[0])
			result = append(result, v)
		}
		data = result
	}
	data, err := pkg.AesEncrypt(data, []byte(p.SecretKey))
	if err != nil {
		return nil, err
	}
	data = append(proxyStart, data...)
	return append(data, proxyEnd...), nil
}

// DecryptData 解密数据
func (p *Proxy) DecryptData(data []byte) ([]byte, error) {
	if len(data) < len(data)-len(proxyEnd) || len(data)-len(proxyEnd) <= 0 {
		return nil, nil
	}
	data = data[len(proxyStart) : len(data)-len(proxyEnd)]
	data, err := pkg.AesDecrypt(data, []byte(p.SecretKey))
	if err != nil {
		return nil, err
	}
	if p.UseSendConfusionData { // 去除随机混淆数据
		data = p.separateConfusionData(data)
	}
	return data, nil
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
	EnData, err := p.EncryptionData(data)
	if err != nil {
		return err
	}
	p.Log.Debug("读取到 %d 明文数据, 加密后数据大小 %d; 加密耗时 %s", n, len(EnData), time.Since(t))
	atomic.AddUint64(&totalSize, uint64(len(EnData)))
	return NewPackage(EnData).Pack(writer)
}

// ReadEncryptionSendPlaintext 读取加密数据, 发送明文
func (p *Proxy) ReadEncryptionSendPlaintext(reader io.Reader, writer io.Writer) error {
	var err error
	readErr := new(Package).Read(reader, func(pck Package) {
		deData, err := p.DecryptData(pck.Data)
		if err != nil {
			p.err("DecryptData error %s", err)
		}
		if len(deData) == 0 {
			return
		}
		if bytes.HasPrefix(deData, randomStart) {
			p.Log.Debug("读取到 %d 随机混淆数据", len(deData))
			return
		}
		p.Log.Debug("读取到 %d 加密数据, 解密后数据大小 %d", len(pck.Data), len(deData))
		atomic.AddUint64(&totalSize, uint64(len(pck.Data)))
		_, err = writer.Write(deData)
		if err != nil {
			fmt.Println(err)
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
		data, err := p.EncryptionData(append(randomStart, randomPingData[index]...))
		if err != nil {
			return
		}
		p.Log.Debug("向客户端写入随机混淆数据 %d", len(data))
		if err := NewPackage(data).Pack(dst); err != nil {
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
		EnData, err := p.EncryptionData([]byte(fmt.Sprintf("remote_address%s", p.SendRemoteAddr)))
		if err != nil {
			return nil, err
		}
		p.Log.Debug("向服务端发送远程地址: %s", p.SendRemoteAddr)
		atomic.AddUint64(&totalSize, uint64(len(EnData)))
		return conn, NewPackage(EnData).Pack(conn)
	}

	// 服务端读取远端数据
	var conn net.Conn
	var err error
	err = new(Package).Read(p.lconn, func(pck Package) {
		deData, err := p.DecryptData(pck.Data)
		if err != nil {
			p.err("DecryptData error %s", err)
		}
		if len(deData) == 0 {
			return
		}
		if bytes.HasPrefix(deData, randomStart) {
			p.Log.Debug("读取到 %d 随机混淆数据", len(deData))
			return
		}
		if bytes.HasPrefix(deData, []byte("remote_address")) {
			deData = bytes.Replace(deData, []byte("remote_address"), nil, 1)
			p.Log.Debug("使用客户端指定的远程地址 %s", deData)
			tcpAdder, err := net.ResolveTCPAddr("tcp", string(deData))
			if err != nil {
				p.err("连接客户端发送的远程地址失败", err)
				return
			}
			p.raddr = tcpAdder
			conn, err = net.Dial("tcp", p.raddr.String())
			if err != nil {
				p.err("连接远程地址失败", err)
				return
			}
			if v, ok := p.Log.(*pkg.ColorLogger); ok {
				v.Prefix = fmt.Sprintf("connection %s(客户端指定) >>", p.raddr.String())
			}
			p.Log.Info("Opened %s >>> %s", p.laddr.String(), tcpAdder.String())
			return
		}

		// 直接使用服务端指定的远程地址
		conn, err = net.Dial("tcp", p.raddr.String())
		if err != nil {
			p.err("连接远程地址失败", err)
			return
		}
		p.Log.Info("Opened %s >>> %s", conn.LocalAddr().String(), conn.RemoteAddr().String())
		_, err = conn.Write(deData)
		if err != nil {
			p.err("写入数据到远程地址失败", err)
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
		name = "读取明文, 发送加密数据"
		f = p.ReadByPlaintextSendEncryption
	default:
		name = "读取加密数据, 发送明文"
		f = p.ReadEncryptionSendPlaintext
	}
	p.Log.Debug("开始 %s", name)
	name = fmt.Sprintf("%s error ", name) + "%s"
	for {
		err := f(src, dst)
		if err != nil {
			p.err(name, err)
			return
		}
	}
}
