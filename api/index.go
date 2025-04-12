package handler

import (
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/samber/lo"
	"github.com/xmdhs/clash2sfa/provide"
)

var handleOnce = sync.OnceValue(func() http.Handler {
	level := &slog.LevelVar{}
	level.Set(slog.Level(-4))
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	handle, _, err := provide.InitializeServer(h)
	lo.Must0(err)
	return handle
})

func Handler(w http.ResponseWriter, r *http.Request) {
	handleOnce().ServeHTTP(w, r)
}
