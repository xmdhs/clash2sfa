package provide

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"runtime/debug"
	"testing"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return newSlog(slog.NewTextHandler(discard{}, nil))
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

func TestNewApp(t *testing.T) {
	handler := NewApp(slog.NewTextHandler(discard{}, nil))
	require.NotNil(t, handler)
}

func TestNewClient(t *testing.T) {
	c := newClient()
	require.NotNil(t, c)
	require.NotNil(t, c.Transport)
	assert.Equal(t, 60*time.Second, c.Timeout)
}

func TestMuxRoutes(t *testing.T) {
	h := handlerForTest(t)

	// 首页
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	assert.Equal(t, 200, rec.Code)
	assert.NotEmpty(t, rec.Body.String())

	// 静态资源
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/static/base64.min.mjs", nil))
	assert.Equal(t, 200, rec.Code)
	assert.NotEmpty(t, rec.Body.String())

	// config 文件服务器
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/config/config.json.template", nil))
	assert.Equal(t, 200, rec.Code)
}

func TestCacheHeader(t *testing.T) {
	h := Cache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	assert.Equal(t, "public, max-age=43200, s-maxage=43200", rec.Header().Get("Cache-Control"))
}

func TestStructuredLogger(t *testing.T) {
	l := &StructuredLogger{Logger: testLogger()}
	req := httptest.NewRequest("GET", "http://example.com/path", nil)
	entry := l.NewLogEntry(req)
	require.NotNil(t, entry)
	entry.Write(200, 10, http.Header{}, 5*time.Millisecond, nil)
	entry.Panic("boom", []byte("stack"))
	entry.(*StructuredLoggerEntry).Logger.LogAttrs(req.Context(), slog.LevelDebug, "x")

	// TLS 分支：scheme 使用 https
	reqTLS := httptest.NewRequest("GET", "https://example.com/path", nil)
	reqTLS.TLS = &tls.ConnectionState{}
	require.NotNil(t, l.NewLogEntry(reqTLS))
}

func TestWarpSlogHandleWithReqID(t *testing.T) {
	base := slog.NewTextHandler(discard{}, nil)
	w := &warpSlogHandle{Handler: base}
	ctx := context.WithValue(context.Background(), middleware.RequestIDKey, "req-123")
	require.NoError(t, w.Handle(ctx, slog.NewRecord(time.Now(), slog.LevelDebug, "msg", 0)))
	// 无 req id 时也能写入
	require.NoError(t, w.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelDebug, "msg", 0)))
}

func TestNewStructuredLogger(t *testing.T) {
	mw := newStructuredLogger(testLogger())
	require.NotNil(t, mw)
}

func TestVCSRevision(t *testing.T) {
	assert.Equal(t, "abc123", vcsRevision([]debug.BuildSetting{{Key: "vcs.revision", Value: "abc123"}, {Key: "other", Value: "x"}}))
	assert.Equal(t, "", vcsRevision([]debug.BuildSetting{{Key: "other", Value: "x"}}))
}

func handlerForTest(t *testing.T) http.Handler {
	t.Helper()
	return NewApp(slog.NewTextHandler(discard{}, nil))
}
