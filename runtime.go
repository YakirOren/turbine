package pbdbos

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// Runtime is the core DBOS-style durable execution runtime for PocketBase.
// Create with New(), register workflows before Launch(), then Shutdown() when done.
type Runtime struct {
	ctx           context.Context
	ctxCancelFunc context.CancelCauseFunc

	launched atomic.Bool

	app      core.App
	systemDB systemDatabase
	config   *Config
	logger   *slog.Logger

	// Queue runner
	queueRunner *queueRunner

	// Application metadata
	applicationVersion string
	applicationID      string
	executorID         string

	// Wait group for workflow goroutines
	workflowsWg *sync.WaitGroup

	// Workflow registry
	workflowRegistry        *sync.Map // map[string]WorkflowRegistryEntry
	workflowCustomNameToFQN *sync.Map // maps custom names → FQN

	// Set of workflow IDs currently running (key = workflow ID, value = struct{}{})
	activeWorkflowIDs *sync.Map
}

// New creates a new Runtime. Must be called before Launch().
func New(app core.App, config Config) *Runtime {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
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

	sysDB := newSQLiteSysDB(app, eb, config.Logger)

	rt := &Runtime{
		ctx:                     baseCtx,
		ctxCancelFunc:           cancelFunc,
		app:                     app,
		systemDB:                sysDB,
		config:                  &config,
		logger:                  config.Logger,
		applicationVersion:      config.ApplicationVersion,
		executorID:              config.ExecutorID,
		workflowsWg:            &sync.WaitGroup{},
		workflowRegistry:        &sync.Map{},
		workflowCustomNameToFQN: &sync.Map{},
		activeWorkflowIDs:       &sync.Map{},
	}

	rt.queueRunner = newQueueRunner(rt.logger)
	newWorkflowQueue(rt, _DBOS_INTERNAL_QUEUE_NAME)

	return rt
}

// Launch starts the runtime: ensures collections, launches sysdb, starts queue runner, and recovers pending workflows.
func (rt *Runtime) Launch() error {
	if rt.launched.Load() {
		return fmt.Errorf("pbdbos: runtime is already launched")
	}

	rt.applicationID = rt.app.Settings().Meta.AppName

	rt.systemDB.launch(rt.ctx)

	// Recover pending workflows before starting the queue runner
	// to avoid racing between recovery and dequeue
	handles, err := recoverPendingWorkflows(rt, []string{rt.executorID})
	if err != nil {
		return fmt.Errorf("pbdbos: failed to recover workflows: %w", err)
	}
	if len(handles) > 0 {
		rt.logger.Info("recovered pending workflows", "count", len(handles))
	}

	rt.launched.Store(true)

	// Start the queue runner after recovery is complete
	go rt.queueRunner.run(rt)
	rt.logger.Debug("queue runner started")

	// Register garbage collection cron job if retention is positive
	if rt.config.GCRetention > 0 {
		if err := rt.app.Cron().Add("pbdbos_gc", rt.config.GCSchedule, func() {
			if !rt.launched.Load() {
				return
			}
			cutoff := time.Now().Add(-rt.config.GCRetention)
			if err := rt.systemDB.garbageCollectWorkflows(rt.ctx, garbageCollectWorkflowsInput{
				cutoffTime: cutoff,
			}); err != nil {
				rt.logger.Error("workflow garbage collection failed", "error", err)
			}
		}); err != nil {
			return fmt.Errorf("pbdbos: failed to register GC cron job: %w", err)
		}
	}

	rt.logger.Info("pbdbos launched", "app_name", rt.applicationID, "executor_id", rt.executorID)
	return nil
}

// Shutdown gracefully stops the runtime.
func (rt *Runtime) Shutdown(timeout time.Duration) {
	rt.logger.Debug("pbdbos shutting down")

	rt.ctxCancelFunc(errors.New("pbdbos shutdown"))

	// Wait for workflows
	done := make(chan struct{})
	go func() {
		rt.workflowsWg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		rt.logger.Warn("timeout waiting for workflows to complete")
	}

	// Wait for queue runner
	if rt.queueRunner != nil && rt.launched.Load() {
		select {
		case <-rt.queueRunner.completionChan:
		case <-time.After(timeout):
			rt.logger.Warn("timeout waiting for queue runner")
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
