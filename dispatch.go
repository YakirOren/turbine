package turbine

import (
	"encoding/json"

	"github.com/pocketbase/pocketbase/core"
)

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

// reloadDispatchCaches loads webhook and alert channel records into memory.
func (rt *Runtime) reloadDispatchCaches() {
	rt.reloadWebhookCache()
	rt.reloadAlertChannelCache()
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
				rt.reloadWebhookCache()
				return e.Next()
			}
		case collectionAlertChannels:
			reload = func(e *core.RecordEvent) error {
				rt.reloadAlertChannelCache()
				return e.Next()
			}
		}

		rt.app.OnRecordAfterCreateSuccess(col).BindFunc(reload)
		rt.app.OnRecordAfterUpdateSuccess(col).BindFunc(reload)
		rt.app.OnRecordAfterDeleteSuccess(col).BindFunc(reload)
	}
}

// parseEvents extracts a []string from a PocketBase JSON field value.
func parseEvents(raw any) []string {
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var events []string
	_ = json.Unmarshal(b, &events)
	return events
}

// matchesEvent checks if eventName matches any entry in the events list,
// including wildcard patterns like "workflow.*".
func matchesEvent(events []string, eventName string) bool {
	for _, ev := range events {
		if ev == eventName || ev == "workflow.*" {
			return true
		}
	}
	return false
}
