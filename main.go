package main

import (
	"crypto/tls"
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"log/slog"

	"filippo.io/intermediates"
	"github.com/xmdhs/clash2sfa/db"
	"github.com/xmdhs/clash2sfa/handle"
	"github.com/xmdhs/clash2sfa/utils"
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

	l := slog.New(&warpSlogHandle{
		Handler: slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		}),
	})

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
		VerifyConnection:   intermediates.VerifyConnection,
	}
	c := &http.Client{
		Timeout: 10 * time.Second,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/put", handle.PutArg(db, l))
	mux.HandleFunc("/sub", handle.Sub(c, db, configByte, l))
	mux.HandleFunc("/config", handle.Frontend(configByte, 604800))
	mux.HandleFunc("/", handle.Frontend(frontendByte, 604800))

	trackid := atomic.Uint64{}

	s := http.Server{
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		Addr:              port,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if l.Enabled(ctx, slog.LevelDebug) {
				ip, _ := utils.GetIP(r)
				trackid.Add(1)
				ctx = setCtx(ctx, &reqInfo{
					URL:     r.URL.String(),
					IP:      ip,
					TrackId: trackid.Load(),
				})
				r = r.WithContext(ctx)
			}
			mux.ServeHTTP(w, r)
		}),
	}
	fmt.Println(s.ListenAndServe())
}
