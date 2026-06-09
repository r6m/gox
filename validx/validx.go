// Package validx provides configured struct validation and readable field errors.
package validx

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

// FieldErrors maps JSON field paths to readable validation messages.
type FieldErrors map[string]string

// Error summarizes the validation failure.
func (e FieldErrors) Error() string { return "validation failed" }

// Fields returns a plain map suitable for an HTTP error response.
func (e FieldErrors) Fields() map[string]string { return e }

var (
	instance = newValidator()
	mu       sync.RWMutex
)

func newValidator() *validator.Validate {
	v := validator.New()
	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		if name == "" {
			return field.Name
		}
		return name
	})
	return v
}

// Struct validates a struct value.
func Struct(value any) error {
	mu.RLock()
	err := instance.Struct(value)
	mu.RUnlock()
	if err == nil {
		return nil
	}
	fields := Fields(err)
	if len(fields) > 0 {
		return fields
	}
	return err
}

// Fields converts validator errors to readable field errors.
func Fields(err error) FieldErrors {
	var fieldErrors FieldErrors
	if errors.As(err, &fieldErrors) {
		return fieldErrors
	}
	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) {
		return nil
	}
	fields := make(FieldErrors, len(validationErrors))
	for _, fieldErr := range validationErrors {
		name := fieldErr.Namespace()
		if dot := strings.IndexByte(name, '.'); dot >= 0 {
			name = name[dot+1:]
		}
		fields[name] = message(fieldErr)
	}
	return fields
}

// RegisterValidation registers a custom validation tag.
func RegisterValidation(tag string, fn validator.Func) error {
	mu.Lock()
	defer mu.Unlock()
	return instance.RegisterValidation(tag, fn)
}

func message(err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email"
	case "min":
		return "must be at least " + err.Param() + lengthUnit(err)
	case "max":
		return "must be at most " + err.Param() + lengthUnit(err)
	case "len":
		return "must be exactly " + err.Param() + lengthUnit(err)
	case "oneof":
		return "must be one of: " + err.Param()
	case "uuid":
		return "must be a valid UUID"
	case "url":
		return "must be a valid URL"
	case "gte":
		return "must be greater than or equal to " + err.Param()
	case "lte":
		return "must be less than or equal to " + err.Param()
	default:
		return fmt.Sprintf("failed validation for %s", err.Tag())
	}
}

func lengthUnit(err validator.FieldError) string {
	kind := err.Kind()
	if kind == reflect.String || kind == reflect.Array || kind == reflect.Slice || kind == reflect.Map {
		if err.Param() == "1" {
			return " character"
		}
		return " characters"
	}
	return ""
}

var _ error = FieldErrors{}
