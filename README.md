# clash2sfa
用于将 clash 格式的订阅链接转换为 sing-box 格式，可用于安卓版本的 [SFA](https://sing-box.sagernet.org/installation/clients/sfa/)，ios 版本未测试。

## 部署
环境变量 `port` 控制程序运行所在的端口，若未设置默认开放在 8080 端口。

因为转换格式需要把订阅链接发送到服务器并保存在数据库中，建议自己部署。

## 使用
启动后使用浏览器访问 http://ip:port

## 配置文件模板
对配置文件模板中大多数修改都将被保留，在模板中的 outbounds 中增加节点也会被保留。

## 可转换的协议
通常的 clash 配置文件都可以转换，如果有哪个协议不能转换或者转换错误，请告诉我。

## 命令行版本
https://github.com/xmdhs/clash2singbox
