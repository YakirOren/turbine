# Communication

Turbine has two durable communication mechanisms: Send/Recv for direct messaging and Events for key-value signaling.

## Send / Recv

Point-to-point messaging between workflows.

```go
// In workflow A: send a message to workflow B
turbine.Send(ctx, targetWorkflowID, "payload", "my-topic")

// In workflow B: receive (blocks until message arrives or timeout)
msg, err := turbine.Recv[string](ctx, "my-topic", 30*time.Second)
```

If the timeout expires before a message arrives, `Recv` returns the zero value.

Messages are recorded as durable steps — on recovery, if the message was already received, the saved result is replayed.

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
