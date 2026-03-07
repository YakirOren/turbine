package pbdbos

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

const (
	_DBOS_INTERNAL_QUEUE_NAME = "_dbos_internal_queue"
	_DB_RETRY_INTERVAL        = 1 * time.Second
)

type sqliteSysDB struct {
	app      core.App
	eventBus *eventBus
	logger   *slog.Logger
	launched bool
}

func newSQLiteSysDB(app core.App, eb *eventBus, logger *slog.Logger) *sqliteSysDB {
	if logger == nil {
		logger = slog.Default()
	}
	return &sqliteSysDB{
		app:      app,
		eventBus: eb,
		logger:   logger.With("service", "sysdb_sqlite"),
	}
}

func (s *sqliteSysDB) launch(ctx context.Context) {
	s.launched = true
	s.logger.Debug("SQLite system database launched")
}

func (s *sqliteSysDB) shutdown(ctx context.Context, timeout time.Duration) {
	s.launched = false
	s.logger.Debug("SQLite system database shut down")
}

// derefStr converts a *string to string, returning "" for nil.
// PocketBase TextFields are NOT NULL with default "", so we must never bind nil.
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// isSQLiteUniqueViolation checks if an error is a SQLite UNIQUE constraint violation.
func isSQLiteUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

/*******************************/
/******* WORKFLOWS ********/
/*******************************/

func (s *sqliteSysDB) insertWorkflowStatus(ctx context.Context, input insertWorkflowStatusDBInput) (*insertWorkflowResult, error) {
	attempts := 1
	if input.status.Status == WorkflowStatusEnqueued {
		attempts = 0
	}

	updatedAt := time.Now()
	if !input.status.UpdatedAt.IsZero() {
		updatedAt = input.status.UpdatedAt
	}

	var deadline int64
	if !input.status.Deadline.IsZero() {
		deadline = input.status.Deadline.UnixMilli()
	}

	var timeoutMs int64
	if input.status.Timeout > 0 {
		timeoutMs = input.status.Timeout.Round(time.Millisecond).Milliseconds()
	}

	applicationVersion := input.status.ApplicationVersion

	deduplicationID := input.status.DeduplicationID
	queuePartitionKey := input.status.QueuePartitionKey
	parentWorkflowID := input.status.ParentWorkflowID

	recoveryIncrement := 0
	if input.incrementAttempts {
		recoveryIncrement = 1
	}

	query := `INSERT INTO dbos_workflow_status (
		id, status, name, queue_name,
		executor_id, application_version, application_id,
		created_at_epoch_ms, recovery_attempts, updated_at_epoch_ms,
		workflow_timeout_ms, workflow_deadline_epoch_ms,
		inputs, deduplication_id, priority, queue_partition_key,
		owner_xid, parent_workflow_uuid
	) VALUES(
		{:id}, {:status}, {:name}, {:queue_name},
		{:executor_id}, {:app_version}, {:app_id},
		{:created_at}, {:attempts}, {:updated_at},
		{:timeout_ms}, {:deadline_ms},
		{:inputs}, {:dedup_id}, {:priority}, {:partition_key},
		{:owner_xid}, {:parent_wf_id}
	)
	ON CONFLICT (id)
	DO UPDATE SET
		recovery_attempts = CASE
			WHEN excluded.status != {:enqueued_status1} THEN dbos_workflow_status.recovery_attempts + {:recovery_inc}
			ELSE dbos_workflow_status.recovery_attempts
		END,
		updated_at_epoch_ms = excluded.updated_at_epoch_ms,
		executor_id = CASE
			WHEN excluded.status = {:enqueued_status2} THEN dbos_workflow_status.executor_id
			ELSE excluded.executor_id
		END
	RETURNING recovery_attempts, status, name, queue_name, workflow_timeout_ms, workflow_deadline_epoch_ms, owner_xid`

	var result insertWorkflowResult
	var timeoutMSResult int64
	var workflowDeadlineEpochMS int64
	var queueNameReturn, ownerXIDReturn string

	ownerXID := ""
	if input.ownerXID != nil {
		ownerXID = *input.ownerXID
	}

	// PocketBase TextFields are NOT NULL (default ""), so coerce nil → ""
	inputs := ""
	if input.status.Input != nil {
		switch v := input.status.Input.(type) {
		case string:
			inputs = v
		case *string:
			if v != nil {
				inputs = *v
			}
		}
	}

	err := s.app.DB().NewQuery(query).Bind(dbx.Params{
		"id":               input.status.ID,
		"status":           input.status.Status,
		"name":             input.status.Name,
		"queue_name":       input.status.QueueName,
		"executor_id":      input.status.ExecutorID,
		"app_version":      applicationVersion,
		"app_id":           input.status.ApplicationID,
		"created_at":       input.status.CreatedAt.Round(time.Millisecond).UnixMilli(),
		"attempts":         attempts,
		"updated_at":       updatedAt.UnixMilli(),
		"timeout_ms":       timeoutMs,
		"deadline_ms":      deadline,
		"inputs":           inputs,
		"dedup_id":         deduplicationID,
		"priority":         input.status.Priority,
		"partition_key":    queuePartitionKey,
		"owner_xid":        ownerXID,
		"parent_wf_id":     parentWorkflowID,
		"enqueued_status1":  string(WorkflowStatusEnqueued),
		"enqueued_status2":  string(WorkflowStatusEnqueued),
		"recovery_inc":     recoveryIncrement,
	}).Row(&result.attempts, &result.status, &result.name, &queueNameReturn, &timeoutMSResult, &workflowDeadlineEpochMS, &ownerXIDReturn)

	result.ownerXID = ownerXIDReturn
	if queueNameReturn != "" {
		result.queueName = &queueNameReturn
	}
	if err != nil {
		if isSQLiteUniqueViolation(err) {
			return nil, newQueueDeduplicatedError(input.status.ID, input.status.QueueName, input.status.DeduplicationID)
		}
		return nil, fmt.Errorf("failed to insert workflow status: %w", err)
	}

	if timeoutMSResult > 0 {
		result.timeout = time.Duration(timeoutMSResult) * time.Millisecond
	}
	if workflowDeadlineEpochMS > 0 {
		result.workflowDeadline = time.Unix(0, workflowDeadlineEpochMS*int64(time.Millisecond))
	}

	if len(input.status.Name) > 0 && result.name != input.status.Name {
		return nil, newConflictingWorkflowError(input.status.ID, fmt.Sprintf("Workflow already exists with a different name: %s, but the provided name is: %s", result.name, input.status.Name))
	}
	if len(input.status.QueueName) > 0 && result.queueName != nil && input.status.QueueName != *result.queueName {
		return nil, newConflictingWorkflowError(input.status.ID, fmt.Sprintf("Workflow already exists in a different queue: %s, but the provided queue is: %s", *result.queueName, input.status.QueueName))
	}

	if result.status != WorkflowStatusSuccess && result.status != WorkflowStatusError &&
		input.maxRetries > 0 && result.attempts > input.maxRetries+1 {

		_, dlqErr := s.app.DB().NewQuery(`UPDATE dbos_workflow_status
			SET status = {:status}, deduplication_id = '', queue_name = ''
			WHERE id = {:id} AND status = {:pending}`).Bind(dbx.Params{
			"status":  string(WorkflowStatusMaxRecoveryAttemptsExceeded),
			"id":      input.status.ID,
			"pending": string(WorkflowStatusPending),
		}).Execute()
		if dlqErr != nil {
			return nil, fmt.Errorf("failed to update workflow to %s: %w", WorkflowStatusMaxRecoveryAttemptsExceeded, dlqErr)
		}
		return nil, newDeadLetterQueueError(input.status.ID, input.maxRetries)
	}

	return &result, nil
}

