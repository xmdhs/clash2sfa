package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmdhs/clash2sfa/model"
	cmodel "github.com/xmdhs/clash2singbox/model"
	"github.com/xmdhs/clash2singbox/model/singbox"
)

// --- urlTestDetourSet 的 sing 链与 any 链全分支 ---

func TestUrlTestDetourSetSingChain(t *testing.T) {
	s := []singbox.SingBoxOut{
		{Type: "vmess", Tag: "B"},
		{Type: "vmess", Tag: "hidden", Ignored: true},
		{Type: "vmess", Tag: "n1", Detour: "n2"},
		{Type: "vmess", Tag: "n2"},
	}
	outs := []map[string]any{
		{"type": "selector", "tag": "sel"},
		{"type": "urltest", "tag": "ut"},
	}
	config := []byte(`{"outbounds":[{"type":"selector","tag":"proxy","outbounds":["B"],"detour":"n1"}]}`)

	newS, newOuts, extTags := urlTestDetourSet(s, config, outs, nil)
	require.Len(t, newS, 6)
	// 忽略节点不参与，ext 的 selector/urltest 不进入 allTags；链尾与链首都生成
	tailNode, headNode := newS[4], newS[5]
	assert.Equal(t, "B - n2 [proxy]", tailNode.Tag)
	assert.Equal(t, "B", tailNode.Detour)
	assert.Equal(t, []string{"_hide"}, tailNode.Visible)

	assert.Equal(t, "B - n1 [proxy]", headNode.Tag)
	assert.Equal(t, "B - n2 [proxy]", headNode.Detour)
	assert.Equal(t, []string{"proxy"}, headNode.Visible)

	// sing 链的节点经由 newSingOut 返回，extTags 仅承载 any 链；outs 原样保留
	assert.Empty(t, extTags)
	assert.Equal(t, outs, newOuts)
}

func TestUrlTestDetourSetAnyChain(t *testing.T) {
	s := []singbox.SingBoxOut{{Type: "vmess", Tag: "B"}}
	outs := []map[string]any{
		{"type": "http", "tag": "w1", "detour": "w2"},
		{"type": "http", "tag": "w2", "detour": ""},
		{"type": "selector", "tag": "sel"},
	}
	config := []byte(`{"outbounds":[{"type":"selector","tag":"proxy","outbounds":["B"],"detour":"w1"}]}`)

	_, newOuts, extTags := urlTestDetourSet(s, config, outs, nil)
	require.Len(t, newOuts, len(outs)+2)
	tail := newOuts[len(newOuts)-1]
	assert.Equal(t, "B - w1 [proxy]", tail["tag"])
	assert.Equal(t, "B - w2 [proxy]", tail["detour"])

	require.Len(t, extTags, 2)
	assert.Equal(t, []string{"_hide"}, extTags[0].Visible)
	assert.Equal(t, []string{"proxy"}, extTags[1].Visible)
}

// --- convert2sing / MakeConfig 的分支覆盖 ---

func TestMakeConfigSubFetchError(t *testing.T) {
	c := NewConvert(&http.Client{}, newSilentLogger())
	arg := model.ConvertArg{
		Sub:    "http://exa mple.com/sub",
		Config: []byte(`{"outbounds":[]}`),
		Ver:    cmodel.SINGLATEST,
	}
	_, err := c.MakeConfig(context.Background(), arg, nil, "sing-box/2.0")
	assert.Error(t, err)
}

func TestMakeConfigConfigUrlFetchError(t *testing.T) {
	client := &http.Client{Transport: mapRT{m: map[string][]byte{
		// 未注册的 URL 返回 404 → HttpGet 报错
	}}}
	c := NewConvert(client, newSilentLogger())
	arg := model.ConvertArg{
		Sub:       "https://example.com/sub",
		ConfigUrl: "https://example.com/missing",
		Ver:       cmodel.SINGLATEST,
	}
	_, err := c.MakeConfig(context.Background(), arg, nil, "sing-box/2.0")
	assert.Error(t, err)
}

