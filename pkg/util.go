package pkg

import (
	"crypto/md5"
	"errors"
	"fmt"
	"time"
)

var (
	CLIENTID string
)

func Try(f func() bool, maxTry int) error {
	var n int
	for n < maxTry {
		if f() {
			return nil
		}
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

func Md5(text string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(text)))
}
