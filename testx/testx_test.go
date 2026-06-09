package testx

import (
	"net/http"
	"strings"
	"testing"
)

func TestJSONHelpers(t *testing.T) {
	req := JSONRequest(t, http.MethodPost, "/", map[string]string{"name": "reza"})
	if req.Header.Get("Content-Type") != "application/json" {
		t.Fatal("content type not set")
	}
	value := DecodeJSON[map[string]string](t, req.Body)
	if value["name"] != "reza" {
		t.Fatalf("unexpected value: %#v", value)
	}
	AssertStatus(t, http.StatusCreated, http.StatusCreated)

	decoded := DecodeJSON[map[string]int](t, strings.NewReader(`{"count":2}`))
	if decoded["count"] != 2 {
		t.Fatalf("unexpected value: %#v", decoded)
	}
}
