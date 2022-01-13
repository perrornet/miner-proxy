package status

import (
	"fmt"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/liushuochen/gotable"
)

var (
	status    sync.Map
	startTime = time.Now()
)

type Status struct {
	Ip            string    `json:"ip"`
	Size          int64     `json:"size"`
	ConnTime      time.Time `json:"conn_time"`
	RemoteAddress string    `json:"remote_address"`
}

func Show() {
	table, _ := gotable.Create("Ip", "传输数据大小", "连接时长", "矿池")
	var result []map[string]string
	var total int64
	now := time.Now()
	status.Range(func(key, value interface{}) bool {
		s := value.(*Status)
		total += s.Size
		result = append(result, map[string]string{
			"Ip":     s.Ip,
			"传输数据大小": humanize.Bytes(uint64(s.Size)),
			"连接时长":   now.Sub(s.ConnTime).String(),
			"矿池":     s.RemoteAddress,
		})
		return true
	})
	result = append(result, map[string]string{
		"Ip":     "总计",
		"传输数据大小": humanize.Bytes(uint64(total)),
		"连接时长":   now.Sub(startTime).String(),
		"矿池":     "",
	})
	table.AddRows(result)
	fmt.Println(table.String())
}

func Add(ip string, size int64, remoteAddress string) {
	v, _ := status.LoadOrStore(ip, &Status{Ip: ip, ConnTime: time.Now(), RemoteAddress: remoteAddress})
	obj := v.(*Status)
	obj.RemoteAddress = remoteAddress
	obj.Size += size
	status.Store(ip, obj)
}

func Del(ip string) {
	status.Delete(ip)
}
