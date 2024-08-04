# clash2sfa
用于将 Clash.Meta 格式的订阅链接转换为 sing-box 格式订阅，可用于[SFA](https://sing-box.sagernet.org/zh/clients/)。

## 部署

[![Deploy with Vercel](https://vercel.com/button)](https://vercel.com/new/clone?repository-url=https%3A%2F%2Fgithub.com%2Fxmdhs%2Fclash2sfa)  一键部署到 Vercel

demo https://clash2sfa.xmdhs.com （建议自行部署）
## docker
```
docker run -d -p 8080:8080 ghcr.io/xmdhs/clash2sfa
```
## 使用
启动后使用浏览器访问 http://ip:port

SFA remote 中填入链接，可以通过 https://yacd.metacubex.one/ 切换节点和全局/分流模式等。
## 配置文件模板
对配置文件模板中大多数修改都将被保留，在模板中的 outbounds 中增加节点也会被保留。

## 可转换的协议
见 https://github.com/xmdhs/clash2singbox#%E6%94%AF%E6%8C%81%E5%8D%8F%E8%AE%AE

## 命令行版本
https://github.com/xmdhs/clash2singbox
