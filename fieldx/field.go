// Package fieldx provides a generic three-state field for partial updates.
package fieldx

import (
	"bytes"
	"encoding/json"
	"errors"
)

// Field represents an omitted value, an explicit null, or an explicit value.
//
// A value Field cannot be omitted by encoding/json's omitempty option because
// it is a struct. Use a pointer field when omitempty output is required, or use
// omitzero with Go versions that support it. Directly marshaling an unset Field
// produces null.
type Field[T any] struct {
	value T
	set   bool
	null  bool
}

// Unset returns a field that was not supplied.
func Unset[T any]() Field[T] {
	return Field[T]{}
}

// Value returns a field explicitly set to value.
func Value[T any](value T) Field[T] {
	return Field[T]{value: value, set: true}
}

// Null returns a field explicitly set to null.
func Null[T any]() Field[T] {
	return Field[T]{set: true, null: true}
}

// IsSet reports whether the field was explicitly supplied.
func (f Field[T]) IsSet() bool {
	return f.set
}

// IsNull reports whether the field was explicitly supplied as null.
func (f Field[T]) IsNull() bool {
	return f.set && f.null
}

// Value returns the field value and true when it is set and non-null.
func (f Field[T]) Value() (T, bool) {
	return f.value, f.set && !f.null
}

// MustValue returns the field value or panics when it is unset or null.
func (f Field[T]) MustValue() T {
	if !f.set || f.null {
		panic("fieldx: field has no value")
	}
	return f.value
}

// Reset changes the field to the unset state. This is useful when reusing a
// containing DTO because encoding/json does not visit omitted object members.
func (f *Field[T]) Reset() {
	if f != nil {
		*f = Field[T]{}
	}
}

// IsZero reports whether the field is unset. This supports encoding/json's
// omitzero option on Go versions that recognize IsZero.
func (f Field[T]) IsZero() bool {
	return !f.set
}

// MarshalJSON encodes a set value or null. An unset field also encodes as null;
// MarshalJSON cannot cause a containing struct field to be omitted.
func (f Field[T]) MarshalJSON() ([]byte, error) {
	if !f.set || f.null {
		return []byte("null"), nil
	}
	return json.Marshal(f.value)
}

// UnmarshalJSON marks the field as supplied and distinguishes null from a
// decoded value. It resets previous state before decoding.
func (f *Field[T]) UnmarshalJSON(data []byte) error {
	if f == nil {
		return errors.New("fieldx: unmarshal into nil Field")
	}
	var zero T
	f.value = zero
	f.set = true
	f.null = false
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		f.null = true
		return nil
	}
	if err := json.Unmarshal(data, &f.value); err != nil {
		return err
	}
	return nil
}
