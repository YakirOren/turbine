package pbdbos

import "fmt"

// DBOSErrorCode represents the different types of errors that can occur in DBOS operations.
type DBOSErrorCode int

const (
	ConflictingIDError           DBOSErrorCode = iota + 1
	InitializationError
	NonExistentWorkflowError
	ConflictingWorkflowError
	WorkflowCancelled
	UnexpectedStep
	AwaitedWorkflowCancelled
	ConflictingRegistrationError
	WorkflowUnexpectedTypeError
	WorkflowExecutionError
	StepExecutionError
	DeadLetterQueueError
	MaxStepRetriesExceeded
	QueueDeduplicated
	TimeoutError
)

// DBOSError is the unified error type for all DBOS operations.
type DBOSError struct {
	Message         string
	Code            DBOSErrorCode
	WorkflowID      string
	DestinationID   string
	StepName        string
	QueueName       string
	DeduplicationID string
	StepID          int
	ExpectedName    string
	RecordedName    string
	MaxRetries      int
	wrappedErr      error
}

func (e *DBOSError) Error() string {
	return fmt.Sprintf("DBOS Error %d: %s", int(e.Code), e.Message)
}

func (e *DBOSError) Unwrap() error {
	return e.wrappedErr
}

func (e *DBOSError) Is(target error) bool {
	t, ok := target.(*DBOSError)
	if !ok {
		return false
	}
	return t.Code != 0 && e.Code == t.Code
}

func newConflictingWorkflowError(workflowID, message string) *DBOSError {
	msg := fmt.Sprintf("Conflicting workflow invocation with the same ID (%s)", workflowID)
	if message != "" {
		msg += ": " + message
	}
	return &DBOSError{Message: msg, Code: ConflictingWorkflowError, WorkflowID: workflowID}
}

func newInitializationError(message string) *DBOSError {
	return &DBOSError{Message: fmt.Sprintf("Error initializing DBOS Transact: %s", message), Code: InitializationError}
}

func newNonExistentWorkflowError(workflowID string) *DBOSError {
	return &DBOSError{Message: fmt.Sprintf("workflow %s does not exist", workflowID), Code: NonExistentWorkflowError, DestinationID: workflowID}
}

func newConflictingRegistrationError(name string) *DBOSError {
	return &DBOSError{Message: fmt.Sprintf("%s is already registered", name), Code: ConflictingRegistrationError}
}

func newUnexpectedStepError(workflowID string, stepID int, expectedName, recordedName string) *DBOSError {
	return &DBOSError{
		Message:      fmt.Sprintf("During execution of workflow %s step %d, function %s was recorded when %s was expected. Check that your workflow is deterministic.", workflowID, stepID, recordedName, expectedName),
		Code:         UnexpectedStep,
		WorkflowID:   workflowID,
		StepID:       stepID,
		ExpectedName: expectedName,
		RecordedName: recordedName,
	}
}

func newAwaitedWorkflowCancelledError(workflowID string) *DBOSError {
	return &DBOSError{Message: fmt.Sprintf("Awaited workflow %s was cancelled", workflowID), Code: AwaitedWorkflowCancelled, WorkflowID: workflowID}
}

func newAwaitedWorkflowMaxStepRetriesExceeded(workflowID string) *DBOSError {
	return &DBOSError{Message: fmt.Sprintf("Awaited workflow %s has exceeded the maximum number of step retries", workflowID), Code: MaxStepRetriesExceeded, WorkflowID: workflowID}
}

func newWorkflowCancelledError(workflowID string) *DBOSError {
	return &DBOSError{Message: fmt.Sprintf("Workflow %s was cancelled", workflowID), Code: WorkflowCancelled}
}

func newWorkflowConflictIDError(workflowID string) *DBOSError {
	return &DBOSError{Message: fmt.Sprintf("Conflicting workflow ID %s", workflowID), Code: ConflictingIDError, WorkflowID: workflowID}
}

func newWorkflowUnexpectedResultType(workflowID, expectedType, actualType string) *DBOSError {
	return &DBOSError{Message: fmt.Sprintf("Workflow %s returned unexpected result type: expected %s, got %s", workflowID, expectedType, actualType), Code: WorkflowUnexpectedTypeError, WorkflowID: workflowID}
}

func newWorkflowUnexpectedInputType(workflowName, expectedType, actualType string) *DBOSError {
	return &DBOSError{Message: fmt.Sprintf("Workflow %s received unexpected input type: expected %s, got %s", workflowName, expectedType, actualType), Code: WorkflowUnexpectedTypeError}
}

func newWorkflowExecutionError(workflowID string, err error) *DBOSError {
	return &DBOSError{Message: fmt.Sprintf("Workflow %s execution error: %s", workflowID, err.Error()), Code: WorkflowExecutionError, WorkflowID: workflowID, wrappedErr: err}
}

func newStepExecutionError(workflowID, stepName string, err error) *DBOSError {
	return &DBOSError{Message: fmt.Sprintf("Step %s in workflow %s execution error: %v", stepName, workflowID, err), Code: StepExecutionError, WorkflowID: workflowID, StepName: stepName, wrappedErr: err}
}

func newDeadLetterQueueError(workflowID string, maxRetries int) *DBOSError {
	return &DBOSError{Message: fmt.Sprintf("Workflow %s has been moved to the dead-letter queue after exceeding the maximum of %d retries", workflowID, maxRetries), Code: DeadLetterQueueError, WorkflowID: workflowID, MaxRetries: maxRetries}
}

func newMaxStepRetriesExceededError(workflowID, stepName string, maxRetries int, err error) *DBOSError {
	return &DBOSError{Message: fmt.Sprintf("Step %s has exceeded its maximum of %d retries: %v", stepName, maxRetries, err), Code: MaxStepRetriesExceeded, WorkflowID: workflowID, StepName: stepName, MaxRetries: maxRetries, wrappedErr: err}
}

func newQueueDeduplicatedError(workflowID, queueName, deduplicationID string) *DBOSError {
	return &DBOSError{Message: fmt.Sprintf("Workflow %s was deduplicated due to an existing workflow in queue %s with deduplication ID %s", workflowID, queueName, deduplicationID), Code: QueueDeduplicated, WorkflowID: workflowID, QueueName: queueName, DeduplicationID: deduplicationID}
}

func newTimeoutError(workflowID, stepName, message string) *DBOSError {
	msg := "Operation timed out"
	if stepName != "" {
		msg = fmt.Sprintf("Step %s timed out", stepName)
	}
	if workflowID != "" {
		msg += fmt.Sprintf(" in workflow %s", workflowID)
	}
	if message != "" {
		msg += ": " + message
	}
	return &DBOSError{Message: msg, Code: TimeoutError, WorkflowID: workflowID, StepName: stepName}
}
