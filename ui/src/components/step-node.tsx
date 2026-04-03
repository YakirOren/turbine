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
  [key: string]: unknown;
}

const statusIcon: Record<string, React.ReactNode> = {
  success: <CheckCircle className="h-4 w-4 text-green-400" />,
  error: <XCircle className="h-4 w-4 text-red-400" />,
  running: <Loader2 className="h-4 w-4 animate-spin text-blue-400" />,
};

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
        "rounded-lg border bg-card px-4 py-2 shadow-sm",
        statusBorder[d.status] ?? "border-border"
      )}
    >
      <Handle type="target" position={Position.Top} className="!bg-muted-foreground" />
      <div className="flex items-center gap-2">
        {statusIcon[d.status] ?? <Circle className="h-4 w-4 text-muted-foreground" />}
        <span className="text-sm font-medium">{d.label}</span>
        {d.functionId != null && (
          <span className="rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">
            #{d.functionId}
          </span>
        )}
      </div>
      <Handle type="source" position={Position.Bottom} className="!bg-muted-foreground" />
    </div>
  );
}

export function ChildWorkflowNode({ data }: NodeProps) {
  const d = data as StepNodeData;
  return (
    <div
      className={cn(
        "cursor-pointer rounded-lg border border-dashed bg-card px-4 py-2 shadow-sm hover:border-primary",
        statusBorder[d.status] ?? "border-border"
      )}
      onClick={() => d.childWorkflowId && d.onChildClick?.(d.childWorkflowId)}
    >
      <Handle type="target" position={Position.Top} className="!bg-muted-foreground" />
      <div className="flex items-center gap-2">
        <GitBranch className="h-4 w-4 text-muted-foreground" />
        {statusIcon[d.status] ?? <Circle className="h-4 w-4" />}
        <span className="text-sm font-medium">{d.label}</span>
        {d.functionId != null && (
          <span className="rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">
            #{d.functionId}
          </span>
        )}
      </div>
      <Handle type="source" position={Position.Bottom} className="!bg-muted-foreground" />
    </div>
  );
}

export function WorkflowResultNode({ data }: NodeProps) {
  const d = data as StepNodeData;
  const isError = d.status === "error";
  return (
    <div
      className={cn(
        "rounded-full border px-4 py-2 text-center",
        isError
          ? "border-red-500/40 bg-red-500/10 text-red-400"
          : d.status === "running"
            ? "border-blue-500/40 bg-blue-500/10 text-blue-400"
            : "border-green-500/40 bg-green-500/10 text-green-400"
      )}
    >
      <Handle type="target" position={Position.Top} className="!bg-muted-foreground" />
      <div className="flex items-center justify-center gap-2">
        {statusIcon[d.status]}
        <span className="text-sm font-semibold">{d.label}</span>
      </div>
    </div>
  );
}

export const nodeTypes = {
  step: StepNode,
  "child-workflow": ChildWorkflowNode,
  "workflow-result": WorkflowResultNode,
};
