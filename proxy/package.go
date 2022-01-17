package proxy

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"miner-proxy/pkg/status"
	"strconv"
	"strings"
	"time"
)

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
	startTime := time.Now()
	if _, err := writer.Write([]byte{uint8(PackageStart)}); err != nil {
		return err
	}
	status.UpdateTimeParse(time.Since(startTime).Nanoseconds())
	length := strconv.Itoa(p.Length)
	var lengthBuf = bytes.NewBufferString(length)
	for lengthBuf.Len() < 10 {
		lengthBuf.WriteString("-")
	}
	startTime = time.Now()
	if _, err := writer.Write(lengthBuf.Bytes()); err != nil {
		return err
	}
	status.UpdateTimeParse(time.Since(startTime).Nanoseconds())

	startTime = time.Now()
	if _, err := writer.Write(p.Data); err != nil {
		return err
	}
	status.UpdateTimeParse(time.Since(startTime).Nanoseconds())
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
		if data[0] == uint8(PackageStart) && len(data) == 1 {
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
