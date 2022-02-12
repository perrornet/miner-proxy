package handles

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"miner-proxy/pkg"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/segmentio/ksuid"
	"github.com/spf13/cast"
)

var (
	BASEDIR = "./download"
)

type ZipParams struct {
	ClientSystemType   string `json:"client_system_type"`
	ClientSystemStruct string `json:"client_system_struct"`
	ClientRunType      string `json:"client_run_type"`
	ClientListenPort   string `json:"client_listen_port"`
	ClientPool         string `json:"client_pool"`
}

func (z ZipParams) ID() string {
	name := pkg.Md5(fmt.Sprintf("%s-%s-%s-%s-%s",
		z.ClientSystemType, z.ClientSystemStruct, z.ClientRunType, z.ClientListenPort, z.ClientPool))[:10]
	return pkg.CLIENTID + name
}

func (z ZipParams) build(filename, secretKey, serverPort, serverHost string) []byte {
	var args []string

	if strings.ToLower(z.ClientSystemType) == "windows" { // bat
		args = append(args, fmt.Sprintf(".\\%s", filename))
	} else {
		args = append(args, fmt.Sprintf("./%s", filename))
	}

	args = append(args, "--key", secretKey,
		"-r", fmt.Sprintf("%s:%s", serverHost, serverPort))
	if z.ClientPool != "" {
		args = append(args, "--pool", z.ClientPool)
	}
	args = append(args, "-l", fmt.Sprintf(":%s", z.ClientListenPort), "--client")

	switch strings.ToLower(z.ClientRunType) {
	case "service":
		args = append(args, "install")
		if strings.ToLower(z.ClientSystemType) == "windows" { // bat
			args = []string{
				fmt.Sprintf("%s\npause", strings.Join(args, " ")),
			}
		}
	case "backend":
		if strings.ToLower(z.ClientSystemType) == "windows" { // bat
			args = append([]string{"start", "/b"}, args...)
			args = []string{
				fmt.Sprintf(`%s\npause`, strings.Join(args, " ")),
			}
		} else {
			args = append([]string{"nohup"}, args...)
			args = append(args, ">>", "../miner-proxy.log", "2>&", "1&")
		}
	case "frontend":
		if strings.ToLower(z.ClientSystemType) == "windows" { // bat
			args = []string{
				fmt.Sprintf(`@cd /d "*%~dp0"\n%s\npause`, strings.Join(args, " ")),
			}
		}
	}
	return []byte(strings.Join(args, " "))
}

func (z ZipParams) Check() error {
	if z.ClientSystemType == "" || z.ClientSystemStruct == "" || z.ClientRunType == "" || z.ClientListenPort == "" {
		return fmt.Errorf("参数错误")
	}
	if cast.ToInt(z.ClientListenPort) <= 0 {
		return fmt.Errorf("客户端端口错误")
	}
	return nil
}

func PackScriptFile(c *gin.Context) {
	args := new(ZipParams)
	if err := c.BindJSON(args); err != nil {
		c.JSON(200, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	currentDir := ksuid.New().String()
	dir := filepath.Join(BASEDIR, currentDir)
	// build download url
	u := "https://github.abskoop.workers.dev/https://github.com/PerrorOne/miner-proxy/releases/download/" + c.GetString("tag")
	filename := fmt.Sprintf("miner-proxy_%s_%s", args.ClientSystemType, args.ClientSystemStruct)
	if err := os.MkdirAll(dir, 0666); err != nil {
		c.JSON(200, gin.H{"code": 500, "msg": fmt.Sprintf("创建临时文件失败: %s", err)})
		return
	}
	if args.ClientSystemType == "windows" {
		filename += ".exe"
	}

	if _, err := os.Stat(filepath.Join(dir, filename)); err != nil {
		resp, err := http.Get(fmt.Sprintf("%s/%s", u, filename))
		if err != nil {
			c.JSON(200, gin.H{"code": 500, "msg": fmt.Sprintf("从github中下载脚本失败: %s", err)})
			return
		}
		if resp.StatusCode != 200 {
			c.JSON(200, gin.H{"code": 500, "msg": fmt.Sprintf("没有在 %s 中发现任何脚本内容, 请检查您的设置是否有问题", fmt.Sprintf("%s/%s", u, filename))})
			return
		}
		defer resp.Body.Close()
		f, err := os.OpenFile(filepath.Join(dir, filename), os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			c.JSON(200, gin.H{"code": 500, "msg": fmt.Sprintf("创建临时文件失败: %s", err)})
			return
		}
		if _, err := io.Copy(f, resp.Body); err != nil {
			f.Close()
			c.JSON(200, gin.H{"code": 500, "msg": fmt.Sprintf("创建临时文件失败: %s", err)})
			return
		}
		f.Close()
	}
	// 构建文件
	data := args.build(filename, strings.TrimRight(c.GetString("secretKey"), "0"), c.GetString("server_port"), strings.Split(c.Request.Host, ":")[0])
	if err := os.MkdirAll(dir, 0666); err != nil {
		c.JSON(200, gin.H{"code": 500, "msg": fmt.Sprintf("创建临时文件失败: %s", err)})
		return
	}
	var runName = "run.bat"
	if args.ClientSystemType != "windows" {
		runName = "run.sh"
	}
	if err := ioutil.WriteFile(filepath.Join(dir, runName), data, 0666); err != nil {
		c.JSON(200, gin.H{"code": 500, "msg": fmt.Sprintf("写入文件失败: %s", err)})
		return
	}
	// 压缩
	zipName := fmt.Sprintf("miner-proxy-%s.zip", args.ID())
	zapF, err := os.OpenFile(filepath.Join(BASEDIR, zipName), os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		c.JSON(200, gin.H{"code": 500, "msg": fmt.Sprintf("创建zip文件失败: %s", err)})
		return
	}
	defer zapF.Close()
	if err := pkg.Zip(dir, zapF); err != nil {
		c.JSON(200, gin.H{"code": 500, "msg": fmt.Sprintf("压缩文件失败: %s", err)})
		return
	}
	downloadPath := fmt.Sprintf("/download/%s?name=%s", zipName, currentDir)
	downloadPath = strings.ReplaceAll(downloadPath, "\\", "/")
	c.JSON(200, gin.H{"code": 200, "msg": "ok", "data": downloadPath})
	return
}

func File(c *gin.Context) {
	info, err := os.Stat(filepath.Join(BASEDIR, c.Param("fileName")))
	if err != nil {
		c.AbortWithStatus(404)
		return
	}
	if info.IsDir() || info.Size() >= 50*1024*1024 {
		c.AbortWithStatus(404)
		return
	}

	info, err = os.Stat(filepath.Join(BASEDIR, c.Query("name")))
	if err != nil {
		c.AbortWithStatus(404)
		return
	}

	if !info.IsDir() {
		c.AbortWithStatus(404)
		return
	}

	data, err := ioutil.ReadFile(filepath.Join(BASEDIR, c.Param("fileName")))
	if err != nil {
		c.AbortWithStatus(404)
		return
	}

	c.DataFromReader(200, int64(len(data)), "application/x-zip-compressed", bytes.NewReader(data), map[string]string{
		"Content-Disposition": fmt.Sprintf(`attachment; filename="%s"`, c.Param("fileName")),
	})
	return
}
