package handle

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmdhs/clash2sfa/service"
)

type mixFS struct{ base fs.FS }

func (m mixFS) Open(name string) (fs.File, error) {
	if name == "badread" {
		return failFile{}, nil
	}
	return m.base.Open(name)
}

type failFile struct{}

func (failFile) Read([]byte) (int, error)             { return 0, errors.New("read error") }
func (failFile) Close() error                         { return nil }
func (failFile) Stat() (fs.FileInfo, error)           { return nil, nil }
func (failFile) ReadDir(n int) ([]fs.DirEntry, error) { return nil, nil }

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func zlibEncode(b []byte) string {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	_, _ = w.Write(b)
	_ = w.Close()
	return base64.RawURLEncoding.EncodeToString(buf.Bytes())
}

func testConfigFS() fs.FS {
	return fstest.MapFS{
		"config.json.template":         &fstest.MapFile{Data: []byte(`{"outbounds":[]}`)},
		"config.json-1.11.0+.template": &fstest.MapFile{Data: []byte(`{"outbounds":[]}`)},
		"config.json-1.12.0+.template": &fstest.MapFile{Data: []byte(`{"outbounds":[]}`)},
		"myconfig":                     &fstest.MapFile{Data: []byte(`{"outbounds":[]}`)},
	}
}

func newTestHandle(t *testing.T) *Handle {
	t.Helper()
	client := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{},
			Body: io.NopCloser(bytes.NewReader([]byte(`
proxies:
  - name: n1
    type: vmess
    server: 1.2.3.4
    port: "443"
    uuid: u
`))),
		}, nil
	})}
	convert := service.NewConvert(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return NewHandle(convert, slog.New(slog.NewTextHandler(io.Discard, nil)), testConfigFS())
}

func TestFrontend(t *testing.T) {
	h := Frontend([]byte("hello"))
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/", nil))
	assert.Equal(t, "hello", rec.Body.String())
}

func TestSubSuccess(t *testing.T) {
	h := newTestHandle(t)
	req := httptest.NewRequest("GET", "/sub?sub=https://example.com/sub&config="+zlibEncode([]byte(`{"outbounds":[]}`)), nil)
	req.Header.Set("User-Agent", "sing-box 1.12.0")
	rec := httptest.NewRecorder()
	h.Sub(rec, req)
	require.Equal(t, 200, rec.Code)
	assert.Contains(t, rec.Body.String(), "n1")
}

func TestSubNoSubParam(t *testing.T) {
	h := newTestHandle(t)
	req := httptest.NewRequest("GET", "/sub", nil)
	rec := httptest.NewRecorder()
	h.Sub(rec, req)
	assert.Equal(t, 400, rec.Code)
}

func TestSubBadConfig(t *testing.T) {
	h := newTestHandle(t)
	req := httptest.NewRequest("GET", "/sub?sub=https://example.com/sub&config=not-zlib-garbage", nil)
	req.Header.Set("User-Agent", "sing-box 1.12.0")
	rec := httptest.NewRecorder()
	h.Sub(rec, req)
	assert.Equal(t, 500, rec.Code)
}

func TestSubConfigFromFS(t *testing.T) {
	// configurl 指向 configFs 内的文件
	h := newTestHandle(t)
	req := httptest.NewRequest("GET", "/sub?sub=https://example.com/sub&configurl=myconfig", nil)
	req.Header.Set("User-Agent", "sing-box 1.12.0")
	rec := httptest.NewRecorder()
	h.Sub(rec, req)
	require.Equal(t, 200, rec.Code)
	assert.Contains(t, rec.Body.String(), "n1")
}

func TestSubWithAddTagAndDisableUrlTest(t *testing.T) {
	h := newTestHandle(t)
	req := httptest.NewRequest("GET", "/sub?sub=https://example.com/sub&config="+zlibEncode([]byte(`{"outbounds":[]}`))+"&addTag=true&disableUrlTest=true", nil)
	req.Header.Set("User-Agent", "sing-box 1.12.0")
	rec := httptest.NewRecorder()
	h.Sub(rec, req)
	require.Equal(t, 200, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "n1[example.com]") // addTag 生效
	assert.NotContains(t, body, "urltest")      // disableUrlTest 生效，不生成 urltest
}

func TestSubOutFields(t *testing.T) {
	// UA 为新版 → 默认关闭 outFields；显式 outFields=1 时生成 dns-out / block
	h := newTestHandle(t)
	req := httptest.NewRequest("GET", "/sub?sub=https://example.com/sub&config="+zlibEncode([]byte(`{"outbounds":[]}`))+"&outFields=1", nil)
	req.Header.Set("User-Agent", "sing-box 10.0.0")
	rec := httptest.NewRecorder()
	h.Sub(rec, req)
	require.Equal(t, 200, rec.Code)
	assert.Contains(t, rec.Body.String(), `"dns-out"`)
	assert.Contains(t, rec.Body.String(), `"block"`)

	// outFields=0 时不含 dns-out/block
	req2 := httptest.NewRequest("GET", "/sub?sub=https://example.com/sub&config="+zlibEncode([]byte(`{"outbounds":[]}`))+"&outFields=0", nil)
	req2.Header.Set("User-Agent", "sing-box 10.0.0")
	rec2 := httptest.NewRecorder()
	h.Sub(rec2, req2)
	require.Equal(t, 200, rec2.Code)
	assert.NotContains(t, rec2.Body.String(), `"dns-out"`)
}

func TestSubConfigUrlReadError(t *testing.T) {
	// Open 成功但 Read 失败
	convert := service.NewConvert(&http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader([]byte(`
proxies:
  - name: n1
    type: vmess
    server: 1.2.3.4
    port: "443"
    uuid: u
`)))}, nil
	})}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h := NewHandle(convert, slog.New(slog.NewTextHandler(io.Discard, nil)), mixFS{base: testConfigFS()})
	req := httptest.NewRequest("GET", "/sub?sub=https://example.com/sub&configurl=badread", nil)
	req.Header.Set("User-Agent", "sing-box 1.12.0")
	rec := httptest.NewRecorder()
	h.Sub(rec, req)
	require.Equal(t, 400, rec.Code)
}

func TestSubConfigUrlMissing(t *testing.T) {
	h := newTestHandle(t)
	req := httptest.NewRequest("GET", "/sub?sub=https://example.com/sub&configurl=nofile", nil)
	req.Header.Set("User-Agent", "sing-box 1.12.0")
	rec := httptest.NewRecorder()
	h.Sub(rec, req)
	require.Equal(t, 400, rec.Code)
}

func TestZlibDecode(t *testing.T) {
	orig := []byte(`{"outbounds":[]}`)
	got, err := zlibDecode(zlibEncode(orig))
	require.NoError(t, err)
	assert.Equal(t, orig, got)

	_, err = zlibDecode("not-valid")
	assert.Error(t, err)
	// 合法 base64 但非 zlib 流 → NewReader 失败
	_, err = zlibDecode(base64.RawURLEncoding.EncodeToString([]byte("not-zlib-data")))
	assert.Error(t, err)
	// 合法 zlib 头但数据损坏 → ReadAll 失败
	_, err = zlibDecode(base64.RawURLEncoding.EncodeToString([]byte{0x78, 0x9c, 0xff}))
	assert.Error(t, err)
}
