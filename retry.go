package turbine

import (
	"context"
	"log/slog"
	"math/rand"
	"strings"
	"time"
)

// isSQLiteRetryable returns true if the error is a transient SQLite error worth retrying.
func isSQLiteRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "database table is locked") ||
		strings.Contains(msg, "SQLITE_BUSY")
}

type retryConfig struct {
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
	logger     *slog.Logger
}

type retryOption func(*retryConfig)

func withRetrierLogger(l *slog.Logger) retryOption {
	return func(c *retryConfig) {
		c.logger = l
	}
}

func withMaxRetries(n int) retryOption {
	return func(c *retryConfig) {
		c.maxRetries = n
	}
}

// retry calls fn repeatedly when it returns a retryable SQLite error.
// By default it retries indefinitely (maxRetries = -1) until the context is cancelled.
func retry(ctx context.Context, fn func() error, opts ...retryOption) error {
	cfg := &retryConfig{
		maxRetries: -1,
		baseDelay:  100 * time.Millisecond,
		maxDelay:   5 * time.Second,
	}
	for _, o := range opts {
		o(cfg)
	}

	delay := cfg.baseDelay
	attempt := 0
	for {
		err := fn()
		if err == nil {
			return nil
		}
		if !isSQLiteRetryable(err) {
			return err
		}
		if cfg.maxRetries >= 0 && attempt >= cfg.maxRetries {
			return err
		}
		if cfg.logger != nil {
			cfg.logger.Debug("retrying operation", "attempt", attempt+1, "delay", delay, "error", err)
		}

		jitter := 0.95 + rand.Float64()*0.1 // #nosec G404
		select {
		case <-time.After(time.Duration(float64(delay) * jitter)):
		case <-ctx.Done():
			return ctx.Err()
		}
		delay = min(time.Duration(float64(delay)*2.0), cfg.maxDelay)
		attempt++
	}
}

// retryWithResult is the generic version of retry that returns a result.
func retryWithResult[T any](ctx context.Context, fn func() (T, error), opts ...retryOption) (T, error) {
	var result T
	err := retry(ctx, func() error {
		var e error
		result, e = fn()
		return e
	}, opts...)
	return result, err
}
