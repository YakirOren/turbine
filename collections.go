package pbdbos

import (
	"github.com/pocketbase/pocketbase/core"
)

const (
	collectionWorkflowStatus       = "dbos_workflow_status"
	collectionOperationOutputs     = "dbos_operation_outputs"
	collectionNotifications        = "dbos_notifications"
	collectionWorkflowEvents       = "dbos_workflow_events"
	collectionWorkflowEventsHist   = "dbos_workflow_events_history"
)

func ensureCollections(app core.App) error {
	collections := []func(core.App) (*core.Collection, error){
		workflowStatusCollection,
		operationOutputsCollection,
		notificationsCollection,
		workflowEventsCollection,
		workflowEventsHistoryCollection,
	}

	for _, createFn := range collections {
		col, err := createFn(app)
		if err != nil {
			return err
		}

		// Check if already exists
		if _, findErr := app.FindCollectionByNameOrId(col.Name); findErr == nil {
			continue
		}

		if err := app.Save(col); err != nil {
			return err
		}
	}

	// Create composite unique indexes via raw SQL (PocketBase doesn't support composite PKs natively)
	indexes := []string{
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_operation_outputs_pk ON dbos_operation_outputs (workflow_uuid, function_id)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_workflow_events_pk ON dbos_workflow_events (workflow_uuid, key)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_workflow_events_history_pk ON dbos_workflow_events_history (workflow_uuid, function_id, key)",
		"CREATE INDEX IF NOT EXISTS idx_notifications_dest_topic ON dbos_notifications (destination_uuid, topic) WHERE consumed = FALSE",
		"CREATE INDEX IF NOT EXISTS idx_workflow_status_executor ON dbos_workflow_status (executor_id, status, application_version)",
	}

	for _, sql := range indexes {
		if _, err := app.DB().NewQuery(sql).Execute(); err != nil {
			return err
		}
	}

	return nil
}

func workflowStatusCollection(app core.App) (*core.Collection, error) {
	col := core.NewBaseCollection(collectionWorkflowStatus)

	col.Fields.Add(
		&core.TextField{Name: "status", Required: true},
		&core.TextField{Name: "name", Required: true},
		&core.TextField{Name: "inputs"},
		&core.TextField{Name: "output"},
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

	return col, nil
}

func operationOutputsCollection(app core.App) (*core.Collection, error) {
	col := core.NewBaseCollection(collectionOperationOutputs)

	col.Fields.Add(
		&core.TextField{Name: "workflow_uuid", Required: true},
		&core.NumberField{Name: "function_id", Required: true},
		&core.TextField{Name: "output"},
		&core.TextField{Name: "error"},
		&core.TextField{Name: "child_workflow_id"},
		&core.TextField{Name: "function_name"},
		&core.NumberField{Name: "started_at_epoch_ms"},
		&core.NumberField{Name: "ended_at_epoch_ms"},
	)

	return col, nil
}

func notificationsCollection(app core.App) (*core.Collection, error) {
	col := core.NewBaseCollection(collectionNotifications)

	col.Fields.Add(
		&core.TextField{Name: "destination_uuid", Required: true},
		&core.TextField{Name: "topic"},
		&core.TextField{Name: "message"},
		&core.NumberField{Name: "created_at_epoch_ms"},
		&core.BoolField{Name: "consumed"},
	)

	return col, nil
}

func workflowEventsCollection(app core.App) (*core.Collection, error) {
	col := core.NewBaseCollection(collectionWorkflowEvents)

	col.Fields.Add(
		&core.TextField{Name: "workflow_uuid", Required: true},
		&core.TextField{Name: "key", Required: true},
		&core.TextField{Name: "value", Required: true},
	)

	return col, nil
}

func workflowEventsHistoryCollection(app core.App) (*core.Collection, error) {
	col := core.NewBaseCollection(collectionWorkflowEventsHist)

	col.Fields.Add(
		&core.TextField{Name: "workflow_uuid", Required: true},
		&core.NumberField{Name: "function_id", Required: true},
		&core.TextField{Name: "key", Required: true},
		&core.TextField{Name: "value", Required: true},
	)

	return col, nil
}
