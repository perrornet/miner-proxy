package router

import (
	"miner-proxy/admin/app/handle"

	"github.com/gin-gonic/gin"
)

func NewClientRouter(app *gin.RouterGroup) {
	app = app.Group("/clients")

	app.GET("/", handle.ListClient)
	app.POST("/", handle.CreateClient)
	app.PUT("/", handle.UpdateClient)
	app.DELETE("/", handle.DeleteClient)

	app = app.Group("/forwards")
	app.POST("/", handle.CreateForward)
	app.PUT("/", handle.UpdateForward)
	app.DELETE("/", handle.DeleteForward)

}
