package turbine

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

type workflowStateKeyType struct{}

var workflowStateKey = workflowStateKeyType{}

// Workflow is a type-safe workflow function.
type Workflow[P any, R any] func(ctx Context, input P) (R, error)

// workflowFunc is a type-erased workflow function used internally.
type workflowFunc func(ctx Context, input any) (any, error)

// Step is a function executed as a durable step within a workflow.
// Steps are automatically recorded and replayed during recovery.
type Step[R any] func(ctx context.Context) (R, error)

// stepFunc is a type-erased step function.
type stepFunc func(ctx context.Context) (any, error)

// AsyncResult holds the result and error from a concurrent step started with DoAsync.
type AsyncResult[R any] struct {
	Result R
	Err    error
}

type workflowOptions struct {
	WorkflowName        string
	WorkflowID          string
	QueueName           string
	ApplicationVersion  string
	MaxRetries          int
	DeduplicationID     string
	Priority            uint
	QueuePartitionKey   string
	Timeout             time.Duration
	Deadline            time.Time
	Tags                []string
	Summary             string
	alreadyEncodedInput bool
	isDequeue           bool
	isRecovery          bool
}

// WorkflowOption configures workflow execution.
type WorkflowOption func(*workflowOptions)

func WithID(id string) WorkflowOption {
	return func(p *workflowOptions) { p.WorkflowID = id }
}

func WithQueue(queueName string) WorkflowOption {
	return func(p *workflowOptions) { p.QueueName = queueName }
}

func WithApplicationVersion(version string) WorkflowOption {
	return func(p *workflowOptions) { p.ApplicationVersion = version }
}

func WithDeduplicationID(id string) WorkflowOption {
	return func(p *workflowOptions) { p.DeduplicationID = id }
}

func WithPriority(priority uint) WorkflowOption {
	return func(p *workflowOptions) { p.Priority = priority }
}

func WithQueuePartitionKey(partitionKey string) WorkflowOption {
	return func(p *workflowOptions) { p.QueuePartitionKey = partitionKey }
}

// WithTimeout sets a timeout duration for the workflow execution.
// The workflow's context will be cancelled after this duration.
func WithTimeout(d time.Duration) WorkflowOption {
	return func(p *workflowOptions) { p.Timeout = d }
}

// WithDeadline sets an absolute deadline for the workflow execution.
// The workflow's context will be cancelled at this time.
func WithDeadline(t time.Time) WorkflowOption {
	return func(p *workflowOptions) { p.Deadline = t }
}

func withWorkflowName(name string) WorkflowOption {
	return func(p *workflowOptions) {
		if p.WorkflowName == "" {
			p.WorkflowName = name
		}
	}
}

func withAlreadyEncodedInput() WorkflowOption {
	return func(p *workflowOptions) { p.alreadyEncodedInput = true }
}

func withIsDequeue() WorkflowOption {
	return func(p *workflowOptions) { p.isDequeue = true; p.alreadyEncodedInput = true }
}

func withIsRecovery() WorkflowOption {
	return func(p *workflowOptions) { p.isRecovery = true }
}

func withTags(tags []string) WorkflowOption {
	return func(o *workflowOptions) { o.Tags = tags }
}

type workflowOutcome[R any] struct {
	result R
	err    error
}

type baseHandle struct {
	workflowID string
	runtime    *Runtime
}

func (h *baseHandle) GetWorkflowID() string {
	return h.workflowID
}

func (h *baseHandle) GetStatus() (Status, error) {
	statuses, err := retryWithResult(h.runtime.ctx, func() ([]Status, error) {
		return h.runtime.systemDB.listWorkflows(h.runtime.ctx, listWorkflowsDBInput{
			workflowIDs: []string{h.workflowID},
		})
	}, withRetrierLogger(h.runtime.app.Logger()), withMaxRetries(3))
	if err != nil {
		return Status{}, fmt.Errorf("failed to get workflow status: %w", err)
	}
	if len(statuses) == 0 {
		return Status{}, newErrWorkflowNotFound(h.workflowID)
	}
	return statuses[0], nil
}

// workflowHandle is returned when a workflow is started locally.
type workflowHandle[R any] struct {
	baseHandle
	outcomeChan chan workflowOutcome[R]
}

func (h *workflowHandle[R]) GetResult(opts ...GetResultOption) (R, error) {
	options := &getResultOptions{pollInterval: dbRetryInterval}
	for _, opt := range opts {
		opt(options)
	}

	select {
	case outcome, ok := <-h.outcomeChan:
		if !ok {
			return *new(R), errors.New("workflow result channel already closed")
		}
		return outcome.result, outcome.err
	case <-h.runtime.ctx.Done():
		return *new(R), context.Cause(h.runtime.ctx)
	}
}

// workflowPollingHandle is returned for enqueued or recovered workflows.
type workflowPollingHandle[R any] struct {
	baseHandle
}

func (h *workflowPollingHandle[R]) GetResult(opts ...GetResultOption) (R, error) {
	options := &getResultOptions{pollInterval: dbRetryInterval}
	for _, opt := range opts {
		opt(options)
	}

	encodedResult, err := retryWithResult(h.runtime.ctx, func() (*string, error) {
		return h.runtime.systemDB.awaitWorkflowResult(h.runtime.ctx, h.workflowID, options.pollInterval)
	}, withRetrierLogger(h.runtime.app.Logger()))
	if err != nil {
		return *new(R), err
	}
	if encodedResult == nil {
		return *new(R), nil
	}
	return decodeJSON[R](encodedResult)
}

