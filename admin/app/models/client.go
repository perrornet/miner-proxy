package models

import (
	"fmt"

	"gorm.io/gorm"
)

type Client struct {
	gorm.Model
	CreatedBy uint      `json:"created_by"`
	Key       string    `json:"key"`
	Version   string    `json:"version"`
	IsOnline  bool      `json:"is_online" gorm:"-"`
	Forwards  []Forward `json:"forwards"`
	Remark    string    `json:"remark"`
}

type Forward struct {
	gorm.Model
	ClientId   uint
	ServerPort int    `json:"server_port" gorm:"uniqueIndex:port_index"`
	ClientPort int    `json:"client_port" gorm:"uniqueIndex:port_index"`
	RemoteAddr string `json:"remote_addr"`
	// 客户端类型, 有 内网穿透/转发, reverse/forward
	Type string `json:"type"`
	// CryptType 加密方式
	CryptType string `json:"crypt_type"`
	// 密钥
	Password string `json:"password"`
	Remark   string `json:"remark"`

	IsOnline bool  `json:"is_online" gorm:"-"`
	DataSize int64 `json:"data_size" gorm:"-"`
	ConnSize int64 `json:"conn_size" gorm:"-"`
}

func (c *Client) OnlineKey() string {
	return fmt.Sprintf("client:online:%d", c.ID)
}

func (f *Forward) OnlineKey() string {
	return fmt.Sprintf("client:forward:online:%d", f.ID)
}

func (f *Forward) ConnSizeKey() string {
	return fmt.Sprintf("client:forward:conn_size:%d", f.ID)
}

func (f *Forward) DataSizeKey() string {
	return fmt.Sprintf("client:forward:data_size:%d", f.ID)
}
