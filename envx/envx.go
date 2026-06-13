// Package envx provides legacy environment variable parsing.
//
// Deprecated: use configx for typed, error-returning configuration loading.
package envx

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// String returns an environment value or fallback when unset.
func String(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// Required returns an environment value or panics when unset or empty.
func Required(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return value
}

// Int returns a parsed integer or fallback when unset or invalid.
func Int(key string, fallback int) int {
	value, ok, err := LookupInt(key)
	if !ok || err != nil {
		return fallback
	}
	return value
}

// Bool returns a parsed boolean or fallback when unset or invalid.
func Bool(key string, fallback bool) bool {
	value, ok, err := LookupBool(key)
	if !ok || err != nil {
		return fallback
	}
	return value
}

// Duration returns a parsed duration or fallback when unset or invalid.
func Duration(key string, fallback time.Duration) time.Duration {
	value, ok, err := LookupDuration(key)
	if !ok || err != nil {
		return fallback
	}
	return value
}

// LookupInt parses an optional integer environment variable.
func LookupInt(key string) (int, bool, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return 0, false, nil
	}
	parsed, err := strconv.Atoi(value)
	return parsed, true, err
}

// LookupBool parses an optional boolean environment variable.
func LookupBool(key string) (bool, bool, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return false, false, nil
	}
	parsed, err := strconv.ParseBool(value)
	return parsed, true, err
}

// LookupDuration parses an optional duration environment variable.
func LookupDuration(key string) (time.Duration, bool, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return 0, false, nil
	}
	parsed, err := time.ParseDuration(value)
	return parsed, true, err
}