// WithHandlePollingInterval sets the polling interval for GetResult.
func WithHandlePollingInterval(interval time.Duration) GetResultOption {
	return func(opts *getResultOptions) {
		if interval > 0 {
			opts.pollInterval = interval
		}
	}
}

const defaultMaxRecoveryAttempts = 100

type wrappedWorkflowFunc func(rt *Runtime, input any, opts ...WorkflowOption) (Handle[any], error)

// workflowRegistryEntry stores a registered workflow's metadata.
type workflowRegistryEntry struct {
	wrappedFunction wrappedWorkflowFunc
	summaryFunc     func(encodedInput *string) string
	MaxRetries      int
	Name            string
	FQN             string
	CronSchedule    string
	Triggerable     bool
	InputSchema     map[string]any
	Tags            []string
}

type workflowRegistrationOptions struct {
	maxRetries   int
	name         string
	cronSchedule string
	triggerable  bool
	inputSchema  map[string]any
	tags         []string
	summaryFunc  func(encodedInput *string) string
}

// WorkflowRegistrationOption configures workflow registration.
type WorkflowRegistrationOption func(*workflowRegistrationOptions)

func WithMaxRetries(maxRetries int) WorkflowRegistrationOption {
	return func(p *workflowRegistrationOptions) { p.maxRetries = maxRetries }
}

func WithName(name string) WorkflowRegistrationOption {
	return func(p *workflowRegistrationOptions) { p.name = name }
}

// WithSchedule registers the workflow as a scheduled workflow using cron syntax.
// Scheduled workflows must accept time.Time as input, they receive the scheduled execution time.
func WithSchedule(cronExpr string) WorkflowRegistrationOption {
	return func(p *workflowRegistrationOptions) { p.cronSchedule = cronExpr }
}

func WithDashboardTrigger() WorkflowRegistrationOption {
	return func(p *workflowRegistrationOptions) { p.triggerable = true }
}

// WithInputSchema attaches a JSON schema to the workflow, enabling the
// dashboard to render a typed form instead of a raw JSON textarea.
func WithInputSchema(schema map[string]any) WorkflowRegistrationOption {
	return func(p *workflowRegistrationOptions) { p.inputSchema = schema }
}

func WithTags(tags ...string) WorkflowRegistrationOption {
	return func(p *workflowRegistrationOptions) { p.tags = tags }
}

// WithSummaryFunc registers a function that generates a human-readable summary
// from the workflow input. The summary is computed once at workflow start
// and stored in the database. Maximum 200 characters.
func WithSummaryFunc[P any](fn func(P) string) WorkflowRegistrationOption {
	return func(opts *workflowRegistrationOptions) {
		opts.summaryFunc = func(encodedInput *string) string {
			typedInput, err := decodeJSON[P](encodedInput)
			if err != nil {
				return ""
			}
			return fn(typedInput)
		}
	}
}

const maxSummaryLength = 200

func computeSummary(summaryFunc func(*string) string, input any, rt *Runtime) (result string) {
	defer func() {
		if r := recover(); r != nil {
			rt.app.Logger().Warn("summary func panicked", "error", r, "source", "system")
			result = ""
		}
	}()

	var encodedInput *string
	switch v := input.(type) {
	case *string:
		encodedInput = v
	default:
		// Direct call: encode to JSON first so summaryFunc can decode to P
		encoded, err := encodeJSON[any](input)
		if err != nil {
			return ""
		}
		encodedInput = encoded
	}

	result = summaryFunc(encodedInput)
	if len([]rune(result)) > maxSummaryLength {
		result = string([]rune(result)[:maxSummaryLength])
	}
	return result
}

func resolveWorkflowFunctionName[P any, R any](fn Workflow[P, R]) string {
	ptr := reflect.ValueOf(fn).Pointer()
	fqn := runtime.FuncForPC(ptr).Name()
	if strings.Contains(fqn, "[") {
		fqn = strings.Split(fqn, "[")[0]
		fqn = fmt.Sprintf("%s[%s,%s]", fqn, reflect.TypeFor[P]().String(), reflect.TypeFor[R]().String())
	}
	return fqn
}

