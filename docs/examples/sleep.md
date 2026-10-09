# Sleep

Durable sleep that survives crashes and restarts. If the process crashes during the sleep, it resumes with only the remaining time.

**What to notice:**
- `turbine.Sleep` records the wake-up time as a step
- On recovery, if the time has already passed, it returns immediately
- The example uses a short sleep for demo, durations like `24*time.Hour` are common in production

<<< @/../examples/sleep/main.go
