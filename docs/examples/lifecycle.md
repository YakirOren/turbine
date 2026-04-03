# Lifecycle

Cancel, resume, and inspect running workflows.

**What to notice:**
- `WithID("job-"+id)` makes workflow IDs deterministic and predictable
- `rt.Cancel` / `rt.Resume` control workflow execution externally
- `handle.GetStatus()` returns the current status without blocking
- `rt.Steps` returns detailed step execution info (name, output, timing)

<<< @/../examples/lifecycle/main.go
