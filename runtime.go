package turbine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// ErrShuttingDown is returned when a workflow is rejected because the runtime is draining.
var ErrShuttingDown = errors.New("turbine: runtime is shutting down")

// Runtime is the core durable execution runtime for PocketBase.
// Create with New(), register workflows before Launch(), then Shutdown() when done.
type Runtime struct {
	ctx           context.Context
	ctxCancelFunc context.CancelCauseFunc

	draining        atomic.Bool
	drainCtx        context.Context
	drainCancelFunc context.CancelFunc

	launched atomic.Bool

	app      core.App
	systemDB systemDatabase
	config   *Config

	// Queue runner
	queueRunner *queueRunner

	// Application metadata
	applicationVersion string
	applicationID      string
	executorID         string

	// Wait group for workflow goroutines
	workflowsWg *sync.WaitGroup

	// Workflow registry
	workflowRegistry        *sync.Map // map[string]workflowRegistryEntry
	workflowCustomNameToFQN *sync.Map // maps custom names → FQN

	// Set of workflow IDs currently running (key = workflow ID, value = struct{}{})
	activeWorkflowIDs *sync.Map

	scheduleManager *scheduleManager

	productSender ProductSender
}

// New creates a new Runtime. Must be called before Launch().
func New(app core.App, config Config) *Runtime {
	if config.ExecutorID == "" {
		config.ExecutorID = "local"
	}
	if config.ApplicationVersion == "" {
		config.ApplicationVersion = computeApplicationVersion()
	}
	if config.GCRetention == 0 {
		config.GCRetention = 72 * time.Hour
	}
	if config.GCSchedule == "" {
		config.GCSchedule = "0 0 * * *"
	}

	baseCtx, cancelFunc := context.WithCancelCause(context.Background())
	drainCtx, drainCancel := context.WithCancel(baseCtx)
	eb := newEventBus()

	sysDB := newSQLiteSysDB(app, eb)

	rt := &Runtime{
		ctx:                     baseCtx,
		ctxCancelFunc:           cancelFunc,
		drainCtx:                drainCtx,
		drainCancelFunc:         drainCancel,
		app:                     app,
		systemDB:                sysDB,
		config:                  &config,
		applicationVersion:      config.ApplicationVersion,
		executorID:              config.ExecutorID,
		workflowsWg:            &sync.WaitGroup{},
		workflowRegistry:        &sync.Map{},
		workflowCustomNameToFQN: &sync.Map{},
		activeWorkflowIDs:       &sync.Map{},
		productSender:           config.ProductSender,
	}

	rt.queueRunner = newQueueRunner()
	newWorkflowQueue(rt, _PT_INTERNAL_QUEUE_NAME)
	rt.scheduleManager = newScheduleManager()

	return rt
}

// Launch starts the runtime: ensures collections, launches sysdb, starts queue runner, and recovers pending workflows.
func (rt *Runtime) Launch() error {
	if rt.launched.Load() {
		return fmt.Errorf("turbine: runtime is already launched")
	}

	rt.applicationID = rt.app.Settings().Meta.AppName

	rt.systemDB.launch(rt.ctx)

	// Recover pending workflows before starting the queue runner
	// to avoid racing between recovery and dequeue
	handles, err := recoverPendingWorkflows(rt, []string{rt.executorID})
	if err != nil {
		return fmt.Errorf("turbine: failed to recover workflows: %w", err)
	}
	if len(handles) > 0 {
		rt.app.Logger().Info("recovered pending workflows", "count", len(handles))
	}

	rt.launched.Store(true)

	// Start the queue runner after recovery is complete
	go rt.queueRunner.run(rt)
	rt.app.Logger().Debug("queue runner started")

	// Register schedule hooks (must happen before loading to avoid race)
	rt.scheduleManager.registerHooks(rt)

	// Load existing schedules from database
	if err := rt.scheduleManager.loadExisting(rt); err != nil {
		rt.app.Logger().Error("failed to load schedules", "error", err)
	}

	// Register garbage collection cron job if retention is positive
	if rt.config.GCRetention > 0 {
		if err := rt.app.Cron().Add("turbine_gc", rt.config.GCSchedule, func() {
			if !rt.launched.Load() {
				return
			}
			cutoff := time.Now().Add(-rt.config.GCRetention)
			if err := rt.systemDB.garbageCollectWorkflows(rt.ctx, garbageCollectWorkflowsInput{
				cutoffTime: cutoff,
			}); err != nil {
				rt.app.Logger().Error("workflow garbage collection failed", "error", err)
			}
		}); err != nil {
			return fmt.Errorf("turbine: failed to register GC cron job: %w", err)
		}
	}

	rt.app.Logger().Info("turbine launched", "app_name", rt.applicationID, "executor_id", rt.executorID)
	return nil
}

// IsDraining returns true if the runtime is in the process of shutting down.
func (rt *Runtime) IsDraining() bool { return rt.draining.Load() }

