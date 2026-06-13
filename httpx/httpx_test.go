package httpx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	if err := OK(rec, req, map[string]string{"name": "reza"}); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"data":{"name":"reza"}}` {
		t.Fatalf("unexpected response: %d %s", rec.Code, rec.Body.String())
	}
}

func TestWriteJSONIsRaw(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := WriteJSON(rec, http.StatusAccepted, map[string]string{"status": "queued"}); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusAccepted || strings.TrimSpace(rec.Body.String()) != `{"status":"queued"}` {
		t.Fatalf("unexpected response: %d %s", rec.Code, rec.Body.String())
	}
}

func TestBindInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{"))
	_, err := Bind[map[string]any](req)
	httpErr, ok := IsHTTPError(err)
	if !ok || httpErr.Status != http.StatusBadRequest || httpErr.Code != "invalid_json" {
		t.Fatalf("unexpected error: %#v", err)
	}
}

func TestHandlerHTTPError(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	Handler(func(http.ResponseWriter, *http.Request) error {
		return Conflict("already exists").WithCode("duplicate")
	})(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"code":"duplicate"`) {
		t.Fatalf("unexpected response: %d %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerUnknownError(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	Handler(func(http.ResponseWriter, *http.Request) error {
		return errors.New("database password leaked")
	})(rec, req)
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "password") {
		t.Fatalf("unexpected response: %d %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerWithErrorWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	writer := ErrorWriterFunc(func(w http.ResponseWriter, _ *http.Request, err error) {
		http.Error(w, err.Error(), http.StatusTeapot)
	})
	HandlerWithErrorWriter(func(http.ResponseWriter, *http.Request) error {
		return errors.New("mapped")
	}, writer)(rec, req)
	if rec.Code != http.StatusTeapot || !strings.Contains(rec.Body.String(), "mapped") {
		t.Fatalf("unexpected response: %d %s", rec.Code, rec.Body.String())
	}
}

type invalidInput struct{}

func (*invalidInput) Validate() error { return errors.New("invalid") }

func TestBindValidation(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}"))
	_, err := Bind[invalidInput](req)
	httpErr, ok := IsHTTPError(err)
	if !ok || httpErr.Code != "validation_failed" {
		t.Fatalf("unexpected error: %#v", err)
	}
}

func TestBindValidationForPointerType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}"))
	_, err := Bind[*invalidInput](req)
	httpErr, ok := IsHTTPError(err)
	if !ok || httpErr.Code != "validation_failed" {
		t.Fatalf("unexpected error: %#v", err)
	}
}
