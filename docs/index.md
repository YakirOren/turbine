# What is Turbine?

Turbine is a durable workflow engine for [PocketBase](https://pocketbase.io). It runs entirely on SQLite — no external dependencies.

You define workflows as Go functions. Turbine handles persistence, retries, scheduling, and recovery.

## Install

```bash
go get github.com/YakirOren/turbine
```

## Use Cases

### Background Jobs

Enqueue work to run asynchronously with concurrency control and rate limiting.

```go
handle, err := turbine.Run(rt, SendEmail, recipient,
    turbine.WithQueue("emails"),
)
```

### Multi-Step Pipelines

Each step checkpoints its result. If the process crashes, it resumes from the last completed step.

```go
func OrderWorkflow(ctx turbine.Context, orderID string) (string, error) {
    charge, err := turbine.Do(ctx, ChargePayment, turbine.WithStepName("charge"))
    if err != nil {
        return "", err
    }

    _, err = turbine.Do(ctx, ShipOrder, turbine.WithStepName("ship"))
    if err != nil {
        return "", err
    }

    return fmt.Sprintf("order %s shipped (charge: %s)", orderID, charge), nil
}
```

### Scheduled Tasks

Run workflows on a cron schedule using PocketBase's built-in scheduler.

```go
turbine.Register(rt, DailyCleanup, turbine.WithSchedule("0 3 * * *"))
```

### Human-in-the-Loop

Pause a workflow and wait for a human decision. Survives crashes and restarts.

```go
result, err := turbine.WaitForApproval(ctx)
if !result.Approved {
    return "rejected: " + result.Comment, nil
}
```

### Rate-Limited Queues

Control concurrency, enforce rate limits, and partition work across tenants.

```go
rt.Queue("api-calls",
    turbine.WithWorkerConcurrency(5),
    turbine.WithRateLimiter(turbine.RateLimiter{Limit: 100, Period: time.Minute}),
    turbine.WithPartitionQueue(),
)
```

## Features

| Feature | Description |
|---|---|
| [Durable Steps](/concepts/steps) | Step results recorded in SQLite, replayed on recovery |
| [Step Retries](/concepts/steps#step-retries) | Automatic retries with exponential backoff |
| [Queues](/concepts/queues) | Concurrency control, priority, rate limiting, partitioning |
| [Scheduling](/concepts/scheduling) | Cron expressions via PocketBase's built-in scheduler |
| [Communication](/concepts/communication) | Inter-workflow messaging and key-value events |
| [Approvals](/concepts/approvals) | Human-in-the-loop approval gates |
| [Products](/concepts/products) | File outputs from workflows |
| [KV Store](/concepts/kv-store) | Global persistent key-value storage |
| [Webhooks](/concepts/webhooks) | HTTP notifications on workflow completion |
| [Dashboard](/examples/dashboard) | Built-in UI for monitoring workflows, queues, and schedules |
