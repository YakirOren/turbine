import { useState } from "react";
import { useShow, useList, useInvalidate } from "@refinedev/core";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Button } from "@/components/ui/button";
import { StatusBadge, AppStatusBadge } from "@/components/status-badge";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { Separator } from "@/components/ui/separator";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { toast } from "sonner";
import { Ban, Play, Copy, ChevronDown, GitBranch, Trash2, CheckCircle, XCircle } from "lucide-react";
import { useNavigate } from "react-router";
import { pbClient } from "@/providers/pocketbase";

function formatRelativeTime(epochMs: number): string {
  const diff = Date.now() - epochMs;
  if (diff < 0) return "just now";
  const seconds = Math.floor(diff / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function formatDuration(startMs: number, endMs: number): string {
  const diff = endMs - startMs;
  if (diff < 1000) return `${diff}ms`;
  const seconds = Math.floor(diff / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const remainSec = seconds % 60;
  if (minutes < 60) return `${minutes}m ${remainSec}s`;
  const hours = Math.floor(minutes / 60);
  const remainMin = minutes % 60;
  return `${hours}h ${remainMin}m`;
}

function CopyField({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between py-1">
      <span className="text-sm text-muted-foreground">{label}</span>
      <div className="flex items-center gap-1">
        <span className="font-mono text-sm">{value.slice(0, 16)}{value.length > 16 ? "..." : ""}</span>
        <Button
          variant="ghost"
          size="sm"
          className="h-6 w-6 p-0"
          onClick={() => navigator.clipboard.writeText(value)}
        >
          <Copy className="h-3 w-3" />
        </Button>
      </div>
    </div>
  );
}

function JsonSection({ label, data }: { label: string; data: unknown }) {
  if (!data) return null;
  return (
    <Collapsible>
      <CollapsibleTrigger className="group flex w-full items-center justify-between py-1 text-sm text-muted-foreground hover:text-foreground">
        {label}
        <ChevronDown className="h-4 w-4 transition-transform group-data-[state=open]:rotate-180" />
      </CollapsibleTrigger>
      <CollapsibleContent>
        <pre className="mt-1 max-h-48 overflow-auto rounded bg-muted p-2 text-xs">
          {typeof data === "string" ? data : JSON.stringify(data, null, 2)}
        </pre>
      </CollapsibleContent>
    </Collapsible>
  );
}

function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <span className="text-xs uppercase tracking-wide text-muted-foreground">
      {children}
    </span>
  );
}

export function WorkflowSidebar({
  workflowId,
  onClose,
}: {
  workflowId: string | null;
  onClose: () => void;
}) {
  const navigate = useNavigate();
  const invalidate = useInvalidate();
  const [actionLoading, setActionLoading] = useState<"cancel" | "resume" | null>(null);

  const handleAction = async (action: "cancel" | "resume") => {
    setActionLoading(action);
    try {
      await pbClient.send(`/api/pt/workflows/${workflowId}/${action}`, { method: "POST" });
      invalidate({ resource: "pt_workflow_status", invalidates: ["detail", "list"] });
      toast.success(action === "cancel" ? "Workflow cancelled" : "Workflow resumed");
    } catch {
      toast.error(`Failed to ${action} workflow`);
    } finally {
      setActionLoading(null);
    }
  };

  const [approvalLoading, setApprovalLoading] = useState<"approve" | "reject" | null>(null);

  const handleApproval = async (approved: boolean) => {
    setApprovalLoading(approved ? "approve" : "reject");
    try {
      await pbClient.send(`/api/pt/workflows/${workflowId}/approve`, {
        method: "POST",
        body: { approved, comment: "" },
      });
      invalidate({ resource: "pt_workflow_status", invalidates: ["detail", "list"] });
      toast.success(approved ? "Workflow approved" : "Workflow rejected");
    } catch {
      toast.error("Failed to send approval decision");
    } finally {
      setApprovalLoading(null);
    }
  };

  const { query } = useShow({
    resource: "pt_workflow_status",
    id: workflowId ?? "",
    queryOptions: { enabled: !!workflowId },
  });
  const record = query?.data?.data as Record<string, unknown> | undefined;

  const { query: childQuery } = useList({
    resource: "pt_workflow_status",
    filters: [
      { field: "parent_workflow_id", operator: "eq", value: workflowId ?? "" },
    ],
    queryOptions: { enabled: !!workflowId },
    pagination: { pageSize: 50 },
  });
  const children = childQuery?.data?.data ?? [];

  const status = (record?.status as string) ?? "";
  const canCancel = status === "PENDING" || status === "ENQUEUED";
  const canResume =
    status === "ERROR" ||
    status === "CANCELLED" ||
    status === "MAX_RECOVERY_ATTEMPTS_EXCEEDED";
  const canDelete =
    status === "SUCCESS" ||
    status === "ERROR" ||
    status === "CANCELLED" ||
    status === "MAX_RECOVERY_ATTEMPTS_EXCEEDED";

  const createdMs = record?.created_at_epoch_ms as number | undefined;
  const updatedMs = record?.updated_at_epoch_ms as number | undefined;
  const isTerminal = status === "SUCCESS" || status === "ERROR" || status === "CANCELLED" || status === "MAX_RECOVERY_ATTEMPTS_EXCEEDED";

  return (
    <Sheet open={!!workflowId} onOpenChange={() => onClose()}>
      <SheetContent className="w-[480px] overflow-y-auto p-6 sm:max-w-lg">
          {record && (
            <>
              <SheetHeader>
                <SheetTitle className="flex items-center gap-2">
                  {record.name as string}
                  <StatusBadge status={status} />
                  {typeof record.app_status === "string" && record.app_status && (
                    <AppStatusBadge
                      label={record.app_status}
                      color={record.app_status_color as string}
                    />
                  )}
                </SheetTitle>
              </SheetHeader>

              {/* Actions: Show Steps is primary, Cancel/Resume only when applicable, Delete separated */}
              <div className="mt-4 flex items-center gap-2">
                <Button
                  size="sm"
                  onClick={() => navigate(`/workflows/${workflowId}/steps`)}
                >
                  <GitBranch className="mr-1 h-4 w-4" /> Show Steps
                </Button>
                {canCancel && (
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={actionLoading === "cancel"}
                    onClick={() => handleAction("cancel")}
                  >
                    <Ban className="mr-1 h-4 w-4" />
                    {actionLoading === "cancel" ? "Cancelling..." : "Cancel"}
                  </Button>
                )}
                {canResume && (
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={actionLoading === "resume"}
                    onClick={() => handleAction("resume")}
                  >
                    <Play className="mr-1 h-4 w-4" />
                    {actionLoading === "resume" ? "Resuming..." : "Resume"}
                  </Button>
                )}
                {typeof record.app_status === "string" && record.app_status === "waiting for approval" && (
                  <>
                    <Button
                      variant="outline"
                      size="sm"
                      className="border-green-500/40 text-green-600 hover:bg-green-500/10 dark:text-green-400"
                      disabled={approvalLoading !== null}
                      onClick={() => handleApproval(true)}
                    >
                      <CheckCircle className="mr-1 h-4 w-4" />
                      {approvalLoading === "approve" ? "Approving..." : "Approve"}
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      className="border-red-500/40 text-red-600 hover:bg-red-500/10 dark:text-red-400"
                      disabled={approvalLoading !== null}
                      onClick={() => handleApproval(false)}
                    >
                      <XCircle className="mr-1 h-4 w-4" />
                      {approvalLoading === "reject" ? "Rejecting..." : "Reject"}
                    </Button>
                  </>
                )}
                <div className="flex-1" />
                <AlertDialog>
                  <AlertDialogTrigger asChild>
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={!canDelete}
                      className="h-8 w-8 p-0 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>Delete workflow?</AlertDialogTitle>
                      <AlertDialogDescription>
                        This will permanently delete this workflow execution and
                        all its steps, events, and notifications. Logs will be
                        preserved.
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel>Cancel</AlertDialogCancel>
                      <AlertDialogAction
                        variant="destructive"
                        onClick={() =>
                          pbClient
                            .collection("pt_workflow_status")
                            .delete(workflowId!)
                            .then(() => {
                              toast.success("Workflow deleted");
                              onClose();
                            })
                            .catch(() => toast.error("Failed to delete workflow"))
                        }
                      >
                        Delete
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              </div>

              {/* Error: shown prominently right after actions */}
              {record.error && (
                <div className="mt-3 rounded-md border border-red-500/20 bg-red-500/10 px-3 py-2 text-sm text-red-400">
                  {record.error as string}
                </div>
              )}

              <Separator className="my-4" />

              {/* Details section */}
              <SectionLabel>Details</SectionLabel>
              <div className="mt-1 space-y-1">
                <CopyField
                  label="Workflow ID"
                  value={record.id as string}
                />
                <CopyField
                  label="App Version"
                  value={(record.application_version as string) ?? ""}
                />
                {typeof record.queue_name === "string" && (
                  <div className="flex items-center justify-between py-1">
                    <span className="text-sm text-muted-foreground">
                      Queue
                    </span>
                    <span className="text-sm">{record.queue_name as string}</span>
                  </div>
                )}
                {(record.priority as number) > 0 && (
                  <div className="flex items-center justify-between py-1">
                    <span className="text-sm text-muted-foreground">
                      Priority
                    </span>
                    <span className="text-sm">{record.priority as number}</span>
                  </div>
                )}
                {createdMs != null && (
                  <div className="flex items-center justify-between py-1">
                    <span className="text-sm text-muted-foreground">
                      Created
                    </span>
                    <span
                      className="text-sm"
                      title={new Date(createdMs).toLocaleString()}
                    >
                      {formatRelativeTime(createdMs)}
                    </span>
                  </div>
                )}
                {createdMs != null && updatedMs != null && isTerminal && (
                  <div className="flex items-center justify-between py-1">
                    <span className="text-sm text-muted-foreground">
                      Duration
                    </span>
                    <span className="font-mono text-sm">
                      {formatDuration(createdMs, updatedMs)}
                    </span>
                  </div>
                )}
              </div>

              <Separator className="my-4" />

              {/* Data section */}
              <SectionLabel>Data</SectionLabel>
              <div className="mt-1">
                <JsonSection label="Input" data={record.inputs} />
                <JsonSection label="Output" data={record.output} />
              </div>

              {children.length > 0 && (
                <>
                  <Separator className="my-4" />
                  <SectionLabel>Child Workflows</SectionLabel>
                  <div className="mt-1 space-y-2">
                    {children.map((child: Record<string, unknown>) => (
                      <button
                        type="button"
                        key={child.id as string}
                        className="flex w-full items-center justify-between rounded border p-2 text-sm hover:bg-accent"
                        onClick={() => navigate(`/workflows/${child.id}/steps`)}
                      >
                        <div>
                          <span className="font-medium">
                            {child.name as string}
                          </span>
                          <span className="ml-2 font-mono text-xs text-muted-foreground">
                            {(child.id as string).slice(0, 8)}
                          </span>
                        </div>
                        <StatusBadge status={child.status as string} />
                      </button>
                    ))}
                  </div>
                </>
              )}
            </>
          )}
        </SheetContent>
    </Sheet>
  );
}
