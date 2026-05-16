package turbine

import (
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/tests"
)

func TestRegisteredWorkflowsReturnsAllWorkflows(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	rt := NewRuntime(app, Config{})

	triggerableWF := func(ctx Context, input string) (string, error) {
		return input, nil
	}
	normalWF := func(ctx Context, input int) (int, error) {
		return input, nil
	}

	Register(rt, triggerableWF, WithName("triggerable-wf"), WithDashboardTrigger())
	Register(rt, normalWF, WithName("normal-wf"))

	workflows := rt.RegisteredWorkflows()

	if len(workflows) != 2 {
		t.Fatalf("expected 2 workflows, got %d", len(workflows))
	}

	found := map[string]RegisteredWorkflow{}
	for _, wf := range workflows {
		found[wf.Name] = wf
	}

	tw, ok := found["triggerable-wf"]
	if !ok {
		t.Fatal("triggerable-wf not found")
	}
	if !tw.Triggerable {
		t.Error("expected triggerable-wf to be triggerable")
	}

	nw, ok := found["normal-wf"]
	if !ok {
		t.Fatal("normal-wf not found")
	}
	if nw.Triggerable {
		t.Error("expected normal-wf to not be triggerable")
	}
}

func TestTriggerByFQN(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	var received atomic.Value

	myWF := func(ctx Context, input map[string]any) (string, error) {
		received.Store(input)
		return "done", nil
	}

	fqn := resolveWorkflowFunctionName(myWF)
	Register(rt, myWF, WithDashboardTrigger())

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown()

	rawInput := json.RawMessage(`{"key":"value","num":42}`)
	workflowID, err := rt.TriggerByFQN(fqn, rawInput)
	if err != nil {
		t.Fatalf("TriggerByFQN failed: %v", err)
	}
	if workflowID == "" {
		t.Fatal("expected non-empty workflow ID")
	}

	// Wait for the workflow to execute
	deadline := time.After(5 * time.Second)
	for {
		val := received.Load()
		if val != nil {
			got := val.(map[string]any)
			if got["key"] != "value" {
				t.Errorf("expected key=value, got key=%v", got["key"])
			}
			if got["num"] != float64(42) {
				t.Errorf("expected num=42, got num=%v", got["num"])
			}
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for workflow execution")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestTriggerByFQNRejectsNonTriggerable(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx Context, input string) (string, error) {
		return input, nil
	}

	fqn := resolveWorkflowFunctionName(myWF)
	Register(rt, myWF) // no WithDashboardTrigger

	_, err := rt.TriggerByFQN(fqn, json.RawMessage(`"hello"`))
	if err == nil {
		t.Fatal("expected error for non-triggerable workflow")
	}
}
