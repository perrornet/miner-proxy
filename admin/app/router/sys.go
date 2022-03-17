package router

import (
	"miner-proxy/admin/app/handle"

	"github.com/gin-gonic/gin"
)

func NewSysRouter(app *gin.RouterGroup) {
	app = app.Group("/sys")

	app.GET("/user/", handle.ListUser)
	app.POST("/user/", handle.CreateUser)
	app.PUT("/user/", handle.UpdateUser)
	app.DELETE("/user/", handle.DeleteUser)
}
