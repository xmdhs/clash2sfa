package provide

import (
	"bytes"
	"context"
	"crypto/tls"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"text/template"
	"time"

	"filippo.io/intermediates"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/samber/lo"
	"github.com/xmdhs/clash2sfa/handle"
	"github.com/xmdhs/clash2sfa/service"
)

//go:embed static
var static embed.FS

//go:embed frontend.html
var FrontendByte []byte

type html struct {
	Path string
	Hash string
}

var info html

func init() {
	buildInfo, ok := debug.ReadBuildInfo()
	var hash string
	if ok {
		hash = vcsRevision(buildInfo.Settings)
	}
	info = html{
		Path: buildInfo.Main.Path,
		Hash: hash,
	}
	if hash == "" {
		info.Hash = os.Getenv("VERCEL_GIT_COMMIT_SHA")
	}
}

func vcsRevision(settings []debug.BuildSetting) string {
	for _, v := range settings {
		if v.Key == "vcs.revision" {
			return v.Value
		}
	}
	return ""
}

func NewApp(h slog.Handler) http.Handler {
	client := newClient()
	logger := newSlog(h)
	return newMux(client, logger)
}

func newClient() *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
		VerifyConnection:   intermediates.VerifyConnection,
	}
	return &http.Client{
		Transport: tr,
		Timeout:   60 * time.Second,
	}
}

func newSlog(h slog.Handler) *slog.Logger {
	l := slog.New(&warpSlogHandle{
		Handler: h,
	})
	return l
}

func newMux(c *http.Client, l *slog.Logger) *chi.Mux {
	staticFS := lo.Must(fs.Sub(static, "static"))
	convert := service.NewConvert(c, l)
	subH := handle.NewHandle(convert, l, staticFS)

	mux := chi.NewMux()

	mux.Use(middleware.RequestID)
	mux.Use(middleware.RealIP)
	mux.Use(newStructuredLogger(l))

	mux.Get("/sub", subH.Sub)

	mux.With(Cache).Mount("/config", http.StripPrefix("/config", http.FileServerFS(staticFS)))
	mux.With(Cache).Mount("/static", http.StripPrefix("/static", http.FileServerFS(staticFS)))

	bw := &bytes.Buffer{}
	lo.Must(template.New("index").Delims("[[", "]]").Parse(string(FrontendByte))).ExecuteTemplate(bw, "index", info)
	mux.With(Cache).HandleFunc("/", handle.Frontend(bw.Bytes()))

	return mux
}

func newStructuredLogger(Logger *slog.Logger) func(next http.Handler) http.Handler {
	return middleware.RequestLogger(&StructuredLogger{Logger: Logger})
}

type StructuredLogger struct {
	Logger *slog.Logger
}

func (l *StructuredLogger) NewLogEntry(r *http.Request) middleware.LogEntry {
	var logFields []slog.Attr
	ctx := r.Context()

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	logFields = append(logFields,
		slog.String("http_method", r.Method),
		slog.String("remote_addr", r.RemoteAddr),
		slog.String("user_agent", r.UserAgent()),
		slog.String("uri", fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)))

	l.Logger.LogAttrs(ctx, slog.LevelDebug, "request started", logFields...)
	entry := StructuredLoggerEntry{Logger: l.Logger, ctx: ctx}

	return &entry
}

type StructuredLoggerEntry struct {
	Logger *slog.Logger
	ctx    context.Context
}

func (l *StructuredLoggerEntry) Write(status, bytes int, header http.Header, elapsed time.Duration, extra interface{}) {
	l.Logger.LogAttrs(l.ctx, slog.LevelDebug, "request complete",
		slog.Int("resp_status", status),
		slog.Int("resp_byte_length", bytes),
		slog.Float64("resp_elapsed_ms", float64(elapsed.Nanoseconds())/1000000.0),
	)
}

func (l *StructuredLoggerEntry) Panic(v interface{}, stack []byte) {
	l.Logger.LogAttrs(l.ctx, slog.LevelDebug, "",
		slog.String("stack", string(stack)),
		slog.String("panic", fmt.Sprintf("%+v", v)),
	)
}

type warpSlogHandle struct {
	slog.Handler
}

func (w *warpSlogHandle) Handle(ctx context.Context, r slog.Record) error {
	id := middleware.GetReqID(ctx)
	if id != "" {
		r.AddAttrs(slog.String("req_id", id))
	}
	return w.Handler.Handle(ctx, r)
}

func Cache(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=43200, s-maxage=43200")
		h.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
