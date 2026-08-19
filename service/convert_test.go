package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmdhs/clash2sfa/model"
	cmodel "github.com/xmdhs/clash2singbox/model"
	"github.com/xmdhs/clash2singbox/model/singbox"
)

func TestGetExtTag(t *testing.T) {
	nodes, err := getExtTag([]byte(`{
  "outbounds":[
    {"type":"vmess","tag":"a"},
    {"type":"direct","tag":"direct"},
    {"type":"block","tag":"block"},
    {"type":"dns","tag":"dns-out"},
    {"type":"selector","tag":"sel"}
  ]
}`))
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	assert.Equal(t, "a", nodes[0].tag)
	assert.Equal(t, "vmess", nodes[0].nodeType)
	assert.Equal(t, "sel", nodes[1].tag)
	assert.Equal(t, "selector", nodes[1].nodeType)
}

func TestGetExtTagInvalidJSON(t *testing.T) {
	_, err := getExtTag([]byte(`{`))
	assert.ErrorIs(t, err, ErrFormat)
}

func TestGetExtTagMissingOutbounds(t *testing.T) {
	_, err := getExtTag([]byte(`{}`))
	assert.ErrorIs(t, err, ErrFormat)
}

func TestSingDetourList(t *testing.T) {
	singMap := map[string]singbox.SingBoxOut{
		"a": {Tag: "a", Detour: "b"},
		"b": {Tag: "b", Detour: ""},
		"c": {Tag: "c", Detour: "a"},
	}
	tags, out := singDetourList("c", singMap)
	assert.Equal(t, []string{"c", "a", "b"}, tags)
	assert.Len(t, out, 3)
	assert.Equal(t, "b", out[2].Tag)

	// 缺失节点
	tags, out = singDetourList("nope", singMap)
	assert.Empty(t, tags)
	assert.Empty(t, out)
}

func TestSingDetourListCycle(t *testing.T) {
	singMap := map[string]singbox.SingBoxOut{
		"x": {Tag: "x", Detour: "y"},
		"y": {Tag: "y", Detour: "x"},
	}
	tags, _ := singDetourList("x", singMap)
	assert.Equal(t, []string{"x", "y"}, tags)
}

func TestAnyDetourList(t *testing.T) {
	anyMap := map[string]map[string]any{
		"w1": {"tag": "w1", "detour": "w2"},
		"w2": {"tag": "w2", "detour": ""},
	}
	tags, out := anyDetourList("w1", anyMap)
	assert.Equal(t, []string{"w1", "w2"}, tags)
	assert.Len(t, out, 2)

	tags, out = anyDetourList("missing", anyMap)
	assert.Empty(t, tags)
	assert.Empty(t, out)
}

func TestAnyDetourListCycle(t *testing.T) {
	anyMap := map[string]map[string]any{
		"x": {"tag": "x", "detour": "y"},
		"y": {"tag": "y", "detour": "x"},
	}
	tags, _ := anyDetourList("x", anyMap)
	assert.Equal(t, []string{"x", "y"}, tags)
}

func TestUrlTestDetourSet(t *testing.T) {
	s := []singbox.SingBoxOut{{Type: "vmess", Tag: "B"}}
	outs := []map[string]any{
		{"type": "http", "tag": "wrapper", "server": "example.com", "detour": ""},
	}
	config := []byte(`{"outbounds":[{"type":"selector","tag":"proxy","outbounds":["B"],"detour":"wrapper"}]}`)

	_, newOuts, extTags := urlTestDetourSet(s, config, outs, nil)
	require.Len(t, newOuts, 2)
	assert.Equal(t, "wrapper", newOuts[0]["tag"])

	chained := newOuts[1]
	assert.Equal(t, "B - wrapper [proxy]", chained["tag"])
	assert.Equal(t, "B", chained["detour"])

	require.Len(t, extTags, 1)
	assert.Equal(t, "B - wrapper [proxy]", extTags[0].Tag)
	assert.Equal(t, []string{"proxy"}, extTags[0].Visible)
}

