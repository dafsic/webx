package utils

import (
	"fmt"
	"reflect"
)

func StructToMap(s any) map[string]any {
	result := make(map[string]any)
	v := reflect.ValueOf(s).Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		name := t.Field(i).Tag.Get("json")
		result[name] = field.Interface()
	}

	return result
}

func MapToStruct(m map[string]any, out any) error {
	v := reflect.ValueOf(out)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return fmt.Errorf("out must be a non-nil pointer to struct")
	}
	v = v.Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		sf := t.Field(i)

		name := sf.Tag.Get("json")
		if name == "" {
			continue
		}

		if val, ok := m[name]; ok {
			fv := reflect.ValueOf(val)
			// 类型兼容时直接赋值
			if fv.Type().AssignableTo(field.Type()) {
				field.Set(fv)
			} else if fv.Type().ConvertibleTo(field.Type()) {
				field.Set(fv.Convert(field.Type()))
			}
		}
	}
	return nil
}
