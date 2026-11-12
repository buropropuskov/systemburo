package validator

import (
	"encoding/json"
	"reflect"
	"strings"

	"systemburo/internal/apperr"
)

// DatabaseText validates explicitly selected database-bound values. Callers must
// not pass credentials or opaque tokens: those have their own validation. Binary
// data is left untouched; RawMessage is decoded because it represents JSON text.
// Values are checked without normalization or a second URL-decoding pass.
func DatabaseText(values ...any) error {
	for _, value := range values {
		if err := databaseTextValue(reflect.ValueOf(value)); err != nil {
			return err
		}
	}
	return nil
}

func databaseTextValue(value reflect.Value) error {
	if !value.IsValid() {
		return nil
	}
	if value.CanInterface() {
		if raw, ok := value.Interface().(json.RawMessage); ok {
			if len(raw) == 0 {
				return nil
			}
			var decoded any
			if err := json.Unmarshal(raw, &decoded); err != nil {
				return apperr.Validation("Некорректный JSON")
			}
			return databaseTextValue(reflect.ValueOf(decoded))
		}
	}
	switch value.Kind() {
	case reflect.String:
		if strings.ContainsRune(value.String(), '\x00') {
			return apperr.Validation("Недопустимый символ в текстовом поле")
		}
	case reflect.Pointer, reflect.Interface:
		if !value.IsNil() {
			return databaseTextValue(value.Elem())
		}
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if value.Field(i).CanInterface() {
				if err := databaseTextValue(value.Field(i)); err != nil {
					return err
				}
			}
		}
	case reflect.Slice, reflect.Array:
		if value.Type().Elem().Kind() == reflect.Uint8 {
			return nil
		}
		for i := 0; i < value.Len(); i++ {
			if err := databaseTextValue(value.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		iter := value.MapRange()
		for iter.Next() {
			if err := databaseTextValue(iter.Key()); err != nil {
				return err
			}
			if err := databaseTextValue(iter.Value()); err != nil {
				return err
			}
		}
	}
	return nil
}
