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

	nodes, edges := buildStepsTree(steps, "SUCCESS")

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

	nodes, edges := buildStepsTree(steps, "SUCCESS")

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

	nodes, edges := buildStepsTree(steps, "SUCCESS")

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	assertEdge(t, edges[0], "0", "result")
}

func TestBuildStepsTree_Empty(t *testing.T) {
	nodes, edges := buildStepsTree([]stepRow{}, "PENDING")

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
		{FunctionID: 1, FunctionName: "pf.sleep", StartedAtMs: 100, EndedAtMs: 5000},
		{FunctionID: 2, FunctionName: "after_sleep", StartedAtMs: 5010, EndedAtMs: 5100},
	}

	_, edges := buildStepsTree(steps, "SUCCESS")

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

	nodes, _ := buildStepsTree(steps, "SUCCESS")

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
		"ERROR",
	)

	resultNode := nodes[len(nodes)-1]
	if resultNode.Status != "error" {
		t.Fatalf("expected error status on result node, got %s", resultNode.Status)
	}
}

func assertEdge(t *testing.T, edge stepsTreeEdge, source, target string) {
	t.Helper()
	if edge.Source != source || edge.Target != target {
		t.Errorf("expected edge %s→%s, got %s→%s", source, target, edge.Source, edge.Target)
	}
}
