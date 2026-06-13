// Package testx provides small helpers for HTTP tests.
package testx

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

// AssertJSON decodes a response body and compares it with want.
func AssertJSON[T comparable](t testing.TB, body io.Reader, want T) {
	t.Helper()
	if got := DecodeJSON[T](t, body); got != want {
		t.Fatalf("unexpected JSON: got %#v, want %#v", got, want)
	}
}

// Serve records an HTTP handler response for req.
func Serve(t testing.TB, handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

// OpenPostgres opens a PostgreSQL test pool and registers cleanup.
func OpenPostgres(t testing.TB, databaseURL string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("open PostgreSQL pool: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Fatalf("ping PostgreSQL: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// OpenPostgresEnv opens a PostgreSQL test pool using an environment variable.
// The test is skipped when the variable is unset.
func OpenPostgresEnv(t testing.TB, key string) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv(key)
	if databaseURL == "" {
		t.Skipf("%s is not set", key)
	}
	return OpenPostgres(t, databaseURL)
}

// PreparePostgres opens a pool and runs an application-provided migration
// callback. The callback owns all schema and migration decisions.
func PreparePostgres(
	t testing.TB,
	databaseURL string,
	migrate func(context.Context, *pgxpool.Pool) error,
) *pgxpool.Pool {
	t.Helper()
	pool := OpenPostgres(t, databaseURL)
	if migrate != nil {
		if err := migrate(context.Background(), pool); err != nil {
			t.Fatalf("migrate PostgreSQL: %v", err)
		}
	}
	return pool
}

// CleanupPostgres registers an application-provided database cleanup callback.
func CleanupPostgres(
	t testing.TB,
	pool *pgxpool.Pool,
	cleanup func(context.Context, *pgxpool.Pool) error,
) {
	t.Helper()
	if cleanup == nil {
		return
	}
	t.Cleanup(func() {
		if err := cleanup(context.Background(), pool); err != nil {
			t.Errorf("clean up PostgreSQL: %v", err)
		}
	})
}

// PostgresTx starts a transaction that is rolled back when the test finishes.
func PostgresTx(t testing.TB, pool *pgxpool.Pool) pgx.Tx {
	t.Helper()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin PostgreSQL transaction: %v", err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(context.Background()); err != nil && err != pgx.ErrTxClosed {
			t.Errorf("roll back PostgreSQL transaction: %v", err)
		}
	})
	return tx
}
