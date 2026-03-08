package pbdbos

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
)

/*******************************/
/******* FUNCTION TYPES *******/
/*******************************/

type dbosContextKey string

const workflowStateKey dbosContextKey = "workflowState"

// Workflow is a type-safe workflow function.
type Workflow[P any, R any] func(ctx context.Context, rt *Runtime, input P) (R, error)

// WorkflowFunc is a type-erased workflow function used internally.
type WorkflowFunc func(ctx context.Context, rt *Runtime, input any) (any, error)

// Step is a function executed as a durable step within a workflow.
// Steps are automatically recorded and replayed during recovery.
type Step[R any] func(ctx context.Context) (R, error)

// StepFunc is a type-erased step function.
type StepFunc func(ctx context.Context) (any, error)

// StepOutcome holds the result and error from a concurrent step started with Go.
type StepOutcome[R any] struct {
	Result R
	Err    error
}

/********************************/
/******* WORKFLOW OPTIONS *******/
/********************************/

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
	alreadyEncodedInput bool
	isDequeue           bool
	isRecovery          bool
}

// WorkflowOption configures workflow execution.
type WorkflowOption func(*workflowOptions)

func WithWorkflowID(id string) WorkflowOption {
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

/*******************************/
/******* WORKFLOW HANDLES *****/
/*******************************/

type workflowOutcome[R any] struct {
	result R
	err    error
}

type baseWorkflowHandle struct {
	workflowID string
	runtime    *Runtime
}

func (h *baseWorkflowHandle) GetWorkflowID() string {
	return h.workflowID
}

func (h *baseWorkflowHandle) GetStatus() (WorkflowStatus, error) {
	statuses, err := retryWithResult(h.runtime.ctx, func() ([]WorkflowStatus, error) {
		return h.runtime.systemDB.listWorkflows(h.runtime.ctx, listWorkflowsDBInput{
			workflowIDs: []string{h.workflowID},
		})
	}, withRetrierLogger(h.runtime.logger))
	if err != nil {
		return WorkflowStatus{}, fmt.Errorf("failed to get workflow status: %w", err)
	}
	if len(statuses) == 0 {
		return WorkflowStatus{}, newNonExistentWorkflowError(h.workflowID)
	}
	return statuses[0], nil
}

// workflowHandle is returned when a workflow is started locally.
type workflowHandle[R any] struct {
	baseWorkflowHandle
	outcomeChan chan workflowOutcome[R]
}

func (h *workflowHandle[R]) GetResult(opts ...GetResultOption) (R, error) {
	options := &getResultOptions{pollInterval: _DB_RETRY_INTERVAL}
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
	baseWorkflowHandle
}

func (h *workflowPollingHandle[R]) GetResult(opts ...GetResultOption) (R, error) {
	options := &getResultOptions{pollInterval: _DB_RETRY_INTERVAL}
	for _, opt := range opts {
		opt(options)
	}

	encodedResult, err := retryWithResult(h.runtime.ctx, func() (*string, error) {
		return h.runtime.systemDB.awaitWorkflowResult(h.runtime.ctx, h.workflowID, options.pollInterval)
	}, withRetrierLogger(h.runtime.logger))
	if err != nil {
		return *new(R), err
	}
	if encodedResult == nil {
		return *new(R), nil
	}
	serializer := newJSONSerializer[R]()
	return serializer.Decode(encodedResult)
}

// WithHandlePollingInterval sets the polling interval for GetResult.
func WithHandlePollingInterval(interval time.Duration) GetResultOption {
	return func(opts *getResultOptions) {
		if interval > 0 {
			opts.pollInterval = interval
		}
	}
}

/***************************************/
/******* WORKFLOW REGISTRATION ********/
/***************************************/

const _DEFAULT_MAX_RECOVERY_ATTEMPTS = 100

type wrappedWorkflowFunc func(rt *Runtime, input any, opts ...WorkflowOption) (WorkflowHandle[any], error)

// WorkflowRegistryEntry stores a registered workflow's metadata.
type WorkflowRegistryEntry struct {
	wrappedFunction wrappedWorkflowFunc
	MaxRetries      int
	Name            string
	FQN             string
	CronSchedule    string
}

type workflowRegistrationOptions struct {
	maxRetries   int
	name         string
	cronSchedule string
}

// WorkflowRegistrationOption configures workflow registration.
type WorkflowRegistrationOption func(*workflowRegistrationOptions)

func WithMaxRetries(maxRetries int) WorkflowRegistrationOption {
	return func(p *workflowRegistrationOptions) { p.maxRetries = maxRetries }
}

func WithWorkflowName(name string) WorkflowRegistrationOption {
	return func(p *workflowRegistrationOptions) { p.name = name }
}

// WithSchedule registers the workflow as a scheduled workflow using cron syntax.
// Scheduled workflows must accept time.Time as input — they receive the scheduled execution time.
func WithSchedule(cronExpr string) WorkflowRegistrationOption {
	return func(p *workflowRegistrationOptions) { p.cronSchedule = cronExpr }
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

// RegisterWorkflow registers a typed workflow function with the runtime.
// Must be called before Launch().
func RegisterWorkflow[P any, R any](rt *Runtime, fn Workflow[P, R], opts ...WorkflowRegistrationOption) {
	if rt.launched.Load() {
		panic("cannot register workflow after runtime has launched")
	}

	regOpts := workflowRegistrationOptions{maxRetries: _DEFAULT_MAX_RECOVERY_ATTEMPTS}
	for _, opt := range opts {
		opt(&regOpts)
	}

	fqn := resolveWorkflowFunctionName(fn)

	// Type-erased wrapper for recovery and queue runner
	typedErasedWF := WorkflowFunc(func(ctx context.Context, rt *Runtime, input any) (any, error) {
		var encodedInput *string
		if input != nil {
			var ok bool
			encodedInput, ok = input.(*string)
			if !ok {
				return nil, fmt.Errorf("expected *string input, got %T", input)
			}
		}
		serializer := newJSONSerializer[P]()
		typedInput, err := serializer.Decode(encodedInput)
		if err != nil {
			return nil, fmt.Errorf("failed to decode input: %w", err)
		}
		return fn(ctx, rt, typedInput)
	})

	wrapped := wrappedWorkflowFunc(func(rt *Runtime, input any, opts ...WorkflowOption) (WorkflowHandle[any], error) {
		opts = append(opts, withWorkflowName(fqn))
		return runWorkflowInternal(rt, typedErasedWF, input, opts...)
	})

	entry := WorkflowRegistryEntry{
		wrappedFunction: wrapped,
		FQN:             fqn,
		MaxRetries:      regOpts.maxRetries,
		Name:            regOpts.name,
	}

	if _, exists := rt.workflowRegistry.LoadOrStore(fqn, entry); exists {
		panic(newConflictingRegistrationError(fqn))
	}
	customName := regOpts.name
	if customName == "" {
		customName = fqn
	}
	if _, exists := rt.workflowCustomNameToFQN.LoadOrStore(customName, fqn); exists {
		panic(newConflictingRegistrationError(customName))
	}

	// If this is a scheduled workflow, register a cron job via PocketBase
	if regOpts.cronSchedule != "" {
		if reflect.TypeFor[P]() != reflect.TypeFor[time.Time]() {
			panic(fmt.Sprintf("scheduled workflow must accept time.Time as input, got %s", reflect.TypeFor[P]().String()))
		}

		// Update entry with cron schedule
		entry.CronSchedule = regOpts.cronSchedule
		rt.workflowRegistry.Store(fqn, entry)

		cronJobID := fmt.Sprintf("pbdbos_sched_%s", customName)
		if err := rt.app.Cron().Add(cronJobID, regOpts.cronSchedule, func() {
			if !rt.launched.Load() {
				return
			}
			scheduledTime := time.Now()
			wfID := fmt.Sprintf("sched-%s-%s", customName, scheduledTime.UTC().Format(time.RFC3339))
			_, err := wrapped(rt, scheduledTime,
				WithWorkflowID(wfID),
				WithQueue(_DBOS_INTERNAL_QUEUE_NAME),
			)
			if err != nil {
				rt.logger.Error("failed to run scheduled workflow", "name", customName, "error", err)
			}
		}); err != nil {
			panic(fmt.Sprintf("failed to register cron job for %s: %v", customName, err))
		}
	}
}

/**********************************/
/******* RUN WORKFLOW ********/
/**********************************/

// RunWorkflow starts a typed workflow. Returns a handle to get the result.
func RunWorkflow[P any, R any](rt *Runtime, fn Workflow[P, R], input P, opts ...WorkflowOption) (WorkflowHandle[R], error) {
	opts = append(opts, withWorkflowName(resolveWorkflowFunctionName(fn)))

	typedErasedWF := WorkflowFunc(func(ctx context.Context, rt *Runtime, inputAny any) (any, error) {
		return fn(ctx, rt, inputAny.(P))
	})

	handle, err := runWorkflowInternal(rt, typedErasedWF, input, opts...)
	if err != nil {
		return nil, err
	}

	// If polling handle, convert to typed
	if ph, ok := handle.(*workflowPollingHandle[any]); ok {
		return &workflowPollingHandle[R]{baseWorkflowHandle: ph.baseWorkflowHandle}, nil
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
			baseWorkflowHandle: wh.baseWorkflowHandle,
			outcomeChan:        typedChan,
		}, nil
	}

	return nil, fmt.Errorf("unexpected handle type")
}

// runWorkflowInternal is the core workflow execution logic.
func runWorkflowInternal(rt *Runtime, fn WorkflowFunc, input any, opts ...WorkflowOption) (WorkflowHandle[any], error) {
	params := workflowOptions{ApplicationVersion: rt.applicationVersion}
	for _, opt := range opts {
		opt(&params)
	}

	// Lookup registry for registration-time options
	registeredAny, exists := rt.workflowRegistry.Load(params.WorkflowName)
	if !exists {
		return nil, newNonExistentWorkflowError(params.WorkflowName)
	}
	registered := registeredAny.(WorkflowRegistryEntry)
	if registered.MaxRetries > 0 {
		params.MaxRetries = registered.MaxRetries
	}
	if registered.Name != "" {
		params.WorkflowName = registered.Name
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
		workflowID = uuid.New().String()
	}

	var status WorkflowStatusType
	if params.QueueName != "" {
		status = WorkflowStatusEnqueued
	} else {
		status = WorkflowStatusPending
	}

	// Serialize input
	var encodedInput any
	if params.alreadyEncodedInput {
		encodedInput = input
	} else {
		ser := newJSONSerializer[any]()
		var err error
		encodedInput, err = ser.Encode(input)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize input: %w", err)
		}
	}

	wfStatus := WorkflowStatus{
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
	}

	ownerXID := uuid.New().String()
	insertInput := insertWorkflowStatusDBInput{
		status:            wfStatus,
		maxRetries:        params.MaxRetries,
		ownerXID:          &ownerXID,
		incrementAttempts: params.isDequeue || params.isRecovery,
	}
	insertResult, err := rt.systemDB.insertWorkflowStatus(rt.ctx, insertInput)
	if err != nil {
		return nil, fmt.Errorf("failed to insert workflow: %w", err)
	}

	// Check if we should skip execution
	_, loaded := rt.activeWorkflowIDs.Load(workflowID)
	shouldSkip := params.QueueName != "" ||
		insertResult.status == WorkflowStatusSuccess ||
		insertResult.status == WorkflowStatusError ||
		(!params.isDequeue && !params.isRecovery && insertResult.ownerXID != ownerXID) ||
		loaded

	if shouldSkip {
		return &workflowPollingHandle[any]{baseWorkflowHandle: baseWorkflowHandle{workflowID: workflowID, runtime: rt}}, nil
	}

	// Create workflow state
	wfState := &workflowState{workflowID: workflowID}
	wfState.stepID.Store(-1)
	wfCtx := context.WithValue(rt.ctx, workflowStateKey, wfState)

	// Apply deadline if set
	var cancelTimeout context.CancelFunc
	if insertResult.timeout > 0 && insertResult.workflowDeadline.IsZero() {
		wfCtx, cancelTimeout = context.WithTimeout(wfCtx, insertResult.timeout)
	} else if !insertResult.workflowDeadline.IsZero() {
		wfCtx, cancelTimeout = context.WithDeadline(wfCtx, insertResult.workflowDeadline)
	}

	outcomeChan := make(chan workflowOutcome[any], 1)
	rt.workflowsWg.Add(1)
	go func() {
		defer rt.workflowsWg.Done()
		if cancelTimeout != nil {
			defer cancelTimeout()
		}
		rt.activeWorkflowIDs.Store(workflowID, struct{}{})
		defer rt.activeWorkflowIDs.Delete(workflowID)

		result, fnErr := fn(wfCtx, rt, input)

		// Handle workflow ID conflict — another goroutine owns this workflow ID.
		// Wait for the existing workflow to complete and return its result.
		if errors.Is(fnErr, &DBOSError{Code: ConflictingIDError}) {
			rt.logger.Warn("workflow ID conflict, waiting for existing workflow", "workflow_id", workflowID)
			encoded, awaitErr := retryWithResult(rt.ctx, func() (*string, error) {
				return rt.systemDB.awaitWorkflowResult(rt.ctx, workflowID, _DB_RETRY_INTERVAL)
			}, withRetrierLogger(rt.logger))
			outcomeChan <- workflowOutcome[any]{result: encoded, err: awaitErr}
			close(outcomeChan)
			return
		}

		outcomeStatus := WorkflowStatusSuccess
		if fnErr != nil {
			outcomeStatus = WorkflowStatusError
		}

		// Serialize output
		ser := newJSONSerializer[any]()
		encodedOutput, serErr := ser.Encode(result)
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
		}, withRetrierLogger(rt.logger))
		if recordErr != nil {
			outcomeChan <- workflowOutcome[any]{err: recordErr}
			close(outcomeChan)
			return
		}

		outcomeChan <- workflowOutcome[any]{result: result, err: fnErr}
		close(outcomeChan)
	}()

	return &workflowHandle[any]{
		baseWorkflowHandle: baseWorkflowHandle{workflowID: workflowID, runtime: rt},
		outcomeChan:        outcomeChan,
	}, nil
}

