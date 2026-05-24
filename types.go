package turbine

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/YakirOren/turbine/internal/sysdb"
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
//
// Underlying type lives in internal/sysdb so the storage layer can reference
// it without importing root turbine. Aliased here so the public API surface
// (turbine.StatusType, turbine.Status*, methods on StatusType) is unchanged.
type StatusType = sysdb.StatusType

const (
	StatusPending                     = sysdb.StatusPending
	StatusEnqueued                    = sysdb.StatusEnqueued
	StatusSuccess                     = sysdb.StatusSuccess
	StatusError                       = sysdb.StatusError
	StatusCancelled                   = sysdb.StatusCancelled
	StatusMaxRecoveryAttemptsExceeded = sysdb.StatusMaxRecoveryAttemptsExceeded
	StatusWaitingForApproval          = sysdb.StatusWaitingForApproval
)

// Status contains information about a workflow's current state.
//
// Underlying type lives in internal/sysdb; aliased here for public API
// stability. All fields and JSON tags are preserved.
type Status = sysdb.Status

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
//
// Underlying type lives in internal/sysdb; aliased here for public API stability.
type RateLimiter = sysdb.RateLimiter

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

