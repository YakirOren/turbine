package turbine

import (
	"context"
	"testing"
	"time"
)

func TestSetValueAndGetValue(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	producer := func(ctx Context, _ string) (string, error) {
		if err := SetValue(ctx, "status", "ready"); err != nil {
			return "", err
		}
		return "produced", nil
	}

	consumer := func(ctx Context, targetID string) (string, error) {
		val, err := GetValue[string](ctx, targetID, "status", 5*time.Second)
		if err != nil {
			return "", err
		}
		return val, nil
	}

	Register(rt, producer)
	Register(rt, consumer)
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown()

	// Run producer first
	ph, err := Run(rt, producer, "", WithID("producer-1"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ph.GetResult(); err != nil {
		t.Fatalf("producer failed: %v", err)
	}

	// Run consumer that reads producer's value
	ch, err := Run(rt, consumer, "producer-1", WithID("consumer-1"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := ch.GetResult()
	if err != nil {
		t.Fatalf("consumer failed: %v", err)
	}
	if result != "ready" {
		t.Fatalf("expected 'ready', got %q", result)
	}
}

func TestGetValueTimeoutWorkflow(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx Context, _ string) (string, error) {
		val, err := GetValue[string](ctx, "nonexistent-wf", "key", 50*time.Millisecond)
		if err != nil {
			return "", err
		}
		return val, nil
	}

	Register(rt, myWF)
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown()

	handle, err := Run(rt, myWF, "")
	if err != nil {
		t.Fatal(err)
	}
	result, err := handle.GetResult()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "" {
		t.Fatalf("expected empty string for timed out GetValue, got %q", result)
	}
}

func TestPause(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx Context, _ string) (string, error) {
		if err := Pause(ctx, 50*time.Millisecond); err != nil {
			return "", err
		}
		return "done", nil
	}

	Register(rt, myWF)
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown()

	start := time.Now()
	handle, err := Run(rt, myWF, "", WithID("pause-test"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := handle.GetResult()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 50*time.Millisecond {
		t.Fatalf("expected at least 50ms elapsed, got %v", elapsed)
	}
	if result != "done" {
		t.Fatalf("expected 'done', got %q", result)
	}

	// Verify pt.sleep step was recorded
	steps, err := rt.Steps(handle.GetWorkflowID())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range steps {
		if s.FunctionName == "pt.sleep" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected pt.sleep step to be recorded")
	}
}

func TestCancel(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	started := make(chan struct{})
	myWF := func(ctx Context, _ string) (string, error) {
		close(started)
		// Block until cancelled
		<-ctx.Done()
		return "", ctx.Err()
	}

	Register(rt, myWF)
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown()

	handle, err := Run(rt, myWF, "", WithID("cancel-test"), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	// Wait for workflow to actually start
	<-started

	if err := rt.Cancel("cancel-test"); err != nil {
		t.Fatalf("cancel failed: %v", err)
	}

	// Poll for status since cancellation is async
	dl := time.Now().Add(3 * time.Second)
	for time.Now().Before(dl) {
		status, err := handle.GetStatus()
		if err != nil {
			t.Fatal(err)
		}
		if status.Status == StatusCancelled {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("workflow was not cancelled within timeout")
}

func TestCancelAndResume(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	started := make(chan struct{}, 1)
	myWF := func(ctx Context, _ string) (string, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return "", ctx.Err()
	}

	Register(rt, myWF)
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown()

	handle, err := Run(rt, myWF, "", WithID("cancel-resume-test"))
	if err != nil {
		t.Fatal(err)
	}

	<-started

	// Cancel the running workflow
	if err := rt.Cancel("cancel-resume-test"); err != nil {
		t.Fatalf("cancel failed: %v", err)
	}

	// Wait for cancellation
	var status Status
	dl := time.Now().Add(3 * time.Second)
	for time.Now().Before(dl) {
		var err error
		status, err = handle.GetStatus()
		if err != nil {
			t.Fatal(err)
		}
		if status.Status == StatusCancelled {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if status.Status != StatusCancelled {
		t.Fatalf("workflow was not cancelled within timeout, got %s", status.Status)
	}

	// Resume the cancelled workflow — status should move to ENQUEUED
	if err := rt.Resume("cancel-resume-test"); err != nil {
		t.Fatalf("resume failed: %v", err)
	}

	status, err = handle.GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != StatusEnqueued && status.Status != StatusPending {
		t.Fatalf("expected ENQUEUED or PENDING after resume, got %s", status.Status)
	}
}

func TestList(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	fastWF := func(ctx Context, input string) (string, error) {
		return "result:" + input, nil
	}

	Register(rt, fastWF)
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown()

	// Run two workflows
	h1, err := Run(rt, fastWF, "a", WithID("list-a"))
	if err != nil {
		t.Fatal(err)
	}
	h2, err := Run(rt, fastWF, "b", WithID("list-b"))
	if err != nil {
		t.Fatal(err)
	}

	// Wait for completion
	if _, err := h1.GetResult(); err != nil {
		t.Fatal(err)
	}
	if _, err := h2.GetResult(); err != nil {
		t.Fatal(err)
	}

	// List by status
	results, err := rt.List(listWorkflowsDBInput{
		status: []StatusType{StatusSuccess},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 successful workflows, got %d", len(results))
	}

	// List by workflow ID
	results, err = rt.List(listWorkflowsDBInput{
		workflowIDs: []string{"list-a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for ID filter, got %d", len(results))
	}
	if results[0].ID != "list-a" {
		t.Fatalf("expected ID 'list-a', got %q", results[0].ID)
	}

	// List with limit
	results, err = rt.List(listWorkflowsDBInput{
		status: []StatusType{StatusSuccess},
		limit:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result with limit, got %d", len(results))
	}
}

func TestDoAsync(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx Context, _ string) (int, error) {
		ch1, err := DoAsync(ctx, func(ctx context.Context) (int, error) {
			return 10, nil
		}, WithStepName("step1"))
		if err != nil {
			return 0, err
		}

		ch2, err := DoAsync(ctx, func(ctx context.Context) (int, error) {
			return 20, nil
		}, WithStepName("step2"))
		if err != nil {
			return 0, err
		}

		r1 := <-ch1
		if r1.Err != nil {
			return 0, r1.Err
		}
		r2 := <-ch2
		if r2.Err != nil {
			return 0, r2.Err
		}

		return r1.Result + r2.Result, nil
	}

	Register(rt, myWF)
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown()

	handle, err := Run(rt, myWF, "")
	if err != nil {
		t.Fatal(err)
	}
	result, err := handle.GetResult()
	if err != nil {
		t.Fatal(err)
	}
	if result != 30 {
		t.Fatalf("expected 30, got %d", result)
	}
}
