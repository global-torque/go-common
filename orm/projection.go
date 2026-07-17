package orm

import (
	"fmt"
	"reflect"
)

func projectionFields[Source, Projection any]() ([]string, error) {
	sourceType, err := structTypeOf[Source]()
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}

	projectionType, err := structTypeOf[Projection]()
	if err != nil {
		return nil, fmt.Errorf("projection: %w", err)
	}

	sourceColumns := make(map[string]struct{}, sourceType.NumField())
	for i := range sourceType.NumField() {
		field := sourceType.Field(i)

		column, tagged := field.Tag.Lookup("db")
		if !tagged {
			continue
		}

		column = cleanTag(column)
		if column == "" || column == "-" {
			continue
		}

		if _, exists := sourceColumns[column]; exists {
			return nil, fmt.Errorf("%w: source column %q is duplicated", ErrInvalidProjection, column)
		}

		sourceColumns[column] = struct{}{}
	}

	if projectionType.NumField() == 0 {
		return nil, fmt.Errorf("%w: projection has no fields", ErrInvalidProjection)
	}

	columns := make([]string, 0, projectionType.NumField())

	seen := make(map[string]struct{}, projectionType.NumField())
	for i := range projectionType.NumField() {
		field := projectionType.Field(i)
		if !field.IsExported() {
			return nil, fmt.Errorf("%w: projection field %s is not exported", ErrInvalidProjection, field.Name)
		}

		tag, tagged := field.Tag.Lookup("db")
		if !tagged {
			return nil, fmt.Errorf("%w: projection field %s has no db tag", ErrInvalidProjection, field.Name)
		}

		column := cleanTag(tag)
		if column == "" || column == "-" {
			return nil, fmt.Errorf(
				"%w: projection field %s has invalid db tag %q",
				ErrInvalidProjection,
				field.Name,
				tag,
			)
		}

		if _, exists := seen[column]; exists {
			return nil, fmt.Errorf("%w: projection column %q is duplicated", ErrInvalidProjection, column)
		}

		if _, exists := sourceColumns[column]; !exists {
			return nil, fmt.Errorf("%w: projection column %q is not present on source", ErrInvalidProjection, column)
		}

		seen[column] = struct{}{}
		columns = append(columns, column)
	}

	return columns, nil
}

func structTypeOf[T any]() (reflect.Type, error) {
	typ := reflect.TypeFor[T]()
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%w: expected struct, got %s", ErrInvalidProjection, typ.Kind())
	}

	return typ, nil
}
