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

## 后处理脚本
生成的配置还可以用一段 JavaScript 做最后加工，适合模板本身表达不了的情况，比如按节点名重排分组、批量改写 tag、增删规则。

脚本以函数的形参 `config` 拿到完整配置（普通 JS 对象），可以就地修改，也可以 `return` 一个新对象替换它；返回 `undefined` 时沿用修改后的 `config`。返回值必须是 JSON 对象。

```js
// 给香港节点加上中文前缀，并把它们放进一个独立的分组
const hk = [];
for (const o of config.outbounds) {
    if (typeof o.tag === "string" && o.tag.startsWith("hk")) {
        o.tag = "香港 " + o.tag;
        hk.push(o.tag);
    }
}
config.outbounds.push({ type: "selector", tag: "香港", outbounds: hk });
```

两种提供方式，直链优先：

| 参数 | 说明 |
| --- | --- |
| `scripturl` | 脚本直链，服务端会抓取它，大小上限 1 MiB |
| `script` | 内联脚本，`base64url(zlib(源码))`，与 `config` 参数同一套编码 |

例如 `https://example.com/sub?sub=...&scripturl=https://example.com/pp.js`。

限制：

- 单次执行 2 秒超时，超时中断；单次执行内存上限 64 MiB
- `eval` / `new Function` 不可用
- `console.*` 可以调用，但不产生任何输出
- 脚本在 detour 展开、分组 `include:` / `exclude:` 解析之后运行，看到的是最终配置
- 公共部署请谨慎开启：任何能访问 `/sub` 的人都可以执行任意 JS（虽受上面限制约束，仍会消耗 CPU）

编码也可以用命令行生成：

```sh
# zlib 压缩 + base64url，去掉末尾的 =
printf '%s' 'config.log = { level: "debug" };' | python3 -c 'import sys,zlib,base64; print(base64.urlsafe_b64encode(zlib.compress(sys.stdin.buffer.read(),9)).decode().rstrip("="))'
```

## 可转换的协议
见 https://github.com/xmdhs/clash2singbox#%E6%94%AF%E6%8C%81%E5%8D%8F%E8%AE%AE

## 命令行版本
https://github.com/xmdhs/clash2singbox
