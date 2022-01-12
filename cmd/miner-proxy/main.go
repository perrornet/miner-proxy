package main

import (
	"flag"
	"fmt"
	"log"
	"miner-proxy/pkg"
	"miner-proxy/proxy"
	"net"
	"os"
	"strings"

	"github.com/kardianos/service"
)

var (
	localAddr            = flag.String("l", ":9999", "本地监听地址")
	remoteAddr           = flag.String("r", "localhost:80", "远程代理地址或者远程本程序的监听地址")
	SendRemoteAddr       = flag.String("sr", "", "客户端如果设置了这个参数, 那么服务端将会直接使用客户端的参数连接")
	secretKey            = flag.String("secret_key", "", "数据包加密密钥, 只有远程地址也是本服务时才可使用")
	isClient             = flag.Bool("client", false, "是否是客户端, 该参数必须准确, 默认服务端, 只有 secret_key 不为空时需要区分")
	UseSendConfusionData = flag.Bool("sc", false, "是否使用混淆数据, 如果指定了, 将会不定时在server/client之间发送随机的混淆数据以及在挖矿数据中插入随机数据")
	debug                = flag.Bool("debug", false, "是否开启debug")
	install              = flag.Bool("install", false, "添加到系统服务, 并且开机自动启动")
	remove               = flag.Bool("remove", false, "移除系统服务, 并且关闭开机自动启动")
	stop                 = flag.Bool("stop", false, "暂停代理服务")
	restart              = flag.Bool("restart", false, "重启代理服务")
	start                = flag.Bool("start", false, "启动代理服务")
	stat                 = flag.Bool("stat", false, "查看代理服务状态")
)

type proxyService struct{}

func (p *proxyService) Start(_ service.Service) error {
	go p.run()
	return nil
}

func (p *proxyService) run() {

	logger := pkg.ColorLogger{}

	logger.Info("miner-proxy  proxing from %v to %v ", *localAddr, *remoteAddr)

	laddr, err := net.ResolveTCPAddr("tcp", *localAddr)
	if err != nil {
		logger.Warn("Failed to resolve local address: %s", err)
		os.Exit(1)
	}
	raddr, err := net.ResolveTCPAddr("tcp", *remoteAddr)
	if err != nil {
		logger.Warn("Failed to resolve remote address: %s", err)
		os.Exit(1)
	}
	listener, err := net.ListenTCP("tcp", laddr)
	if err != nil {
		logger.Warn("Failed to open local port to listen: %s", err)
		os.Exit(1)
	}

	if len(*secretKey)%16 != 0 {
		for len(*secretKey)%16 != 0 {
			*secretKey += "0"
		}
	}

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			logger.Warn("Failed to accept connection '%s'", err)
			continue
		}
		p := proxy.New(conn, laddr, raddr)
		p.SecretKey = *secretKey
		p.IsClient = *isClient
		p.SendRemoteAddr = *SendRemoteAddr
		p.UseSendConfusionData = *UseSendConfusionData
		l := &pkg.ColorLogger{
			Verbose: *debug,
		}
		if *isClient {
			l.Prefix = fmt.Sprintf("connection %s>> ", raddr.String())
		}

		if !*isClient {
			l.Prefix = fmt.Sprintf("connection %s>> ", conn.RemoteAddr().String())
		}
		p.Log = l
		go p.Start()
	}
}

func (p *proxyService) Stop(_ service.Service) error {
	return nil
}

func getArgs() []string {
	var result []string
	cmds := []string{
		"install", "remove", "stop", "restart", "start", "stat",
	}
A:
	for _, v := range os.Args[1:] {
		for _, c := range cmds {
			if strings.Contains(v, c) {
				continue A
			}
		}
		result = append(result, v)
	}
	return result
}

func main() {
	flag.Parse()
	svcConfig := &service.Config{
		Name:        "miner-proxy",
		DisplayName: "miner-proxy",
		Description: "miner encryption proxy service",
		Arguments:   getArgs(),
	}
	s, err := service.New(new(proxyService), svcConfig)
	if err != nil {
		log.Fatalln(err)
	}
	if *install {
		if err := s.Install(); err != nil {
			log.Fatalln("代理服务安装失败", err)
			return
		}
		log.Println("代理服务安装成功")
		return
	}

	if *remove {
		if err := s.Uninstall(); err != nil {
			log.Fatalln("移除代理系统服务失败", err)
		}
		log.Println("成功移除代理系统服务")
		return
	}

	if *stop {
		if err := s.Stop(); err != nil {
			log.Fatalln("停止代理服务失败", err)
		}
		log.Println("成功停止代理服务")
		return
	}

	if *restart {
		if err := s.Restart(); err != nil {
			log.Fatalln("重启代理服务失败", err)
		}
		log.Println("成功重启代理服务")
		return
	}
	if *stat {
		stat, err := s.Status()
		if err != nil {
			log.Fatalln("获取代理服务状态失败", err)
		}
		switch stat {
		case service.StatusUnknown:
			log.Println("未知的状态")
		case service.StatusRunning:
			log.Println("运行中")
		case service.StatusStopped:
			log.Println("停止运行")
		}
		return
	}

	if *start {
		if err := s.Start(); err != nil {
			log.Fatalln("启动代理服务失败", err)
		}
		log.Println("启动代理服务成功")
		return
	}
	_ = s.Run()
}
