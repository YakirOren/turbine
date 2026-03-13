package pocketflow

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/tests"
)

// helper: creates a test app with collections and a sysdb instance.
func setupSysDB(t *testing.T) (*sqliteSysDB, func()) {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	eb := newEventBus()
	sysDB := newSQLiteSysDB(app, eb, slog.Default())
	sysDB.launch(context.Background())
	return sysDB, app.Cleanup
}

func makeStatus(id string) Status {
	return Status{
		ID:         id,
		Status:     StatusPending,
		Name:       "testWorkflow",
		ExecutorID: "local",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

func TestInsertStatus(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	wfID := "wf-insert-test-1"
	input := insertStatusDBInput{
		status: makeStatus(wfID),
	}
	result, err := sysDB.insertStatus(context.Background(), input)
	if err != nil {
		t.Fatalf("insertStatus failed: %v", err)
	}
	if result.status != StatusPending {
		t.Fatalf("expected PENDING, got %s", result.status)
	}
	if result.name != "testWorkflow" {
		t.Fatalf("expected name testWorkflow, got %s", result.name)
	}
	if result.attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", result.attempts)
	}
}

func TestInsertStatusIdempotent(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	wfID := "wf-idempotent-1"
	input := insertStatusDBInput{
		status: makeStatus(wfID),
	}

	_, err := sysDB.insertStatus(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	// Insert again — should succeed (ON CONFLICT DO UPDATE)
	result, err := sysDB.insertStatus(context.Background(), input)
	if err != nil {
		t.Fatalf("second insert failed: %v", err)
	}
	if result.status != StatusPending {
		t.Fatalf("expected PENDING, got %s", result.status)
	}
}

func TestInsertStatusConflictingName(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	wfID := "wf-conflict-name-1"
	input := insertStatusDBInput{
		status: makeStatus(wfID),
	}
	_, err := sysDB.insertStatus(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	// Insert with different name — should error
	input.status.Name = "differentWorkflow"
	_, err = sysDB.insertStatus(context.Background(), input)
	if err == nil {
		t.Fatal("expected conflicting workflow error, got nil")
	}
}

func TestRecordAndCheckOperationResult(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	wfID := "wf-op-test-1"
	input := insertStatusDBInput{
		status: makeStatus(wfID),
	}
	_, err := sysDB.insertStatus(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	// Record an operation result
	outputStr := `"hello world"`
	recordInput := recordOperationResultDBInput{
		workflowUUID: wfID,
		functionID:   1,
		functionName: "myStep",
		output:       &outputStr,
		startedAt:    time.Now().UnixMilli(),
		endedAt:      time.Now().UnixMilli(),
	}
	err = sysDB.recordOperationResult(context.Background(), recordInput)
	if err != nil {
		t.Fatalf("recordOperationResult failed: %v", err)
	}

	// Check operation execution
	checkInput := checkOperationExecutionDBInput{
		workflowUUID: wfID,
		functionID:   1,
	}
	result, err := sysDB.checkOperationExecution(context.Background(), checkInput)
	if err != nil {
		t.Fatalf("checkOperationExecution failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected recorded result, got nil")
	}
	if result.output == nil || *result.output != outputStr {
		t.Fatalf("expected output %q, got %v", outputStr, result.output)
	}
}

func TestCheckOperationExecutionNotFound(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	// Create a workflow first so the status check passes
	wfID := "wf-check-notfound-1"
	input := insertStatusDBInput{
		status: makeStatus(wfID),
	}
	_, err := sysDB.insertStatus(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	// Check for a non-recorded operation on that workflow
	checkInput := checkOperationExecutionDBInput{
		workflowUUID: wfID,
		functionID:   999,
	}
	result, err := sysDB.checkOperationExecution(context.Background(), checkInput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil for nonexistent operation")
	}
}

func TestUpdateWorkflowOutcome(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	wfID := "wf-outcome-1"
	input := insertStatusDBInput{
		status: makeStatus(wfID),
	}
	_, err := sysDB.insertStatus(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	outputStr := `{"result": 42}`
	err = sysDB.updateWorkflowOutcome(context.Background(), updateWorkflowOutcomeDBInput{
		workflowID: wfID,
		status:     StatusSuccess,
		output:     &outputStr,
	})
	if err != nil {
		t.Fatalf("updateWorkflowOutcome failed: %v", err)
	}
}

func TestCancelWorkflow(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	wfID := "wf-cancel-1"
	input := insertStatusDBInput{
		status: makeStatus(wfID),
	}
	_, err := sysDB.insertStatus(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	err = sysDB.cancelWorkflow(context.Background(), cancelWorkflowDBInput{workflowID: wfID})
	if err != nil {
		t.Fatalf("cancelWorkflow failed: %v", err)
	}
}

func TestSendAndRecv(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	destWfID := "wf-recv-1"
	// Create destination workflow
	input := insertStatusDBInput{
		status: makeStatus(destWfID),
	}
	_, err := sysDB.insertStatus(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	// Send a message
	msg := `"hello"`
	sendInput := SendInput{
		DestinationUUID: destWfID,
		Topic:           "greet",
		Message:         &msg,
	}
	err = sysDB.send(context.Background(), sendInput)
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	// Recv the message (with short timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	recvIn := recvInput{
		workflowUUID: destWfID,
		topic:        "greet",
		timeout:      2 * time.Second,
	}
	result, err := sysDB.recv(ctx, recvIn)
	if err != nil {
		t.Fatalf("recv failed: %v", err)
	}
	if result == nil || *result != msg {
		t.Fatalf("expected %q, got %v", msg, result)
	}
}

func TestRecvTimeout(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	recvIn := recvInput{
		workflowUUID: "nonexistent-wf",
		topic:        "nothing",
		timeout:      0, // immediate
	}
	result, err := sysDB.recv(ctx, recvIn)
	if err != nil {
		t.Fatalf("recv should not error on timeout, got: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil on timeout, got %v", result)
	}
}

func TestSetAndGetEvent(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	wfID := "wf-event-1"
	input := insertStatusDBInput{
		status: makeStatus(wfID),
	}
	_, err := sysDB.insertStatus(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	// Set event
	val := `"event_value"`
	setIn := SetValueInput{
		WorkflowUUID: wfID,
		Key:          "myKey",
		Value:        &val,
	}
	err = sysDB.setEvent(context.Background(), setIn)
	if err != nil {
		t.Fatalf("setEvent failed: %v", err)
	}

	// Get event (with short timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	getIn := getEventInput{
		targetWorkflowUUID: wfID,
		key:                "myKey",
		timeout:            2 * time.Second,
	}
	result, err := sysDB.getEvent(ctx, getIn)
	if err != nil {
		t.Fatalf("getEvent failed: %v", err)
	}
	if result == nil || *result != val {
		t.Fatalf("expected %q, got %v", val, result)
	}
}

func TestSetEventIdempotent(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	wfID := "wf-event-idem-1"
	input := insertStatusDBInput{
		status: makeStatus(wfID),
	}
	_, err := sysDB.insertStatus(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	val := `"first"`
	setIn := SetValueInput{
		WorkflowUUID: wfID,
		Key:          "k1",
		Value:        &val,
	}
	if err := sysDB.setEvent(context.Background(), setIn); err != nil {
		t.Fatal(err)
	}
	// Set again with different value — ON CONFLICT DO UPDATE
	val2 := `"second"`
	setIn.Value = &val2
	if err := sysDB.setEvent(context.Background(), setIn); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	getIn := getEventInput{
		targetWorkflowUUID: wfID,
		key:                "k1",
		timeout:            1 * time.Second,
	}
	result, err := sysDB.getEvent(ctx, getIn)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || *result != val2 {
		t.Fatalf("expected updated value %q, got %v", val2, result)
	}
}

func TestGetEventTimeout(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	getIn := getEventInput{
		targetWorkflowUUID: "nonexistent-wf",
		key:                "nokey",
		timeout:            0,
	}
	result, err := sysDB.getEvent(ctx, getIn)
	if err != nil {
		t.Fatalf("getEvent should not error on timeout, got: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil on timeout, got %v", result)
	}
}

func TestGetWorkflowSteps(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	wfID := "wf-steps-1"
	input := insertStatusDBInput{
		status: makeStatus(wfID),
	}
	_, err := sysDB.insertStatus(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	// Record two operations
	out1 := `"step1"`
	out2 := `"step2"`
	for i, out := range []*string{&out1, &out2} {
		err = sysDB.recordOperationResult(context.Background(), recordOperationResultDBInput{
			workflowUUID: wfID,
			functionID:   i + 1,
			functionName: "step",
			output:       out,
			startedAt:    time.Now().UnixMilli(),
			endedAt:      time.Now().UnixMilli(),
		})
		if err != nil {
			t.Fatalf("record step %d: %v", i+1, err)
		}
	}

	steps, err := sysDB.getWorkflowSteps(context.Background(), getWorkflowStepsInput{workflowID: wfID})
	if err != nil {
		t.Fatalf("getWorkflowSteps failed: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if steps[0].functionID != 1 || steps[1].functionID != 2 {
		t.Fatalf("unexpected step order: %v", steps)
	}
}

func TestGarbageCollectWorkflows(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	// Insert a completed workflow
	wfID := "wf-gc-1"
	input := insertStatusDBInput{
		status: makeStatus(wfID),
	}
	_, err := sysDB.insertStatus(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	// Mark as success
	out := `"done"`
	err = sysDB.updateWorkflowOutcome(context.Background(), updateWorkflowOutcomeDBInput{
		workflowID: wfID,
		status:     StatusSuccess,
		output:     &out,
	})
	if err != nil {
		t.Fatal(err)
	}

	// GC with cutoff in the future — should delete completed workflows
	cutoff := time.Now().Add(1 * time.Minute)
	gcInput := garbageCollectWorkflowsInput{
		cutoffTime: cutoff,
	}
	err = sysDB.garbageCollectWorkflows(context.Background(), gcInput)
	if err != nil {
		t.Fatalf("garbageCollectWorkflows failed: %v", err)
	}
}
