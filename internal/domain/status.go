package domain

import "fmt"

// Status represents the lifecycle state of a job.
type Status string

const (
	StatusPending   Status = "pending"
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusDead      Status = "dead"
)

// ValidTransitions defines allowed status transitions.
var ValidTransitions = map[Status][]Status{
	StatusPending:   {StatusActive},
	StatusActive:    {StatusCompleted, StatusFailed, StatusDead},
	StatusFailed:    {StatusPending, StatusDead},
}

// CanTransition checks if a status transition is allowed.
func CanTransition(from, to Status) bool {
	allowed, ok := ValidTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// ValidateTransition returns an error if the transition is not allowed.
func ValidateTransition(from, to Status) error {
	if !CanTransition(from, to) {
		return fmt.Errorf("invalid status transition: %s → %s", from, to)
	}
	return nil
}
