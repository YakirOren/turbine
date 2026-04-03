package dashboard

import (
	"net/http"
	"strconv"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type stepsTreeNode struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Type            string  `json:"type"`
	Status          string  `json:"status"`
	FunctionID      int     `json:"functionId"`
	StartedAtMs     int64   `json:"startedAtMs"`
	EndedAtMs       int64   `json:"endedAtMs"`
	Output          *string `json:"output"`
	Error           *string `json:"error"`
	ChildWorkflowID *string `json:"childWorkflowId"`
}

type stepsTreeEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type stepsTreeResponse struct {
	Nodes          []stepsTreeNode `json:"nodes"`
	Edges          []stepsTreeEdge `json:"edges"`
	WorkflowStatus string          `json:"workflowStatus"`
}

type stepRow struct {
	FunctionID      int     `db:"function_id"`
	FunctionName    string  `db:"function_name"`
	Output          *string `db:"output"`
	Error           *string `db:"error"`
	ChildWorkflowID *string `db:"child_workflow_id"`
	StartedAtMs     int64   `db:"started_at_epoch_ms"`
	EndedAtMs       int64   `db:"ended_at_epoch_ms"`
}

func (h *handlers) stepsTree(e *core.RequestEvent) error {
	workflowID := e.Request.PathValue("id")
	if workflowID == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "missing workflow id"})
	}

	record, err := h.app.FindRecordById("pt_workflow_status", workflowID)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "workflow not found"})
	}
	workflowStatus := record.GetString("status")

	var steps []stepRow
	err = h.app.DB().
		NewQuery("SELECT function_id, function_name, output, error, child_workflow_id, started_at_epoch_ms, ended_at_epoch_ms FROM pt_operation_outputs WHERE workflow_id = {:wfID} ORDER BY function_id ASC").
		Bind(dbx.Params{"wfID": workflowID}).
		All(&steps)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	nodes, edges := buildStepsTree(steps, workflowStatus)

	return e.JSON(http.StatusOK, stepsTreeResponse{
		Nodes:          nodes,
		Edges:          edges,
		WorkflowStatus: workflowStatus,
	})
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func isBarrierStep(name string) bool {
	return name == "pt.sleep" || name == "pt.getResult"
}

func buildStepsTree(steps []stepRow, workflowStatus string) ([]stepsTreeNode, []stepsTreeEdge) {
	if len(steps) == 0 {
		return []stepsTreeNode{}, []stepsTreeEdge{}
	}

	nodes := make([]stepsTreeNode, 0, len(steps)+1)
	edges := make([]stepsTreeEdge, 0, len(steps))

	for _, s := range steps {
		nodeType := "step"
		if s.ChildWorkflowID != nil && *s.ChildWorkflowID != "" {
			nodeType = "child-workflow"
		}

		status := "success"
		if s.Error != nil && *s.Error != "" {
			status = "error"
		} else if s.EndedAtMs == 0 {
			status = "running"
		}

		nodes = append(nodes, stepsTreeNode{
			ID:              strconv.Itoa(s.FunctionID),
			Name:            s.FunctionName,
			Type:            nodeType,
			Status:          status,
			FunctionID:      s.FunctionID,
			StartedAtMs:     s.StartedAtMs,
			EndedAtMs:       s.EndedAtMs,
			Output:          s.Output,
			Error:           s.Error,
			ChildWorkflowID: s.ChildWorkflowID,
		})
	}

	resultNodeID := "result"
	resultStatus := "success"
	if workflowStatus == "ERROR" || workflowStatus == "MAX_RECOVERY_ATTEMPTS_EXCEEDED" {
		resultStatus = "error"
	} else if workflowStatus != "SUCCESS" {
		resultStatus = "running"
	}
	nodes = append(nodes, stepsTreeNode{
		ID:     resultNodeID,
		Name:   workflowStatus,
		Type:   "workflow-result",
		Status: resultStatus,
	})

	if len(steps) == 1 {
		edges = append(edges, stepsTreeEdge{Source: strconv.Itoa(steps[0].FunctionID), Target: resultNodeID})
		return nodes, edges
	}

	// Detect parallel groups by checking if consecutive steps have overlapping
	// execution windows. Two steps overlap if the later one started before
	// the earlier one ended.
	type group struct {
		parentID string
		members  []string
		maxEnd   int64
	}

	var prevSequentialID string
	var lastSequentialID string
	var currentGroup *group

	for i, s := range steps {
		sid := strconv.Itoa(s.FunctionID)

		if i == 0 {
			lastSequentialID = sid
			continue
		}

		if isBarrierStep(s.FunctionName) {
			if currentGroup != nil {
				for _, memberID := range currentGroup.members {
					edges = append(edges, stepsTreeEdge{Source: memberID, Target: sid})
				}
				currentGroup = nil
			} else {
				edges = append(edges, stepsTreeEdge{Source: lastSequentialID, Target: sid})
			}
			prevSequentialID = lastSequentialID
			lastSequentialID = sid
			continue
		}

		// Check overlap with previous step (the one right before in function_id order)
		prev := steps[i-1]
		// Two steps are parallel if they started at approximately the same time
		// (within 50ms), which is the pattern for DoAsync calls
		const startToleranceMs int64 = 50
		isParallel := !isBarrierStep(prev.FunctionName) && abs(s.StartedAtMs-prev.StartedAtMs) <= startToleranceMs

		if isParallel {
			if currentGroup == nil {
				prevSid := strconv.Itoa(prev.FunctionID)

				if prevSequentialID == "" || prevSid == lastSequentialID && prevSequentialID == "" {
					// Previous step is the very first node — it becomes the parent
					currentGroup = &group{
						parentID: prevSid,
						members:  []string{sid},
						maxEnd:   s.EndedAtMs,
					}
					edges = append(edges, stepsTreeEdge{Source: prevSid, Target: sid})
				} else {
					// Previous step is a parallel sibling — rewire it into a group
					// The step before prev (prevSequentialID) becomes the parent
					parentID := prevSequentialID
					currentGroup = &group{
						parentID: parentID,
						members:  []string{prevSid, sid},
						maxEnd:   s.EndedAtMs,
					}
					if prev.EndedAtMs > currentGroup.maxEnd {
						currentGroup.maxEnd = prev.EndedAtMs
					}
					// Remove the sequential edge we already added for prev
					// and replace with parallel edges from parent
					edges = edges[:len(edges)-1]
					edges = append(edges,
						stepsTreeEdge{Source: parentID, Target: prevSid},
						stepsTreeEdge{Source: parentID, Target: sid},
					)
				}
			} else {
				currentGroup.members = append(currentGroup.members, sid)
				if s.EndedAtMs > currentGroup.maxEnd {
					currentGroup.maxEnd = s.EndedAtMs
				}
				edges = append(edges, stepsTreeEdge{Source: currentGroup.parentID, Target: sid})
			}
		} else {
			if currentGroup != nil {
				for _, memberID := range currentGroup.members {
					edges = append(edges, stepsTreeEdge{Source: memberID, Target: sid})
				}
				currentGroup = nil
			} else {
				edges = append(edges, stepsTreeEdge{Source: lastSequentialID, Target: sid})
			}
			prevSequentialID = lastSequentialID
			lastSequentialID = sid
		}
	}

	if currentGroup != nil {
		for _, memberID := range currentGroup.members {
			edges = append(edges, stepsTreeEdge{Source: memberID, Target: resultNodeID})
		}
	} else {
		edges = append(edges, stepsTreeEdge{Source: lastSequentialID, Target: resultNodeID})
	}

	return nodes, edges
}
