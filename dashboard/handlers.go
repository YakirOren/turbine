package dashboard

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/YakirOren/turbine"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type handlers struct {
	rt  *turbine.Runtime
	app core.App
}

func (h *handlers) cancelWorkflow(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	if id == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "missing workflow id"})
	}
	if err := h.rt.Cancel(id); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *handlers) resumeWorkflow(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	if id == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "missing workflow id"})
	}
	if err := h.rt.Resume(id); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]string{"status": "resumed"})
}

func (h *handlers) listQueues(e *core.RequestEvent) error {
	queues := h.rt.Queues()
	result := make([]map[string]any, 0, len(queues))
	for _, q := range queues {
		if q.Name == "_pt_internal_queue" {
			continue
		}
		entry := map[string]any{
			"name":            q.Name,
			"priorityEnabled": q.PriorityEnabled,
			"partitioned":     q.PartitionQueue,
		}
		if q.WorkerConcurrency != nil {
			entry["workerConcurrency"] = *q.WorkerConcurrency
		}
		if q.GlobalConcurrency != nil {
			entry["globalConcurrency"] = *q.GlobalConcurrency
		}
		if q.RateLimit != nil {
			entry["rateLimit"] = map[string]any{
				"limit":  q.RateLimit.Limit,
				"period": q.RateLimit.Period.String(),
			}
		}
		result = append(result, entry)
	}
	return e.JSON(http.StatusOK, result)
}

func (h *handlers) queueStats(e *core.RequestEvent) error {
	name := e.Request.PathValue("name")
	if name == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "missing queue name"})
	}

	type stat struct {
		Status string `db:"status"`
		Count  int    `db:"cnt"`
	}

	var stats []stat
	err := h.app.DB().
		NewQuery("SELECT status, COUNT(*) as cnt FROM pt_workflow_status WHERE queue_name = {:name} GROUP BY status").
		Bind(dbx.Params{"name": name}).
		All(&stats)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	result := map[string]int{
		"enqueued":  0,
		"running":   0,
		"completed": 0,
		"failed":    0,
	}
	for _, s := range stats {
		switch turbine.StatusType(s.Status) {
		case turbine.StatusEnqueued, turbine.StatusPending:
			result["enqueued"] += s.Count
		case turbine.StatusSuccess:
			result["completed"] += s.Count
		case turbine.StatusError, turbine.StatusMaxRecoveryAttemptsExceeded:
			result["failed"] += s.Count
		}
	}

	return e.JSON(http.StatusOK, result)
}

func (h *handlers) listScheduled(e *core.RequestEvent) error {
	scheduled := h.rt.ScheduledWorkflows()
	if scheduled == nil {
		scheduled = []turbine.ScheduledWorkflow{}
	}
	return e.JSON(http.StatusOK, scheduled)
}

func (h *handlers) listRegistered(e *core.RequestEvent) error {
	registered := h.rt.RegisteredWorkflows()
	if registered == nil {
		registered = []turbine.RegisteredWorkflow{}
	}
	return e.JSON(http.StatusOK, registered)
}

func (h *handlers) triggerWorkflow(e *core.RequestEvent) error {
	var req struct {
		WorkflowFQN string          `json:"workflow_fqn"`
		Input       json.RawMessage `json:"input"`
	}
	if err := e.BindBody(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if req.WorkflowFQN == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "workflow_fqn is required"})
	}

	wfID, err := h.rt.TriggerByFQN(req.WorkflowFQN, req.Input)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "not triggerable") {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return e.JSON(http.StatusOK, map[string]string{"workflow_id": wfID})
}

func (h *handlers) listSchedules(e *core.RequestEvent) error {
	records, err := h.app.FindAllRecords("pt_schedules")
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	result := make([]map[string]any, 0, len(records))
	for _, r := range records {
		result = append(result, map[string]any{
			"id":              r.Id,
			"workflow_fqn":    r.GetString("workflow_fqn"),
			"input":           r.Get("input"),
			"type":            r.GetString("type"),
			"cron_expression": r.GetString("cron_expression"),
			"jitter":          r.GetString("jitter"),
			"scheduled_at":    r.GetString("scheduled_at"),
			"created":         r.GetString("created"),
		})
	}
	return e.JSON(http.StatusOK, result)
}

func (h *handlers) createSchedule(e *core.RequestEvent) error {
	var req struct {
		WorkflowFQN    string          `json:"workflow_fqn"`
		Input          json.RawMessage `json:"input"`
		Type           string          `json:"type"`
		CronExpression string          `json:"cron_expression"`
		Jitter         string          `json:"jitter"`
		ScheduledAt    string          `json:"scheduled_at"`
	}
	if err := e.BindBody(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if req.WorkflowFQN == "" || req.Type == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "workflow_fqn and type are required"})
	}
	if req.Type != "cron" && req.Type != "once" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "type must be 'cron' or 'once'"})
	}
	if req.Type == "cron" && req.CronExpression == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "cron_expression is required for type=cron"})
	}
	if req.Type == "once" && req.ScheduledAt == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "scheduled_at is required for type=once"})
	}

	if req.Jitter != "" {
		if _, err := time.ParseDuration(req.Jitter); err != nil {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid jitter format (e.g. 30s, 2m)"})
		}
	}

	if !h.rt.IsTriggerable(req.WorkflowFQN) {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "workflow not found or not triggerable"})
	}

	col, err := h.app.FindCollectionByNameOrId("pt_schedules")
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	record := core.NewRecord(col)
	record.Set("workflow_fqn", req.WorkflowFQN)
	record.Set("input", string(req.Input))
	record.Set("type", req.Type)
	if req.CronExpression != "" {
		record.Set("cron_expression", req.CronExpression)
	}
	if req.Jitter != "" {
		record.Set("jitter", req.Jitter)
	}
	if req.ScheduledAt != "" {
		record.Set("scheduled_at", req.ScheduledAt)
	}
	if err := h.app.Save(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return e.JSON(http.StatusCreated, map[string]string{"id": record.Id})
}

func (h *handlers) deleteSchedule(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	if id == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "missing schedule id"})
	}

	record, err := h.app.FindRecordById("pt_schedules", id)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "schedule not found"})
	}

	if err := h.app.Delete(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return e.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}
