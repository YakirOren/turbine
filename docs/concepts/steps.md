# Steps

Steps are the unit of durable execution. Each step's result is recorded in SQLite. On recovery, recorded steps replay their saved result instead of re-executing.

```go
result, err := turbine.Do(ctx, func(ctx context.Context) (int, error) {
    return callExternalAPI()  // only called once, even across crashes
}, turbine.WithStepName("call-api"))
```

::: tip
Steps receive `context.Context` (not `turbine.Context`). This prevents calling `Do` or `Sleep` inside a step at compile time.
:::

## Step Retries

Steps support automatic retries with exponential backoff:

```go
result, err := turbine.Do(ctx, callUnreliableAPI,
    turbine.WithStepName("fetch"),
    turbine.WithStepMaxRetries(5),                    // retry up to 5 times
    turbine.WithBackoffFactor(2.0),                   // exponential backoff multiplier (default: 2.0)
    turbine.WithBaseInterval(500*time.Millisecond),   // initial delay between retries (default: 100ms)
    turbine.WithMaxInterval(10*time.Second),          // cap on retry delay (default: 5s)
)
```

::: warning
When a step exceeds its max retries, the workflow fails with `ErrMaxRetries`. The original error is wrapped and accessible via `errors.Unwrap`. See [Error Handling](/concepts/errors).
:::

## Concurrent Steps

Run steps in parallel with `DoAsync`. Returns a channel of `AsyncResult[R]`:

```go
ch, _ := turbine.DoAsync(ctx, func(ctx context.Context) (int, error) {
    return expensiveComputation()
}, turbine.WithStepName("compute"))

result := <-ch
// result.Result, result.Err
```

## Pause

Durable pause that survives crashes and restarts. The wake-up time is recorded as a step — on recovery, if the time has passed, it returns immediately; otherwise it pauses only the remaining duration.

```go
if err := turbine.Pause(ctx, 24*time.Hour); err != nil {
    return "", err
}
```

## Accessing Context

Use helper functions to access the logger and PocketBase app from within steps:

```go
result, err := turbine.Do(ctx, func(ctx context.Context) (string, error) {
    logger := turbine.LoggerFrom(ctx)
    app := turbine.AppFrom(ctx)

    logger.Info("doing work")
    // use app for PocketBase operations
    return "done", nil
}, turbine.WithStepName("work"))
```

## Skipping Checkpoints

By default, every step result is saved to SQLite for replay on recovery. For steps that produce non-serializable results (like network connections), use `WithoutCheckpoint()` to always re-execute. See [Checkpoints](/concepts/checkpoints) for details.
