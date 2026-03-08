package pbdbos

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/tests"
)

func setupRuntime(t *testing.T) (*Runtime, func()) {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	rt := New(app, Config{})
	return rt, app.Cleanup
}

func TestRunWorkflowSimple(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx context.Context, rt *Runtime, input string) (string, error) {
		return "hello " + input, nil
	}

	RegisterWorkflow(rt, myWF)

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	handle, err := RunWorkflow(rt, myWF, "world")
	if err != nil {
		t.Fatalf("RunWorkflow failed: %v", err)
	}

	result, err := handle.GetResult()
	if err != nil {
		t.Fatalf("GetResult failed: %v", err)
	}
	if result != "hello world" {
		t.Fatalf("expected 'hello world', got %q", result)
	}
}

func TestRunWorkflowWithStep(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx context.Context, rt *Runtime, input int) (int, error) {
		doubled, err := RunAsStep(ctx, rt, func(ctx context.Context) (int, error) {
			return input * 2, nil
		}, WithStepName("double"))
		if err != nil {
			return 0, err
		}
		return doubled, nil
	}

	RegisterWorkflow(rt, myWF)

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	handle, err := RunWorkflow(rt, myWF, 21)
	if err != nil {
		t.Fatal(err)
	}

	result, err := handle.GetResult()
	if err != nil {
		t.Fatal(err)
	}
	if result != 42 {
		t.Fatalf("expected 42, got %d", result)
	}
}

func TestRunWorkflowWithError(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx context.Context, rt *Runtime, input string) (string, error) {
		return "", fmt.Errorf("intentional error")
	}

	RegisterWorkflow(rt, myWF)

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	handle, err := RunWorkflow(rt, myWF, "test")
	if err != nil {
		t.Fatal(err)
	}

	_, err = handle.GetResult()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRunWorkflowGetStatus(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx context.Context, rt *Runtime, input string) (string, error) {
		return "done", nil
	}

	RegisterWorkflow(rt, myWF)

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	handle, err := RunWorkflow(rt, myWF, "test")
	if err != nil {
		t.Fatal(err)
	}

	// Wait for completion
	_, _ = handle.GetResult()

	status, err := handle.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Status != WorkflowStatusSuccess {
		t.Fatalf("expected SUCCESS, got %s", status.Status)
	}
}

func TestRunWorkflowMultipleSteps(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx context.Context, rt *Runtime, input int) (int, error) {
		a, err := RunAsStep(ctx, rt, func(ctx context.Context) (int, error) {
			return input + 1, nil
		}, WithStepName("step1"))
		if err != nil {
			return 0, err
		}

		b, err := RunAsStep(ctx, rt, func(ctx context.Context) (int, error) {
			return a * 2, nil
		}, WithStepName("step2"))
		if err != nil {
			return 0, err
		}

		return b, nil
	}

	RegisterWorkflow(rt, myWF)

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	handle, err := RunWorkflow(rt, myWF, 10)
	if err != nil {
		t.Fatal(err)
	}

	result, err := handle.GetResult()
	if err != nil {
		t.Fatal(err)
	}
	if result != 22 {
		t.Fatalf("expected 22, got %d", result)
	}

	// Verify steps were recorded
	steps, err := GetWorkflowSteps(rt, handle.GetWorkflowID())
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
}

func TestRunWorkflowWithTimeout(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx context.Context, rt *Runtime, input string) (string, error) {
		select {
		case <-time.After(5 * time.Second):
			return "should not reach", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	RegisterWorkflow(rt, myWF)

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	handle, err := RunWorkflow(rt, myWF, "test", WithTimeout(100*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}

	_, err = handle.GetResult()
	if err == nil {
		t.Fatal("expected error from timeout, got nil")
	}

	// Verify status shows the timeout was persisted
	status, err := handle.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Timeout != 100*time.Millisecond {
		t.Fatalf("expected timeout 100ms, got %v", status.Timeout)
	}
}

func TestRunWorkflowWithDeadline(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx context.Context, rt *Runtime, input string) (string, error) {
		select {
		case <-time.After(5 * time.Second):
			return "should not reach", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	RegisterWorkflow(rt, myWF)

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	deadline := time.Now().Add(100 * time.Millisecond)
	handle, err := RunWorkflow(rt, myWF, "test", WithDeadline(deadline))
	if err != nil {
		t.Fatal(err)
	}

	_, err = handle.GetResult()
	if err == nil {
		t.Fatal("expected error from deadline, got nil")
	}

	// Verify status shows the deadline was persisted
	status, err := handle.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Deadline.IsZero() {
		t.Fatal("expected non-zero deadline in status")
	}
}

func TestGarbageCollect(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx context.Context, rt *Runtime, input string) (string, error) {
		return "done", nil
	}

	RegisterWorkflow(rt, myWF)

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	// Run a workflow to completion
	handle, err := RunWorkflow(rt, myWF, "gc-test", WithWorkflowID("gc-test-1"))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = handle.GetResult()

	// Verify it exists
	status, err := handle.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Status != WorkflowStatusSuccess {
		t.Fatalf("expected SUCCESS, got %s", status.Status)
	}

	// Set retention to 0 (effectively delete everything completed) and run GC
	rt.config.GCRetention = 1 * time.Millisecond
	time.Sleep(2 * time.Millisecond)
	if err := rt.GarbageCollect(); err != nil {
		t.Fatalf("GarbageCollect failed: %v", err)
	}

	// Verify the workflow was deleted
	_, err = handle.GetStatus()
	if err == nil {
		t.Fatal("expected error after GC, workflow should be deleted")
	}
}

func TestGarbageCollectPreservesPending(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	// Use a workflow that blocks until we signal it
	blocker := make(chan struct{})
	myWF := func(ctx context.Context, rt *Runtime, input string) (string, error) {
		<-blocker
		return "done", nil
	}

	RegisterWorkflow(rt, myWF)

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	handle, err := RunWorkflow(rt, myWF, "pending-test", WithWorkflowID("gc-pending-1"))
	if err != nil {
		t.Fatal(err)
	}

	// GC with tiny retention — should NOT delete pending workflow
	rt.config.GCRetention = 1 * time.Millisecond
	time.Sleep(2 * time.Millisecond)
	if err := rt.GarbageCollect(); err != nil {
		t.Fatalf("GarbageCollect failed: %v", err)
	}

	// Pending workflow should still exist
	status, err := handle.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Status != WorkflowStatusPending {
		t.Fatalf("expected PENDING, got %s", status.Status)
	}

	close(blocker)
}

func TestRetrieveWorkflow(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx context.Context, rt *Runtime, input string) (string, error) {
		return "result:" + input, nil
	}

	RegisterWorkflow(rt, myWF)

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	handle, err := RunWorkflow(rt, myWF, "test", WithWorkflowID("retrieve-test-1"))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = handle.GetResult() // Wait for completion

	// Retrieve by ID
	retrieved := RetrieveWorkflow[string](rt, "retrieve-test-1")
	result, err := retrieved.GetResult()
	if err != nil {
		t.Fatalf("retrieved GetResult failed: %v", err)
	}
	if result != "result:test" {
		t.Fatalf("expected 'result:test', got %q", result)
	}
}
