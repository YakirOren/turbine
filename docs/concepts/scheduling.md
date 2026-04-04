# Scheduling

Turbine uses PocketBase's built-in cron for scheduled workflows.

## Compile-Time Schedules

Register a cron schedule at compile time with `WithSchedule`. Scheduled workflows must accept `time.Time` as input — they receive the scheduled execution time.

```go
cleanup := func(ctx turbine.Context, scheduledAt time.Time) (string, error) {
    return "cleaned", nil
}

turbine.Register(rt, cleanup, turbine.WithSchedule("0 * * * *"))
```

### Cron Syntax

Standard five-field cron expressions:

```
┌───────────── minute (0-59)
│ ┌───────────── hour (0-23)
│ │ ┌───────────── day of month (1-31)
│ │ │ ┌───────────── month (1-12)
│ │ │ │ ┌───────────── day of week (0-6, Sun=0)
│ │ │ │ │
* * * * *
```

| Expression | Meaning |
|---|---|
| `0 * * * *` | Every hour |
| `*/5 * * * *` | Every 5 minutes |
| `0 3 * * *` | Daily at 3am |
| `0 0 * * 1` | Every Monday at midnight |

## Runtime Schedules

Turbine also supports schedules created at runtime via the `pt_schedules` collection. These can be managed from the dashboard or the PocketBase API.

Two types:

- **Cron** — recurring schedule with a cron expression and optional jitter
- **Once** — one-time execution at a specific time

Runtime schedules reference workflows by their fully-qualified name (FQN) and can be enabled/disabled without redeploying.

## Deduplication

::: info
Each scheduled run gets a deterministic workflow ID based on the schedule name and timestamp (e.g., `sched-cleanup-2026-03-27T03:00:00Z`). If a schedule fires while a previous run is still executing, the new run gets a unique ID and runs independently.
:::

## Enable / Disable

Compile-time schedules can be disabled at runtime through the `pt_schedules` collection. When disabled, the cron job still fires but the workflow is skipped. Re-enabling takes effect immediately — no restart needed.

::: tip
The `scheduledAt` parameter is useful for idempotency — you can use it to derive deterministic IDs or to check whether a scheduled window was already processed.
:::

See the [scheduled example](/examples/scheduled) for a working demo.
