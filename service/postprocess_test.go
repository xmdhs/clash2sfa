package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmdhs/clash2sfa/model"
	cmodel "github.com/xmdhs/clash2singbox/model"
)

// multiRT 按 URL 返回不同内容，用于模拟订阅与脚本直链。
type multiRT map[string][]byte

func (m multiRT) RoundTrip(r *http.Request) (*http.Response, error) {
	body, ok := m[r.URL.String()]
	if !ok {
		return &http.Response{StatusCode: http.StatusNotFound, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("not found"))}, nil
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(body))}, nil
}

const ppSubYAML = `
proxies:
  - name: hk-01
    type: vmess
    server: 1.2.3.4
    port: "443"
    uuid: uuid
  - name: jp-01
    type: vmess
    server: 5.6.7.8
    port: "443"
    uuid: uuid
`

func ppConvert(t *testing.T, script string, scriptURL string) map[string]any {
	t.Helper()
	client := &http.Client{Transport: multiRT{
		"https://example.com/sub":   []byte(ppSubYAML),
		"https://example.com/pp.js": []byte(script),
	}}
	c := NewConvert(client, newSilentLogger())
	arg := model.ConvertArg{
		Sub:       "https://example.com/sub",
		Config:    []byte(`{"outbounds":[]}`),
		Ver:       cmodel.SINGLATEST,
		Script:    nil,
		ScriptUrl: scriptURL,
	}
	if scriptURL == "" {
		arg.Script = []byte(script)
	}
	b, err := c.MakeConfig(context.Background(), arg, nil, "sing-box/2.0")
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	return m
}

func outboundTags(m map[string]any) []string {
	list, _ := m["outbounds"].([]any)
	tags := make([]string, 0, len(list))
	for _, item := range list {
		o, _ := item.(map[string]any)
		if tag, ok := o["tag"].(string); ok {
			tags = append(tags, tag)
		}
	}
	return tags
}

func TestPostprocessMutatesInPlace(t *testing.T) {
	m := ppConvert(t, `
config.log = { level: "warn" };
config.outbounds = config.outbounds.filter(o => o.type !== "urltest");
`, "")
	assert.Equal(t, map[string]any{"level": "warn"}, m["log"])
	assert.NotContains(t, outboundTags(m), "urltest")
	assert.Contains(t, outboundTags(m), "hk-01")
}

func TestPostprocessReturnsNewConfig(t *testing.T) {
	m := ppConvert(t, `return { outbounds: [{ tag: "只有我", type: "direct" }] };`, "")
	assert.Equal(t, []string{"只有我"}, outboundTags(m))
}

func TestPostprocessFromScriptURL(t *testing.T) {
	m := ppConvert(t, `config.log = { level: "error" };`, "https://example.com/pp.js")
	assert.Equal(t, map[string]any{"level": "error"}, m["log"])
}

func TestPostprocessRenameNodes(t *testing.T) {
	m := ppConvert(t, `
for (const o of config.outbounds) {
  if (typeof o.tag === "string" && /^(hk|jp)-/.test(o.tag)) o.tag = o.tag.toUpperCase();
}
`, "")
	tags := outboundTags(m)
	assert.Contains(t, tags, "HK-01")
	assert.Contains(t, tags, "JP-01")
}

