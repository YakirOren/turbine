package sysdb

import "time"

// StatusType represents the current execution state of a workflow.
type StatusType string

const (
	StatusPending                     StatusType = "PENDING"
	StatusEnqueued                    StatusType = "ENQUEUED"
	StatusSuccess                     StatusType = "SUCCESS"
	StatusError                       StatusType = "ERROR"
	StatusCancelled                   StatusType = "CANCELLED"
	StatusMaxRecoveryAttemptsExceeded StatusType = "MAX_RECOVERY_ATTEMPTS_EXCEEDED"
	StatusWaitingForApproval          StatusType = "WAITING_FOR_APPROVAL"
)

// IsTerminal reports whether the status is a final state, the workflow will
// not progress further on its own.
func (s StatusType) IsTerminal() bool {
	switch s {
	case StatusSuccess, StatusError, StatusCancelled, StatusMaxRecoveryAttemptsExceeded:
		return true
	}
	return false
}

// IsTerminalFailure reports whether the status is a terminal failure state.
// Success workflows are terminal but not failed, this returns false for them.
func (s StatusType) IsTerminalFailure() bool {
	switch s {
	case StatusError, StatusCancelled, StatusMaxRecoveryAttemptsExceeded:
		return true
	}
	return false
}

// Status contains information about a workflow's current state.
type Status struct {
	ID                 string        `json:"workflow_id"`
	Status             StatusType    `json:"status"`
	Name               string        `json:"name"`
	Output             any           `json:"output,omitempty"`
	Error              error         `json:"error,omitempty"`
	ExecutorID         string        `json:"executor_id"`
	CreatedAt          time.Time     `json:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at"`
	ApplicationVersion string        `json:"application_version"`
	ApplicationID      string        `json:"application_id,omitempty"`
	Attempts           int           `json:"attempts"`
	QueueName          string        `json:"queue_name,omitempty"`
	Timeout            time.Duration `json:"timeout,omitempty"`
	Deadline           time.Time     `json:"deadline"`
	StartedAt          time.Time     `json:"started_at"`
	DeduplicationID    string        `json:"deduplication_id,omitempty"`
	Input              any           `json:"input,omitempty"`
	Priority           int           `json:"priority,omitempty"`
	QueuePartitionKey  string        `json:"queue_partition_key,omitempty"`
	ForkedFrom         string        `json:"forked_from,omitempty"`
	ParentWorkflowID   string        `json:"parent_workflow_id,omitempty"`
	AppStatus          string        `json:"app_status,omitempty"`
	AppStatusColor     string        `json:"app_status_color,omitempty"`
	Summary            string        `json:"summary,omitempty"`
	Tags               []string      `json:"tags,omitempty"`
}

// RateLimiter configures rate limiting for workflow queue execution.
type RateLimiter struct {
	Limit  int
	Period time.Duration
}
