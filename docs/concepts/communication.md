# Communication

Turbine has two durable communication mechanisms: Send/Recv for direct messaging and Events for key-value signaling.

## Send / Recv

Point-to-point messaging between workflows.

```go
// In workflow A: send a message to workflow B
turbine.Send(ctx, targetWorkflowID, "payload", "my-topic")

// In workflow B: receive (blocks until message arrives or timeout)
msg, ok, err := turbine.Recv[string](ctx, "my-topic", 30*time.Second)
```

`ok` reports whether a message arrived. If the timeout expires first, `Recv` returns the zero value and `ok` is false.

Messages are recorded as durable steps, on recovery, if the message was already received, the saved result is replayed. The timeout is durable too, a workflow recovered mid-wait waits only for the remaining time. It is measured from the first time the wait ran, so resuming a workflow after its deadline returns at once. A timeout of 0 or less checks for a message once without waiting.

### Waiting for an Event

`Recv` doubles as a durable "suspend until X happens" primitive. Give the workflow a deterministic ID, block on `Recv`, and send to it from wherever the event happens, such as an HTTP handler or a record hook. The workflow can wait for hours or days and survives restarts while it waits.

```go
// In the workflow: wait up to a day for the user's first todo
todo, ok, err := turbine.Recv[Todo](ctx, "first-todo", 24*time.Hour)
if err != nil {
    return "", err
}
if !ok {
    return "no todo within a day", nil
}

// In the create-todo handler
err := turbine.Send(rt.NewContext(re.Request.Context()), "onboarding-"+userID, todo, "first-todo")
```

See the [onboarding example](/examples/onboarding) for a full flow.

### Sending from HTTP Handlers

Outside a workflow, use `rt.SendToWorkflow()` or create a context with `rt.NewContext()`:

```go
// Option 1: SendToWorkflow (convenience)
err := rt.SendToWorkflow(workflowID, "payload", "my-topic")

// Option 2: NewContext + Send
tCtx := rt.NewContext(re.Request.Context())
err := turbine.Send(tCtx, workflowID, "payload", "my-topic")
```

See [Context](/concepts/context) for more on using Turbine APIs from handlers.

## Events

Key-value signaling scoped to a workflow. `GetValue` blocks until the key is set or the timeout expires.

```go
// In workflow A: set a key-value event
turbine.SetValue(ctx, "status", "ready")

// In workflow B: get the event (blocks until set or timeout)
val, err := turbine.GetValue[string](ctx, workflowID, "status", 10*time.Second)
```

## When to Use Which

| | Send/Recv | Events |
|---|---|---|
| **Pattern** | Point-to-point messaging | Publish/observe |
| **Blocking** | Receiver blocks until message or timeout | Reader blocks until key is set or timeout |
| **Direction** | Sender must know the target workflow ID | Reader must know the target workflow ID |
| **Multiplicity** | One message consumed by one receiver | Value readable by many observers |
| **Use case** | Approval decisions, task delegation | Status reporting, coordination flags |

See the [events example](/examples/events) for a working demo.
