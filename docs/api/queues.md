# Queues API

## `Runtime.Queue`

```go
func (rt *Runtime) Queue(name string, opts ...QueueOption) WorkflowQueue
```

Create or get a named queue. Options:

| Option | Description |
|---|---|
| `WithWorkerConcurrency(n)` | Max concurrent workers per instance |
| `WithGlobalConcurrency(n)` | Max concurrent workers across all instances |
| `WithRateLimiter(rl)` | Rate limit configuration |
| `WithPriorityEnabled()` | Enable priority-based ordering |
| `WithPartitionQueue()` | Enable partitioned processing (see below) |
| `WithMaxTasksPerIteration(n)` | Max workflows dequeued per poll cycle (default: 100) |

## `RateLimiter`

```go
type RateLimiter struct {
    Limit  int
    Period time.Duration
}
```

## `Runtime.Listen`

```go
func (rt *Runtime) Listen(queues ...WorkflowQueue)
```

Start processing workflows from a queue. Accepts multiple queues. Use in multi-instance setups to control which queues each instance handles.

## `Runtime.Queues`

```go
func (rt *Runtime) Queues() []WorkflowQueue
```

Returns all registered queues.

## Partitioned Queues

When `WithPartitionQueue()` is enabled, workflows with the same `QueuePartitionKey` are processed sequentially, only one workflow per partition key runs at a time. Different partition keys run concurrently.

This is useful for per-tenant or per-resource ordering:

```go
rt.Queue("orders", turbine.WithPartitionQueue())

// These run sequentially (same partition key)
turbine.Run(rt, processOrder, order1, turbine.WithQueue("orders"), turbine.WithQueuePartitionKey("tenant-A"))
turbine.Run(rt, processOrder, order2, turbine.WithQueue("orders"), turbine.WithQueuePartitionKey("tenant-A"))

// This runs concurrently with the above (different partition key)
turbine.Run(rt, processOrder, order3, turbine.WithQueue("orders"), turbine.WithQueuePartitionKey("tenant-B"))
```
