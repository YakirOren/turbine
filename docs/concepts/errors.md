# Error Handling

All Turbine errors use the `Error` type with specific error codes.

## Error

```go
type Error struct {
    Message         string
    Code            ErrorCode
    WorkflowID      string
    DestinationID   string
    StepName        string
    QueueName       string
    DeduplicationID string
    MaxRetries      int
}
```

## Error Codes

| Code | Meaning |
|---|---|
| `ErrConflictingID` | A workflow with this ID already exists |
| `ErrWorkflowNotFound` | No workflow found with the given ID |
| `ErrWorkflowConflict` | Conflicting workflow invocation with the same ID |
| `ErrCancelled` | Workflow was cancelled |
| `ErrAwaitCancelled` | A workflow being awaited was cancelled |
| `ErrRegistrationConflict` | Workflow name or FQN already registered |
| `ErrDeadLetter` | Workflow exceeded max recovery attempts |
| `ErrMaxRetries` | Step exceeded max retry attempts |
| `ErrDeduplicated` | Workflow was deduplicated (duplicate in queue) |

## Matching Errors

Use `errors.Is` to check for specific error codes:

```go
handle, err := turbine.Run(rt, myWorkflow, input, turbine.WithID("order-123"))
if err != nil {
    var ptErr *turbine.Error
    if errors.As(err, &ptErr) {
        fmt.Println(ptErr.Code, ptErr.WorkflowID)
    }

    // Or match by code directly
    if errors.Is(err, &turbine.Error{Code: turbine.ErrConflictingID}) {
        // workflow with this ID already exists
    }
    if errors.Is(err, &turbine.Error{Code: turbine.ErrDeduplicated}) {
        // duplicate was suppressed
    }
}
```

## Common Patterns

### Idempotent Workflow Starts

Use `WithID` and handle `ErrConflictingID` to safely retry:

```go
handle, err := turbine.Run(rt, processOrder, orderID,
    turbine.WithID("order-" + orderID),
)
if errors.Is(err, &turbine.Error{Code: turbine.ErrConflictingID}) {
    // already running — retrieve existing handle
    handle = turbine.Retrieve[string](rt, "order-" + orderID)
}
```

### Handling Cancellation

```go
result, err := handle.GetResult()
if errors.Is(err, &turbine.Error{Code: turbine.ErrCancelled}) {
    // workflow was cancelled
}
```

### Step Retry Exhaustion

When a step exceeds its max retries, the error wraps the original cause:

```go
result, err := turbine.Do(ctx, unreliableStep,
    turbine.WithStepMaxRetries(3),
)
if errors.Is(err, &turbine.Error{Code: turbine.ErrMaxRetries}) {
    // step failed after 3 retries
    // errors.Unwrap(err) returns the last step error
}
```

## Sentinel Errors

In addition to `Error`, Turbine defines sentinel errors:

| Error | Meaning |
|---|---|
| `turbine.ErrShuttingDown` | Runtime is shutting down, not accepting new workflows |
| `turbine.ErrApprovalTimeout` | `WaitForApproval` timed out (see [Approvals](/concepts/approvals)) |

```go
if errors.Is(err, turbine.ErrShuttingDown) {
    // runtime is draining
}
```
