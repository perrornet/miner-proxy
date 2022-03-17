package router

import (
	"miner-proxy/admin/app/handle"

	"github.com/gin-gonic/gin"
)

func NewAuth(app *gin.RouterGroup) {
	app.POST("/login", handle.Login)
	app.DELETE("/logout", handle.Logout)
}
