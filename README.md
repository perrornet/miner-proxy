<p align="center">
<a title="Require Go Version" target="_blank" href="https://perrorone.github.io/miner-proxy/">
<img src="https://github.com/PerrorOne/miner-proxy/blob/master/images/logo.png?raw=true"/>
</a>
<a title="Build Status" target="_blank" href="https://github.com/PerrorOne/miner-proxy/actions?query=workflow%3ABuild+Release"><img src="https://img.shields.io/github/workflow/status/PerrorOne/miner-proxy/Build%20Release?style=flat-square&logo=github-actions" /></a>
<a title="Supported Platforms" target="_blank" href="https://github.com/PerrorOne/miner-proxy"><img src="https://img.shields.io/badge/platform-Linux%20%7C%20FreeBSD%20%7C%20Windows%7C%20Mac-549688?style=flat-square&logo=launchpad" /></a>
<a title="Require Go Version" target="_blank" href="https://github.com/PerrorOne/miner-proxy"><img src="https://img.shields.io/badge/go-%3E%3D1.17-30dff3?style=flat-square&logo=go" /></a>
<a title="Release" target="_blank" href="https://github.com/PerrorOne/miner-proxy/releases"><img src="https://img.shields.io/github/v/release/PerrorOne/miner-proxy.svg?color=161823&style=flat-square&logo=smartthings" /></a>
<a title="Tag" target="_blank" href="https://github.com/PerrorOne/miner-proxy/tags"><img src="https://img.shields.io/github/v/tag/PerrorOne/miner-proxy?color=%23ff8936&logo=fitbit&style=flat-square" /></a>
</p>

# 简介
* **此软件仅仅只是个人练手项目, 初衷是熟练golang语言以及网络编程! 请勿将此软件用于挖矿活动!, 此软件不具备稳定性!! 如果你依旧使用那么本项目所有已经编译好的二进制文件将会阻断你所有挖矿的连接**
* golang编程交流Q群: 538528847
* **-debug为测试时使用!, 正式使用的时候请删除-debug参数, 否则磁盘不够大的情况下,极有可能导致磁盘爆满**
* 性能: 得益于golang的并发支持, 在作者win 4核32G电脑上同时部署一个客户端/一个服务端, **同时使用1000个tcp连接模拟矿机, 运行2小时后平均每秒处理 36.4M数据**
## 安装
###  视频教程 

https://user-images.githubusercontent.com/23651751/148649355-03d04371-efb9-4c80-884d-c08b64300f22.mp4


