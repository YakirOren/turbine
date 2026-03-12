package pocketflow

import (
	"context"
	"sync/atomic"
	"time"
)

// WorkflowStatusType represents the current execution state of a workflow.
type WorkflowStatusType string

const (
	WorkflowStatusPending                     WorkflowStatusType = "PENDING"
	WorkflowStatusEnqueued                    WorkflowStatusType = "ENQUEUED"
	WorkflowStatusSuccess                     WorkflowStatusType = "SUCCESS"
	WorkflowStatusError                       WorkflowStatusType = "ERROR"
	WorkflowStatusCancelled                   WorkflowStatusType = "CANCELLED"
	WorkflowStatusMaxRecoveryAttemptsExceeded WorkflowStatusType = "MAX_RECOVERY_ATTEMPTS_EXCEEDED"
)

// WorkflowStatus contains information about a workflow's current state.
type WorkflowStatus struct {
	ID                 string             `json:"workflow_id"`
	Status             WorkflowStatusType `json:"status"`
	Name               string             `json:"name"`
	Output             any                `json:"output,omitempty"`
	Error              error              `json:"error,omitempty"`
	ExecutorID         string             `json:"executor_id"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
	ApplicationVersion string             `json:"application_version"`
	ApplicationID      string             `json:"application_id,omitempty"`
	Attempts           int                `json:"attempts"`
	QueueName          string             `json:"queue_name,omitempty"`
	Timeout            time.Duration      `json:"timeout,omitempty"`
	Deadline           time.Time          `json:"deadline"`
	StartedAt          time.Time          `json:"started_at"`
	DeduplicationID    string             `json:"deduplication_id,omitempty"`
	Input              any                `json:"input,omitempty"`
	Priority           int                `json:"priority,omitempty"`
	QueuePartitionKey  string             `json:"queue_partition_key,omitempty"`
	ForkedFrom         string             `json:"forked_from,omitempty"`
	ParentWorkflowID   string             `json:"parent_workflow_id,omitempty"`
}

// WorkflowHandle provides methods to interact with a running or completed workflow.
type WorkflowHandle[R any] interface {
	GetResult(opts ...GetResultOption) (R, error)
	GetStatus() (WorkflowStatus, error)
	GetWorkflowID() string
}

// GetResultOption is a functional option for GetResult.
type GetResultOption func(*getResultOptions)

type getResultOptions struct {
	pollInterval time.Duration
}

// StepInfo contains information about a workflow step execution.
type StepInfo struct {
	WorkflowID string `json:"workflow_id"`
	FunctionID   int    `json:"function_id"`
	FunctionName string `json:"function_name"`
	Output       string `json:"output,omitempty"`
	Error        string `json:"error,omitempty"`
	StartedAt    int64  `json:"started_at_epoch_ms,omitempty"`
	EndedAt      int64  `json:"ended_at_epoch_ms,omitempty"`
}

// workflowState holds the runtime state for a workflow execution.
type workflowState struct {
	workflowID   string
	stepID       atomic.Int64
	isWithinStep bool
}

func (ws *workflowState) nextStepID() int {
	return int(ws.stepID.Add(1))
}

// Internal DB input/output types

type insertWorkflowResult struct {
	attempts         int
	status           WorkflowStatusType
	name             string
	queueName        *string
	timeout          time.Duration
	workflowDeadline time.Time
	ownerXID         string
}

type insertWorkflowStatusDBInput struct {
	status            WorkflowStatus
	maxRetries        int
	ownerXID          *string
	incrementAttempts bool
}

type updateWorkflowOutcomeDBInput struct {
	workflowID string
	status     WorkflowStatusType
	output     *string
	errorMsg   *string
}

type listWorkflowsDBInput struct {
	status             []WorkflowStatusType
	workflowName       []string
	executorIDs        []string
	applicationVersion []string
	workflowIDs        []string
	limit              int
	loadInput          bool
	sortAscending      bool
	createdBefore      *time.Time
	createdAfter       *time.Time
}

type cancelWorkflowDBInput struct {
	workflowID string
}

type resumeWorkflowDBInput struct {
	workflowID  string
	executorID  string
	appVersion  string
}

type forkWorkflowDBInput struct {
	originalWorkflowID string
	newWorkflowID      string
	startStepID        int
	input              *string
}

type recordChildWorkflowDBInput struct {
	workflowUUID   string
	functionID     int
	childWorkflowID string
}

type recordChildGetResultDBInput struct {
	workflowUUID string
	functionID   int
	output       *string
	errorMsg     *string
}

type recordOperationResultDBInput struct {
	workflowUUID string
	functionID   int
	functionName string
	output       *string
	errorMsg     *string
	startedAt    int64
	endedAt      int64
}

type checkOperationExecutionDBInput struct {
	workflowUUID string
	functionID   int
}

type recordedResult struct {
	output       *string
	errorMsg     *string
	functionName *string
}

type getWorkflowStepsInput struct {
	workflowID string
}

type stepInfo struct {
	workflowUUID string
	functionID   int
	functionName string
	output       *string
	errorMsg     *string
	startedAt    *int64
	endedAt      *int64
}

// WorkflowSendInput is the input for sending a notification to a workflow.
type WorkflowSendInput struct {
	DestinationUUID string
	Topic           string
	Message         *string
}

type recvInput struct {
	workflowUUID string
	topic        string
	timeout      time.Duration
}

// WorkflowSetEventInput is the input for setting a workflow event.
type WorkflowSetEventInput struct {
	WorkflowUUID string
	Key          string
	Value        *string
}

type getEventInput struct {
	targetWorkflowUUID string
	key                string
	timeout            time.Duration
}

type dequeueWorkflowsInput struct {
	queueName         string
	executorID        string
	appVersion        string
	limit             int
	workerConcurrency *int
	globalConcurrency *int
	priorityEnabled   bool
	rateLimit         *RateLimiter
	partitioned       bool
	partitionKey      string
}

type dequeuedWorkflow struct {
	workflowID string
	queueName  string
	name       string
	input      *string
}

type garbageCollectWorkflowsInput struct {
	cutoffTime time.Time
}

// RateLimiter configures rate limiting for workflow queue execution.
type RateLimiter struct {
	Limit  int
	Period time.Duration
}

// systemDatabase is the internal interface for database operations.
type systemDatabase interface {
	launch(ctx context.Context)
	shutdown(ctx context.Context, timeout time.Duration)

	insertWorkflowStatus(ctx context.Context, input insertWorkflowStatusDBInput) (*insertWorkflowResult, error)
	listWorkflows(ctx context.Context, input listWorkflowsDBInput) ([]WorkflowStatus, error)
	updateWorkflowOutcome(ctx context.Context, input updateWorkflowOutcomeDBInput) error
	awaitWorkflowResult(ctx context.Context, workflowID string, pollInterval time.Duration) (*string, error)
	cancelWorkflow(ctx context.Context, input cancelWorkflowDBInput) error
	resumeWorkflow(ctx context.Context, input resumeWorkflowDBInput) error
	forkWorkflow(ctx context.Context, input forkWorkflowDBInput) (string, error)

	recordChildWorkflow(ctx context.Context, input recordChildWorkflowDBInput) error
	checkChildWorkflow(ctx context.Context, workflowUUID string, functionID int) (*string, error)
	recordChildGetResult(ctx context.Context, input recordChildGetResultDBInput) error

	recordOperationResult(ctx context.Context, input recordOperationResultDBInput) error
	checkOperationExecution(ctx context.Context, input checkOperationExecutionDBInput) (*recordedResult, error)
	getWorkflowSteps(ctx context.Context, input getWorkflowStepsInput) ([]stepInfo, error)

	send(ctx context.Context, input WorkflowSendInput) error
	recv(ctx context.Context, input recvInput) (*string, error)
	setEvent(ctx context.Context, input WorkflowSetEventInput) error
	getEvent(ctx context.Context, input getEventInput) (*string, error)

	dequeueWorkflows(ctx context.Context, input dequeueWorkflowsInput) ([]dequeuedWorkflow, error)
	clearQueueAssignment(ctx context.Context, workflowID string) (bool, error)
	getQueuePartitions(ctx context.Context, queueName string) ([]string, error)
	waitForEnqueue(ctx context.Context, queueName string) chan struct{}
	stopWaitForEnqueue(queueName string, ch chan struct{})

	garbageCollectWorkflows(ctx context.Context, input garbageCollectWorkflowsInput) error
}
