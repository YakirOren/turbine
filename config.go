package turbine

import (
	"log/slog"
	"time"
)

// Config holds configuration parameters for initializing the turbine plugin.
type Config struct {
	ApplicationVersion string        // Optional: defaults to binary hash
	ExecutorID         string        // Optional: defaults to "local"
	GCRetention        time.Duration // How long to keep completed workflows. 0 = use default (72h). Negative = disabled.
	GCSchedule         string        // Cron expression for GC. Default: "0 0 * * *" (daily midnight)
	ProductSender      ProductSender // Optional: sender for dispatching products to external systems
	WebhookMaxRetries  int           // Max webhook delivery attempts. 0 = use default (3).
	WebhookTimeout     time.Duration // Timeout per webhook delivery attempt. 0 = use default (10s).
	ShutdownTimeout    time.Duration // Max drain duration on Shutdown. 0 = use default (30s).
	Logger             *slog.Logger  // Optional: overrides the default logger used by LoggerFrom and ctx.Logger().
}