在 [Releases](https://github.com/PerrorOne/miner-proxy/releases) 选择适合你系统的二进制文件下载
### 您也可以自行编译
* golang version >= 1.17
* `git clone https://github.com/PerrorOne/miner-proxy && cd miner-proxy`
* `go mod tidy && cd ./cmd/miner-proxy && go build .`


## 使用

### 参数说明

参数 | 示例 | 说明
---|---|---
-client | - | 是否是客户端, 该参数必须准确, 默认服务端, 只有 secret_key 不为空时需要区分
-debug| - | 是否开启debug, 如果设置了这个参数, 那么将会开启更为详细的日志,, 建议测试时开启 
-l| -l :9999 | 本地监听地址 (default ":9999")
-r| -r 远程ip:端口 | 远程矿池地址或者远程本程序的监听地址 (default "localhost:80")
-log_file| -log_file ./miner-proxy.log |  将日志输入到./miner-proxy.log, 建议使用绝对路径
-secret_key| -secret_key 123456789 |  数据包加密密钥, 只有远程地址也是本服务时才可使用
-sc | - |  是否使用混淆数据, 如果指定了, 将会不定时在server/client之间发送随机的混淆数据以及在挖矿数据中插入随机数据
-sr | -sr baidu.com:80 |  客户端如果设置了这个参数, 那么服务端将会直接使用客户端的参数连接, 多个客户端互不干扰
-rsh |- | 指定该参数后, 客户端将会10-60秒发送一次http请求到定义好的随机网站中
-wx |-wx T_XXXX | 掉线微信通知token, 该参数只有在服务端生效, ,请在 https://wxpusher.zjiecode.com/admin/main/app/appToken 注册获取appToken
-add_wx_user |- | 绑定微信账号到微信通知中, 详细文档查看[v0.3.4](https://github.com/PerrorOne/miner-proxy/releases/tag/v0.3.4)
-offline|-offline 360 |掉线多少秒之后就发送微信通知,默认4分钟
-install | - |   添加到系统服务, 并且开机自动启动
-remove | - |   移除系统服务, 并且关闭开机自动启动
-restart| - |    重启代理服务
-start| -|   启动代理服务
-stat| -|   查看代理服务状态
-stop | - |    暂停代理服务
-h | - |help


### win 端使用
#### 启动服务, 无界面运行, 并且开机启动(推荐)
1. 按住 win + R 输入 cmd 回车
2. 安装服务
```
# 在cmd中输入以下命令
完整目录/miner-proxy_windows_amd64.exe -l :5555 -r 服务端ip:服务端端口 -secret_key xxxx -sc -install -client
```
3.  启动服务(**一台电脑只能安装一个服务**)
```
完整目录/miner-proxy_windows_amd64.exe -start
```
#### 不启动服务, 有界面运行, 并且开机启动(可以多开,只需要创建不同的bat文件名称即可)
1. 新建 start-miner-proxy.bat 文件写入一下内容
```
完整目录/miner-proxy_windows_amd64.exe -l :5555 -r 服务端ip:服务端端口 -secret_key xxxx -sc -client
```
2. 按住 win + R 输入 shell:startup 回车将会打开一个目录, 将bat文件放在该目录下, 点击bat文件运行

### linux 端使用
#### 创建服务启动(推荐**一台电脑只能安装一个服务**)
1. 安装服务: `完整目录/miner-proxy_linux_amd64 -l :5555 -r 矿池域名:矿池端口 -secret_key xxxx -sc -install`
2. 启动服务: `完整目录/miner-proxy_linux_amd64 -start`
3. 查看服务状态: `完整目录/miner-proxy_linux_amd64 -stat`
4. 查看日志:  `journalctl -f -u miner-proxy`

#### 通过supervisor启动(**可以多开, 需要修改miner-proxy.init为miner-proxy1.init, miner-proxy2.init, 以及[program:miner-proxy1], [program:miner-proxy2]**)
1. 安装supervisor, 请自行搜索supervisor在您系统中的安装方式
2. 写入配置文件, 输入命令: `vim /etc/supervisor/conf.d/miner-proxy.init`
3. 按i键进行编辑, 复制一下内容到文件中, 并将"完整目录"替换为 miner-proxy_linux_amd64 所在的目录
```
[program:miner-proxy]
command=完整目录/miner-proxy_linux_amd64 -l :5555 -r 矿池域名:矿池端口 -secret_key xxxx -sc
stdout_logfile=完整目录/miner-proxy.log
autostart=true
autorestart=true
ikillasgroup=true
```
4. 按ESC键, 随后输入:wq回车后即可保存
5. 输入命令: `supervisorctl reload && supervisorctl start miner-proxy && supervisorctl status`


* 分为服务端以及客户端
* 以f2pool挖erg为例
* 因为是在本地运行, 所以示例的ip, 服务端为: localhost, 客户端为: 127.0.0.1
### 服务端
```
# 监听 0.0.0.0:34567 并且转发请求到 erg.f2pool.com:7200
# 并且 服务端接受来自客户端的流量使用123456789密钥加密后插入混淆数据
./miner-proxy -l 0.0.0.0:34567 -r erg.f2pool.com:7200 -secret_key 123456789 -sc
# 输出: 
> miner-proxy (0.0.0-src) proxing from 0.0.0.0:34567 to erg.f2pool.com:7200
```

### 客户端
```
# 监听 0.0.0.0:34568 并且转发请求到 服务端ip:34567 
# 并且客户端的流量使用123456789密钥加密后插入混淆数据
./miner-proxy -l 0.0.0.0:34568 -r localhost:34567 -secret_key 123456789 -client -sc
# 输出:
> miner-proxy (0.0.0-src) proxing from 0.0.0.0:34568 to localhost:34567
```

### 使用者(与客户端处于同一台机器上或者同一个局域网内)
```
# 挖矿软件 Nbminer 设置
nbminer.exe -a ergo -o stratum+tcp://127.0.0.1:34568 -u perror.test -mt 3
```
### 作者本地测试日志截图
#### 客户端
```
2021-12-02 14:14:04 Connection 0xc000012e78 #001 读取到 520 加密数据, 解密后数据大小 246
2021/12/02 14:14:08 从 2021-12-02 13:55:38 至现在总计加密转发 26 kB 数据; 平均转发速度 23 B/秒
2021-12-02 14:14:26 Connection 0xc000012e78 #001 读取到 520 加密数据, 解密后数据大小 246
2021/12/02 14:14:38 从 2021-12-02 13:55:38 至现在总计加密转发 26 kB 数据; 平均转发速度 22 B/秒
2021-12-02 14:15:00 Connection 0xc000012e78 #001 读取到 183 明文数据, 加密后数据大小 392
2021-12-02 14:15:00 Connection 0xc000012e78 #001 读取到 152 加密数据, 解密后数据大小 58
2021/12/02 14:15:08 从 2021-12-02 13:55:38 至现在总计加密转发 27 kB 数据; 平均转发速度 22 B/秒
2021/12/02 14:15:38 从 2021-12-02 13:55:38 至现在总计加密转发 27 kB 数据; 平均转发速度 22 B/秒
2021/12/02 14:16:08 从 2021-12-02 13:55:38 至现在总计加密转发 27 kB 数据; 平均转发速度 21 B/秒
```
#### 服务端
```
2021-12-02 06:14:02 Connection 0xc0000ac5b0 #005 读取到 246 明文数据, 加密后数据大小 520
2021/12/02 06:14:13 从 2021-12-02 05:27:43 至现在总计加密转发 101 kB 数据; 平均转发速度 36 B/秒 
2021-12-02 06:14:24 Connection 0xc0000ac5b0 #005 读取到 246 明文数据, 加密后数据大小 520
2021/12/02 06:14:43 从 2021-12-02 05:27:43 至现在总计加密转发 101 kB 数据; 平均转发速度 35 B/秒 
2021-12-02 06:14:59 Connection 0xc0000ac5b0 #005 读取到 392 加密数据, 解密后数据大小 183
2021-12-02 06:14:59 Connection 0xc0000ac5b0 #005 读取到 58 明文数据, 加密后数据大小 152
2021/12/02 06:15:13 从 2021-12-02 05:27:43 至现在总计加密转发 102 kB 数据; 平均转发速度 35 B/秒 
2021/12/02 06:15:43 从 2021-12-02 05:27:43 至现在总计加密转发 102 kB 数据; 平均转发速度 35 B/秒 
2021/12/02 06:16:13 从 2021-12-02 05:27:43 至现在总计加密转发 102 kB 数据; 平均转发速度 34 B/秒
```

#### 挖矿端
```
[14:14:26] INFO - ================ [nbminer v39.1] Summary 2021-12-02 14:14:26 ================
[14:14:26] INFO - |ID|Device|Hashrate|Accept|Reject|Inv|Powr|Temp|Fan|CClk|GMClk|MUtl|Eff/Watt|
[14:14:26] INFO - | 0|  1060| 51.93 M|    14|     0|  0|  80|  67| 48|1739| 4303|  35| 649.2 K|
[14:14:26] INFO - |------------------+------+------+---+----+---------------------------------|
[14:14:26] INFO - |    Total: 51.93 M|    14|     0|  0|  80| Uptime:  0D 00:46:04   CPU: 82% |
[14:14:26] INFO - =============================================================================
[14:14:26] INFO - ergo - On Pool   10m: 69.37 M   4h: 44.20 M   24h: 44.20 M
[14:14:26] INFO - ergo - New job: 127.0.0.1:9999, ID: 236, HEIGHT: 632309, DIFF: 8.726G
[14:14:32] INFO - Device 0 ready for height 632309, 6.01 s.
[14:14:56] INFO - ================ [nbminer v39.1] Summary 2021-12-02 14:14:56 ================
[14:14:56] INFO - |ID|Device|Hashrate|Accept|Reject|Inv|Powr|Temp|Fan|CClk|GMClk|MUtl|Eff/Watt|
[14:14:56] INFO - | 0|  1060| 51.49 M|    14|     0|  0|  84|  67| 48|1739| 4303|  35| 613.0 K|
[14:14:56] INFO - |------------------+------+------+---+----+---------------------------------|
[14:14:56] INFO - |    Total: 51.49 M|    14|     0|  0|  84| Uptime:  0D 00:46:34   CPU: 82% |
[14:14:56] INFO - =============================================================================
[14:14:56] INFO - ergo - On Pool   10m: 72.72 M   4h: 43.73 M   24h: 43.73 M
[14:15:00] INFO - ergo - #15 Share accepted, 78 ms. [DEVICE 0, #15]
[14:15:26] INFO - ================ [nbminer v39.1] Summary 2021-12-02 14:15:26 ================
[14:15:26] INFO - |ID|Device|Hashrate|Accept|Reject|Inv|Powr|Temp|Fan|CClk|GMClk|MUtl|Eff/Watt|
[14:15:26] INFO - | 0|  1060| 52.09 M|    15|     0|  0|  83|  68| 48|1739| 4303|  35| 627.6 K|
[14:15:26] INFO - |------------------+------+------+---+----+---------------------------------|
[14:15:26] INFO - |    Total: 52.09 M|    15|     0|  0|  83| Uptime:  0D 00:47:04   CPU: 84% |
[14:15:26] INFO - =============================================================================
[14:15:26] INFO - ergo - On Pool   10m: 83.11 M   4h: 46.35 M   24h: 46.35 M
```


## 参数说明
* 客户端创建服务: `./miner-proxy -install -debug -client -l :5556 -r {服务器ip}:5558  -secret_key {自定义密钥} -sc` 
* 服务端创建服务: `./miner-proxy -install -debug -l :5558 -r {矿池host+port}  -secret_key {自定义密钥} -sc`
* 运行服务`./miner-proxy -start`
* 停止服务`./miner-proxy -stop`
* 重启服务`./miner-proxy -restart`
* 删除服务`./miner-proxy -remove`
* 查看服务状态`./miner-proxy -stat`

## 添加Docker启动方式
为方便快速部署，可移植性，采用Docker容器化方式部署

### 构建镜像
```
docker build -t miner-proxy:latest .
```

### 启动服务端容器
```
docker run \
      -p 9999:9999 \
      --restart=always \
      --name miner-proxy \
      -d miner-proxy:latest \
      miner-proxy -l :9999 -r 矿池地址:矿池端口号 -secret_key 12345 -sc
```

### 启动客户端容器
```
docker run \
      -p 9999:9999 \
      --restart=always \
      --name miner-proxy \
      -d miner-proxy:latest \
      miner-proxy -l :9999 -r 服务端ip:服务端端口 -secret_key 12345 -sc -client
```

### 启动客户端容器
```
docker run \
      -p 9999:9999 \
      --restart=always \
      --name miner-proxy \
      -d miner-proxy:latest \
      miner-proxy -l :9999 -r 服务端ip:服务端端口 -secret_key 12345 -sc -client
```
### docker-compose 启动方式
 
```
 # 第一次运行时会自动构建镜像 参数调整请进入docker-compose.yml 进行修改
 # 代码更新后需要 强制更新一次镜像 使用 docker-compose up -d --build server 或者 client
 # -d 后台运行
docker-compose up -d server 
docker-compose up -d client   
```

### 查看容器日志
```
docker logs -f -t --tail=100 miner-proxy  # -f 实时查看 -t带时间戳的 --tail=100最新100行日志
```

### 查看容器状态
```
docker stats miner-proxy
```

### 查看容器内进程状态
```
docker top miner-proxy
```
