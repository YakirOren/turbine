package dashboard

import (
	"net/http"

	"github.com/YakirOren/pocketflow"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type handlers struct {
	rt  *pocketflow.Runtime
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
		if q.Name == "_pf_internal_queue" {
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
		NewQuery("SELECT status, COUNT(*) as cnt FROM pf_workflow_status WHERE queue_name = {:name} GROUP BY status").
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
		switch pocketflow.StatusType(s.Status) {
		case pocketflow.StatusEnqueued, pocketflow.StatusPending:
			result["enqueued"] += s.Count
		case pocketflow.StatusSuccess:
			result["completed"] += s.Count
		case pocketflow.StatusError, pocketflow.StatusMaxRecoveryAttemptsExceeded:
			result["failed"] += s.Count
		}
	}

	return e.JSON(http.StatusOK, result)
}
