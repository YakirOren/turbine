# Basic

The simplest Turbine workflow: register a function and run it.

**What to notice:**
- `turbine.NewStandalone` creates the runtime, `rt.Launch()` starts it, the deferred `rt.Shutdown()` tears it down on exit
- Workflow signature: `func(ctx turbine.Context, input P) (R, error)`
- `handle.GetResult()` blocks until the workflow completes

<<< @/../examples/basic/main.go
