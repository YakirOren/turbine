# Checkpoints

By default, every step result is saved to SQLite and replayed on recovery. Use `WithoutCheckpoint()` for steps that must always re-execute.

## When to Skip Checkpointing

Use `WithoutCheckpoint()` for steps that produce non-serializable results like:

- Network connections (SSH, database, gRPC)
- File handles
- Mutex locks or other runtime resources

```go
type Deployer struct {
    ssh *SSHClient
}

func (d *Deployer) Connect(ctx context.Context) (*SSHClient, error) {
    client, err := DialSSH(ctx, "server.example.com")
    d.ssh = client
    return client, err
}

func (d *Deployer) Deploy(ctx context.Context) (string, error) {
    return d.ssh.Run(ctx, "./deploy.sh")
}
```

```go
func DeployWorkflow(ctx turbine.Context, host string) (string, error) {
    d := &Deployer{}

    // Always re-runs on recovery, connection can't be serialized
    _, err := turbine.Do(ctx, d.Connect,
        turbine.WithoutCheckpoint(),
        turbine.WithStepName("connect"),
    )
    if err != nil {
        return "", err
    }
    defer d.ssh.Close()

    // Checkpointed normally, replays from DB on recovery
    result, err := turbine.Do(ctx, d.Deploy,
        turbine.WithStepName("deploy"),
    )
    if err != nil {
        return "", err
    }

    return result, nil
}
```

## How It Works

| | Checkpointed (default) | `WithoutCheckpoint()` |
|---|---|---|
| First run | Executes, saves result to SQLite | Executes, result is not saved |
| Recovery | Replays saved result, skips execution | Re-executes the step |
| Use case | API calls, computations, side effects | Connections, handles, runtime resources |

::: tip
A non-checkpointed step still consumes a step ID. Steps after it are checkpointed normally and replay correctly on recovery.
:::

::: warning
Non-checkpointed steps should be idempotent or side-effect-free, since they will re-execute on every recovery. Establishing a connection is fine, sending an email is not.
:::
