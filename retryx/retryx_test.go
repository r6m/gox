package retryx

import (
	"testing"
	"time"
)

func TestDelay(t *testing.T) {
	cfg := Config{BaseDelay: time.Second, Factor: 2, MaxDelay: 5 * time.Second}
	wants := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second}
	for attempt, want := range wants {
		got, err := Delay(cfg, attempt, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("attempt %d: got %s, want %s", attempt, got, want)
		}
	}
}

func TestJitter(t *testing.T) {
	cfg := Config{BaseDelay: time.Second, Factor: 2, Jitter: 0.2}
	low, err := Delay(cfg, 0, func() float64 { return 0 })
	if err != nil {
		t.Fatal(err)
	}
	high, err := Delay(cfg, 0, func() float64 { return 1 })
	if err != nil {
		t.Fatal(err)
	}
	if low != 800*time.Millisecond || high != 1200*time.Millisecond {
		t.Fatalf("unexpected jitter range: %s %s", low, high)
	}
}

func TestInvalidConfig(t *testing.T) {
	if _, err := Delay(Config{BaseDelay: time.Second}, 0, nil); err == nil {
		t.Fatal("expected factor error")
	}
	if _, err := Delay(Config{Factor: 2, Jitter: 2}, 0, nil); err == nil {
		t.Fatal("expected jitter error")
	}
}