func TestPostprocessScriptError(t *testing.T) {
	client := &http.Client{Transport: staticRT{body: []byte(ppSubYAML)}}
	c := NewConvert(client, newSilentLogger())
	arg := model.ConvertArg{
		Sub:    "https://example.com/sub",
		Config: []byte(`{"outbounds":[]}`),
		Ver:    cmodel.SINGLATEST,
		Script: []byte(`throw new Error("脚本炸了")`),
	}
	_, err := c.MakeConfig(context.Background(), arg, nil, "sing-box/2.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "脚本炸了")
}

func TestPostprocessSyntaxError(t *testing.T) {
	client := &http.Client{Transport: staticRT{body: []byte(ppSubYAML)}}
	c := NewConvert(client, newSilentLogger())
	arg := model.ConvertArg{
		Sub:    "https://example.com/sub",
		Config: []byte(`{"outbounds":[]}`),
		Ver:    cmodel.SINGLATEST,
		Script: []byte(`function (`),
	}
	_, err := c.MakeConfig(context.Background(), arg, nil, "sing-box/2.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SyntaxError")
}

func TestPostprocessWrongResultType(t *testing.T) {
	client := &http.Client{Transport: staticRT{body: []byte(ppSubYAML)}}
	c := NewConvert(client, newSilentLogger())
	arg := model.ConvertArg{
		Sub:    "https://example.com/sub",
		Config: []byte(`{"outbounds":[]}`),
		Ver:    cmodel.SINGLATEST,
		Script: []byte(`return 42`),
	}
	_, err := c.MakeConfig(context.Background(), arg, nil, "sing-box/2.0")
	require.ErrorIs(t, err, errScriptResult)
}

func TestPostprocessInfiniteLoopTimeout(t *testing.T) {
	client := &http.Client{Transport: staticRT{body: []byte(ppSubYAML)}}
	c := NewConvert(client, newSilentLogger())
	arg := model.ConvertArg{
		Sub:    "https://example.com/sub",
		Config: []byte(`{"outbounds":[]}`),
		Ver:    cmodel.SINGLATEST,
		Script: []byte(`for (;;) {}`),
	}
	_, err := c.MakeConfig(context.Background(), arg, nil, "sing-box/2.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deadline")
}

func TestPostprocessMemoryLimit(t *testing.T) {
	client := &http.Client{Transport: staticRT{body: []byte(ppSubYAML)}}
	c := NewConvert(client, newSilentLogger())
	arg := model.ConvertArg{
		Sub:    "https://example.com/sub",
		Config: []byte(`{"outbounds":[]}`),
		Ver:    cmodel.SINGLATEST,
		Script: []byte(`const a = []; for (;;) { a.push(new Array(10000).fill("x")); }`),
	}
	_, err := c.MakeConfig(context.Background(), arg, nil, "sing-box/2.0")
	require.Error(t, err)
}

func TestPostprocessScriptURLTooLarge(t *testing.T) {
	big := bytes.Repeat([]byte("// padding\n"), maxScriptBytes/11+1)
	client := &http.Client{Transport: multiRT{
		"https://example.com/sub":    []byte(ppSubYAML),
		"https://example.com/big.js": big,
	}}
	c := NewConvert(client, newSilentLogger())
	arg := model.ConvertArg{
		Sub:       "https://example.com/sub",
		Config:    []byte(`{"outbounds":[]}`),
		Ver:       cmodel.SINGLATEST,
		ScriptUrl: "https://example.com/big.js",
	}
	_, err := c.MakeConfig(context.Background(), arg, nil, "sing-box/2.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "超过")
}

func TestPostprocessScriptURLNotFound(t *testing.T) {
	client := &http.Client{Transport: multiRT{"https://example.com/sub": []byte(ppSubYAML)}}
	c := NewConvert(client, newSilentLogger())
	arg := model.ConvertArg{
		Sub:       "https://example.com/sub",
		Config:    []byte(`{"outbounds":[]}`),
		Ver:       cmodel.SINGLATEST,
		ScriptUrl: "https://example.com/missing.js",
	}
	_, err := c.MakeConfig(context.Background(), arg, nil, "sing-box/2.0")
	require.Error(t, err)
}

func TestPostprocessScriptURLPreferred(t *testing.T) {
	// 同时给出时以直链为准
	client := &http.Client{Transport: multiRT{
		"https://example.com/sub":   []byte(ppSubYAML),
		"https://example.com/pp.js": []byte(`config.log = { level: "remote" };`),
	}}
	c := NewConvert(client, newSilentLogger())
	arg := model.ConvertArg{
		Sub:       "https://example.com/sub",
		Config:    []byte(`{"outbounds":[]}`),
		Ver:       cmodel.SINGLATEST,
		Script:    []byte(`config.log = { level: "inline" };`),
		ScriptUrl: "https://example.com/pp.js",
	}
	b, err := c.MakeConfig(context.Background(), arg, nil, "sing-box/2.0")
	require.NoError(t, err)
	assert.Contains(t, string(b), "remote")
	assert.NotContains(t, string(b), "inline")
}

func TestPostprocessConsoleSilent(t *testing.T) {
	// console 可用但不写日志：脚本不报错，日志里也看不到任何内容
	var buf bytes.Buffer
	c := NewConvert(&http.Client{Transport: staticRT{body: []byte(ppSubYAML)}},
		slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	arg := model.ConvertArg{
		Sub:    "https://example.com/sub",
		Config: []byte(`{"outbounds":[]}`),
		Ver:    cmodel.SINGLATEST,
		Script: []byte(`console.log("节点数", config.outbounds.length); console.warn("提醒"); console.error("错误");`),
	}
	b, err := c.MakeConfig(context.Background(), arg, nil, "sing-box/2.0")
	require.NoError(t, err)
	assert.NotContains(t, buf.String(), "节点数")
	assert.NotContains(t, buf.String(), "提醒")
	assert.NotContains(t, buf.String(), "错误")
	assert.NotEmpty(t, b)
}

func TestPostprocessDisabledWithoutScript(t *testing.T) {
	// 未提供脚本时行为与之前完全一致
	client := &http.Client{Transport: staticRT{body: []byte(ppSubYAML)}}
	c := NewConvert(client, newSilentLogger())
	arg := model.ConvertArg{
		Sub:    "https://example.com/sub",
		Config: []byte(`{"outbounds":[]}`),
		Ver:    cmodel.SINGLATEST,
	}
	b, err := c.MakeConfig(context.Background(), arg, nil, "sing-box/2.0")
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	assert.Contains(t, outboundTags(m), "urltest")
}

func TestPostprocessWithDetourTemplate(t *testing.T) {
	// 脚本在 detour 展开与分组解析之后运行，看到的是最终形态
	c := NewConvert(&http.Client{Transport: staticRT{body: []byte(ppSubYAML)}}, newSilentLogger())
	arg := model.ConvertArg{
		Sub: "https://example.com/sub",
		Config: []byte(`{"outbounds":[
			{"type":"selector","tag":"proxy","outbounds":["include: hk"],"detour":"direct"},
			{"type":"direct","tag":"direct"}
		]}`),
		Ver:    cmodel.SINGLATEST,
		Script: []byte(`config.outbounds.push({ type: "selector", tag: "脚本追加", outbounds: ["direct"] });`),
	}
	b, err := c.MakeConfig(context.Background(), arg, nil, "sing-box/2.0")
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	assert.Contains(t, outboundTags(m), "脚本追加")
	assert.Contains(t, string(b), "hk-01")
}