func (s *sqliteSysDB) listWorkflows(ctx context.Context, input listWorkflowsDBInput) ([]WorkflowStatus, error) {
	query := `SELECT id, status, name, executor_id, created_at_epoch_ms, updated_at_epoch_ms,
		application_version, application_id, recovery_attempts, queue_name,
		workflow_timeout_ms, workflow_deadline_epoch_ms,
		deduplication_id, priority, queue_partition_key, forked_from_workflow_uuid, parent_workflow_uuid`

	if input.loadInput {
		query += ", inputs"
	}

	query += " FROM dbos_workflow_status"

	var conditions []string
	params := dbx.Params{}
	paramIdx := 0

	addParam := func(name string, val any) string {
		paramIdx++
		key := fmt.Sprintf("%s_%d", name, paramIdx)
		params[key] = val
		return "{:" + key + "}"
	}

	if len(input.status) > 0 {
		placeholders := make([]string, len(input.status))
		for i, s := range input.status {
			placeholders[i] = addParam("status", string(s))
		}
		conditions = append(conditions, fmt.Sprintf("status IN (%s)", strings.Join(placeholders, ",")))
	}

	if len(input.workflowName) > 0 {
		placeholders := make([]string, len(input.workflowName))
		for i, n := range input.workflowName {
			placeholders[i] = addParam("name", n)
		}
		conditions = append(conditions, fmt.Sprintf("name IN (%s)", strings.Join(placeholders, ",")))
	}

	if len(input.executorIDs) > 0 {
		placeholders := make([]string, len(input.executorIDs))
		for i, e := range input.executorIDs {
			placeholders[i] = addParam("executor", e)
		}
		conditions = append(conditions, fmt.Sprintf("executor_id IN (%s)", strings.Join(placeholders, ",")))
	}

	if len(input.applicationVersion) > 0 {
		placeholders := make([]string, len(input.applicationVersion))
		for i, v := range input.applicationVersion {
			placeholders[i] = addParam("appver", v)
		}
		conditions = append(conditions, fmt.Sprintf("application_version IN (%s)", strings.Join(placeholders, ",")))
	}

	if len(input.workflowIDs) > 0 {
		placeholders := make([]string, len(input.workflowIDs))
		for i, id := range input.workflowIDs {
			placeholders[i] = addParam("wfid", id)
		}
		conditions = append(conditions, fmt.Sprintf("id IN (%s)", strings.Join(placeholders, ",")))
	}

	if input.createdBefore != nil {
		p := addParam("before", input.createdBefore.UnixMilli())
		conditions = append(conditions, fmt.Sprintf("created_at_epoch_ms <= %s", p))
	}

	if input.createdAfter != nil {
		p := addParam("after", input.createdAfter.UnixMilli())
		conditions = append(conditions, fmt.Sprintf("created_at_epoch_ms >= %s", p))
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	if input.sortAscending {
		query += " ORDER BY created_at_epoch_ms ASC"
	} else {
		query += " ORDER BY created_at_epoch_ms DESC"
	}

	if input.limit > 0 {
		p := addParam("limit", input.limit)
		query += " LIMIT " + p
	}

	rows, err := s.app.DB().NewQuery(query).Bind(params).Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows: %w", err)
	}
	defer rows.Close()

	var workflows []WorkflowStatus
	for rows.Next() {
		var wf WorkflowStatus
		var createdAtMs, updatedAtMs int64
		var timeoutMs, deadlineMs sql.NullInt64
		var queueName, appVersion, dedupID, partitionKey, forkedFrom, parentWfID sql.NullString
		var inputStr sql.NullString

		scanArgs := []any{
			&wf.ID, &wf.Status, &wf.Name, &wf.ExecutorID,
			&createdAtMs, &updatedAtMs,
			&appVersion, &wf.ApplicationID, &wf.Attempts,
			&queueName, &timeoutMs, &deadlineMs,
			&dedupID, &wf.Priority, &partitionKey, &forkedFrom, &parentWfID,
		}
		if input.loadInput {
			scanArgs = append(scanArgs, &inputStr)
		}

		if err := rows.Scan(scanArgs...); err != nil {
			return nil, fmt.Errorf("failed to scan workflow row: %w", err)
		}

		wf.CreatedAt = time.Unix(0, createdAtMs*int64(time.Millisecond))
		wf.UpdatedAt = time.Unix(0, updatedAtMs*int64(time.Millisecond))

		if queueName.Valid {
			wf.QueueName = queueName.String
		}
		if appVersion.Valid {
			wf.ApplicationVersion = appVersion.String
		}
		if dedupID.Valid {
			wf.DeduplicationID = dedupID.String
		}
		if partitionKey.Valid {
			wf.QueuePartitionKey = partitionKey.String
		}
		if forkedFrom.Valid {
			wf.ForkedFrom = forkedFrom.String
		}
		if parentWfID.Valid {
			wf.ParentWorkflowID = parentWfID.String
		}
		if timeoutMs.Valid && timeoutMs.Int64 > 0 {
			wf.Timeout = time.Duration(timeoutMs.Int64) * time.Millisecond
		}
		if deadlineMs.Valid {
			wf.Deadline = time.Unix(0, deadlineMs.Int64*int64(time.Millisecond))
		}
		if input.loadInput && inputStr.Valid {
			wf.Input = &inputStr.String
		}

		workflows = append(workflows, wf)
	}

	return workflows, rows.Err()
}

