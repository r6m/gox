package page

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParse(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?offset=10&limit=25", nil)
	got, err := Parse(req, Config{DefaultLimit: 20, MaxLimit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if got.Offset != 10 || got.Limit != 25 {
		t.Fatalf("unexpected params: %#v", got)
	}
}

func TestParseDefaults(t *testing.T) {
	got, err := Parse(httptest.NewRequest(http.MethodGet, "/", nil), Config{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Offset != 0 || got.Limit != 20 {
		t.Fatalf("unexpected params: %#v", got)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		query string
		field string
	}{
		{"?offset=-1", "offset"},
		{"?offset=no", "offset"},
		{"?limit=0", "limit"},
		{"?limit=101", "limit"},
	}
	for _, test := range tests {
		t.Run(test.query, func(t *testing.T) {
			_, err := Parse(httptest.NewRequest(http.MethodGet, "/"+test.query, nil), Config{})
			pageErr, ok := err.(*Error)
			if !ok || pageErr.Field != test.field {
				t.Fatalf("unexpected error: %#v", err)
			}
		})
	}
}
