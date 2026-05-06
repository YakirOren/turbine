# Steps API

## `turbine.Do`

```go
func Do[R any](ctx turbine.Context, fn func(context.Context) (R, error), opts ...StepOption) (R, error)
```

Execute a durable step. The result is recorded in SQLite and replayed on recovery.

Options:

| Option | Description |
|---|---|
| `WithStepName(name)` | Name for the step |
| `WithStepMaxRetries(n)` | Max retry attempts |
| `WithBackoffFactor(f)` | Exponential backoff multiplier |
| `WithBaseInterval(d)` | Initial delay between retries |
| `WithMaxInterval(d)` | Maximum delay between retries |
| `WithoutCheckpoint()` | Skip persisting result — step re-executes on recovery ([details](/concepts/checkpoints)) |

## `turbine.DoAsync`

```go
func DoAsync[R any](ctx turbine.Context, fn Step[R], opts ...StepOption) (chan AsyncResult[R], error)
```

Execute a step asynchronously. Returns a channel that receives the result.

## `turbine.Pause`

```go
func Pause(ctx turbine.Context, duration time.Duration) error
```

Durable pause. Survives crashes — on recovery, pauses only the remaining duration.

## `turbine.WaitForApproval`

```go
func WaitForApproval(ctx turbine.Context, opts ...ApprovalOption) (ApprovalResult, error)
```

Block until an approval or rejection is received (via dashboard or API). See [Approvals](/concepts/approvals).

Options:

| Option | Description |
|---|---|
| `WithApprovalTimeout(d)` | Max wait duration (default: indefinite) |

## Context Helpers

```go
turbine.LoggerFrom(ctx context.Context) *slog.Logger
turbine.AppFrom(ctx context.Context) core.App
```

Access the logger and PocketBase app from within step functions.

## `turbine.SetAppStatus`

```go
func SetAppStatus(ctx context.Context, label, color string) error
```

Set [app status](/concepts/app-status) from within a step.

## `turbine.SendProduct`

```go
func SendProduct(ctx context.Context, fileName string, data io.Reader, metadata map[string]any) error
```

Store a [product file](/concepts/products) from within a step.

## `turbine.SendNotification`

```go
func SendNotification(ctx context.Context, name, message string) error
```

Send an ad-hoc message to a configured [notification channel](/concepts/notifications) by name. Disabled channels are a silent no-op. Also available as `rt.SendNotification(name, message)` from host code.
