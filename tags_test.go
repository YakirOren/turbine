package turbine

import (
	"testing"
)

func TestWithTags_IncludedInRegisteredWorkflows(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	wf := func(ctx Context, input string) (string, error) {
		return input, nil
	}

	Register(rt, wf, WithName("tagged-wf"), WithTags("deploy", "critical"))

	registered := rt.RegisteredWorkflows()
	if len(registered) != 1 {
		t.Fatalf("expected 1 registered workflow, got %d", len(registered))
	}
	r := registered[0]
	if len(r.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(r.Tags))
	}
	if r.Tags[0] != "deploy" || r.Tags[1] != "critical" {
		t.Fatalf("unexpected tags: %v", r.Tags)
	}
}

func TestWithTags_StoredOnWorkflowStatus(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	wf := func(ctx Context, input string) (string, error) {
		return "done", nil
	}

	Register(rt, wf, WithName("tagged-run"), WithTags("api", "v2"))
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rt.Shutdown() }()

	handle, err := Run(rt, wf, "test")
	if err != nil {
		t.Fatal(err)
	}

	_, err = handle.GetResult()
	if err != nil {
		t.Fatal(err)
	}

	status, err := handle.GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Tags) != 2 {
		t.Fatalf("expected 2 tags on status, got %d", len(status.Tags))
	}
}
