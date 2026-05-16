# Concurrent Steps

Run multiple steps in parallel with `DoAsync`. Each step is independently durable, on recovery, completed steps replay from the DB.

**What to notice:**
- `DoAsync` returns a `chan AsyncResult[R]`, read it with `<-ch`
- Each concurrent step gets its own checkpoint
- Error handling checks each result individually after all channels are read

<<< @/../examples/concurrent/main.go
