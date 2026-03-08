package pbdbos

import (
	"log/slog"
	"time"
)

// Config holds configuration parameters for initializing the pbdbos plugin.
type Config struct {
	ApplicationVersion string        // Optional: defaults to binary hash
	ExecutorID         string        // Optional: defaults to "local"
	Logger             *slog.Logger  // Optional: defaults to slog default
	GCRetention        time.Duration // How long to keep completed workflows. 0 = use default (72h). Negative = disabled.
	GCSchedule         string        // Cron expression for GC. Default: "0 0 * * *" (daily midnight)
}
