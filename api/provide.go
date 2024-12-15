package handler

import (
	"crypto/tls"
	"net/http"
	"time"

	"filippo.io/intermediates"
	"github.com/go-chi/chi/v5"
	"github.com/google/wire"
)

var All = wire.NewSet(NewSlog, NewClient, SetMux, NewHttpServer)

func NewClient() *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
		VerifyConnection:   intermediates.VerifyConnection,
	}
	return &http.Client{
		Transport: tr,
		Timeout:   60 * time.Second,
	}
}

func NewHttpServer(m *chi.Mux) http.Handler {
	return m
}
