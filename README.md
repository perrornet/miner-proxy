# tcp 加密代理
*  [视频教程](https://perror.oss-cn-shanghai.aliyuncs.com/202112132134.mp4)
* 可以自定义密码通过AES对称加密流量,任何人都无法获悉流量数据包, 只要你的secret_key设置的足够复杂,几乎不可能破解
* 可以设置对矿工数据进行混淆处理, 如果使用了-sc参数, 将会对已经加密的数据插入同等数量的随机字符, 注意**这会导致带宽使用量翻倍**
* 本程序是为了国内挖矿流量加密而设置, 客户端运行在矿场局域网任意一台机器内, 服务端可以选择在香港云服务器上
* 不同与其他ssl加密代理, ssl加密依旧可以通过中间人的方式去篡改数据, 而采用这种方式, 除非你的密钥泄露再无其他办法解密数据
* 所有代码开源, 不会存在任何抽水! 也不会开发反抽水(请尊重软件开发者)! 
* 可以使用多次转发, 客户端 -> 服务器A端口转发 -> 服务器B服务端
* 支持在客户端指定服务端连接矿池地址, 在客户端使用`-sr`参数指定矿池地址, 则服务端将会舍弃启动时的`-r`参数, 连接客户端的`-sr`参数的地址, 示例:`-sr asia-erg.2miners.com:18888`, 对个客户端可以使用同一个服务端

## 安装
在 [Releases](https://github.com/PerrorOne/miner-proxy/releases) 选择适合你系统的二进制文件下载
### 您也可以自行编译
* golang version >= 1.13
* `git clone https://github.com/PerrorOne/miner-proxy && cd miner-proxy`
* `go mod tidy && cd ./cmd/miner && go build .`


## 使用

### win 端使用
#### 启动服务, 无界面运行, 并且开机启动(推荐)
1. 按住 win + R 输入 cmd 回车
2. 安装服务
```
# 在cmd中输入以下命令
完整目录/miner-proxy_windows_amd64.exe -l :5555 -r 服务端ip:服务端端口 -secret_key xxxx -sc -install -client
```
3.  启动服务
```
完整目录/miner-proxy_windows_amd64.exe -start
```
#### 不启动服务, 有界面运行, 并且开机启动
1. 新建 start-miner-proxy.bat 文件写入一下内容
```
完整目录/miner-proxy_windows_amd64.exe -l :5555 -r 服务端ip:服务端端口 -secret_key xxxx -sc -client
```
2. 按住 win + R 输入 shell:startup 回车将会打开一个目录, 将bat文件放在该目录下, 点击bat文件运行

### linux 端使用
#### 创建服务启动(推荐)
1. 安装服务: `完整目录/miner-proxy_linux_amd64 -l :5555 -r 矿池域名:矿池端口 -secret_key xxxx -sc -install`
2. 启动服务: `完整目录/miner-proxy_linux_amd64 -start`
2. 查看服务状态: `完整目录/miner-proxy_linux_amd64 -stat`
#### 通过supervisor启动
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

### 参数说明
```
  -client
        是否是客户端, 该参数必须准确, 默认服务端, 只有 secret_key 不为空时需要区分
  -debug
        是否开启debug
  -install
        添加到系统服务, 并且开机自动启动
  -l string
        本地监听地址 (default ":9999")
  -r string
        远程代理地址或者远程本程序的监听地址 (default "localhost:80")
  -remove
        移除系统服务, 并且关闭开机自动启动
  -restart
        重启代理服务
  -sc
        是否使用混淆数据, 如果指定了, 将会不定时在server/client之间发送随机的混淆数据以及在挖矿数据中插入随机数据
  -secret_key string
        数据包加密密钥, 只有远程地址也是本服务时才可使用
  -start
        启动代理服务
  -stat
        查看代理服务状态
  -stop
        暂停代理服务
```

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


## 矿工添加矿池示例
### 开源矿工
1. ![](./images/open-miner-add.png)
2. ![](./images/open-miner-add-pool.png)
3. 点击保存后点击"主矿池"搜索第一步中填写的矿池名称

### 轻松矿工
1. ![](./images/qskg-add.png)
2. ![](./images/qskg-add-02.png)
3. ![](./images/qskg-add-pool.png)
4. 点击"确定"后返回主界面, "矿池"选择刚才填写的矿池名称

### hiveos
1. ![](./images/hiveos-add.png)
2. ![](./images/hiveos-add-02.png)
3. ![](./images/hiveos-add-pool.png)
4. 点击"应用"后再点击更新即可
