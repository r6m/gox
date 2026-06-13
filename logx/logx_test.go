package logx

import (
	"bytes"
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

func TestConfiguration(t *testing.T) {
	var output bytes.Buffer
	logger := New(Config{
		Writer: &output,
		Format: FormatJSON,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.MessageKey {
				attr.Value = slog.StringValue("replaced")
			}
			return attr
		},
	})
	logger.Info("original")
	if !bytes.Contains(output.Bytes(), []byte(`"msg":"replaced"`)) {
		t.Fatalf("unexpected output: %s", output.String())
	}
}
