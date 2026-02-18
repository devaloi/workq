package handler

import (
	"context"
	"testing"
)

func TestRegistry_RegisterAndLookup(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	called := false
	h := func(_ context.Context, _ []byte) error {
		called = true
		return nil
	}

	if err := r.Register("email", h); err != nil {
		t.Fatalf("register: %v", err)
	}

	got, err := r.Lookup("email")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}

	_ = got(context.Background(), nil)
	if !called {
		t.Fatal("handler was not called")
	}
}

func TestRegistry_UnknownType(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	_, err := r.Lookup("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	h := func(_ context.Context, _ []byte) error { return nil }
	_ = r.Register("test", h)

	if err := r.Register("test", h); err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

func TestRegistry_EmptyType(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	h := func(_ context.Context, _ []byte) error { return nil }
	if err := r.Register("", h); err == nil {
		t.Fatal("expected error for empty type")
	}
}

func TestRegistry_NilHandler(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	if err := r.Register("test", nil); err == nil {
		t.Fatal("expected error for nil handler")
	}
}

func TestRegistry_Types(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	h := func(_ context.Context, _ []byte) error { return nil }
	_ = r.Register("a", h)
	_ = r.Register("b", h)

	types := r.Types()
	if len(types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(types))
	}
}
