At work I've used Temporal, Airflow, and a couple of custom-built workflow systems. They do the job, but they take over. You end up with a whole separate system to operate and debug alongside your actual code, its own deployment pipeline, its own learning curve.

For my personal projects I use PocketBase it's simple, it uses SQLite, no fuss. I wanted to add workflows in those projects but everything out there required separate infrastructure. That's the opposite of why I picked PocketBase.

So I built **Turbine** — a durable workflow engine that runs as a PocketBase plugin. No external dependencies, everything stored in the same SQLite database PocketBase already uses.

## What it does

Workflows are regular Go functions. Each step result is recorded in SQLite — if the process crashes, execution resumes
from the last completed step.

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

Setup hooks into PocketBase's lifecycle automatically (creates collections on first launch, recovers pending workflows
on restart, drains gracefully on shutdown):

```go
app := pocketbase.New()
rt := turbine.Setup(app, turbine.Config{})
turbine.Register(rt, OrderWorkflow)
app.Start()
```

## What's in the box

- **Durable steps** — checkpointed to SQLite, replayed on recovery
- **Step retries** — exponential backoff
- **Queues** — concurrency control, rate limiting, priority, partitioned processing
- **Cron scheduling** — compile-time or runtime, manageable from the dashboard
- **Human-in-the-loop** — pause a workflow and wait for approval, survives restarts
- **Dashboard** — opt-in UI for monitoring workflows, queues, and schedules

## The dashboard

The dashboard is completely opt-in — all features work without it. It's a separate import you mount when you want a UI:

```go
import "github.com/YakirOren/turbine/dashboard"

dashboard.Mount(app, rt) // serves at /_/turbine/
```

You can view workflow status and step trees, inspect outputs,
trigger workflows (with typed input forms), approve/reject workflows waiting for human input, and manage schedules.

<!-- screenshots here -->

## Links

- Repo: https://github.com/YakirOren/turbine
- Docs: <!-- docs url here -->
- `go get github.com/YakirOren/turbine`

Would love to hear what you think — and thanks to @ganigeorgiev for building something so extensible that this was even
possible.
