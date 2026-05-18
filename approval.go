package turbine

import (
	"errors"
	"time"
)

// ApprovalResult holds the decision from a human approver.
type ApprovalResult struct {
	Approved bool   `json:"approved"`
	Comment  string `json:"comment,omitempty"`
}

const approvalTopic = "pt.approval"

// maxApprovalWait is used when no timeout is specified. Recv requires a positive
// duration (zero means "don't wait"), so we use a very large value to simulate
// indefinite blocking.
const maxApprovalWait = 100 * 365 * 24 * time.Hour

// ErrApprovalTimeout is returned when WaitForApproval times out.
var ErrApprovalTimeout = errors.New("turbine: approval timed out")

type approvalOptions struct {
	timeout time.Duration
}

// ApprovalOption configures WaitForApproval behavior.
type ApprovalOption func(*approvalOptions)

// WithApprovalTimeout sets a timeout for how long to wait for approval.
// If zero (default), waits indefinitely.
func WithApprovalTimeout(d time.Duration) ApprovalOption {
	return func(o *approvalOptions) { o.timeout = d }
}

// WaitForApproval pauses the workflow and waits for a human decision.
// The workflow's app status is set to "waiting for approval" (yellow) while blocked.
// Use Send(ctx, workflowID, ApprovalResult{...}, "pt.approval") or the
// POST /api/pt/workflows/{id}/approve endpoint to deliver the decision.
func WaitForApproval(ctx Context, opts ...ApprovalOption) (ApprovalResult, error) {
	o := approvalOptions{}
	for _, opt := range opts {
		opt(&o)
	}

	timeout := o.timeout
	if timeout <= 0 {
		timeout = maxApprovalWait
	}

	ctx.SetAppStatus("waiting for approval", "yellow")

	rt := runtimeFromContext(ctx)
	wfState, _ := ctx.Value(workflowStateKey).(*workflowState)
	if rt != nil && wfState != nil && !wfState.recovering {
		go rt.dispatchEvent(wfState.workflowID, wfState.workflowName, StatusWaitingForApproval, nil, nil)
	}

	result, err := Recv[*ApprovalResult](ctx, approvalTopic, timeout)
	if err != nil {
		return ApprovalResult{}, err
	}

	if result == nil {
		return ApprovalResult{}, ErrApprovalTimeout
	}

	return *result, nil
}