// Register registers a typed workflow function with the runtime.
// Must be called before Launch().
func Register[P any, R any](rt *Runtime, fn Workflow[P, R], opts ...WorkflowRegistrationOption) {
	if rt.launched.Load() {
		panic("cannot register workflow after runtime has launched")
	}

	regOpts := workflowRegistrationOptions{maxRetries: defaultMaxRecoveryAttempts}
	for _, opt := range opts {
		opt(&regOpts)
	}

	fqn := resolveWorkflowFunctionName(fn)

	// Type-erased wrapper for recovery and queue runner
	typedErasedWF := workflowFunc(func(ctx Context, input any) (any, error) {
		var encodedInput *string
		if input != nil {
			var ok bool
			encodedInput, ok = input.(*string)
			if !ok {
				return nil, fmt.Errorf("expected *string input, got %T", input)
			}
		}
		typedInput, err := decodeJSON[P](encodedInput)
		if err != nil {
			return nil, fmt.Errorf("failed to decode input: %w", err)
		}
		return fn(ctx, typedInput)
	})

	wrapped := wrappedWorkflowFunc(func(rt *Runtime, input any, opts ...WorkflowOption) (Handle[any], error) {
		opts = append(opts, withWorkflowName(fqn), withTags(regOpts.tags))
		return runWorkflowInternal(rt, typedErasedWF, input, opts...)
	})

	entry := workflowRegistryEntry{
		wrappedFunction: wrapped,
		summaryFunc:     regOpts.summaryFunc,
		FQN:             fqn,
		MaxRetries:      regOpts.maxRetries,
		Name:            regOpts.name,
		Triggerable:     regOpts.triggerable,
		InputSchema:     regOpts.inputSchema,
		Tags:            regOpts.tags,
	}

	if _, exists := rt.workflowRegistry.LoadOrStore(fqn, entry); exists {
		panic(newErrRegistrationConflict(fqn))
	}
	customName := regOpts.name
	if customName == "" {
		customName = fqn
	}
	if _, exists := rt.workflowCustomNameToFQN.LoadOrStore(customName, fqn); exists {
		panic(newErrRegistrationConflict(customName))
	}

	// If this is a scheduled workflow, register a cron job via PocketBase
	if regOpts.cronSchedule != "" {
		if reflect.TypeFor[P]() != reflect.TypeFor[time.Time]() {
			panic(fmt.Sprintf("scheduled workflow must accept time.Time as input, got %s", reflect.TypeFor[P]().String()))
		}

		// Update entry with cron schedule and tags
		entry.CronSchedule = regOpts.cronSchedule
		entry.Tags = regOpts.tags
		rt.workflowRegistry.Store(fqn, entry)

		cronJobID := fmt.Sprintf("pt_sched_%s", customName)
		if err := rt.app.Cron().Add(cronJobID, regOpts.cronSchedule, func() {
			defer func() {
				if r := recover(); r != nil {
					rt.app.Logger().Error("cron schedule panicked",
						"fqn", fqn,
						"panic", r,
						"stack", string(debug.Stack()),
						"source", "system")
				}
			}()
			if !rt.launched.Load() {
				return
			}
			if _, disabled := rt.disabledSchedules.Load(fqn); disabled {
				return
			}
			scheduledTime := time.Now()
			wfID := fmt.Sprintf("sched-%s-%s", customName, scheduledTime.UTC().Format(time.RFC3339))
			_, err := wrapped(rt, scheduledTime,
				WithID(wfID),
				WithQueue(ptInternalQueueName),
			)
			if err != nil {
				rt.app.Logger().Error("failed to run scheduled workflow", "name", customName, "error", err)
			}
		}); err != nil {
			panic(fmt.Sprintf("failed to register cron job for %s: %v", customName, err))
		}
	}
}

// Run starts a typed workflow. Returns a handle to get the result.
func Run[P any, R any](rt *Runtime, fn Workflow[P, R], input P, opts ...WorkflowOption) (Handle[R], error) {
	opts = append(opts, withWorkflowName(resolveWorkflowFunctionName(fn)))

	typedErasedWF := workflowFunc(func(ctx Context, inputAny any) (any, error) {
		return fn(ctx, inputAny.(P))
	})

	handle, err := runWorkflowInternal(rt, typedErasedWF, input, opts...)
	if err != nil {
		return nil, err
	}

	// If polling handle, convert to typed
	if ph, ok := handle.(*workflowPollingHandle[any]); ok {
		return &workflowPollingHandle[R]{baseHandle: ph.baseHandle}, nil
	}

	// If local handle, bridge the channel types
	if wh, ok := handle.(*workflowHandle[any]); ok {
		typedChan := make(chan workflowOutcome[R], 1)
		go func() {
			defer close(typedChan)
			outcome := <-wh.outcomeChan
			var typedResult R
			if outcome.result != nil {
				if typed, ok := outcome.result.(R); ok {
					typedResult = typed
				}
			}
			typedChan <- workflowOutcome[R]{result: typedResult, err: outcome.err}
		}()
		return &workflowHandle[R]{
			baseHandle:  wh.baseHandle,
			outcomeChan: typedChan,
		}, nil
	}

	return nil, fmt.Errorf("unexpected handle type")
}

