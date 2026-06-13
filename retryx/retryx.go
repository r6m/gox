// Package retryx provides pure retry backoff calculations.
package retryx

import (
	"errors"
	"math"
	"time"
)

// Config controls exponential backoff.
type Config struct {
	BaseDelay time.Duration
	Factor    float64
	MaxDelay  time.Duration
	// Jitter is a fraction in the range [0, 1]. A value of 0.2 applies a
	// multiplier in the range [0.8, 1.2].
	Jitter float64
}

// Delay calculates the delay for a zero-based attempt. random must return a
// value in [0, 1] when jitter is enabled; nil uses the midpoint and therefore
// applies no random adjustment.
func Delay(cfg Config, attempt int, random func() float64) (time.Duration, error) {
	if cfg.BaseDelay < 0 || cfg.MaxDelay < 0 {
		return 0, errors.New("retryx: delays must be non-negative")
	}
	if cfg.Factor <= 0 {
		return 0, errors.New("retryx: factor must be positive")
	}
	if cfg.Jitter < 0 || cfg.Jitter > 1 {
		return 0, errors.New("retryx: jitter must be between 0 and 1")
	}
	if attempt < 0 {
		return 0, errors.New("retryx: attempt must be non-negative")
	}

	delay := float64(cfg.BaseDelay) * math.Pow(cfg.Factor, float64(attempt))
	if cfg.Jitter > 0 {
		sample := 0.5
		if random != nil {
			sample = random()
		}
		if sample < 0 || sample > 1 {
			return 0, errors.New("retryx: random value must be between 0 and 1")
		}
		delay *= 1 - cfg.Jitter + (2 * cfg.Jitter * sample)
	}
	if cfg.MaxDelay > 0 && delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}
	if delay > float64(math.MaxInt64) {
		return time.Duration(math.MaxInt64), nil
	}
	return time.Duration(delay), nil
}
