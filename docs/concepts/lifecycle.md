# Lifecycle

## Setup vs New + Launch

`Setup` hooks into PocketBase's lifecycle automatically:

```go
rt := turbine.Setup(app, turbine.Config{})
// Register workflows...
// app.Start() launches Turbine automatically
```

For manual control:

```go
rt := turbine.New(app, turbine.Config{})
// Register workflows...
rt.Launch()          // start runtime, recover pending workflows
defer rt.Shutdown(30 * time.Second)
```

## Shutdown

Two-phase process:

1. **Drain** — stop accepting new work, let running workflows finish naturally
2. **Force** — if the timeout expires, cancel all remaining workflows

```go
rt.Shutdown(30 * time.Second)
```

During shutdown:
- `rt.IsDraining()` returns `true`
- Queue runners stop dequeuing
- New `Run` calls return `turbine.ErrShuttingDown`
- Running workflows get their context cancelled after the timeout

::: info
`Setup` uses a hardcoded 30-second shutdown timeout. For custom timeouts, use `New` + `Launch` + `Shutdown`.
:::

## Runtime State

| Method | Returns |
|---|---|
| `rt.IsLaunched()` | `true` after `Launch()`, `false` after `Shutdown()` |
| `rt.IsDraining()` | `true` during shutdown drain phase |
| `rt.GetExecutorID()` | Instance identifier (default: `"local"`) |
| `rt.GetApplicationVersion()` | App version (default: binary hash) |
| `rt.GetApplicationID()` | PocketBase app name |
| `rt.App()` | The PocketBase `core.App` |

## Introspection

### Registered Workflows

```go
workflows := rt.RegisteredWorkflows()
for _, wf := range workflows {
    fmt.Printf("%s (triggerable=%v, cron=%s, tags=%v)\n",
        wf.Name, wf.Triggerable, wf.CronSchedule, wf.Tags)
}
```

Returns `RegisteredWorkflow` with fields: `Name`, `FQN`, `Triggerable`, `CronSchedule`, `InputSchema`, `Tags`.

### Registered Queues

```go
queues := rt.Queues()
for _, q := range queues {
    fmt.Printf("queue %s (workers=%v, priority=%v)\n",
        q.Name, q.WorkerConcurrency, q.PriorityEnabled)
}
```

### Workflow Steps

```go
steps, err := rt.Steps(workflowID)
for _, s := range steps {
    fmt.Printf("step %d: %s (output=%s)\n", s.FunctionID, s.FunctionName, s.Output)
}
```

## Workflow Status Types

| Status | Meaning |
|---|---|
| `PENDING` | Workflow started, running or waiting for recovery |
| `ENQUEUED` | Waiting in a queue to be picked up |
| `SUCCESS` | Completed successfully |
| `ERROR` | Failed with an error |
| `CANCELLED` | Cancelled via `rt.Cancel()` |
| `MAX_RECOVERY_ATTEMPTS_EXCEEDED` | Exceeded max recovery retries (dead letter) |
