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
	if config.AppName == "" {
		config.Logger.Warn("pbdbos: AppName is empty, using 'default'")
		config.AppName = "default"
	}
	if config.ApplicationVersion == "" {
		config.ApplicationVersion = computeApplicationVersion()
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

	if err := ensureCollections(rt.app); err != nil {
		return fmt.Errorf("pbdbos: failed to ensure collections: %w", err)
	}

	rt.systemDB.launch(rt.ctx)

	// Start the queue runner
	go rt.queueRunner.run(rt)
	rt.logger.Debug("queue runner started")

	// Recover pending workflows
	handles, err := recoverPendingWorkflows(rt, []string{rt.executorID})
	if err != nil {
		return fmt.Errorf("pbdbos: failed to recover workflows: %w", err)
	}
	if len(handles) > 0 {
		rt.logger.Info("recovered pending workflows", "count", len(handles))
	}

	rt.launched.Store(true)
	rt.logger.Info("pbdbos launched", "app_name", rt.config.AppName, "executor_id", rt.executorID)
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

// Accessors

func (rt *Runtime) GetApplicationVersion() string { return rt.applicationVersion }
func (rt *Runtime) GetExecutorID() string         { return rt.executorID }
func (rt *Runtime) GetApplicationID() string      { return rt.applicationID }
func (rt *Runtime) IsLaunched() bool              { return rt.launched.Load() }
