package utils

import (
	"net/http"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/xmdhs/clash2singbox/model"
)

func TestGetIPInvalidHostFromRemoteAddr(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	r.RemoteAddr = "not-an-ip:123"
	_, err := GetIP(r)
	assert.Error(t, err)
}

func TestAnyGetTypeMismatch(t *testing.T) {
	// Kind 为 Map 但类型断言到 map[string]any 失败 → 返回零值
	v := AnyGet[int](map[string]int{"n": 1}, "n")
	assert.Equal(t, 0, v)
	v2 := AnyGet[string]([]string{"x"}, "tag")
	assert.Equal(t, "", v2)
}

func TestAnySetNonMapDefault(t *testing.T) {
	// 指针指向非 map 类型时返回 true
	assert.True(t, AnySet(&struct{}{}, "x", "tag"))
}

func TestGetSingBoxVersionSemverError(t *testing.T) {
	// 数字形式 pre-release 带前导零，semver 解析失败 → 返回 SINGLATEST
	// （SINGLATEST 是 int 类型常量，需显式转换为 SingBoxVer 再比较）
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent", "sing-box 1.2.3-01 (linux; amd64)")
	assert.Equal(t, model.SingBoxVer(model.SINGLATEST), GetSingBoxVersion(r))
}

func TestGetConfigDefault(t *testing.T) {
	fsys := fstest.MapFS{
		"config.json.template":         &fstest.MapFile{Data: []byte("v1.10")},
		"config.json-1.11.0+.template": &fstest.MapFile{Data: []byte("v1.11")},
		"config.json-1.12.0+.template": &fstest.MapFile{Data: []byte("v1.12")},
	}
	// SING110=0/SING111=1/SING112=2，负版本落入 default 分支
	assert.Equal(t, []byte("v1.12"), GetConfig(model.SingBoxVer(-1), fsys))
}
