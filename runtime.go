package pocketflow

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

// Runtime is the core durable execution runtime for PocketBase.
// Create with New(), register workflows before Launch(), then Shutdown() when done.
type Runtime struct {
	ctx           context.Context
	ctxCancelFunc context.CancelCauseFunc

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
	eb := newEventBus()

	sysDB := newSQLiteSysDB(app, eb)

	rt := &Runtime{
		ctx:                     baseCtx,
		ctxCancelFunc:           cancelFunc,
		app:                     app,
		systemDB:                sysDB,
		config:                  &config,
		applicationVersion:      config.ApplicationVersion,
		executorID:              config.ExecutorID,
		workflowsWg:            &sync.WaitGroup{},
		workflowRegistry:        &sync.Map{},
		workflowCustomNameToFQN: &sync.Map{},
		activeWorkflowIDs:       &sync.Map{},
	}

	rt.queueRunner = newQueueRunner()
	newWorkflowQueue(rt, _PF_INTERNAL_QUEUE_NAME)
	rt.scheduleManager = newScheduleManager()

	return rt
}

// Launch starts the runtime: ensures collections, launches sysdb, starts queue runner, and recovers pending workflows.
func (rt *Runtime) Launch() error {
	if rt.launched.Load() {
		return fmt.Errorf("pocketflow: runtime is already launched")
	}

	rt.applicationID = rt.app.Settings().Meta.AppName

	rt.systemDB.launch(rt.ctx)

	// Recover pending workflows before starting the queue runner
	// to avoid racing between recovery and dequeue
	handles, err := recoverPendingWorkflows(rt, []string{rt.executorID})
	if err != nil {
		return fmt.Errorf("pocketflow: failed to recover workflows: %w", err)
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
		if err := rt.app.Cron().Add("pocketflow_gc", rt.config.GCSchedule, func() {
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
			return fmt.Errorf("pocketflow: failed to register GC cron job: %w", err)
		}
	}

	rt.app.Logger().Info("pocketflow launched", "app_name", rt.applicationID, "executor_id", rt.executorID)
	return nil
}

// Shutdown gracefully stops the runtime.
func (rt *Runtime) Shutdown(timeout time.Duration) {
	rt.app.Logger().Debug("pocketflow shutting down")

	rt.ctxCancelFunc(errors.New("pocketflow shutdown"))

	// Wait for workflows
	done := make(chan struct{})
	go func() {
		rt.workflowsWg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		rt.app.Logger().Warn("timeout waiting for workflows to complete")
	}

	// Wait for queue runner
	if rt.queueRunner != nil && rt.launched.Load() {
		select {
		case <-rt.queueRunner.completionChan:
		case <-time.After(timeout):
			rt.app.Logger().Warn("timeout waiting for queue runner")
		}
	}

	if rt.systemDB != nil {
		rt.systemDB.shutdown(rt.ctx, timeout)
	}

	rt.launched.Store(false)
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
	Name         string `json:"name"`
	FQN          string `json:"fqn"`
	Triggerable  bool   `json:"triggerable"`
	CronSchedule string `json:"cronSchedule"`
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
		WithQueue(_PF_INTERNAL_QUEUE_NAME),
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

func (rt *Runtime) IsTriggerable(fqn string) bool {
	registeredAny, ok := rt.workflowRegistry.Load(fqn)
	if !ok {
		return false
	}
	return registeredAny.(workflowRegistryEntry).Triggerable
}
