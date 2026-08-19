package utils

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetIPFromRealIP(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("X-REAL-IP", "1.2.3.4")
	got, err := GetIP(r)
	require.NoError(t, err)
	assert.Equal(t, "1.2.3.4", got)
}

func TestGetIPFromForwardedFor(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	// X-REAL-IP 无效时回退到 X-FORWARDED-FOR 首个合法项
	r.Header.Set("X-REAL-IP", "not-an-ip")
	r.Header.Set("X-Forwarded-For", "8.8.8.8, 8.8.4.4")
	got, err := GetIP(r)
	require.NoError(t, err)
	assert.Equal(t, "8.8.8.8", got)
}

func TestGetIPFromRemoteAddr(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:5678"
	got, err := GetIP(r)
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", got)
}

func TestGetIPAllInvalid(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	r.RemoteAddr = "not-a-socket"
	_, err := GetIP(r)
	assert.Error(t, err)
}
