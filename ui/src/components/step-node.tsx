import { Handle, Position, type NodeProps } from "@xyflow/react";
import { cn } from "@/lib/utils";
import { formatDuration } from "@/lib/format";
import { GitBranch, Loader2 } from "lucide-react";
import { fallbackStatusStyle, statusStyles } from "@/components/step-status";

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

function nodeShell(selected: boolean | undefined, status: string) {
  const s = statusStyles[status] ?? fallbackStatusStyle;
  return cn(
    "group relative flex min-w-[128px] items-center gap-2 rounded-lg bg-card px-2.5 py-1.5 shadow-xs dark:shadow-none cursor-pointer transition-colors hover:bg-muted",
    "border",
    s.border,
    selected && cn("bg-accent ring-1 ring-inset", s.ring),
  );
}

function StatusDot({ status }: { status: string }) {
  const s = statusStyles[status] ?? fallbackStatusStyle;
  return (
    <span
      className={cn(
        "inline-flex h-4 w-4 shrink-0 items-center justify-center rounded-full border",
        s.fill,
        s.border,
      )}
    >
      {s.icon}
    </span>
  );
}

export function StepNode({ data }: NodeProps) {
  const d = data as StepNodeData;
  return (
    <div className={nodeShell(d.selected, d.status)}>
      {d.hasInput !== false && (
        <Handle type="target" position={Position.Left} className="!bg-muted-foreground" />
      )}
      <StatusDot status={d.status} />
      <span className="flex-1 font-mono text-[12px] font-semibold text-foreground">
        {d.label}
      </span>
      {d.durationMs != null && (
        <span className="font-mono text-[11px] text-muted-foreground">
          {formatDuration(d.durationMs)}
        </span>
      )}
      {d.hasOutput !== false && (
        <Handle type="source" position={Position.Right} className="!bg-muted-foreground" />
      )}
    </div>
  );
}

export function ChildWorkflowNode({ data }: NodeProps) {
  const d = data as StepNodeData;
  return (
    <div
      className={cn(nodeShell(d.selected, d.status), "border-dashed")}
      onClick={() => d.childWorkflowId && d.onChildClick?.(d.childWorkflowId)}
    >
      {d.hasInput !== false && (
        <Handle type="target" position={Position.Left} className="!bg-muted-foreground" />
      )}
      <GitBranch className="h-3 w-3 shrink-0 text-muted-foreground" />
      <StatusDot status={d.status} />
      <span className="flex-1 font-mono text-[12px] font-semibold text-foreground">
        {d.label}
      </span>
      {d.durationMs != null && (
        <span className="font-mono text-[11px] text-muted-foreground">
          {formatDuration(d.durationMs)}
        </span>
      )}
      {d.hasOutput !== false && (
        <Handle type="source" position={Position.Right} className="!bg-muted-foreground" />
      )}
    </div>
  );
}

export function WorkflowResultNode({ data }: NodeProps) {
  const d = data as StepNodeData;
  const tone =
    d.status === "error"
      ? "bg-danger-soft text-danger-foreground border-danger"
      : d.status === "running"
        ? "bg-info-soft text-info-foreground border-info"
        : "bg-success-soft text-success-foreground border-success";
  return (
    <div className={cn("rounded-full border px-3 py-1 text-center", tone)}>
      <Handle type="target" position={Position.Left} className="!bg-muted-foreground" />
      <div className="flex items-center justify-center gap-1.5">
        <StatusDot status={d.status === "error" ? "error" : d.status === "running" ? "running" : "success"} />
        <span className="font-mono text-[11px] font-semibold">{d.label}</span>
      </div>
    </div>
  );
}

export function ApprovalNode({ data }: NodeProps) {
  const d = data as StepNodeData;
  return (
    <div
      className={cn(
        "flex min-w-[140px] items-center gap-2 rounded-lg border border-warning bg-warning-soft px-2.5 py-1.5 shadow-sm",
      )}
    >
      <Handle type="target" position={Position.Left} className="!bg-muted-foreground" />
      <Loader2 className="h-3 w-3 shrink-0 animate-spin text-warning" />
      <span className="font-mono text-[12px] font-semibold text-warning-foreground">
        {d.label}
      </span>
      <Handle type="source" position={Position.Right} className="!bg-muted-foreground" />
    </div>
  );
}

export const nodeTypes = {
  step: StepNode,
  "child-workflow": ChildWorkflowNode,
  "workflow-result": WorkflowResultNode,
  approval: ApprovalNode,
};