func (s *sqliteSysDB) updateWorkflowOutcome(ctx context.Context, input updateWorkflowOutcomeDBInput) error {
	_, err := s.app.DB().NewQuery(`UPDATE dbos_workflow_status
		SET status = {:status}, output = {:output}, error = {:error},
		    updated_at_epoch_ms = {:updated_at}, deduplication_id = ''
		WHERE id = {:id}
		  AND NOT (status = {:cancelled} AND {:status} IN ({:success}, {:err_status}))`).Bind(dbx.Params{
		"status":     string(input.status),
		"output":     derefStr(input.output),
		"error":      derefStr(input.errorMsg),
		"updated_at": time.Now().UnixMilli(),
		"id":         input.workflowID,
		"cancelled":  string(WorkflowStatusCancelled),
		"success":    string(WorkflowStatusSuccess),
		"err_status": string(WorkflowStatusError),
	}).Execute()

	if err != nil {
		return fmt.Errorf("failed to update workflow status: %w", err)
	}
	return nil
}

func (s *sqliteSysDB) awaitWorkflowResult(ctx context.Context, workflowID string, pollInterval time.Duration) (*string, error) {
	if pollInterval <= 0 {
		pollInterval = _DB_RETRY_INTERVAL
	}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var status WorkflowStatusType
		var outputString, errorStr sql.NullString
		var attempts int

		err := s.app.DB().NewQuery(`SELECT status, output, error, recovery_attempts
			FROM dbos_workflow_status WHERE id = {:id}`).Bind(dbx.Params{
			"id": workflowID,
		}).Row(&status, &outputString, &errorStr, &attempts)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				time.Sleep(pollInterval)
				continue
			}
			return nil, fmt.Errorf("failed to query workflow status: %w", err)
		}

		var output *string
		if outputString.Valid {
			output = &outputString.String
		}

		switch status {
		case WorkflowStatusSuccess, WorkflowStatusError:
			if !errorStr.Valid || errorStr.String == "" {
				return output, nil
			}
			return output, errors.New(errorStr.String)
		case WorkflowStatusCancelled:
			return output, newAwaitedWorkflowCancelledError(workflowID)
		case WorkflowStatusMaxRecoveryAttemptsExceeded:
			return output, newDeadLetterQueueError(workflowID, attempts-2)
		default:
			time.Sleep(pollInterval)
		}
	}
}