// runWorkflowInternal is the core workflow execution logic.
func runWorkflowInternal(rt *Runtime, fn workflowFunc, input any, opts ...WorkflowOption) (Handle[any], error) {
	params := workflowOptions{ApplicationVersion: rt.applicationVersion}
	for _, opt := range opts {
		opt(&params)
	}

	// Reject new workflows during drain (allow dequeue and recovery to finish)
	if rt.draining.Load() && !params.isDequeue && !params.isRecovery {
		return nil, ErrShuttingDown
	}

	// Lookup registry for registration-time options
	registeredAny, exists := rt.workflowRegistry.Load(params.WorkflowName)
	if !exists {
		return nil, newErrWorkflowNotFound(params.WorkflowName)
	}
	registered := registeredAny.(workflowRegistryEntry)
	if registered.MaxRetries > 0 {
		params.MaxRetries = registered.MaxRetries
	}
	if registered.Name != "" {
		params.WorkflowName = registered.Name
	}
	if len(params.Tags) == 0 && len(registered.Tags) > 0 {
		params.Tags = registered.Tags
	}
	if params.Summary == "" && registered.summaryFunc != nil {
		params.Summary = computeSummary(registered.summaryFunc, input, rt)
	}

	// Validate queue options
	if params.QueuePartitionKey != "" && params.QueueName == "" {
		return nil, fmt.Errorf("partition key provided but queue name is missing")
	}
	if params.QueuePartitionKey != "" && params.DeduplicationID != "" {
		return nil, fmt.Errorf("partition key and deduplication ID cannot be used together")
	}
	if params.QueueName != "" && rt.queueRunner.getQueue(params.QueueName) == nil {
		return nil, fmt.Errorf("queue %s does not exist", params.QueueName)
	}
	if params.Priority > uint(math.MaxInt) {
		return nil, fmt.Errorf("priority %d exceeds max", params.Priority)
	}

	// Generate workflow ID
	workflowID := params.WorkflowID
	if workflowID == "" {
		workflowID = core.GenerateDefaultRandomId()
	}

	var status StatusType
	if params.QueueName != "" {
		status = StatusEnqueued
	} else {
		status = StatusPending
	}

	// Serialize input
	var encodedInput any
	if params.alreadyEncodedInput {
		encodedInput = input
	} else {
		var err error
		encodedInput, err = encodeJSON[any](input)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize input: %w", err)
		}
	}

	wfStatus := Status{
		Name:               params.WorkflowName,
		ApplicationVersion: params.ApplicationVersion,
		ExecutorID:         rt.executorID,
		Status:             status,
		ID:                 workflowID,
		CreatedAt:          time.Now(),
		Input:              encodedInput,
		ApplicationID:      rt.applicationID,
		QueueName:          params.QueueName,
		DeduplicationID:    params.DeduplicationID,
		Priority:           int(params.Priority),
		QueuePartitionKey:  params.QueuePartitionKey,
		Timeout:            params.Timeout,
		Deadline:           params.Deadline,
		Tags:               params.Tags,
		Summary:            params.Summary,
	}

	ownerXID := core.GenerateDefaultRandomId()
	insertInput := insertStatusDBInput{
		status:            wfStatus,
		maxRetries:        params.MaxRetries,
		ownerXID:          &ownerXID,
		incrementAttempts: params.isDequeue || params.isRecovery,
	}
	insertResult, err := rt.systemDB.insertStatus(rt.ctx, insertInput)
	if err != nil {
		var ptErr *Error
		if errors.As(err, &ptErr) && ptErr.Code == ErrDeadLetter {
			go rt.dispatchEvent(workflowID, params.WorkflowName, StatusMaxRecoveryAttemptsExceeded, nil, &ptErr.Message)
		}
		return nil, fmt.Errorf("failed to insert workflow: %w", err)
	}

	// Check if we should skip execution
	_, loaded := rt.activeWorkflowIDs.Load(workflowID)
	shouldSkip := params.QueueName != "" ||
		insertResult.status == StatusSuccess ||
		insertResult.status == StatusError ||
		(!params.isDequeue && !params.isRecovery && insertResult.ownerXID != ownerXID) ||
		loaded

	if shouldSkip {
		return &workflowPollingHandle[any]{baseHandle: baseHandle{workflowID: workflowID, runtime: rt}}, nil
	}

	// Create workflow state
	wfState := &workflowState{
		workflowID:     workflowID,
		workflowName:   params.WorkflowName,
		recovering:     insertResult.hasSteps,
		appStatus:      insertResult.appStatus,
		appStatusColor: insertResult.appStatusColor,
	}
	wfState.stepID.Store(-1)
	baseCtx := context.WithValue(rt.ctx, workflowStateKey, wfState)

	// Apply deadline if set
	var cancelTimeout context.CancelFunc
	if insertResult.timeout > 0 && insertResult.workflowDeadline.IsZero() {
		baseCtx, cancelTimeout = context.WithTimeout(baseCtx, insertResult.timeout)
	} else if !insertResult.workflowDeadline.IsZero() {
		baseCtx, cancelTimeout = context.WithDeadline(baseCtx, insertResult.workflowDeadline)
	}

	wfCtx := &ptContext{Context: baseCtx, runtime: rt}

	rt.app.Logger().Info("workflow started", "workflow_id", workflowID, "name", wfStatus.Name, "source", "system")

	outcomeChan := make(chan workflowOutcome[any], 1)
	rt.workflowsWg.Add(1)
	go func() {
		defer rt.workflowsWg.Done()
		if cancelTimeout != nil {
			defer cancelTimeout()
		}
		rt.activeWorkflowIDs.Store(workflowID, struct{}{})
		defer rt.activeWorkflowIDs.Delete(workflowID)

		defer func() {
			r := recover()
			if r == nil {
				return
			}
			panicMsg := fmt.Sprintf("workflow panicked: %v", r)
			rt.app.Logger().Error("workflow goroutine panicked",
				"workflow_id", workflowID,
				"name", params.WorkflowName,
				"panic", r,
				"stack", string(debug.Stack()),
				"source", "system")
			_ = retry(rt.ctx, func() error {
				return rt.systemDB.updateWorkflowOutcome(rt.ctx, updateWorkflowOutcomeDBInput{
					workflowID: workflowID,
					status:     StatusError,
					output:     nil,
					errorMsg:   &panicMsg,
				})
			}, withRetrierLogger(rt.app.Logger()))
			go rt.dispatchEvent(workflowID, params.WorkflowName, StatusError, nil, &panicMsg)
			outcomeChan <- workflowOutcome[any]{err: errors.New(panicMsg)}
			close(outcomeChan)
		}()

		result, fnErr := fn(wfCtx, input)

		// Handle workflow ID conflict, another goroutine owns this workflow ID.
		// Wait for the existing workflow to complete and return its result.
		if errors.Is(fnErr, &Error{Code: ErrConflictingID}) {
			rt.app.Logger().Warn("workflow ID conflict, waiting for existing workflow", "workflow_id", workflowID)
			encoded, awaitErr := retryWithResult(rt.ctx, func() (*string, error) {
				return rt.systemDB.awaitWorkflowResult(rt.ctx, workflowID, dbRetryInterval)
			}, withRetrierLogger(rt.app.Logger()))
			outcomeChan <- workflowOutcome[any]{result: encoded, err: awaitErr}
			close(outcomeChan)
			return
		}

		outcomeStatus := StatusSuccess
		if fnErr != nil {
			outcomeStatus = StatusError
			rt.app.Logger().Error("workflow failed", "workflow_id", workflowID, "error", fnErr.Error(), "source", "system")
		} else {
			rt.app.Logger().Info("workflow completed", "workflow_id", workflowID, "source", "system")
		}

		// Serialize output
		encodedOutput, serErr := encodeJSON[any](result)
		if serErr != nil {
			outcomeChan <- workflowOutcome[any]{err: fmt.Errorf("failed to serialize output: %w", serErr)}
			close(outcomeChan)
			return
		}

		var errorMsg *string
		if fnErr != nil {
			s := fnErr.Error()
			errorMsg = &s
		}

		recordErr := retry(rt.ctx, func() error {
			return rt.systemDB.updateWorkflowOutcome(rt.ctx, updateWorkflowOutcomeDBInput{
				workflowID: workflowID,
				status:     outcomeStatus,
				output:     encodedOutput,
				errorMsg:   errorMsg,
			})
		}, withRetrierLogger(rt.app.Logger()))
		if recordErr != nil {
			outcomeChan <- workflowOutcome[any]{err: recordErr}
			close(outcomeChan)
			return
		}

		// Dispatch webhooks and notifications after outcome is recorded
		go rt.dispatchEvent(workflowID, params.WorkflowName, outcomeStatus, encodedOutput, errorMsg)

		outcomeChan <- workflowOutcome[any]{result: result, err: fnErr}
		close(outcomeChan)
	}()

	return &workflowHandle[any]{
		baseHandle:  baseHandle{workflowID: workflowID, runtime: rt},
		outcomeChan: outcomeChan,
	}, nil
}

