package turbine

import "fmt"

// ErrorCode represents the different types of errors that can occur in Turbine operations.
type ErrorCode int

const (
	ErrConflictingID ErrorCode = iota + 1
	_                          // was InitializationError
	ErrWorkflowNotFound
	ErrWorkflowConflict
	ErrCancelled
	_ // was UnexpectedStep
	ErrAwaitCancelled
	ErrRegistrationConflict
	_ // was WorkflowUnexpectedTypeError
	_ // was WorkflowExecutionError
	_ // was StepExecutionError
	ErrDeadLetter
	ErrMaxRetries
	ErrDeduplicated
)

// Error is the unified error type for all Turbine operations.
type Error struct {
	Message         string
	Code            ErrorCode
	WorkflowID      string
	StepName        string
	QueueName       string
	DeduplicationID string
	MaxRetries      int
	wrappedErr      error
}

func (e *Error) Error() string {
	return fmt.Sprintf("Turbine Error %d: %s", int(e.Code), e.Message)
}

func (e *Error) Unwrap() error {
	return e.wrappedErr
}

func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return t.Code != 0 && e.Code == t.Code
}

func newErrWorkflowConflict(workflowID, message string) *Error {
	msg := fmt.Sprintf("Conflicting workflow invocation with the same ID (%s)", workflowID)
	if message != "" {
		msg += ": " + message
	}
	return &Error{Message: msg, Code: ErrWorkflowConflict, WorkflowID: workflowID}
}

func newErrWorkflowNotFound(workflowID string) *Error {
	return &Error{Message: fmt.Sprintf("workflow %s does not exist", workflowID), Code: ErrWorkflowNotFound, WorkflowID: workflowID}
}

func newErrRegistrationConflict(name string) *Error {
	return &Error{Message: fmt.Sprintf("%s is already registered", name), Code: ErrRegistrationConflict}
}

func newErrAwaitCancelled(workflowID string) *Error {
	return &Error{Message: fmt.Sprintf("Awaited workflow %s was cancelled", workflowID), Code: ErrAwaitCancelled, WorkflowID: workflowID}
}

func newErrCancelled(workflowID string) *Error {
	return &Error{Message: fmt.Sprintf("Workflow %s was cancelled", workflowID), Code: ErrCancelled}
}

func newErrDeadLetter(workflowID string, maxRetries int) *Error {
	return &Error{Message: fmt.Sprintf("Workflow %s has been moved to the dead-letter queue after exceeding the maximum of %d retries", workflowID, maxRetries), Code: ErrDeadLetter, WorkflowID: workflowID, MaxRetries: maxRetries}
}

func newErrMaxRetries(workflowID, stepName string, maxRetries int, err error) *Error {
	return &Error{Message: fmt.Sprintf("Step %s has exceeded its maximum of %d retries: %v", stepName, maxRetries, err), Code: ErrMaxRetries, WorkflowID: workflowID, StepName: stepName, MaxRetries: maxRetries, wrappedErr: err}
}

func newErrDeduplicated(workflowID, queueName, deduplicationID string) *Error {
	return &Error{Message: fmt.Sprintf("Workflow %s was deduplicated due to an existing workflow in queue %s with deduplication ID %s", workflowID, queueName, deduplicationID), Code: ErrDeduplicated, WorkflowID: workflowID, QueueName: queueName, DeduplicationID: deduplicationID}
}
