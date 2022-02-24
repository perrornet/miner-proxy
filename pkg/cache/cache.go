package cache

import (
	"time"

	"github.com/patrickmn/go-cache"
)

var Client *cache.Cache

func init() {
	Client = cache.New(time.Second*60, time.Second*20)
}
