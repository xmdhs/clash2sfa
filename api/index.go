package handler

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"text/template"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/samber/lo"
	"github.com/xmdhs/clash2sfa/handle"
	"github.com/xmdhs/clash2sfa/service"
)

//go:embed config.json.template
var ConfigByte []byte

//go:embed frontend.html
var FrontendByte []byte

func SetMux(h slog.Handler) *chi.Mux {
	c := &http.Client{
		Timeout: 60 * time.Second,
	}
	l := NewSlog(h)

	convert := service.NewConvert(c, ConfigByte, l)
	subH := handle.NewHandle(convert, l)

	mux := chi.NewMux()

	mux.Use(middleware.RequestID)
	mux.Use(middleware.RealIP)
	mux.Use(NewStructuredLogger(h))

	mux.Get("/sub", subH.Sub)
	mux.With(middleware.NoCache).Get("/config", handle.Frontend(ConfigByte, 0))

	buildInfo, _ := debug.ReadBuildInfo()
	var hash string
	for _, v := range buildInfo.Settings {
		if v.Key == "vcs.revision" {
			hash = v.Value
		}
	}
	bw := &bytes.Buffer{}
	lo.Must(template.New("index").Delims("[[", "]]").Parse(string(FrontendByte))).ExecuteTemplate(bw, "index", []string{buildInfo.Main.Path, hash})
	mux.HandleFunc("/", handle.Frontend(bw.Bytes(), 0))

	return mux
}

func Handler(w http.ResponseWriter, r *http.Request) {
	level := &slog.LevelVar{}
	level.Set(slog.Level(-4))
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	SetMux(h).ServeHTTP(w, r)
}

func NewStructuredLogger(handler slog.Handler) func(next http.Handler) http.Handler {
	return middleware.RequestLogger(&StructuredLogger{Logger: handler})
}

type StructuredLogger struct {
	Logger slog.Handler
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

	logger := NewSlog(l.Logger)

	logger.LogAttrs(ctx, slog.LevelDebug, "request started", logFields...)

	entry := StructuredLoggerEntry{Logger: logger, ctx: ctx}

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

func NewSlog(h slog.Handler) *slog.Logger {
	l := slog.New(&warpSlogHandle{
		Handler: h,
	})
	return l
}
