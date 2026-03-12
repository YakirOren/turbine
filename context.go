package pocketflow

import (
	"context"
	"fmt"
	"log/slog"
)

type runtimeContextKey struct{}

// Context is the unified context for pocketflow workflows.
// It embeds context.Context and provides access to the runtime's logger and workflow ID.
type Context interface {
	context.Context
	Logger() *slog.Logger
	WorkflowID() (string, error)
}

// pfContext is the private implementation of Context.
type pfContext struct {
	context.Context
	runtime *Runtime
}

func (c *pfContext) Logger() *slog.Logger {
	return c.runtime.logger
}

func (c *pfContext) WorkflowID() (string, error) {
	wfState, ok := c.Context.Value(workflowStateKey).(*workflowState)
	if !ok || wfState == nil {
		return "", fmt.Errorf("not within a workflow")
	}
	return wfState.workflowID, nil
}

// Value overrides context.Context.Value so the runtime is discoverable
// even after the context is wrapped by WithTimeout, WithDeadline, etc.
func (c *pfContext) Value(key any) any {
	if _, ok := key.(runtimeContextKey); ok {
		return c.runtime
	}
	return c.Context.Value(key)
}

// runtimeFromContext extracts the *Runtime from a Context.
func runtimeFromContext(ctx context.Context) *Runtime {
	rt, _ := ctx.Value(runtimeContextKey{}).(*Runtime)
	return rt
}

// NewContext wraps a standard context.Context with the runtime.
// Use this from HTTP handlers or other external callers.
func (rt *Runtime) NewContext(ctx context.Context) Context {
	return &pfContext{Context: ctx, runtime: rt}
}

// FromContext extracts a pocketflow.Context from a plain context.Context.
// Returns false if no runtime is present.
func FromContext(ctx context.Context) (Context, bool) {
	rt := runtimeFromContext(ctx)
	if rt == nil {
		return nil, false
	}
	return &pfContext{Context: ctx, runtime: rt}, true
}