/******************************/
/******* STEP EXECUTION *******/
/******************************/

const (
	_DEFAULT_STEP_BASE_INTERVAL  = 100 * time.Millisecond
	_DEFAULT_STEP_MAX_INTERVAL   = 5 * time.Second
	_DEFAULT_STEP_BACKOFF_FACTOR = 2.0
)

type stepOptions struct {
	maxRetries    int
	backoffFactor float64
	baseInterval  time.Duration
	maxInterval   time.Duration
	stepName      string
	preStepID     *int
}

func (opts *stepOptions) setDefaults() {
	if opts.backoffFactor == 0 {
		opts.backoffFactor = _DEFAULT_STEP_BACKOFF_FACTOR
	}
	if opts.baseInterval == 0 {
		opts.baseInterval = _DEFAULT_STEP_BASE_INTERVAL
	}
	if opts.maxInterval == 0 {
		opts.maxInterval = _DEFAULT_STEP_MAX_INTERVAL
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

func WithNextStepID(stepID int) StepOption {
	return func(opts *stepOptions) { opts.preStepID = &stepID }
}

// RunAsStep executes a function as a durable step within a workflow.
func RunAsStep[R any](ctx context.Context, rt *Runtime, fn Step[R], opts ...StepOption) (R, error) {
	stepName := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	opts = append(opts, WithStepName(stepName))

	typeErased := StepFunc(func(ctx context.Context) (any, error) { return fn(ctx) })

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
		ser := newJSONSerializer[R]()
		return ser.Decode(encoded)
	}

	if typed, ok := result.(R); ok {
		return typed, err
	}
	return *new(R), fmt.Errorf("unexpected step result type %T", result)
}

type stepCheckpointedOutcome struct {
	value any
}

func runAsStepInternal(ctx context.Context, rt *Runtime, fn StepFunc, opts ...StepOption) (any, error) {
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

	// Check if already executed
	recorded, err := retryWithResult(ctx, func() (*recordedResult, error) {
		return rt.systemDB.checkOperationExecution(ctx, checkOperationExecutionDBInput{
			workflowUUID: wfState.workflowID,
			functionID:   stepID,
		})
	}, withRetrierLogger(rt.logger))
	if err != nil {
		return nil, fmt.Errorf("checking step execution: %w", err)
	}
	if recorded != nil {
		var stepErr error
		if recorded.errorMsg != nil && *recorded.errorMsg != "" {
			stepErr = errors.New(*recorded.errorMsg)
		}
		return stepCheckpointedOutcome{value: recorded.output}, stepErr
	}

	// Execute with retry
	stepState := &workflowState{workflowID: wfState.workflowID, isWithinStep: true}
	stepState.stepID.Store(int64(stepID))
	stepCtx := context.WithValue(ctx, workflowStateKey, stepState)
	startTime := time.Now()

	stepOutput, stepErr := executeStepWithRetry(ctx, rt, stepOpts, func() (any, error) { return fn(stepCtx) })

	// Serialize and record
	ser := newJSONSerializer[any]()
	encodedOutput, serErr := ser.Encode(stepOutput)
	if serErr != nil {
		return nil, fmt.Errorf("failed to serialize step output: %w", serErr)
	}

	endTime := time.Now()
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
	}, withRetrierLogger(rt.logger))
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
		rt.logger.Error("step failed, retrying", "step_name", opts.stepName, "retry", retry, "delay", delay)
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
	return output, newMaxStepRetriesExceededError("", opts.stepName, opts.maxRetries, joinedErrors)
}

