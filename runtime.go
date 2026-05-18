package turbine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// ErrShuttingDown is returned when a workflow is rejected because the runtime is draining.
var ErrShuttingDown = errors.New("turbine: runtime is shutting down")

// Runtime is the core durable execution runtime.
// Create with NewRuntime, register workflows before Launch, then Shutdown when done.
// For higher-level entry points see New (standalone) and NewApp (HTTP-serving).
type Runtime struct {
	ctx           context.Context
	ctxCancelFunc context.CancelCauseFunc

	draining        atomic.Bool
	drainCtx        context.Context
	drainCancelFunc context.CancelFunc

	launched atomic.Bool

	app core.App
	// ownedApp is non-nil when a constructor owns the PocketBase app lifecycle
	// (currently: NewStandalone). When set, Launch calls Bootstrap and Shutdown
	// calls ResetBootstrapState.
	ownedApp core.App
	systemDB systemDatabase
	config   *Config
	logger   *slog.Logger // overrides app.Logger() for workflow + step logs when non-nil

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
	workflowCustomNameToFQN *sync.Map // maps custom names -> FQN

	// Set of workflow IDs currently running (key = workflow ID, value = struct{}{})
	activeWorkflowIDs *sync.Map

	scheduleManager *scheduleManager

	// Disabled compile-time schedules (key = schedule name, value = struct{}{})
	disabledSchedules *sync.Map

	// Cached dispatch targets, reloaded via collection hooks
	webhookCache      atomic.Value // []cachedWebhook
	alertChannelCache atomic.Value // []cachedAlertChannel

	productSender ProductSender
}

// NewRuntime creates a new Runtime. Must be called before Launch().
func NewRuntime(app core.App, config Config) *Runtime {
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
	if config.WebhookMaxRetries == 0 {
		config.WebhookMaxRetries = 3
	}
	if config.WebhookTimeout == 0 {
		config.WebhookTimeout = 10 * time.Second
	}
	if config.ShutdownTimeout == 0 {
		config.ShutdownTimeout = 30 * time.Second
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
		logger:                  config.Logger,
		applicationVersion:      config.ApplicationVersion,
		executorID:              config.ExecutorID,
		workflowsWg:             &sync.WaitGroup{},
		workflowRegistry:        &sync.Map{},
		workflowCustomNameToFQN: &sync.Map{},
		activeWorkflowIDs:       &sync.Map{},
		disabledSchedules:       &sync.Map{},
		productSender:           config.ProductSender,
	}

	rt.queueRunner = newQueueRunner()
	newWorkflowQueue(rt, ptInternalQueueName)
	rt.scheduleManager = newScheduleManager()

	return rt
}

