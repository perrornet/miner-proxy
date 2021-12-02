package proxy

import (
	"crypto/tls"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/jmcvetta/randutil"
	"io"
	"log"
	"miner-proxy/pkg"
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
	tlsUnwrapp    bool
	tlsAddress    string

	Matcher  func([]byte)
	Replacer func([]byte) []byte

	// Settings
	Nagles               bool
	Log                  pkg.Logger
	OutputHex            bool
	SecretKey            string
	IsClient             bool
	UseSendConfusionData bool
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

var (
	once      sync.Once
	startTime = time.Now()
)

func (p Proxy) TimerPrint() {
	once.Do(func() {
		t := time.Now()
		for range time.Tick(time.Second * 30) {
			total := atomic.LoadUint64(&totalSize)

			log.Printf("从 %s 至现在总计加密转发 %s 数据; 平均转发速度 %s/秒 \n",
				t.Format("2006-01-02 15:04:05"),
				humanize.Bytes(total),
				humanize.Bytes(uint64(float64(total)/time.Since(startTime).Seconds())),
			)
		}
	})

}

// Start - open connection to remote and start proxying data.
func (p *Proxy) Start() {
	defer pkg.Recover(true)
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
	go p.pipe(p.lconn, p.rconn)
	go p.pipe(p.rconn, p.lconn)

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
)

// separateConfusionData 分离混淆的数据
func (p *Proxy) separateConfusionData(data []byte) []byte {
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

	EnData, err := p.EncryptionData(data)
	if err != nil {
		return err
	}
	p.Log.Debug("读取到 %d 明文数据, 加密后数据大小 %d", n, len(EnData))
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
		p.Log.Debug("读取到 %d 加密数据, 解密后数据大小 %d", len(pck.Data), len(deData))
		atomic.AddUint64(&totalSize, uint64(len(pck.Data)))
		_, err = writer.Write(deData)
		if err != nil {
			fmt.Println(err)
		}
	})
	if readErr != nil {
		return err
	}
	return err
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
		if err := f(src, dst); err != nil {
			p.err(name, err)
			return
		}
	}
}
