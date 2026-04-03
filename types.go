package turbine

import (
	"context"
	"fmt"
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
)

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
	stepID         atomic.Int64
	isWithinStep   bool
	recovering     bool
	appStatus      string
	appStatusColor string
}

func (ws *workflowState) nextStepID() int {
	return int(ws.stepID.Add(1))
}

// Internal DB input/output types

type insertWorkflowResult struct {
	attempts         int
	status           StatusType
	name             string
	queueName        *string
	timeout          time.Duration
	workflowDeadline time.Time
	ownerXID         string
	appStatus        string
	appStatusColor   string
	hasSteps         bool
}

type insertStatusDBInput struct {
	status            Status
	maxRetries        int
	ownerXID          *string
	incrementAttempts bool
}

type updateWorkflowOutcomeDBInput struct {
	workflowID string
	status     StatusType
	output     *string
	errorMsg   *string
}

type listWorkflowsDBInput struct {
	status             []StatusType
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
	workflowID string
	executorID string
	appVersion string
}

type forkWorkflowDBInput struct {
	originalWorkflowID string
	newWorkflowID      string
	startStepID        int
	input              *string
}

type recordChildWorkflowDBInput struct {
	workflowUUID    string
	functionID      int
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

// SendInput is the input for sending a notification to a workflow.
type SendInput struct {
	DestinationUUID string
	Topic           string
	Message         *string
}

type recvInput struct {
	workflowUUID string
	topic        string
	timeout      time.Duration
}

// SetValueInput is the input for setting a workflow event.
type SetValueInput struct {
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

type updateAppStatusDBInput struct {
	workflowID     string
	appStatus      string
	appStatusColor string
}

type garbageCollectWorkflowsInput struct {
	cutoffTime time.Time
}

// RateLimiter configures rate limiting for workflow queue execution.
type RateLimiter struct {
	Limit  int
	Period time.Duration
}

// ProductSender is the interface for sending products to external systems.
// Users implement this to define custom destinations.
type ProductSender interface {
	Send(ctx context.Context, product ProductRecord) error
}

// ProductRecord is the reference to a stored product passed to ProductSender.Send().
type ProductRecord struct {
	ID       string         `json:"id"`
	FileName string         `json:"file_name"`
	Size     int            `json:"size"`
	Metadata map[string]any `json:"metadata"`
	FileURL  string         `json:"file_url"`
}

type setKVInput struct {
	key   string
	value *string
}

type getKVInput struct {
	key string
}

type deleteKVInput struct {
	key string
}

// systemDatabase is the internal interface for database operations.
type systemDatabase interface {
	launch(ctx context.Context)
	shutdown(ctx context.Context, timeout time.Duration)

	insertStatus(ctx context.Context, input insertStatusDBInput) (*insertWorkflowResult, error)
	listWorkflows(ctx context.Context, input listWorkflowsDBInput) ([]Status, error)
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

	send(ctx context.Context, input SendInput) error
	recv(ctx context.Context, input recvInput) (*string, error)
	setEvent(ctx context.Context, input SetValueInput) error
	getEvent(ctx context.Context, input getEventInput) (*string, error)

	dequeueWorkflows(ctx context.Context, input dequeueWorkflowsInput) ([]dequeuedWorkflow, error)
	clearQueueAssignment(ctx context.Context, workflowID string) (bool, error)
	getQueuePartitions(ctx context.Context, queueName string) ([]string, error)
	waitForEnqueue(ctx context.Context, queueName string) chan struct{}
	stopWaitForEnqueue(queueName string, ch chan struct{})

	updateAppStatus(ctx context.Context, input updateAppStatusDBInput) error

	garbageCollectWorkflows(ctx context.Context, input garbageCollectWorkflowsInput) error

	setKV(ctx context.Context, input setKVInput) error
	getKV(ctx context.Context, input getKVInput) (*string, error)
	deleteKV(ctx context.Context, input deleteKVInput) error
}
