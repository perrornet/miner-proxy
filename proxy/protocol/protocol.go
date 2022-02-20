package protocol

import (
	"encoding/binary"
	"fmt"
	"miner-proxy/pkg"
	"net"

	"github.com/panjf2000/gnet"
	"github.com/smallnest/goframe"
	"github.com/vmihailenco/msgpack/v5"
	"go.uber.org/atomic"
)

type RequestType int

const (
	LOGIN RequestType = iota
	INIT
	DATA
	PING
	PONG
	ACK
	ERROR
	// CLOSE 关闭矿机的连接
	CLOSE
)

var (
	confusionCount = atomic.NewInt64(1)
)

func (r RequestType) String() string {
	switch r {
	case LOGIN:
		return "login"
	case DATA:
		return "data"
	case CLOSE:
		return "close"
	case ERROR:
		return "error"
	case PING:
		return "ping"
	case PONG:
		return "pong"
	case INIT:
		return "init"
	case ACK:
		return "ack"
	}
	return ""
}

type EncryptionProtocol struct {
	secretKey            string
	useSendConfusionData bool
}

// separateConfusionData 分离混淆的数据
func (p *EncryptionProtocol) separateConfusionData(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	if !p.useSendConfusionData {
		return data
	}
	var result = make([]byte, 0, len(data)/2)
	for index, v := range data {
		if index%2 == 0 {
			continue
		}
		result = append(result, v)
	}
	return result
}

// buildConfusionData 构建混淆数据
// 从 10 - 135中随机一个数字作为本次随机数据的长度 N
// 循环 N 次, 每次从 1 - 255 中随机一个数字作为本次随机数据
func (p *EncryptionProtocol) buildConfusionData() []byte {
	count := int(confusionCount.Inc())
	number := count % 356
	if number < 10 {
		number = 10
	}
	var data = make([]byte, number)
	for i := 0; i < number; i++ {
		count = int(confusionCount.Inc())
		data[i] = uint8((count % 254) + 1)
	}
	return data
}

// EncryptionData 构建需要发送的加密数据
// 先使用 SecretKey aes 加密 data 如果 UseSendConfusionData 等于 true
// 那么将会每25个字符插入 buildConfusionData 生成的随机字符
func (p *EncryptionProtocol) EncryptionData(data []byte) ([]byte, error) {
	if p.useSendConfusionData { // 插入随机混淆数据
		confusionData := p.buildConfusionData()
		var result []byte
		for _, v := range data {
			result = append(result, confusionData[0])
			confusionData = append(confusionData[1:], confusionData[0])
			result = append(result, v)
		}
		data = result
	}
	if p.secretKey != "" {
		return pkg.AesEncrypt(data, []byte(p.secretKey))
	}
	return data, nil
}

func (cc *EncryptionProtocol) DecryptData(data []byte) (result []byte, err error) {
	if cc.secretKey != "" {
		data, err = pkg.AesDecrypt(data, []byte(cc.secretKey))
		if err != nil {
			return nil, err
		}
	}

	if cc.useSendConfusionData { // 去除随机混淆数据
		data = cc.separateConfusionData(data)
	}
	return data, nil
}

type Request struct {
	ClientId string      `msgpack:"client_id"`
	MinerId  string      `msgpack:"miner_id"`
	Hash     string      `msgpack:"hash"`
	Type     RequestType `msgpack:"type"`
	Data     []byte      `msgpack:"data"`
}

func CopyRequest(req Request) Request {
	return Request{
		ClientId: req.ClientId,
		MinerId:  req.MinerId,
		Type:     req.Type,
	}
}

func (r *Request) SetClientId(clientId string) *Request {
	r.ClientId = clientId
	return r
}

func (r *Request) SetMinerId(MinerId string) *Request {
	r.MinerId = MinerId
	return r
}

func (r *Request) SetData(data []byte) *Request {
	r.Data = data
	return r
}

func (r *Request) SetType(Type RequestType) *Request {
	r.Type = Type
	return r
}

func (r *Request) End() Request {
	return *r
}

func (r Request) String() string {
	return fmt.Sprintf("clientId=%s,hash=%s,miner_id=%s,type=%s,data_size=%d", r.ClientId, r.Hash, r.MinerId, r.Type, len(r.Data))
}

