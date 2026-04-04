package turbine

import (
	"github.com/pocketbase/pocketbase/core"
)

const (
	collectionStatus             = "pt_workflow_status"
	collectionOperationOutputs   = "pt_operation_outputs"
	collectionNotifications      = "pt_notifications"
	collectionWorkflowEvents     = "pt_workflow_events"
	collectionWorkflowEventsHist = "pt_workflow_events_history"
	collectionSchedules          = "pt_schedules"
	collectionProducts           = "pt_products"
	collectionKV                 = "pt_kv"
	collectionWebhooks           = "pt_webhooks"
	collectionWorkflows          = "pt_workflows"
	collectionAlertChannels      = "pt_alert_channels"
)

func init() {
	core.AppMigrations.Register(upCreateCollections, downCreateCollections, "pt_1_create_collections")
}

func init() {
	core.AppMigrations.Register(upCreateProducts, downCreateProducts, "pt_2_create_products")
}

func upCreateProducts(app core.App) error {
	wfStatus, err := app.FindCollectionByNameOrId(collectionStatus)
	if err != nil {
		return err
	}

	products := core.NewBaseCollection(collectionProducts)
	products.Fields.Add(
		&core.RelationField{Name: "workflow_id", CollectionId: wfStatus.Id, Required: true, CascadeDelete: true, MaxSelect: 1},
		&core.NumberField{Name: "function_id"},
		&core.TextField{Name: "function_name"},
		&core.FileField{Name: "file", MaxSize: 0, MaxSelect: 1},
		&core.TextField{Name: "file_name", Required: true},
		&core.JSONField{Name: "metadata"},
		&core.NumberField{Name: "size"},
		&core.TextField{Name: "status", Required: true},
		&core.TextField{Name: "error"},
		&core.AutodateField{Name: "created", OnCreate: true},
	)
	products.AddIndex("idx_products_workflow", false, "workflow_id", "")
	products.AddIndex("idx_products_dedup", true, "workflow_id, function_id, file_name", "")
	return app.Save(products)
}

func downCreateProducts(app core.App) error {
	col, err := app.FindCollectionByNameOrId(collectionProducts)
	if err != nil {
		return nil
	}
	return app.Delete(col)
}

func init() {
	core.AppMigrations.Register(upCreateKV, downCreateKV, "pt_3_create_kv")
}

func upCreateKV(app core.App) error {
	kv := core.NewBaseCollection(collectionKV)
	kv.Fields.Add(
		&core.TextField{Name: "key", Required: true},
		&core.JSONField{Name: "value", Required: true},
		&core.NumberField{Name: "updated_at_epoch_ms"},
	)
	kv.AddIndex("idx_kv_key", true, "key", "")
	return app.Save(kv)
}

func downCreateKV(app core.App) error {
	col, err := app.FindCollectionByNameOrId(collectionKV)
	if err != nil {
		return nil
	}
	return app.Delete(col)
}

func init() {
	core.AppMigrations.Register(func(app core.App) error {
		col, err := app.FindCollectionByNameOrId(collectionStatus)
		if err != nil {
			return err
		}
		col.Fields.Add(&core.JSONField{Name: "tags"})
		return app.Save(col)
	}, func(app core.App) error {
		col, err := app.FindCollectionByNameOrId(collectionStatus)
		if err != nil {
			return nil
		}
		col.Fields.RemoveByName("tags")
		return app.Save(col)
	}, "pt_4_add_tags")
}

func init() {
	core.AppMigrations.Register(upCreateWebhooks, downCreateWebhooks, "pt_5_create_webhooks")
}

func upCreateWebhooks(app core.App) error {
	webhooks := core.NewBaseCollection(collectionWebhooks)
	webhooks.Fields.Add(
		&core.TextField{Name: "url", Required: true},
		&core.JSONField{Name: "events", Required: true},
		&core.BoolField{Name: "enabled"},
		&core.TextField{Name: "secret"},
		&core.AutodateField{Name: "created", OnCreate: true},
	)
	return app.Save(webhooks)
}

func downCreateWebhooks(app core.App) error {
	col, err := app.FindCollectionByNameOrId(collectionWebhooks)
	if err != nil {
		return nil
	}
	return app.Delete(col)
}

