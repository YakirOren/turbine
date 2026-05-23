package turbine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// steps manages the pt_operation_outputs table.
// Step checkpoints + child-workflow records.
type steps struct {
	app    core.App
	logger *slog.Logger
}

func newSteps(app core.App, logger *slog.Logger) *steps {
	return &steps{
		app:    app,
		logger: logger.With("service", "steps"),
	}
}

func (s *steps) recordOperationStart(ctx context.Context, input recordOperationStartDBInput) error {
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

func (s *steps) recordOperationResult(ctx context.Context, input recordOperationResultDBInput) error {
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

func (s *steps) checkOperationExecution(ctx context.Context, input checkOperationExecutionDBInput) (*recordedResult, error) {
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

func (s *steps) getWorkflowSteps(ctx context.Context, input getWorkflowStepsInput) ([]stepInfo, error) {
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

func (s *steps) recordChildWorkflow(ctx context.Context, input recordChildWorkflowDBInput) error {
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

func (s *steps) checkChildWorkflow(ctx context.Context, workflowUUID string, functionID int) (*string, error) {
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

func (s *steps) recordChildGetResult(ctx context.Context, input recordChildGetResultDBInput) error {
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
