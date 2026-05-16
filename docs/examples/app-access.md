# App Access

Access the app from within workflow steps to create records, query data, and use its APIs.

**What to notice:**
- `turbine.AppFrom(ctx)` extracts the app from a step's context
- The step creates a record directly, full access to collections, queries, etc.
- The struct input type (`NoteInput`) is automatically JSON-serialized

<<< @/../examples/app-access/main.go
