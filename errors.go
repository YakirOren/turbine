package pocketflow

import "fmt"

// PFErrorCode represents the different types of errors that can occur in PocketFlow operations.
type PFErrorCode int

const (
	ConflictingIDError           PFErrorCode = iota + 1
	_                                          // was InitializationError
	NonExistentWorkflowError
	ConflictingWorkflowError
	WorkflowCancelled
	_                            // was UnexpectedStep
	AwaitedWorkflowCancelled
	ConflictingRegistrationError
	_                            // was WorkflowUnexpectedTypeError
	_                            // was WorkflowExecutionError
	_                            // was StepExecutionError
	DeadLetterQueueError
	MaxStepRetriesExceeded
	QueueDeduplicated
)

// PFError is the unified error type for all PocketFlow operations.
type PFError struct {
	Message         string
	Code            PFErrorCode
	WorkflowID      string
	DestinationID   string
	StepName        string
	QueueName       string
	DeduplicationID string
	MaxRetries      int
	wrappedErr      error
}

func (e *PFError) Error() string {
	return fmt.Sprintf("PocketFlow Error %d: %s", int(e.Code), e.Message)
}

func (e *PFError) Unwrap() error {
	return e.wrappedErr
}

func (e *PFError) Is(target error) bool {
	t, ok := target.(*PFError)
	if !ok {
		return false
	}
	return t.Code != 0 && e.Code == t.Code
}

func newConflictingWorkflowError(workflowID, message string) *PFError {
	msg := fmt.Sprintf("Conflicting workflow invocation with the same ID (%s)", workflowID)
	if message != "" {
		msg += ": " + message
	}
	return &PFError{Message: msg, Code: ConflictingWorkflowError, WorkflowID: workflowID}
}

func newNonExistentWorkflowError(workflowID string) *PFError {
	return &PFError{Message: fmt.Sprintf("workflow %s does not exist", workflowID), Code: NonExistentWorkflowError, DestinationID: workflowID}
}

func newConflictingRegistrationError(name string) *PFError {
	return &PFError{Message: fmt.Sprintf("%s is already registered", name), Code: ConflictingRegistrationError}
}

func newAwaitedWorkflowCancelledError(workflowID string) *PFError {
	return &PFError{Message: fmt.Sprintf("Awaited workflow %s was cancelled", workflowID), Code: AwaitedWorkflowCancelled, WorkflowID: workflowID}
}

func newWorkflowCancelledError(workflowID string) *PFError {
	return &PFError{Message: fmt.Sprintf("Workflow %s was cancelled", workflowID), Code: WorkflowCancelled}
}

func newWorkflowConflictIDError(workflowID string) *PFError {
	return &PFError{Message: fmt.Sprintf("Conflicting workflow ID %s", workflowID), Code: ConflictingIDError, WorkflowID: workflowID}
}

func newDeadLetterQueueError(workflowID string, maxRetries int) *PFError {
	return &PFError{Message: fmt.Sprintf("Workflow %s has been moved to the dead-letter queue after exceeding the maximum of %d retries", workflowID, maxRetries), Code: DeadLetterQueueError, WorkflowID: workflowID, MaxRetries: maxRetries}
}

func newMaxStepRetriesExceededError(workflowID, stepName string, maxRetries int, err error) *PFError {
	return &PFError{Message: fmt.Sprintf("Step %s has exceeded its maximum of %d retries: %v", stepName, maxRetries, err), Code: MaxStepRetriesExceeded, WorkflowID: workflowID, StepName: stepName, MaxRetries: maxRetries, wrappedErr: err}
}

func newQueueDeduplicatedError(workflowID, queueName, deduplicationID string) *PFError {
	return &PFError{Message: fmt.Sprintf("Workflow %s was deduplicated due to an existing workflow in queue %s with deduplication ID %s", workflowID, queueName, deduplicationID), Code: QueueDeduplicated, WorkflowID: workflowID, QueueName: queueName, DeduplicationID: deduplicationID}
}