func (s *sqliteSysDB) cancelWorkflow(ctx context.Context, input cancelWorkflowDBInput) error {
	wfs, err := s.listWorkflows(ctx, listWorkflowsDBInput{
		workflowIDs: []string{input.workflowID},
	})
	if err != nil {
		return err
	}
	if len(wfs) == 0 {
		return newNonExistentWorkflowError(input.workflowID)
	}

	switch wfs[0].Status {
	case WorkflowStatusSuccess, WorkflowStatusError, WorkflowStatusCancelled:
		return nil
	}

	_, err = s.app.DB().NewQuery(`UPDATE dbos_workflow_status
		SET status = {:status}, updated_at_epoch_ms = {:updated_at}
		WHERE id = {:id}`).Bind(dbx.Params{
		"status":     string(WorkflowStatusCancelled),
		"updated_at": time.Now().UnixMilli(),
		"id":         input.workflowID,
	}).Execute()

	if err != nil {
		return fmt.Errorf("failed to update workflow status to CANCELLED: %w", err)
	}
	return nil
}

func (s *sqliteSysDB) resumeWorkflow(ctx context.Context, input resumeWorkflowDBInput) error {
	wfs, err := s.listWorkflows(ctx, listWorkflowsDBInput{
		workflowIDs: []string{input.workflowID},
	})
	if err != nil {
		return err
	}
	if len(wfs) == 0 {
		return newNonExistentWorkflowError(input.workflowID)
	}

	if wfs[0].Status == WorkflowStatusSuccess || wfs[0].Status == WorkflowStatusError {
		return nil
	}

	_, err = s.app.DB().NewQuery(`UPDATE dbos_workflow_status
		SET status = {:status}, queue_name = {:queue}, recovery_attempts = 0,
		    workflow_deadline_epoch_ms = 0, deduplication_id = '',
		    updated_at_epoch_ms = {:updated_at}
		WHERE id = {:id}`).Bind(dbx.Params{
		"status":     string(WorkflowStatusEnqueued),
		"queue":      _DBOS_INTERNAL_QUEUE_NAME,
		"updated_at": time.Now().UnixMilli(),
		"id":         input.workflowID,
	}).Execute()

	if err != nil {
		return fmt.Errorf("failed to update workflow status to ENQUEUED: %w", err)
	}
	return nil
}

