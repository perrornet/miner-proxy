package server

import (
	"fmt"
	"miner-proxy/pkg"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/emirpasic/gods/sets/hashset"
	"github.com/liushuochen/gotable"
	"github.com/panjf2000/gnet"
	"github.com/spf13/cast"
	"github.com/wxpusher/wxpusher-sdk-go/model"
	"go.uber.org/atomic"
)

var (
	startTime       = time.Now()
	pushers         sync.Map
	m               sync.Mutex
	TOTALSIZE       = atomic.NewInt64(0)
	clientDelayZero sync.Map
)

type ClientSize struct {
	lastTime time.Time
	size     int64
}

func init() {
	go func() {
		for {
			pushers.Range(func(key, value interface{}) bool {
				p, ok := value.(*pusher)
				if !ok {
					return true
				}
				if err := p.UpdateUsers(); err != nil {
					pkg.Error("更新订阅用户失败: %s", err)
					return true
				}
				return true
			})
			time.Sleep(time.Second*5*60 + 30) // 每5分30秒执行一次
		}
	}()
}

func Show(offlineTime time.Duration) {
	table, _ := gotable.Create("客户端id", "矿工id", "Ip", "传输数据大小", "连接时长", "是否在线", "客户端-服务端延迟", "矿池连接", "预估算力(仅通过流量大小判断)")
	var result []map[string]string
	var (
		totalHashRate float64
		onlineCount   int64
	)
	var offlineClient = hashset.New()
	clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)
		v, ok := clientDelay.Load(client.clientId)
		var delay = "-"
		if ok && !v.(*Delay).endTime.IsZero() && !v.(*Delay).startTime.IsZero() {
			startTime := v.(*Delay).startTime
			endTime := v.(*Delay).endTime
			if endTime.Sub(startTime).Seconds() > 0 {
				delay = endTime.Sub(startTime).String()
			}

		}
		if !client.stopTime.IsZero() && time.Since(client.stopTime).Seconds() >= offlineTime.Seconds() {
			if client.dataSize.Load() >= 1024 {
				offlineClient.Add(fmt.Sprintf("ip: %s; id: %s", client.ip, client.id))
			}
			// 删除数据
			clients.Delete(key)
		} else {
			onlineCount++
			if delay == "-" {
				var delayZeroCount = 1
				v, _ := clientDelayZero.Load(client.clientId)
				delayZeroCount += cast.ToInt(v)
				clientDelayZero.Store(client.clientId, delayZeroCount)
				if delayZeroCount >= 5 {
					client.Close()
				}
			}
		}

		hashRate := pkg.GetHashRateBySize(client.dataSize.Load(), time.Since(client.startTime))
		result = append(result, map[string]string{
			"客户端id":     client.clientId,
			"矿工id":      client.id,
			"Ip":        client.ip,
			"传输数据大小":    humanize.Bytes(uint64(client.dataSize.Load())),
			"连接时长":      time.Since(client.startTime).String(),
			"矿池连接":      client.pool.Address(),
			"是否在线":      cast.ToString(client.stopTime.IsZero()),
			"客户端-服务端延迟": delay,
			"预估算力(仅通过流量大小判断)": pkg.GetHumanizeHashRateBySize(hashRate),
		})
		totalHashRate += hashRate
		return true
	})

	result = append(result, map[string]string{
		"客户端id":     "总计",
		"Ip":        "-",
		"传输数据大小":    humanize.Bytes(uint64(TOTALSIZE.Load())),
		"连接时长":      time.Since(startTime).String(),
		"矿池连接":      " - ",
		"是否在线":      cast.ToString(onlineCount),
		"客户端-服务端延迟": " - ",
		"预估算力(仅通过流量大小判断)": pkg.GetHumanizeHashRateBySize(totalHashRate),
	})

	table.AddRows(result)
	fmt.Println(table.String())
	if offlineClient.Size() != 0 { // 发送掉线通知
		var offlineClients []string
		for _, v := range offlineClient.Values() {
			offlineClients = append(offlineClients, cast.ToString(v))
		}
		SendOfflineIps(offlineClients)
	}
}

type Pusher interface {
	SendMessage(text string, uid ...string) error
	GetAllUser() ([]model.WxUser, error)
	GetToken() string
}

type pusher struct {
	Pusher         Pusher
	Users          []model.WxUser
	m              sync.Mutex
	lastUpdateUser time.Time
}

