package main

import (
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"log/slog"

	"github.com/samber/do/v2"
	"github.com/xmdhs/clash2sfa/provide"
)

func main() {
	port := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	levels := os.Getenv("level")
	leveln, err := strconv.Atoi(levels)
	if err != nil {
		leveln = -4
	}

	level := &slog.LevelVar{}
	level.Set(slog.Level(leveln))
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})

	injector := do.New()
	provide.RegisterProviders(injector, h)
	handler := do.MustInvoke[http.Handler](injector)

	s := http.Server{
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		Addr:              port,
		Handler:           handler,
	}
	fmt.Println(s.ListenAndServe())
}