func (s *sqliteSysDB) forkWorkflow(ctx context.Context, input forkWorkflowDBInput) (string, error) {
	newID := input.newWorkflowID
	if newID == "" {
		newID = uuid.New().String()
	}
	if input.startStepID < 0 {
		return "", fmt.Errorf("startStepID must be >= 0, got %d", input.startStepID)
	}

	wfs, err := s.listWorkflows(ctx, listWorkflowsDBInput{
		workflowIDs: []string{input.originalWorkflowID},
		loadInput:   true,
	})
	if err != nil {
		return "", err
	}
	if len(wfs) == 0 {
		return "", newNonExistentWorkflowError(input.originalWorkflowID)
	}
	orig := wfs[0]

	now := time.Now().UnixMilli()
	_, err = s.app.DB().NewQuery(`INSERT INTO dbos_workflow_status (
		id, status, name, application_version, application_id,
		queue_name, inputs, created_at_epoch_ms, updated_at_epoch_ms,
		recovery_attempts, forked_from_workflow_uuid
	) VALUES (
		{:id}, {:status}, {:name}, {:app_ver}, {:app_id},
		{:queue}, {:inputs}, {:created_at}, {:updated_at},
		0, {:forked_from}
	)`).Bind(dbx.Params{
		"id":          newID,
		"status":      string(WorkflowStatusEnqueued),
		"name":        orig.Name,
		"app_ver":     orig.ApplicationVersion,
		"app_id":      orig.ApplicationID,
		"queue":       _DBOS_INTERNAL_QUEUE_NAME,
		"inputs":      input.input,
		"created_at":  now,
		"updated_at":  now,
		"forked_from": input.originalWorkflowID,
	}).Execute()
	if err != nil {
		return "", fmt.Errorf("failed to insert forked workflow: %w", err)
	}

	if input.startStepID > 0 {
		_, err = s.app.DB().NewQuery(`INSERT INTO dbos_operation_outputs
			(id, workflow_uuid, function_id, output, error, function_name, child_workflow_id, started_at_epoch_ms, ended_at_epoch_ms)
			SELECT {:new_uuid} || '_' || function_id, workflow_uuid, function_id, output, error, function_name, child_workflow_id, started_at_epoch_ms, ended_at_epoch_ms
			FROM dbos_operation_outputs
			WHERE workflow_uuid = {:orig_id} AND function_id < {:start_step}`).Bind(dbx.Params{
			"new_uuid":   newID,
			"orig_id":    input.originalWorkflowID,
			"start_step": input.startStepID,
		}).Execute()
		if err != nil {
			return "", fmt.Errorf("failed to copy operation outputs: %w", err)
		}

		// Copy workflow events history, replacing workflow_uuid with new ID
		_, err = s.app.DB().NewQuery(`INSERT INTO dbos_workflow_events_history
			(id, workflow_uuid, function_id, key, value)
			SELECT {:new_uuid} || '_' || function_id || '_' || key, {:new_id}, function_id, key, value
			FROM dbos_workflow_events_history
			WHERE workflow_uuid = {:orig_id} AND function_id < {:start_step}`).Bind(dbx.Params{
			"new_uuid":   newID,
			"new_id":     newID,
			"orig_id":    input.originalWorkflowID,
			"start_step": input.startStepID,
		}).Execute()
		if err != nil {
			return "", fmt.Errorf("failed to copy workflow events history: %w", err)
		}

		// Copy latest events (SQLite equivalent of DISTINCT ON)
		_, err = s.app.DB().NewQuery(`INSERT INTO dbos_workflow_events
			(id, workflow_uuid, key, value)
			SELECT {:new_uuid} || '_' || sub.key, {:new_id}, sub.key, sub.value
			FROM (
				SELECT key, value
				FROM dbos_workflow_events_history
				WHERE workflow_uuid = {:orig_id} AND function_id < {:start_step}
				GROUP BY key
				HAVING function_id = MAX(function_id)
			) AS sub`).Bind(dbx.Params{
			"new_uuid":   newID,
			"new_id":     newID,
			"orig_id":    input.originalWorkflowID,
			"start_step": input.startStepID,
		}).Execute()
		if err != nil {
			return "", fmt.Errorf("failed to copy latest workflow events: %w", err)
		}
	}

	return newID, nil
}

/*******************************/
/******* STEPS ********/
/*******************************/

func (s *sqliteSysDB) recordOperationResult(ctx context.Context, input recordOperationResultDBInput) error {
	_, err := s.app.DB().NewQuery(`INSERT INTO dbos_operation_outputs
		(id, workflow_uuid, function_id, output, error, function_name, started_at_epoch_ms, ended_at_epoch_ms)
		VALUES ({:id}, {:wf_id}, {:func_id}, {:output}, {:error}, {:func_name}, {:started_at}, {:ended_at})`).Bind(dbx.Params{
		"id":         fmt.Sprintf("%s_%d", input.workflowUUID, input.functionID),
		"wf_id":      input.workflowUUID,
		"func_id":    input.functionID,
		"output":     derefStr(input.output),
		"error":      derefStr(input.errorMsg),
		"func_name":  input.functionName,
		"started_at": input.startedAt,
		"ended_at":   input.endedAt,
	}).Execute()

	if err != nil {
		if isSQLiteUniqueViolation(err) {
			return newWorkflowConflictIDError(input.workflowUUID)
		}
		return err
	}
	return nil
}

func (s *sqliteSysDB) checkOperationExecution(ctx context.Context, input checkOperationExecutionDBInput) (*recordedResult, error) {
	// Check workflow status first
	var workflowStatus WorkflowStatusType
	err := s.app.DB().NewQuery(`SELECT status FROM dbos_workflow_status WHERE id = {:id}`).Bind(dbx.Params{
		"id": input.workflowUUID,
	}).Row(&workflowStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, newNonExistentWorkflowError(input.workflowUUID)
		}
		return nil, fmt.Errorf("failed to get workflow status: %w", err)
	}

	if workflowStatus == WorkflowStatusCancelled {
		return nil, newWorkflowCancelledError(input.workflowUUID)
	}

	// Check operation outputs
	var outputString, errorStr, functionName sql.NullString
	err = s.app.DB().NewQuery(`SELECT output, error, function_name
		FROM dbos_operation_outputs
		WHERE workflow_uuid = {:wf_id} AND function_id = {:func_id}`).Bind(dbx.Params{
		"wf_id":   input.workflowUUID,
		"func_id": input.functionID,
	}).Row(&outputString, &errorStr, &functionName)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get operation outputs: %w", err)
	}

	// functionName check is skipped — checkOperationExecutionDBInput doesn't carry it.
	_ = functionName

	result := &recordedResult{}
	if outputString.Valid {
		result.output = &outputString.String
	}
	if errorStr.Valid && errorStr.String != "" {
		result.errorMsg = &errorStr.String
	}
	return result, nil
}

