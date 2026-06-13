package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORS(t *testing.T) {
	handler := CORS(CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{http.MethodGet, http.MethodPost},
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent ||
		rec.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Fatalf("unexpected response: %d %#v", rec.Code, rec.Header())
	}
}

func TestRequestID(t *testing.T) {
	handler := RequestID(RequestIDConfig{Generate: func() string { return "generated" }})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, _ := RequestIDFromContext(r.Context())
			_, _ = w.Write([]byte(id))
		}),
	)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Header().Get("X-Request-ID") != "generated" || rec.Body.String() != "generated" {
		t.Fatalf("unexpected response: %#v %s", rec.Header(), rec.Body.String())
	}
}

func TestRecovery(t *testing.T) {
	var mapped error
	handler := Recovery(RecoveryConfig{
		WriteError: func(w http.ResponseWriter, _ *http.Request, err error) {
			mapped = err
			w.WriteHeader(http.StatusServiceUnavailable)
		},
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("failed")
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusServiceUnavailable || mapped == nil {
		t.Fatalf("unexpected recovery: %d %v", rec.Code, mapped)
	}
}

func TestRequestLoggerEnriches(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	handler := RequestID(RequestIDConfig{Generate: func() string { return "request-1" }})(
		RequestLogger(RequestLogConfig{
			Logger: logger,
			Enrich: func(*http.Request) []slog.Attr {
				return []slog.Attr{slog.String("tenant", "tenant-1")}
			},
		})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
		})),
	)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/users", nil))
	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["status"] != float64(http.StatusCreated) ||
		entry["request_id"] != "request-1" ||
		entry["tenant"] != "tenant-1" ||
		!strings.Contains(entry["path"].(string), "/users") {
		t.Fatalf("unexpected log entry: %#v", entry)
	}
}
