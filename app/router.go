package app

import (
	"miner-proxy/app/handles"
	"miner-proxy/proxy/server"

	"github.com/gin-gonic/gin"
)

func NewRouter(app *gin.Engine) {

	app.GET("/api/clients/", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": server.ClientInfo(), "code": 200})
	})

	app.GET("/api/server/version/", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": c.GetString("tag"), "code": 200})
	})

	app.POST("/api/client/download/", handles.PackScriptFile)
	app.GET("/download/:fileName", handles.File)

}