const (
	defaultStepBaseInterval  = 100 * time.Millisecond
	defaultStepMaxInterval   = 5 * time.Second
	defaultStepBackoffFactor = 2.0
)

type stepOptions struct {
	maxRetries     int
	backoffFactor  float64
	baseInterval   time.Duration
	maxInterval    time.Duration
	stepName       string
	preStepID      *int
	skipCheckpoint bool
}

func (opts *stepOptions) setDefaults() {
	if opts.backoffFactor == 0 {
		opts.backoffFactor = defaultStepBackoffFactor
	}
	if opts.baseInterval == 0 {
		opts.baseInterval = defaultStepBaseInterval
	}
	if opts.maxInterval == 0 {
		opts.maxInterval = defaultStepMaxInterval
	}
}

// StepOption configures step execution.
type StepOption func(*stepOptions)

func WithStepName(name string) StepOption {
	return func(opts *stepOptions) {
		if opts.stepName == "" {
			opts.stepName = name
		}
	}
}

func WithStepMaxRetries(maxRetries int) StepOption {
	return func(opts *stepOptions) { opts.maxRetries = maxRetries }
}

func WithBackoffFactor(factor float64) StepOption {
	return func(opts *stepOptions) { opts.backoffFactor = factor }
}

func WithBaseInterval(interval time.Duration) StepOption {
	return func(opts *stepOptions) { opts.baseInterval = interval }
}

func WithMaxInterval(interval time.Duration) StepOption {
	return func(opts *stepOptions) { opts.maxInterval = interval }
}

func withNextStepID(stepID int) StepOption {
	return func(opts *stepOptions) { opts.preStepID = &stepID }
}

// WithoutCheckpoint skips persisting the step result to the database.
// The step will always re-execute during recovery instead of replaying from a checkpoint.
// Use this for steps that establish non-serializable resources like network connections.
func WithoutCheckpoint() StepOption {
	return func(opts *stepOptions) { opts.skipCheckpoint = true }
}

// Do executes a function as a durable step within a workflow.
func Do[R any](ctx Context, fn Step[R], opts ...StepOption) (R, error) {
	rt := runtimeFromContext(ctx)
	stepName := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	opts = append(opts, WithStepName(stepName))

	typeErased := stepFunc(func(ctx context.Context) (any, error) { return fn(ctx) })

	result, err := runAsStepInternal(ctx, rt, typeErased, opts...)
	if result == nil {
		return *new(R), err
	}

	// Check if result is a checkpointed (encoded) value
	if cp, ok := result.(stepCheckpointedOutcome); ok {
		encoded, ok := cp.value.(*string)
		if !ok {
			return *new(R), fmt.Errorf("checkpointed value is not *string")
		}
		return decodeJSON[R](encoded)
	}

	if typed, ok := result.(R); ok {
		return typed, err
	}
	return *new(R), fmt.Errorf("unexpected step result type %T", result)
}

type stepCheckpointedOutcome struct {
	value any
}

