package utils

import "reflect"

func AnyGet[K any](d any, f string) K {
	var zero K
	var m map[string]any

	switch v := d.(type) {
	case map[string]any:
		m = v
	case *map[string]any:
		if v == nil {
			return zero
		}
		m = *v
	case *any:
		if v == nil {
			return zero
		}
		var ok bool
		m, ok = (*v).(map[string]any)
		if !ok {
			return zero
		}
	default:
		return zero
	}

	value, ok := m[f]
	if !ok {
		return zero
	}
	result, ok := value.(K)
	if !ok {
		return zero
	}
	return result
}

func AnySet(t, d any, fieldName string) bool {
	var m map[string]any

	switch v := t.(type) {
	case *map[string]any:
		if v == nil {
			return false
		}
		m = *v
	case *any:
		if v == nil {
			return false
		}
		var ok bool
		m, ok = (*v).(map[string]any)
		if !ok {
			return false
		}
	case **map[string]any:
		if v == nil || *v == nil {
			return false
		}
		m = **v
	case **any:
		if v == nil || *v == nil {
			return false
		}
		var ok bool
		m, ok = (**v).(map[string]any)
		if !ok {
			return false
		}
	default:
		// 保持旧 API 对“指向非 map 的指针”返回 true 的行为；
		// 常见 map 路径不会进入这里，因此不再为热路径支付反射开销。
		rv := reflect.ValueOf(t)
		if !rv.IsValid() || rv.Kind() != reflect.Pointer || rv.IsNil() {
			return false
		}
		rv = rv.Elem()
		if rv.IsValid() {
			rv = reflect.Indirect(rv)
		}
		if rv.IsValid() && rv.Kind() == reflect.Map {
			return false
		}
		return true
	}

	if m == nil {
		return false
	}
	m[fieldName] = d
	return true
}
