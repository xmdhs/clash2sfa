package main

import (
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"log/slog"

	"github.com/samber/lo"
	handler "github.com/xmdhs/clash2sfa/api"
)

func main() {
	port := ":8080"
	if p := os.Getenv("port"); p != "" {
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

	handler, _ := lo.Must2(handler.InitializeServer(h))

	s := http.Server{
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		Addr:              port,
		Handler:           handler,
	}
	fmt.Println(s.ListenAndServe())
}
