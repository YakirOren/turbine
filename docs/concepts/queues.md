# Queues

Concurrency control, rate limiting, priority, and partitioned processing for workflows.

## Creating a Queue

```go
q := rt.Queue("emails",
    turbine.WithWorkerConcurrency(5),
    turbine.WithGlobalConcurrency(10),
    turbine.WithRateLimiter(turbine.RateLimiter{Limit: 100, Period: time.Minute}),
    turbine.WithPriorityEnabled(),
    turbine.WithPartitionQueue(),
)
```

### Queue Options

| Option | Description |
|---|---|
| `WithWorkerConcurrency(n)` | Max concurrent workflows this instance will run from this queue |
| `WithGlobalConcurrency(n)` | Max concurrent workflows across all instances (enforced at dequeue time) |
| `WithRateLimiter(rl)` | Limit how many workflows are dequeued per time period |
| `WithPriorityEnabled()` | Dequeue lower-priority-number workflows first |
| `WithPartitionQueue()` | Process workflows with the same partition key sequentially |
| `WithMaxTasksPerIteration(n)` | Max workflows dequeued per poll cycle (default: 100) |

## Enqueuing Workflows

```go
turbine.Run(rt, sendEmail, recipient, turbine.WithQueue("emails"))
```

You can also set priority, deduplication, and partition keys:

```go
turbine.Run(rt, sendEmail, recipient,
    turbine.WithQueue("emails"),
    turbine.WithPriority(1),                       // lower = higher priority
    turbine.WithDeduplicationID("user-123"),        // prevent duplicates
    turbine.WithQueuePartitionKey("tenant-1"),      // partition key
)
```

## Partitioned Queues

With `WithPartitionQueue()`, workflows sharing the same `QueuePartitionKey` run sequentially. Different partition keys run concurrently.

```go
rt.Queue("orders", turbine.WithPartitionQueue())

// These run sequentially (same partition key)
turbine.Run(rt, processOrder, order1, turbine.WithQueue("orders"), turbine.WithQueuePartitionKey("tenant-A"))
turbine.Run(rt, processOrder, order2, turbine.WithQueue("orders"), turbine.WithQueuePartitionKey("tenant-A"))

// This runs concurrently with the above (different partition key)
turbine.Run(rt, processOrder, order3, turbine.WithQueue("orders"), turbine.WithQueuePartitionKey("tenant-B"))
```

## Multi-Instance Setup

::: info
By default, every instance processes all queues. Use `Listen` to restrict which queues a specific instance handles.
:::

```go
rt.Listen(q)
```

See the [queue example](/examples/queue) for a working demo.
