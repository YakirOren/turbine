package pocketflow

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