// Go runs a step in a goroutine. Must be within a workflow.
func Go[R any](ctx context.Context, rt *Runtime, fn Step[R], opts ...StepOption) (chan StepOutcome[R], error) {
	wfState, ok := ctx.Value(workflowStateKey).(*workflowState)
	if !ok || wfState == nil {
		return nil, fmt.Errorf("Go must be called within a workflow")
	}
	opts = append(opts, WithNextStepID(wfState.nextStepID()))

	ch := make(chan StepOutcome[R], 1)
	go func() {
		defer close(ch)
		res, err := RunAsStep(ctx, rt, fn, opts...)
		ch <- StepOutcome[R]{Result: res, Err: err}
	}()
	return ch, nil
}

/*****************************************/
/******* WORKFLOW COMMUNICATIONS *********/
/*****************************************/

// Send sends a message to another workflow.
func Send(ctx context.Context, rt *Runtime, destinationID string, message any, topic string) error {
	ser := newJSONSerializer[any]()
	encoded, err := ser.Encode(message)
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// If within a workflow, record as step
	wfState, ok := ctx.Value(workflowStateKey).(*workflowState)
	if ok && wfState != nil {
		if wfState.isWithinStep {
			return fmt.Errorf("cannot call Send within a step")
		}
		_, err = RunAsStep(ctx, rt, func(ctx context.Context) (any, error) {
			return nil, rt.systemDB.send(ctx, WorkflowSendInput{
				DestinationUUID: destinationID,
				Topic:           topic,
				Message:         encoded,
			})
		}, WithStepName("DBOS.send"))
		return err
	}

	return retry(ctx, func() error {
		return rt.systemDB.send(ctx, WorkflowSendInput{
			DestinationUUID: destinationID,
			Topic:           topic,
			Message:         encoded,
		})
	}, withRetrierLogger(rt.logger))
}

