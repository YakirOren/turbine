package turbine

import (
	"time"
)

// Config holds configuration parameters for initializing the turbine plugin.
type Config struct {
	ApplicationVersion string        // Optional: defaults to binary hash
	ExecutorID         string        // Optional: defaults to "local"
	GCRetention        time.Duration // How long to keep completed workflows. 0 = use default (72h). Negative = disabled.
	GCSchedule         string        // Cron expression for GC. Default: "0 0 * * *" (daily midnight)
	ProductSender      ProductSender // Optional: sender for dispatching products to external systems
}
