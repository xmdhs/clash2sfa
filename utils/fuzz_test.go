package utils

import (
	"encoding/json"
	"testing"
)

// FuzzAnyGetAnySet 任意结构/字段名都不应 panic（AnyGet 曾对 nil 入参 panic）。
func FuzzAnyGetAnySet(f *testing.F) {
	f.Add([]byte(`{"tag":"a","num":42}`), "tag")
	f.Add([]byte(`["x"]`), "tag")
	f.Add([]byte(``), "x")
	f.Add([]byte(`{"nested":{"k":"v"}}`), "nested")
	f.Fuzz(func(t *testing.T, data []byte, field string) {
		var v any
		_ = json.Unmarshal(data, &v)
		_ = AnyGet[string](v, field)
		_ = AnyGet[int](v, field)

		m := map[string]any{}
		_ = AnySet(&m, field, field)

		var nilMap map[string]any
		_ = AnySet(&nilMap, field, field)
	})
}