func (p *pusher) UpdateUsers() error {
	if !p.lastUpdateUser.IsZero() && time.Since(p.lastUpdateUser).Minutes() < 5 {
		return nil
	}
	users, _ := p.Pusher.GetAllUser()
	if len(users) == 0 {
		return nil
	}
	m.Lock()
	defer m.Unlock()
	p.Users = users
	p.lastUpdateUser = time.Now()
	return nil
}

func (p *pusher) SendMessage2All(msg string) error {
	var uids []string
	for _, v := range p.Users {
		uids = append(uids, v.UId)
	}
	return p.Pusher.SendMessage(msg, uids...)
}

func AddConnectErrorCallback(p Pusher) error {
	obj := &pusher{
		Pusher: p,
	}
	if err := obj.UpdateUsers(); err != nil {
		return err
	}
	pushers.Store(obj.Pusher.GetToken(), obj)
	return nil
}

func SendOfflineIps(offlineIps []string) {
	if len(offlineIps) <= 0 {
		return
	}
	var ips = strings.Join(offlineIps, "\n")
	if len(offlineIps) > 10 {
		ips = fmt.Sprintf("%s 等 %d个ip", strings.Join(offlineIps[:10], "\n"), len(offlineIps))
	}
	ips = fmt.Sprintf("您有掉线的机器:\n%s", ips)
	pushers.Range(func(key, value interface{}) bool {
		p := value.(*pusher)
		pkg.Info("发送掉线通知: %+v", p.Users)
		if err := p.SendMessage2All(ips); err != nil {
			pkg.Error("发送通知失败: %s", err)
		}
		return true
	})
}

func ClearTotalSize() {
	TOTALSIZE.Store(0)
}

type ClientStatus struct {
	Id              string `json:"id"`
	ClientId        string `json:"client_id"`
	Ip              string `json:"ip"`
	Size            string `json:"size"`
	ConnectDuration string `json:"connect_duration"`
	RemoteAddr      string `json:"remote_addr"`
	IsOnline        bool   `json:"is_online"`
	// 根据传输数据大小判断预估算力
	HashRate        string `json:"hash_rate"`
	Delay           string `json:"delay"`
	connectDuration time.Duration
}

type ClientStatusArray []ClientStatus

func (c ClientStatusArray) Len() int {
	return len(c)
}

func (c ClientStatusArray) Less(i, j int) bool {
	return c[i].connectDuration.Nanoseconds() > c[j].connectDuration.Nanoseconds()
}

// Swap swaps the elements with indexes i and j.
func (c ClientStatusArray) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]

}

func GetClientStatus() (result ClientStatusArray) {
	var (
		totalHashRate float64
		onlineCount   int64
	)
	clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)
		v, ok := clientDelay.Load(client.clientId)
		var delay = "-"
		if ok {
			delay = v.(*Delay).endTime.Sub(v.(*Delay).startTime).String()
		}
		if client.stopTime.IsZero() {
			onlineCount++
		}

		hashRate := pkg.GetHashRateBySize(client.dataSize.Load(), time.Since(client.startTime))
		result = append(result, ClientStatus{
			Id:              client.id,
			ClientId:        client.clientId,
			Ip:              client.ip,
			Size:            humanize.Bytes(uint64(client.dataSize.Load())),
			ConnectDuration: time.Since(client.startTime).String(),
			RemoteAddr:      client.pool.Address(),
			IsOnline:        client.stopTime.IsZero(),
			HashRate:        pkg.GetHumanizeHashRateBySize(hashRate),
			Delay:           delay,
		})
		totalHashRate += hashRate
		return true
	})

	sort.Sort(result)
	result = append(result, ClientStatus{
		Id:              "-",
		ClientId:        "总计",
		Ip:              "-",
		Size:            humanize.Bytes(uint64(TOTALSIZE.Load())),
		ConnectDuration: time.Since(startTime).String(),
		RemoteAddr:      "-",
		HashRate:        pkg.GetHumanizeHashRateBySize(totalHashRate),
		Delay:           "-",
	})
	return
}

type ClientRemoteAddr struct {
	ClientId   string `json:"client_id"`
	RemoteAddr string `json:"remote_address"`
}

func ClientInfo() map[string][]string {
	var clientIds = make(map[string][]string)
	clientConn.Range(func(key, value interface{}) bool {
		k := strings.Split(cast.ToString(key), "-")[0]
		clientIds[k] = append(clientIds[k], value.(gnet.Conn).RemoteAddr().String())
		return true
	})
	for k := range clientIds {
		sort.Strings(clientIds[k])
	}
	return clientIds
}
