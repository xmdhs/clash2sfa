package utils

import (
	"net/http"
	"testing"

	"github.com/xmdhs/clash2singbox/model"
)

func TestIsBrowser(t *testing.T) {
	cases := []struct {
		ua   string
		want bool
	}{
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36", true},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)", true},
		{"sing-box/1.12.0 (linux; amd64)", false},
		{"ClashForAndroid/2.5.12", false},
		{"", false},
	}
	for _, c := range cases {
		got := IsBrowser(c.ua)
		if got != c.want {
			t.Errorf("IsBrowser(%q) = %v, want %v", c.ua, got, c.want)
		}
	}
}

func TestGetSingBoxVersion(t *testing.T) {
	cases := []struct {
		ua   string
		want model.SingBoxVer
	}{
		{"sing-box 1.10.0 (linux; amd64)", model.SING110},
		{"sing-box 1.10.7 (linux; amd64)", model.SING110},
		{"sing-box 1.11.0 (linux; amd64)", model.SING111},
		{"sing-box 1.12.0 (linux; amd64)", model.SING112},
		{"sing-box 1.13.0 (linux; amd64)", model.SINGLATEST},
		// 无版本信息时返回 SINGLATEST
		{"ClashForAndroid/2.5.12", model.SINGLATEST},
		{"", model.SINGLATEST},
	}
	for _, c := range cases {
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("User-Agent", c.ua)
		got := GetSingBoxVersion(req)
		if got != c.want {
			t.Errorf("GetSingBoxVersion(%q) = %v, want %v", c.ua, got, c.want)
		}
	}
}
