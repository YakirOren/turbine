package pocketflow

func recoverPendingWorkflows(rt *Runtime, executorIDs []string) ([]Handle[any], error) {
	appVersion := []string{}
	if rt.applicationVersion != "" {
		appVersion = []string{rt.applicationVersion}
	}

	pendingWorkflows, err := retryWithResult(rt.ctx, func() ([]Status, error) {
		return rt.systemDB.listWorkflows(rt.ctx, listWorkflowsDBInput{
			status:             []StatusType{StatusPending},
			executorIDs:        executorIDs,
			applicationVersion: appVersion,
			loadInput:          true,
		})
	}, withRetrierLogger(rt.logger))
	if err != nil {
		return nil, err
	}

	var handles []Handle[any]

	for _, wf := range pendingWorkflows {
		if wf.QueueName != "" {
			cleared, err := retryWithResult(rt.ctx, func() (bool, error) {
				return rt.systemDB.clearQueueAssignment(rt.ctx, wf.ID)
			}, withRetrierLogger(rt.logger))
			if err != nil {
				rt.logger.Error("error clearing queue assignment", "workflow_id", wf.ID, "error", err)
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
			rt.logger.Error("workflow not found in registry for recovery", "workflow_name", wf.Name)
			continue
		}

		registeredAny, exists := rt.workflowRegistry.Load(wfFQN.(string))
		if !exists {
			rt.logger.Error("workflow function not found in registry", "workflow_id", wf.ID, "name", wf.Name)
			continue
		}
		registered := registeredAny.(workflowRegistryEntry)

		opts := []WorkflowOption{
			WithWorkflowID(wf.ID),
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
