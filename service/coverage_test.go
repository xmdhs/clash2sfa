package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmdhs/clash2sfa/model"
	cmodel "github.com/xmdhs/clash2singbox/model"
)

// mapRT 按完整 URL 返回不同 body，用于区分 ConfigUrl 与 Sub
type mapRT struct {
	m map[string][]byte
}

func (s mapRT) RoundTrip(req *http.Request) (*http.Response, error) {
	body, ok := s.m[req.URL.String()]
	if !ok {
		return &http.Response{StatusCode: 404, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(nil))}, nil
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(body))}, nil
}

const subYAML = `
proxies:
  - name: n1
    type: vmess
    server: 1.2.3.4
    port: "443"
    uuid: u
`

func TestMakeConfigUsesConfigByte(t *testing.T) {
	client := &http.Client{Transport: mapRT{m: map[string][]byte{
		"https://example.com/sub": []byte(subYAML),
	}}}
	c := NewConvert(client, newSilentLogger())
	arg := model.ConvertArg{Sub: "https://example.com/sub", Ver: cmodel.SINGLATEST}
	b, err := c.MakeConfig(context.Background(), arg, []byte(`{"outbounds":[]}`), "sing-box/2.0")
	require.NoError(t, err)
	assert.Contains(t, string(b), "n1")
}

func TestMakeConfigFromConfigUrl(t *testing.T) {
	client := &http.Client{Transport: mapRT{m: map[string][]byte{
		"https://example.com/cfg": []byte(`{"outbounds":[]}`),
		"https://example.com/sub": []byte(subYAML),
	}}}
	c := NewConvert(client, newSilentLogger())
	arg := model.ConvertArg{
		Sub:       "https://example.com/sub",
		ConfigUrl: "https://example.com/cfg",
		Ver:       cmodel.SINGLATEST,
	}
	b, err := c.MakeConfig(context.Background(), arg, nil, "sing-box/2.0")
	require.NoError(t, err)
	assert.Contains(t, string(b), "n1")
}

func TestMakeConfigInvalidTemplate(t *testing.T) {
	client := &http.Client{Transport: mapRT{m: map[string][]byte{
		"https://example.com/sub": []byte(subYAML),
	}}}
	c := NewConvert(client, newSilentLogger())
	arg := model.ConvertArg{
		Sub:    "https://example.com/sub",
		Config: []byte(`not-json{`),
		Ver:    cmodel.SINGLATEST,
	}
	_, err := c.MakeConfig(context.Background(), arg, nil, "sing-box/2.0")
	assert.Error(t, err)
}

func TestConfigUrlTestParserDetour(t *testing.T) {
	config := map[string]any{
		"outbounds": []any{
			map[string]any{
				"type": "selector", "tag": "proxy",
				"outbounds": []any{"include: HK"}, "detour": "wrapper",
			},
		},
	}
	tags := []TagWithVisible{{Tag: "HK-01", Visible: []string{"proxy"}}, {Tag: "JP-01"}}
	res, err := configUrlTestParser(config, tags)
	require.NoError(t, err)

	out := res["outbounds"].([]any)
	sel := out[0].(map[string]any)
	// detour 被删除
	_, hasDetour := sel["detour"]
	assert.False(t, hasDetour)
	assert.Equal(t, []string{"HK-01"}, sel["outbounds"].([]string))
}

func TestConfigUrlTestParserNoDirective(t *testing.T) {
	config := map[string]any{
		"outbounds": []any{
			map[string]any{"type": "selector", "tag": "proxy", "outbounds": []any{"direct"}},
		},
	}
	res, err := configUrlTestParser(config, []TagWithVisible{{Tag: "HK-01"}})
	require.NoError(t, err)
	out := res["outbounds"].([]any)
	sel := out[0].(map[string]any)
	// 无 include/exclude 指令 → 原样保留
	assert.Equal(t, []any{"direct"}, sel["outbounds"])
}

func TestConfigUrlTestParserBadRegex(t *testing.T) {
	config := map[string]any{
		"outbounds": []any{
			map[string]any{"tag": "proxy", "outbounds": []any{"include: ["}},
		},
	}
	_, err := configUrlTestParser(config, nil)
	assert.Error(t, err)
}

func TestUrlTestParserInvalidRegex(t *testing.T) {
	_, err := urlTestParser([]string{"include: ["}, []string{"HK-01"})
	assert.Error(t, err)
}
