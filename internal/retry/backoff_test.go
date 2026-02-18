package retry

import (
	"testing"
	"time"
)

func TestBackoff_ExponentialGrowth(t *testing.T) {
	t.Parallel()

	b := &Backoff{
		Base:      1 * time.Second,
		Max:       5 * time.Minute,
		JitterMax: 0, // no jitter for deterministic test
	}

	expected := []time.Duration{
		1 * time.Second,  // 2^0 = 1
		2 * time.Second,  // 2^1 = 2
		4 * time.Second,  // 2^2 = 4
		8 * time.Second,  // 2^3 = 8
		16 * time.Second, // 2^4 = 16
	}

	for i, want := range expected {
		got := b.NextDelay(i)
		if got != want {
			t.Errorf("attempt %d: expected %v, got %v", i, want, got)
		}
	}
}

func TestBackoff_MaxCap(t *testing.T) {
	t.Parallel()

	b := &Backoff{
		Base:      1 * time.Second,
		Max:       10 * time.Second,
		JitterMax: 0,
	}

	// Attempt 10: 2^10 = 1024s, should be capped at 10s.
	got := b.NextDelay(10)
	if got != 10*time.Second {
		t.Fatalf("expected max cap 10s, got %v", got)
	}
}

func TestBackoff_JitterWithinRange(t *testing.T) {
	t.Parallel()

	b := &Backoff{
		Base:      1 * time.Second,
		Max:       5 * time.Minute,
		JitterMax: 500 * time.Millisecond,
	}

	// Run multiple times, all should be within [base, base+jitterMax].
	for i := 0; i < 100; i++ {
		got := b.NextDelay(0)
		if got < 1*time.Second || got > 1*time.Second+500*time.Millisecond {
			t.Fatalf("attempt 0: delay %v out of range [1s, 1.5s]", got)
		}
	}
}

func TestBackoff_DefaultValues(t *testing.T) {
	t.Parallel()

	b := DefaultBackoff()
	if b.Base != 1*time.Second {
		t.Fatalf("expected base 1s, got %v", b.Base)
	}
	if b.Max != 5*time.Minute {
		t.Fatalf("expected max 5m, got %v", b.Max)
	}
	if b.JitterMax != 500*time.Millisecond {
		t.Fatalf("expected jitter max 500ms, got %v", b.JitterMax)
	}
}
