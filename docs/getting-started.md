# Getting Started

## Install

```bash
go get github.com/YakirOren/turbine
```

## Quick Start

```go
package main

import (
	"context"
	"log"

	"github.com/YakirOren/turbine"
)

func main() {
	rt := turbine.NewStandalone(turbine.Config{})
	defer rt.Shutdown()

	processOrder := func(ctx turbine.Context, orderID string) (string, error) {
		// Each step's result is recorded in SQLite.
		// On crash, the workflow resumes from the last completed step.
		chargeID, err := turbine.Do(ctx, func(ctx context.Context) (string, error) {
			return "ch_" + orderID, nil
		}, turbine.WithStepName("charge"))
		if err != nil {
			return "", err
		}

		return turbine.Do(ctx, func(ctx context.Context) (string, error) {
			return "shipped " + orderID + " (charge: " + chargeID + ")", nil
		}, turbine.WithStepName("ship"))
	}
	turbine.Register(rt, processOrder)

	if err := rt.Launch(); err != nil {
		log.Fatal(err)
	}

	handle, _ := turbine.Run(rt, processOrder, "ord-42")
	result, _ := handle.GetResult()
	log.Println(result)
}
```

## Serving HTTP Alongside Workflows

If you want to trigger workflows from HTTP routes, use `turbine.NewApp` instead. It returns both the app (for routing, `Start`, etc.) and the runtime:

```go
app, rt := turbine.NewApp(turbine.Config{})

turbine.Register(rt, processOrder)

app.OnServe().BindFunc(func(e *turbine.ServeEvent) error {
	e.Router.POST("/orders/{id}", func(re *turbine.RequestEvent) error {
		handle, _ := turbine.Run(rt, processOrder, re.Request.PathValue("id"))
		result, _ := handle.GetResult()
		return re.JSON(200, map[string]string{"result": result})
	})
	return e.Next()
})

log.Fatal(app.Start())
```

## What Happens on First Launch

::: info What happens on first launch
On first launch, Turbine automatically:

1. Creates the SQLite collections it needs (`pt_workflow_status`, `pt_operation_outputs`, etc.)
2. Recovers any pending workflows from a previous run
3. Starts the queue runner and cron scheduler
:::

## What's Next?

- [Workflows](/concepts/workflows), how workflows work, registration, and run options
- [Steps](/concepts/steps), durable execution units
- [Queues](/concepts/queues), concurrency control and rate limiting
- [Scheduling](/concepts/scheduling), cron-based workflows
- [Communication](/concepts/communication), inter-workflow messaging
- [Approvals](/concepts/approvals), human-in-the-loop gates
- [Products](/concepts/products), file outputs from workflows
- [KV Store](/concepts/kv-store), global key-value storage
- [Error Handling](/concepts/errors), error types and patterns
- [Lifecycle](/concepts/lifecycle), setup, shutdown, and introspection
