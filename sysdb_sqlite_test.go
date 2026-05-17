package turbine

import (
	"context"
	"testing"
	"time"

	"github.com/pocketbase/dbx"
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
	sysDB := newSQLiteSysDB(app, eb)
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

	startedAt := time.Now().UnixMilli()
	if err := sysDB.recordOperationStart(context.Background(), recordOperationStartDBInput{
		workflowUUID: wfID,
		functionID:   1,
		functionName: "myStep",
		startedAt:    startedAt,
	}); err != nil {
		t.Fatalf("recordOperationStart failed: %v", err)
	}

	outputStr := `"hello world"`
	recordInput := recordOperationResultDBInput{
		workflowUUID: wfID,
		functionID:   1,
		functionName: "myStep",
		output:       &outputStr,
		startedAt:    startedAt,
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
		return
	}
	if result.output == nil || *result.output != outputStr {
		t.Fatalf("expected output %q, got %v", outputStr, result.output)
	}
}

func TestCheckOperationExecutionRecoversCrashedStep(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	wfID := "wf-op-crash-1"
	if _, err := sysDB.insertStatus(context.Background(), insertStatusDBInput{
		status: makeStatus(wfID),
	}); err != nil {
		t.Fatal(err)
	}

	// Simulate a crash: start was recorded, result never arrived.
	if err := sysDB.recordOperationStart(context.Background(), recordOperationStartDBInput{
		workflowUUID: wfID,
		functionID:   1,
		functionName: "myStep",
		startedAt:    time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("recordOperationStart failed: %v", err)
	}

	result, err := sysDB.checkOperationExecution(context.Background(), checkOperationExecutionDBInput{
		workflowUUID: wfID,
		functionID:   1,
	})
	if err != nil {
		t.Fatalf("checkOperationExecution failed: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result (step should re-execute), got %+v", result)
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

	_, err = sysDB.cancelWorkflow(context.Background(), cancelWorkflowDBInput{workflowID: wfID})
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

func TestSetAndGetValue(t *testing.T) {
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

func TestSetValueIdempotent(t *testing.T) {
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

func TestGetValueTimeout(t *testing.T) {
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

	out1 := `"step1"`
	out2 := `"step2"`
	for i, out := range []*string{&out1, &out2} {
		startedAt := time.Now().UnixMilli()
		if err := sysDB.recordOperationStart(context.Background(), recordOperationStartDBInput{
			workflowUUID: wfID,
			functionID:   i + 1,
			functionName: "step",
			startedAt:    startedAt,
		}); err != nil {
			t.Fatalf("record start %d: %v", i+1, err)
		}
		err = sysDB.recordOperationResult(context.Background(), recordOperationResultDBInput{
			workflowUUID: wfID,
			functionID:   i + 1,
			functionName: "step",
			output:       out,
			startedAt:    startedAt,
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

func TestUpdateAppStatus(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	wfID := "wf-app-status-1"
	input := insertStatusDBInput{
		status: makeStatus(wfID),
	}
	_, err := sysDB.insertStatus(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	// Set app status
	err = sysDB.updateAppStatus(context.Background(), updateAppStatusDBInput{
		workflowID:     wfID,
		appStatus:      "pending-approval",
		appStatusColor: "yellow",
	})
	if err != nil {
		t.Fatalf("updateAppStatus failed: %v", err)
	}

	// Verify via listWorkflows
	workflows, err := sysDB.listWorkflows(context.Background(), listWorkflowsDBInput{
		workflowIDs: []string{wfID},
	})
	if err != nil {
		t.Fatalf("listWorkflows failed: %v", err)
	}
	if len(workflows) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(workflows))
	}
	if workflows[0].AppStatus != "pending-approval" {
		t.Fatalf("expected app_status 'pending-approval', got %q", workflows[0].AppStatus)
	}
	if workflows[0].AppStatusColor != "yellow" {
		t.Fatalf("expected app_status_color 'yellow', got %q", workflows[0].AppStatusColor)
	}
}

func TestValidateAppStatusColor(t *testing.T) {
	for _, color := range []string{"green", "red", "yellow", "blue", "gray", "lime", "orange", "purple", "pink", "cyan"} {
		if err := validateAppStatusColor(color); err != nil {
			t.Fatalf("expected valid color %q, got error: %v", color, err)
		}
	}
	for _, color := range []string{"", "neon", "darkblue", "GREEN"} {
		if err := validateAppStatusColor(color); err == nil {
			t.Fatalf("expected error for invalid color %q", color)
		}
	}
}

func TestKVSetAndGet(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	val := `"hello"`
	err := sysDB.setKV(context.Background(), setKVInput{key: "test-key", value: &val})
	if err != nil {
		t.Fatalf("setKV failed: %v", err)
	}

	result, err := sysDB.getKV(context.Background(), getKVInput{key: "test-key"})
	if err != nil {
		t.Fatalf("getKV failed: %v", err)
	}
	if result == nil || *result != val {
		t.Fatalf("expected %q, got %v", val, result)
	}
}

func TestKVGetMissing(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	result, err := sysDB.getKV(context.Background(), getKVInput{key: "nonexistent"})
	if err != nil {
		t.Fatalf("getKV should not error on missing key: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil for missing key, got %v", result)
	}
}

func TestKVSetOverwrite(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	val1 := `"first"`
	val2 := `"second"`

	err := sysDB.setKV(context.Background(), setKVInput{key: "overwrite-key", value: &val1})
	if err != nil {
		t.Fatal(err)
	}

	err = sysDB.setKV(context.Background(), setKVInput{key: "overwrite-key", value: &val2})
	if err != nil {
		t.Fatal(err)
	}

	result, err := sysDB.getKV(context.Background(), getKVInput{key: "overwrite-key"})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || *result != val2 {
		t.Fatalf("expected %q, got %v", val2, result)
	}
}

func TestKVDelete(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	val := `"to-delete"`
	err := sysDB.setKV(context.Background(), setKVInput{key: "del-key", value: &val})
	if err != nil {
		t.Fatal(err)
	}

	err = sysDB.deleteKV(context.Background(), deleteKVInput{key: "del-key"})
	if err != nil {
		t.Fatalf("deleteKV failed: %v", err)
	}

	result, err := sysDB.getKV(context.Background(), getKVInput{key: "del-key"})
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fatalf("expected nil after delete, got %v", result)
	}
}

func TestKVDeleteMissing(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	err := sysDB.deleteKV(context.Background(), deleteKVInput{key: "never-existed"})
	if err != nil {
		t.Fatalf("deleteKV should not error on missing key: %v", err)
	}
}

func TestKVPublicAPIRoundTrip(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	rt := &Runtime{systemDB: sysDB}

	type config struct {
		RateLimit int    `json:"rate_limit"`
		Endpoint  string `json:"endpoint"`
	}
	err := rt.KVSet(context.Background(), "config:api", config{RateLimit: 100, Endpoint: "/api"})
	if err != nil {
		t.Fatalf("KVSet failed: %v", err)
	}

	val, ok, err := KVGet[config](rt, context.Background(), "config:api")
	if err != nil {
		t.Fatalf("KVGet failed: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if val.RateLimit != 100 || val.Endpoint != "/api" {
		t.Fatalf("unexpected value: %+v", val)
	}
}

func TestKVSetEmptyKey(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	rt := &Runtime{systemDB: sysDB}
	err := rt.KVSet(context.Background(), "", "value")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestKVSetNilValue(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	rt := &Runtime{systemDB: sysDB}
	err := rt.KVSet(context.Background(), "nil-key", nil)
	if err == nil {
		t.Fatal("expected error for nil value")
	}
}

func TestKVGetNotFound(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	rt := &Runtime{systemDB: sysDB}
	val, ok, err := KVGet[string](rt, context.Background(), "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for missing key")
	}
	if val != "" {
		t.Fatalf("expected zero value, got %q", val)
	}
}

func TestKVGetTypeMismatch(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	rt := &Runtime{systemDB: sysDB}

	err := rt.KVSet(context.Background(), "type-key", "hello")
	if err != nil {
		t.Fatal(err)
	}

	// Try to get as []int — json.Unmarshal will fail on "hello" → []int
	_, _, err = KVGet[[]int](rt, context.Background(), "type-key")
	if err == nil {
		t.Fatal("expected deserialization error for type mismatch")
	}
}

func TestKVRoundTripInt(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	rt := &Runtime{systemDB: sysDB}

	err := rt.KVSet(context.Background(), "counter", 42)
	if err != nil {
		t.Fatalf("KVSet failed: %v", err)
	}

	val, ok, err := KVGet[int](rt, context.Background(), "counter")
	if err != nil {
		t.Fatalf("KVGet failed: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if val != 42 {
		t.Fatalf("expected 42, got %d", val)
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

func TestCancelWorkflowClearsDedupAndQueue(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	wfID := "wf-cancel-dedup-1"
	s := makeStatus(wfID)
	s.Status = StatusEnqueued
	s.QueueName = "my-queue"
	s.DeduplicationID = "dedup-abc"
	input := insertStatusDBInput{status: s}
	_, err := sysDB.insertStatus(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	_, err = sysDB.cancelWorkflow(context.Background(), cancelWorkflowDBInput{workflowID: wfID})
	if err != nil {
		t.Fatalf("cancelWorkflow failed: %v", err)
	}

	// Verify dedup_id and queue_name were cleared
	var dedupID, queueName string
	err = sysDB.app.DB().Select("deduplication_id", "queue_name").
		From("pt_workflow_status").
		Where(dbx.HashExp{"id": wfID}).
		Row(&dedupID, &queueName)
	if err != nil {
		t.Fatalf("failed to query workflow: %v", err)
	}
	if dedupID != "" {
		t.Fatalf("expected deduplication_id to be cleared, got %q", dedupID)
	}
	if queueName != "" {
		t.Fatalf("expected queue_name to be cleared, got %q", queueName)
	}

	// Verify we can now enqueue a new workflow with the same dedup ID
	wfID2 := "wf-cancel-dedup-2"
	s2 := makeStatus(wfID2)
	s2.Status = StatusEnqueued
	s2.QueueName = "my-queue"
	s2.DeduplicationID = "dedup-abc"
	_, err = sysDB.insertStatus(context.Background(), insertStatusDBInput{status: s2})
	if err != nil {
		t.Fatalf("expected to enqueue with same dedup ID after cancel, got: %v", err)
	}
}

func TestGetQueuePartitionsExcludesTerminal(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	// Insert an ENQUEUED workflow with a partition key
	s1 := makeStatus("wf-partition-1")
	s1.Status = StatusEnqueued
	s1.QueueName = "partitioned-q"
	s1.QueuePartitionKey = "tenant-a"
	_, err := sysDB.insertStatus(context.Background(), insertStatusDBInput{status: s1})
	if err != nil {
		t.Fatal(err)
	}

	// Insert a CANCELLED workflow with a different partition key
	s2 := makeStatus("wf-partition-2")
	s2.Status = StatusEnqueued
	s2.QueueName = "partitioned-q"
	s2.QueuePartitionKey = "tenant-b"
	_, err = sysDB.insertStatus(context.Background(), insertStatusDBInput{status: s2})
	if err != nil {
		t.Fatal(err)
	}
	_, err = sysDB.cancelWorkflow(context.Background(), cancelWorkflowDBInput{workflowID: "wf-partition-2"})
	if err != nil {
		t.Fatal(err)
	}

	partitions, err := sysDB.getQueuePartitions(context.Background(), "partitioned-q")
	if err != nil {
		t.Fatalf("getQueuePartitions failed: %v", err)
	}

	// Should only contain tenant-a, not tenant-b (cancelled)
	if len(partitions) != 1 {
		t.Fatalf("expected 1 partition, got %d: %v", len(partitions), partitions)
	}
	if partitions[0] != "tenant-a" {
		t.Fatalf("expected partition tenant-a, got %s", partitions[0])
	}
}
