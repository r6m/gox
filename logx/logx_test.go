package logx

import (
	"context"
	"log/slog"
	"testing"
)

func TestLevels(t *testing.T) {
	logger := New(Config{Env: "dev", Level: "debug"})
	if !logger.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("debug should be enabled")
	}
	logger = New(Config{Level: "bad"})
	if logger.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("invalid level should default to info")
	}
}
