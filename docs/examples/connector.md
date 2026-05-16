# Connector

Use `WithoutCheckpoint()` for steps that manage non-serializable resources like connections. The connection step always re-runs on recovery, while subsequent steps replay from the database.

**What to notice:**
- `WithoutCheckpoint()` on the connect step, connections can't be serialized to SQLite
- The `Deployer` struct holds live state (SSH client) shared across steps
- Methods on the struct (`d.Connect`, `d.Deploy`) are passed directly as step functions
- See [Checkpoints](/concepts/checkpoints) for when and why to skip checkpointing

<<< @/../examples/connector/main.go
