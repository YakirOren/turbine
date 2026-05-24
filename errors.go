package turbine

import (
	"github.com/YakirOren/turbine/internal/turberror"
)

// ErrorCode represents the different types of errors that can occur in Turbine operations.
//
// Underlying type lives in internal/turberror; aliased here for public API stability.
type ErrorCode = turberror.ErrorCode

const (
	ErrConflictingID        = turberror.ErrConflictingID
	ErrWorkflowNotFound     = turberror.ErrWorkflowNotFound
	ErrWorkflowConflict     = turberror.ErrWorkflowConflict
	ErrCancelled            = turberror.ErrCancelled
	ErrAwaitCancelled       = turberror.ErrAwaitCancelled
	ErrRegistrationConflict = turberror.ErrRegistrationConflict
	ErrDeadLetter           = turberror.ErrDeadLetter
	ErrMaxRetries           = turberror.ErrMaxRetries
	ErrDeduplicated         = turberror.ErrDeduplicated
)

// Error is the unified error type for all Turbine operations.
//
// Underlying type lives in internal/turberror; aliased here for public API stability.
type Error = turberror.Error

// Unexported constructor aliases keep existing call sites at root unchanged.
var (
	newErrWorkflowConflict     = turberror.NewErrWorkflowConflict
	newErrWorkflowNotFound     = turberror.NewErrWorkflowNotFound
	newErrRegistrationConflict = turberror.NewErrRegistrationConflict
	newErrAwaitCancelled       = turberror.NewErrAwaitCancelled
	newErrCancelled            = turberror.NewErrCancelled
	newErrDeadLetter           = turberror.NewErrDeadLetter
	newErrMaxRetries           = turberror.NewErrMaxRetries
	newErrDeduplicated         = turberror.NewErrDeduplicated
)
