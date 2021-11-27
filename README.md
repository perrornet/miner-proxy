**本分支还在开发中, 请勿使用**
# tcp 加密代理 
* 可以自定义密码通过AES对称加密流量,任何人都无法获悉流量数据包, 只要你的secret_key设置的足够复杂,几乎不可能破解
* 本程序是为了国内挖矿流量加密而设置, 客户端运行在矿场局域网任意一台机器内, 服务端可以选择在香港云服务器上
* 不同与其他ssl加密代理, ssl加密依旧可以通过中间人的方式去篡改数据, 而采用这种方式, 除非你的密钥泄露再无其他办法解密数据
* 所有代码开源, 不会存在任何抽水! 也不会开发反抽水(请尊重软件开发者)! 

## 安装
在Releases选择适合你系统的二进制文件下载

## 使用
* 分为服务端以及客户端
* 以f2pool挖erg为例
* 因为是在本地运行, 所以示例的ip, 服务端为: localhost, 客户端为: 127.0.0.1
### 服务端
```
# 监听 0.0.0.0:34567 并且转发请求到 erg.f2pool.com:7200
# 并且 服务端接受来自客户端的流量使用123456789密钥加密
./miner-proxy -l 0.0.0.0:34567 -r erg.f2pool.com:7200 -secret_key 123456789 
# 输出: 
> miner-proxy (0.0.0-src) proxing from 0.0.0.0:34567 to erg.f2pool.com:7200
```

