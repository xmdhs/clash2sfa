package utils

import (
	"reflect"
)

func AnyGet[K any](d any, f string) K {
	rv := reflect.ValueOf(d)
	rv = reflect.Indirect(rv)

	var k K

	switch rv.Type().Kind() {
	case reflect.Struct:
		f := rv.FieldByName(f)
		if !f.IsValid() {
			return k
		}
		d, ok := f.Interface().(K)
		if !ok {
			return k
		}
		return d
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

func AnySet(t, d any, f string) bool {
	rv := reflect.ValueOf(t)

	if rv.Kind() != reflect.Pointer {
		panic("must is Pointer")
	}

	rv = rv.Elem()
	rv = reflect.Indirect(rv)

	switch rv.Type().Kind() {
	case reflect.Struct:
		f := rv.FieldByName(f)
		if !f.IsValid() {
			return false
		}
		f.Set(reflect.ValueOf(d))

	case reflect.Map, reflect.Interface:
		m, ok := rv.Interface().(map[string]any)
		if !ok {
			return false
		}
		m[f] = d
	}
	return true
}