// Shutdown gracefully stops the runtime with a two-phase approach:
// 1. Drain — stop accepting new work, let running workflows finish naturally
// 2. Force — if timeout expires, cancel root context to kill remaining workflows
func (rt *Runtime) Shutdown(timeout time.Duration) {
	rt.app.Logger().Info("turbine shutting down")

	// Phase 1: Drain — stop accepting new work
	rt.draining.Store(true)
	rt.launched.Store(false)
	rt.drainCancelFunc() // unblock queue runners and schedule timers

	// Wait for queue runners to exit
	if rt.queueRunner != nil {
		select {
		case <-rt.queueRunner.completionChan:
		case <-time.After(timeout):
			rt.app.Logger().Warn("timeout waiting for queue runner to drain")
		}
	}

	// Wait for in-flight workflows
	done := make(chan struct{})
	go func() {
		rt.workflowsWg.Wait()
		close(done)
	}()
	select {
	case <-done:
		// all workflows completed gracefully
	case <-time.After(timeout):
		// Phase 2: Force cancel
		rt.app.Logger().Warn("timeout waiting for workflows, force-cancelling")
		rt.ctxCancelFunc(errors.New("turbine shutdown timeout"))
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}

	if rt.systemDB != nil {
		rt.systemDB.shutdown(rt.ctx, timeout)
	}
}

// GarbageCollect removes completed workflows older than the configured retention period.
func (rt *Runtime) GarbageCollect() error {
	cutoff := time.Now().Add(-rt.config.GCRetention)
	return rt.systemDB.garbageCollectWorkflows(rt.ctx, garbageCollectWorkflowsInput{cutoffTime: cutoff})
}

// Accessors

func (rt *Runtime) GetApplicationVersion() string { return rt.applicationVersion }
func (rt *Runtime) GetExecutorID() string         { return rt.executorID }
func (rt *Runtime) GetApplicationID() string      { return rt.applicationID }
func (rt *Runtime) IsLaunched() bool              { return rt.launched.Load() }
func (rt *Runtime) App() core.App                { return rt.app }
func (rt *Runtime) Queues() []WorkflowQueue      { return rt.queueRunner.listQueues() }

// SetProductSender sets the product sender after construction.
// Use this when the sender needs a reference to the runtime (e.g., WorkflowSender).
func (rt *Runtime) SetProductSender(sender ProductSender) {
	rt.productSender = sender
}

// ScheduledWorkflow describes a registered scheduled workflow.
type ScheduledWorkflow struct {
	Name         string `json:"name"`
	FQN          string `json:"fqn"`
	CronSchedule string `json:"cronSchedule"`
}

// ScheduledWorkflows returns all workflows registered with a cron schedule.
func (rt *Runtime) ScheduledWorkflows() []ScheduledWorkflow {
	var result []ScheduledWorkflow
	rt.workflowRegistry.Range(func(_, value any) bool {
		entry, ok := value.(workflowRegistryEntry)
		if ok && entry.CronSchedule != "" {
			name := entry.Name
			if name == "" {
				name = entry.FQN
			}
			result = append(result, ScheduledWorkflow{
				Name:         name,
				FQN:          entry.FQN,
				CronSchedule: entry.CronSchedule,
			})
		}
		return true
	})
	return result
}

type RegisteredWorkflow struct {
	Name         string         `json:"name"`
	FQN          string         `json:"fqn"`
	Triggerable  bool           `json:"triggerable"`
	CronSchedule string         `json:"cronSchedule"`
	InputSchema  map[string]any `json:"inputSchema,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
}

func (rt *Runtime) RegisteredWorkflows() []RegisteredWorkflow {
	var result []RegisteredWorkflow
	rt.workflowRegistry.Range(func(_, value any) bool {
		entry, ok := value.(workflowRegistryEntry)
		if !ok {
			return true
		}
		name := entry.Name
		if name == "" {
			name = entry.FQN
		}
		result = append(result, RegisteredWorkflow{
			Name:         name,
			FQN:          entry.FQN,
			Triggerable:  entry.Triggerable,
			CronSchedule: entry.CronSchedule,
			InputSchema:  entry.InputSchema,
			Tags:         entry.Tags,
		})
		return true
	})
	return result
}

func (rt *Runtime) triggerByFQNWithOpts(fqn string, rawInput json.RawMessage, extraOpts ...WorkflowOption) (string, error) {
	registeredAny, ok := rt.workflowRegistry.Load(fqn)
	if !ok {
		return "", fmt.Errorf("workflow not found: %s", fqn)
	}
	entry := registeredAny.(workflowRegistryEntry)
	if !entry.Triggerable {
		return "", fmt.Errorf("workflow is not triggerable: %s", fqn)
	}

	encoded := string(rawInput)
	opts := append([]WorkflowOption{
		withAlreadyEncodedInput(),
		WithQueue(_PT_INTERNAL_QUEUE_NAME),
	}, extraOpts...)

	handle, err := entry.wrappedFunction(rt, &encoded, opts...)
	if err != nil {
		return "", fmt.Errorf("failed to trigger workflow: %w", err)
	}
	return handle.GetWorkflowID(), nil
}

func (rt *Runtime) TriggerByFQN(fqn string, rawInput json.RawMessage) (string, error) {
	return rt.triggerByFQNWithOpts(fqn, rawInput)
}

// SendToWorkflow sends a message to a workflow from outside a workflow context.
// This is used by HTTP handlers and other external callers.
func (rt *Runtime) SendToWorkflow(workflowID string, message any, topic string) error {
	ctx := rt.NewContext(rt.ctx)
	return Send(ctx, workflowID, message, topic)
}

func (rt *Runtime) IsTriggerable(fqn string) bool {
	registeredAny, ok := rt.workflowRegistry.Load(fqn)
	if !ok {
		return false
	}
	return registeredAny.(workflowRegistryEntry).Triggerable
}