### 客户端
```
# 监听 0.0.0.0:34568 并且转发请求到 服务端ip:34567 
# 并且客户端的流量使用123456789密钥加密
./miner-proxy -l 0.0.0.0:34568 -r localhost:34567 -secret_key 123456789 -client
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
2021-11-21 18:59:05 miner-proxy (0.0.0-src) proxing from 0.0.0.0:34568 to localhost:34567
2021-11-21 18:59:06 Connection #001 Opened 0.0.0.0:34568 >>> 127.0.0.1:34567
2021-11-21 18:59:06 Connection #001 >>> 63 bytes sent
2021-11-21 18:59:06 Connection #001 发送到服务端数据包, 稍后加密数据: {"id":1,"method":"mining.subscribe","params":["NBMiner/39.6"]}
2021-11-21 18:59:06 Connection #001 <<< 155 bytes recieved
2021-11-21 18:59:06 Connection #001 解密后数据包: {"id":1,"result":[[["mining.set_difficulty","mining.set_difficulty"],["mining.notify","mining.notify"]],"f04b",6],"error":null}
2021-11-21 18:59:06 Connection #001 >>> 131 bytes sent
2021-11-21 18:59:06 Connection #001 发送到服务端数据包, 稍后加密数据: {"id":2,"method":"mining.authorize","params":["perror.MVFQPPLWRN",""]}
{"id":3,"method":"mining.extranonce.subscribe","params":[]}
2021-11-21 18:59:06 Connection #001 <<< 347 bytes recieved
2021-11-21 18:59:06 Connection #001 解密后数据包: {"id":2,"result":true,"error":null}
{"id":null,"method":"mining.set_difficulty","params":[4294967296]}
{"id":null,"method":"mining.notify","params":["c74c0300f04b",624668,"c1a11cf6d24d23bcec87c2b88bc50eeb2a7a53981e7ce98205c643c4e7f3872d","","",2,"26959946667150639794667015087019630673637144422540572481103610249216","",true]}
2021-11-21 18:59:18 Connection #001 >>> 125 bytes sent
2021-11-21 18:59:18 Connection #001 发送到服务端数据包, 稍后加密数据: {"id":4,"method":"mining.submit","params":["perror.MVFQPPLWRN","c74c0300f04b","000026457d37","00000000","f04b000026457d37"]}
2021-11-21 18:59:18 Connection #001 <<< 59 bytes recieved
2021-11-21 18:59:18 Connection #001 解密后数据包: {"id":4,"result":true,"error":null}
2021-11-21 18:59:26 Connection #001 <<< 251 bytes recieved
2021-11-21 18:59:26 Connection #001 解密后数据包: {"id":null,"method":"mining.notify","params":["11ad40300f04b",624669,"edf4109bb740316603045bd4cb00915d1fdcfeae2f35ad6f05058cfa3f1227fd","","",2,"2695994666715063979466701508701963067
3637144422540572481103610249216","",true]}
2021/11/21 18:59:36 从 2021-11-21 18:59:06 至现在总计加密转发 1.1 kB 数据
2021-11-21 18:59:45 Connection #001 <<< 251 bytes recieved
2021-11-21 18:59:45 Connection #001 解密后数据包: {"id":null,"method":"mining.notify","params":["1245c0300f04b",624670,"2d51bb44fbc99cab8d1176bc3d18e880957a043f22888f49fa25b834dcd472c0","","",2,"2695994666715063979466701508701963067
3637144422540572481103610249216","",true]}
2021/11/21 19:00:06 从 2021-11-21 18:59:06 至现在总计加密转发 1.3 kB 数据
2021/11/21 19:00:36 从 2021-11-21 18:59:06 至现在总计加密转发 1.3 kB 数据
```
#### 服务端
```
Connection #007 Opened 0.0.0.0:34567 >>> 39.104.177.208:7200
Connection #007 >>> 75 bytes sent
Connection #007 解密后数据包: {"id":1,"method":"mining.subscribe","params":["NBMiner/39.6"]}
Connection #007 <<< 128 bytes recieved
Connection #007 发送到客户端数据包, 稍后加密数据: {"id":1,"result":[[["mining.set_difficulty","mining.set_difficulty"],["mining.notify","mining.notify"]],"f04b",6],"error":null}
Connection #007 >>> 155 bytes sent
Connection #007 解密后数据包: {"id":2,"method":"mining.authorize","params":["perror.MVFQPPLWRN",""]}
{"id":3,"method":"mining.extranonce.subscribe","params":[]}
Connection #007 <<< 327 bytes recieved
Connection #007 发送到客户端数据包, 稍后加密数据: {"id":2,"result":true,"error":null}
{"id":null,"method":"mining.set_difficulty","params":[4294967296]}
{"id":null,"method":"mining.notify","params":["c74c0300f04b",624668,"c1a11cf6d24d23bcec87c2b88bc50eeb2a7a53981e7ce98205c643c4e7f3872d","","",2,"26959946667150639794667015087019630673637144422540572481103610249216","",true]}
Connection #007 >>> 139 bytes sent
Connection #007 解密后数据包: {"id":4,"method":"mining.submit","params":["perror.MVFQPPLWRN","c74c0300f04b","000026457d37","00000000","f04b000026457d37"]}
Connection #007 <<< 36 bytes recieved
Connection #007 发送到客户端数据包, 稍后加密数据: {"id":4,"result":true,"error":null}
Connection #007 <<< 225 bytes recieved
Connection #007 发送到客户端数据包, 稍后加密数据: {"id":null,"method":"mining.notify","params":["11ad40300f04b",624669,"edf4109bb740316603045bd4cb00915d1fdcfeae2f35ad6f05058cfa3f1227fd","","",2,"2695994666715063979466701508701963067
3637144422540572481103610249216","",true]}
Connection #007 <<< 225 bytes recieved
Connection #007 发送到客户端数据包, 稍后加密数据: {"id":null,"method":"mining.notify","params":["1245c0300f04b",624670,"2d51bb44fbc99cab8d1176bc3d18e880957a043f22888f49fa25b834dcd472c0","","",2,"2695994666715063979466701508701963067
3637144422540572481103610249216","",true]}
```

