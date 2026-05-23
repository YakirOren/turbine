package turbine

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
	// Tests use httptest.NewServer which binds 127.0.0.1; opt out of the
	// production SSRF guard so the loopback receivers in webhook tests work.
	// Short ShutdownTimeout caps cleanup wait when a workflow blocks past test return.
	rt := NewRuntime(app, Config{
		AllowPrivateAddresses: true,
		ShutdownTimeout:       500 * time.Millisecond,
	})
	return rt, app.Cleanup
}

func TestRunSimple(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx Context, input string) (string, error) {
		return "hello " + input, nil
	}

	Register(rt, myWF)

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown()

	handle, err := Run(rt, myWF, "world")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	result, err := handle.GetResult()
	if err != nil {
		t.Fatalf("GetResult failed: %v", err)
	}
	if result != "hello world" {
		t.Fatalf("expected 'hello world', got %q", result)
	}
}

func TestRunWithStep(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx Context, input int) (int, error) {
		doubled, err := Do(ctx, func(ctx context.Context) (int, error) {
			return input * 2, nil
		}, WithStepName("double"))
		if err != nil {
			return 0, err
		}
		return doubled, nil
	}

	Register(rt, myWF)

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown()

	handle, err := Run(rt, myWF, 21)
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

func TestRunWithError(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx Context, input string) (string, error) {
		return "", fmt.Errorf("intentional error")
	}

	Register(rt, myWF)

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown()

	handle, err := Run(rt, myWF, "test")
	if err != nil {
		t.Fatal(err)
	}

	_, err = handle.GetResult()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRunGetStatus(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx Context, input string) (string, error) {
		return "done", nil
	}

	Register(rt, myWF)

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown()

	handle, err := Run(rt, myWF, "test")
	if err != nil {
		t.Fatal(err)
	}

	// Wait for completion
	_, _ = handle.GetResult()

	status, err := handle.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Status != StatusSuccess {
		t.Fatalf("expected SUCCESS, got %s", status.Status)
	}
}

func TestRunMultipleSteps(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx Context, input int) (int, error) {
		a, err := Do(ctx, func(ctx context.Context) (int, error) {
			return input + 1, nil
		}, WithStepName("step1"))
		if err != nil {
			return 0, err
		}

		b, err := Do(ctx, func(ctx context.Context) (int, error) {
			return a * 2, nil
		}, WithStepName("step2"))
		if err != nil {
			return 0, err
		}

		return b, nil
	}

	Register(rt, myWF)

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown()

	handle, err := Run(rt, myWF, 10)
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
	steps, err := rt.Steps(handle.GetWorkflowID())
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
}

func TestRunWithTimeout(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx Context, input string) (string, error) {
		select {
		case <-time.After(5 * time.Second):
			return "should not reach", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	Register(rt, myWF)

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown()

	handle, err := Run(rt, myWF, "test", WithTimeout(100*time.Millisecond))
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

func TestRunWithDeadline(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx Context, input string) (string, error) {
		select {
		case <-time.After(5 * time.Second):
			return "should not reach", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	Register(rt, myWF)

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown()

	deadline := time.Now().Add(100 * time.Millisecond)
	handle, err := Run(rt, myWF, "test", WithDeadline(deadline))
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

	myWF := func(ctx Context, input string) (string, error) {
		return "done", nil
	}

	Register(rt, myWF)

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown()

	// Run a workflow to completion
	handle, err := Run(rt, myWF, "gc-test", WithID("gc-test-1"))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = handle.GetResult()

	// Verify it exists
	status, err := handle.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Status != StatusSuccess {
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
	myWF := func(ctx Context, input string) (string, error) {
		<-blocker
		return "done", nil
	}

	Register(rt, myWF)

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown()

	handle, err := Run(rt, myWF, "pending-test", WithID("gc-pending-1"))
	if err != nil {
		t.Fatal(err)
	}

	// GC with tiny retention, should NOT delete pending workflow
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
	if status.Status != StatusPending {
		t.Fatalf("expected PENDING, got %s", status.Status)
	}

	close(blocker)
}

func TestRetrieve(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx Context, input string) (string, error) {
		return "result:" + input, nil
	}

	Register(rt, myWF)

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown()

	handle, err := Run(rt, myWF, "test", WithID("retrieve-test-1"))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = handle.GetResult() // Wait for completion

	// Retrieve by ID
	retrieved := Retrieve[string](rt, "retrieve-test-1")
	result, err := retrieved.GetResult()
	if err != nil {
		t.Fatalf("retrieved GetResult failed: %v", err)
	}
	if result != "result:test" {
		t.Fatalf("expected 'result:test', got %q", result)
	}
}
