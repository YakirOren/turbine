# Steps

Durable steps that survive crashes. Each step's result is saved, if the process crashes mid-workflow, it resumes from the last completed step without re-executing it.

**What to notice:**
- Each `turbine.Do` call is a checkpoint, its result is recorded in SQLite
- Steps receive `context.Context`, not `turbine.Context`, this prevents nesting steps
- On recovery, completed steps replay their saved result instead of re-executing

<<< @/../examples/steps/main.go
