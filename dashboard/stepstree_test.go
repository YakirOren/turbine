package dashboard

import (
	"testing"
)

func TestBuildStepsTree_Sequential(t *testing.T) {
	steps := []stepRow{
		{FunctionID: 0, FunctionName: "step_a", StartedAtMs: 0, EndedAtMs: 100},
		{FunctionID: 1, FunctionName: "step_b", StartedAtMs: 110, EndedAtMs: 200},
		{FunctionID: 2, FunctionName: "step_c", StartedAtMs: 210, EndedAtMs: 300},
	}

	nodes, edges := buildStepsTree(steps, "SUCCESS", "")

	if len(nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(nodes))
	}
	if len(edges) != 3 {
		t.Fatalf("expected 3 edges, got %d", len(edges))
	}
	assertEdge(t, edges[0], "0", "1")
	assertEdge(t, edges[1], "1", "2")
	assertEdge(t, edges[2], "2", "result")
}

func TestBuildStepsTree_ParallelGroup(t *testing.T) {
	steps := []stepRow{
		{FunctionID: 0, FunctionName: "process_tasks", StartedAtMs: 0, EndedAtMs: 100},
		{FunctionID: 1, FunctionName: "step_one", StartedAtMs: 5, EndedAtMs: 40},
		{FunctionID: 2, FunctionName: "step_one", StartedAtMs: 5, EndedAtMs: 45},
		{FunctionID: 3, FunctionName: "step_one", StartedAtMs: 6, EndedAtMs: 42},
		{FunctionID: 4, FunctionName: "postprocess", StartedAtMs: 110, EndedAtMs: 150},
	}

	nodes, edges := buildStepsTree(steps, "SUCCESS", "")

	if len(nodes) != 6 {
		t.Fatalf("expected 6 nodes, got %d", len(nodes))
	}
	if len(edges) != 7 {
		t.Fatalf("expected 7 edges, got %d: %+v", len(edges), edges)
	}
}

func TestBuildStepsTree_SingleStep(t *testing.T) {
	steps := []stepRow{
		{FunctionID: 0, FunctionName: "only_step", StartedAtMs: 0, EndedAtMs: 100},
	}

	nodes, edges := buildStepsTree(steps, "SUCCESS", "")

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	assertEdge(t, edges[0], "0", "result")
}

func TestBuildStepsTree_Empty(t *testing.T) {
	nodes, edges := buildStepsTree([]stepRow{}, "PENDING", "")

	if len(nodes) != 0 {
		t.Fatalf("expected 0 nodes, got %d", len(nodes))
	}
	if len(edges) != 0 {
		t.Fatalf("expected 0 edges, got %d", len(edges))
	}
}

func TestBuildStepsTree_SleepBarrier(t *testing.T) {
	steps := []stepRow{
		{FunctionID: 0, FunctionName: "work", StartedAtMs: 0, EndedAtMs: 100},
		{FunctionID: 1, FunctionName: "pt.sleep", StartedAtMs: 100, EndedAtMs: 5000},
		{FunctionID: 2, FunctionName: "after_sleep", StartedAtMs: 5010, EndedAtMs: 5100},
	}

	_, edges := buildStepsTree(steps, "SUCCESS", "")

	if len(edges) != 3 {
		t.Fatalf("expected 3 edges (sequential), got %d: %+v", len(edges), edges)
	}
	assertEdge(t, edges[0], "0", "1")
	assertEdge(t, edges[1], "1", "2")
}

func TestBuildStepsTree_ChildWorkflowNode(t *testing.T) {
	childID := "child-abc"
	steps := []stepRow{
		{FunctionID: 0, FunctionName: "launch_child", StartedAtMs: 0, EndedAtMs: 100, ChildWorkflowID: &childID},
	}

	nodes, _ := buildStepsTree(steps, "SUCCESS", "")

	if nodes[0].Type != "child-workflow" {
		t.Fatalf("expected child-workflow type, got %s", nodes[0].Type)
	}
	if *nodes[0].ChildWorkflowID != "child-abc" {
		t.Fatalf("expected child-abc, got %s", *nodes[0].ChildWorkflowID)
	}
}

func TestBuildStepsTree_ErrorStatus(t *testing.T) {
	nodes, _ := buildStepsTree(
		[]stepRow{{FunctionID: 0, FunctionName: "x", StartedAtMs: 0, EndedAtMs: 1}},
		"ERROR", "",
	)

	resultNode := nodes[len(nodes)-1]
	if resultNode.Status != "error" {
		t.Fatalf("expected error status on result node, got %s", resultNode.Status)
	}
}

