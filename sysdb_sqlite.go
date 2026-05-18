package turbine

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

const (
	ptInternalQueueName = "_pt_internal_queue"
	dbRetryInterval     = 1 * time.Second
)

type sqliteSysDB struct {
	app      core.App
	eventBus *eventBus
	logger   *slog.Logger
	launched bool
}

func newSQLiteSysDB(app core.App, eb *eventBus) *sqliteSysDB {
	return &sqliteSysDB{
		app:      app,
		eventBus: eb,
		logger:   slog.Default().With("service", "sysdb_sqlite"),
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

// workflowExists returns nil if the workflow exists, or an ErrWorkflowNotFound if not.
func (s *sqliteSysDB) workflowExists(workflowID string) error {
	var exists int
	err := s.app.DB().Select("1").
		From("pt_workflow_status").
		Where(dbx.HashExp{"id": workflowID}).
		Limit(1).
		Row(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return newErrWorkflowNotFound(workflowID)
		}
		return fmt.Errorf("failed to check workflow existence: %w", err)
	}
	return nil
}

// isSQLiteUniqueViolation checks if an error is a SQLite UNIQUE constraint violation.
func isSQLiteUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func (s *sqliteSysDB) insertStatus(ctx context.Context, input insertStatusDBInput) (*insertWorkflowResult, error) {
	attempts := 1
	if input.status.Status == StatusEnqueued {
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

	query := `INSERT INTO pt_workflow_status (
		id, status, name, queue_name,
		executor_id, application_version, application_id,
		created_at_epoch_ms, recovery_attempts, updated_at_epoch_ms,
		workflow_timeout_ms, workflow_deadline_epoch_ms,
		inputs, deduplication_id, priority, queue_partition_key,
		owner_xid, parent_workflow_id, tags, summary
	) VALUES(
		{:id}, {:status}, {:name}, {:queue_name},
		{:executor_id}, {:app_version}, {:app_id},
		{:created_at}, {:attempts}, {:updated_at},
		{:timeout_ms}, {:deadline_ms},
		{:inputs}, {:dedup_id}, {:priority}, {:partition_key},
		{:owner_xid}, {:parent_wf_id}, {:tags}, {:summary}
	)
	ON CONFLICT (id)
	DO UPDATE SET
		recovery_attempts = CASE
			WHEN excluded.status != {:enqueued_status1} THEN pt_workflow_status.recovery_attempts + {:recovery_inc}
			ELSE pt_workflow_status.recovery_attempts
		END,
		updated_at_epoch_ms = excluded.updated_at_epoch_ms,
		executor_id = CASE
			WHEN excluded.status = {:enqueued_status2} THEN pt_workflow_status.executor_id
			ELSE excluded.executor_id
		END
	RETURNING recovery_attempts, status, name, queue_name, workflow_timeout_ms, workflow_deadline_epoch_ms, owner_xid, COALESCE(app_status, ''), COALESCE(app_status_color, ''),
		(SELECT COUNT(*) > 0 FROM pt_operation_outputs WHERE workflow_id = pt_workflow_status.id)`

	var result insertWorkflowResult
	var timeoutMSResult int64
	var workflowDeadlineEpochMS int64
	var queueNameReturn, ownerXIDReturn string

	ownerXID := ""
	if input.ownerXID != nil {
		ownerXID = *input.ownerXID
	}

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
		"id":            input.status.ID,
		"status":        input.status.Status,
		"name":          input.status.Name,
		"queue_name":    input.status.QueueName,
		"executor_id":   input.status.ExecutorID,
		"app_version":   applicationVersion,
		"app_id":        input.status.ApplicationID,
		"created_at":    input.status.CreatedAt.Round(time.Millisecond).UnixMilli(),
		"attempts":      attempts,
		"updated_at":    updatedAt.UnixMilli(),
		"timeout_ms":    timeoutMs,
		"deadline_ms":   deadline,
		"inputs":        inputs,
		"dedup_id":      deduplicationID,
		"priority":      input.status.Priority,
		"partition_key": queuePartitionKey,
		"owner_xid":     ownerXID,
		"parent_wf_id":  parentWorkflowID,
		"tags": func() string {
			if len(input.status.Tags) == 0 {
				return "[]"
			}
			b, _ := json.Marshal(input.status.Tags)
			return string(b)
		}(),
		"summary":          input.status.Summary,
		"enqueued_status1": string(StatusEnqueued),
		"enqueued_status2": string(StatusEnqueued),
		"recovery_inc":     recoveryIncrement,
	}).Row(&result.attempts, &result.status, &result.name, &queueNameReturn, &timeoutMSResult, &workflowDeadlineEpochMS, &ownerXIDReturn, &result.appStatus, &result.appStatusColor, &result.hasSteps)

	result.ownerXID = ownerXIDReturn
	if queueNameReturn != "" {
		result.queueName = &queueNameReturn
	}
	if result.status == StatusEnqueued && queueNameReturn != "" {
		s.eventBus.Notify("queue::" + queueNameReturn)
	}
	if err != nil {
		if isSQLiteUniqueViolation(err) {
			return nil, newErrDeduplicated(input.status.ID, input.status.QueueName, input.status.DeduplicationID)
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
		return nil, newErrWorkflowConflict(input.status.ID, fmt.Sprintf("Workflow already exists with a different name: %s, but the provided name is: %s", result.name, input.status.Name))
	}
	if len(input.status.QueueName) > 0 && result.queueName != nil && input.status.QueueName != *result.queueName {
		return nil, newErrWorkflowConflict(input.status.ID, fmt.Sprintf("Workflow already exists in a different queue: %s, but the provided queue is: %s", *result.queueName, input.status.QueueName))
	}

	if result.status != StatusSuccess && result.status != StatusError &&
		input.maxRetries > 0 && result.attempts > input.maxRetries+1 {

		dlqErr := s.app.RunInTransaction(func(txApp core.App) error {
			record, err := txApp.FindRecordById(collectionStatus, input.status.ID)
			if err != nil {
				return err
			}
			if record.GetString("status") != string(StatusPending) {
				return nil
			}
			record.Set("status", string(StatusMaxRecoveryAttemptsExceeded))
			record.Set("deduplication_id", "")
			record.Set("queue_name", "")
			return txApp.SaveNoValidate(record)
		})
		if dlqErr != nil {
			return nil, fmt.Errorf("failed to update workflow to %s: %w", StatusMaxRecoveryAttemptsExceeded, dlqErr)
		}
		s.eventBus.Notify("workflow::" + input.status.ID)
		return nil, newErrDeadLetter(input.status.ID, input.maxRetries)
	}

	return &result, nil
}

func (s *sqliteSysDB) listWorkflows(ctx context.Context, input listWorkflowsDBInput) ([]Status, error) {
	cols := []string{
		"id", "status", "name", "executor_id", "created_at_epoch_ms", "updated_at_epoch_ms",
		"application_version", "application_id", "recovery_attempts", "queue_name",
		"workflow_timeout_ms", "workflow_deadline_epoch_ms",
		"deduplication_id", "priority", "queue_partition_key", "forked_from_workflow_id", "parent_workflow_id",
		"app_status", "app_status_color", "tags", "summary",
	}
	if input.loadInput {
		cols = append(cols, "inputs")
	}

	q := s.app.DB().Select(cols...).From("pt_workflow_status")

	if len(input.status) > 0 {
		vals := make([]any, len(input.status))
		for i, s := range input.status {
			vals[i] = string(s)
		}
		q.AndWhere(dbx.In("status", vals...))
	}
	if len(input.workflowName) > 0 {
		vals := make([]any, len(input.workflowName))
		for i, n := range input.workflowName {
			vals[i] = n
		}
		q.AndWhere(dbx.In("name", vals...))
	}
	if len(input.executorIDs) > 0 {
		vals := make([]any, len(input.executorIDs))
		for i, e := range input.executorIDs {
			vals[i] = e
		}
		q.AndWhere(dbx.In("executor_id", vals...))
	}
	if len(input.applicationVersion) > 0 {
		vals := make([]any, len(input.applicationVersion))
		for i, v := range input.applicationVersion {
			vals[i] = v
		}
		q.AndWhere(dbx.In("application_version", vals...))
	}
	if len(input.workflowIDs) > 0 {
		vals := make([]any, len(input.workflowIDs))
		for i, id := range input.workflowIDs {
			vals[i] = id
		}
		q.AndWhere(dbx.In("id", vals...))
	}
	if input.createdBefore != nil {
		q.AndWhere(dbx.NewExp("created_at_epoch_ms <= {:before}", dbx.Params{"before": input.createdBefore.UnixMilli()}))
	}
	if input.createdAfter != nil {
		q.AndWhere(dbx.NewExp("created_at_epoch_ms >= {:after}", dbx.Params{"after": input.createdAfter.UnixMilli()}))
	}

	if input.sortAscending {
		q.OrderBy("created_at_epoch_ms ASC")
	} else {
		q.OrderBy("created_at_epoch_ms DESC")
	}

	if input.limit > 0 {
		q.Limit(int64(input.limit))
	}

	rows, err := q.Build().Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var workflows []Status
	for rows.Next() {
		var wf Status
		var createdAtMs, updatedAtMs int64
		var timeoutMs, deadlineMs sql.NullInt64
		var queueName, appVersion, dedupID, partitionKey, forkedFrom, parentWfID sql.NullString
		var appStatus, appStatusColor sql.NullString
		var inputStr sql.NullString
		var tagsStr sql.NullString
		var summary sql.NullString

		scanArgs := []any{
			&wf.ID, &wf.Status, &wf.Name, &wf.ExecutorID,
			&createdAtMs, &updatedAtMs,
			&appVersion, &wf.ApplicationID, &wf.Attempts,
			&queueName, &timeoutMs, &deadlineMs,
			&dedupID, &wf.Priority, &partitionKey, &forkedFrom, &parentWfID,
			&appStatus, &appStatusColor, &tagsStr, &summary,
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
		if appStatus.Valid {
			wf.AppStatus = appStatus.String
		}
		if appStatusColor.Valid {
			wf.AppStatusColor = appStatusColor.String
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
		if tagsStr.Valid && tagsStr.String != "" {
			if unmarshalErr := json.Unmarshal([]byte(tagsStr.String), &wf.Tags); unmarshalErr != nil {
				s.app.Logger().Warn("failed to parse workflow tags", "workflow_id", wf.ID, "error", unmarshalErr)
			}
		}
		if summary.Valid {
			wf.Summary = summary.String
		}

		workflows = append(workflows, wf)
	}

	return workflows, rows.Err()
}

func (s *sqliteSysDB) updateWorkflowOutcome(ctx context.Context, input updateWorkflowOutcomeDBInput) error {
	var queueName string
	err := s.app.DB().NewQuery(`UPDATE pt_workflow_status
		SET status = {:status}, output = {:output}, error = {:error},
		    updated_at_epoch_ms = {:updated_at}, deduplication_id = ''
		WHERE id = {:id}
		  AND NOT (status = {:cancelled} AND {:status} IN ({:success}, {:err_status}))
		RETURNING COALESCE(queue_name, '')`).Bind(dbx.Params{
		"status":     string(input.status),
		"output":     derefStr(input.output),
		"error":      derefStr(input.errorMsg),
		"updated_at": time.Now().UnixMilli(),
		"id":         input.workflowID,
		"cancelled":  string(StatusCancelled),
		"success":    string(StatusSuccess),
		"err_status": string(StatusError),
	}).Row(&queueName)

	if err != nil {
		// No row matched (e.g. already cancelled). Not an error, just nothing to notify.
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("failed to update workflow status: %w", err)
	}
	s.eventBus.Notify("workflow::" + input.workflowID)
	// Wake the queue runner so it re-evaluates concurrency/rate limits and can
	// dequeue more work that was held back while this slot was occupied.
	if queueName != "" {
		s.eventBus.Notify("queue::" + queueName)
	}
	return nil
}

func (s *sqliteSysDB) awaitWorkflowResult(ctx context.Context, workflowID string, _ time.Duration) (*string, error) {
	key := "workflow::" + workflowID
	ch := s.eventBus.Wait(key)
	defer s.eventBus.Remove(key, ch)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var status StatusType
		var outputString, errorStr sql.NullString
		var attempts int

		err := s.app.DB().Select("status", "output", "error", "recovery_attempts").
			From("pt_workflow_status").
			Where(dbx.HashExp{"id": workflowID}).
			Row(&status, &outputString, &errorStr, &attempts)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				select {
				case <-ch:
					ch = s.eventBus.Swap(key, ch)
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				continue
			}
			return nil, fmt.Errorf("failed to query workflow status: %w", err)
		}

		var output *string
		if outputString.Valid {
			output = &outputString.String
		}

		switch status {
		case StatusSuccess, StatusError:
			if !errorStr.Valid || errorStr.String == "" {
				return output, nil
			}
			return output, errors.New(errorStr.String)
		case StatusCancelled:
			return output, newErrAwaitCancelled(workflowID)
		case StatusMaxRecoveryAttemptsExceeded:
			return output, newErrDeadLetter(workflowID, attempts-2)
		default:
			select {
			case <-ch:
				ch = s.eventBus.Swap(key, ch)
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
}

func (s *sqliteSysDB) cancelWorkflow(ctx context.Context, input cancelWorkflowDBInput) (bool, error) {
	// Read queue_name before clearing it so we can wake the queue runner if the
	// cancellation frees a concurrency/rate slot.
	var priorQueueName string
	_ = s.app.DB().NewQuery(`SELECT COALESCE(queue_name, '') FROM pt_workflow_status WHERE id = {:id}`).
		Bind(dbx.Params{"id": input.workflowID}).Row(&priorQueueName)

	result, err := s.app.DB().NewQuery(`UPDATE pt_workflow_status
		SET status = {:status}, updated_at_epoch_ms = {:updated_at},
		    deduplication_id = '', queue_name = ''
		WHERE id = {:id}
		  AND status NOT IN ({:success}, {:error}, {:cancelled})`).Bind(dbx.Params{
		"status":     string(StatusCancelled),
		"updated_at": time.Now().UnixMilli(),
		"id":         input.workflowID,
		"success":    string(StatusSuccess),
		"error":      string(StatusError),
		"cancelled":  string(StatusCancelled),
	}).Execute()

	if err != nil {
		return false, fmt.Errorf("failed to update workflow status to CANCELLED: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		// Either doesn't exist or already in a terminal state. Both are fine.
		if err := s.workflowExists(input.workflowID); err != nil {
			return false, err
		}
		return false, nil
	}
	s.eventBus.Notify("workflow::" + input.workflowID)
	if priorQueueName != "" {
		s.eventBus.Notify("queue::" + priorQueueName)
	}
	return true, nil
}

func (s *sqliteSysDB) resumeWorkflow(ctx context.Context, input resumeWorkflowDBInput) error {
	result, err := s.app.DB().NewQuery(`UPDATE pt_workflow_status
		SET status = {:status}, queue_name = {:queue}, recovery_attempts = 0,
		    workflow_deadline_epoch_ms = 0, deduplication_id = '',
		    updated_at_epoch_ms = {:updated_at}
		WHERE id = {:id}
		  AND status NOT IN ({:success}, {:error})`).Bind(dbx.Params{
		"status":     string(StatusEnqueued),
		"queue":      ptInternalQueueName,
		"updated_at": time.Now().UnixMilli(),
		"id":         input.workflowID,
		"success":    string(StatusSuccess),
		"error":      string(StatusError),
	}).Execute()

	if err != nil {
		return fmt.Errorf("failed to update workflow status to ENQUEUED: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		s.eventBus.Notify("queue::" + ptInternalQueueName)
	}
	if rowsAffected == 0 {
		if err := s.workflowExists(input.workflowID); err != nil {
			return err
		}
	}
	return nil
}

func (s *sqliteSysDB) forkWorkflow(ctx context.Context, input forkWorkflowDBInput) (string, error) {
	newID := input.newWorkflowID
	if newID == "" {
		newID = core.GenerateDefaultRandomId()
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
		return "", newErrWorkflowNotFound(input.originalWorkflowID)
	}
	orig := wfs[0]

	// Use a transaction to ensure all fork operations are atomic
	now := time.Now().UnixMilli()
	err = s.app.RunInTransaction(func(txApp core.App) error {
		_, txErr := txApp.DB().NewQuery(`INSERT INTO pt_workflow_status (
			id, status, name, application_version, application_id,
			queue_name, inputs, created_at_epoch_ms, updated_at_epoch_ms,
			recovery_attempts, forked_from_workflow_id
		) VALUES (
			{:id}, {:status}, {:name}, {:app_ver}, {:app_id},
			{:queue}, {:inputs}, {:created_at}, {:updated_at},
			0, {:forked_from}
		)`).Bind(dbx.Params{
			"id":          newID,
			"status":      string(StatusEnqueued),
			"name":        orig.Name,
			"app_ver":     orig.ApplicationVersion,
			"app_id":      orig.ApplicationID,
			"queue":       ptInternalQueueName,
			"inputs":      input.input,
			"created_at":  now,
			"updated_at":  now,
			"forked_from": input.originalWorkflowID,
		}).Execute()
		if txErr != nil {
			return fmt.Errorf("failed to insert forked workflow: %w", txErr)
		}

		if input.startStepID > 0 {
			_, txErr = txApp.DB().NewQuery(`INSERT INTO pt_operation_outputs
				(id, workflow_id, function_id, output, error, function_name, child_workflow_id, started_at_epoch_ms, ended_at_epoch_ms)
				SELECT {:new_uuid} || '_' || function_id, workflow_id, function_id, output, error, function_name, child_workflow_id, started_at_epoch_ms, ended_at_epoch_ms
				FROM pt_operation_outputs
				WHERE workflow_id = {:orig_id} AND function_id < {:start_step}`).Bind(dbx.Params{
				"new_uuid":   newID,
				"orig_id":    input.originalWorkflowID,
				"start_step": input.startStepID,
			}).Execute()
			if txErr != nil {
				return fmt.Errorf("failed to copy operation outputs: %w", txErr)
			}

			_, txErr = txApp.DB().NewQuery(`INSERT INTO pt_workflow_events_history
				(id, workflow_id, function_id, key, value)
				SELECT {:new_uuid} || '_' || function_id || '_' || key, {:new_id}, function_id, key, value
				FROM pt_workflow_events_history
				WHERE workflow_id = {:orig_id} AND function_id < {:start_step}`).Bind(dbx.Params{
				"new_uuid":   newID,
				"new_id":     newID,
				"orig_id":    input.originalWorkflowID,
				"start_step": input.startStepID,
			}).Execute()
			if txErr != nil {
				return fmt.Errorf("failed to copy workflow events history: %w", txErr)
			}

			_, txErr = txApp.DB().NewQuery(`INSERT INTO pt_workflow_events
				(id, workflow_id, key, value)
				SELECT {:new_uuid} || '_' || h.key, {:new_id}, h.key, h.value
				FROM pt_workflow_events_history h
				WHERE h.workflow_id = {:orig_id} AND h.function_id < {:start_step}
				  AND h.function_id = (
					SELECT MAX(h2.function_id) FROM pt_workflow_events_history h2
					WHERE h2.workflow_id = {:orig_id} AND h2.key = h.key AND h2.function_id < {:start_step}
				  )`).Bind(dbx.Params{
				"new_uuid":   newID,
				"new_id":     newID,
				"orig_id":    input.originalWorkflowID,
				"start_step": input.startStepID,
			}).Execute()
			if txErr != nil {
				return fmt.Errorf("failed to copy latest workflow events: %w", txErr)
			}
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	s.eventBus.Notify("queue::" + ptInternalQueueName)

	return newID, nil
}

func (s *sqliteSysDB) recordOperationStart(ctx context.Context, input recordOperationStartDBInput) error {
	// ON CONFLICT path: a row already exists for this (workflow, function), the
	// previous run crashed mid-step or this is a recovery retry. Clear the
	// prior partial state so checkOperationExecution returns (nil, nil) and
	// the step re-executes.
	_, err := s.app.DB().NewQuery(`INSERT INTO pt_operation_outputs
		(id, workflow_id, function_id, function_name, output, error, started_at_epoch_ms, ended_at_epoch_ms)
		VALUES ({:id}, {:wf_id}, {:func_id}, {:fn_name}, '', '', {:started_at}, 0)
		ON CONFLICT(workflow_id, function_id) DO UPDATE SET
			function_name = excluded.function_name,
			output = '',
			error = '',
			started_at_epoch_ms = excluded.started_at_epoch_ms,
			ended_at_epoch_ms = 0`).Bind(dbx.Params{
		"id":         fmt.Sprintf("%s_%d", input.workflowUUID, input.functionID),
		"wf_id":      input.workflowUUID,
		"func_id":    input.functionID,
		"fn_name":    input.functionName,
		"started_at": input.startedAt,
	}).Execute()
	if err != nil {
		return fmt.Errorf("failed to record step start: %w", err)
	}
	return nil
}

func (s *sqliteSysDB) recordOperationResult(ctx context.Context, input recordOperationResultDBInput) error {
	_, err := s.app.DB().NewQuery(`UPDATE pt_operation_outputs SET
			output = {:output},
			error = {:error},
			function_name = {:fn_name},
			started_at_epoch_ms = {:started_at},
			ended_at_epoch_ms = {:ended_at}
		WHERE workflow_id = {:wf_id} AND function_id = {:func_id}`).Bind(dbx.Params{
		"output":     derefStr(input.output),
		"error":      derefStr(input.errorMsg),
		"fn_name":    input.functionName,
		"started_at": input.startedAt,
		"ended_at":   input.endedAt,
		"wf_id":      input.workflowUUID,
		"func_id":    input.functionID,
	}).Execute()
	if err != nil {
		return fmt.Errorf("failed to record step result: %w", err)
	}
	return nil
}

func (s *sqliteSysDB) checkOperationExecution(ctx context.Context, input checkOperationExecutionDBInput) (*recordedResult, error) {
	// Check workflow status first
	var workflowStatus StatusType
	err := s.app.DB().Select("status").
		From("pt_workflow_status").
		Where(dbx.HashExp{"id": input.workflowUUID}).
		Row(&workflowStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, newErrWorkflowNotFound(input.workflowUUID)
		}
		return nil, fmt.Errorf("failed to get workflow status: %w", err)
	}

	if workflowStatus == StatusCancelled {
		return nil, newErrCancelled(input.workflowUUID)
	}

	// Check operation outputs, only completed rows (ended_at_epoch_ms != 0) are
	// treated as checkpoints. A row with ended_at_epoch_ms == 0 means a previous
	// run started the step but crashed before saving the result, so we re-execute.
	var outputString, errorStr, functionName sql.NullString
	var endedAt sql.NullInt64
	err = s.app.DB().Select("output", "error", "function_name", "ended_at_epoch_ms").
		From("pt_operation_outputs").
		Where(dbx.HashExp{"workflow_id": input.workflowUUID, "function_id": input.functionID}).
		Row(&outputString, &errorStr, &functionName, &endedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get operation outputs: %w", err)
	}

	if !endedAt.Valid || endedAt.Int64 == 0 {
		return nil, nil
	}

	result := &recordedResult{}
	if outputString.Valid {
		result.output = &outputString.String
	}
	if errorStr.Valid && errorStr.String != "" {
		result.errorMsg = &errorStr.String
	}
	if functionName.Valid {
		result.functionName = &functionName.String
	}
	return result, nil
}

func (s *sqliteSysDB) getWorkflowSteps(ctx context.Context, input getWorkflowStepsInput) ([]stepInfo, error) {
	rows, err := s.app.DB().Select("function_id", "function_name", "output", "error", "started_at_epoch_ms", "ended_at_epoch_ms").
		From("pt_operation_outputs").
		Where(dbx.HashExp{"workflow_id": input.workflowID}).
		OrderBy("function_id ASC").
		Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to query workflow steps: %w", err)
	}
	defer func() { _ = rows.Close() }()

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

func (s *sqliteSysDB) recordChildWorkflow(ctx context.Context, input recordChildWorkflowDBInput) error {
	_, err := s.app.DB().Insert("pt_operation_outputs", dbx.Params{
		"id":                fmt.Sprintf("%s_%d", input.workflowUUID, input.functionID),
		"workflow_id":       input.workflowUUID,
		"function_id":       input.functionID,
		"function_name":     "childWorkflow",
		"child_workflow_id": input.childWorkflowID,
		"output":            "",
		"error":             "",
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
	err := s.app.DB().Select("child_workflow_id").
		From("pt_operation_outputs").
		Where(dbx.HashExp{"workflow_id": workflowUUID, "function_id": functionID}).
		Row(&childID)

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
	_, err := s.app.DB().NewQuery(`INSERT INTO pt_operation_outputs
		(id, workflow_id, function_id, function_name, output, error, child_workflow_id)
		VALUES ({:id}, {:wf_id}, {:func_id}, 'pt.getResult', {:output}, {:error}, {:child_id})
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

func (s *sqliteSysDB) send(ctx context.Context, input sendInput) error {
	// When called as a workflow step, derive a deterministic row id from
	// (producer_workflow, producer_step) so step replay after crash does not
	// re-insert the message. ON CONFLICT DO NOTHING makes the insert idempotent.
	var msgID string
	if input.ProducerWorkflow != "" {
		msgID = fmt.Sprintf("snd_%s_%d", input.ProducerWorkflow, input.ProducerStepID)
	} else {
		msgID = core.GenerateDefaultRandomId()
	}
	_, err := s.app.DB().NewQuery(`INSERT INTO pt_notifications
		(id, destination_id, topic, message, created_at_epoch_ms, consumed)
		VALUES ({:id}, {:dest}, {:topic}, {:msg}, {:ts}, FALSE)
		ON CONFLICT (id) DO NOTHING`).Bind(dbx.Params{
		"id":    msgID,
		"dest":  input.DestinationUUID,
		"topic": input.Topic,
		"msg":   derefStr(input.Message),
		"ts":    time.Now().UnixMilli(),
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
			FROM pt_notifications
			WHERE destination_id = {:dest} AND topic = {:topic} AND consumed = FALSE
			ORDER BY created_at_epoch_ms ASC
			LIMIT 1
		)
		UPDATE pt_notifications SET consumed = TRUE
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

	// No message found, wait with event bus
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
			ch = s.eventBus.Swap(payload, ch)

			// Try again
			err = s.app.DB().NewQuery(`
				WITH oldest AS (
					SELECT id, message
					FROM pt_notifications
					WHERE destination_id = {:dest} AND topic = {:topic} AND consumed = FALSE
					ORDER BY created_at_epoch_ms ASC
					LIMIT 1
				)
				UPDATE pt_notifications SET consumed = TRUE
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

func (s *sqliteSysDB) setEvent(ctx context.Context, input setValueInput) error {
	_, err := s.app.DB().NewQuery(`INSERT INTO pt_workflow_events (id, workflow_id, key, value)
		VALUES ({:id}, {:wf_id}, {:key}, {:value})
		ON CONFLICT (workflow_id, key)
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
	err := s.app.DB().Select("value").
		From("pt_workflow_events").
		Where(dbx.HashExp{"workflow_id": input.targetWorkflowUUID, "key": input.key}).
		Row(&value)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	if value.Valid {
		return &value.String, nil
	}

	// Event not found, wait with event bus
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
			ch = s.eventBus.Swap(payload, ch)

			err = s.app.DB().Select("value").
				From("pt_workflow_events").
				Where(dbx.HashExp{"workflow_id": input.targetWorkflowUUID, "key": input.key}).
				Row(&value)

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

func (s *sqliteSysDB) dequeueWorkflows(ctx context.Context, input dequeueWorkflowsInput) ([]dequeuedWorkflow, error) {
	limit := input.limit
	if limit <= 0 {
		limit = defaultMaxTasksPerIteration
	}

	hasWorker := input.workerConcurrency != nil && *input.workerConcurrency > 0
	hasGlobal := input.globalConcurrency != nil && *input.globalConcurrency > 0
	hasRate := input.rateLimit != nil && input.rateLimit.Limit > 0 && input.rateLimit.Period > 0

	params := dbx.Params{
		"pending":    string(StatusPending),
		"enqueued":   string(StatusEnqueued),
		"executor":   input.executorID,
		"app_ver":    input.appVersion,
		"updated_at": time.Now().UnixMilli(),
		"base_limit": limit,
	}

	// Build partition filter used by both counts and candidates
	partFilter := ""
	if input.partitioned && input.partitionKey != "" {
		partFilter = " AND queue_partition_key = {:part_key}"
		params["part_key"] = input.partitionKey
	}

	orderBy := "created_at_epoch_ms ASC"
	if input.priorityEnabled {
		orderBy = "priority ASC, " + orderBy
	}

	if !hasWorker && !hasGlobal && !hasRate {
		// No concurrency/rate constraints, use query builder for a simple dequeue
		sub := s.app.DB().Select("id").
			From("pt_workflow_status").
			Where(dbx.HashExp{"queue_name": input.queueName, "status": string(StatusEnqueued)})
		if input.partitioned && input.partitionKey != "" {
			sub.AndWhere(dbx.HashExp{"queue_partition_key": input.partitionKey})
		}
		if input.priorityEnabled {
			sub.OrderBy("priority ASC", "created_at_epoch_ms ASC")
		} else {
			sub.OrderBy("created_at_epoch_ms ASC")
		}
		sub.Limit(int64(limit))

		subQuery := sub.Build()
		for k, v := range subQuery.Params() {
			params[k] = v
		}

		query := fmt.Sprintf(`UPDATE pt_workflow_status
			SET status = {:pending}, executor_id = {:executor},
			    application_version = {:app_ver}, updated_at_epoch_ms = {:updated_at}
			WHERE id IN (%s)
			RETURNING id, queue_name, name, inputs`, subQuery.SQL())

		return s.scanDequeuedWorkflows(s.app.DB().NewQuery(query).Bind(params))
	}

	// Concurrency/rate constraints present, use a single atomic CTE so the
	// counts and the UPDATE share the same write-lock (no TOCTOU race).

	var rateCutoff int64
	if hasRate {
		rateCutoff = time.Now().Add(-input.rateLimit.Period).UnixMilli()
	}
	params["rate_cutoff"] = rateCutoff
	params["queue_name"] = input.queueName

	// CTE 1: counts, single scan of the queue to compute all constraint counts
	countsCTE := fmt.Sprintf(`counts AS (
		SELECT
			COALESCE(SUM(CASE WHEN status = {:pending} AND executor_id = {:executor} THEN 1 ELSE 0 END), 0) AS worker_running,
			COALESCE(SUM(CASE WHEN status = {:pending} THEN 1 ELSE 0 END), 0) AS global_running,
			COALESCE(SUM(CASE WHEN status != {:enqueued} AND updated_at_epoch_ms >= {:rate_cutoff} THEN 1 ELSE 0 END), 0) AS recent_count
		FROM pt_workflow_status
		WHERE queue_name = {:queue_name}%s
	)`, partFilter)

	// CTE 2: effective_limit, MIN of base limit and each configured constraint
	limitParts := []string{"SELECT {:base_limit} AS lim"}
	if hasWorker {
		params["worker_conc"] = *input.workerConcurrency
		limitParts = append(limitParts, "SELECT {:worker_conc} - worker_running FROM counts")
	}
	if hasGlobal {
		params["global_conc"] = *input.globalConcurrency
		limitParts = append(limitParts, "SELECT {:global_conc} - global_running FROM counts")
	}
	if hasRate {
		params["rate_max"] = input.rateLimit.Limit
		limitParts = append(limitParts, "SELECT {:rate_max} - recent_count FROM counts")
	}
	effectiveLimitCTE := fmt.Sprintf(
		"effective_limit AS (SELECT MIN(lim) AS lim FROM (%s))",
		strings.Join(limitParts, " UNION ALL "),
	)

	// CTE 3: candidates, rows to dequeue, capped by the effective limit
	candidatesCTE := fmt.Sprintf(`candidates AS (
		SELECT id FROM pt_workflow_status
		WHERE queue_name = {:queue_name} AND status = {:enqueued}%s
		ORDER BY %s
		LIMIT MAX((SELECT lim FROM effective_limit), 0)
	)`, partFilter, orderBy)

	query := fmt.Sprintf(`WITH %s, %s, %s
		UPDATE pt_workflow_status
		SET status = {:pending}, executor_id = {:executor},
		    application_version = {:app_ver}, updated_at_epoch_ms = {:updated_at}
		WHERE id IN (SELECT id FROM candidates)
		RETURNING id, queue_name, name, inputs`,
		countsCTE, effectiveLimitCTE, candidatesCTE,
	)

	return s.scanDequeuedWorkflows(s.app.DB().NewQuery(query).Bind(params))
}

func (s *sqliteSysDB) scanDequeuedWorkflows(q *dbx.Query) ([]dequeuedWorkflow, error) {
	rows, err := q.Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to dequeue workflows: %w", err)
	}
	defer func() { _ = rows.Close() }()

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
	return workflows, nil
}

func (s *sqliteSysDB) clearQueueAssignment(ctx context.Context, workflowID string) (bool, error) {
	var changed bool
	err := s.app.RunInTransaction(func(txApp core.App) error {
		record, err := txApp.FindRecordById(collectionStatus, workflowID)
		if err != nil {
			return err
		}
		if record.GetString("status") != string(StatusPending) {
			return nil
		}
		record.Set("status", string(StatusEnqueued))
		record.Set("queue_name", ptInternalQueueName)
		record.Set("updated_at_epoch_ms", time.Now().UnixMilli())
		if err := txApp.SaveNoValidate(record); err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("failed to clear queue assignment: %w", err)
	}

	if changed {
		s.eventBus.Notify("queue::" + ptInternalQueueName)
	}
	return changed, nil
}

func (s *sqliteSysDB) getQueuePartitions(ctx context.Context, queueName string) ([]string, error) {
	rows, err := s.app.DB().Select("queue_partition_key").
		Distinct(true).
		From("pt_workflow_status").
		Where(dbx.And(
			dbx.HashExp{"queue_name": queueName},
			dbx.NewExp("queue_partition_key IS NOT NULL"),
			dbx.NewExp("queue_partition_key != ''"),
		)).
		Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to get queue partitions: %w", err)
	}
	defer func() { _ = rows.Close() }()

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

func (s *sqliteSysDB) waitForEnqueue(ctx context.Context, queueName string) chan struct{} {
	return s.eventBus.Wait("queue::" + queueName)
}

func (s *sqliteSysDB) stopWaitForEnqueue(queueName string, ch chan struct{}) {
	s.eventBus.Remove("queue::"+queueName, ch)
}

func (s *sqliteSysDB) updateAppStatus(ctx context.Context, input updateAppStatusDBInput) error {
	record, err := s.app.FindRecordById(collectionStatus, input.workflowID)
	if err != nil {
		return fmt.Errorf("failed to find workflow status record: %w", err)
	}

	record.Set("app_status", input.appStatus)
	record.Set("app_status_color", input.appStatusColor)
	record.Set("updated_at_epoch_ms", time.Now().UnixMilli())

	if err := s.app.SaveNoValidate(record); err != nil {
		return fmt.Errorf("failed to update app status: %w", err)
	}
	return nil
}

func (s *sqliteSysDB) garbageCollectWorkflows(ctx context.Context, input garbageCollectWorkflowsInput) error {
	cutoffMs := input.cutoffTime.UnixMilli()

	params := dbx.Params{
		"cutoff":   cutoffMs,
		"pending":  string(StatusPending),
		"enqueued": string(StatusEnqueued),
	}

	const idSubquery = `SELECT id FROM pt_workflow_status
		WHERE created_at_epoch_ms < {:cutoff}
		  AND status NOT IN ({:pending}, {:enqueued})`

	// Related tables reference pt_workflow_status with CascadeDelete, but raw SQL
	// bypasses PocketBase's cascade logic. Delete from related tables first.
	type relatedTable struct {
		name string
		fk   string // foreign key column pointing to pt_workflow_status.id
	}
	related := []relatedTable{
		{collectionOperationOutputs, "workflow_id"},
		{collectionNotifications, "destination_id"},
		{collectionWorkflowEvents, "workflow_id"},
		{collectionWorkflowEventsHist, "workflow_id"},
		{collectionProducts, "workflow_id"},
	}

	var rowsAffected int64
	err := s.app.RunInTransaction(func(txApp core.App) error {
		for _, rel := range related {
			if _, err := txApp.DB().NewQuery(
				fmt.Sprintf("DELETE FROM %s WHERE %s IN (%s)", rel.name, rel.fk, idSubquery),
			).Bind(params).Execute(); err != nil {
				return fmt.Errorf("failed to gc %s: %w", rel.name, err)
			}
		}

		result, err := txApp.DB().NewQuery(`DELETE FROM pt_workflow_status
			WHERE created_at_epoch_ms < {:cutoff}
			  AND status NOT IN ({:pending}, {:enqueued})`).Bind(params).Execute()
		if err != nil {
			return fmt.Errorf("failed to gc workflows: %w", err)
		}
		rowsAffected, _ = result.RowsAffected()
		return nil
	})
	if err != nil {
		return err
	}

	s.logger.Info("Garbage collected workflows", "cutoff", cutoffMs, "deleted", rowsAffected)
	return nil
}

func (s *sqliteSysDB) setKV(ctx context.Context, input setKVInput) error {
	_, err := s.app.DB().NewQuery(`INSERT INTO pt_kv (id, key, value, updated_at_epoch_ms)
		VALUES ({:id}, {:key}, {:value}, {:updated_at})
		ON CONFLICT (key)
		DO UPDATE SET value = excluded.value, updated_at_epoch_ms = excluded.updated_at_epoch_ms`).Bind(dbx.Params{
		"id":         core.GenerateDefaultRandomId(),
		"key":        input.key,
		"value":      derefStr(input.value),
		"updated_at": time.Now().UnixMilli(),
	}).Execute()

	if err != nil {
		return fmt.Errorf("failed to set KV: %w", err)
	}
	return nil
}

func (s *sqliteSysDB) getKV(ctx context.Context, input getKVInput) (*string, error) {
	var value sql.NullString
	err := s.app.DB().Select("value").
		From("pt_kv").
		Where(dbx.HashExp{"key": input.key}).
		Row(&value)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get KV: %w", err)
	}

	if value.Valid {
		return &value.String, nil
	}
	return nil, nil
}

func (s *sqliteSysDB) deleteKV(ctx context.Context, input deleteKVInput) error {
	_, err := s.app.DB().Delete("pt_kv", dbx.HashExp{
		"key": input.key,
	}).Execute()

	if err != nil {
		return fmt.Errorf("failed to delete KV: %w", err)
	}
	return nil
}
