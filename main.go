package main

import (
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"log/slog"

	"github.com/xmdhs/clash2sfa/provide"
)

func main() {
	fmt.Println(newServer(parsePort(), parseLevel()).ListenAndServe())
}

func newServer(port string, leveln int) *http.Server {
	level := &slog.LevelVar{}
	level.Set(slog.Level(leveln))
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})

	return &http.Server{
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		Addr:              port,
		Handler:           provide.NewApp(h),
	}
}

func parsePort() string {
	port := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		port = ":" + p
	}
	if p := os.Getenv("port"); p != "" {
		port = p
	}
	return port
}

func parseLevel() int {
	levels := os.Getenv("level")
	leveln, err := strconv.Atoi(levels)
	if err != nil {
		leveln = -4
	}
	return leveln
}
