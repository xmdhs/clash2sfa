package main

import (
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"time"

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
	c := &http.Client{}

	mux := http.NewServeMux()
	mux.HandleFunc("/put", handle.PutArg(db))
	mux.HandleFunc("/sub", handle.Sub(c, db, configByte))
	mux.HandleFunc("/config", handle.Frontend(configByte, 604800))
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