func runAsStepInternal(ctx context.Context, rt *Runtime, fn stepFunc, opts ...StepOption) (any, error) {
	stepOpts := &stepOptions{}
	for _, opt := range opts {
		opt(stepOpts)
	}
	stepOpts.setDefaults()

	wfState, ok := ctx.Value(workflowStateKey).(*workflowState)
	if !ok || wfState == nil {
		return nil, fmt.Errorf("step must be called within a workflow")
	}
	if wfState.isWithinStep {
		return fn(ctx)
	}

	var stepID int
	if stepOpts.preStepID != nil {
		stepID = *stepOpts.preStepID
	} else {
		stepID = wfState.nextStepID()
	}

	if !stepOpts.skipCheckpoint {
		// Check if already executed
		recorded, err := retryWithResult(ctx, func() (*recordedResult, error) {
			return rt.systemDB.checkOperationExecution(ctx, checkOperationExecutionDBInput{
				workflowUUID: wfState.workflowID,
				functionID:   stepID,
			})
		}, withRetrierLogger(rt.app.Logger()))
		if err != nil {
			return nil, fmt.Errorf("checking step execution: %w", err)
		}
		if recorded != nil {
			rt.app.Logger().Debug("step replayed from checkpoint", "workflow_id", wfState.workflowID, "step", stepOpts.stepName, "step_id", stepID, "source", "system")
			wfState.recovering = true
			var stepErr error
			if recorded.errorMsg != nil && *recorded.errorMsg != "" {
				stepErr = errors.New(*recorded.errorMsg)
			}
			return stepCheckpointedOutcome{value: recorded.output}, stepErr
		}

		// Clear recovering flag, we've passed replay and are executing new steps
		wfState.recovering = false
	}

	// Execute with retry
	rt.app.Logger().Info("step started", "workflow_id", wfState.workflowID, "step", stepOpts.stepName, "step_id", stepID, "source", "system")
	stepState := &workflowState{workflowID: wfState.workflowID, isWithinStep: true}
	stepState.stepID.Store(int64(stepID))
	stepCtx := context.WithValue(ctx, workflowStateKey, stepState)
	startTime := time.Now()

	startErr := retry(ctx, func() error {
		return rt.systemDB.recordOperationStart(ctx, recordOperationStartDBInput{
			workflowUUID: wfState.workflowID,
			functionID:   stepID,
			functionName: stepOpts.stepName,
			startedAt:    startTime.UnixMilli(),
		})
	}, withRetrierLogger(rt.app.Logger()))
	if startErr != nil {
		return nil, fmt.Errorf("recording step start: %w", startErr)
	}

	stepOutput, stepErr := executeStepWithRetry(ctx, rt, stepOpts, func() (any, error) { return fn(stepCtx) })

	endTime := time.Now()
	dur := endTime.Sub(startTime)
	if stepErr != nil {
		rt.app.Logger().Error("step failed", "workflow_id", wfState.workflowID, "step", stepOpts.stepName, "step_id", stepID, "duration", dur, "error", stepErr.Error(), "source", "system")
	} else {
		rt.app.Logger().Info("step completed", "workflow_id", wfState.workflowID, "step", stepOpts.stepName, "step_id", stepID, "duration", dur, "source", "system")
	}

	var encodedOutput *string
	if !stepOpts.skipCheckpoint {
		// WithoutCheckpoint steps may return non-serializable values (connections, handles).
		var serErr error
		encodedOutput, serErr = encodeJSON[any](stepOutput)
		if serErr != nil {
			return nil, fmt.Errorf("failed to serialize step output: %w", serErr)
		}
	}

	var errorMsg *string
	if stepErr != nil {
		s := stepErr.Error()
		errorMsg = &s
	}

	recErr := retry(ctx, func() error {
		return rt.systemDB.recordOperationResult(ctx, recordOperationResultDBInput{
			workflowUUID: wfState.workflowID,
			functionID:   stepID,
			functionName: stepOpts.stepName,
			output:       encodedOutput,
			errorMsg:     errorMsg,
			startedAt:    startTime.UnixMilli(),
			endedAt:      endTime.UnixMilli(),
		})
	}, withRetrierLogger(rt.app.Logger()))
	if recErr != nil {
		return nil, fmt.Errorf("recording step result: %w", recErr)
	}

	return stepOutput, stepErr
}

func executeStepWithRetry(ctx context.Context, rt *Runtime, opts *stepOptions, runOnce func() (any, error)) (any, error) {
	output, err := runOnce()
	if err == nil || opts.maxRetries <= 0 {
		return output, err
	}

	var joinedErrors error
	joinedErrors = errors.Join(joinedErrors, err)
	for retry := 1; retry <= opts.maxRetries; retry++ {
		delay := opts.baseInterval
		if retry > 1 {
			expDelay := float64(opts.baseInterval) * math.Pow(opts.backoffFactor, float64(retry-1))
			delay = time.Duration(math.Min(expDelay, float64(opts.maxInterval)))
		}
		rt.app.Logger().Error("step failed, retrying", "step_name", opts.stepName, "retry", retry, "delay", delay)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
		case <-time.After(delay):
		}
		output, err = runOnce()
		if err == nil {
			return output, nil
		}
		joinedErrors = errors.Join(joinedErrors, err)
	}
	return output, newErrMaxRetries("", opts.stepName, opts.maxRetries, joinedErrors)
}

// Go runs a step in a goroutine. Must be within a workflow.
func DoAsync[R any](ctx Context, fn Step[R], opts ...StepOption) (chan AsyncResult[R], error) {
	wfState, ok := ctx.Value(workflowStateKey).(*workflowState)
	if !ok || wfState == nil {
		return nil, fmt.Errorf("doAsync must be called within a workflow")
	}
	opts = append(opts, withNextStepID(wfState.nextStepID()))

	ch := make(chan AsyncResult[R], 1)
	rt := runtimeFromContext(ctx)
	go func() {
		defer close(ch)
		defer func() {
			if r := recover(); r != nil {
				if rt != nil && rt.app != nil {
					rt.app.Logger().Error("DoAsync goroutine panicked",
						"panic", r,
						"stack", string(debug.Stack()),
						"source", "system")
				}
				ch <- AsyncResult[R]{Err: fmt.Errorf("DoAsync panicked: %v", r)}
			}
		}()
		res, err := Do(ctx, fn, opts...)
		ch <- AsyncResult[R]{Result: res, Err: err}
	}()
	return ch, nil
}

