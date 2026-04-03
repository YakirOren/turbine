package turbine

import (
	"context"
	"testing"
	"time"
)

func TestWaitForApproval_Approved(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	var capturedResult ApprovalResult

	wf := func(ctx Context, input string) (string, error) {
		var err error
		capturedResult, err = WaitForApproval(ctx)
		if err != nil {
			return "", err
		}
		if !capturedResult.Approved {
			return "rejected", nil
		}
		return "approved", nil
	}

	Register(rt, wf, WithName("approval-test"))
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	handle, err := Run(rt, wf, "test-input")
	if err != nil {
		t.Fatal(err)
	}

	// Give workflow time to reach WaitForApproval
	time.Sleep(200 * time.Millisecond)

	// Check app status is set
	status, err := handle.GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.AppStatus != "waiting for approval" {
		t.Fatalf("expected app_status 'waiting for approval', got %q", status.AppStatus)
	}

	// Approve the workflow via Send (same mechanism the HTTP handler uses)
	ctx := rt.NewContext(context.Background())
	err = Send(ctx, handle.GetWorkflowID(), ApprovalResult{
		Approved: true,
		Comment:  "looks good",
	}, approvalTopic)
	if err != nil {
		t.Fatal(err)
	}

	output, err := handle.GetResult()
	if err != nil {
		t.Fatal(err)
	}
	if output != "approved" {
		t.Fatalf("expected 'approved', got %q", output)
	}
	if !capturedResult.Approved {
		t.Fatal("expected ApprovalResult.Approved to be true")
	}
	if capturedResult.Comment != "looks good" {
		t.Fatalf("expected comment 'looks good', got %q", capturedResult.Comment)
	}
}

func TestWaitForApproval_Rejected(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	wf := func(ctx Context, input string) (string, error) {
		result, err := WaitForApproval(ctx)
		if err != nil {
			return "", err
		}
		if !result.Approved {
			return "rejected", nil
		}
		return "approved", nil
	}

	Register(rt, wf, WithName("rejection-test"))
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	handle, err := Run(rt, wf, "test-input")
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	ctx := rt.NewContext(context.Background())
	err = Send(ctx, handle.GetWorkflowID(), ApprovalResult{
		Approved: false,
		Comment:  "needs changes",
	}, approvalTopic)
	if err != nil {
		t.Fatal(err)
	}

	output, err := handle.GetResult()
	if err != nil {
		t.Fatal(err)
	}
	if output != "rejected" {
		t.Fatalf("expected 'rejected', got %q", output)
	}
}

func TestWaitForApproval_WithTimeout(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	wf := func(ctx Context, input string) (string, error) {
		_, err := WaitForApproval(ctx, WithApprovalTimeout(200*time.Millisecond))
		if err != nil {
			return "timed out", nil
		}
		return "approved", nil
	}

	Register(rt, wf, WithName("timeout-test"))
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	handle, err := Run(rt, wf, "test-input")
	if err != nil {
		t.Fatal(err)
	}

	output, err := handle.GetResult()
	if err != nil {
		t.Fatal(err)
	}
	if output != "timed out" {
		t.Fatalf("expected 'timed out', got %q", output)
	}
}
