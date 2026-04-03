# Sleep

Durable pause that survives crashes and restarts. If the process crashes during the sleep, it resumes with only the remaining time.

**What to notice:**
- `turbine.Pause` records the wake-up time as a step
- On recovery, if the time has already passed, it returns immediately
- The 202 response returns right away — the workflow runs in the background

<<< @/../examples/sleep/main.go
