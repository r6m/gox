// Package page provides opt-in offset pagination helpers for HTTP APIs.
package page

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

// Config controls pagination query parsing.
type Config struct {
	DefaultLimit int
	MaxLimit     int
}

// Params contains validated offset pagination parameters.
type Params struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

// Page is an offset-paginated result.
type Page[T any] struct {
	Items  []T `json:"items"`
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
	Total  int `json:"total"`
}

// Error describes an invalid pagination query field.
type Error struct {
	Field   string
	Message string
}

// Error returns the validation message.
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Parse reads offset and limit query parameters and validates them.
func Parse(r *http.Request, cfg Config) (Params, error) {
	if cfg.DefaultLimit <= 0 {
		cfg.DefaultLimit = 20
	}
	if cfg.MaxLimit <= 0 {
		cfg.MaxLimit = 100
	}
	if cfg.DefaultLimit > cfg.MaxLimit {
		return Params{}, errors.New("page: DefaultLimit must not exceed MaxLimit")
	}

	offset, err := parseNonNegative(r.URL.Query().Get("offset"), 0, "offset")
	if err != nil {
		return Params{}, err
	}
	limit, err := parseNonNegative(r.URL.Query().Get("limit"), cfg.DefaultLimit, "limit")
	if err != nil {
		return Params{}, err
	}
	if limit < 1 {
		return Params{}, &Error{Field: "limit", Message: "must be at least 1"}
	}
	if limit > cfg.MaxLimit {
		return Params{}, &Error{
			Field:   "limit",
			Message: fmt.Sprintf("must be at most %d", cfg.MaxLimit),
		}
	}
	return Params{Offset: offset, Limit: limit}, nil
}

func parseNonNegative(raw string, fallback int, field string) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, &Error{Field: field, Message: "must be an integer"}
	}
	if value < 0 {
		return 0, &Error{Field: field, Message: "must be non-negative"}
	}
	return value, nil
}