func init() {
	core.AppMigrations.Register(func(app core.App) error {
		col, err := app.FindCollectionByNameOrId(collectionKV)
		if err != nil {
			return err
		}
		col.Fields.Add(&core.JSONField{Name: "schema"})
		return app.Save(col)
	}, func(app core.App) error {
		col, err := app.FindCollectionByNameOrId(collectionKV)
		if err != nil {
			return nil
		}
		col.Fields.RemoveByName("schema")
		return app.Save(col)
	}, "pt_6_add_kv_schema")

	core.AppMigrations.Register(func(app core.App) error {
		col, err := app.FindCollectionByNameOrId(collectionSchedules)
		if err != nil {
			return err
		}
		col.Fields.Add(&core.BoolField{Name: "enabled"})
		if err := app.Save(col); err != nil {
			return err
		}
		// Enable all existing schedules by default.
		if _, err := app.DB().NewQuery("UPDATE " + collectionSchedules + " SET enabled = TRUE").Execute(); err != nil {
			return err
		}
		return nil
	}, func(app core.App) error {
		col, err := app.FindCollectionByNameOrId(collectionSchedules)
		if err == nil {
			col.Fields.RemoveByName("enabled")
			_ = app.Save(col)
		}
		return nil
	}, "pt_7_add_schedule_enabled")
}

func init() {
	core.AppMigrations.Register(upCreateWorkflows, downCreateWorkflows, "pt_8_create_workflows")
}

func upCreateWorkflows(app core.App) error {
	workflows := core.NewBaseCollection(collectionWorkflows)
	workflows.Fields.Add(
		&core.TextField{Name: "name", Required: true},
		&core.TextField{Name: "fqn", Required: true},
		&core.BoolField{Name: "triggerable"},
		&core.TextField{Name: "cron_schedule"},
		&core.JSONField{Name: "input_schema"},
		&core.JSONField{Name: "tags"},
	)
	workflows.AddIndex("idx_workflows_fqn", true, "fqn", "")
	return app.Save(workflows)
}

func downCreateWorkflows(app core.App) error {
	col, err := app.FindCollectionByNameOrId(collectionWorkflows)
	if err != nil {
		return nil
	}
	return app.Delete(col)
}

func init() {
	core.AppMigrations.Register(func(app core.App) error {
		col, err := app.FindCollectionByNameOrId(collectionStatus)
		if err != nil {
			return err
		}
		col.Fields.Add(&core.TextField{Name: "summary"})
		return app.Save(col)
	}, func(app core.App) error {
		col, err := app.FindCollectionByNameOrId(collectionStatus)
		if err != nil {
			return nil
		}
		col.Fields.RemoveByName("summary")
		return app.Save(col)
	}, "pt_9_add_summary")
}

func init() {
	core.AppMigrations.Register(upCreateAlertChannels, downCreateAlertChannels, "pt_10_create_alert_channels")
}

func upCreateAlertChannels(app core.App) error {
	alertChannels := core.NewBaseCollection(collectionAlertChannels)
	alertChannels.Fields.Add(
		&core.TextField{Name: "name", Required: true},
		&core.TextField{Name: "url", Required: true},
		&core.JSONField{Name: "events", Required: true},
		&core.BoolField{Name: "enabled"},
		&core.AutodateField{Name: "created", OnCreate: true},
	)
	return app.Save(alertChannels)
}

func downCreateAlertChannels(app core.App) error {
	col, err := app.FindCollectionByNameOrId(collectionAlertChannels)
	if err != nil {
		return nil
	}
	return app.Delete(col)
}

func init() {
	core.AppMigrations.Register(func(app core.App) error {
		col, err := app.FindCollectionByNameOrId(collectionStatus)
		if err != nil {
			return nil // collection created by pt_1 which includes these indexes
		}
		col.AddIndex("idx_workflow_status_queue_status", false, "queue_name, status", "")
		col.AddIndex("idx_workflow_status_created", false, "created_at_epoch_ms", "")
		col.AddIndex("idx_workflow_status_queue_partition", false, "queue_name, status, queue_partition_key", "")
		return app.Save(col)
	}, func(app core.App) error {
		col, err := app.FindCollectionByNameOrId(collectionStatus)
		if err != nil {
			return nil
		}
		col.RemoveIndex("idx_workflow_status_queue_status")
		col.RemoveIndex("idx_workflow_status_created")
		col.RemoveIndex("idx_workflow_status_queue_partition")
		return app.Save(col)
	}, "pt_11_add_performance_indexes")
}

