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

	// AllowPrivateAddresses opts out of the SSRF guard that blocks outbound webhook
	// and alert-channel delivery to loopback / link-local / RFC1918 / CGNAT
	// addresses. Leave false in any multi-tenant or internet-exposed deployment.
	// Set true only when webhook receivers legitimately run on the same host or
	// inside a trusted private network. Also enables localhost in tests.
	AllowPrivateAddresses bool
}
