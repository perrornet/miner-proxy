package proxy

import (
	"bytes"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"miner-proxy/pkg"
	"strconv"
	"strings"
)

var (
	LocalAddr            = flag.String("l", "0.0.0.0:9999", "本地监听地址")
	RemoteAddr           = flag.String("r", "localhost:80", "远程代理地址或者远程本程序的监听地址")
	SecretKey            = flag.String("secret_key", "", "数据包加密密钥, 只有远程地址也是本服务时才可使用")
	IsClient             = flag.Bool("client", false, "是否是客户端, 该参数必须准确, 默认服务端, 只有 secret_key 不为空时需要区分")
	UseSendConfusionData = flag.Bool("sc", false, "是否使用混淆数据, 如果指定了, 将会不定时在server/client之间发送随机的混淆数据以及在挖矿数据中插入随机数据")
	Debug                = flag.Bool("debug", false, "是否开启debug")
	SendRemoteAddr       = flag.String("sr", "", "远程代理地址或者远程本程序的监听地址")
)

type Proxy interface {
	// ParseConfigByCmd 从命令行中解析配置到程序中, 这个是服务真正运行时调用
	ParseConfigByCmd() error
	// InputConfig 让用户输入配置信息, 当用户使用-install参数时调用
	InputConfig() error
	// OutputConfig 将运行时的参数返回, 当用户使用-install参数并且在 调用完 InputConfig 之后调用
	OutputConfig() []string
	// Run 运行服务, 在运行 ParseConfigByCmd 之后调用
	Run() error
}

type Args struct {
	LocalAddr            string `json:"local_addr" notes:"本地监听地址"`
	SecretKey            string `json:"secret_key" notes:"数据包加密密钥, 只有远程地址也是本服务时才可使用"`
	IsClient             bool   `json:"is_client" notes:"是否是客户端, 该参数必须准确, 默认服务端, 只有 secret_key 不为空时需要区分"`
	UseSendConfusionData bool   `json:"use_send_confusion_data" notes:"是否使用混淆数据, 如果指定了, 将会不定时在server/client之间发送随机的混淆数据以及在挖矿数据中插入随机数据"`
	Debug                bool   `json:"debug" notes:"是否开启debug"`
}

func (r *Args) InputConfig() error {
	pkg.Input("请输入本地监听地址:", func(s string) bool {
		r.LocalAddr = s
		return true
	})

	pkg.Input("您是否当前是在运行客户端吗?(y/n):", func(s string) bool {
		s = strings.ToLower(s)
		if s == "y" {
			r.IsClient = true
			return true
		}
		return true
	})
	pkg.Input("请输入您的密钥(服务/客户端必须一致):", func(s string) bool {
		r.SecretKey = s
		return true
	})

	pkg.Input("您是否需要在挖矿数据中插入随机数据?(y/n):", func(s string) bool {
		s = strings.ToLower(s)
		if s == "y" {
			r.UseSendConfusionData = true
			return true
		}
		return true
	})

	pkg.Input("您是否想要开启详细的日志记录?(y/n):", func(s string) bool {
		s = strings.ToLower(s)
		if s == "y" {
			r.Debug = true
			return true
		}
		return true
	})
	return nil
}

type BufferedConn struct {
	conn io.Reader
	pos  int
	buf  []byte
	size int
}

func (bc *BufferedConn) Read() error {
	n, err := bc.conn.Read(bc.buf[bc.pos:])
	bc.pos = bc.pos + n
	if err != nil {
		return err
	}
	return nil
}

func (bc *BufferedConn) Buffered() int {
	return bc.pos
}

func (bc *BufferedConn) Size() int {
	return bc.size
}

func (bc *BufferedConn) Peek(start, stop int) []byte {
	if start > bc.pos || stop < 0 {
		return nil
	}

	return bc.buf[start:stop]
}

func (bc *BufferedConn) Grow(extendLen int) {
	extendBuf := make([]byte, extendLen)
	bc.buf = append(bc.buf, extendBuf[:]...)
	bc.size = bc.size + extendLen
	log.Printf("the buf`s max size has been set: %d \n", bc.size)
}

func (bc *BufferedConn) Clear(position int) error {
	if position > bc.size || position < 0 {
		return errors.New("position wrong")
	}

	newPos := bc.pos - position
	if position < bc.pos {
		src := bc.buf[position:bc.pos]
		for i := range src {
			bc.buf[i] = src[i]
		}
		needZero := bc.buf[newPos:bc.pos]
		for i := range needZero {
			needZero[i] = 0
		}
	} else {
		needZero := bc.buf[:position]
		for i := range needZero {
			needZero[i] = 0
		}
	}
	bc.pos = newPos

	return nil
}

var PackageStart = 'P'

type Package struct {
	Length int
	Data   []byte
}

func NewPackage(data []byte) *Package {
	return &Package{
		Data:   data,
		Length: len(data),
	}
}

func (p *Package) Pack(writer io.Writer) error {
	if _, err := writer.Write([]byte{uint8(PackageStart)}); err != nil {
		return err
	}

	length := strconv.Itoa(p.Length)
	var lengthBuf = bytes.NewBufferString(length)
	for lengthBuf.Len() < 10 {
		lengthBuf.WriteString("-")
	}
	if _, err := writer.Write(lengthBuf.Bytes()); err != nil {
		return err
	}

	if _, err := writer.Write(p.Data); err != nil {
		return err
	}
	return nil
}

func (p *Package) Unpack(reader io.Reader) error {
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}
	p.Data = data
	p.Length = len(data)
	return nil
}

func (p *Package) String() string {
	return fmt.Sprintf("length:%d data:%s",
		p.Length,
		hex.Dump(p.Data)[:55],
	)
}

func (p *Package) Read(reader io.Reader, f func(Package)) error {
	var data = make([]byte, 1)
	var buf []byte
	var length int
	for {
		n, err := reader.Read(data)
		if err != nil {
			return err
		}
		if data[0] == uint8(PackageStart) {
			// 再读取 11个字节
			data = make([]byte, 10)
			continue
		}
		if len(data[:n]) == 10 && bytes.Contains(data[:n], []byte("-")) {
			dataLength, err := strconv.Atoi(strings.ReplaceAll(string(data[:n]), "-", ""))
			if err != nil {
				return err
			}
			data = make([]byte, dataLength)
			length = dataLength
			continue
		}
		buf = append(buf, data[:n]...)
		if len(buf) < length-1 {
			data = make([]byte, length-len(buf))
			continue
		}

		f(Package{
			Length: n,
			Data:   buf,
		})
		return nil
	}
}
