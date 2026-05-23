package turbine

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pocketbase/pocketbase/core"
)

var workflowIDLogKey = "workflow_id"

type runtimeContextKey struct{}

// Context is the unified context for turbine workflows.
// It embeds context.Context and provides access to the app, logger, and workflow ID.
type Context interface {
	context.Context
	App() core.App
	Logger() *slog.Logger
	WorkflowID() (string, error)
	SetAppStatus(label, color string)
}

// ptContext is the private implementation of Context.
type ptContext struct {
	context.Context
	runtime *Runtime
}

func (c *ptContext) App() core.App {
	return c.runtime.app
}

func (c *ptContext) Logger() *slog.Logger {
	logger := c.runtime.baseLogger()
	wfState, ok := c.Context.Value(workflowStateKey).(*workflowState)
	if ok && wfState != nil {
		logger = logger.With(workflowIDLogKey, wfState.workflowID)
		if wfState.isWithinStep {
			logger = logger.With("step_id", wfState.stepID.Load())
		}
	}
	return logger
}

func (c *ptContext) WorkflowID() (string, error) {
	wfState, ok := c.Context.Value(workflowStateKey).(*workflowState)
	if !ok || wfState == nil {
		return "", fmt.Errorf("not within a workflow")
	}
	return wfState.workflowID, nil
}

func (c *ptContext) SetAppStatus(label, color string) {
	if err := setAppStatus(c.Context, c.runtime, label, color); err != nil {
		c.runtime.app.Logger().Error("SetAppStatus failed", "error", err, "source", "system")
	}
}

// Value overrides context.Context.Value so the runtime is discoverable
// even after the context is wrapped by WithTimeout, WithDeadline, etc.
func (c *ptContext) Value(key any) any {
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
	return &ptContext{Context: ctx, runtime: rt}
}

// FromContext extracts a turbine.Context from a plain context.Context.
// Returns false if no runtime is present.
func FromContext(ctx context.Context) (Context, bool) {
	rt := runtimeFromContext(ctx)
	if rt == nil {
		return nil, false
	}
	return &ptContext{Context: ctx, runtime: rt}, true
}

// SetAppStatus sets a user-defined application status on the current workflow.
// It can be called from a step's context.Context (same pattern as LoggerFrom/AppFrom).
func SetAppStatus(ctx context.Context, label, color string) error {
	rt := runtimeFromContext(ctx)
	if rt == nil {
		return fmt.Errorf("not within a turbine context")
	}
	return setAppStatus(ctx, rt, label, color)
}

func setAppStatus(ctx context.Context, rt *Runtime, label, color string) error {
	if label == "" {
		return fmt.Errorf("app status label must not be empty")
	}
	if err := validateAppStatusColor(color); err != nil {
		return err
	}
	wfState, ok := ctx.Value(workflowStateKey).(*workflowState)
	if !ok || wfState == nil {
		return fmt.Errorf("not within a workflow")
	}
	if wfState.recovering || (wfState.appStatus == label && wfState.appStatusColor == color) {
		return nil
	}
	err := rt.workflows.updateAppStatus(ctx, updateAppStatusDBInput{
		workflowID:     wfState.workflowID,
		appStatus:      label,
		appStatusColor: color,
	})
	if err != nil {
		return err
	}
	wfState.appStatus = label
	wfState.appStatusColor = color
	rt.app.Logger().Info("app status changed", "workflow_id", wfState.workflowID, "app_status", label, "app_status_color", color, "source", "system")
	return nil
}

// SendNotification sends a custom message to the named alert channel.
// Usable from inside a step via the step's context.Context, mirroring
// the SetAppStatus/LoggerFrom/AppFrom pattern.
func SendNotification(ctx context.Context, name, message string) error {
	rt := runtimeFromContext(ctx)
	if rt == nil {
		return fmt.Errorf("not within a turbine context")
	}
	return rt.SendNotification(name, message)
}

// LoggerFrom returns a logger from a step's context.Context.
// The logger includes workflow_id and step_id attributes automatically.
// Returns a no-op logger if called outside a turbine context.
func LoggerFrom(ctx context.Context) *slog.Logger {
	rt := runtimeFromContext(ctx)
	if rt == nil {
		return slog.Default()
	}
	logger := rt.baseLogger()
	wfState, ok := ctx.Value(workflowStateKey).(*workflowState)
	if ok && wfState != nil {
		logger = logger.With(workflowIDLogKey, wfState.workflowID)
		if wfState.isWithinStep {
			logger = logger.With("step_id", wfState.stepID.Load())
		}
	}
	return logger
}

// AppFrom returns the app from a step's context.Context.
// Returns nil if called outside a turbine context.
func AppFrom(ctx context.Context) core.App {
	rt := runtimeFromContext(ctx)
	if rt == nil {
		return nil
	}
	return rt.app
}
