package retry

import (
	"math"
	"math/rand"
	"time"
)

// Backoff calculates exponential backoff delays with jitter.
type Backoff struct {
	Base     time.Duration
	Max      time.Duration
	JitterMax time.Duration
}

// DefaultBackoff returns a Backoff with sensible defaults.
func DefaultBackoff() *Backoff {
	return &Backoff{
		Base:      1 * time.Second,
		Max:       5 * time.Minute,
		JitterMax: 500 * time.Millisecond,
	}
}

// NextDelay returns the backoff delay for the given attempt number (0-indexed).
// The exponent is capped to prevent floating-point overflow at high attempt counts.
func (b *Backoff) NextDelay(attempt int) time.Duration {
	// Cap exponent at 62 to avoid float64 overflow (2^63 exceeds int64 range).
	const maxExponent = 62
	if attempt > maxExponent {
		attempt = maxExponent
	}

	exp := math.Pow(2, float64(attempt))
	delay := time.Duration(float64(b.Base) * exp)

	// Add jitter.
	if b.JitterMax > 0 {
		jitter := time.Duration(rand.Int63n(int64(b.JitterMax)))
		delay += jitter
	}

	// Cap at max.
	if b.Max > 0 && delay > b.Max {
		delay = b.Max
	}

	return delay
}
