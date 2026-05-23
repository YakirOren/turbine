package turbine

import (
	"sync"
)

const (
	defaultMaxTasksPerIteration = 100
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
	workflowQueueRegistry map[string]WorkflowQueue
	queueGoroutinesWg     sync.WaitGroup
	completionChan        chan struct{}
}

func newQueueRunner() *queueRunner {
	return &queueRunner{
		workflowQueueRegistry: make(map[string]WorkflowQueue),
		completionChan:        make(chan struct{}, 1),
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
		queuesToListen[ptInternalQueueName] = qr.workflowQueueRegistry[ptInternalQueueName]
	}

	for _, q := range queuesToListen {
		qr.queueGoroutinesWg.Add(1)
		go qr.runQueue(rt, q)
	}

	qr.queueGoroutinesWg.Wait()
	rt.app.Logger().Debug("all queue goroutines completed")
	qr.completionChan <- struct{}{}
}

func (qr *queueRunner) runQueue(rt *Runtime, queue WorkflowQueue) {
	defer qr.queueGoroutinesWg.Done()
	queueLogger := rt.app.Logger().With("queue_name", queue.Name)
	for {
		if rt.draining.Load() {
			queueLogger.Debug("queue goroutine stopping, runtime is draining")
			return
		}

		func() {
			defer recoverGoroutine(queueLogger, "queue iteration panicked")

			skipDequeue := false

			partitionKeys := []string{""}
			if queue.PartitionQueue {
				parts, err := retryWithResult(rt.ctx, func() ([]string, error) {
					return rt.workflows.getQueuePartitions(rt.ctx, queue.Name)
				}, withRetrierLogger(queueLogger))
				if err != nil {
					skipDequeue = true
					queueLogger.Error("error getting queue partitions", "error", err)
				} else {
					partitionKeys = parts
				}
			}

			if skipDequeue {
				return
			}

			for _, partKey := range partitionKeys {
				limit := queue.MaxTasksPerIteration
				if limit == 0 {
					limit = defaultMaxTasksPerIteration
				}

				workflows, err := retryWithResult(rt.ctx, func() ([]dequeuedWorkflow, error) {
					return rt.workflows.dequeueWorkflows(rt.ctx, dequeueWorkflowsInput{
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
		}()

		// Wait for enqueue event or shutdown
		enqueueCh := rt.messages.subscribeQueue(queue.Name)
		select {
		case <-rt.drainCtx.Done():
			rt.messages.unsubscribeQueue(queue.Name, enqueueCh)
			queueLogger.Debug("queue goroutine stopping", "cause", "draining")
			return
		case <-enqueueCh:
			// Workflow enqueued, immediately dequeue
		}
	}
}

// Queue registers a named queue with the runtime. Must be called before Launch().
func (rt *Runtime) Queue(name string, options ...QueueOption) WorkflowQueue {
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
		MaxTasksPerIteration: defaultMaxTasksPerIteration,
	}
	for _, opt := range options {
		opt(&q)
	}
	rt.queueRunner.workflowQueueRegistry[name] = q
	return q
}

// Listen configures which queues the runtime should poll. Must be called before Launch().
func (rt *Runtime) Listen(queues ...WorkflowQueue) {
	if rt.launched.Load() {
		panic("cannot call Listen after runtime has launched")
	}
	for _, queue := range queues {
		if rq, exists := rt.queueRunner.workflowQueueRegistry[queue.Name]; exists {
			rq.listen = true
			rt.queueRunner.workflowQueueRegistry[queue.Name] = rq
		}
	}
}
