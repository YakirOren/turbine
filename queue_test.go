package pbdbos

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestQueueEnqueueAndDequeue(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	var executed atomic.Bool

	myWF := func(ctx context.Context, rt *Runtime, input string) (string, error) {
		executed.Store(true)
		return "queued:" + input, nil
	}

	RegisterWorkflow(rt, myWF)
	NewWorkflowQueue(rt, "test-queue")

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	handle, err := RunWorkflow(rt, myWF, "hello", WithQueue("test-queue"))
	if err != nil {
		t.Fatalf("RunWorkflow with queue failed: %v", err)
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

	myWF := func(ctx context.Context, rt *Runtime, input int) (int, error) {
		return input * 3, nil
	}

	RegisterWorkflow(rt, myWF)
	NewWorkflowQueue(rt, "id-queue")

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	handle, err := RunWorkflow(rt, myWF, 7,
		WithQueue("id-queue"),
		WithWorkflowID("custom-queue-wf-1"),
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
		if err == nil && status.Status == WorkflowStatusSuccess {
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
