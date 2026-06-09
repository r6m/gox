// Package httpx provides small helpers for JSON HTTP handlers.
package httpx

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
)

// HandlerFunc is an HTTP handler that can return an error.
type HandlerFunc func(http.ResponseWriter, *http.Request) error

// Handler adapts an error-returning handler to http.HandlerFunc.
func Handler(fn HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			writeError(w, r, err)
		}
	}
}

// Bind decodes a JSON request and invokes Validate when implemented by T.
func Bind[T any](r *http.Request) (T, error) {
	var value T
	if err := render.DecodeJSON(r.Body, &value); err != nil {
		return value, BadRequest("invalid JSON body").WithCode("invalid_json").WithInternal(err)
	}

	validatable, ok := any(value).(interface{ Validate() error })
	if !ok {
		validatable, ok = any(&value).(interface{ Validate() error })
	}
	if ok {
		if err := validatable.Validate(); err != nil {
			httpErr := BadRequest("validation failed").WithCode("validation_failed").WithInternal(err)
			if fieldProvider, ok := err.(interface{ Fields() map[string]string }); ok {
				httpErr.WithFields(fieldProvider.Fields())
			}
			return value, httpErr
		}
	}

	return value, nil
}

// JSON writes a JSON success response using the standard data envelope.
func JSON(w http.ResponseWriter, r *http.Request, status int, value any) error {
	render.Status(r, status)
	render.JSON(w, r, map[string]any{"data": value})
	return nil
}

// OK writes a 200 JSON response.
func OK(w http.ResponseWriter, r *http.Request, value any) error {
	return JSON(w, r, http.StatusOK, value)
}

// Created writes a 201 JSON response.
func Created(w http.ResponseWriter, r *http.Request, value any) error {
	return JSON(w, r, http.StatusCreated, value)
}

// NoContent writes a 204 response without a body.
func NoContent(w http.ResponseWriter, _ *http.Request) error {
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	httpErr, ok := IsHTTPError(err)
	if !ok {
		httpErr = Internal("internal server error").WithInternal(err)
	}
	render.Status(r, httpErr.Status)
	render.JSON(w, r, map[string]any{"error": httpErr})
}

// Error is a client-safe HTTP error.
type Error struct {
	Status  int               `json:"-"`
	Code    string            `json:"code,omitempty"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
	Err     error             `json:"-"`
}

// Error returns the client-safe error message.
func (e *Error) Error() string { return e.Message }

// Unwrap returns the internal cause.
func (e *Error) Unwrap() error { return e.Err }

// WithCode adds a machine-readable error code.
func (e *Error) WithCode(code string) *Error {
	e.Code = code
	return e
}

// WithFields adds field validation errors.
func (e *Error) WithFields(fields map[string]string) *Error {
	e.Fields = fields
	return e
}

// WithInternal records an internal cause that is not serialized.
func (e *Error) WithInternal(err error) *Error {
	e.Err = err
	return e
}

// BadRequest creates a 400 error.
func BadRequest(message string) *Error { return newError(http.StatusBadRequest, message) }

// Unauthorized creates a 401 error.
func Unauthorized(message string) *Error { return newError(http.StatusUnauthorized, message) }

// Forbidden creates a 403 error.
func Forbidden(message string) *Error { return newError(http.StatusForbidden, message) }

// NotFound creates a 404 error.
func NotFound(message string) *Error { return newError(http.StatusNotFound, message) }

// Conflict creates a 409 error.
func Conflict(message string) *Error { return newError(http.StatusConflict, message) }

// Internal creates a 500 error.
func Internal(message string) *Error { return newError(http.StatusInternalServerError, message) }

// IsHTTPError finds an Error in an error chain.
func IsHTTPError(err error) (*Error, bool) {
	var httpErr *Error
	ok := errors.As(err, &httpErr)
	return httpErr, ok
}

func newError(status int, message string) *Error {
	return &Error{Status: status, Message: message}
}
