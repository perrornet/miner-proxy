package handle

import (
	"miner-proxy/admin/app/models"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetPage(c *gin.Context) (pageIndex, pageSize int) {
	pageIndex, _ = strconv.Atoi(c.Query("page"))
	pageSize, _ = strconv.Atoi(c.Query("size"))
	if pageIndex < 0 {
		pageIndex = 0
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	return
}

func GetUser(c *gin.Context) *models.User {
	v, ok := c.Get("user")
	if !ok {
		return nil
	}
	return v.(*models.User)
}
