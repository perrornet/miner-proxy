package proxy

import (
	"bytes"
	"fmt"
	"io"
	"miner-proxy/pkg"
	"miner-proxy/pkg/status"
	"net"
	"strings"
	"time"

	"github.com/jmcvetta/randutil"
)

// Proxy - Manages a Proxy connection, piping data between local and remote.
type Proxy struct {
	laddr, raddr         *net.TCPAddr
	lconn, rconn         io.ReadWriteCloser
	erred                bool
	errsig               chan bool
	OutputHex            bool
	SecretKey            string
	IsClient             bool
	SendRemoteAddr       string
	UseSendConfusionData bool
}

// New - Create a new Proxy instance. Takes over local connection passed in,
// and closes it when finished.
func New(lconn *net.TCPConn, laddr, raddr *net.TCPAddr) *Proxy {
	return &Proxy{
		lconn:  lconn,
		laddr:  laddr,
		raddr:  raddr,
		erred:  false,
		errsig: make(chan bool, 1),
	}
}

// Start - open connection to remote and start proxying data.
func (p *Proxy) Start() {
	defer pkg.Recover(true)
	defer p.lconn.Close()
	defer func() {
		if !p.IsClient && p.rconn != nil {
			status.Del(p.lconn.(net.Conn).RemoteAddr().String())
		}
	}()

	conn, err := p.Init()
	if err != nil || conn == nil {
		p.err(fmt.Sprintf("请检查 %s 是否能够联通, 可以使用tcping 工具测试, 并检查该ip所在的防火墙是否开放", p.raddr.String()), nil)
		return
	}

	if !p.IsClient {
		status.Add(p.lconn.(net.Conn).RemoteAddr().String(), 0, conn.RemoteAddr().String())
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
}

func (p *Proxy) err(s string, err error) {
	if p.erred {
		return
	}
	switch err {
	case nil:
		pkg.Warn(s)
	case io.EOF:
	default:
		pkg.Warn(s, err)
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
	for i := 0; i < 100; i++ {
		dataLength, _ := randutil.IntRange(1024, 2048)
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

	pkg.Debug("读取到 %d 明文数据, 加密后数据大小 %d; 加密耗时 %s", n, len(EnData), time.Since(t))
	if c, ok := writer.(net.Conn); ok && !p.IsClient {
		status.Add(c.RemoteAddr().String(), int64(len(data)), reader.(net.Conn).RemoteAddr().String())
	}

	return NewPackage(EnData).Pack(writer)
}

// ReadEncryptionSendPlaintext 读取加密数据, 发送明文
func (p *Proxy) ReadEncryptionSendPlaintext(reader io.Reader, writer io.Writer) error {
	var err error
	readErr := new(Package).Read(reader, func(pck Package) {
		deData, err := p.DecryptData(pck.Data)
		if err != nil {
			p.err("检查客户端密钥是否与服务端密钥一致!", nil)
			return
		}
		if len(deData) == 0 {
			return
		}
		if bytes.HasPrefix(deData, randomStart) {
			pkg.Debug("读取到 %d 随机混淆数据", len(deData))
			return
		}
		pkg.Debug("读取到 %d 加密数据, 解密后数据大小 %d", len(pck.Data), len(deData))
		if c, ok := writer.(net.Conn); ok && !p.IsClient {
			status.Add(reader.(net.Conn).RemoteAddr().String(), int64(len(pck.Data)), c.RemoteAddr().String())
		}
		_, err = writer.Write(deData)
		if err != nil {
			p.err("写入数据到远端失败: %v", err)
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
	for !p.erred {
		time.Sleep(time.Second * time.Duration(sleepTime))

		// 写入随机数据
		index, _ := randutil.IntRange(0, len(randomPingData))
		data, err := p.EncryptionData(append(randomStart, randomPingData[index]...))
		if err != nil {
			return
		}
		pkg.Debug("向客户端写入随机混淆数据 %d", len(data))
		if err := NewPackage(data).Pack(dst); err != nil {
			p.err("客户端关闭了连接, 请检查客户端设置, 或者客户端至本服务器的网络情况", nil)
			return
		}
	}
}

func (p *Proxy) Init() (net.Conn, error) {
	if p.IsClient {
		conn, err := net.Dial("tcp", p.raddr.String())
		if err != nil {
			return nil, err
		}

		// 发送本地内网地址
		clientIp := p.lconn.(net.Conn).RemoteAddr().String()
		EnData, err := p.EncryptionData([]byte(fmt.Sprintf("client_address%s", clientIp)))
		if err != nil {
			return nil, err
		}
		pkg.Debug("向服务端发送客户端地址: %s", clientIp)
		if err := NewPackage(EnData).Pack(conn); err != nil {
			return nil, err
		}
		if p.SendRemoteAddr == "" {
			return conn, nil
		}

		// 发送远端地址
		EnData, err = p.EncryptionData([]byte(fmt.Sprintf("remote_address%s", p.SendRemoteAddr)))
		if err != nil {
			return nil, err
		}
		pkg.Debug("向服务端发送远程地址: %s", p.SendRemoteAddr)
		return conn, NewPackage(EnData).Pack(conn)
	}
	var conn net.Conn
	var err error
	var loopStop bool
	for !loopStop {
		// 服务端读取客户端数据
		err = new(Package).Read(p.lconn, func(pck Package) {
			deData, err := p.DecryptData(pck.Data)
			if err != nil {
				p.err("检查客户端密钥是否与服务端密钥一致!", nil)
				return
			}
			if len(deData) == 0 {
				return
			}
			switch {
			case bytes.HasPrefix(deData, []byte("remote_address")):
				loopStop = true
				deData = bytes.Replace(deData, []byte("remote_address"), nil, 1)
				pkg.Debug("使用客户端指定的远程地址 %s", deData)
				tcpAdder, err := net.ResolveTCPAddr("tcp", string(deData))

				if err != nil {
					p.err("连接客户端发送的远程地址失败", err)
					return
				}

				p.raddr = tcpAdder
				conn, err = net.DialTimeout("tcp", p.raddr.String(), time.Second*10)
				if err != nil {
					p.err(fmt.Sprintf("连接远程地址失败, 请检查 %s 是否能够连接", p.raddr.String()), nil)
					return
				}

				return
			case bytes.HasPrefix(deData, []byte("client_address")):
				deData = bytes.Replace(deData, []byte("client_address"), nil, 1)
				status.Add(p.lconn.(net.Conn).RemoteAddr().String(), 0, "", string(deData))
			default:
				loopStop = true
				// 直接使用服务端指定的远程地址
				conn, err = net.Dial("tcp", p.raddr.String())
				if err != nil {
					p.err(fmt.Sprintf("连接远程地址失败, 请检查 %s 是否能够连接", p.raddr.String()), nil)
					return
				}

				_, err = conn.Write(deData)
				if err != nil {
					p.err("写入数据到远程地址失败", err)
				}
			}
			return
		})
	}
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
	pkg.Debug("开始 %s", name)
	name = fmt.Sprintf("%s error ", name) + "%s"
	for !p.erred {
		err := f(src, dst)
		if p.erred {
			return
		}
		if err != nil {
			switch {
			case strings.Contains(name, "读取加密数据, 发送明文"):
				if p.IsClient {
					p.err("检查服务服是否正确启动, 服务器防火墙是否开启, 本地网络到服务器网络是否畅通, 矿机端是否设置正确", nil)
					return
				}
				p.err("客户端关闭了连接, 请检查矿机/客户端设置", nil)
				return
			default:
				if p.IsClient {
					p.err("矿机关闭了连接, 请检查矿机设置正常", nil)
					return
				}
				p.err("矿池关闭了连接, 请检查矿池/矿机/客户端设置", nil)
			}
			return
		}
	}
}
