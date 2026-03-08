package pbdbos

import (
	"github.com/pocketbase/pocketbase/core"
)

const (
	collectionWorkflowStatus     = "dbos_workflow_status"
	collectionOperationOutputs   = "dbos_operation_outputs"
	collectionNotifications      = "dbos_notifications"
	collectionWorkflowEvents     = "dbos_workflow_events"
	collectionWorkflowEventsHist = "dbos_workflow_events_history"
)

func init() {
	core.AppMigrations.Register(upCreateCollections, downCreateCollections, "pbdbos_1_create_collections")
}

func upCreateCollections(app core.App) error {
	// 1. Workflow status
	wfStatus := core.NewBaseCollection(collectionWorkflowStatus)
	wfStatus.Fields.Add(
		&core.TextField{Name: "status", Required: true},
		&core.TextField{Name: "name", Required: true},
		&core.JSONField{Name: "inputs"},
		&core.JSONField{Name: "output"},
		&core.TextField{Name: "error"},
		&core.TextField{Name: "executor_id", Required: true},
		&core.TextField{Name: "application_version"},
		&core.TextField{Name: "application_id"},
		&core.NumberField{Name: "recovery_attempts"},
		&core.NumberField{Name: "created_at_epoch_ms"},
		&core.NumberField{Name: "updated_at_epoch_ms"},
		&core.TextField{Name: "queue_name"},
		&core.TextField{Name: "queue_partition_key"},
		&core.TextField{Name: "deduplication_id"},
		&core.NumberField{Name: "priority"},
		&core.NumberField{Name: "workflow_timeout_ms"},
		&core.NumberField{Name: "workflow_deadline_epoch_ms"},
		&core.TextField{Name: "owner_xid"},
		&core.TextField{Name: "forked_from_workflow_uuid"},
		&core.TextField{Name: "parent_workflow_uuid"},
		&core.NumberField{Name: "parent_function_id"},
	)
	wfStatus.AddIndex("idx_workflow_status_executor", false, "executor_id, status, application_version", "")
	wfStatus.AddIndex("idx_workflow_status_dedup", true, "queue_name, deduplication_id", "deduplication_id != ''")
	if err := app.Save(wfStatus); err != nil {
		return err
	}

	// 2. Operation outputs
	opOutputs := core.NewBaseCollection(collectionOperationOutputs)
	opOutputs.Fields.Add(
		&core.TextField{Name: "workflow_uuid", Required: true},
		&core.NumberField{Name: "function_id", Required: true},
		&core.JSONField{Name: "output"},
		&core.TextField{Name: "error"},
		&core.TextField{Name: "child_workflow_id"},
		&core.TextField{Name: "function_name"},
		&core.NumberField{Name: "started_at_epoch_ms"},
		&core.NumberField{Name: "ended_at_epoch_ms"},
	)
	opOutputs.AddIndex("idx_operation_outputs_pk", true, "workflow_uuid, function_id", "")
	if err := app.Save(opOutputs); err != nil {
		return err
	}

	// 3. Notifications
	notifs := core.NewBaseCollection(collectionNotifications)
	notifs.Fields.Add(
		&core.TextField{Name: "destination_uuid", Required: true},
		&core.TextField{Name: "topic"},
		&core.JSONField{Name: "message"},
		&core.NumberField{Name: "created_at_epoch_ms"},
		&core.BoolField{Name: "consumed"},
	)
	notifs.AddIndex("idx_notifications_dest_topic", false, "destination_uuid, topic", "consumed = FALSE")
	if err := app.Save(notifs); err != nil {
		return err
	}

	// 4. Workflow events
	wfEvents := core.NewBaseCollection(collectionWorkflowEvents)
	wfEvents.Fields.Add(
		&core.TextField{Name: "workflow_uuid", Required: true},
		&core.TextField{Name: "key", Required: true},
		&core.JSONField{Name: "value"},
	)
	wfEvents.AddIndex("idx_workflow_events_pk", true, "workflow_uuid, key", "")
	if err := app.Save(wfEvents); err != nil {
		return err
	}

	// 5. Workflow events history
	wfEventsHist := core.NewBaseCollection(collectionWorkflowEventsHist)
	wfEventsHist.Fields.Add(
		&core.TextField{Name: "workflow_uuid", Required: true},
		&core.NumberField{Name: "function_id", Required: true},
		&core.TextField{Name: "key", Required: true},
		&core.JSONField{Name: "value"},
	)
	wfEventsHist.AddIndex("idx_workflow_events_history_pk", true, "workflow_uuid, function_id, key", "")
	if err := app.Save(wfEventsHist); err != nil {
		return err
	}

	return nil
}

func downCreateCollections(app core.App) error {
	names := []string{
		collectionWorkflowEventsHist,
		collectionWorkflowEvents,
		collectionNotifications,
		collectionOperationOutputs,
		collectionWorkflowStatus,
	}
	for _, name := range names {
		col, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			continue
		}
		if err := app.Delete(col); err != nil {
			return err
		}
	}
	return nil
}
