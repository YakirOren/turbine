package turbine

import (
	"strings"
	"time"
)

const (
	ptInternalQueueName = "_pt_internal_queue"
	dbRetryInterval     = 1 * time.Second
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

// Internal DB input/output types
//
// These types describe the request and response shapes for sysdb methods.
// They are not part of the public surface of package turbine and may change
// without notice.

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

type recordOperationStartDBInput struct {
	workflowUUID string
	functionID   int
	functionName string
	startedAt    int64
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

// sendInput is the input for sending a notification to a workflow.
//
// ProducerWorkflow + ProducerStepID, when ProducerWorkflow is non-empty, form
// an idempotency key so a step that crashes mid-Send and replays on recovery
// does not deliver the message twice. Direct (non-step) Send leaves
// ProducerWorkflow empty and falls back to a random row id, the producer is
// responsible for at-most-once semantics in that case.
type sendInput struct {
	DestinationUUID  string
	Topic            string
	Message          *string
	ProducerWorkflow string
	ProducerStepID   int
}

type recvInput struct {
	workflowUUID string
	topic        string
	timeout      time.Duration
}

// setValueInput is the input for setting a workflow event.
type setValueInput struct {
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
