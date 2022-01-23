package pkg

import (
	"net"
	"strings"
)

func LocalIPv4s() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	res := conn.LocalAddr().String()
	res = strings.Split(res, ":")[0]
	return res
}
