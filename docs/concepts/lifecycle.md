# Lifecycle

## Choosing a constructor

Turbine exposes five entry points. Pick by answering two questions: **who owns the PocketBase app** and **does the app serve HTTP**.

| I want to... | Use this constructor |
|---|---|
| Build a PocketBase plugin and let the app drive everything (HTTP server, OnServe, OnTerminate) | `turbine.Setup(app, cfg)` |
| Quickly stand up a turbine-backed app with HTTP serving and no app-construction boilerplate | `app, rt := turbine.NewApp(cfg)` |
| Run workflows from a script, cron job, or background worker, no HTTP server | `rt := turbine.NewStandalone(cfg)` |
| Run workflows alongside a PocketBase app I configured myself, but without serving HTTP | `rt := turbine.SetupStandalone(app, cfg)` |
| Embed turbine into a custom lifecycle where I manage Launch and Shutdown by hand | `rt := turbine.NewRuntime(app, cfg)` |

**Lifecycle ordering.** Every constructor returns a runtime in the **unlaunched** state. The required order is:

1. Construct the runtime via one of the five entry points
2. `turbine.Register(rt, MyWorkflow)`, register every workflow you want to run
3. Start the runtime:
   - For `Setup` and `NewApp`: call `app.Start()`, which fires `OnServe` and triggers `rt.Launch()` for you
   - For `NewStandalone`, `SetupStandalone`, and `NewRuntime`: call `rt.Launch()` yourself
4. Use the runtime
5. Shut it down:
   - For `Setup` and `NewApp`: `OnTerminate` calls `rt.Shutdown()` for you when the app exits
   - For `NewStandalone`, `SetupStandalone`, and `NewRuntime`: call `rt.Shutdown()` yourself (recommended: `defer rt.Shutdown()` right after construction)

**`Register` must run before `Launch`.** Calling `turbine.Register` after the runtime has launched will panic. The construct -> register -> launch ordering above prevents this by design.

**Logger defaults.** `NewStandalone` defaults `cfg.Logger` to a stdout `slog` handler so script output is visible. Every other constructor leaves `cfg.Logger` as the caller provided it; when `cfg.Logger` is `nil`, workflow and step logs fall back to the PocketBase `app.Logger()`. PocketBase's own log destinations (the `_logs` collection, etc.) are independent of `cfg.Logger`.

## Shutdown

Two-phase process:

1. **Drain**, stop accepting new work, let running workflows finish naturally
2. **Force**, if `Config.ShutdownTimeout` expires, cancel all remaining workflows

```go
err := rt.Shutdown()
```

During shutdown:
- `rt.IsDraining()` returns `true`
- Queue runners stop dequeuing
- New `Run` calls return `turbine.ErrShuttingDown`
- Running workflows get their context cancelled after the timeout

`Shutdown` returns an error if the drain phase timed out and workflows were force-cancelled. Calling `Shutdown` before `Launch` is safe and returns `nil`.

::: info
`Shutdown` always uses `Config.ShutdownTimeout` (default 30s). Set it on the `Config` passed to `NewRuntime` / `Setup` to tune the deadline.
:::

## Runtime State

| Method | Returns |
|---|---|
| `rt.IsLaunched()` | `true` after `Launch()`, `false` after `Shutdown()` |
| `rt.IsDraining()` | `true` during shutdown drain phase |
| `rt.GetExecutorID()` | Instance identifier (default: `"local"`) |
| `rt.GetApplicationVersion()` | App version (default: binary hash) |
| `rt.GetApplicationID()` | App name |
| `rt.App()` | The `core.App` |

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
