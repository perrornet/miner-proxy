package router

import (
	"miner-proxy/admin/app/middleware"

	"github.com/gin-gonic/gin"
)

func InitRouter(app *gin.Engine) {
	g := app.Group("/api")
	NewAuth(g)

	g.Use(middleware.Cors(), middleware.Auth())
	NewClientRouter(g)
	NewSysRouter(g)
}
