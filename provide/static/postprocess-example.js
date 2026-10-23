/*
 * 后处理脚本示例：按节点名把节点归到地区分组。
 *
 * 服务端会把本文件包进 function (config) { ... } 执行，config 是完整的
 * sing-box 配置对象，就地修改即可，也可以 return 一个新对象整体替换。
 *
 * 本地测试（不需要启动本服务）：
 *   1. 保存一份配置。用 sing-box 的 UA 请求得到单行 JSON：
 *        curl -s -A 'sing-box 1.12.0' 'https://example.com/sub?sub=订阅地址' -o config.json
 *   2. 同目录下运行：
 *        node --input-type=module -e '
 *          import fs from "node:fs";
 *          const config = JSON.parse(fs.readFileSync("config.json", "utf8"));
 *          new Function("config", fs.readFileSync("postprocess-example.js", "utf8"))(config);
 *          console.log(JSON.stringify(config, null, 4));
 *        '
 *   3. 检查输出里是否多出了「香港」「日本」「新加坡」三个分组。
 *
 * 注意：console.* 在本服务端是空实现，不产生任何输出，只能靠本地 node 调试。
 */

const regions = [
    { name: "香港", re: /^(hk|香港)/i },
    { name: "日本", re: /^(jp|日本)/i },
    { name: "新加坡", re: /^(sg|新加坡)/i },
];

// 已有的分组 tag，避免重复添加
const groups = new Set(config.outbounds.map((o) => o.tag));

for (const r of regions) {
    const tags = config.outbounds
        .filter((o) => typeof o.tag === "string" && r.re.test(o.tag))
        .map((o) => o.tag);
    if (tags.length === 0 || groups.has(r.name)) continue;
    config.outbounds.push({ type: "urltest", tag: r.name, outbounds: tags });
    groups.add(r.name);
}
