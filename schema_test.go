package turbine

import (
	"testing"
)

func TestWithInputSchema_IncludedInRegisteredWorkflows(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	schema := map[string]any{
		"fields": []map[string]any{
			{"name": "url", "type": "string", "label": "Target URL", "required": true},
			{"name": "count", "type": "number", "label": "Retry Count", "default": 3},
		},
	}

	wf := func(ctx Context, input string) (string, error) {
		return input, nil
	}

	Register(rt, wf, WithName("schema-test"), WithDashboardTrigger(), WithInputSchema(schema))

	registered := rt.RegisteredWorkflows()
	if len(registered) != 1 {
		t.Fatalf("expected 1 registered workflow, got %d", len(registered))
	}

	r := registered[0]
	if r.InputSchema == nil {
		t.Fatal("expected InputSchema to be set")
	}
	fields, ok := r.InputSchema["fields"]
	if !ok {
		t.Fatal("expected 'fields' key in schema")
	}
	fieldSlice, ok := fields.([]map[string]any)
	if !ok {
		t.Fatalf("expected []map[string]any, got %T", fields)
	}
	if len(fieldSlice) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(fieldSlice))
	}
	if fieldSlice[0]["name"] != "url" {
		t.Fatalf("expected first field name 'url', got %q", fieldSlice[0]["name"])
	}
}

func TestWithoutInputSchema_NilInRegisteredWorkflows(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	wf := func(ctx Context, input string) (string, error) {
		return input, nil
	}

	Register(rt, wf, WithName("no-schema-test"), WithDashboardTrigger())

	registered := rt.RegisteredWorkflows()
	if len(registered) != 1 {
		t.Fatalf("expected 1 registered workflow, got %d", len(registered))
	}
	if registered[0].InputSchema != nil {
		t.Fatal("expected InputSchema to be nil")
	}
}
