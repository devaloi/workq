package deadletter

import (
	"testing"

	"github.com/devaloi/workq/internal/domain"
)

func TestStore_AddAndList(t *testing.T) {
	t.Parallel()

	s := NewStore()
	j, _ := domain.NewJob("test", []byte("data"), 1)
	j.Error = "failed permanently"

	s.Add(j)

	if s.Len() != 1 {
		t.Fatalf("expected 1 dead job, got %d", s.Len())
	}

	jobs := s.List()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job in list, got %d", len(jobs))
	}
	if jobs[0].Status != domain.StatusDead {
		t.Fatalf("expected dead status, got %s", jobs[0].Status)
	}
}

func TestStore_Get(t *testing.T) {
	t.Parallel()

	s := NewStore()
	j, _ := domain.NewJob("test", nil, 1)
	s.Add(j)

	got, err := s.Get(j.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != j.ID {
		t.Fatalf("expected ID %s, got %s", j.ID, got.ID)
	}

	_, err = s.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent ID")
	}
}

func TestStore_Retry(t *testing.T) {
	t.Parallel()

	s := NewStore()
	j, _ := domain.NewJob("test", nil, 3)
	j.Attempts = 3
	j.Error = "max retries"
	s.Add(j)

	retried, err := s.Retry(j.ID)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if retried.Status != domain.StatusPending {
		t.Fatalf("expected pending status, got %s", retried.Status)
	}
	if retried.Attempts != 0 {
		t.Fatalf("expected attempts reset to 0, got %d", retried.Attempts)
	}
	if retried.Error != "" {
		t.Fatalf("expected error cleared, got %s", retried.Error)
	}
	if s.Len() != 0 {
		t.Fatalf("expected 0 dead jobs after retry, got %d", s.Len())
	}
}

func TestStore_RetryNotFound(t *testing.T) {
	t.Parallel()

	s := NewStore()
	_, err := s.Retry("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent ID")
	}
}

func TestStore_Purge(t *testing.T) {
	t.Parallel()

	s := NewStore()
	for i := 0; i < 5; i++ {
		j, _ := domain.NewJob("test", nil, 1)
		s.Add(j)
	}

	count := s.Purge()
	if count != 5 {
		t.Fatalf("expected 5 purged, got %d", count)
	}
	if s.Len() != 0 {
		t.Fatalf("expected 0 after purge, got %d", s.Len())
	}
}

func TestStore_ImmutableCopies(t *testing.T) {
	t.Parallel()

	s := NewStore()
	j, _ := domain.NewJob("test", nil, 1)
	s.Add(j)

	// Modify the original — store should be unaffected.
	j.Error = "modified"

	got, _ := s.Get(j.ID)
	if got.Error == "modified" {
		t.Fatal("store should hold independent copies")
	}
}
