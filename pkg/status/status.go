package status

import (
	"fmt"
	"miner-proxy/pkg"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/liushuochen/gotable"
	"github.com/spf13/cast"
	"github.com/wxpusher/wxpusher-sdk-go/model"
)

var (
	totalSzie       int64
	status          sync.Map
	lastonlineCount int64
	startTime       = time.Now()
	pushers         sync.Map
	m               sync.Mutex
)

type Status struct {
	Ip            string        `json:"ip"`
	ClientIp      string        `json:"client_ip"`
	Status        bool          `json:"status"`
	StopTime      time.Time     `json:"stop_time"`
	Size          int64         `json:"size"`
	ConnTime      time.Time     `json:"conn_time"`
	RemoteAddress string        `json:"remote_address"`
	Delay         time.Duration `json:"delay"`
	PingTime      time.Time     `json:"start_ping_time"`
}

func (s Status) GetStatus() ClientStatus {
	now := time.Now()
	var HashRate string
	if s.Status {
		HashRateInt := float64(s.Size) / now.Sub(s.ConnTime).Seconds() * 0.85
		switch {
		case HashRateInt < 1000:
			HashRate = fmt.Sprintf("%.2f MH/S", HashRateInt)
		default:
			HashRate = fmt.Sprintf("%.2f G/S", HashRateInt)
		}
	}

	var delay string
	if s.Delay.Microseconds() > 0 {
		delay = s.Delay.String()
	}
	var ip = s.Ip
	if s.ClientIp != "" {
		ip = s.ClientIp
	}
	return ClientStatus{
		Ip:              ip,
		Size:            humanize.Bytes(uint64(s.Size)),
		ConnectDuration: now.Sub(s.ConnTime).String(),
		RemoteAddr:      s.RemoteAddress,
		IsOnline:        s.Status,
		connectDuration: now.Sub(s.ConnTime),
		HashRate:        HashRate,
		Delay:           delay,
	}
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
	table, _ := gotable.Create("Ip", "传输数据大小", "连接时长", "是否在线", "客户端-服务端延迟", "矿池连接", "预估算力(仅通过流量大小判断)")
	var result []map[string]string
	var total int64
	now := time.Now()
	var (
		offlineIps     []string
		realOfflineIps []string
		nowOnlineCount int64
	)

	status.Range(func(key, value interface{}) bool {
		s := value.(*Status)

		nowOnlineCount++
		total += s.Size
		clientStatus := s.GetStatus()
		if !s.Status && time.Since(s.StopTime).Seconds() >= offlineTime.Seconds() {
			offlineIps = append(offlineIps, clientStatus.Ip)
			realOfflineIps = append(realOfflineIps, cast.ToString(key))
			return true
		}
		result = append(result, map[string]string{
			"Ip":        clientStatus.Ip,
			"传输数据大小":    clientStatus.Size,
			"连接时长":      clientStatus.ConnectDuration,
			"矿池连接":      clientStatus.RemoteAddr,
			"是否在线":      cast.ToString(clientStatus.IsOnline),
			"客户端-服务端延迟": clientStatus.Delay,
			"预估算力(仅通过流量大小判断)": clientStatus.HashRate,
		})
		return true
	})
	result = append(result, map[string]string{
		"Ip":     "总计",
		"传输数据大小": humanize.Bytes(uint64(atomic.LoadInt64(&totalSzie))),
		"连接时长":   now.Sub(startTime).String(),
		"矿池":     "",
	})
	result = append(result, map[string]string{
		"Ip":        "总计",
		"传输数据大小":    humanize.Bytes(uint64(atomic.LoadInt64(&totalSzie))),
		"连接时长":      now.Sub(startTime).String(),
		"矿池连接":      "-",
		"是否在线":      "-",
		"客户端-服务端延迟": "-",
		"预估算力(仅通过流量大小判断)": "-",
	})
	table.AddRows(result)
	fmt.Println(table.String())
	// 删除这些过期的ip
	for _, v := range realOfflineIps {
		status.Delete(v)
	}

	if nowOnlineCount < lastonlineCount { // 掉线通知
		go SendOfflineIps(offlineIps)
		if len(offlineIps) != 0 {
			lastonlineCount = nowOnlineCount
		}
	} else {
		lastonlineCount = nowOnlineCount
	}
}

type ClientStatus struct {
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

func GetClientStatus() ClientStatusArray {
	var result ClientStatusArray
	status.Range(func(key, value interface{}) bool {
		result = append(result, value.(*Status).GetStatus())
		return true
	})
	sort.Sort(result)
	return result
}

func SendOfflineIps(offlineIps []string) {
	if len(offlineIps) <= 0 {
		return
	}
	var ips = strings.Join(offlineIps, "\n")
	if len(offlineIps) > 10 {
		ips = fmt.Sprintf("%s 等 %d个ip", strings.Join(offlineIps[:10], "\n"), len(offlineIps))
	}
	ips = fmt.Sprintf("您有掉线的矿机:\n%s", ips)
	pushers.Range(func(key, value interface{}) bool {
		p := value.(*pusher)
		pkg.Info("发送掉线通知: %+v", p.Users)
		if err := p.SendMessage2All(ips); err != nil {
			pkg.Error("发送通知失败: %s", err)
		}
		return true
	})
}

func Add(ip string, size int64, remoteAddress string, clientIp ...string) {
	v, _ := status.LoadOrStore(ip, &Status{Ip: ip, ConnTime: time.Now(), RemoteAddress: remoteAddress, Status: true})
	obj := v.(*Status)
	if len(clientIp) != 0 && clientIp[0] != "" {
		obj.ClientIp = clientIp[0]
	}
	if remoteAddress != "" {
		obj.RemoteAddress = remoteAddress
	}
	obj.Size += size
	atomic.AddInt64(&totalSzie, size)
	status.Store(ip, obj)
}

func SetPing(ip string) {
	v, ok := status.Load(ip)
	if !ok {
		return
	}
	obj := v.(*Status)
	obj.PingTime = time.Now()
	status.Store(ip, obj)
}

func SetPong(ip string) {
	v, ok := status.Load(ip)
	if !ok {
		return
	}
	obj := v.(*Status)
	obj.Delay = time.Now().Sub(obj.PingTime)
	if obj.Delay.Seconds() < 0 {
		return
	}
	obj.PingTime = time.Time{}
	status.Store(ip, obj)
}

func Del(ip string) {
	v, ok := status.Load(ip)
	if !ok {
		return
	}
	s := v.(*Status)
	s.Status = false
	s.StopTime = time.Now()
	status.Store(ip, s)
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