func TestUrlTestDetourSetNoDetourReturnsInput(t *testing.T) {
	s := []singbox.SingBoxOut{{Type: "vmess", Tag: "B"}}
	outs := []map[string]any{{"type": "http", "tag": "wrapper"}}
	config := []byte(`{"outbounds":[{"type":"selector","tag":"proxy","outbounds":["B"]}]}`)

	newS, newOuts, extTags := urlTestDetourSet(s, config, outs, []string{"ext"})
	assert.Equal(t, s, newS)
	assert.Equal(t, outs, newOuts)
	// 传入的 extTag 被包装为无 Visible 的 TagWithVisible
	require.Len(t, extTags, 1)
	assert.Equal(t, "ext", extTags[0].Tag)
	assert.Nil(t, extTags[0].Visible)
}

func TestConfigUrlTestParser(t *testing.T) {
	config := map[string]any{
		"outbounds": []any{
			map[string]any{"type": "selector", "tag": "proxy", "outbounds": []any{"direct", "include: HK"}},
		},
	}
	tags := []TagWithVisible{{Tag: "HK-01"}, {Tag: "JP-01"}}
	res, err := configUrlTestParser(config, tags)
	require.NoError(t, err)

	out := res["outbounds"].([]any)
	sel := out[0].(map[string]any)
	got := sel["outbounds"].([]string)
	assert.Equal(t, []string{"direct", "HK-01"}, got)
}

func TestConfigUrlTestParserBadOutbounds(t *testing.T) {
	_, err := configUrlTestParser(map[string]any{}, nil)
	assert.Error(t, err)
}

// --- 全链路：用静态 RoundTripper 模拟订阅抓取（不触发真实网络） ---

type staticRT struct {
	body []byte
}

func (s staticRT) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       io.NopCloser(bytes.NewReader(s.body)),
	}, nil
}

func newSilentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestMakeConfigFullChain(t *testing.T) {
	// 订阅返回一个 clash YAML 节点
	client := &http.Client{Transport: staticRT{body: []byte(`
proxies:
  - name: hk-node
    type: vmess
    server: 1.2.3.4
    port: "443"
    uuid: uuid
`)}}
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
	out := m["outbounds"].([]any)
	require.Len(t, out, 4)

	tags := map[string]bool{}
	for _, item := range out {
		tags[item.(map[string]any)["tag"].(string)] = true
	}
	assert.True(t, tags["hk-node"])
	assert.True(t, tags["direct"])
	assert.True(t, tags["select"])
	assert.True(t, tags["urltest"])
}

func TestMakeConfigBrowserFormatting(t *testing.T) {
	client := &http.Client{Transport: staticRT{body: []byte(`
proxies:
  - name: n1
    type: ss
    server: example.com
    port: "8388"
    cipher: aes-256-gcm
    password: pass
`)}}
	c := NewConvert(client, newSilentLogger())
	arg := model.ConvertArg{
		Sub:    "https://example.com/sub",
		Config: []byte(`{"outbounds":[]}`),
		Ver:    cmodel.SINGLATEST,
	}
	b, err := c.MakeConfig(context.Background(), arg, nil, "Mozilla/5.0 (Windows NT 10.0) AppleWebKit/537.36")
	require.NoError(t, err)
	// 浏览器请求应返回格式化（多行）JSON
	assert.Contains(t, string(b), "\n")
}

func TestMakeConfigSupportsJSONC(t *testing.T) {
	client := &http.Client{Transport: staticRT{body: []byte(`
proxies:
  - name: n1
    type: ss
    server: example.com
    port: "8388"
    cipher: aes-256-gcm
    password: pass
`)}}
	c := NewConvert(client, newSilentLogger())
	// 注释的 JSONC 配置
	arg := model.ConvertArg{
		Sub:    "https://example.com/sub",
		Config: []byte("{\n  // template with comments\n  \"outbounds\": []\n}"),
		Ver:    cmodel.SINGLATEST,
	}
	b, err := c.MakeConfig(context.Background(), arg, nil, "sing-box/2.0")
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	require.NotNil(t, m["outbounds"])
}
