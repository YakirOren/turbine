package turbine

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestQueueEnqueueAndDequeue(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	var executed atomic.Bool

	myWF := func(ctx Context, input string) (string, error) {
		executed.Store(true)
		return "queued:" + input, nil
	}

	Register(rt, myWF)
	rt.Queue("test-queue")

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	handle, err := Run(rt, myWF, "hello", WithQueue("test-queue"))
	if err != nil {
		t.Fatalf("Run with queue failed: %v", err)
	}

	// The workflow is enqueued — queue runner should pick it up
	// Poll for completion
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if executed.Load() {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if !executed.Load() {
		t.Fatal("queued workflow was not executed within timeout")
	}

	// Verify we can get the result via polling handle
	result, err := handle.GetResult()
	if err != nil {
		t.Fatalf("GetResult failed: %v", err)
	}
	if result != "queued:hello" {
		t.Fatalf("expected 'queued:hello', got %q", result)
	}
}

func TestQueueWithCustomID(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx Context, input int) (int, error) {
		return input * 3, nil
	}

	Register(rt, myWF)
	rt.Queue("id-queue")

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	handle, err := Run(rt, myWF, 7,
		WithQueue("id-queue"),
		WithID("custom-queue-wf-1"),
	)
	if err != nil {
		t.Fatal(err)
	}

	if handle.GetWorkflowID() != "custom-queue-wf-1" {
		t.Fatalf("expected custom ID, got %s", handle.GetWorkflowID())
	}

	// Wait for queue runner
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, err := handle.GetStatus()
		if err == nil && status.Status == StatusSuccess {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	result, err := handle.GetResult()
	if err != nil {
		t.Fatalf("GetResult failed: %v", err)
	}
	if result != 21 {
		t.Fatalf("expected 21, got %d", result)
	}
}

func TestQueueEventDrivenWakeUp(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	var executedAt atomic.Int64

	myWF := func(ctx Context, input string) (string, error) {
		executedAt.Store(time.Now().UnixMilli())
		return "fast:" + input, nil
	}

	Register(rt, myWF)
	rt.Queue("fast-queue")

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	enqueueTime := time.Now()
	handle, err := Run(rt, myWF, "test", WithQueue("fast-queue"))
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	result, err := handle.GetResult()
	if err != nil {
		t.Fatalf("GetResult failed: %v", err)
	}
	if result != "fast:test" {
		t.Fatalf("expected 'fast:test', got %q", result)
	}

	// Verify the workflow was picked up quickly (under 1 second)
	execTime := time.UnixMilli(executedAt.Load())
	latency := execTime.Sub(enqueueTime)
	if latency > 1*time.Second {
		t.Fatalf("queue wake-up too slow: %v (expected < 1s)", latency)
	}
}

// TestQueueGlobalConcurrencyDrains verifies that when a queue has a concurrency
// cap that throttles the initial dequeue, the queue runner is woken back up by
// each completion so the remaining backlog drains. Without the completion
// notification, only the first batch (= concurrency limit) would run and the
// rest would sit in `enqueued` forever.
func TestQueueGlobalConcurrencyDrains(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	const total = 10
	const concurrency = 3

	var executed atomic.Int32

	myWF := func(ctx Context, input int) (int, error) {
		// Small amount of work so multiple workflows actually overlap and
		// hold the concurrency slots simultaneously.
		time.Sleep(50 * time.Millisecond)
		executed.Add(1)
		return input, nil
	}

	Register(rt, myWF)
	rt.Queue("conc-queue", WithGlobalConcurrency(concurrency))

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(10 * time.Second)

	handles := make([]Handle[int], 0, total)
	for i := 0; i < total; i++ {
		h, err := Run(rt, myWF, i, WithQueue("conc-queue"))
		if err != nil {
			t.Fatalf("Run %d failed: %v", i, err)
		}
		handles = append(handles, h)
	}

	for i, h := range handles {
		if _, err := h.GetResult(); err != nil {
			t.Fatalf("workflow %d did not complete: %v", i, err)
		}
	}

	if got := executed.Load(); got != int32(total) {
		t.Fatalf("expected %d workflows to execute, got %d", total, got)
	}
}
