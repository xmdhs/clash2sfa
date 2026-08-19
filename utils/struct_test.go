package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnyGet(t *testing.T) {
	m := map[string]any{"tag": "a", "num": 42}
	assert.Equal(t, "a", AnyGet[string](m, "tag"))
	assert.Equal(t, 42, AnyGet[int](m, "num"))
	// 缺字段返回零值
	assert.Equal(t, "", AnyGet[string](m, "missing"))
	// 类型不匹配返回零值
	assert.Equal(t, int(0), AnyGet[int](m, "tag"))
	// 非 map 返回零值
	assert.Equal(t, "", AnyGet[string]("not-a-map", "tag"))
	assert.Equal(t, "", AnyGet[string](nil, "tag"))
}

func TestAnySet(t *testing.T) {
	m := map[string]any{}
	assert.True(t, AnySet(&m, "x", "tag"))
	assert.Equal(t, "x", AnyGet[string](m, "tag"))

	// 非指针直接失败
	m2 := map[string]any{}
	assert.False(t, AnySet(m2, "x", "tag"))
	assert.Equal(t, "", AnyGet[string](m2, "tag"))

	// nil map 失败
	var m3 map[string]any
	assert.False(t, AnySet(&m3, "x", "tag"))
}
