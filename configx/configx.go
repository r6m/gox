// Package configx loads typed configuration from environment variables.
package configx

import (
	"fmt"
	"reflect"

	"github.com/caarlos0/env/v11"
)

// Validator is implemented by configurations that validate themselves after
// environment decoding.
type Validator interface {
	Validate() error
}

// Options controls environment loading.
type Options struct {
	// Prefix is prepended to every environment variable name.
	Prefix string
	// Environment overrides the process environment when non-nil. It is useful
	// for tests and callers that source configuration elsewhere.
	Environment map[string]string
	// FuncMap adds parsers for types that cannot implement encoding.TextUnmarshaler.
	FuncMap map[reflect.Type]env.ParserFunc
}

// Load decodes environment variables into T and validates the result when it
// implements Validator.
func Load[T any](options ...Options) (T, error) {
	var value T
	opts := env.Options{}
	if len(options) > 1 {
		return value, fmt.Errorf("configx: expected at most one Options value")
	}
	if len(options) == 1 {
		opts.Prefix = options[0].Prefix
		opts.Environment = options[0].Environment
		opts.FuncMap = options[0].FuncMap
	}
	if err := env.ParseWithOptions(&value, opts); err != nil {
		return value, fmt.Errorf("configx: load environment: %w", err)
	}
	if validator, ok := any(value).(Validator); ok {
		if err := validator.Validate(); err != nil {
			return value, fmt.Errorf("configx: validate: %w", err)
		}
	} else if validator, ok := any(&value).(Validator); ok {
		if err := validator.Validate(); err != nil {
			return value, fmt.Errorf("configx: validate: %w", err)
		}
	}
	return value, nil
}