// Send sends a message to another workflow.
func Send(ctx Context, destinationID string, message any, topic string) error {
	rt := runtimeFromContext(ctx)
	encoded, err := encodeJSON[any](message)
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// If within a workflow, record as step
	wfState, ok := ctx.Value(workflowStateKey).(*workflowState)
	if ok && wfState != nil {
		if wfState.isWithinStep {
			return fmt.Errorf("cannot call Send within a step")
		}
		_, err = Do(ctx, func(stepCtx context.Context) (any, error) {
			stepState, _ := stepCtx.Value(workflowStateKey).(*workflowState)
			in := sendInput{
				DestinationUUID: destinationID,
				Topic:           topic,
				Message:         encoded,
			}
			if stepState != nil {
				in.ProducerWorkflow = stepState.workflowID
				in.ProducerStepID = int(stepState.stepID.Load())
				in.HasProducer = true
			}
			return nil, rt.systemDB.send(stepCtx, in)
		}, WithStepName("pt.send"))
		return err
	}

	return retry(ctx, func() error {
		return rt.systemDB.send(ctx, sendInput{
			DestinationUUID: destinationID,
			Topic:           topic,
			Message:         encoded,
		})
	}, withRetrierLogger(rt.app.Logger()))
}

// Recv receives a message within a workflow.
func Recv[R any](ctx Context, topic string, timeout time.Duration) (R, error) {
	rt := runtimeFromContext(ctx)
	wfState, ok := ctx.Value(workflowStateKey).(*workflowState)
	if !ok || wfState == nil {
		return *new(R), fmt.Errorf("Recv must be called within a workflow")
	}
	if wfState.isWithinStep {
		return *new(R), fmt.Errorf("cannot call Recv within a step")
	}

	encoded, err := retryWithResult(ctx, func() (*string, error) {
		return rt.systemDB.recv(ctx, recvInput{
			workflowUUID: wfState.workflowID,
			topic:        topic,
			timeout:      timeout,
		})
	}, withRetrierLogger(rt.app.Logger()))
	if err != nil {
		return *new(R), err
	}
	if encoded == nil {
		return *new(R), nil
	}
	return decodeJSON[R](encoded)
}

// SetValue sets a key-value event for the current workflow.
func SetValue(ctx Context, key string, value any) error {
	rt := runtimeFromContext(ctx)
	encoded, err := encodeJSON[any](value)
	if err != nil {
		return fmt.Errorf("failed to serialize event value: %w", err)
	}

	wfState, ok := ctx.Value(workflowStateKey).(*workflowState)
	if !ok || wfState == nil {
		return fmt.Errorf("SetValue must be called within a workflow")
	}

	_, err = Do(ctx, func(ctx context.Context) (any, error) {
		return nil, rt.systemDB.setEvent(ctx, setValueInput{
			WorkflowUUID: wfState.workflowID,
			Key:          key,
			Value:        encoded,
		})
	}, WithStepName("pt.setEvent"))
	return err
}

// GetValue gets a key-value event from a target workflow.
func GetValue[R any](ctx Context, targetWorkflowID string, key string, timeout time.Duration) (R, error) {
	rt := runtimeFromContext(ctx)
	encoded, err := retryWithResult(ctx, func() (*string, error) {
		return rt.systemDB.getEvent(ctx, getEventInput{
			targetWorkflowUUID: targetWorkflowID,
			key:                key,
			timeout:            timeout,
		})
	}, withRetrierLogger(rt.app.Logger()))
	if err != nil {
		return *new(R), err
	}
	if encoded == nil {
		return *new(R), nil
	}
	return decodeJSON[R](encoded)
}

