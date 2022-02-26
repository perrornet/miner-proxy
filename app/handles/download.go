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
)

var (
	BASEDIR = "./download"
)

type ZipParams struct {
	ClientVersion      string    `json:"client_version"`
	ClientSystemType   string    `json:"client_system_type"`
	ClientSystemStruct string    `json:"client_system_struct"`
	ClientRunType      string    `json:"client_run_type"`
	Forward            []Forward `json:"forward"`
}

type Forward struct {
	Port string `json:"port"`
	Pool string `json:"pool"`
}

func (z ZipParams) ID() string {
	var ports []string
	for _, v := range z.Forward {
		ports = append(ports, v.Port, v.Pool)
	}
	name := pkg.Crc32IEEEStr(fmt.Sprintf("%s-%s-%s-%s-%s",
		z.ClientSystemType, z.ClientSystemStruct, z.ClientRunType, strings.Join(ports, ","), z.ClientVersion))
	return name
}

func (z ZipParams) build(filename, secretKey, serverPort, serverHost string) []byte {
	var args []string

	if strings.ToLower(z.ClientSystemType) == "windows" { // bat
		filename = fmt.Sprintf(".\\%s", filename)
	} else {
		filename = fmt.Sprintf("./%s", filename)
	}
	args = append(args, filename)
	args = append(args, "-k", secretKey,
		"-r", fmt.Sprintf("%s:%s", serverHost, serverPort))

	var port []string
	var pool []string
	for _, v := range z.Forward {
		port = append(port, fmt.Sprintf(":%s", v.Port))
		pool = append(pool, v.Pool)
	}

	args = append(args, "-l", strings.Join(port, ","), "-c")
	args = append(args, "-u", strings.Join(pool, ","))

	switch strings.ToLower(z.ClientRunType) {
	case "service":
		args = append(args, "install")
		if strings.ToLower(z.ClientSystemType) == "windows" { // bat
			args = []string{
				fmt.Sprintf("%s --delete\npause", strings.Join(args, " ")),
			}
		} else {
			args = []string{fmt.Sprintf("chmod +x %s\n%s --delete", fmt.Sprintf("%s", filename), strings.Join(args, " "))}
		}
	case "backend":
		if strings.ToLower(z.ClientSystemType) == "windows" { // bat
			cmd := "@echo off\nif \"%1\"==\"h\" goto begin\n\nstart mshta vbscript:createobject(\"wscript.shell\").run(\"\"\"%~nx0\"\" h\",0)(window.close)&&exit\n\n:begin"

			args = []string{
				fmt.Sprintf("%s\n\n%s", cmd, strings.Join(args, " ")),
			}
		} else {
			args = []string{
				fmt.Sprintf("chmod +x %s\nnohup %s > ./miner-proxy.log 2>& 1&", filename, strings.Join(args, " ")),
			}
		}
	case "frontend":
		if strings.ToLower(z.ClientSystemType) == "windows" { // bat
			args = []string{
				fmt.Sprintf("%s\npause", strings.Join(args, " ")),
			}
		} else {
			args = []string{fmt.Sprintf("sudo su\nchmod +x %s\n%s", filename, strings.Join(args, " "))}
		}
	}
	return []byte(strings.Join(args, " "))
}

func (z ZipParams) Check() error {
	if z.ClientSystemType == "" || z.ClientSystemStruct == "" || z.ClientRunType == "" || len(z.Forward) == 0 {
		return fmt.Errorf("参数错误")
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

	u := "https://github.com/PerrorOne/miner-proxy/releases/download/" + args.ClientVersion
	if dgu := c.GetString("download_github_url"); dgu != "" {
		if !strings.HasSuffix(dgu, "/") {
			dgu = dgu + "/"
		}

		if strings.HasPrefix(dgu, "http") {
			u = fmt.Sprintf("%s%s", dgu, u)
		}
	}

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
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			c.JSON(200, gin.H{"code": 500, "msg": fmt.Sprintf("没有在 %s 中发现任何脚本内容, 请检查您的设置是否有问题", fmt.Sprintf("%s/%s", u, filename))})
			return
		}

		f, err := os.OpenFile(filepath.Join(dir, filename), os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			c.JSON(200, gin.H{"code": 500, "msg": fmt.Sprintf("创建临时文件失败: %s", err)})
			return
		}
		if _, err := io.Copy(f, resp.Body); err != nil {
			_ = f.Close()
			c.JSON(200, gin.H{"code": 500, "msg": fmt.Sprintf("创建临时文件失败: %s", err)})
			return
		}
		_ = f.Close()
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
