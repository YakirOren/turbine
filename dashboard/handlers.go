package dashboard

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

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

func (h *handlers) approveWorkflow(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	if id == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "missing workflow id"})
	}

	// Validate the workflow exists and is actually waiting for approval
	record, err := h.app.FindRecordById("pt_workflow_status", id)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "workflow not found"})
	}
	if record.GetString("app_status") != "waiting for approval" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "workflow is not waiting for approval"})
	}

	var req struct {
		Approved bool   `json:"approved"`
		Comment  string `json:"comment"`
	}
	if err := e.BindBody(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	err = h.rt.SendToWorkflow(id, turbine.ApprovalResult{
		Approved: req.Approved,
		Comment:  req.Comment,
	}, "pt.approval")
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return e.JSON(http.StatusOK, map[string]string{"status": "sent"})
}

func (h *handlers) calendarStats(e *core.RequestEvent) error {
	q := e.Request.URL.Query()

	fromMsStr := q.Get("from_ms")
	toMsStr := q.Get("to_ms")
	bucketMinsStr := q.Get("bucket_mins")
	if fromMsStr == "" || toMsStr == "" || bucketMinsStr == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "from_ms, to_ms, and bucket_mins query params are required"})
	}

	var fromMs, toMs, bucketMins int64
	for _, pair := range []struct {
		s string
		v *int64
	}{{fromMsStr, &fromMs}, {toMsStr, &toMs}, {bucketMinsStr, &bucketMins}} {
		n := int64(0)
		for _, c := range pair.s {
			if c < '0' || c > '9' {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "params must be integers"})
			}
			n = n*10 + int64(c-'0')
		}
		*pair.v = n
	}

	if bucketMins < 1 {
		bucketMins = 1
	}
	bucketMs := bucketMins * 60 * 1000

	name := q.Get("name")
	status := q.Get("status")
	tag := q.Get("tag")

	where := "created_at_epoch_ms >= {:from} AND created_at_epoch_ms < {:to}"
	params := dbx.Params{"from": fromMs, "to": toMs, "bucket": bucketMs}

	if name != "" {
		where += " AND name = {:name}"
		params["name"] = name
	}
	if status != "" {
		where += " AND status = {:status}"
		params["status"] = status
	}
	if tag != "" {
		where += " AND tags LIKE {:tag}"
		params["tag"] = "%" + tag + "%"
	}

	type bucketStat struct {
		Bucket int64  `db:"bucket" json:"bucket"`
		Status string `db:"status" json:"status"`
		Count  int    `db:"cnt" json:"count"`
	}

	var stats []bucketStat
	err := h.app.DB().
		NewQuery(`SELECT (created_at_epoch_ms / {:bucket}) * {:bucket} as bucket, status, COUNT(*) as cnt
			FROM pt_workflow_status
			WHERE ` + where + `
			GROUP BY bucket, status
			ORDER BY bucket ASC`).
		Bind(params).
		All(&stats)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	type bucketResult struct {
		Time      int64 `json:"time"`
		Success   int   `json:"success"`
		Error     int   `json:"error"`
		Cancelled int   `json:"cancelled"`
	}

	bucketMap := make(map[int64]*bucketResult)
	for _, s := range stats {
		b, ok := bucketMap[s.Bucket]
		if !ok {
			b = &bucketResult{Time: s.Bucket}
			bucketMap[s.Bucket] = b
		}
		switch turbine.StatusType(s.Status) {
		case turbine.StatusSuccess:
			b.Success += s.Count
		case turbine.StatusError, turbine.StatusMaxRecoveryAttemptsExceeded:
			b.Error += s.Count
		case turbine.StatusCancelled:
			b.Cancelled += s.Count
		}
	}

	result := make([]bucketResult, 0, len(bucketMap))
	for _, b := range bucketMap {
		result = append(result, *b)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Time < result[j].Time
	})

	return e.JSON(http.StatusOK, result)
}
