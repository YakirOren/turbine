// Package sysdb owns turbine's SQLite persistence layer: workflow status,
// step checkpoints, durable messaging, and the global key-value store. All
// four aggregates share the PocketBase app + event bus and are exposed
// together via Store.
//
// These types describe the request and response shapes for sysdb methods.
// They are not part of the public surface of package turbine and may change
// without notice.
package sysdb

import (
	"strings"
	"time"
)

const (
	// PtInternalQueueName is the queue name used for workflows that are not
	// otherwise assigned to a named queue. Subscribers wake up on this queue
	// any time a workflow is enqueued or a step is recorded.
	PtInternalQueueName = "_pt_internal_queue"
	// DBRetryInterval is the default poll interval for blocking-wait helpers.
	DBRetryInterval = 1 * time.Second
)

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

type InsertWorkflowResult struct {
	Attempts         int
	Status           StatusType
	Name             string
	QueueName        *string
	Timeout          time.Duration
	WorkflowDeadline time.Time
	OwnerXID         string
	AppStatus        string
	AppStatusColor   string
	HasSteps         bool
}

type InsertStatusDBInput struct {
	Status            Status
	MaxRetries        int
	OwnerXID          *string
	IncrementAttempts bool
}

type UpdateWorkflowOutcomeDBInput struct {
	WorkflowID string
	Status     StatusType
	Output     *string
	ErrorMsg   *string
}

type ListWorkflowsDBInput struct {
	Status             []StatusType
	WorkflowName       []string
	ExecutorIDs        []string
	ApplicationVersion []string
	WorkflowIDs        []string
	Limit              int
	LoadInput          bool
	SortAscending      bool
	CreatedBefore      *time.Time
	CreatedAfter       *time.Time
}

type CancelWorkflowDBInput struct {
	WorkflowID string
}

type ResumeWorkflowDBInput struct {
	WorkflowID string
	ExecutorID string
	AppVersion string
}

type ForkWorkflowDBInput struct {
	OriginalWorkflowID string
	NewWorkflowID      string
	StartStepID        int
	Input              *string
}

type RecordChildWorkflowDBInput struct {
	WorkflowUUID    string
	FunctionID      int
	ChildWorkflowID string
}

type RecordChildGetResultDBInput struct {
	WorkflowUUID string
	FunctionID   int
	Output       *string
	ErrorMsg     *string
}

type RecordOperationStartDBInput struct {
	WorkflowUUID string
	FunctionID   int
	FunctionName string
	StartedAt    int64
}

type RecordOperationResultDBInput struct {
	WorkflowUUID string
	FunctionID   int
	FunctionName string
	Output       *string
	ErrorMsg     *string
	StartedAt    int64
	EndedAt      int64
}

type CheckOperationExecutionDBInput struct {
	WorkflowUUID string
	FunctionID   int
}

type RecordedResult struct {
	Output       *string
	ErrorMsg     *string
	FunctionName *string
}

type GetWorkflowStepsInput struct {
	WorkflowID string
}

// StepRow is the sysdb representation of a row in pt_operation_outputs.
// Note: this is distinct from the public turbine.StepInfo, which is a
// presentation-friendly view used in the public API.
type StepRow struct {
	WorkflowUUID string
	FunctionID   int
	FunctionName string
	Output       *string
	ErrorMsg     *string
	StartedAt    *int64
	EndedAt      *int64
}

// SendInput is the input for sending a notification to a workflow.
//
// ProducerWorkflow + ProducerStepID, when ProducerWorkflow is non-empty, form
// an idempotency key so a step that crashes mid-Send and replays on recovery
// does not deliver the message twice. Direct (non-step) Send leaves
// ProducerWorkflow empty and falls back to a random row id, the producer is
// responsible for at-most-once semantics in that case.
type SendInput struct {
	DestinationUUID  string
	Topic            string
	Message          *string
	ProducerWorkflow string
	ProducerStepID   int
}

type RecvInput struct {
	WorkflowUUID string
	Topic        string
	Timeout      time.Duration
}

// SetValueInput is the input for setting a workflow event.
type SetValueInput struct {
	WorkflowUUID string
	Key          string
	Value        *string
}

type GetEventInput struct {
	TargetWorkflowUUID string
	Key                string
	Timeout            time.Duration
}

type DequeueWorkflowsInput struct {
	QueueName         string
	ExecutorID        string
	AppVersion        string
	Limit             int
	WorkerConcurrency *int
	GlobalConcurrency *int
	PriorityEnabled   bool
	RateLimit         *RateLimiter
	Partitioned       bool
	PartitionKey      string
}

type DequeuedWorkflow struct {
	WorkflowID string
	QueueName  string
	Name       string
	Input      *string
}

type UpdateAppStatusDBInput struct {
	WorkflowID     string
	AppStatus      string
	AppStatusColor string
}

type GarbageCollectWorkflowsInput struct {
	CutoffTime time.Time
}

type SetKVInput struct {
	Key   string
	Value *string
}

type GetKVInput struct {
	Key string
}

type DeleteKVInput struct {
	Key string
}
