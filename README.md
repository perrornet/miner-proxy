<p align="center">
<a title="Require Go Version" target="_blank" href="https://perrorone.github.io/miner-proxy/">
<img src="https://github.com/PerrorOne/miner-proxy/blob/master/images/logo.png?raw=true"/>
</a>
<br/>
<a title="Build Status" target="_blank" href="https://github.com/PerrorOne/miner-proxy/actions?query=workflow%3ABuild+Release"><img src="https://img.shields.io/github/workflow/status/PerrorOne/miner-proxy/Build%20Release?style=flat-square&logo=github-actions" /></a>
<a title="Supported Platforms" target="_blank" href="https://github.com/PerrorOne/miner-proxy"><img src="https://img.shields.io/badge/platform-Linux%20%7C%20FreeBSD%20%7C%20Windows%7C%20Mac-549688?style=flat-square&logo=launchpad" /></a>
<a title="Require Go Version" target="_blank" href="https://github.com/PerrorOne/miner-proxy"><img src="https://img.shields.io/badge/go-%3E%3D1.17-30dff3?style=flat-square&logo=go" /></a>
<a title="Release" target="_blank" href="https://github.com/PerrorOne/miner-proxy/releases"><img src="https://img.shields.io/github/v/release/PerrorOne/miner-proxy.svg?color=161823&style=flat-square&logo=smartthings" /></a>
<a title="Tag" target="_blank" href="https://github.com/PerrorOne/miner-proxy/tags"><img src="https://img.shields.io/github/v/tag/PerrorOne/miner-proxy?color=%23ff8936&logo=fitbit&style=flat-square" /></a>
<a title="Chat Room" target="_blank" href="https://jq.qq.com/?_wv=1027&k=xh9ZfSix"><img src="https://github.com/PerrorOne/miner-proxy/blob/master/images/qq.svg?raw=true" /></a>
</p>

# 📃 简介
* `miner-proxy`底层基于TCP协议传输，支持stratum、openvpn、socks5、http、ssl等协议。
* `miner-proxy`内置加密、数据检验算法，使得他人无法篡改、查看您的原数据。混淆算法改变了您的数据流量特征无惧机器学习检测。
* `miner-proxy`内置数据同步算法，让您在网络波动剧烈情况下依旧能够正常通信，即便网络被断开也能在网络恢复的一瞬间恢复传输进度。

# 🛠️ 功能
- [x] 加密混淆数据, 破坏数据特征
- [x] 客户端支持随机http请求, 混淆上传下载数据
- [x] 服务端管理页面快捷下载客户端运行脚本
- [x] 单个客户端监听多端口并支持转发多个地址
- [x] 客户端支持随机http请求, 混淆上传下载数据
- [x] [官网](https://perrorone.github.io/miner-proxy/) 可以快速下载服务端运行脚本
- [x] 临时断网自动恢复数据传输, 无掉线
- [x] 多协议支持




# 🏛 官网
你可以访问 [miner-proxy](https://perrorone.github.io/miner-proxy/) 获取服务端的安装方式

# ⚠️ 证书
`miner-proxy` 需在遵循 [MIT](https://github.com/PerrorOne/miner-proxy/blob/master/LICENSE) 开源证书的前提下使用。