package socks5

import (
	"bytes"
	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
	"github.com/txthinking/socks5"
	"io"
	"miner-proxy/pkg"
	"miner-proxy/proxy"
	"miner-proxy/proxy/encryption"
	"net"
	"time"
)

type clientProxy struct {
	socks5.DefaultHandle
	tcpLocalAddr, remoteAddr string
	Log                      pkg.Logger
	SecretKey                string
	IsClient                 bool
	UseSendConfusionData     bool
	timeout                  int
	isStop                   bool
}

func (p *clientProxy) err(msg string, err error) {
	if err != io.EOF {
		p.Log.Warn(msg, err)
	}
	p.isStop = true
}

func NewSocks5Client(tcpLocalAddr, remoteAddr string, timeout time.Duration) *clientProxy {
	return &clientProxy{
		tcpLocalAddr: tcpLocalAddr,
		remoteAddr:   remoteAddr,
		timeout:      int(timeout.Seconds()),
		Log:          pkg.NullLogger{},
	}
}

func (p *clientProxy) Run() error {
	if p.IsClient {
		li, err := net.Listen("tcp", p.tcpLocalAddr)
		if err != nil {
			return err
		}
		for {
			conn, err := li.Accept()
			if err != nil {
				return err
			}
			go p.Client(conn)
		}
	}
	return nil
}

// ReadEncryptionSendPlaintext 读取加密数据, 发送明文
func (p *clientProxy) ReadEncryptionSendPlaintext(reader io.Reader, writer io.Writer) error {
	return new(proxy.Package).Read(reader, func(pck proxy.Package) {
		t := time.Now()
		deData, err := encryption.DecryptData(pck.Data, p.UseSendConfusionData, p.SecretKey)
		if err != nil {
			p.Log.Warn("DecryptData error %s", err)
			return
		}
		if bytes.HasPrefix(deData, randomStart) {
			p.Log.Debug("get random confusion data %s", humanize.Bytes(uint64(len(deData))))
			return
		}
		p.Log.Debug("encryption(%s) -%s> plaintext(%s)", humanize.Bytes(uint64(len(pck.Data))), time.Since(t), humanize.Bytes(uint64(len(deData))))
		_, err = writer.Write(deData)
		if err != nil {
			p.err("write data to server or client error %s", err)
			return
		}
	})
}

// ReadByPlaintextSendEncryption 读取明文, 发送加密数据
func (p *clientProxy) ReadByPlaintextSendEncryption(reader io.Reader, writer io.Writer) error {
	data := make([]byte, 1024)
	n, err := reader.Read(data)
	if err != nil {
		return errors.Wrap(err, "read remote error")
	}
	data = data[:n]
	t := time.Now()
	EnData, err := encryption.EncryptionData(data, p.UseSendConfusionData, p.SecretKey)
	if err != nil {
		return errors.Wrap(err, "EncryptionData error")
	}
	p.Log.Debug("plaintext(%s) -%s> encryption(%s)", humanize.Bytes(uint64(len(data))), time.Since(t), humanize.Bytes(uint64(len(EnData))))
	return proxy.NewPackage(EnData).Pack(writer)
}

func (p *clientProxy) Client(lconn net.Conn) {
	// 连接远程服务器
	p.Log.Debug("new connection to server %s", p.remoteAddr)
	rconn, err := net.Dial("tcp", p.remoteAddr)
	if err != nil {
		p.err("Dial error %s", err)
		return
	}
	defer rconn.Close()
	go func() { // 服务端发来的数据
		for !p.isStop {
			// 需要解密
			if err := p.ReadEncryptionSendPlaintext(rconn, lconn); err != nil {
				p.err("ReadEncryptionSendPlaintext error %s", err)
				return
			}
		}
	}()
	// 客户端发来的数据
	for !p.isStop {
		if err := p.ReadByPlaintextSendEncryption(lconn, rconn); err != nil {
			p.err("ReadEncryptionSendPlaintext error %s", err)
			return
		}
	}
	return
}
