package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"time"

	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/samber/lo"
	"github.com/xmdhs/clash2sfa/db"
	"github.com/xmdhs/clash2sfa/handle"
	"github.com/xmdhs/clash2sfa/service"
)

//go:embed config.json.template
var configByte []byte

//go:embed frontend.html
var frontendByte []byte

func main() {
	port := ":8080"
	if p := os.Getenv("port"); p != "" {
		port = p
	}

	db, err := db.NewBBolt("bbolt.db")
	if err != nil {
		panic(err)
	}

	levels := os.Getenv("level")
	leveln, err := strconv.Atoi(levels)
	if err != nil {
		leveln = -4
	}

	level := &slog.LevelVar{}
	level.Set(slog.Level(leveln))

	c := &http.Client{
		Timeout: 60 * time.Second,
	}
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	l := NewSlog(h)

	convert := service.NewConvert(c, db, configByte, l)
	subH := handle.NewHandle(convert, l)

	mux := chi.NewMux()

	mux.Use(middleware.RequestID)
	mux.Use(middleware.RealIP)
	mux.Use(NewStructuredLogger(h))

	mux.Put("/put", subH.PutArg)
	mux.Get("/sub", subH.Sub)
	mux.With(middleware.NoCache).Get("/config", handle.Frontend(configByte, 0))

	buildInfo, _ := debug.ReadBuildInfo()
	var hash string
	for _, v := range buildInfo.Settings {
		if v.Key == "vcs.revision" {
			hash = v.Value
		}
	}
	bw := &bytes.Buffer{}
	lo.Must(template.New("index").Delims("[[", "]]").Parse(string(frontendByte))).ExecuteTemplate(bw, "index", []string{buildInfo.Main.Path, hash})

	mux.HandleFunc("/", handle.Frontend(bw.Bytes(), 0))

	s := http.Server{
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		Addr:              port,
		Handler:           mux,
	}
	fmt.Println(s.ListenAndServe())
}
