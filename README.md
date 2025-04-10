# clash2sfav vvvv
用于将 Clash.Meta 格式的订阅链接转换为 sing-box 格式，可用于安卓版本的 [SFA](https://sing-box.sagernet.org/installation/clients/sfa/)，ios 版本未测试。

## 部署
环境变量 `port` 控制程序运行所在的端口，若未设置默认开放在 8080 端口。

## docker
```
docker volume create clash2sfa    
docker run -d -p 8080:8080 -v clash2sfa:/server/db ghcr.io/xmdhs/clash2sfa
```
## 使用
启动后使用浏览器访问 http://ip:port

SFA remote 中填入链接，可以通过 https://yacd.metacubex.one/ 切换节点和全局/分流模式等。

demo https://clash2sfa-xmdhs.koyeb.app/ （因为转换格式需要把订阅链接发送到服务器并保存在数据库中，且数据库会**不定时删除**，建议自己部署。不过若使用参数**保存在链接中**则不会储存到数据库中，也不会因为数据库删除导致配置信息丢失。）
## 配置文件模板
对配置文件模板中大多数修改都将被保留，在模板中的 outbounds 中增加节点也会被保留。

## 可转换的协议
见 https://github.com/xmdhs/clash2singbox#%E6%94%AF%E6%8C%81%E5%8D%8F%E8%AE%AE

## 命令行版本
https://github.com/xmdhs/clash2singbox
