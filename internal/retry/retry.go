// Package retry provides SQLite-aware retry with exponential backoff and
// panic-recovery helpers used by turbine's internal subsystems.
//
// Retry/RetryWithResult only retry transient SQLite locking errors. RecoverGoroutine
// and LogPanic are the standard defer guards for log-only goroutines.
package retry

import (
	"context"
	"log/slog"
	"math/rand"
	"runtime/debug"
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

type config struct {
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
	logger     *slog.Logger
}

type Option func(*config)

func WithLogger(l *slog.Logger) Option {
	return func(c *config) {
		c.logger = l
	}
}

func WithMaxRetries(n int) Option {
	return func(c *config) {
		c.maxRetries = n
	}
}

// Retry calls fn repeatedly when it returns a retryable SQLite error.
// By default it retries indefinitely (maxRetries = -1) until the context is cancelled.
func Retry(ctx context.Context, fn func() error, opts ...Option) error {
	cfg := &config{
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

// RetryWithResult is the generic version of Retry that returns a result.
func RetryWithResult[T any](ctx context.Context, fn func() (T, error), opts ...Option) (T, error) {
	var result T
	err := Retry(ctx, func() error {
		var e error
		result, e = fn()
		return e
	}, opts...)
	return result, err
}

// LogPanic logs a captured panic value with a stack trace and source=system tag.
// Use this when the caller already called recover() and needs the panic value
// for additional handling (e.g. propagating it on a channel).
func LogPanic(logger *slog.Logger, r any, msg string, fields ...any) {
	args := make([]any, 0, len(fields)+6)
	args = append(args, fields...)
	args = append(args, "panic", r, "stack", string(debug.Stack()), "source", "system")
	logger.Error(msg, args...)
}

// RecoverGoroutine is the standard defer guard for log-only goroutines. Use
// `defer retry.RecoverGoroutine(logger, "X goroutine panicked", "k", v)`. Sites that
// need the panic value (workflow main goroutine, DoAsync) inline their own
// recover and pass r to LogPanic.
func RecoverGoroutine(logger *slog.Logger, msg string, fields ...any) {
	if r := recover(); r != nil {
		LogPanic(logger, r, msg, fields...)
	}
}
