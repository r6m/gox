package testx

import (
	"net/http"
	"net/http/httptest"
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

func TestServeAndAssertJSON(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"queued"}`))
	})
	recorder := Serve(t, handler, httptest.NewRequest(http.MethodGet, "/", nil))
	AssertStatus(t, recorder.Code, http.StatusAccepted)
	AssertJSON(t, recorder.Body, struct {
		Status string `json:"status"`
	}{Status: "queued"})
}
