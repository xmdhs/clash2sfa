package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"log/slog"

	"github.com/go-chi/chi/v5/middleware"
)

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