#### 挖矿端
```
[18:59:05] INFO - ergo - Logging in to 127.0.0.1:34568 ...
[18:59:06] INFO - Set extranonce: f04b
[18:59:06] INFO - ergo - Login succeeded.
[18:59:06] INFO - ergo - New job: 127.0.0.1:34568, ID: 0300f04b, HEIGHT: 624668, DIFF: 4.295G
[18:59:06] INFO - ================== [nbminer v39.6] Summary 2021-11-21 18:59:06 ===================
[18:59:06] INFO - |ID|Device|Hashrate|Accept|Reject|Inv|Powr|CTmp|MTmp|Fan|CClk|GMClk|MUtl|Eff/Watt|
[18:59:06] INFO - | 0|  1060| 52.61 M|     8|     0|  0|  89|  61|    | 98|1759| 4303|  35| 591.1 K|
[18:59:06] INFO - |------------------+------+------+---+----+--------------------------------------|
[18:59:06] INFO - |    Total: 52.61 M|     8|     0|  0|  89| Uptime:  0D 00:18:30        CPU: 27% |
[18:59:06] INFO - ==================================================================================
[18:59:06] INFO - ergo - On Pool   10m: 50.11 M   4h: 30.95 M   24h: 30.95 M
[18:59:18] INFO - ergo - #9 Share accepted, 59 ms. [DEVICE 0, #9]
[18:59:26] INFO - ergo - New job: 127.0.0.1:34568, ID: 0300f04b, HEIGHT: 624669, DIFF: 4.295G
[18:59:32] INFO - Device 0 ready for height 624669, 5.82 s.
[18:59:36] INFO - ================== [nbminer v39.6] Summary 2021-11-21 18:59:36 ===================
[18:59:36] INFO - |ID|Device|Hashrate|Accept|Reject|Inv|Powr|CTmp|MTmp|Fan|CClk|GMClk|MUtl|Eff/Watt|
[18:59:36] INFO - | 0|  1060| 53.19 M|     9|     0|  0|  90|  61|    | 97|1645| 4303|   1| 591.0 K|
[18:59:36] INFO - |------------------+------+------+---+----+--------------------------------------|
[18:59:36] INFO - |    Total: 53.19 M|     9|     0|  0|  90| Uptime:  0D 00:19:00        CPU:  3% |
[18:59:36] INFO - ==================================================================================
[18:59:36] INFO - ergo - On Pool   10m: 54.54 M   4h: 33.91 M   24h: 33.91 M
[18:59:45] INFO - ergo - New job: 127.0.0.1:34568, ID: 0300f04b, HEIGHT: 624670, DIFF: 4.295G
[18:59:51] INFO - Device 0 ready for height 624670, 5.83 s.
[19:00:06] INFO - ================== [nbminer v39.6] Summary 2021-11-21 19:00:06 ===================
[19:00:06] INFO - |ID|Device|Hashrate|Accept|Reject|Inv|Powr|CTmp|MTmp|Fan|CClk|GMClk|MUtl|Eff/Watt|
[19:00:06] INFO - | 0|  1060| 53.38 M|     9|     0|  0|  89|  61|    | 98|1759| 4303|  35| 599.8 K|
[19:00:06] INFO - |------------------+------+------+---+----+--------------------------------------|
[19:00:06] INFO - |    Total: 53.38 M|     9|     0|  0|  89| Uptime:  0D 00:19:30        CPU:  3% |
[19:00:06] INFO - ==================================================================================
[19:00:06] INFO - ergo - On Pool   10m: 57.27 M   4h: 33.04 M   24h: 33.04 M
[19:00:36] INFO - ================== [nbminer v39.6] Summary 2021-11-21 19:00:36 ===================
[19:00:36] INFO - |ID|Device|Hashrate|Accept|Reject|Inv|Powr|CTmp|MTmp|Fan|CClk|GMClk|MUtl|Eff/Watt|
[19:00:36] INFO - | 0|  1060| 53.13 M|     9|     0|  0|  88|  61|    | 97|1759| 4303|  35| 603.7 K|
[19:00:36] INFO - |------------------+------+------+---+----+--------------------------------------|
[19:00:36] INFO - |    Total: 53.13 M|     9|     0|  0|  88| Uptime:  0D 00:20:00        CPU: 15% |
[19:00:36] INFO - ==================================================================================
[19:00:36] INFO - ergo - On Pool   10m: 42.95 M   4h: 32.21 M   24h: 32.21 M
19:01:19 669  UI NTMiner Info        手动停止挖矿
19:01:19 847     NTMiner Warn        挖矿已停止
```


## 参数说明
* 客户端使用服务的方式启动
```
# 创建服务
./miner-proxy -install -debug -client -l :5556 -r {服务器ip}:5558  -secret_key 123456789

```
* 服务端使用服务的方式启动
```
./miner-proxy -install -debug -l :5558 -r {矿池host+port}  -secret_key 123456789
```
* 运行服务`./miner-proxy -start`
* 停止服务`./miner-proxy -stop`
* 重启服务`./miner-proxy -restart`
* 删除服务`./miner-proxy -remove`
* 删查看服务状态`./miner-proxy -stat`


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

### 本程序基础转发代码来自 https://github.com/jpillora/miner-proxy 存储库
