package configurator

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// RequiredString is a required string env value that rejects missing, empty,
// or whitespace-only input.
type RequiredString string

var requiredStringType = reflect.TypeOf(RequiredString(""))

// Decode implements envconfig.Decoder.
func (s *RequiredString) Decode(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("missing value")
	}
	*s = RequiredString(value)
	return nil
}

func (s RequiredString) String() string {
	return string(s)
}

func validateRequiredStrings(conf any) error {
	value := reflect.ValueOf(conf)
	if !value.IsValid() {
		return nil
	}

	return validateRequiredStringsValue(value, "")
}

func validateRequiredStringsValue(value reflect.Value, path string) error {
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			if value.Type().Elem() == requiredStringType {
				return fmt.Errorf("%s is required", path)
			}

			return nil
		}

		value = value.Elem()
	}

	if value.Type() == requiredStringType {
		if strings.TrimSpace(value.String()) == "" {
			return fmt.Errorf("%s is required", path)
		}

		return nil
	}

	if value.Kind() != reflect.Struct {
		return nil
	}

	valueType := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := valueType.Field(i)
		if field.PkgPath != "" {
			continue
		}

		fieldPath := field.Name
		if path != "" {
			fieldPath = path + "." + field.Name
		}

		if err := validateRequiredStringsValue(value.Field(i), fieldPath); err != nil {
			return err
		}
	}

	return nil
}
