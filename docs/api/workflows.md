# Workflows API

## `turbine.Setup`

```go
func Setup(app core.App, config Config) *Runtime
```

Initialize Turbine with a PocketBase app and configuration. Creates required SQLite collections automatically.

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
| `GetApplicationID()` | Get PocketBase app name |
| `App()` | Get the PocketBase `core.App` |

## `GetResultOption`

| Option | Description |
|---|---|
| `WithHandlePollingInterval(d)` | Custom polling interval for enqueued workflows (default: 500ms) |

## Free Functions

| Function | Description |
|---|---|
| `turbine.KVGet[R](rt, ctx, key)` | Get a value from the [KV store](/concepts/kv-store) |
| `turbine.FromContext(ctx)` | Extract a `turbine.Context` from a `context.Context` |
