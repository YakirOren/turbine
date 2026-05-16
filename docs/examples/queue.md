# Queue

Queue workflows with concurrency control and rate limiting.

**What to notice:**
- `rt.Queue` defines the queue with `WithWorkerConcurrency(3)` and a rate limiter
- `turbine.WithQueue("emails")` enqueues the workflow instead of running it immediately
- The example enqueues three workflows and waits for each result, the queue dispatches up to 3 concurrently subject to the rate limiter

<<< @/../examples/queue/main.go
