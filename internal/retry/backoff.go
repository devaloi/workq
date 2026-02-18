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
func (b *Backoff) NextDelay(attempt int) time.Duration {
	exp := math.Pow(2, float64(attempt))
	delay := time.Duration(float64(b.Base) * exp)

	// Add jitter.
	if b.JitterMax > 0 {
		jitter := time.Duration(rand.Int63n(int64(b.JitterMax)))
		delay += jitter
	}

	// Cap at max.
	if delay > b.Max {
		delay = b.Max
	}

	return delay
}
