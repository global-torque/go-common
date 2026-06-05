package orm

import (
	"reflect"
	"strings"
)

// DefaultFields returns db tag names, falling back to json tags.
func DefaultFields(obj any) []string {
	fields := []string{}

	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}

	if !val.IsValid() || val.Kind() != reflect.Struct {
		return fields
	}

	typ := val.Type()
	for i := range val.NumField() {
		field := typ.Field(i)

		dbtag := cleanTag(field.Tag.Get("db"))
		if dbtag == "" {
			dbtag = cleanTag(field.Tag.Get("json"))
		}

		if dbtag != "" && dbtag != "-" {
			fields = append(fields, dbtag)
		}
	}

	return fields
}

func cleanTag(tag string) string {
	cleaned, _, _ := strings.Cut(tag, ",")

	return cleaned
}
