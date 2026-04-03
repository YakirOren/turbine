# Queue

Queue workflows with concurrency control and rate limiting.

**What to notice:**
- `rt.Queue` defines the queue with `WithWorkerConcurrency(3)` and a rate limiter
- `turbine.WithQueue("emails")` enqueues the workflow instead of running it immediately
- The endpoint returns 202 with the workflow ID — the caller doesn't wait for completion

<<< @/../examples/queue/main.go
