//go:build wireinject

package provide

import (
	"log/slog"
	"net/http"

	"github.com/google/wire"
)

func InitializeServer(h slog.Handler) (http.Handler, func(), error) {
	panic(wire.Build(All))
}