func (s *sqliteSysDB) getWorkflowSteps(ctx context.Context, input getWorkflowStepsInput) ([]stepInfo, error) {
	rows, err := s.app.DB().NewQuery(`SELECT function_id, function_name, output, error, started_at_epoch_ms, ended_at_epoch_ms
		FROM dbos_operation_outputs
		WHERE workflow_uuid = {:wf_id}
		ORDER BY function_id ASC`).Bind(dbx.Params{
		"wf_id": input.workflowID,
	}).Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to query workflow steps: %w", err)
	}
	defer rows.Close()

	var steps []stepInfo
	for rows.Next() {
		var step stepInfo
		var funcName, output, errStr sql.NullString
		var startedAt, endedAt sql.NullInt64

		if err := rows.Scan(&step.functionID, &funcName, &output, &errStr, &startedAt, &endedAt); err != nil {
			return nil, fmt.Errorf("failed to scan step row: %w", err)
		}

		if funcName.Valid {
			step.functionName = funcName.String
		}
		if output.Valid {
			step.output = &output.String
		}
		if errStr.Valid {
			step.errorMsg = &errStr.String
		}
		if startedAt.Valid {
			step.startedAt = &startedAt.Int64
		}
		if endedAt.Valid {
			step.endedAt = &endedAt.Int64
		}

		steps = append(steps, step)
	}

	return steps, rows.Err()
}

/*******************************/
/******* CHILD WORKFLOWS ********/
/*******************************/

func (s *sqliteSysDB) recordChildWorkflow(ctx context.Context, input recordChildWorkflowDBInput) error {
	_, err := s.app.DB().NewQuery(`INSERT INTO dbos_operation_outputs
		(id, workflow_uuid, function_id, function_name, child_workflow_id, output, error)
		VALUES ({:id}, {:wf_id}, {:func_id}, {:func_name}, {:child_id}, '', '')`).Bind(dbx.Params{
		"id":        fmt.Sprintf("%s_%d", input.workflowUUID, input.functionID),
		"wf_id":     input.workflowUUID,
		"func_id":   input.functionID,
		"func_name": "childWorkflow",
		"child_id":  input.childWorkflowID,
	}).Execute()

	if err != nil {
		if isSQLiteUniqueViolation(err) {
			return fmt.Errorf("child workflow %s already registered for parent %s (step %d)", input.childWorkflowID, input.workflowUUID, input.functionID)
		}
		return fmt.Errorf("failed to record child workflow: %w", err)
	}
	return nil
}

func (s *sqliteSysDB) checkChildWorkflow(ctx context.Context, workflowUUID string, functionID int) (*string, error) {
	var childID sql.NullString
	err := s.app.DB().NewQuery(`SELECT child_workflow_id FROM dbos_operation_outputs
		WHERE workflow_uuid = {:wf_id} AND function_id = {:func_id}`).Bind(dbx.Params{
		"wf_id":   workflowUUID,
		"func_id": functionID,
	}).Row(&childID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to check child workflow: %w", err)
	}

	if childID.Valid {
		return &childID.String, nil
	}
	return nil, nil
}

func (s *sqliteSysDB) recordChildGetResult(ctx context.Context, input recordChildGetResultDBInput) error {
	_, err := s.app.DB().NewQuery(`INSERT INTO dbos_operation_outputs
		(id, workflow_uuid, function_id, function_name, output, error, child_workflow_id)
		VALUES ({:id}, {:wf_id}, {:func_id}, 'DBOS.getResult', {:output}, {:error}, {:child_id})
		ON CONFLICT DO NOTHING`).Bind(dbx.Params{
		"id":       fmt.Sprintf("%s_%d", input.workflowUUID, input.functionID),
		"wf_id":    input.workflowUUID,
		"func_id":  input.functionID,
		"output":   derefStr(input.output),
		"error":    derefStr(input.errorMsg),
		"child_id": "",
	}).Execute()

	if err != nil {
		return fmt.Errorf("failed to record get result: %w", err)
	}
	return nil
}

/****************************************/
/******* WORKFLOW COMMUNICATIONS ********/
/****************************************/

