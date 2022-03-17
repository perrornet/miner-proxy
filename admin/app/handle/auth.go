package handle

import (
	"fmt"
	"miner-proxy/admin/app/middleware"
	"miner-proxy/admin/app/models"
	"miner-proxy/admin/app/models/common"
	"miner-proxy/admin/app/response"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func Login(c *gin.Context) {
	type login struct {
		User     string `json:"user"`
		Password string `json:"password"`
	}
	l := new(login)
	if err := c.Bind(l); err != nil {
		response.Error(c, http.StatusBadRequest, err, "")
		return
	}
	var user = new(models.User)
	if err := common.First(&models.User{Name: l.User}, user); err != nil {
		response.Error(c, http.StatusNotFound, err, "")
		return
	}
	fmt.Println(user.Pass, models.Password(l.Password).Encryption())
	if user.Pass != models.Password(l.Password).Encryption() {
		response.Error(c, http.StatusForbidden, nil, "password error")
		return
	}

	value := uuid.NewSHA1(uuid.New(), []byte("miner-proxy")).String()
	middleware.UserLoginCache.Set(value, user, time.Hour*24)
	c.SetCookie("miner-proxy-cookie",
		value, 1*24*60*60, "/", "", true, true)
	response.OK(c, user, "ok")
	return
}

func Logout(c *gin.Context) {
	value, _ := c.Cookie("miner-proxy-cookie")
	middleware.UserLoginCache.Delete(value)
	response.OK(c, nil, "ok")
	return
}
