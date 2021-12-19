package socks5

import (
	"flag"
	"github.com/jmcvetta/randutil"
	"log"
	"miner-proxy/pkg"
	"miner-proxy/proxy"
	"time"
)

var (
	randomStart = []byte("random-proxy")
	// 启动时随机生成
	randomPingData [][]byte
)

func InitSocks5Data() {
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

type Args struct {
	proxy.Args
	RemoteAddr string `json:"remote_addr" notes:"远程代理地址或者远程本程序的监听地址"`
}

func Socks2Args() proxy.Proxy {
	return new(Args)
}

// ParseConfigByCmd 从命令行中解析配置到程序中, 这个是服务真正运行时调用
func (r *Args) ParseConfigByCmd() error {
	flag.Parse()
	r.LocalAddr = *proxy.LocalAddr
	r.RemoteAddr = *proxy.RemoteAddr
	r.SecretKey = *proxy.SecretKey
	r.IsClient = *proxy.IsClient
	r.UseSendConfusionData = *proxy.UseSendConfusionData
	r.Debug = *proxy.Debug
	return nil
}

// InputConfig 让用户输入配置信息, 当用户使用-install参数时调用
func (a *Args) InputConfig() error {
	a.Args.InputConfig()
	if a.IsClient {
		pkg.Input("请输入本程序的服务端地址(默认0.0.0.0:9999):", func(s string) bool {
			a.RemoteAddr = s
			return true
		})
	}
	return nil
}

// OutputConfig 将运行时的参数返回, 当用户使用-install参数并且在 调用完 InputConfig 之后调用
func (r *Args) OutputConfig() []string {
	var cmds []string
	cmds = append(cmds, "-l", r.LocalAddr)
	if r.IsClient {
		cmds = append(cmds, "-client")
		cmds = append(cmds, "-r", r.RemoteAddr)
	}

	if r.UseSendConfusionData {
		cmds = append(cmds, "-sc")
	}

	if r.Debug {
		cmds = append(cmds, "-debug")
	}
	return cmds
}

// Run 运行服务, 在运行 ParseConfigByCmd 之后调用
func (a *Args) Run() error {
	if len(a.SecretKey)%16 != 0 {
		for len(a.SecretKey)%16 != 0 {
			a.SecretKey += "0"
		}
	}
	InitSocks5Data()
	if a.IsClient {
		s := NewSocks5Client(a.LocalAddr, a.RemoteAddr, time.Second*10)
		s.IsClient = a.IsClient
		s.Log = pkg.ColorLogger{
			Verbose: a.Debug,
			Prefix:  "socks5 >>",
		}
		s.SecretKey = a.SecretKey
		s.UseSendConfusionData = a.UseSendConfusionData
		return s.Run()
	}

	log.Println("start server", a.LocalAddr)
	s := NewSocks5Server(a.LocalAddr)
	s.IsClient = a.IsClient
	s.Log = pkg.ColorLogger{
		Verbose: a.Debug,
		Prefix:  "socks5 >>",
	}
	s.SecretKey = a.SecretKey
	s.UseSendConfusionData = a.UseSendConfusionData
	s.Start()
	return nil
}
