package client

import (
	kcp "github.com/xtaci/kcp-go/v5"
)

func dial(RemoteAddr string, block kcp.BlockCrypt) (*kcp.UDPSession, error) {
	return kcp.DialWithOptions(RemoteAddr, block, 10, 3)
}
