package pocketflow

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestScheduledWorkflowRegistration(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	var executed atomic.Bool
	var receivedTime atomic.Value

	myWF := func(ctx Context, scheduledAt time.Time) (string, error) {
		executed.Store(true)
		receivedTime.Store(scheduledAt)
		return "scheduled-ok", nil
	}

	RegisterWorkflow(rt, myWF, WithSchedule("* * * * *"))

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	// Verify cron job was registered
	entry, ok := rt.workflowRegistry.Load(resolveWorkflowFunctionName(myWF))
	if !ok {
		t.Fatal("workflow not found in registry")
	}
	regEntry := entry.(WorkflowRegistryEntry)
	if regEntry.CronSchedule != "* * * * *" {
		t.Fatalf("expected cron schedule '* * * * *', got %q", regEntry.CronSchedule)
	}
}

func TestScheduledWorkflowExecution(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	var executed atomic.Bool

	myWF := func(ctx Context, scheduledAt time.Time) (string, error) {
		executed.Store(true)
		return "done", nil
	}

	RegisterWorkflow(rt, myWF, WithSchedule("* * * * *"))

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	// Simulate what the cron callback does: enqueue the workflow via the internal queue
	now := time.Now()
	fqn := resolveWorkflowFunctionName(myWF)
	registeredAny, _ := rt.workflowRegistry.Load(fqn)
	registered := registeredAny.(WorkflowRegistryEntry)

	wfID := "sched-test-" + now.UTC().Format(time.RFC3339)
	_, err := registered.wrappedFunction(rt, now,
		WithWorkflowID(wfID),
		WithQueue(_PF_INTERNAL_QUEUE_NAME),
	)
	if err != nil {
		t.Fatalf("failed to enqueue scheduled workflow: %v", err)
	}

	// Wait for the queue runner to pick it up
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if executed.Load() {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !executed.Load() {
		t.Fatal("scheduled workflow was not executed within timeout")
	}
}

func TestScheduledWorkflowPanicsOnWrongInputType(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx Context, input string) (string, error) {
		return input, nil
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for non-time.Time input")
		}
	}()

	RegisterWorkflow(rt, myWF, WithSchedule("* * * * *"))
}
