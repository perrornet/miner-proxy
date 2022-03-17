package middleware

import (
	"miner-proxy/admin/app/response"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
)

var (
	UserLoginCache = cache.New(time.Hour*24, time.Second*60)
)

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		value, _ := c.Cookie("miner-proxy-cookie")
		if value == "" {
			response.Error(c, http.StatusForbidden, errors.New("no cookie"), "need login")
			return
		}
		v, ok := UserLoginCache.Get(value)
		if !ok {
			response.Error(c, http.StatusForbidden, errors.New("no cookie"), "need login")
			return
		}
		c.Set("user", v)
	}
}