func TestBuildStepsTree_DoAsyncParallel(t *testing.T) {
	// Simulates: validate → (charge || inventory) → reserve → (ship || confirm)
	steps := []stepRow{
		{FunctionID: 0, FunctionName: "validate", StartedAtMs: 0, EndedAtMs: 100},
		{FunctionID: 1, FunctionName: "charge", StartedAtMs: 100, EndedAtMs: 300},
		{FunctionID: 2, FunctionName: "inventory", StartedAtMs: 100, EndedAtMs: 250},
		{FunctionID: 3, FunctionName: "reserve", StartedAtMs: 300, EndedAtMs: 400},
		{FunctionID: 4, FunctionName: "ship", StartedAtMs: 400, EndedAtMs: 700},
		{FunctionID: 5, FunctionName: "confirm", StartedAtMs: 400, EndedAtMs: 450},
	}

	_, edges := buildStepsTree(steps, "SUCCESS", "")

	// Expected edges:
	// 0→1, 0→2 (parallel group 1)
	// 1→3, 2→3 (converge to reserve)
	// 3→4, 3→5 (parallel group 2)
	// 4→result, 5→result (end)
	if len(edges) != 8 {
		t.Fatalf("expected 8 edges, got %d: %+v", len(edges), edges)
	}
	assertEdge(t, edges[0], "0", "1")
	assertEdge(t, edges[1], "0", "2")
	assertEdge(t, edges[2], "1", "3")
	assertEdge(t, edges[3], "2", "3")
	assertEdge(t, edges[4], "3", "4")
	assertEdge(t, edges[5], "3", "5")
	assertEdge(t, edges[6], "4", "result")
	assertEdge(t, edges[7], "5", "result")
}

func TestBuildStepsTree_RealWorldDoAsync(t *testing.T) {
	// Real timing data from the dashboard example
	steps := []stepRow{
		{FunctionID: 0, FunctionName: "validate", StartedAtMs: 1773417398118, EndedAtMs: 1773417398219},
		{FunctionID: 1, FunctionName: "charge", StartedAtMs: 1773417398221, EndedAtMs: 1773417398422},
		{FunctionID: 2, FunctionName: "inventory", StartedAtMs: 1773417398221, EndedAtMs: 1773417398372},
		{FunctionID: 3, FunctionName: "reserve", StartedAtMs: 1773417398424, EndedAtMs: 1773417398526},
		{FunctionID: 4, FunctionName: "ship", StartedAtMs: 1773417398528, EndedAtMs: 1773417398829},
		{FunctionID: 5, FunctionName: "confirm", StartedAtMs: 1773417398529, EndedAtMs: 1773417398580},
	}

	_, edges := buildStepsTree(steps, "SUCCESS", "")

	// validate → (charge || inventory) → reserve → (ship || confirm) → result
	if len(edges) != 8 {
		t.Fatalf("expected 8 edges, got %d: %+v", len(edges), edges)
	}
	assertEdge(t, edges[0], "0", "1")
	assertEdge(t, edges[1], "0", "2")
	assertEdge(t, edges[2], "1", "3")
	assertEdge(t, edges[3], "2", "3")
	assertEdge(t, edges[4], "3", "4")
	assertEdge(t, edges[5], "3", "5")
	assertEdge(t, edges[6], "4", "result")
	assertEdge(t, edges[7], "5", "result")
}

func TestBuildStepsTree_ApprovalNode(t *testing.T) {
	steps := []stepRow{
		{FunctionID: 1, FunctionName: "prepare-data", StartedAtMs: 1000, EndedAtMs: 2000},
	}
	nodes, edges := buildStepsTree(steps, "PENDING", "waiting for approval")

	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}

	var approvalNode *stepsTreeNode
	for i := range nodes {
		if nodes[i].Type == "approval" {
			approvalNode = &nodes[i]
			break
		}
	}
	if approvalNode == nil {
		t.Fatal("expected an approval node")
	}
	if approvalNode.Status != "running" {
		t.Fatalf("expected approval node status 'running', got %q", approvalNode.Status)
	}
	if approvalNode.Name != "Waiting for Approval" {
		t.Fatalf("expected name 'Waiting for Approval', got %q", approvalNode.Name)
	}

	hasEdge := func(src, tgt string) bool {
		for _, e := range edges {
			if e.Source == src && e.Target == tgt {
				return true
			}
		}
		return false
	}
	if !hasEdge("1", "approval-wait") {
		t.Fatal("expected edge from step 1 to approval-wait")
	}
	if !hasEdge("approval-wait", "result") {
		t.Fatal("expected edge from approval-wait to result")
	}
}

func TestBuildStepsTree_NoApprovalNodeWhenNotWaiting(t *testing.T) {
	steps := []stepRow{
		{FunctionID: 1, FunctionName: "do-work", StartedAtMs: 1000, EndedAtMs: 2000},
	}
	nodes, _ := buildStepsTree(steps, "SUCCESS", "")

	for _, n := range nodes {
		if n.Type == "approval" {
			t.Fatal("should not have approval node when not waiting")
		}
	}
}

func assertEdge(t *testing.T, edge stepsTreeEdge, source, target string) {
	t.Helper()
	if edge.Source != source || edge.Target != target {
		t.Errorf("expected edge %s→%s, got %s→%s", source, target, edge.Source, edge.Target)
	}
}