func (s *sqliteSysDB) send(ctx context.Context, input WorkflowSendInput) error {
	msgID := uuid.New().String()
	_, err := s.app.DB().NewQuery(`INSERT INTO dbos_notifications
		(id, destination_uuid, topic, message, created_at_epoch_ms, consumed)
		VALUES ({:id}, {:dest}, {:topic}, {:message}, {:created_at}, FALSE)`).Bind(dbx.Params{
		"id":         msgID,
		"dest":       input.DestinationUUID,
		"topic":      input.Topic,
		"message":    derefStr(input.Message),
		"created_at": time.Now().UnixMilli(),
	}).Execute()

	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	// Signal event bus
	payload := fmt.Sprintf("%s::%s", input.DestinationUUID, input.Topic)
	s.eventBus.Notify(payload)

	return nil
}

func (s *sqliteSysDB) recv(ctx context.Context, input recvInput) (*string, error) {
	// Try to consume a message directly
	var message sql.NullString
	err := s.app.DB().NewQuery(`
		WITH oldest AS (
			SELECT id, message
			FROM dbos_notifications
			WHERE destination_uuid = {:dest} AND topic = {:topic} AND consumed = FALSE
			ORDER BY created_at_epoch_ms ASC
			LIMIT 1
		)
		UPDATE dbos_notifications SET consumed = TRUE
		WHERE id = (SELECT id FROM oldest)
		RETURNING message`).Bind(dbx.Params{
		"dest":  input.workflowUUID,
		"topic": input.topic,
	}).Row(&message)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to consume notification: %w", err)
	}

	if message.Valid {
		return &message.String, nil
	}

	// No message found — wait with event bus
	if input.timeout <= 0 {
		return nil, nil
	}

	payload := fmt.Sprintf("%s::%s", input.workflowUUID, input.topic)
	ch := s.eventBus.Wait(payload)
	defer s.eventBus.Remove(payload, ch)

	timer := time.NewTimer(input.timeout)
	defer timer.Stop()

	for {
		select {
		case <-ch:
			// Re-register for next signal
			s.eventBus.Remove(payload, ch)
			ch = s.eventBus.Wait(payload)

			// Try again
			err = s.app.DB().NewQuery(`
				WITH oldest AS (
					SELECT id, message
					FROM dbos_notifications
					WHERE destination_uuid = {:dest} AND topic = {:topic} AND consumed = FALSE
					ORDER BY created_at_epoch_ms ASC
					LIMIT 1
				)
				UPDATE dbos_notifications SET consumed = TRUE
				WHERE id = (SELECT id FROM oldest)
				RETURNING message`).Bind(dbx.Params{
				"dest":  input.workflowUUID,
				"topic": input.topic,
			}).Row(&message)

			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("failed to consume notification: %w", err)
			}
			if message.Valid {
				return &message.String, nil
			}

		case <-timer.C:
			return nil, nil

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (s *sqliteSysDB) setEvent(ctx context.Context, input WorkflowSetEventInput) error {
	_, err := s.app.DB().NewQuery(`INSERT INTO dbos_workflow_events (id, workflow_uuid, key, value)
		VALUES ({:id}, {:wf_id}, {:key}, {:value})
		ON CONFLICT (workflow_uuid, key)
		DO UPDATE SET value = excluded.value`).Bind(dbx.Params{
		"id":    fmt.Sprintf("%s_%s", input.WorkflowUUID, input.Key),
		"wf_id": input.WorkflowUUID,
		"key":   input.Key,
		"value": derefStr(input.Value),
	}).Execute()

	if err != nil {
		return fmt.Errorf("failed to set event: %w", err)
	}

	// Signal event bus
	payload := fmt.Sprintf("%s::%s", input.WorkflowUUID, input.Key)
	s.eventBus.Notify(payload)

	return nil
}

func (s *sqliteSysDB) getEvent(ctx context.Context, input getEventInput) (*string, error) {
	var value sql.NullString
	err := s.app.DB().NewQuery(`SELECT value FROM dbos_workflow_events
		WHERE workflow_uuid = {:wf_id} AND key = {:key}`).Bind(dbx.Params{
		"wf_id": input.targetWorkflowUUID,
		"key":   input.key,
	}).Row(&value)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	if value.Valid {
		return &value.String, nil
	}

	// Event not found — wait with event bus
	if input.timeout <= 0 {
		return nil, nil
	}

	payload := fmt.Sprintf("%s::%s", input.targetWorkflowUUID, input.key)
	ch := s.eventBus.Wait(payload)
	defer s.eventBus.Remove(payload, ch)

	timer := time.NewTimer(input.timeout)
	defer timer.Stop()

	for {
		select {
		case <-ch:
			s.eventBus.Remove(payload, ch)
			ch = s.eventBus.Wait(payload)

			err = s.app.DB().NewQuery(`SELECT value FROM dbos_workflow_events
				WHERE workflow_uuid = {:wf_id} AND key = {:key}`).Bind(dbx.Params{
				"wf_id": input.targetWorkflowUUID,
				"key":   input.key,
			}).Row(&value)

			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("failed to get event: %w", err)
			}
			if value.Valid {
				return &value.String, nil
			}

		case <-timer.C:
			return nil, nil

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

/*******************************/
/******* SLEEP ********/
/*******************************/

func (s *sqliteSysDB) sleep(ctx context.Context, input sleepInput) (time.Duration, error) {
	remainingDuration := input.duration
	time.Sleep(remainingDuration)
	return remainingDuration, nil
}

/*******************************/
/******* QUEUES ********/
/*******************************/

func (s *sqliteSysDB) dequeueWorkflows(ctx context.Context, input dequeueWorkflowsInput) ([]dequeuedWorkflow, error) {
	query := `SELECT id, queue_name, name, inputs FROM dbos_workflow_status
		WHERE queue_name = {:queue} AND status = {:status}`

	params := dbx.Params{
		"queue":  input.queueName,
		"status": string(WorkflowStatusEnqueued),
	}

	if input.priorityEnabled {
		query += " ORDER BY priority ASC, created_at_epoch_ms ASC"
	} else {
		query += " ORDER BY created_at_epoch_ms ASC"
	}

	if input.limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", input.limit)
	}

	rows, err := s.app.DB().NewQuery(query).Bind(params).Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to dequeue workflows: %w", err)
	}
	defer rows.Close()

	var workflows []dequeuedWorkflow
	for rows.Next() {
		var wf dequeuedWorkflow
		var inputStr sql.NullString
		if err := rows.Scan(&wf.workflowID, &wf.queueName, &wf.name, &inputStr); err != nil {
			return nil, fmt.Errorf("failed to scan dequeued workflow: %w", err)
		}
		if inputStr.Valid {
			wf.input = &inputStr.String
		}
		workflows = append(workflows, wf)
	}

	// Update dequeued workflows to PENDING
	for _, wf := range workflows {
		_, err := s.app.DB().NewQuery(`UPDATE dbos_workflow_status
			SET status = {:status}, executor_id = {:executor}, application_version = {:app_ver},
			    updated_at_epoch_ms = {:updated_at}
			WHERE id = {:id}`).Bind(dbx.Params{
			"status":     string(WorkflowStatusPending),
			"executor":   input.executorID,
			"app_ver":    input.appVersion,
			"updated_at": time.Now().UnixMilli(),
			"id":         wf.workflowID,
		}).Execute()
		if err != nil {
			return nil, fmt.Errorf("failed to update dequeued workflow: %w", err)
		}
	}

	return workflows, nil
}

func (s *sqliteSysDB) clearQueueAssignment(ctx context.Context, workflowID string) (bool, error) {
	result, err := s.app.DB().NewQuery(`UPDATE dbos_workflow_status
		SET status = {:status}, queue_name = {:queue}, updated_at_epoch_ms = {:updated_at}
		WHERE id = {:id} AND status = {:pending}`).Bind(dbx.Params{
		"status":     string(WorkflowStatusEnqueued),
		"queue":      _DBOS_INTERNAL_QUEUE_NAME,
		"updated_at": time.Now().UnixMilli(),
		"id":         workflowID,
		"pending":    string(WorkflowStatusPending),
	}).Execute()
	if err != nil {
		return false, fmt.Errorf("failed to clear queue assignment: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	return rowsAffected > 0, nil
}

func (s *sqliteSysDB) getQueuePartitions(ctx context.Context, queueName string) ([]string, error) {
	rows, err := s.app.DB().NewQuery(`SELECT DISTINCT queue_partition_key FROM dbos_workflow_status
		WHERE queue_name = {:queue} AND queue_partition_key IS NOT NULL AND queue_partition_key != ''`).Bind(dbx.Params{
		"queue": queueName,
	}).Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to get queue partitions: %w", err)
	}
	defer rows.Close()

	var partitions []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		partitions = append(partitions, key)
	}
	return partitions, rows.Err()
}

/*******************************/
/******* GARBAGE COLLECTION ********/
/*******************************/

func (s *sqliteSysDB) garbageCollectWorkflows(ctx context.Context, input garbageCollectWorkflowsInput) error {
	cutoffMs := input.cutoffTime.UnixMilli()

	result, err := s.app.DB().NewQuery(`DELETE FROM dbos_workflow_status
		WHERE created_at_epoch_ms < {:cutoff}
		  AND status NOT IN ({:pending}, {:enqueued})`).Bind(dbx.Params{
		"cutoff":   cutoffMs,
		"pending":  string(WorkflowStatusPending),
		"enqueued": string(WorkflowStatusEnqueued),
	}).Execute()
	if err != nil {
		return fmt.Errorf("failed to garbage collect workflows: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	s.logger.Info("Garbage collected workflows", "cutoff", cutoffMs, "deleted", rowsAffected)
	return nil
}
