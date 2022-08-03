package util

import (
	"fmt"
	"reflect"
)

const replacement = "**********"

func RedactStruct(struc interface{}, fields ...string) interface{} {
	v := reflect.ValueOf(struc).Elem()
	if v.Type().Kind() != reflect.Struct {
		panic(fmt.Sprintf("Unsupported type, %v", v.Type().String()))
	}
	for _, field := range fields {
		f := v.FieldByName(field)
		if f.Kind() == reflect.String {
			f.SetString(replacement)
		} else {
			f.Set(reflect.Zero(v.Type()))
		}
	}
	return struc
}
