package pocketflow

import (
	"context"
	"log/slog"
	"sync"
)

const (
	_DEFAULT_MAX_TASKS_PER_ITERATION = 100
)

// WorkflowQueue defines a named queue with concurrency and rate limiting options.
type WorkflowQueue struct {
	Name                 string
	WorkerConcurrency    *int
	GlobalConcurrency    *int
	PriorityEnabled      bool
	RateLimit            *RateLimiter
	MaxTasksPerIteration int
	PartitionQueue       bool

	listen bool
}

// QueueOption configures a workflow queue.
type QueueOption func(*WorkflowQueue)

func WithWorkerConcurrency(n int) QueueOption {
	return func(q *WorkflowQueue) { q.WorkerConcurrency = &n }
}

func WithGlobalConcurrency(n int) QueueOption {
	return func(q *WorkflowQueue) { q.GlobalConcurrency = &n }
}

func WithPriorityEnabled() QueueOption {
	return func(q *WorkflowQueue) { q.PriorityEnabled = true }
}

func WithRateLimiter(limiter RateLimiter) QueueOption {
	return func(q *WorkflowQueue) { q.RateLimit = &limiter }
}

func WithMaxTasksPerIteration(n int) QueueOption {
	return func(q *WorkflowQueue) { q.MaxTasksPerIteration = n }
}

func WithPartitionQueue() QueueOption {
	return func(q *WorkflowQueue) { q.PartitionQueue = true }
}


type queueRunner struct {
	logger              *slog.Logger
	workflowQueueRegistry map[string]WorkflowQueue
	queueGoroutinesWg   sync.WaitGroup
	completionChan      chan struct{}
}

func newQueueRunner(logger *slog.Logger) *queueRunner {
	return &queueRunner{
		logger:              logger,
		workflowQueueRegistry: make(map[string]WorkflowQueue),
		completionChan:      make(chan struct{}, 1),
	}
}

func (qr *queueRunner) getQueue(name string) *WorkflowQueue {
	q, ok := qr.workflowQueueRegistry[name]
	if !ok {
		return nil
	}
	return &q
}

func (qr *queueRunner) listQueues() []WorkflowQueue {
	queues := make([]WorkflowQueue, 0, len(qr.workflowQueueRegistry))
	for _, q := range qr.workflowQueueRegistry {
		queues = append(queues, q)
	}
	return queues
}

func (qr *queueRunner) run(rt *Runtime) {
	queuesToListen := make(map[string]WorkflowQueue)
	for _, q := range qr.workflowQueueRegistry {
		if q.listen {
			queuesToListen[q.Name] = q
		}
	}
	// Default: listen to all
	if len(queuesToListen) == 0 {
		queuesToListen = qr.workflowQueueRegistry
	} else {
		queuesToListen[_PF_INTERNAL_QUEUE_NAME] = qr.workflowQueueRegistry[_PF_INTERNAL_QUEUE_NAME]
	}

	for _, q := range queuesToListen {
		qr.queueGoroutinesWg.Add(1)
		go qr.runQueue(rt, q)
	}

	qr.queueGoroutinesWg.Wait()
	qr.logger.Debug("all queue goroutines completed")
	qr.completionChan <- struct{}{}
}

func (qr *queueRunner) runQueue(rt *Runtime, queue WorkflowQueue) {
	defer qr.queueGoroutinesWg.Done()
	queueLogger := qr.logger.With("queue_name", queue.Name)
	for {
		skipDequeue := false

		partitionKeys := []string{""}
		if queue.PartitionQueue {
			parts, err := retryWithResult(rt.ctx, func() ([]string, error) {
				return rt.systemDB.getQueuePartitions(rt.ctx, queue.Name)
			}, withRetrierLogger(queueLogger))
			if err != nil {
				skipDequeue = true
				queueLogger.Error("error getting queue partitions", "error", err)
			} else {
				partitionKeys = parts
			}
		}

		if !skipDequeue {
			for _, partKey := range partitionKeys {
				limit := queue.MaxTasksPerIteration
				if limit == 0 {
					limit = _DEFAULT_MAX_TASKS_PER_ITERATION
				}

				workflows, err := retryWithResult(rt.ctx, func() ([]dequeuedWorkflow, error) {
					return rt.systemDB.dequeueWorkflows(rt.ctx, dequeueWorkflowsInput{
						queueName:         queue.Name,
						executorID:        rt.executorID,
						appVersion:        rt.applicationVersion,
						limit:             limit,
						workerConcurrency: queue.WorkerConcurrency,
						globalConcurrency: queue.GlobalConcurrency,
						priorityEnabled:   queue.PriorityEnabled,
						rateLimit:         queue.RateLimit,
						partitioned:       queue.PartitionQueue,
						partitionKey:      partKey,
					})
				}, withRetrierLogger(queueLogger))
				if err != nil {
					queueLogger.Error("error dequeuing workflows", "error", err)
					continue
				}

				for _, wf := range workflows {
					wfFQN, ok := rt.workflowCustomNameToFQN.Load(wf.name)
					if !ok {
						queueLogger.Error("workflow not found in registry", "workflow_name", wf.name)
						continue
					}
					registeredAny, exists := rt.workflowRegistry.Load(wfFQN.(string))
					if !exists {
						queueLogger.Error("workflow not in registry", "workflow_name", wf.name)
						continue
					}
					registered := registeredAny.(workflowRegistryEntry)
					_, err := registered.wrappedFunction(rt, wf.input, WithID(wf.workflowID), withIsDequeue())
					if err != nil {
						queueLogger.Error("error running queued workflow", "error", err)
					}
				}
			}
		}

		// Wait for enqueue event or shutdown
		enqueueCh := rt.systemDB.waitForEnqueue(rt.ctx, queue.Name)
		select {
		case <-rt.ctx.Done():
			rt.systemDB.stopWaitForEnqueue(queue.Name, enqueueCh)
			queueLogger.Debug("queue goroutine stopping", "cause", context.Cause(rt.ctx))
			return
		case <-enqueueCh:
			// Workflow enqueued — immediately dequeue
		}
	}
}

// NewWorkflowQueue registers a named queue with the runtime. Must be called before Launch().
func NewWorkflowQueue(rt *Runtime, name string, options ...QueueOption) WorkflowQueue {
	if rt.launched.Load() {
		panic("cannot register queue after runtime has launched")
	}
	return newWorkflowQueue(rt, name, options...)
}

func newWorkflowQueue(rt *Runtime, name string, options ...QueueOption) WorkflowQueue {
	if _, exists := rt.queueRunner.workflowQueueRegistry[name]; exists {
		panic("queue already registered: " + name)
	}

	q := WorkflowQueue{
		Name:                 name,
		MaxTasksPerIteration: _DEFAULT_MAX_TASKS_PER_ITERATION,
	}
	for _, opt := range options {
		opt(&q)
	}
	rt.queueRunner.workflowQueueRegistry[name] = q
	return q
}

// ListenQueues configures which queues the runtime should poll. Must be called before Launch().
func ListenQueues(rt *Runtime, queues ...WorkflowQueue) {
	if rt.launched.Load() {
		panic("cannot call ListenQueues after runtime has launched")
	}
	for _, queue := range queues {
		if rq, exists := rt.queueRunner.workflowQueueRegistry[queue.Name]; exists {
			rq.listen = true
			rt.queueRunner.workflowQueueRegistry[queue.Name] = rq
		}
	}
}
