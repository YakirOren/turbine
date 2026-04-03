package turbine

import "fmt"

// PTErrorCode represents the different types of errors that can occur in Turbine operations.
type PTErrorCode int

const (
	ErrConflictingID           PTErrorCode = iota + 1
	_                                          // was InitializationError
	ErrWorkflowNotFound
	ErrWorkflowConflict
	ErrCancelled
	_                            // was UnexpectedStep
	ErrAwaitCancelled
	ErrRegistrationConflict
	_                            // was WorkflowUnexpectedTypeError
	_                            // was WorkflowExecutionError
	_                            // was StepExecutionError
	ErrDeadLetter
	ErrMaxRetries
	ErrDeduplicated
)

// PTError is the unified error type for all Turbine operations.
type PTError struct {
	Message         string
	Code            PTErrorCode
	WorkflowID      string
	DestinationID   string
	StepName        string
	QueueName       string
	DeduplicationID string
	MaxRetries      int
	wrappedErr      error
}

func (e *PTError) Error() string {
	return fmt.Sprintf("Turbine Error %d: %s", int(e.Code), e.Message)
}

func (e *PTError) Unwrap() error {
	return e.wrappedErr
}

func (e *PTError) Is(target error) bool {
	t, ok := target.(*PTError)
	if !ok {
		return false
	}
	return t.Code != 0 && e.Code == t.Code
}

func newErrWorkflowConflict(workflowID, message string) *PTError {
	msg := fmt.Sprintf("Conflicting workflow invocation with the same ID (%s)", workflowID)
	if message != "" {
		msg += ": " + message
	}
	return &PTError{Message: msg, Code: ErrWorkflowConflict, WorkflowID: workflowID}
}

func newErrWorkflowNotFound(workflowID string) *PTError {
	return &PTError{Message: fmt.Sprintf("workflow %s does not exist", workflowID), Code: ErrWorkflowNotFound, DestinationID: workflowID}
}

func newErrRegistrationConflict(name string) *PTError {
	return &PTError{Message: fmt.Sprintf("%s is already registered", name), Code: ErrRegistrationConflict}
}

func newErrAwaitCancelledError(workflowID string) *PTError {
	return &PTError{Message: fmt.Sprintf("Awaited workflow %s was cancelled", workflowID), Code: ErrAwaitCancelled, WorkflowID: workflowID}
}

func newErrCancelledError(workflowID string) *PTError {
	return &PTError{Message: fmt.Sprintf("Workflow %s was cancelled", workflowID), Code: ErrCancelled}
}

func newWorkflowConflictIDError(workflowID string) *PTError {
	return &PTError{Message: fmt.Sprintf("Conflicting workflow ID %s", workflowID), Code: ErrConflictingID, WorkflowID: workflowID}
}

func newErrDeadLetter(workflowID string, maxRetries int) *PTError {
	return &PTError{Message: fmt.Sprintf("Workflow %s has been moved to the dead-letter queue after exceeding the maximum of %d retries", workflowID, maxRetries), Code: ErrDeadLetter, WorkflowID: workflowID, MaxRetries: maxRetries}
}

func newErrMaxRetriesError(workflowID, stepName string, maxRetries int, err error) *PTError {
	return &PTError{Message: fmt.Sprintf("Step %s has exceeded its maximum of %d retries: %v", stepName, maxRetries, err), Code: ErrMaxRetries, WorkflowID: workflowID, StepName: stepName, MaxRetries: maxRetries, wrappedErr: err}
}

func newErrDeduplicatedError(workflowID, queueName, deduplicationID string) *PTError {
	return &PTError{Message: fmt.Sprintf("Workflow %s was deduplicated due to an existing workflow in queue %s with deduplication ID %s", workflowID, queueName, deduplicationID), Code: ErrDeduplicated, WorkflowID: workflowID, QueueName: queueName, DeduplicationID: deduplicationID}
}

