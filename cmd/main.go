package main

import (
	"flag"
	"log"
	"miner-proxy/admin/app/database"
	"miner-proxy/admin/app/models"
	"miner-proxy/admin/app/router"
	"os"
	"path/filepath"

	"github.com/denisbrodbeck/machineid"

	"github.com/gin-gonic/gin"
)

func main() {
	port := flag.String("l", ":63548", "监听端口")
	flag.Parse()

	dir, _ := os.UserConfigDir()
	dir = filepath.Join(dir, "miner-proxy")
	if err := os.MkdirAll(dir, 0666); !os.IsExist(err) && err != nil {
		log.Fatalf("创建 %s 目录失败 %s", dir, err)
	}
	key, _ := machineid.ID()

	if err := database.InitDb(filepath.Join(dir, "miner-proxy.db"), key); err != nil {
		log.Fatalf("初始化数据库失败: %s", err)
	}
	_ = models.InitModels(database.GetDb())

	gin.SetMode(gin.ReleaseMode)
	app := gin.New()
	app.Use(gin.Recovery())

	router.InitRouter(app)
	if err := app.Run(*port); err != nil {
		log.Fatalf("监听 %s 端口失败, 您可以在可执行文件中添加 -l :自定义端口 命令来监听自定义端口", *port)
	}
}
