package pbdbos

import "log/slog"

// Config holds configuration parameters for initializing the pbdbos plugin.
type Config struct {
	AppName            string       // Required: application name
	ApplicationVersion string       // Optional: defaults to binary hash
	ExecutorID         string       // Optional: defaults to "local"
	Logger             *slog.Logger // Optional: defaults to slog default
}
