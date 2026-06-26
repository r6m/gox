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
	defaultValidator = New()
)

// Validator is an independently configurable validator.
type Validator struct {
	instance         *validator.Validate
	fieldxFieldTypes map[reflect.Type]struct{}
	mu               sync.RWMutex
}

// New creates a validator configured to report JSON field names.
func New() *Validator {
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
	return &Validator{instance: v, fieldxFieldTypes: make(map[reflect.Type]struct{})}
}

// Struct validates a struct value.
func (v *Validator) Struct(value any) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	skip := make(map[string]struct{})
	v.prepareFieldxFields(reflect.ValueOf(value), "", skip)
	err := v.instance.StructFiltered(value, func(ns []byte) bool {
		_, ok := skip[string(ns)]
		return ok
	})
	if err == nil {
		return nil
	}
	fields := Fields(err)
	if len(fields) > 0 {
		return fields
	}
	return err
}

// RegisterValidation registers a custom validation tag on v.
func (v *Validator) RegisterValidation(tag string, fn validator.Func) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.instance.RegisterValidation(tag, fn)
}

func (v *Validator) prepareFieldxFields(value reflect.Value, namespace string, skip map[string]struct{}) {
	value = dereference(value)
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return
	}
	typ := value.Type()
	if namespace == "" {
		namespace = typ.Name()
	}
	for i := 0; i < value.NumField(); i++ {
		fieldType := typ.Field(i)
		if fieldType.PkgPath != "" && !fieldType.Anonymous {
			continue
		}
		field := value.Field(i)
		fieldNamespace := fieldType.Name
		if namespace != "" {
			fieldNamespace = namespace + "." + fieldType.Name
		}
		if isFieldxField(field.Type()) {
			v.registerFieldxFieldType(field.Type())
			if !fieldxIsSet(field) && !requiresPresence(fieldType.Tag.Get("validate")) {
				skip[fieldNamespace] = struct{}{}
			}
			continue
		}
		v.prepareNestedFieldxFields(field, fieldNamespace, skip)
	}
}

func (v *Validator) prepareNestedFieldxFields(value reflect.Value, namespace string, skip map[string]struct{}) {
	value = dereference(value)
	if !value.IsValid() {
		return
	}
	switch value.Kind() {
	case reflect.Struct:
		v.prepareFieldxFields(value, namespace, skip)
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			v.prepareNestedFieldxFields(value.Index(i), fmt.Sprintf("%s[%d]", namespace, i), skip)
		}
	}
}

func (v *Validator) registerFieldxFieldType(typ reflect.Type) {
	if _, ok := v.fieldxFieldTypes[typ]; ok {
		return
	}
	v.instance.RegisterCustomTypeFunc(fieldxValue, reflect.New(typ).Elem().Interface())
	v.fieldxFieldTypes[typ] = struct{}{}
}

func fieldxValue(field reflect.Value) any {
	if !field.CanInterface() {
		return nil
	}
	values := field.MethodByName("Value").Call(nil)
	if len(values) != 2 || !values[1].Bool() {
		return nil
	}
	return values[0].Interface()
}

func fieldxIsSet(field reflect.Value) bool {
	if !field.CanInterface() {
		return false
	}
	values := field.MethodByName("IsSet").Call(nil)
	return len(values) == 1 && values[0].Bool()
}

func isFieldxField(typ reflect.Type) bool {
	return typ.Kind() == reflect.Struct &&
		typ.PkgPath() == "github.com/r6m/gox/fieldx" &&
		strings.HasPrefix(typ.Name(), "Field[")
}

func requiresPresence(tag string) bool {
	for _, rule := range strings.Split(tag, ",") {
		name, _, _ := strings.Cut(rule, "=")
		for _, option := range strings.Split(name, "|") {
			if option == "required" || strings.HasPrefix(option, "required_") {
				return true
			}
		}
	}
	return false
}

func dereference(value reflect.Value) reflect.Value {
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	return value
}

// Struct validates with the package default validator.
//
// Use New when custom registrations should be isolated.
func Struct(value any) error {
	return defaultValidator.Struct(value)
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
//
// Deprecated: create a Validator with New and register on that instance.
func RegisterValidation(tag string, fn validator.Func) error {
	return defaultValidator.RegisterValidation(tag, fn)
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
