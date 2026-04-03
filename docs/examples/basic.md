# Basic

The simplest Turbine workflow — register a function and call it from an HTTP endpoint.

**What to notice:**
- `turbine.Setup` hooks into PocketBase's lifecycle — no manual `Launch`/`Shutdown`
- Workflow signature: `func(ctx turbine.Context, input P) (R, error)`
- `handle.GetResult()` blocks until the workflow completes

<<< @/../examples/basic/main.go
