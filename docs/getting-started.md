# Getting Started

## Install

```bash
go get github.com/YakirOren/turbine
```

## Quick Start

```go
package main

import (
	"log"

	"github.com/YakirOren/turbine"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func main() {
	app := pocketbase.New()

	rt := turbine.Setup(app, turbine.Config{})

	greet := func(ctx turbine.Context, name string) (string, error) {
		return "hello " + name, nil
	}

	turbine.Register(rt, greet)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.POST("/greet/{name}", func(re *core.RequestEvent) error {
			name := re.Request.PathValue("name")

			handle, err := turbine.Run(rt, greet, name)
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			result, err := handle.GetResult()
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			return re.JSON(200, map[string]string{"result": result})
		})
		return e.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
```

## What Happens on First Launch

::: info What happens on first launch
When PocketBase starts, Turbine automatically:

1. Creates the SQLite collections it needs (`pt_workflow_status`, `pt_operation_outputs`, etc.)
2. Recovers any pending workflows from a previous run
3. Starts the queue runner and cron scheduler
:::

## What's Next?

- [Workflows](/concepts/workflows) — how workflows work, registration, and run options
- [Steps](/concepts/steps) — durable execution units
- [Queues](/concepts/queues) — concurrency control and rate limiting
- [Scheduling](/concepts/scheduling) — cron-based workflows
- [Communication](/concepts/communication) — inter-workflow messaging
- [Approvals](/concepts/approvals) — human-in-the-loop gates
- [Products](/concepts/products) — file outputs from workflows
- [KV Store](/concepts/kv-store) — global key-value storage
- [Error Handling](/concepts/errors) — error types and patterns
- [Lifecycle](/concepts/lifecycle) — setup, shutdown, and introspection
