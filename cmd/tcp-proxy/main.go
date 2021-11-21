package main

import (
	"flag"
	"fmt"
	"miner-proxy/pkg"
	"miner-proxy/proxy"
	"net"
	"os"
	"regexp"
	"strings"
)

var (
	version = "0.0.0-src"
	matchid = uint64(0)
	connid  = uint64(0)
	logger  pkg.Logger

	localAddr   = flag.String("l", ":9999", "本地监听地址")
	remoteAddr  = flag.String("r", "localhost:80", "远程代理地址或者远程本程序的监听地址")
	secretKey   = flag.String("secret_key", "", "数据包加密密钥, 只有远程地址也是本服务时才可使用")
	isClient =  flag.Bool("client", false, "是否是客户端, 该参数必须准确, 默认服务端, 只有 secret_key 不为空时需要区分")
)

func main() {
	flag.Parse()

	logger := pkg.ColorLogger{}

	logger.Info("miner-proxy (%s) proxing from %v to %v ", version, *localAddr, *remoteAddr)

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
	if len(*secretKey) % 16 != 0{
		for len(*secretKey) % 16 != 0 {
			*secretKey += "0"
		}
	}

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			logger.Warn("Failed to accept connection '%s'", err)
			continue
		}
		connid++
		p := proxy.New(conn, laddr, raddr)
		p.SecretKey = *secretKey
		p.IsClient = *isClient
		p.Log = pkg.ColorLogger{
			Prefix:      fmt.Sprintf("Connection #%03d ", connid),
			Verbose: true,
		}
		go p.Start()
	}
}

func createMatcher(match string) func([]byte) {
	if match == "" {
		return nil
	}
	re, err := regexp.Compile(match)
	if err != nil {
		logger.Warn("Invalid match regex: %s", err)
		return nil
	}

	logger.Info("Matching %s", re.String())
	return func(input []byte) {
		ms := re.FindAll(input, -1)
		for _, m := range ms {
			matchid++
			logger.Info("Match #%d: %s", matchid, string(m))
		}
	}
}

func createReplacer(replace string) func([]byte) []byte {
	if replace == "" {
		return nil
	}
	//split by / (TODO: allow slash escapes)
	parts := strings.Split(replace, "~")
	if len(parts) != 2 {
		logger.Warn("Invalid replace option")
		return nil
	}

	re, err := regexp.Compile(string(parts[0]))
	if err != nil {
		logger.Warn("Invalid replace regex: %s", err)
		return nil
	}

	repl := []byte(parts[1])

	logger.Info("Replacing %s with %s", re.String(), repl)
	return func(input []byte) []byte {
		return re.ReplaceAll(input, repl)
	}
}