type LoginRequest struct {
	PoolAddress string `msgpack:"pool_address"`
	MinerIp     string `msgpack:"miner_ip"`
}

func Encode2Request(data []byte) (Request, error) {
	var result = new(Request)
	err := msgpack.Unmarshal(data, result)
	if err != nil {
		return *result, err
	}
	want := pkg.Crc32IEEEString(result.Data)
	if result.Hash != want && result.Hash != "" {
		return Request{}, fmt.Errorf("data hash must equal, got=%s want=%s", result.Hash, want)
	}
	return *result, err
}

func Decode2Byte(req Request) ([]byte, error) {
	req.Hash = pkg.Crc32IEEEString(req.Data)
	return msgpack.Marshal(req)
}

func Encode2LoginRequest(data []byte) (LoginRequest, error) {
	var result = new(LoginRequest)
	err := msgpack.Unmarshal(data, result)
	return *result, err
}

func DecodeLoginRequest2Byte(req LoginRequest) []byte {
	data, _ := msgpack.Marshal(req)
	return data
}

type GoframeProtocol struct {
	frame goframe.FrameConn
	*EncryptionProtocol
}

func NewGoframeProtocol(secretKey string, useSendConfusionData bool, c net.Conn) goframe.FrameConn {
	encoderConfig := goframe.EncoderConfig{
		ByteOrder:                       binary.BigEndian,
		LengthFieldLength:               4,
		LengthAdjustment:                0,
		LengthIncludesLengthFieldLength: false,
	}
	decoderConfig := goframe.DecoderConfig{
		ByteOrder:           binary.BigEndian,
		LengthFieldOffset:   0,
		LengthFieldLength:   4,
		LengthAdjustment:    0,
		InitialBytesToStrip: 4,
	}
	return &GoframeProtocol{
		frame: goframe.NewLengthFieldBasedFrameConn(encoderConfig, decoderConfig, c),
		EncryptionProtocol: &EncryptionProtocol{
			secretKey:            secretKey,
			useSendConfusionData: useSendConfusionData,
		},
	}
}

func (g *GoframeProtocol) ReadFrame() ([]byte, error) {
	data, err := g.frame.ReadFrame()
	if err != nil {
		return nil, err
	}
	return g.DecryptData(data)
}

func (g *GoframeProtocol) WriteFrame(p []byte) error {
	p, err := g.EncryptionData(p)
	if err != nil {
		return err
	}
	return g.frame.WriteFrame(p)
}

func (g *GoframeProtocol) Close() error {
	return g.frame.Close()
}

func (g *GoframeProtocol) Conn() net.Conn {
	return g.frame.Conn()
}

type Protocol struct {
	*gnet.LengthFieldBasedFrameCodec
	*EncryptionProtocol
}

func NewProtocol(secretKey string, useSendConfusionData bool) *Protocol {
	encoderConfig := gnet.EncoderConfig{
		ByteOrder:                       binary.BigEndian,
		LengthFieldLength:               4,
		LengthAdjustment:                0,
		LengthIncludesLengthFieldLength: false,
	}
	decoderConfig := gnet.DecoderConfig{
		ByteOrder:           binary.BigEndian,
		LengthFieldOffset:   0,
		LengthFieldLength:   4,
		LengthAdjustment:    0,
		InitialBytesToStrip: 4,
	}
	return &Protocol{
		LengthFieldBasedFrameCodec: gnet.NewLengthFieldBasedFrameCodec(encoderConfig, decoderConfig),
		EncryptionProtocol: &EncryptionProtocol{
			secretKey:            secretKey,
			useSendConfusionData: useSendConfusionData,
		},
	}
}

// Encode ...
func (cc *Protocol) Encode(c gnet.Conn, buf []byte) ([]byte, error) {
	buf, err := cc.EncryptionData(buf)
	if err != nil {
		return nil, err
	}
	return cc.LengthFieldBasedFrameCodec.Encode(c, buf)
}

// Decode ...
func (cc *Protocol) Decode(c gnet.Conn) ([]byte, error) {
	data, err := cc.LengthFieldBasedFrameCodec.Decode(c)
	if err != nil {
		return nil, err
	}
	return cc.DecryptData(data)
}
