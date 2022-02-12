package app

import (
	"miner-proxy/app/handles"
	"miner-proxy/proxy/server"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Client struct {
	ClientId string   `json:"client_id"`
	Connects []string `json:"connects"`
}

func NewRouter(app *gin.Engine) {
	app.GET("/api/client/status/", func(c *gin.Context) {
		clients := server.GetClientStatus()
		c.JSON(http.StatusOK, gin.H{
			"data": clients,
		})
		return
	})

	app.DELETE("/api/client/status/total_size/", func(c *gin.Context) {
		server.ClearTotalSize()
		c.JSON(http.StatusOK, gin.H{})
		return
	})

	app.GET("/api/clients/", func(c *gin.Context) {
		var clients []Client
		for k, ip := range server.ClientInfo() {
			clients = append(clients, Client{
				ClientId: k,
				Connects: ip,
			})
		}
		c.JSON(200, gin.H{"data": clients, "code": 200})
	})

	app.POST("/api/client/download/", handles.PackScriptFile)
	app.GET("/download/:fileName", handles.File)

}
