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
		return e.BadRequestError("missing workflow id", nil)
	}
	if err := h.rt.Cancel(id); err != nil {
		return e.InternalServerError("failed to cancel workflow", err)
	}
	return e.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *handlers) resumeWorkflow(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	if id == "" {
		return e.BadRequestError("missing workflow id", nil)
	}
	if err := h.rt.Resume(id); err != nil {
		return e.InternalServerError("failed to resume workflow", err)
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
		return e.BadRequestError("missing queue name", nil)
	}

	type stat struct {
		Status string `db:"status"`
		Count  int    `db:"cnt"`
	}

	var stats []stat
	err := h.app.DB().
		Select("status", "COUNT(*) as cnt").
		From("pt_workflow_status").
		Where(dbx.HashExp{"queue_name": name}).
		GroupBy("status").
		All(&stats)
	if err != nil {
		return e.InternalServerError("failed to query queue stats", err)
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
		return e.BadRequestError("invalid request body", nil)
	}
	if req.WorkflowFQN == "" {
		return e.BadRequestError("workflow_fqn is required", nil)
	}

	wfID, err := h.rt.TriggerByFQN(req.WorkflowFQN, req.Input)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "not triggerable") {
			return e.BadRequestError(err.Error(), nil)
		}
		return e.InternalServerError("failed to trigger workflow", err)
	}

	return e.JSON(http.StatusOK, map[string]string{"workflow_id": wfID})
}

func (h *handlers) approveWorkflow(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	if id == "" {
		return e.BadRequestError("missing workflow id", nil)
	}

	// Validate the workflow exists and is actually waiting for approval
	record, err := h.app.FindRecordById("pt_workflow_status", id)
	if err != nil {
		return e.NotFoundError("workflow not found", nil)
	}
	if record.GetString("app_status") != "waiting for approval" {
		return e.BadRequestError("workflow is not waiting for approval", nil)
	}

	var req struct {
		Approved bool   `json:"approved"`
		Comment  string `json:"comment"`
	}
	if err := e.BindBody(&req); err != nil {
		return e.BadRequestError("invalid request body", nil)
	}

	err = h.rt.SendToWorkflow(id, turbine.ApprovalResult{
		Approved: req.Approved,
		Comment:  req.Comment,
	}, "pt.approval")
	if err != nil {
		return e.InternalServerError("failed to approve workflow", err)
	}

	return e.JSON(http.StatusOK, map[string]string{"status": "sent"})
}

func (h *handlers) testAlertChannel(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	if id == "" {
		return e.BadRequestError("missing channel id", nil)
	}
	if err := h.rt.TestAlertChannel(id); err != nil {
		return e.InternalServerError("failed to test alert channel", err)
	}
	return e.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handlers) resendProduct(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	if id == "" {
		return e.BadRequestError("missing product id", nil)
	}

	record, err := h.app.FindRecordById("pt_products", id)
	if err != nil {
		return e.NotFoundError("product not found", nil)
	}

	if record.GetString("status") != "failed" {
		return e.BadRequestError("only failed products can be resent", nil)
	}

	if err := turbine.ResendProduct(h.app, h.rt, record); err != nil {
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}

	return e.JSON(http.StatusOK, map[string]string{"status": "sent"})
}

func (h *handlers) calendarStats(e *core.RequestEvent) error {
	q := e.Request.URL.Query()

	fromMsStr := q.Get("from_ms")
	toMsStr := q.Get("to_ms")
	bucketMinsStr := q.Get("bucket_mins")
	if fromMsStr == "" || toMsStr == "" || bucketMinsStr == "" {
		return e.BadRequestError("from_ms, to_ms, and bucket_mins query params are required", nil)
	}

	var fromMs, toMs, bucketMins int64
	for _, pair := range []struct {
		s string
		v *int64
	}{{fromMsStr, &fromMs}, {toMsStr, &toMs}, {bucketMinsStr, &bucketMins}} {
		n := int64(0)
		for _, c := range pair.s {
			if c < '0' || c > '9' {
				return e.BadRequestError("params must be integers", nil)
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

	type bucketStat struct {
		Bucket int64  `db:"bucket" json:"bucket"`
		Status string `db:"status" json:"status"`
		Count  int    `db:"cnt" json:"count"`
	}

	qb := h.app.DB().
		Select("(created_at_epoch_ms / {:bucket}) * {:bucket} as bucket", "status", "COUNT(*) as cnt").
		From("pt_workflow_status").
		Where(dbx.NewExp("created_at_epoch_ms >= {:from} AND created_at_epoch_ms < {:to}", dbx.Params{"from": fromMs, "to": toMs})).
		GroupBy("bucket", "status").
		OrderBy("bucket ASC")

	if name != "" {
		qb.AndWhere(dbx.HashExp{"name": name})
	}
	if status != "" {
		qb.AndWhere(dbx.HashExp{"status": status})
	}
	if tag != "" {
		qb.AndWhere(dbx.Like("tags", tag))
	}

	var stats []bucketStat
	err := qb.Build().Bind(dbx.Params{"bucket": bucketMs}).All(&stats)
	if err != nil {
		return e.InternalServerError("failed to query calendar stats", err)
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
