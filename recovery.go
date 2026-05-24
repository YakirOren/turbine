package turbine

import (
	"github.com/YakirOren/turbine/internal/retry"
)

func recoverPendingWorkflows(rt *Runtime, executorIDs []string) ([]Handle[any], error) {
	appVersion := []string{}
	if rt.applicationVersion != "" {
		appVersion = []string{rt.applicationVersion}
	}

	pendingWorkflows, err := retry.RetryWithResult(rt.ctx, func() ([]Status, error) {
		return rt.workflows.listWorkflows(rt.ctx, listWorkflowsDBInput{
			status:             []StatusType{StatusPending},
			executorIDs:        executorIDs,
			applicationVersion: appVersion,
			loadInput:          true,
		})
	}, retry.WithLogger(rt.app.Logger()))
	if err != nil {
		return nil, err
	}

	var handles []Handle[any]

	for _, wf := range pendingWorkflows {
		if wf.QueueName != "" {
			cleared, err := retry.RetryWithResult(rt.ctx, func() (bool, error) {
				return rt.workflows.clearQueueAssignment(rt.ctx, wf.ID)
			}, retry.WithLogger(rt.app.Logger()))
			if err != nil {
				rt.app.Logger().Error("error clearing queue assignment", "workflow_id", wf.ID, "error", err)
				continue
			}
			if cleared {
				handles = append(handles, &workflowPollingHandle[any]{
					baseHandle: baseHandle{workflowID: wf.ID, runtime: rt},
				})
			}
			continue
		}

		wfFQN, ok := rt.workflowCustomNameToFQN.Load(wf.Name)
		if !ok {
			rt.app.Logger().Error("workflow not found in registry for recovery", "workflow_name", wf.Name)
			continue
		}

		registeredAny, exists := rt.workflowRegistry.Load(wfFQN.(string))
		if !exists {
			rt.app.Logger().Error("workflow function not found in registry", "workflow_id", wf.ID, "name", wf.Name)
			continue
		}
		registered := registeredAny.(workflowRegistryEntry)

		opts := []WorkflowOption{
			WithID(wf.ID),
			withIsRecovery(),
		}
		handle, err := registered.wrappedFunction(rt, wf.Input, opts...)
		if err != nil {
			return nil, err
		}
		handles = append(handles, handle)
	}

	return handles, nil
}
