# Events

Inter-workflow communication using Send/Recv and key-value events.

**What to notice:**
- `turbine.SetValue` exposes workflow state that others can read with `GetValue`
- `turbine.Recv` blocks the workflow until a message arrives on the topic
- `turbine.Send` from an HTTP handler uses `rt.NewContext()` to create a Turbine context
- `WithID("approval-"+id)` gives the workflow a deterministic ID for targeting

<<< @/../examples/events/main.go
