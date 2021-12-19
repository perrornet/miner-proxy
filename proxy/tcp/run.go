package tcp

import (
	"flag"
	"fmt"
	"github.com/pkg/errors"
	"log"
	"miner-proxy/pkg"
	"miner-proxy/proxy"
	"net"
)

type Args struct {
	proxy.Args
	RemoteAddr     string `json:"remote_addr" notes:"远程代理地址或者远程本程序的监听地址"`
	SendRemoteAddr string `json:"send_remote_addr" notes:"客户端如果设置了这个参数, 那么服务端将会直接使用客户端的参数连接"`
}

func NewTcpArgs() proxy.Proxy {
	return new(Args)
}

func (r *Args) ParseConfigByCmd() error {
	flag.Parse()
	r.LocalAddr = *proxy.LocalAddr
	r.RemoteAddr = *proxy.RemoteAddr
	r.SecretKey = *proxy.SecretKey
	r.IsClient = *proxy.IsClient
	r.UseSendConfusionData = *proxy.UseSendConfusionData
	r.Debug = *proxy.Debug
	r.SendRemoteAddr = *proxy.SendRemoteAddr
	return nil
}

func (r *Args) OutputConfig() []string {
	var cmds []string
	cmds = append(cmds, "-l", r.LocalAddr)
	cmds = append(cmds, "-r", r.RemoteAddr)
	if r.SendRemoteAddr != "" {
		cmds = append(cmds, "-sr", r.SendRemoteAddr)
	}

	cmds = append(cmds, "-secret_key", r.SecretKey)
	if r.IsClient {
		cmds = append(cmds, "-client")
	}

	if r.UseSendConfusionData {
		cmds = append(cmds, "-sc")
	}

	if r.Debug {
		cmds = append(cmds, "-debug")
	}
	return cmds
}

func (r *Args) InputConfig() error {
	if err := r.Args.InputConfig(); err != nil {
		return err
	}

	if r.IsClient {
		pkg.Input("请输入本程序的服务端地址(默认0.0.0.0:9999):", func(s string) bool {
			r.RemoteAddr = s
			return true
		})
		pkg.Input("请您输入指定的矿池地址, 如果不输入将会使用服务端设置的矿池:", func(s string) bool {
			r.SendRemoteAddr = s
			return true
		})
	}

	if !r.IsClient {
		pkg.Input("请输入默认矿池地址(启动客户端时也可以指定):", func(s string) bool {
			r.RemoteAddr = s
			return true
		})
	}

	return nil
}

func (r Args) Run() error {
	InitTcpData()
	laddr, err := net.ResolveTCPAddr("tcp", r.LocalAddr)
	if err != nil {
		return errors.Wrap(err, "failed to resolve local address")
	}
	raddr, err := net.ResolveTCPAddr("tcp", r.LocalAddr)
	if err != nil {
		return errors.Wrap(err, "failed to resolve remote address")
	}
	listener, err := net.ListenTCP("tcp", laddr)
	if err != nil {
		return errors.Wrap(err, "failed to open local port to listen")
	}

	if len(r.SecretKey)%16 != 0 {
		for len(r.SecretKey)%16 != 0 {
			r.SecretKey += "0"
		}
	}
	log.Println("tcp代理服务运行成功, 可供连接的地址为", r.LocalAddr)
	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			continue
		}
		p := NewTcp(conn, laddr, raddr)
		p.SecretKey = r.SecretKey
		p.IsClient = r.IsClient
		p.SendRemoteAddr = r.SendRemoteAddr
		p.UseSendConfusionData = r.UseSendConfusionData
		l := &pkg.ColorLogger{
			Verbose: r.Debug,
		}
		if r.IsClient {
			l.Prefix = fmt.Sprintf("connection %s>> ", raddr.String())
		}

		if !r.IsClient {
			l.Prefix = fmt.Sprintf("connection %s>> ", conn.RemoteAddr().String())
		}
		p.Log = l
		go p.Start()
	}
}
