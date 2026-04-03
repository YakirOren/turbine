# KV Store

Global key-value store backed by SQLite. Unlike [events](/concepts/communication) which are scoped to a workflow, the KV store is accessible from anywhere — workflows, steps, and HTTP handlers.

## Set a Value

```go
err := rt.KVSet(ctx, "feature-flags/dark-mode", true)
```

Values are JSON-serialized. Any type that can be marshaled to JSON works.

## Get a Value

```go
enabled, exists, err := turbine.KVGet[bool](rt, ctx, "feature-flags/dark-mode")
if !exists {
    // key not found
}
```

Returns `(zero, false, nil)` if the key doesn't exist.

## Delete a Key

```go
err := rt.KVDelete(ctx, "feature-flags/dark-mode")
```

No-op if the key doesn't exist.

## Shared Configuration

You can use the KV store for configuration that workflows read at runtime without passing it as input to every execution:

```go
type EmailConfig struct {
    APIUrl      string `json:"api_url"`
    FromAddress string `json:"from_address"`
    Enabled     bool   `json:"enabled"`
    Environment string `json:"environment"`
    MaxRetries  int    `json:"max_retries"`
}

// Set once (e.g., from an HTTP handler or on startup)
rt.KVSet(ctx, "config/email", EmailConfig{
    APIUrl:      "https://api.sendgrid.com/v3/mail/send",
    FromAddress: "noreply@example.com",
    Enabled:     true,
    Environment: "production",
    MaxRetries:  5,
})

// Read from any workflow step
cfg, exists, err := turbine.KVGet[EmailConfig](rt, ctx, "config/email")
if exists {
    fmt.Println(cfg.APIUrl, cfg.MaxRetries)
}
```

You can update these values at runtime without restarting or re-enqueuing workflows.

## When to Use KV vs Events

| | KV Store | Events |
|---|---|---|
| **Scope** | Global | Per-workflow |
| **Access** | Anywhere (HTTP handlers, workflows, steps) | Within workflows (`SetValue`/`GetValue`) |
| **Use case** | Feature flags, config, shared state | Workflow-to-workflow signaling |
| **Blocking** | No | `GetValue` blocks until set or timeout |

::: tip
Keys are plain strings — Turbine does not namespace them. Use conventions like `"feature-flags/dark-mode"` or `"config/max-retries"` to organize your keys.
:::

::: info
The KV store is persistent across restarts — it's backed by the same SQLite database as everything else. Values are JSON-serialized and stored in the `pt_kv` collection.
:::