// Recv receives a message within a workflow.
func Recv[R any](ctx context.Context, rt *Runtime, topic string, timeout time.Duration) (R, error) {
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
	}, withRetrierLogger(rt.logger))
	if err != nil {
		return *new(R), err
	}
	if encoded == nil {
		return *new(R), nil
	}
	ser := newJSONSerializer[R]()
	return ser.Decode(encoded)
}

// SetEvent sets a key-value event for the current workflow.
func SetEvent(ctx context.Context, rt *Runtime, key string, value any) error {
	ser := newJSONSerializer[any]()
	encoded, err := ser.Encode(value)
	if err != nil {
		return fmt.Errorf("failed to serialize event value: %w", err)
	}

	wfState, ok := ctx.Value(workflowStateKey).(*workflowState)
	if !ok || wfState == nil {
		return fmt.Errorf("SetEvent must be called within a workflow")
	}

	_, err = RunAsStep(ctx, rt, func(ctx context.Context) (any, error) {
		return nil, rt.systemDB.setEvent(ctx, WorkflowSetEventInput{
			WorkflowUUID: wfState.workflowID,
			Key:          key,
			Value:        encoded,
		})
	}, WithStepName("DBOS.setEvent"))
	return err
}

// GetEvent gets a key-value event from a target workflow.
func GetEvent[R any](ctx context.Context, rt *Runtime, targetWorkflowID string, key string, timeout time.Duration) (R, error) {
	encoded, err := retryWithResult(ctx, func() (*string, error) {
		return rt.systemDB.getEvent(ctx, getEventInput{
			targetWorkflowUUID: targetWorkflowID,
			key:                key,
			timeout:            timeout,
		})
	}, withRetrierLogger(rt.logger))
	if err != nil {
		return *new(R), err
	}
	if encoded == nil {
		return *new(R), nil
	}
	ser := newJSONSerializer[R]()
	return ser.Decode(encoded)
}

