package test

import (
	"bytes"
	"fmt"
	"miner-proxy/pkg"
	"miner-proxy/proxy"
	"net"
	"testing"
)

func runMinerProxy(secretKey, remoteAddr string, isClient bool, port *int) error {
	logger := pkg.ColorLogger{}

	laddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	raddr, err := net.ResolveTCPAddr("tcp", remoteAddr)
	if err != nil {
		return err
	}
	listener, err := net.ListenTCP("tcp", laddr)
	if err != nil {
		return err
	}
	*port = listener.Addr().(*net.TCPAddr).Port
	go func() {
		for {
			conn, err := listener.AcceptTCP()
			if err != nil {
				logger.Warn("Failed to accept connection '%s'", err)
				continue
			}
			p := proxy.New(conn, laddr, raddr)
			p.SecretKey = secretKey
			p.IsClient = isClient
			p.Log = pkg.ColorLogger{
				Prefix:  fmt.Sprintf("client: %v: ", isClient),
				Verbose: true,
			}
			go p.Start()
		}
	}()
	return nil
}

func TestMinerProxy(t *testing.T) {
	// 创建一个server
	server, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal("listen server error", err)
	}

	// start miner-proxy server
	var (
		secretKey            = "1234567891234567"
		remoteAddr           = fmt.Sprintf("127.0.0.1:%d", server.Addr().(*net.TCPAddr).Port)
		minerProxyServerPort int
		minerProxyClientPort int
	)
	if err := runMinerProxy(secretKey, remoteAddr, false, &minerProxyServerPort); err != nil {
		t.Fatal(err)
	}

	// start miner-proxy client
	if err := runMinerProxy(secretKey, fmt.Sprintf("127.0.0.1:%d", minerProxyServerPort), true, &minerProxyClientPort); err != nil {
		t.Fatal(err)
	}

	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", minerProxyClientPort))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte("1")); err != nil {
		t.Fatal(err)
	}
	serverConn, err := server.Accept()
	if err != nil {
		t.Fatal(err)
	}
	var data = make([]byte, 1024)
	n, err := serverConn.Read(data)
	if err != nil {
		t.Fatal(err)
	}
	data = data[:n]
	if !bytes.Equal(data, []byte("1")) {
		t.Fatalf("数据不相等!, got %s, want to 1", string(data))
	}
	//t.Log(string(data))
}

func TestMinerProxyHasEncryption(t *testing.T) {
	server, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal("listen server error", err)
	}

	// start miner-proxy server
	var (
		secretKey            = "1234567891234567"
		remoteAddr           = fmt.Sprintf("127.0.0.1:%d", server.Addr().(*net.TCPAddr).Port)
		minerProxyClientPort int
	)

	// start miner-proxy client
	if err := runMinerProxy(secretKey, remoteAddr, true, &minerProxyClientPort); err != nil {
		t.Fatal(err)
	}

	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", minerProxyClientPort))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte("1")); err != nil {
		t.Fatal(err)
	}
	serverConn, err := server.Accept()
	if err != nil {
		t.Fatal(err)
	}
	var data = make([]byte, 1024)
	n, err := serverConn.Read(data)
	if err != nil {
		t.Fatal(err)
	}
	data = data[:n]
	if bytes.Equal(data, []byte("1")) {
		t.Fatalf("数据不相等!, got %s, want to 1", string(data))
	}
	if !bytes.HasPrefix(data, []byte("start-proxy-encryption")) {
		t.Fatalf("数据未加密, 因为数据没有 start-proxy-encryption 前缀")
	}
	if !bytes.HasSuffix(data, []byte("start-proxy-end")) {
		t.Fatalf("数据未加密, 因为数据没有 tart-proxy-end 后缀")
	}
	data = bytes.TrimLeft(data, "start-proxy-encryption")
	data = bytes.TrimRight(data, "start-proxy-end")
	// 解密数据
	data, err = pkg.AesDecrypt(data, []byte(secretKey))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, []byte("1")) {
		t.Fatalf("解密后数据不相等! got %s, want to 1", string(data))
	}
}
