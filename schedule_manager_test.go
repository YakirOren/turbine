package pocketflow

import (
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"
)

func TestScheduleManagerOnceFiresAndCleansUp(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	var executed atomic.Bool
	myWF := func(ctx Context, input map[string]any) (string, error) {
		executed.Store(true)
		return "ok", nil
	}
	Register(rt, myWF, WithName("once-wf"), WithDashboardTrigger())

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	fqn := resolveWorkflowFunctionName(myWF)
	scheduledAt := time.Now().Add(1 * time.Second)
	record, err := createScheduleRecord(rt.app, fqn, json.RawMessage(`{"test":true}`), "once", "", scheduledAt)
	if err != nil {
		t.Fatal(err)
	}

	rt.scheduleManager.registerOnce(rt, record.Id, fqn, json.RawMessage(`{"test":true}`), scheduledAt)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if executed.Load() {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !executed.Load() {
		t.Fatal("one-time schedule was not executed within timeout")
	}

	time.Sleep(500 * time.Millisecond)
	_, err = rt.app.FindRecordById(collectionSchedules, record.Id)
	if err == nil {
		t.Fatal("expected schedule record to be deleted after execution")
	}
}

func TestScheduleManagerOncePastFiresImmediately(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	var executed atomic.Bool
	myWF := func(ctx Context, input map[string]any) (string, error) {
		executed.Store(true)
		return "ok", nil
	}
	Register(rt, myWF, WithName("past-wf"), WithDashboardTrigger())

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	fqn := resolveWorkflowFunctionName(myWF)
	pastTime := time.Now().Add(-1 * time.Hour)

	record, err := createScheduleRecord(rt.app, fqn, json.RawMessage(`{}`), "once", "", pastTime)
	if err != nil {
		t.Fatal(err)
	}

	rt.scheduleManager.registerOnce(rt, record.Id, fqn, json.RawMessage(`{}`), pastTime)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if executed.Load() {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !executed.Load() {
		t.Fatal("past schedule was not fired immediately")
	}
}
