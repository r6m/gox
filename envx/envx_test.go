package envx

import (
	"testing"
	"time"
)

func TestValues(t *testing.T) {
	t.Setenv("STRING", "value")
	t.Setenv("INT", "42")
	t.Setenv("BOOL", "true")
	t.Setenv("DURATION", "2s")
	if String("STRING", "") != "value" || Int("INT", 0) != 42 ||
		!Bool("BOOL", false) || Duration("DURATION", 0) != 2*time.Second {
		t.Fatal("unexpected parsed values")
	}
}

func TestLookupAndFallback(t *testing.T) {
	t.Setenv("BAD_INT", "x")
	if _, ok, err := LookupInt("BAD_INT"); !ok || err == nil {
		t.Fatal("expected parse error")
	}
	if Int("BAD_INT", 7) != 7 || String("MISSING", "fallback") != "fallback" {
		t.Fatal("fallback not returned")
	}
}

func TestRequiredPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	Required("DEFINITELY_MISSING")
}