// Sleep performs a durable sleep within a workflow.
// The wake-up time is recorded as a step so that on recovery,
// if the wake-up time has already passed, Sleep returns immediately;
// otherwise it sleeps only the remaining time.
func Sleep(ctx context.Context, rt *Runtime, duration time.Duration) error {
	wfState, ok := ctx.Value(workflowStateKey).(*workflowState)
	if !ok || wfState == nil {
		return fmt.Errorf("Sleep must be called within a workflow")
	}
	if wfState.isWithinStep {
		return fmt.Errorf("cannot call Sleep within a step")
	}

	// The step records the wake-up time in millis. On first run, the step body
	// executes the sleep. On replay, we get the stored wake-up time back and
	// sleep only the remaining duration.
	wakeUpMs, err := RunAsStep(ctx, rt, func(ctx context.Context) (int64, error) {
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
	}, WithStepName("DBOS.sleep"))
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

/****************************************/
/******* WORKFLOW MANAGEMENT ***********/
/****************************************/

// GetWorkflowID returns the current workflow ID from context.
func GetWorkflowID(ctx context.Context) (string, error) {
	wfState, ok := ctx.Value(workflowStateKey).(*workflowState)
	if !ok || wfState == nil {
		return "", fmt.Errorf("not within a workflow")
	}
	return wfState.workflowID, nil
}

// RetrieveWorkflow returns a handle to an existing workflow.
func RetrieveWorkflow[R any](rt *Runtime, workflowID string) WorkflowHandle[R] {
	return &workflowPollingHandle[R]{baseWorkflowHandle: baseWorkflowHandle{workflowID: workflowID, runtime: rt}}
}

// CancelWorkflow cancels a workflow by ID.
func CancelWorkflow(rt *Runtime, workflowID string) error {
	return retry(rt.ctx, func() error {
		return rt.systemDB.cancelWorkflow(rt.ctx, cancelWorkflowDBInput{workflowID: workflowID})
	}, withRetrierLogger(rt.logger))
}

// ResumeWorkflow resumes a cancelled workflow.
func ResumeWorkflow(rt *Runtime, workflowID string) error {
	return retry(rt.ctx, func() error {
		return rt.systemDB.resumeWorkflow(rt.ctx, resumeWorkflowDBInput{
			workflowID: workflowID,
			executorID: rt.executorID,
			appVersion: rt.applicationVersion,
		})
	}, withRetrierLogger(rt.logger))
}

// ListWorkflows returns workflows matching the given filters.
func ListWorkflows(rt *Runtime, input listWorkflowsDBInput) ([]WorkflowStatus, error) {
	return retryWithResult(rt.ctx, func() ([]WorkflowStatus, error) {
		return rt.systemDB.listWorkflows(rt.ctx, input)
	}, withRetrierLogger(rt.logger))
}

// GetWorkflowSteps returns the execution steps for a workflow.
func GetWorkflowSteps(rt *Runtime, workflowID string) ([]StepInfo, error) {
	steps, err := retryWithResult(rt.ctx, func() ([]stepInfo, error) {
		return rt.systemDB.getWorkflowSteps(rt.ctx, getWorkflowStepsInput{workflowID: workflowID})
	}, withRetrierLogger(rt.logger))
	if err != nil {
		return nil, err
	}
	result := make([]StepInfo, len(steps))
	for i, s := range steps {
		result[i] = StepInfo{
			WorkflowUUID: s.workflowUUID,
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
