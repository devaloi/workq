package domain

import (
	"testing"
	"time"
)

func TestNewJob(t *testing.T) {
	t.Parallel()

	t.Run("valid job", func(t *testing.T) {
		t.Parallel()
		j, err := NewJob("email_send", []byte(`{"to":"a@b.com"}`), 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if j.ID == "" {
			t.Fatal("expected non-empty ID")
		}
		if j.Type != "email_send" {
			t.Fatalf("expected type email_send, got %s", j.Type)
		}
		if j.Status != StatusPending {
			t.Fatalf("expected status pending, got %s", j.Status)
		}
		if j.MaxAttempts != 3 {
			t.Fatalf("expected max attempts 3, got %d", j.MaxAttempts)
		}
		if j.Attempts != 0 {
			t.Fatalf("expected 0 attempts, got %d", j.Attempts)
		}
		if j.CreatedAt.IsZero() {
			t.Fatal("expected non-zero CreatedAt")
		}
	})

	t.Run("empty type", func(t *testing.T) {
		t.Parallel()
		_, err := NewJob("", nil, 3)
		if err == nil {
			t.Fatal("expected error for empty type")
		}
	})

	t.Run("invalid max attempts", func(t *testing.T) {
		t.Parallel()
		_, err := NewJob("test", nil, 0)
		if err == nil {
			t.Fatal("expected error for max attempts < 1")
		}
	})
}

func TestJobTransition(t *testing.T) {
	t.Parallel()

	t.Run("pending to active", func(t *testing.T) {
		t.Parallel()
		j, _ := NewJob("test", nil, 1)
		if err := j.TransitionTo(StatusActive); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if j.Status != StatusActive {
			t.Fatalf("expected active, got %s", j.Status)
		}
	})

	t.Run("active to completed sets CompletedAt", func(t *testing.T) {
		t.Parallel()
		j, _ := NewJob("test", nil, 1)
		_ = j.TransitionTo(StatusActive)
		before := time.Now()
		if err := j.TransitionTo(StatusCompleted); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if j.CompletedAt.Before(before) {
			t.Fatal("CompletedAt should be set to current time")
		}
	})

	t.Run("invalid transition", func(t *testing.T) {
		t.Parallel()
		j, _ := NewJob("test", nil, 1)
		if err := j.TransitionTo(StatusCompleted); err == nil {
			t.Fatal("expected error for pending → completed")
		}
	})

	t.Run("active to dead", func(t *testing.T) {
		t.Parallel()
		j, _ := NewJob("test", nil, 1)
		_ = j.TransitionTo(StatusActive)
		if err := j.TransitionTo(StatusDead); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
