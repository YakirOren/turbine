# App Access

Access the PocketBase app from within workflow steps to create records, query data, and use any PocketBase API.

**What to notice:**
- `turbine.AppFrom(ctx)` extracts the PocketBase app from a step's context
- The step creates a PocketBase record directly — full access to collections, queries, etc.
- The struct input type (`NoteInput`) is automatically JSON-serialized

<<< @/../examples/app-access/main.go
