package turbine

import (
	"strings"
	"testing"
)

func TestWithSummaryFunc_StoredOnWorkflowStatus(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	type orderInput struct {
		ID       string
		Customer string
	}

	wf := func(ctx Context, input orderInput) (string, error) {
		return "done", nil
	}

	Register(rt, wf, WithName("summary-wf"), WithSummaryFunc(func(in orderInput) string {
		return "Order " + in.ID + " for " + in.Customer
	}))

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rt.Shutdown() }()

	handle, err := Run(rt, wf, orderInput{ID: "123", Customer: "Alice"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := handle.GetResult(); err != nil {
		t.Fatal(err)
	}

	status, err := handle.GetStatus()
	if err != nil {
		t.Fatal(err)
	}

	expected := "Order 123 for Alice"
	if status.Summary != expected {
		t.Fatalf("expected summary %q, got %q", expected, status.Summary)
	}
}

func TestWithSummaryFunc_NotSet(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	wf := func(ctx Context, input string) (string, error) {
		return "done", nil
	}

	Register(rt, wf, WithName("no-summary-wf"))

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rt.Shutdown() }()

	handle, err := Run(rt, wf, "test")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := handle.GetResult(); err != nil {
		t.Fatal(err)
	}

	status, err := handle.GetStatus()
	if err != nil {
		t.Fatal(err)
	}

	if status.Summary != "" {
		t.Fatalf("expected empty summary, got %q", status.Summary)
	}
}

func TestWithSummaryFunc_Truncation(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	wf := func(ctx Context, input string) (string, error) {
		return "done", nil
	}

	Register(rt, wf, WithName("long-summary-wf"), WithSummaryFunc(func(in string) string {
		return strings.Repeat("a", 300)
	}))

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rt.Shutdown() }()

	handle, err := Run(rt, wf, "test")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := handle.GetResult(); err != nil {
		t.Fatal(err)
	}

	status, err := handle.GetStatus()
	if err != nil {
		t.Fatal(err)
	}

	if len(status.Summary) != maxSummaryLength {
		t.Fatalf("expected summary length %d, got %d", maxSummaryLength, len(status.Summary))
	}
}

func TestWithSummaryFunc_PanicRecovery(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	wf := func(ctx Context, input string) (string, error) {
		return "done", nil
	}

	Register(rt, wf, WithName("panic-summary-wf"), WithSummaryFunc(func(in string) string {
		panic("boom")
	}))

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rt.Shutdown() }()

	handle, err := Run(rt, wf, "test")
	if err != nil {
		t.Fatal(err)
	}

	result, err := handle.GetResult()
	if err != nil {
		t.Fatalf("workflow should succeed despite panic in summary func: %v", err)
	}
	if result != "done" {
		t.Fatalf("expected 'done', got %q", result)
	}

	status, err := handle.GetStatus()
	if err != nil {
		t.Fatal(err)
	}

	if status.Summary != "" {
		t.Fatalf("expected empty summary after panic, got %q", status.Summary)
	}
}
