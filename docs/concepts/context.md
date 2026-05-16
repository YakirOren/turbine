# Context

Workflows receive `turbine.Context`. Steps receive `context.Context`. This prevents calling `Do` or `Pause` inside a step at compile time.

## turbine.Context

```go
type Context interface {
    context.Context
    App() core.App
    Logger() *slog.Logger
    WorkflowID() (string, error)
    SetAppStatus(label, color string) error
}
```

```go
func MyWorkflow(ctx turbine.Context, input string) (string, error) {
    app := ctx.App()
    logger := ctx.Logger()     // logger with workflow_id and step_id attached
    wfID, _ := ctx.WorkflowID()
    ctx.SetAppStatus("working", "blue")
}
```

## context.Context in Steps

Use helper functions to access Turbine resources from within steps:

```go
result, err := turbine.Do(ctx, func(stepCtx context.Context) (string, error) {
    logger := turbine.LoggerFrom(stepCtx)   // includes workflow_id + step_id
    app := turbine.AppFrom(stepCtx)
    turbine.SetAppStatus(stepCtx, "uploading", "blue")
    return "done", nil
}, turbine.WithStepName("work"))
```

## App Access

`turbine.AppFrom` (in steps) and `ctx.App()` (in workflows) return the `core.App` instance, full access to records, files, the store, and everything else.

```go
result, err := turbine.Do(ctx, func(stepCtx context.Context) (string, error) {
    app := turbine.AppFrom(stepCtx)

    // Create a record
    collection, _ := app.FindCollectionByNameOrId("invoices")
    record := core.NewRecord(collection)
    record.Set("amount", 99.00)
    app.Save(record)

    // Query records
    records, _ := app.FindRecordsByFilter("orders", "status = 'pending'", "", 100, 0)

    // In-memory store (not persistent, lost on restart)
    app.Store().Set("last_sync", time.Now())

    // Access the filesystem
    fsys, _ := app.NewFilesystem()
    defer fsys.Close()

    return record.Id, nil
}, turbine.WithStepName("create-invoice"))
```

::: tip
`app.Store()` is an in-memory cache, it's lost on restart. For persistent key-value data, use the [KV Store](/concepts/kv-store) instead.
:::

See the [app-access example](/examples/app-access) for a complete working demo.

## Logging

Turbine uses the app's structured logger (`slog`). All logs are stored in the app's `_logs` table.

### From a workflow:

```go
logger := ctx.Logger()
logger.Info("processing order", "order_id", orderID)
```

### From a step:

```go
logger := turbine.LoggerFrom(stepCtx)
logger.Info("calling payment API", "amount", 99.00)
```

Both automatically attach `workflow_id` and `step_id` fields to every log entry, so you can filter logs for a specific workflow or step.

### Querying Logs

Since logs live in the app's `_logs` collection, you can query them with `data` filters:

```
data.workflow_id = "abc123"
data.step_id = 3
```

::: info
Turbine's internal logs (recovery, queue runner, GC) use a `data.source = "system"` attribute to distinguish them from user logs.
:::

## Using Turbine from HTTP Handlers

Use `rt.NewContext()` to call Turbine APIs from HTTP handlers:

```go
app.OnServe().BindFunc(func(e *turbine.ServeEvent) error {
    e.Router.POST("/approve/{id}", func(re *turbine.RequestEvent) error {
        tCtx := rt.NewContext(re.Request.Context())
        return turbine.Send(tCtx, re.Request.PathValue("id"),
            &turbine.ApprovalResult{Approved: true}, "pt.approval")
    })
    return e.Next()
})
```

## Extracting Context

```go
tCtx, ok := turbine.FromContext(ctx)
if ok {
    // inside a Turbine context
    tCtx.Logger().Info("turbine context available")
}
```

## SendToWorkflow

Convenience method for sending from outside a workflow:

```go
err := rt.SendToWorkflow(workflowID, "payload", "my-topic")
```

This is equivalent to creating a context with `rt.NewContext()` and calling `turbine.Send()`.
