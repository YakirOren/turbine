# Scheduled

Cron-scheduled workflows using the built-in scheduler.

**What to notice:**
- Scheduled workflows must accept `time.Time` as input
- `WithSchedule("0 * * * *")` runs the workflow every hour
- No HTTP endpoint needed, the scheduler triggers it automatically
- No `app.OnServe` handler, the app just starts and the schedule runs

<<< @/../examples/scheduled/main.go