// Launch starts the runtime: ensures collections, launches sysdb, starts queue runner, and recovers pending workflows.
func (rt *Runtime) Launch() error {
	if rt.launched.Load() {
		return fmt.Errorf("turbine: runtime is already launched")
	}

	// Standalone runtimes (created by a constructor that owns the PocketBase
	// app, like NewStandalone) must bootstrap the app before any DB access.
	if rt.ownedApp != nil {
		if err := rt.ownedApp.Bootstrap(); err != nil {
			return fmt.Errorf("turbine: bootstrap failed: %w", err)
		}
	}

	// Always run app migrations. For standalone runtimes this materializes
	// the pt_* tables; for HTTP runtimes app.Start() has already run them
	// and the runner is a no-op via PocketBase's _migrations table.
	if err := rt.app.RunAppMigrations(); err != nil {
		return fmt.Errorf("turbine: migrations failed: %w", err)
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

	// Sync compile-time schedules to pt_schedules and load disabled state.
	if err := rt.syncCompileTimeSchedules(); err != nil {
		rt.app.Logger().Error("failed to sync compile-time schedules", "error", err)
	}

	// Sync registered workflows to pt_workflows collection.
	if err := rt.syncRegisteredWorkflows(); err != nil {
		rt.app.Logger().Error("failed to sync registered workflows", "error", err)
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

	// Cache dispatch targets and register hooks for invalidation
	rt.reloadDispatchCaches()
	rt.registerDispatchHooks()

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
// 1. Drain, stop accepting new work, let running workflows finish naturally
// 2. Force, if cfg.ShutdownTimeout expires, cancel root context to kill remaining workflows
// Idempotent. Drain progress is logged directly to stdout, not via app.Logger,
// since the app's logger pipeline may itself be shutting down.
func (rt *Runtime) Shutdown() {
	if !rt.launched.Load() {
		if rt.ownedApp != nil {
			_ = rt.ownedApp.ResetBootstrapState()
		}
		return
	}

	shutdownLog := slog.New(slog.NewTextHandler(os.Stdout, nil))
	timeout := rt.config.ShutdownTimeout
	shutdownLog.Info("turbine shutting down")

	rt.draining.Store(true)
	rt.launched.Store(false)
	rt.drainCancelFunc()

	if rt.queueRunner != nil {
		select {
		case <-rt.queueRunner.completionChan:
		case <-time.After(timeout):
			shutdownLog.Warn("timeout waiting for queue runner to drain")
		}
	}

	done := make(chan struct{})
	go func() {
		rt.workflowsWg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		shutdownLog.Warn("timeout waiting for workflows, force-cancelling")
		rt.ctxCancelFunc(errors.New("turbine shutdown timeout"))
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}

	if rt.systemDB != nil {
		rt.systemDB.shutdown(rt.ctx, timeout)
	}

	if rt.ownedApp != nil {
		_ = rt.ownedApp.ResetBootstrapState()
	}
}

// GarbageCollect removes completed workflows older than the configured retention period.
func (rt *Runtime) GarbageCollect() error {
	cutoff := time.Now().Add(-rt.config.GCRetention)
	return rt.systemDB.garbageCollectWorkflows(rt.ctx, garbageCollectWorkflowsInput{cutoffTime: cutoff})
}

// baseLogger returns the runtime's configured logger, falling back to the app logger.
func (rt *Runtime) baseLogger() *slog.Logger {
	if rt.logger != nil {
		return rt.logger
	}
	return rt.app.Logger()
}

// App returns the underlying PocketBase app.
func (rt *Runtime) App() core.App { return rt.app }

// Queues returns the workflow queues registered on the runtime.
func (rt *Runtime) Queues() []WorkflowQueue { return rt.queueRunner.listQueues() }

// SetProductSender sets the product sender after construction.
// Use this when the sender needs a reference to the runtime (e.g., WorkflowSender).
func (rt *Runtime) SetProductSender(sender ProductSender) {
	rt.productSender = sender
}

// syncCompileTimeSchedules ensures every compile-time scheduled workflow has a
// corresponding record in pt_schedules (type = scheduleTypeCompile). This lets the UI
// and toggle API treat all schedules uniformly.
func (rt *Runtime) syncCompileTimeSchedules() error {
	rt.workflowRegistry.Range(func(_, value any) bool {
		entry, ok := value.(workflowRegistryEntry)
		if !ok || entry.CronSchedule == "" {
			return true
		}
		fqn := entry.FQN

		existing, err := findCompileScheduleByFQN(rt.app, fqn)
		if err == nil {
			// Update cron expression in case it changed in code.
			if existing.CronExpression() != entry.CronSchedule {
				existing.SetCronExpression(entry.CronSchedule)
				_ = rt.app.Save(existing)
			}
			// Load disabled state into memory.
			if !existing.Enabled() {
				rt.disabledSchedules.Store(fqn, struct{}{})
			}
			return true
		}

		// Create a new record.
		_, _ = createSchedule(rt.app, fqn, nil, scheduleTypeCompile, entry.CronSchedule, time.Time{})
		return true
	})
	return nil
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

// syncRegisteredWorkflows upserts every registered workflow into the pt_workflows
// collection so the standard PocketBase API can serve them.
func (rt *Runtime) syncRegisteredWorkflows() error {
	rt.workflowRegistry.Range(func(_, value any) bool {
		entry, ok := value.(workflowRegistryEntry)
		if !ok {
			return true
		}
		fqn := entry.FQN
		name := entry.Name
		if name == "" {
			name = fqn
		}

		// Try to find existing record by FQN.
		existing, _ := rt.app.FindFirstRecordByFilter(collectionWorkflows, "fqn = {:fqn}", dbx.Params{"fqn": fqn})
		var rec *core.Record
		if existing != nil {
			rec = existing
		} else {
			col, err := rt.app.FindCollectionByNameOrId(collectionWorkflows)
			if err != nil {
				rt.app.Logger().Error("failed to find pt_workflows collection", "error", err)
				return true
			}
			rec = core.NewRecord(col)
		}

		rec.Set("name", name)
		rec.Set("fqn", fqn)
		rec.Set("triggerable", entry.Triggerable)
		rec.Set("cron_schedule", entry.CronSchedule)
		rec.Set("input_schema", entry.InputSchema)
		rec.Set("tags", entry.Tags)

		if err := rt.app.Save(rec); err != nil {
			rt.app.Logger().Error("failed to sync workflow to pt_workflows", "fqn", fqn, "error", err)
		}
		return true
	})

	// Remove records for workflows that are no longer registered.
	existing, err := rt.app.FindAllRecords(collectionWorkflows)
	if err != nil {
		return err
	}
	for _, rec := range existing {
		if _, ok := rt.workflowRegistry.Load(rec.GetString("fqn")); !ok {
			if err := rt.app.Delete(rec); err != nil {
				rt.app.Logger().Error("failed to delete stale workflow", "source", "system", "fqn", rec.GetString("fqn"), "error", err)
			}
		}
	}
	return nil
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
		WithQueue(ptInternalQueueName),
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
