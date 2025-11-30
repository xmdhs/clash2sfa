package utils

import (
	"reflect"
)

func AnyGet[K any](d any, f string) K {
	rv := reflect.ValueOf(d)
	rv = reflect.Indirect(rv)

	var k K

	switch rv.Type().Kind() {
	case reflect.Map, reflect.Interface:
		m, ok := rv.Interface().(map[string]any)
		if !ok {
			return k
		}
		k, ok := m[f].(K)
		if !ok {
			return k
		}
		return k
	}
	return k
}

func AnySet(t, d any, fieldName string) bool {
	rv := reflect.ValueOf(t)

	if rv.Kind() != reflect.Pointer {
		return false
	}

	rv = rv.Elem()
	rv = reflect.Indirect(rv)

	switch rv.Type().Kind() {
	case reflect.Map, reflect.Interface:
		m, ok := rv.Interface().(map[string]any)
		if !ok || m == nil {
			return false
		}
		m[fieldName] = d
		return true
	}
	return true
}
