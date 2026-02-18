package config

import (
	"os"
	"strconv"
	"time"
)

// Default configuration values.
const (
	DefaultConcurrency     = 4
	DefaultMaxRetries      = 5
	DefaultBackoffBase     = 1 * time.Second
	DefaultBackoffMax      = 5 * time.Minute
	DefaultJitterMax       = 500 * time.Millisecond
	DefaultShutdownTimeout = 30 * time.Second
)

// Config holds all workq configuration.
type Config struct {
	Concurrency    int           // number of worker goroutines
	MaxRetries     int           // max attempts before dead letter
	BackoffBase    time.Duration // base delay for exponential backoff
	BackoffMax     time.Duration // maximum backoff delay
	JitterMax      time.Duration // max jitter added to backoff
	PersistPath    string        // file path for JSON persistence ("" = disabled)
	ShutdownTimeout time.Duration // max wait for in-flight jobs on shutdown
}

// Default returns a Config with sensible defaults.
func Default() Config {
	return Config{
		Concurrency:     DefaultConcurrency,
		MaxRetries:      DefaultMaxRetries,
		BackoffBase:     DefaultBackoffBase,
		BackoffMax:      DefaultBackoffMax,
		JitterMax:       DefaultJitterMax,
		PersistPath:     "",
		ShutdownTimeout: DefaultShutdownTimeout,
	}
}

// FromEnv loads config from environment variables, falling back to defaults.
func FromEnv() Config {
	cfg := Default()

	if v := os.Getenv("WORKQ_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Concurrency = n
		}
	}
	if v := os.Getenv("WORKQ_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.MaxRetries = n
		}
	}
	if v := os.Getenv("WORKQ_BACKOFF_BASE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.BackoffBase = d
		}
	}
	if v := os.Getenv("WORKQ_BACKOFF_MAX"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.BackoffMax = d
		}
	}
	if v := os.Getenv("WORKQ_JITTER_MAX"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.JitterMax = d
		}
	}
	if v := os.Getenv("WORKQ_PERSIST_PATH"); v != "" {
		cfg.PersistPath = v
	}
	if v := os.Getenv("WORKQ_SHUTDOWN_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ShutdownTimeout = d
		}
	}

	return cfg
}
