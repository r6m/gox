// Package logx provides minimal slog configuration.
package logx

import (
	"log/slog"
	"os"
	"strings"
)

// Config configures a slog logger.
type Config struct {
	Env   string
	Level string
}

// New creates a text logger for local environments and JSON elsewhere.
func New(cfg Config) *slog.Logger {
	options := &slog.HandlerOptions{Level: parseLevel(cfg.Level)}
	var handler slog.Handler
	switch strings.ToLower(cfg.Env) {
	case "local", "dev", "development":
		handler = slog.NewTextHandler(os.Stderr, options)
	default:
		handler = slog.NewJSONHandler(os.Stderr, options)
	}
	return slog.New(handler)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
