// Package turbine is a SQLite-based durable workflow engine for Go, built on PocketBase.
//
// A workflow is an ordinary Go function. Inside it, Do executes a step whose
// result is checkpointed to the system database, so on crash recovery the
// workflow resumes from the last completed step instead of restarting.
//
// Quick start:
//
//	rt := turbine.NewStandalone(turbine.Config{})
//	defer rt.Shutdown()
//
//	turbine.Register(rt, MyWorkflow)
//	if err := rt.Launch(); err != nil {
//		log.Fatal(err)
//	}
//
//	handle, err := turbine.Run(rt, MyWorkflow, "input")
//	result, err := handle.GetResult()
//
// Core concepts:
//
//   - Workflows: durable functions registered with Register, invoked with Run.
//   - Steps: side-effecting functions executed inside a workflow via Do or DoAsync,
//     with results checkpointed to the system database.
//   - Queues: named workers with concurrency, priority, partitioning, and rate limiting.
//   - Schedules: cron-driven workflows registered with WithSchedule.
//   - Send and Recv: inter-workflow messaging.
//   - SetValue and GetValue: key-value events between workflows.
//   - Approvals: WaitForApproval pauses a workflow until a decision arrives via
//     Send or the approve HTTP endpoint.
//
// See https://turbine.yakir.io/ for the full documentation and runnable examples
// under examples/ in the source repository.
package turbine
