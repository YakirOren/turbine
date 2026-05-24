package turbine

import (
	"github.com/YakirOren/turbine/internal/dispatch"
	"github.com/YakirOren/turbine/internal/retry"
	"github.com/pocketbase/pocketbase/core"
)

// dispatchEvent sends webhooks and notifications for a workflow event.
// Callers should invoke this in a goroutine: go rt.dispatchEvent(...)
func (rt *Runtime) dispatchEvent(workflowID, name string, status StatusType, output *string, errorMsg *string) {
	defer retry.RecoverGoroutine(rt.app.Logger(), "dispatch panic recovered", "workflow_id", workflowID)
	rt.webhooks.Dispatch(workflowID, name, string(status), output, errorMsg, dispatch.MatchesEvent)
	rt.notifications.Dispatch(workflowID, name, status, errorMsg, dispatch.MatchesEvent)
}

// reloadDispatchCaches loads webhook and alert channel records into memory.
func (rt *Runtime) reloadDispatchCaches() {
	rt.webhooks.ReloadCache()
	rt.notifications.ReloadCache()
}

// registerDispatchHooks registers PocketBase hooks that invalidate the
// webhook and alert channel caches when records are created, updated, or deleted.
func (rt *Runtime) registerDispatchHooks() {
	for _, collection := range []string{collectionWebhooks, collectionAlertChannels} {
		col := collection
		reload := func(_ *core.RecordEvent) error { return nil }

		switch col {
		case collectionWebhooks:
			reload = func(e *core.RecordEvent) error {
				rt.webhooks.ReloadCache()
				return e.Next()
			}
		case collectionAlertChannels:
			reload = func(e *core.RecordEvent) error {
				rt.notifications.ReloadCache()
				return e.Next()
			}
		}

		rt.app.OnRecordAfterCreateSuccess(col).BindFunc(reload)
		rt.app.OnRecordAfterUpdateSuccess(col).BindFunc(reload)
		rt.app.OnRecordAfterDeleteSuccess(col).BindFunc(reload)
	}
}

