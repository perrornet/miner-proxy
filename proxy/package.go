package proxy

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"sync"
)

type BufferedConn struct {
	conn io.Reader
	pos  int
	buf  []byte
	size int
}

func NewBufferedConn(conn io.Reader) *BufferedConn {
	size := 2048 // default size of buf
	buf := make([]byte, size)
	return &BufferedConn{
		conn: conn,
		pos:  0,
		buf:  buf,
		size: size,
	}
}

func NewBufferedConnWithSize(conn io.Reader, size int) *BufferedConn {
	buf := make([]byte, size)
	return &BufferedConn{
		conn: conn,
		pos:  0,
		buf:  buf,
		size: size,
	}
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

var PackagePool = sync.Pool{New: func() interface{} {
	return new(Package)
}}

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
	scanner := bufio.NewScanner(reader)
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		data = bytes.TrimLeftFunc(data, func(r rune) bool {
			return r == 0
		})
		if !atEOF && data[0] == uint8(PackageStart) {
			if len(data) > 11 {
				length, err := strconv.Atoi(strings.ReplaceAll(string(data[1:11]), "-", ""))
				if err != nil {
					return 0, nil, err
				}
				if int(length+11) <= len(data) {
					return length + 11, data[11 : int(length)+11], nil
				}
			}
		}
		return
	})
	for scanner.Scan() {
		data := scanner.Bytes()
		if len(data) == 0 {
			return io.EOF
		}
		scannedPack := new(Package)
		if err := scannedPack.Unpack(bytes.NewReader(scanner.Bytes())); err != nil {
			return fmt.Errorf("unpack error: %s", err)
		}
		f(*scannedPack)
	}
	return scanner.Err()
}
