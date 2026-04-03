import { useState, useEffect, useCallback } from "react";
import {
  ReactFlow,
  ReactFlowProvider,
  Background,
  Controls,
  type Node,
  type Edge,
  useNodesState,
  useEdgesState,
} from "@xyflow/react";
import dagre from "@dagrejs/dagre";
import "@xyflow/react/dist/style.css";

import {
  Dialog,
  DialogContent,
} from "@/components/ui/dialog";
import { ChevronRight } from "lucide-react";
import { pbClient } from "@/providers/pocketbase";
import { nodeTypes } from "@/components/step-node";

interface StepsTreeResponse {
  nodes: Array<{
    id: string;
    name: string;
    type: string;
    status: string;
    functionId: number;
    startedAtMs: number;
    endedAtMs: number;
    output: string | null;
    error: string | null;
    childWorkflowId: string | null;
  }>;
  edges: Array<{ source: string; target: string }>;
  workflowStatus: string;
}

function layoutWithDagre(
  nodes: Node[],
  edges: Edge[]
): { nodes: Node[]; edges: Edge[] } {
  const g = new dagre.graphlib.Graph();
  g.setDefaultEdgeLabel(() => ({}));
  g.setGraph({ rankdir: "TB", nodesep: 50, ranksep: 70 });

  for (const node of nodes) {
    g.setNode(node.id, { width: 220, height: 50 });
  }
  for (const edge of edges) {
    g.setEdge(edge.source, edge.target);
  }

  dagre.layout(g);

  const layoutedNodes = nodes.map((node) => {
    const pos = g.node(node.id);
    return {
      ...node,
      position: { x: pos.x - 110, y: pos.y - 25 },
    };
  });

  return { nodes: layoutedNodes, edges };
}

function StepFlowContent({
  workflowId,
  onClose,
}: {
  workflowId: string;
  onClose: () => void;
}) {
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);
  const [breadcrumbs, setBreadcrumbs] = useState<
    Array<{ id: string; label: string }>
  >([{ id: workflowId, label: "Root" }]);
  const [loading, setLoading] = useState(true);

  const currentWorkflowId = breadcrumbs[breadcrumbs.length - 1].id;

  const navigateToChild = useCallback(
    (childId: string) => {
      setBreadcrumbs((prev) => [...prev, { id: childId, label: childId.slice(0, 8) }]);
    },
    []
  );

  const navigateToBreadcrumb = useCallback((index: number) => {
    setBreadcrumbs((prev) => prev.slice(0, index + 1));
  }, []);

  useEffect(() => {
    setLoading(true);
    pbClient
      .send<StepsTreeResponse>(
        `/api/pf/workflows/${currentWorkflowId}/steps-tree`,
        { method: "GET" }
      )
      .then((data) => {
        const rfNodes: Node[] = data.nodes.map((n) => ({
          id: n.id,
          type: n.type,
          position: { x: 0, y: 0 },
          data: {
            label: n.name,
            status: n.status,
            functionId: n.functionId,
            childWorkflowId: n.childWorkflowId,
            onChildClick: navigateToChild,
          },
        }));

        const rfEdges: Edge[] = data.edges.map((e, i) => ({
          id: `e-${i}`,
          source: e.source,
          target: e.target,
          animated: false,
          style: { stroke: "hsl(var(--muted-foreground))", strokeWidth: 1.5 },
        }));

        const laid = layoutWithDagre(rfNodes, rfEdges);
        setNodes(laid.nodes);
        setEdges(laid.edges);
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, [currentWorkflowId, navigateToChild, setNodes, setEdges]);

  return (
    <Dialog open onOpenChange={() => onClose()}>
      <DialogContent className="h-[85vh] max-w-[95vw] p-0">
        <div className="flex h-full flex-col">
          {/* Breadcrumbs */}
          <div className="flex items-center gap-1 border-b px-4 py-2 text-sm">
            {breadcrumbs.map((bc, i) => (
              <span key={bc.id} className="flex items-center gap-1">
                {i > 0 && (
                  <ChevronRight className="h-3 w-3 text-muted-foreground" />
                )}
                <button
                  className={
                    i === breadcrumbs.length - 1
                      ? "font-medium"
                      : "text-muted-foreground hover:text-foreground"
                  }
                  onClick={() => navigateToBreadcrumb(i)}
                >
                  {bc.label}
                </button>
              </span>
            ))}
          </div>

          {/* React Flow canvas */}
          <div className="flex-1">
            {loading ? (
              <div className="flex h-full items-center justify-center text-muted-foreground">
                Loading steps...
              </div>
            ) : (
              <ReactFlow
                nodes={nodes}
                edges={edges}
                onNodesChange={onNodesChange}
                onEdgesChange={onEdgesChange}
                nodeTypes={nodeTypes}
                fitView
                proOptions={{ hideAttribution: true }}
              >
                <Background />
                <Controls />
              </ReactFlow>
            )}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

export function StepFlowDialog({
  workflowId,
  onClose,
}: {
  workflowId: string;
  onClose: () => void;
}) {
  return (
    <ReactFlowProvider>
      <StepFlowContent workflowId={workflowId} onClose={onClose} />
    </ReactFlowProvider>
  );
}
