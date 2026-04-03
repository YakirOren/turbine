import { Handle, Position, type NodeProps } from "@xyflow/react";
import { cn } from "@/lib/utils";
import {
  CheckCircle,
  XCircle,
  Loader2,
  GitBranch,
  Circle,
} from "lucide-react";

interface StepNodeData {
  label: string;
  status: string;
  functionId?: number;
  childWorkflowId?: string | null;
  onChildClick?: (childId: string) => void;
  hasInput?: boolean;
  hasOutput?: boolean;
  durationMs?: number | null;
  selected?: boolean;
  [key: string]: unknown;
}

const statusIcon: Record<string, React.ReactNode> = {
  success: <CheckCircle className="h-3 w-3 text-green-400" />,
  error: <XCircle className="h-3 w-3 text-red-400" />,
  running: <Loader2 className="h-3 w-3 animate-spin text-blue-400" />,
};

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  return `${(ms / 60000).toFixed(1)}m`;
}

const statusBorder: Record<string, string> = {
  success: "border-green-500/40",
  error: "border-red-500/40",
  running: "border-blue-500/40",
};

export function StepNode({ data }: NodeProps) {
  const d = data as StepNodeData;
  return (
    <div
      className={cn(
        "whitespace-nowrap rounded border bg-card px-2 py-1 shadow-sm cursor-pointer",
        d.selected ? "ring-2 ring-primary" : "",
        statusBorder[d.status] ?? "border-border"
      )}
    >
      {d.hasInput !== false && <Handle type="target" position={Position.Left} className="!bg-muted-foreground" />}
      <div className="flex items-center gap-1.5 justify-center">
        {statusIcon[d.status] ?? <Circle className="h-3 w-3 text-muted-foreground" />}
        <span className="text-xs font-medium">{d.label}</span>
        {d.durationMs != null && (
          <span className="text-[10px] text-muted-foreground">{formatDuration(d.durationMs)}</span>
        )}
      </div>
      {d.hasOutput !== false && <Handle type="source" position={Position.Right} className="!bg-muted-foreground" />}
    </div>
  );
}

export function ChildWorkflowNode({ data }: NodeProps) {
  const d = data as StepNodeData;
  return (
    <div
      className={cn(
        "whitespace-nowrap cursor-pointer rounded border border-dashed bg-card px-2 py-1 shadow-sm hover:border-primary",
        statusBorder[d.status] ?? "border-border"
      )}
      onClick={() => d.childWorkflowId && d.onChildClick?.(d.childWorkflowId)}
    >
      {d.hasInput !== false && <Handle type="target" position={Position.Left} className="!bg-muted-foreground" />}
      <div className="flex items-center gap-1.5">
        <GitBranch className="h-3 w-3 text-muted-foreground" />
        {statusIcon[d.status] ?? <Circle className="h-3 w-3" />}
        <span className="text-xs font-medium">{d.label}</span>
        {d.durationMs != null && (
          <span className="text-[10px] text-muted-foreground">{formatDuration(d.durationMs)}</span>
        )}
      </div>
      {d.hasOutput !== false && <Handle type="source" position={Position.Right} className="!bg-muted-foreground" />}
    </div>
  );
}

export function WorkflowResultNode({ data }: NodeProps) {
  const d = data as StepNodeData;
  const isError = d.status === "error";
  return (
    <div
      className={cn(
        "rounded-full border px-3 py-1 text-center",
        isError
          ? "border-red-500/40 bg-red-500/10 text-red-400"
          : d.status === "running"
            ? "border-blue-500/40 bg-blue-500/10 text-blue-400"
            : "border-green-500/40 bg-green-500/10 text-green-400"
      )}
    >
      <Handle type="target" position={Position.Left} className="!bg-muted-foreground" />
      <div className="flex items-center justify-center gap-1.5">
        {statusIcon[d.status]}
        <span className="text-xs font-semibold">{d.label}</span>
      </div>
    </div>
  );
}

export const nodeTypes = {
  step: StepNode,
  "child-workflow": ChildWorkflowNode,
  "workflow-result": WorkflowResultNode,
};
