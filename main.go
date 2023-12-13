package main

import (
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/xmdhs/clash2sfa/db"
	"github.com/xmdhs/clash2sfa/handle"
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
		Timeout: 10 * time.Second,
	}
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	l := NewSlog(h)

	mux := chi.NewMux()

	mux.Use(middleware.RequestID)
	mux.Use(NewStructuredLogger(h))

	mux.Post("/put", handle.PutArg(db, l))
	mux.Get("/sub", handle.Sub(c, db, configByte, l))
	mux.With(middleware.NoCache).Get("/config", handle.Frontend(configByte, 0))
	mux.HandleFunc("/", handle.Frontend(frontendByte, 604800))

	s := http.Server{
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		Addr:              port,
		Handler:           mux,
	}
	fmt.Println(s.ListenAndServe())
}