func TestMakeConfigGetExtTagError(t *testing.T) {
	client := &http.Client{Transport: mapRT{m: map[string][]byte{
		"https://example.com/sub": []byte(subYAML),
	}}}
	c := NewConvert(client, newSilentLogger())
	arg := model.ConvertArg{
		Sub:    "https://example.com/sub",
		Config: []byte(`{}`), // 无 outbounds → getExtTag 报错
		Ver:    cmodel.SINGLATEST,
	}
	_, err := c.MakeConfig(context.Background(), arg, nil, "sing-box/2.0")
	assert.Error(t, err)
}

func TestMakeConfigRichTemplate(t *testing.T) {
	// 模板含外部节点，触发 convert2sing 的 nodes 循环与 lo.Map(outs)
	client := &http.Client{Transport: mapRT{m: map[string][]byte{
		"https://example.com/sub": []byte(subYAML),
	}}}
	c := NewConvert(client, newSilentLogger())
	tpl := `{"outbounds":[
		{"type":"vmess","tag":"ext1","server":"external"},
		{"type":"selector","tag":"sel","outbounds":["ext1"]},
		{"type":"urltest","tag":"ut","outbounds":["ext1"]}
	]}`
	arg := model.ConvertArg{Sub: "https://example.com/sub", Config: []byte(tpl), Ver: cmodel.SINGLATEST}
	b, err := c.MakeConfig(context.Background(), arg, nil, "sing-box/2.0")
	require.NoError(t, err)
	assert.Contains(t, string(b), "ext1")
}

func TestMakeConfigClash2singErrorLogged(t *testing.T) {
	// 订阅含转换失败的节点，错误被记日志而不中断
	bad := `
proxies:
  - name: bwg
    type: wireguard
    server: example.com
    port: "51820"
    private-key: p
    ip: not-an-ip
`
	client := &http.Client{Transport: mapRT{m: map[string][]byte{
		"https://example.com/sub": []byte(bad),
	}}}
	c := NewConvert(client, newSilentLogger())
	arg := model.ConvertArg{Sub: "https://example.com/sub", Config: []byte(`{"outbounds":[]}`), Ver: cmodel.SINGLATEST}
	b, err := c.MakeConfig(context.Background(), arg, nil, "sing-box/2.0")
	require.NoError(t, err)
	assert.NotEmpty(t, b)
}

func TestMakeConfigIncludeRegexError(t *testing.T) {
	client := &http.Client{Transport: mapRT{m: map[string][]byte{
		"https://example.com/sub": []byte(subYAML),
	}}}
	c := NewConvert(client, newSilentLogger())
	arg := model.ConvertArg{
		Sub:     "https://example.com/sub",
		Config:  []byte(`{"outbounds":[]}`),
		Include: "[",
		Ver:     cmodel.SINGLATEST,
	}
	_, err := c.MakeConfig(context.Background(), arg, nil, "sing-box/2.0")
	assert.Error(t, err)
}

const shadowTLSYAML = `
proxies:
  - name: sx
    type: ss
    server: example.com
    port: "443"
    cipher: aes-256-gcm
    password: pass
    plugin: shadow-tls
    plugin-opts:
      host: stls.example.com
      password: sp
      version: 3
`

func TestMakeConfigShadowTLSIgnoredNode(t *testing.T) {
	// shadow-tls 拆出的 shadowtls outbound 为 Ignored，nodeTag 循环应跳过它
	client := &http.Client{Transport: mapRT{m: map[string][]byte{
		"https://example.com/sub": []byte(shadowTLSYAML),
	}}}
	c := NewConvert(client, newSilentLogger())
	arg := model.ConvertArg{Sub: "https://example.com/sub", Config: []byte(`{"outbounds":[]}`), Ver: cmodel.SINGLATEST}
	b, err := c.MakeConfig(context.Background(), arg, nil, "sing-box/2.0")
	require.NoError(t, err)
	assert.Contains(t, string(b), "sx")
}
