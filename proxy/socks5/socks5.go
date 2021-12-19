package socks5

import (
	"bytes"
	"github.com/dustin/go-humanize"
	"github.com/jmcvetta/randutil"
	"github.com/pkg/errors"
	"log"

	"github.com/txthinking/socks5"
	"io"
	"io/ioutil"
	"miner-proxy/pkg"
	"miner-proxy/proxy"
	"miner-proxy/proxy/encryption"
	"miner-proxy/proxy/status"
	"net"
	"time"
)

type ProxyServer struct {
	socks5.DefaultHandle
	sentBytes     uint64
	receivedBytes uint64
	laddr         string
	erred         bool
	errsig        chan bool

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

func NewSocks5Server(laddr string) *ProxyServer {
	return &ProxyServer{
		laddr:  laddr,
		erred:  false,
		errsig: make(chan bool),
		Log:    pkg.NullLogger{},
	}
}

func (p *ProxyServer) err(s string, err error) {
	if p.erred {
		return
	}
	if err != io.EOF {
		p.Log.Warn(s, err)
	}
	p.errsig <- true
	p.erred = true
}

func (p *ProxyServer) NewRequestFrom(r io.Reader) (*socks5.Request, error) {
	bb := make([]byte, 4)
	if _, err := io.ReadFull(r, bb); err != nil {
		return nil, err
	}
	if bb[0] != socks5.Ver {
		return nil, socks5.ErrVersion
	}
	var addr []byte
	if bb[3] == socks5.ATYPIPv4 {
		addr = make([]byte, 4)
		if _, err := io.ReadFull(r, addr); err != nil {
			return nil, err
		}
	} else if bb[3] == socks5.ATYPIPv6 {
		addr = make([]byte, 16)
		if _, err := io.ReadFull(r, addr); err != nil {
			return nil, err
		}
	} else if bb[3] == socks5.ATYPDomain {
		dal := make([]byte, 1)
		if _, err := io.ReadFull(r, dal); err != nil {
			return nil, err
		}
		if dal[0] == 0 {
			return nil, socks5.ErrBadRequest
		}
		addr = make([]byte, int(dal[0]))
		if _, err := io.ReadFull(r, addr); err != nil {
			return nil, err
		}
		addr = append(dal, addr...)
	} else {
		return nil, socks5.ErrBadRequest
	}
	port := make([]byte, 2)
	if _, err := io.ReadFull(r, port); err != nil {
		return nil, err
	}
	return &socks5.Request{
		Ver:     bb[0],
		Cmd:     bb[1],
		Rsv:     bb[2],
		Atyp:    bb[3],
		DstAddr: addr,
		DstPort: port,
	}, nil
}

func (p *ProxyServer) Negotiate(r io.Reader, conn io.Writer, server *socks5.Server) error {
	rq, err := socks5.NewNegotiationRequestFrom(r)
	if err != nil {
		return err
	}
	var got bool
	var m byte
	for _, m = range rq.Methods {
		if m == server.Method {
			got = true
		}
	}
	if !got {
		rp := socks5.NewNegotiationReply(socks5.MethodUnsupportAll)
		var buf = bytes.NewBuffer([]byte{})
		if _, err := rp.WriteTo(buf); err != nil {
			return err
		}
		EnData, err := encryption.EncryptionData(buf.Bytes(), p.UseSendConfusionData, p.SecretKey)
		if err != nil {
			return err
		}
		if err := proxy.NewPackage(EnData).Pack(conn); err != nil {
			return err
		}
	}

	rp := socks5.NewNegotiationReply(server.Method)
	var buf = bytes.NewBuffer([]byte{})

	if _, err := rp.WriteTo(buf); err != nil {
		return err
	}
	EnData, err := encryption.EncryptionData(buf.Bytes(), p.UseSendConfusionData, p.SecretKey)
	if err != nil {
		return err
	}
	if err := proxy.NewPackage(EnData).Pack(conn); err != nil {
		return err
	}
	return nil
}

// Start - open connection to remote and start proxying data.
func (p *ProxyServer) Start() {
	defer pkg.Recover(true)

	defer func() {
		status.AddOnlineCount("socks5", -1)
	}()
	defer close(p.errsig)
	status.AddOnlineCount("socks5")

	l, err := net.ResolveTCPAddr("tcp", p.laddr)
	if err != nil {
		p.Log.Warn("ResolveTCPAddr error %s", err)
		return
	}

	server, err := socks5.NewClassicServer(l.String(), "127.0.0.1", "", "", 3, 3)
	if err != nil {
		p.Log.Warn("NewClassicServer error %s", err)
		return
	}

	server.TCPListen, err = net.ListenTCP("tcp", server.TCPAddr)
	if err != nil {
		p.Log.Warn("ListenTCP error %s", err)
		return
	}
	defer server.TCPListen.Close()
	for {
		c, err := server.TCPListen.AcceptTCP()
		if err != nil {
			p.Log.Warn("AcceptTCP error %s", err)
			return
		}
		go func(c *net.TCPConn) {
			defer c.Close()

			new(proxy.Package).Read(c, func(pck proxy.Package) {
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

				if err := p.Negotiate(bytes.NewBuffer(deData), c, server); err != nil {
					log.Println(err)
					return
				}
			})

			new(proxy.Package).Read(c, func(pck proxy.Package) {
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

				r, err := p.NewRequestFrom(bytes.NewBuffer(deData))
				if err != nil {
					p.err("NewRequestFrom error %s", err)
					return
				}
				var supported bool
				for _, c := range server.SupportedCommands {
					if r.Cmd == c {
						supported = true
						break
					}
				}
				if !supported {
					var pe *socks5.Reply
					if r.Atyp == socks5.ATYPIPv4 || r.Atyp == socks5.ATYPDomain {
						pe = socks5.NewReply(socks5.RepCommandNotSupported, socks5.ATYPIPv4, []byte{0x00, 0x00, 0x00, 0x00}, []byte{0x00, 0x00})
					} else {
						pe = socks5.NewReply(socks5.RepCommandNotSupported, socks5.ATYPIPv6, []byte(net.IPv6zero), []byte{0x00, 0x00})
					}
					var buf = bytes.NewBuffer([]byte{})
					if _, err := pe.WriteTo(buf); err != nil {
						p.err("WriteTo error %s", err)
						return
					}
					EnData, err := encryption.EncryptionData(buf.Bytes(), p.UseSendConfusionData, p.SecretKey)
					if err != nil {
						p.err("EncryptionData error %s", err)
						return
					}
					if err := proxy.NewPackage(EnData).Pack(c); err != nil {
						p.err("Pack error %s", err)
						return
					}
				}

				if err := p.TCPHandle(server, c, r); err != nil {
					log.Println("TCPHandle error ", err)
				}
			})
		}(c)
	}
}

// ReadByPlaintextSendEncryption 读取明文, 发送加密数据
func (p *ProxyServer) ReadByPlaintextSendEncryption(reader io.Reader, writer io.Writer, readerAddress string) error {
	data := make([]byte, 1024)
	n, err := reader.Read(data)
	if err != nil {
		return err
	}
	data = data[:n]
	t := time.Now()
	EnData, err := encryption.EncryptionData(data, p.UseSendConfusionData, p.SecretKey)
	if err != nil {
		return errors.Wrap(err, "EncryptionData error")
	}
	p.Log.Debug("%s plaintext(%s) -%s> client encryption(%s)", readerAddress, humanize.Bytes(uint64(len(data))), time.Since(t), humanize.Bytes(uint64(len(EnData))))
	return proxy.NewPackage(EnData).Pack(writer)
}

// ReadEncryptionSendPlaintext 读取加密数据, 发送明文
func (p *ProxyServer) ReadEncryptionSendPlaintext(reader io.Reader, writer io.Writer, writerAddress string) error {
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
		p.Log.Debug("client encryption(%s) - %s > %s plaintext(%s)",
			humanize.Bytes(uint64(len(pck.Data))),
			time.Since(t),
			writerAddress,
			humanize.Bytes(uint64(len(deData))))
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

func (p *ProxyServer) SendRandomData(dst io.Writer) {
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

func (p *ProxyServer) Connect(w io.Writer, address string, r *socks5.Request) (*net.TCPConn, error) {
	tmp, err := socks5.Dial.Dial("tcp", address)
	if err != nil {
		var pe *socks5.Reply
		if r.Atyp == socks5.ATYPIPv4 || r.Atyp == socks5.ATYPDomain {
			pe = socks5.NewReply(socks5.RepHostUnreachable, socks5.ATYPIPv4, []byte{0x00, 0x00, 0x00, 0x00}, []byte{0x00, 0x00})
		} else {
			pe = socks5.NewReply(socks5.RepHostUnreachable, socks5.ATYPIPv6, []byte(net.IPv6zero), []byte{0x00, 0x00})
		}

		var buf = bytes.NewBuffer([]byte{})
		if _, err := pe.WriteTo(buf); err != nil {
			return nil, err
		}
		EnData, err := encryption.EncryptionData(buf.Bytes(), p.UseSendConfusionData, p.SecretKey)
		if err != nil {

			return nil, err
		}
		if err := proxy.NewPackage(EnData).Pack(w); err != nil {
			p.err("Pack error %s", err)
			return nil, err
		}
		return nil, err
	}

	rc := tmp.(*net.TCPConn)

	a, addr, port, err := socks5.ParseAddress(rc.LocalAddr().String())
	if err != nil {
		var pe *socks5.Reply
		if r.Atyp == socks5.ATYPIPv4 || r.Atyp == socks5.ATYPDomain {
			pe = socks5.NewReply(socks5.RepHostUnreachable, socks5.ATYPIPv4, []byte{0x00, 0x00, 0x00, 0x00}, []byte{0x00, 0x00})
		} else {
			pe = socks5.NewReply(socks5.RepHostUnreachable, socks5.ATYPIPv6, []byte(net.IPv6zero), []byte{0x00, 0x00})
		}

		var buf = bytes.NewBuffer([]byte{})
		if _, err := pe.WriteTo(buf); err != nil {
			return nil, err
		}
		EnData, err := encryption.EncryptionData(buf.Bytes(), p.UseSendConfusionData, p.SecretKey)
		if err != nil {

			return nil, err
		}
		if err := proxy.NewPackage(EnData).Pack(w); err != nil {
			p.err("Pack error %s", err)
			return nil, err
		}
		return nil, err
	}

	pe := socks5.NewReply(socks5.RepSuccess, a, addr, port)

	var buf = bytes.NewBuffer([]byte{})
	if _, err := pe.WriteTo(buf); err != nil {
		return nil, err
	}
	EnData, err := encryption.EncryptionData(buf.Bytes(), p.UseSendConfusionData, p.SecretKey)
	if err != nil {

		return nil, err
	}
	if err := proxy.NewPackage(EnData).Pack(w); err != nil {
		p.err("Pack error %s", err)
		return nil, err
	}
	return rc, nil
}

func (p *ProxyServer) TCPHandle(s *socks5.Server, c *net.TCPConn, r *socks5.Request) error {

	if r.Cmd == socks5.CmdConnect {
		p.Log.Debug("new proxy connect to %s", r.Address())

		rc, err := p.Connect(c, r.Address(), r)
		if err != nil && rc != nil {
			return errors.Wrap(err, "connect error")
		}
		defer func() {
			if rc != nil {
				rc.Close()
			}
		}()
		go func() {
			for { // 读取远端倍代理的数据, 转发到客户端
				if err := p.ReadByPlaintextSendEncryption(rc, c, r.Address()); err != nil {
					p.err("write client error %s", err)
					return
				}
			}
		}()

		// 读取客户端的数据
		for {
			if err := p.ReadEncryptionSendPlaintext(c, rc, r.Address()); err != nil {
				p.err("read client error %s", err)
				return nil
			}
		}
	}

	if r.Cmd == socks5.CmdUDP {
		caddr, err := r.UDP(c, s.ServerAddr)
		if err != nil {
			return errors.Wrap(err, "udp connect error")
		}
		ch := make(chan byte)
		defer close(ch)
		s.AssociatedUDP.Set(caddr.String(), ch, -1)
		defer s.AssociatedUDP.Delete(caddr.String())
		io.Copy(ioutil.Discard, c)
		return nil
	}
	return socks5.ErrUnsupportCmd
}

func (p *ProxyServer) UDPHandle(s *socks5.Server, l *net.UDPAddr, d *socks5.Datagram) error {
	return p.DefaultHandle.UDPHandle(s, l, d)
}
