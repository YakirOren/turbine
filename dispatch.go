package turbine

// dispatchEvent sends webhooks and notifications for a workflow event.
// Callers should invoke this in a goroutine: go rt.dispatchEvent(...)
func (rt *Runtime) dispatchEvent(workflowID, name string, status StatusType, output *string, errorMsg *string) {
	defer func() {
		if r := recover(); r != nil {
			if rt != nil && rt.app != nil {
				rt.app.Logger().Error("dispatch panic recovered", "panic", r, "workflow_id", workflowID, "source", "system")
			}
		}
	}()
	rt.dispatchWebhooks(workflowID, name, status, output, errorMsg)
	rt.dispatchNotifications(workflowID, name, status, errorMsg)
}
