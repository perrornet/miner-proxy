package pkg

import (
	"errors"
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cast"
)

func Try(f func() bool, maxTry int) error {
	var n int
	for n < maxTry {
		if f() {
			return nil
		}
		n++
	}
	return errors.New("try run function error")
}

func GetHashRateBySize(size int64, parseTime time.Duration) float64 {
	return float64(size) / parseTime.Seconds() / 1.34
}

func GetHumanizeHashRateBySize(hashRate float64) string {
	var result string
	switch {
	case hashRate < 1000:
		result = fmt.Sprintf("%.2f MH/S", hashRate)
	case hashRate < 1000000:
		result = fmt.Sprintf("%.2f G/S", hashRate/1000)
	default:
		result = fmt.Sprintf("%.2f T/S", hashRate/1000000)
	}
	return result
}

func String2Array(text string, seq string) []string {
	var result []string
	for _, v := range strings.Split(text, seq) {
		if v == "" {
			continue
		}
		result = append(result, v)
	}
	return result
}

func Interface2Strings(arr []interface{}) []string {
	var result []string
	for _, v := range arr {
		result = append(result, cast.ToString(v))
	}
	return result
}

func Crc32IEEE(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}

func Crc32IEEEString(data []byte) string {
	return strconv.Itoa(int(Crc32IEEE(data)))
}

func Crc32IEEEStr(data string) string {
	return Crc32IEEEString([]byte(data))
}
