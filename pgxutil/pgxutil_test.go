package pgxutil

import (
	"context"
	"os"
	"testing"
)

func TestOpenPoolIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := OpenPool(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := Ping(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
}
