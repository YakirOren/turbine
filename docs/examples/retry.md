# Retry

Automatic retries with exponential backoff for unreliable operations.

**What to notice:**
- `WithStepMaxRetries(5)` retries the step up to 5 times before failing
- Backoff grows exponentially: 500ms, 1s, 2s, 4s, 8s (capped by `WithMaxInterval`)
- If all retries are exhausted, the workflow fails with `ErrMaxRetries`

<<< @/../examples/retry/main.go
