package turbine

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

var validAppStatusColors = map[string]bool{
	"green": true, "red": true, "yellow": true, "blue": true, "gray": true,
	"lime": true, "orange": true, "purple": true, "pink": true, "cyan": true,
}

func validateAppStatusColor(color string) error {
	if !validAppStatusColors[color] {
		return fmt.Errorf("invalid app status color %q: must be one of green, red, yellow, blue, gray, lime, orange, purple, pink, cyan", color)
	}
	return nil
}

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

// Handle provides methods to interact with a running or completed workflow.
type Handle[R any] interface {
	GetResult(opts ...GetResultOption) (R, error)
	GetStatus() (Status, error)
	GetWorkflowID() string
}

// GetResultOption is a functional option for GetResult.
type GetResultOption func(*getResultOptions)

type getResultOptions struct {
	pollInterval time.Duration
}

// StepInfo contains information about a workflow step execution.
type StepInfo struct {
	WorkflowID   string `json:"workflow_id"`
	FunctionID   int    `json:"function_id"`
	FunctionName string `json:"function_name"`
	Output       string `json:"output,omitempty"`
	Error        string `json:"error,omitempty"`
	StartedAt    int64  `json:"started_at_epoch_ms,omitempty"`
	EndedAt      int64  `json:"ended_at_epoch_ms,omitempty"`
}

// workflowState holds the runtime state for a workflow execution.
type workflowState struct {
	workflowID     string
	workflowName   string
	stepID         atomic.Int64
	isWithinStep   bool
	recovering     bool
	appStatus      string
	appStatusColor string
}

func (ws *workflowState) nextStepID() int {
	return int(ws.stepID.Add(1))
}

// RateLimiter configures rate limiting for workflow queue execution.
type RateLimiter struct {
	Limit  int
	Period time.Duration
}

// ProductSender is the interface for sending products to external systems.
// Users implement this to define custom destinations. The data reader carries
// the full file bytes; the concrete value is *bytes.Reader, so senders that
// need to rewind for retries can type-assert to io.Seeker.
type ProductSender interface {
	Send(ctx context.Context, product ProductRecord, data io.Reader) error
}

// ProductRecord is the metadata reference to a stored product passed to
// ProductSender.Send(). It is JSON-serializable so it can also be used as a
// workflow input (see WorkflowSender).
type ProductRecord struct {
	ID       string         `json:"id"`
	FileName string         `json:"file_name"`
	Size     int            `json:"size"`
	Metadata map[string]any `json:"metadata"`
}

