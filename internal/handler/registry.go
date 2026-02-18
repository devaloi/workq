package handler

import (
	"context"
	"fmt"
	"sync"
)

// HandlerFunc processes a job payload.
type HandlerFunc func(ctx context.Context, payload []byte) error

// Registry maps job types to handler functions.
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]HandlerFunc
}

// NewRegistry creates an empty handler registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]HandlerFunc),
	}
}

// Register adds a handler for a job type.
func (r *Registry) Register(jobType string, h HandlerFunc) error {
	if jobType == "" {
		return fmt.Errorf("job type cannot be empty")
	}
	if h == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[jobType]; exists {
		return fmt.Errorf("handler already registered for type: %s", jobType)
	}

	r.handlers[jobType] = h
	return nil
}

// Lookup returns the handler for a job type.
func (r *Registry) Lookup(jobType string) (HandlerFunc, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	h, ok := r.handlers[jobType]
	if !ok {
		return nil, fmt.Errorf("no handler registered for type: %s", jobType)
	}
	return h, nil
}

// Types returns all registered job types.
func (r *Registry) Types() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.handlers))
	for t := range r.handlers {
		types = append(types, t)
	}
	return types
}
