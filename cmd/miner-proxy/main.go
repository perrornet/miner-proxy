package main

import (
	"flag"
	"fmt"
	"github.com/kardianos/service"
	"log"
	"miner-proxy/pkg"
	"miner-proxy/proxy"
	"miner-proxy/proxy/socks5"
	"miner-proxy/proxy/tcp"
	"strconv"
	"strings"
)

var (
	version   = "0.0.0-src"
	install   = flag.Bool("install", false, "添加到系统服务, 并且开机自动启动")
	remove    = flag.Bool("remove", false, "移除系统服务, 并且关闭开机自动启动")
	restart   = flag.Bool("restart", false, "重启代理服务")
	start     = flag.Bool("start", false, "启动代理服务")
	stat      = flag.Bool("stat", false, "查看代理服务状态")
	proxyType = flag.String("t", "", "启动代理的类型(自动填充, 请勿手动指定)")
)

var (
	proxys = map[string]func() proxy.Proxy{
		"tcp":    tcp.NewTcpArgs,
		"socks5": socks5.Socks2Args,
	}
)

type proxyService struct{}

func (p *proxyService) Start(_ service.Service) error {
	go p.run()
	return nil
}

func (p *proxyService) run() {
	type_ := strings.ToLower(*proxyType)
	if _, ok := proxys[type_]; !ok {
		log.Fatalln("不支持的代理类型")
	}
	proxyObj := proxys[type_]()
	if err := proxyObj.ParseConfigByCmd(); err != nil {
		log.Fatalln(err)
	}
	if err := proxyObj.Run(); err != nil {
		log.Fatalln(err)
	}
}

func (p *proxyService) Stop(_ service.Service) error {
	return nil
}

func main() {
	flag.Parse()
	svcConfig := &service.Config{
		Name:        "miner-proxy",
		DisplayName: "miner-proxy",
		Description: "miner encryption proxy service",
	}
	if *install {
		var i int
		pkg.Input("1: tcp代理\n2: socks5代理\n请在输入列表中的序号, 以便确认您需要运行的代理类型:", func(s string) bool {
			i, _ = strconv.Atoi(s)
			if i == 0 {
				return false
			}
			return true
		})
		var cmds []string
		switch i {
		case 1:
			args := new(tcp.Args)
			if err := args.InputConfig(); err != nil {
				return
			}
			cmds = args.OutputConfig()
			cmds = append(cmds, "-t", "tcp")
		case 2:
		}

		// 创建一个服务
		svcConfig.Arguments = cmds
		s, err := service.New(new(proxyService), svcConfig)
		if err != nil {
			log.Fatalln(err)
		}
		if err := s.Install(); err != nil {
			log.Fatalln("代理服务安装失败", err)
			return
		}
		log.Println("代理服务安装成功")
		pkg.Input("您是否需要立即运行?(y/n):", func(char string) bool {
			char = strings.ToLower(char)
			if char == "y" {
				if err := s.Start(); err != nil {
					log.Fatalln("启动代理服务失败", err)
				}
				log.Println("启动代理服务成功")
				return true
			}
			fmt.Println("您可以后续使用 -start 参数启动服务")
			return true
		})
		return
	}

	if *remove {
		// 创建一个服务
		s, err := service.New(new(proxyService), svcConfig)
		if err != nil {
			log.Fatalln(err)
		}
		s.Stop()
		if err := s.Uninstall(); err != nil {
			log.Fatalln("代理服务移除失败", err)
			return
		}
		log.Println("代理服务移除成功")
		return
	}

	if *restart {
		// 创建一个服务
		s, err := service.New(new(proxyService), svcConfig)
		if err != nil {
			log.Fatalln(err)
		}
		if err := s.Restart(); err != nil {
			log.Fatalln("代理服务重启失败", err)
			return
		}
		log.Println("代理服务重启成功")
		return
	}

	if *stat {
		// 创建一个服务
		s, err := service.New(new(proxyService), svcConfig)
		if err != nil {
			log.Fatalln(err)
		}
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
		s, err := service.New(new(proxyService), svcConfig)
		if err != nil {
			log.Fatalln(err)
		}
		if err := s.Start(); err != nil {
			log.Fatalln("启动代理服务失败", err)
		}
		log.Println("启动代理服务成功")
		return
	}

	s, err := service.New(new(proxyService), svcConfig)
	if err != nil {
		log.Fatalln(err)
	}
	s.Run()
}
