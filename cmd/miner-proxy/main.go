package main

import (
	"flag"
	"fmt"
	"log"
	"miner-proxy/pkg"
	"miner-proxy/pkg/status"
	"miner-proxy/proxy"
	"miner-proxy/proxy/wxPusher"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jmcvetta/randutil"
	"github.com/kardianos/service"
	"github.com/liushuochen/gotable"
	"go.uber.org/zap/zapcore"
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
	logFile              = flag.String("log_file", "", "将日志输入到文件中, 示例: ./miner-proxy.log")
	randomSendHttp       = flag.Bool("rsh", false, "是否随机时间发送随机http请求混淆, 支持客户端")
	wxPusherToken        = flag.String("wx", "", "掉线微信通知token, 该参数只有在服务端生效, ,请在 https://wxpusher.zjiecode.com/admin/main/app/appToken 注册获取appToken")
	newWxPusherUser      = flag.Bool("add_wx_user", false, "绑定微信账号到微信通知中, 该参数只有在服务端生效")
	offlineTime          = flag.Int64("offline", 60*4, "掉线多少秒之后, 发送微信通知")
)

var (
	// build 时加入
	gitCommit string
)
var (
	reqeustUrls = []string{
		"https://www.baidu.com/",
		"https://m.baidu.com/",
		"https://www.jianshu.com/",
		"https://www.jianshu.com/p/4fbdab9fb44c",
		"https://www.jianshu.com/p/5d25218fb22d",
		"https://www.tencent.com/",
		"https://tieba.baidu.com/",
	}
)

type proxyService struct{}

func (p *proxyService) checkWxPusher() error {
	if len(*wxPusherToken) <= 10 {
		pkg.Fatal("您输入的微信通知token无效, 请在 https://wxpusher.zjiecode.com/admin/main/app/appToken 中获取")
	}
	w := wxPusher.NewPusher(*wxPusherToken)
	if *newWxPusherUser {
		qrUrl, err := w.ShowQrCode()
		if err != nil {
			pkg.Fatal("获取二维码url失败: %s", err.Error())
		}
		fmt.Printf("请复制网址, 在浏览器打开, 并使用微信进行扫码登陆: %s\n", qrUrl)
		pkg.Input("您是否扫描完成?(y/n):", func(s string) bool {
			if strings.ToLower(s) == "y" {
				return true
			}
			return false
		})
	}

	users, err := w.GetAllUser()
	if err != nil {
		pkg.Fatal("获取所有的user失败: %s", err.Error())
	}
	table, _ := gotable.Create("uid", "微信昵称")
	for _, v := range users {
		_ = table.AddRow(map[string]string{
			"uid":  v.UId,
			"微信昵称": v.NickName,
		})
	}
	fmt.Println("您已经注册的微信通知用户, 如果您还需要增加用户, 请再次运行 ./miner-proxy -add_wx_user -wx tokne, 增加用户, 已经运行的程序将会在5分钟内更新订阅的用户:")
	fmt.Println(table.String())
	if !*isClient && (*install || *secretKey != "" || *localAddr != "") {
		// 不是客户端并且不是只想要增加新的用户, 就直接将wxpusher obj 注册回调
		if err := status.AddConnectErrorCallback(w); err != nil {
			pkg.Fatal("注册失败通知callback失败: %s", err.Error())
		}
	}
	return nil
}

func (p *proxyService) Start(_ service.Service) error {
	go p.run()
	return nil
}

func (p *proxyService) randomRequestHttp() {
	defer func() {
		sleepTime, _ := randutil.IntRange(10, 60)
		time.AfterFunc(time.Duration(sleepTime)*time.Second, p.randomRequestHttp)
	}()

	client := http.Client{
		Timeout: time.Second * 10,
	}
	index, _ := randutil.IntRange(0, len(reqeustUrls))
	pkg.Debug("http请求: ", reqeustUrls[index])
	resp, _ := client.Get(reqeustUrls[index])
	if resp == nil {
		return
	}
	_ = resp.Body.Close()
}

func (p *proxyService) run() {
	defer func() {
		if err := recover(); err != nil {
			pkg.Error("程序崩溃: %v, 重启中", err)
			p.run()
		}
	}()
	fmt.Printf("监听端口 '%s', 默认矿池地址 '%s'\n", *localAddr, *remoteAddr)
	if *debug {
		pkg.Warn("你开启了-debug 参数, 该参数建议只有在测试时开启")
	}

	laddr, err := net.ResolveTCPAddr("tcp", *localAddr)
	if err != nil {
		pkg.Warn("Failed to resolve local address: %s", err)
		pkg.Error("你输入的监听地址错误, 请输入: 0.0.0.0:端口/:端口/ip:端口")
		os.Exit(1)
	}
	raddr, err := net.ResolveTCPAddr("tcp", *remoteAddr)
	if err != nil {
		pkg.Error("你输入的矿池地址错误, 请输入: ip:端口")
		os.Exit(1)
	}
	listener, err := net.ListenTCP("tcp", laddr)
	if err != nil {
		pkg.Error("监听 '%s' 失败, 请更换一个端口", laddr.String())
		os.Exit(1)
	}

	if len(*secretKey) > 32 {
		pkg.Error("密钥必须小于等于32位!")
		os.Exit(1)
	}

	for len(*secretKey)%16 != 0 {
		*secretKey += "0"
	}

	if !*isClient {
		go func() {
			for range time.Tick(time.Second * 30) {
				status.Show(time.Duration(*offlineTime) * time.Second)
			}
		}()
	}
	if *isClient {
		go status.ShowDelay(time.Second * 30)
	}

	if *isClient && *randomSendHttp {
		go p.randomRequestHttp()
	}

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			continue
		}
		p := proxy.New(conn, laddr, raddr)
		p.SecretKey = *secretKey
		p.IsClient = *isClient
		p.SendRemoteAddr = *SendRemoteAddr
		p.UseSendConfusionData = *UseSendConfusionData
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
	pkg.PrintHelp()
	fmt.Printf("版本日志: %s\n", gitCommit)
	if *debug {
		pkg.InitLog(zapcore.DebugLevel, *logFile)
	}

	if !*debug {
		pkg.InitLog(zapcore.InfoLevel, *logFile)
	}
	if *newWxPusherUser || *wxPusherToken != "" {
		if err := new(proxyService).checkWxPusher(); err != nil {
			pkg.Fatal(err.Error())
		}
	}
	if *newWxPusherUser {
		return
	}
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
		if _, err := s.Status(); err == nil {
			pkg.Input("已经存在一个miner-proxy服务了, 是否需要卸载?(y/n):", func(in string) bool {
				in = strings.ToLower(in)
				if in == "y" {
					if err := s.Uninstall(); err != nil {
						log.Println("删除失败", err)
					}
				}
				return true
			})
			log.Println("如果想要启动第二个程序, linux可以使用nohup ./miner-proxy 你的参数 >> miner-proxy.log 2>&1&启动")
			return
		}
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
		//pkg.Input("是否需要移除代理服务?(y/n)", func(in string) bool {
		//	if in == "y" {
		//		if err := s.Uninstall(); err != nil {
		//			log.Println("删除失败", err)
		//		}
		//	}
		//	return true
		//})
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
