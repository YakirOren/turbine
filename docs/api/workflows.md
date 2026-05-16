# Workflows API

## Constructors

Turbine has five entry points. They differ in who owns the PocketBase app and whether HTTP serving is involved. See [Lifecycle](../concepts/lifecycle.md) for the construct -> register -> launch flow and a decision matrix.

## `turbine.NewRuntime`

```go
func NewRuntime(app core.App, cfg Config) *Runtime
```

The primitive. The caller owns the app and manually invokes `rt.Launch()` and `rt.Shutdown()`. Use when you need fine-grained control over lifecycle, or for tests.

## `turbine.Setup`

```go
func Setup(app core.App, cfg Config) *Runtime
```

HTTP path, bring-your-own app. Binds `rt.Launch` to `app.OnServe()` and `rt.Shutdown` to `app.OnTerminate()`, so calling `app.Start()` drives the runtime. Use when writing a PocketBase plugin that ships alongside an existing app.

## `turbine.NewApp`

```go
func NewApp(cfg Config) (App, *Runtime)
```

HTTP path, owned app. Creates a fresh PocketBase via `pocketbase.New()` and wires it through `Setup`. Returns both the app and the runtime. Use when you want HTTP serving without writing the app-construction boilerplate.

## `turbine.NewStandalone`

```go
func NewStandalone(cfg Config) *Runtime
```

Standalone path, owned app. Creates a fresh PocketBase, but the runtime owns its lifecycle (no HTTP, no `app.Start()`). Defaults `cfg.Logger` to a stdout `slog` handler. The caller is responsible for `rt.Launch()` and `rt.Shutdown()`. Use for scripts, cron jobs, and background workers that need to run workflows without serving HTTP.

## `turbine.SetupStandalone`

```go
func SetupStandalone(app core.App, cfg Config) *Runtime
```

Standalone path, bring-your-own app. Like `NewStandalone` but the caller supplies the PocketBase app. Does NOT default `cfg.Logger`, so caller-configured `app.Logger()` is preserved. Use when you have a PocketBase app you constructed yourself, and you want to run workflows without serving HTTP.

## Type Aliases

| Type | Description |
|---|---|
| `turbine.App` | The underlying app type. Provides HTTP routing, persistence, lifecycle, and more. |
| `turbine.ServeEvent` | Event fired when the HTTP server is starting. |
| `turbine.RequestEvent` | Represents an HTTP request received by the server. |

## `turbine.Register`

```go
func Register[P, R any](rt *Runtime, fn Workflow[P, R], opts ...WorkflowRegistrationOption)
```

Register a workflow function. Options:

| Option | Description |
|---|---|
| `WithName(name)` | Custom workflow name (default: function name) |
| `WithMaxRetries(n)` | Max recovery attempts (default: 100) |
| `WithSchedule(cron)` | Cron expression for scheduled execution |
| `WithDashboardTrigger()` | Enable triggering from the dashboard UI |
| `WithTags(tags...)` | Tags for filtering in the dashboard |
| `WithInputSchema(schema)` | Input schema for typed dashboard forms |
| `WithSummaryFunc[P](fn)` | Generate a one-line summary from the input (max 200 chars) |

## `turbine.Run`

```go
func Run[P, R any](rt *Runtime, fn Workflow[P, R], input P, opts ...WorkflowOption) (Handle[R], error)
```

Start a workflow. Options:

| Option | Description |
|---|---|
| `WithID(id)` | Deterministic workflow ID |
| `WithQueue(name)` | Enqueue to a named queue |
| `WithDeduplicationID(id)` | Prevent duplicate enqueues |
| `WithPriority(n)` | Priority (lower = higher) |
| `WithQueuePartitionKey(key)` | Partition key for partitioned queues |
| `WithTimeout(duration)` | Cancel after duration |
| `WithDeadline(time)` | Cancel at specific time |
| `WithApplicationVersion(v)` | Override app version for this workflow |

## `turbine.Retrieve`

```go
func Retrieve[R any](rt *Runtime, workflowID string) Handle[R]
```

Get a handle to an existing workflow by ID.

## `Handle[R]`

| Method | Description |
|---|---|
| `GetResult(...GetResultOption) (R, error)` | Block until workflow completes, return result |
| `GetStatus() (Status, error)` | Get current workflow status |
| `GetWorkflowID() string` | Get the workflow ID |

## `Runtime`

| Method | Description |
|---|---|
| `Cancel(id)` | Cancel a running workflow |
| `Resume(id)` | Resume a paused workflow |
| `Steps(id)` | Get step details for a workflow |
| `Queue(name, ...opts)` | Create or get a named queue |
| `Listen(queue)` | Start processing a queue (multi-instance) |
| `Queues()` | List all registered queues |
| `RegisteredWorkflows()` | List all registered workflows |
| `NewContext(ctx)` | Wrap a `context.Context` for use with Turbine APIs |
| `SendToWorkflow(id, msg, topic)` | Send a message to a workflow from outside |
| `TriggerByFQN(fqn, input)` | Trigger a workflow by fully-qualified name |
| `IsTriggerable(fqn)` | Check if a workflow can be triggered |
| `KVSet(ctx, key, value)` | Set a [KV store](/concepts/kv-store) value |
| `KVDelete(ctx, key)` | Delete a KV store key |
| `GarbageCollect()` | Manually run garbage collection |
| `SetProductSender(sender)` | Set the [product sender](/concepts/products) |
| `IsLaunched()` | Check if runtime is running |
| `IsDraining()` | Check if runtime is shutting down |
| `GetExecutorID()` | Get instance identifier |
| `GetApplicationVersion()` | Get app version |
| `GetApplicationID()` | Get app name |
| `App()` | Get the `core.App` |

## `GetResultOption`

| Option | Description |
|---|---|
| `WithHandlePollingInterval(d)` | Custom polling interval for enqueued workflows (default: 500ms) |

## Free Functions

| Function | Description |
|---|---|
| `turbine.KVGet[R](rt, ctx, key)` | Get a value from the [KV store](/concepts/kv-store) |
| `turbine.FromContext(ctx)` | Extract a `turbine.Context` from a `context.Context` |