// Sleep performs a durable sleep within a workflow.
// The wake-up time is recorded as a step so that on recovery,
// if the wake-up time has already passed, Pause returns immediately;
// otherwise it sleeps only the remaining time.
func Pause(ctx Context, duration time.Duration) error {
	wfState, ok := ctx.Value(workflowStateKey).(*workflowState)
	if !ok || wfState == nil {
		return fmt.Errorf("pause must be called within a workflow")
	}
	if wfState.isWithinStep {
		return fmt.Errorf("cannot call Sleep within a step")
	}

	// The step records the wake-up time in millis. On first run, the step body
	// executes the sleep. On replay, we get the stored wake-up time back and
	// sleep only the remaining duration.
	wakeUpMs, err := Do(ctx, func(ctx context.Context) (int64, error) {
		wakeUpTime := time.Now().Add(duration)
		remaining := time.Until(wakeUpTime)
		if remaining > 0 {
			select {
			case <-time.After(remaining):
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		}
		return wakeUpTime.UnixMilli(), nil
	}, WithStepName("pt.sleep"))
	if err != nil {
		return err
	}

	// On replay, wakeUpMs is the originally recorded wake-up time.
	// Sleep only the remaining duration (if any).
	wakeUpTime := time.UnixMilli(wakeUpMs)
	remaining := time.Until(wakeUpTime)
	if remaining > 0 {
		select {
		case <-time.After(remaining):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Retrieve returns a handle to an existing workflow.
func Retrieve[R any](rt *Runtime, workflowID string) Handle[R] {
	return &workflowPollingHandle[R]{baseHandle: baseHandle{workflowID: workflowID, runtime: rt}}
}

// Cancel cancels a workflow by ID.
func (rt *Runtime) Cancel(workflowID string) error {
	var transitioned bool
	err := retry(rt.ctx, func() error {
		changed, err := rt.systemDB.cancelWorkflow(rt.ctx, cancelWorkflowDBInput{workflowID: workflowID})
		if err != nil {
			return err
		}
		transitioned = changed
		return nil
	}, withRetrierLogger(rt.app.Logger()), withMaxRetries(3))
	if err != nil {
		return err
	}
	if transitioned {
		rt.app.Logger().Info("workflow cancelled", "workflow_id", workflowID, "source", "system")
		record, err := rt.app.FindRecordById(collectionStatus, workflowID)
		if err != nil {
			rt.app.Logger().Error("failed to look up cancelled workflow for dispatch", "workflow_id", workflowID, "error", err, "source", "system")
			return nil
		}
		name := record.GetString("name")
		go rt.dispatchEvent(workflowID, name, StatusCancelled, nil, nil)
	}

	return nil
}

// Resume resumes a cancelled workflow.
func (rt *Runtime) Resume(workflowID string) error {
	err := retry(rt.ctx, func() error {
		return rt.systemDB.resumeWorkflow(rt.ctx, resumeWorkflowDBInput{
			workflowID: workflowID,
			executorID: rt.executorID,
			appVersion: rt.applicationVersion,
		})
	}, withRetrierLogger(rt.app.Logger()), withMaxRetries(3))
	if err != nil {
		return err
	}
	rt.app.Logger().Info("workflow resumed", "workflow_id", workflowID, "source", "system")
	return nil
}

// ListOption configures a Runtime.List query.
type ListOption func(*listWorkflowsDBInput)

// WithStatus filters results to workflows in any of the given statuses.
func WithStatus(statuses ...StatusType) ListOption {
	return func(o *listWorkflowsDBInput) { o.status = statuses }
}

// WithWorkflowNames filters results to workflows registered under any of the given names.
func WithWorkflowNames(names ...string) ListOption {
	return func(o *listWorkflowsDBInput) { o.workflowName = names }
}

// WithExecutorIDs filters results to workflows executed by any of the given executor IDs.
func WithExecutorIDs(ids ...string) ListOption {
	return func(o *listWorkflowsDBInput) { o.executorIDs = ids }
}

// WithApplicationVersions filters results to workflows recorded against any of the given versions.
func WithApplicationVersions(versions ...string) ListOption {
	return func(o *listWorkflowsDBInput) { o.applicationVersion = versions }
}

// WithWorkflowIDs filters results to workflows with any of the given IDs.
func WithWorkflowIDs(ids ...string) ListOption {
	return func(o *listWorkflowsDBInput) { o.workflowIDs = ids }
}

// WithLimit caps the number of results returned. A value of zero means no limit.
func WithLimit(n int) ListOption {
	return func(o *listWorkflowsDBInput) { o.limit = n }
}

// WithLoadInput populates Status.Input for each returned row. Off by default for efficiency.
func WithLoadInput() ListOption {
	return func(o *listWorkflowsDBInput) { o.loadInput = true }
}

// WithSortAscending sorts results by creation time ascending. Default is descending.
func WithSortAscending() ListOption {
	return func(o *listWorkflowsDBInput) { o.sortAscending = true }
}

// WithCreatedBefore filters results to workflows created strictly before t.
func WithCreatedBefore(t time.Time) ListOption {
	return func(o *listWorkflowsDBInput) { o.createdBefore = &t }
}

// WithCreatedAfter filters results to workflows created strictly after t.
func WithCreatedAfter(t time.Time) ListOption {
	return func(o *listWorkflowsDBInput) { o.createdAfter = &t }
}

// List returns workflows matching the given filters.
func (rt *Runtime) List(opts ...ListOption) ([]Status, error) {
	var input listWorkflowsDBInput
	for _, o := range opts {
		o(&input)
	}
	return retryWithResult(rt.ctx, func() ([]Status, error) {
		return rt.systemDB.listWorkflows(rt.ctx, input)
	}, withRetrierLogger(rt.app.Logger()), withMaxRetries(3))
}

// Steps returns the execution steps for a workflow.
func (rt *Runtime) Steps(workflowID string) ([]StepInfo, error) {
	steps, err := retryWithResult(rt.ctx, func() ([]stepInfo, error) {
		return rt.systemDB.getWorkflowSteps(rt.ctx, getWorkflowStepsInput{workflowID: workflowID})
	}, withRetrierLogger(rt.app.Logger()), withMaxRetries(3))
	if err != nil {
		return nil, err
	}
	result := make([]StepInfo, len(steps))
	for i, s := range steps {
		result[i] = StepInfo{
			WorkflowID:   s.workflowUUID,
			FunctionID:   s.functionID,
			FunctionName: s.functionName,
		}
		if s.output != nil {
			result[i].Output = *s.output
		}
		if s.errorMsg != nil {
			result[i].Error = *s.errorMsg
		}
		if s.startedAt != nil {
			result[i].StartedAt = *s.startedAt
		}
		if s.endedAt != nil {
			result[i].EndedAt = *s.endedAt
		}
	}
	return result, nil
}
