# App Status

A label + color pair that workflows set to report their current phase. Displayed in the dashboard and stored in the database.

## Setting Status from a Workflow

```go
func OrderWorkflow(ctx turbine.Context, orderID string) (string, error) {
    ctx.SetAppStatus("validating", "yellow")
    // ... validation steps ...

    ctx.SetAppStatus("processing", "blue")
    // ... processing steps ...

    ctx.SetAppStatus("fulfilled", "green")
    return "done", nil
}
```

## Setting Status from a Step

Use the standalone function with `context.Context`:

```go
result, err := turbine.Do(ctx, func(stepCtx context.Context) (string, error) {
    turbine.SetAppStatus(stepCtx, "uploading", "blue")
    // ...
    return "done", nil
}, turbine.WithStepName("upload"))
```

## Valid Colors

| Color | Typical Use |
|---|---|
| <span style="display:inline-block;padding:2px 10px;border-radius:9999px;background:#22c55e;color:#000;font-size:0.85em;font-weight:500">green</span> | Success, completed |
| <span style="display:inline-block;padding:2px 10px;border-radius:9999px;background:#ef4444;color:#fff;font-size:0.85em;font-weight:500">red</span> | Error, failed |
| <span style="display:inline-block;padding:2px 10px;border-radius:9999px;background:#eab308;color:#000;font-size:0.85em;font-weight:500">yellow</span> | Warning, waiting |
| <span style="display:inline-block;padding:2px 10px;border-radius:9999px;background:#3b82f6;color:#fff;font-size:0.85em;font-weight:500">blue</span> | In progress |
| <span style="display:inline-block;padding:2px 10px;border-radius:9999px;background:#a1a1aa;color:#000;font-size:0.85em;font-weight:500">gray</span> | Idle, inactive |
| <span style="display:inline-block;padding:2px 10px;border-radius:9999px;background:#84cc16;color:#000;font-size:0.85em;font-weight:500">lime</span> | Positive, minor success |
| <span style="display:inline-block;padding:2px 10px;border-radius:9999px;background:#f97316;color:#000;font-size:0.85em;font-weight:500">orange</span> | Caution |
| <span style="display:inline-block;padding:2px 10px;border-radius:9999px;background:#a855f7;color:#fff;font-size:0.85em;font-weight:500">purple</span> | Processing |
| <span style="display:inline-block;padding:2px 10px;border-radius:9999px;background:#ec4899;color:#fff;font-size:0.85em;font-weight:500">pink</span> | Custom |
| <span style="display:inline-block;padding:2px 10px;border-radius:9999px;background:#06b6d4;color:#000;font-size:0.85em;font-weight:500">cyan</span> | Custom |

Using an invalid color returns an error.

## Reading Status

The current app status is included in the `Status` struct returned by `handle.GetStatus()`:

```go
status, err := handle.GetStatus()
fmt.Println(status.AppStatus, status.AppStatusColor)
```

## Behavior During Recovery

::: info
Status updates are skipped during recovery replay to avoid unnecessary database writes. The status is restored to the last persisted value.
:::
