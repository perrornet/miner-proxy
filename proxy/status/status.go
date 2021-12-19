package status

import (
	"github.com/spf13/cast"
	"sync"
)

var (
	online   sync.Map
	flowSize sync.Map
)

func AddOnlineCount(name string, count ...int) {
	v, _ := online.Load(name)
	var i = 1
	if len(count) != 0 && count[0] != 0 {
		i = count[0]
	}
	online.Store(name, cast.ToInt(v)+i)
}

func AddFlowSize(name string, size int64) {
	v, _ := flowSize.Load(name)
	flowSize.Store(name, cast.ToInt64(v)+size)
}
