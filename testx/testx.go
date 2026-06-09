// Package testx provides small helpers for HTTP tests.
package testx

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// JSONRequest creates a request with a JSON body and content type.
func JSONRequest(t testing.TB, method, target string, body any) *http.Request {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal JSON request: %v", err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, target, reader)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return req
}

// DecodeJSON decodes JSON or fails the test.
func DecodeJSON[T any](t testing.TB, body io.Reader) T {
	t.Helper()
	var value T
	if err := json.NewDecoder(body).Decode(&value); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	return value
}

// AssertStatus fails a test when HTTP statuses differ.
func AssertStatus(t testing.TB, got, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("unexpected status: got %d, want %d", got, want)
	}
}