func upCreateCollections(app core.App) error {
	// 1. Workflow status
	wfStatus := core.NewBaseCollection(collectionStatus)
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
		&core.NumberField{Name: "parent_function_id"},
		&core.TextField{Name: "app_status"},
		&core.TextField{Name: "app_status_color"},
	)
	wfStatus.AddIndex("idx_workflow_status_executor", false, "executor_id, status, application_version", "")
	wfStatus.AddIndex("idx_workflow_status_dedup", true, "queue_name, deduplication_id", "deduplication_id != ''")
	wfStatus.AddIndex("idx_workflow_status_queue_status", false, "queue_name, status", "")
	wfStatus.AddIndex("idx_workflow_status_created", false, "created_at_epoch_ms", "")
	wfStatus.AddIndex("idx_workflow_status_queue_partition", false, "queue_name, status, queue_partition_key", "")
	if err := app.Save(wfStatus); err != nil {
		return err
	}

	// Add self-referencing relation fields (must be done after initial save)
	wfStatus.Fields.Add(
		&core.RelationField{Name: "forked_from_workflow_id", CollectionId: wfStatus.Id, MaxSelect: 1},
		&core.RelationField{Name: "parent_workflow_id", CollectionId: wfStatus.Id, MaxSelect: 1},
	)
	if err := app.Save(wfStatus); err != nil {
		return err
	}

	// 2. Operation outputs
	opOutputs := core.NewBaseCollection(collectionOperationOutputs)
	opOutputs.Fields.Add(
		&core.RelationField{Name: "workflow_id", CollectionId: wfStatus.Id, Required: true, CascadeDelete: true, MaxSelect: 1},
		&core.NumberField{Name: "function_id", Required: true},
		&core.JSONField{Name: "output"},
		&core.TextField{Name: "error"},
		&core.TextField{Name: "child_workflow_id"},
		&core.TextField{Name: "function_name"},
		&core.NumberField{Name: "started_at_epoch_ms"},
		&core.NumberField{Name: "ended_at_epoch_ms"},
	)
	opOutputs.AddIndex("idx_operation_outputs_pk", true, "workflow_id, function_id", "")
	if err := app.Save(opOutputs); err != nil {
		return err
	}

	// 3. Notifications
	notifs := core.NewBaseCollection(collectionNotifications)
	notifs.Fields.Add(
		&core.RelationField{Name: "destination_id", CollectionId: wfStatus.Id, Required: true, CascadeDelete: true, MaxSelect: 1},
		&core.TextField{Name: "topic"},
		&core.JSONField{Name: "message"},
		&core.NumberField{Name: "created_at_epoch_ms"},
		&core.BoolField{Name: "consumed"},
	)
	notifs.AddIndex("idx_notifications_dest_topic", false, "destination_id, topic", "consumed = FALSE")
	if err := app.Save(notifs); err != nil {
		return err
	}

	// 4. Workflow events
	wfEvents := core.NewBaseCollection(collectionWorkflowEvents)
	wfEvents.Fields.Add(
		&core.RelationField{Name: "workflow_id", CollectionId: wfStatus.Id, Required: true, CascadeDelete: true, MaxSelect: 1},
		&core.TextField{Name: "key", Required: true},
		&core.JSONField{Name: "value"},
	)
	wfEvents.AddIndex("idx_workflow_events_pk", true, "workflow_id, key", "")
	if err := app.Save(wfEvents); err != nil {
		return err
	}

	// 5. Workflow events history
	wfEventsHist := core.NewBaseCollection(collectionWorkflowEventsHist)
	wfEventsHist.Fields.Add(
		&core.RelationField{Name: "workflow_id", CollectionId: wfStatus.Id, Required: true, CascadeDelete: true, MaxSelect: 1},
		&core.NumberField{Name: "function_id", Required: true},
		&core.TextField{Name: "key", Required: true},
		&core.JSONField{Name: "value"},
	)
	wfEventsHist.AddIndex("idx_workflow_events_history_pk", true, "workflow_id, function_id, key", "")
	if err := app.Save(wfEventsHist); err != nil {
		return err
	}

	// 6. Schedules (UI-created cron and one-time triggers)
	schedules := core.NewBaseCollection(collectionSchedules)
	schedules.Fields.Add(
		&core.TextField{Name: "workflow_fqn", Required: true},
		&core.JSONField{Name: "input"},
		&core.TextField{Name: "type", Required: true},
		&core.TextField{Name: "cron_expression"},
		&core.TextField{Name: "jitter"},
		&core.DateField{Name: "scheduled_at"},
		&core.BoolField{Name: "enabled"},
	)
	schedules.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
	schedules.AddIndex("idx_schedules_type", false, "type", "")
	if err := app.Save(schedules); err != nil {
		return err
	}

	return nil
}

func downCreateCollections(app core.App) error {
	names := []string{
		collectionSchedules,
		collectionWorkflowEventsHist,
		collectionWorkflowEvents,
		collectionNotifications,
		collectionOperationOutputs,
		collectionStatus,
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
