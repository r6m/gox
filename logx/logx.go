// Package logx provides minimal slog configuration.
package logx

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Format selects a slog handler format.
type Format string

const (
	// FormatAuto uses text for local environments and JSON otherwise.
	FormatAuto Format = ""
	// FormatText selects slog.TextHandler.
	FormatText Format = "text"
	// FormatJSON selects slog.JSONHandler.
	FormatJSON Format = "json"
)

// Config configures a slog logger.
type Config struct {
	Env         string
	Level       string
	Writer      io.Writer
	Format      Format
	AddSource   bool
	ReplaceAttr func([]string, slog.Attr) slog.Attr
}

// New creates a configured slog logger.
func New(cfg Config) *slog.Logger {
	writer := cfg.Writer
	if writer == nil {
		writer = os.Stderr
	}
	options := &slog.HandlerOptions{
		Level:       parseLevel(cfg.Level),
		AddSource:   cfg.AddSource,
		ReplaceAttr: cfg.ReplaceAttr,
	}
	var handler slog.Handler
	format := cfg.Format
	if format == FormatAuto {
		switch strings.ToLower(cfg.Env) {
		case "local", "dev", "development":
			format = FormatText
		default:
			format = FormatJSON
		}
	}
	switch format {
	case FormatText:
		handler = slog.NewTextHandler(writer, options)
	default:
		handler = slog.NewJSONHandler(writer, options)
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
