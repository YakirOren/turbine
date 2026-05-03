import { useShow, useList, useInvalidate } from "@refinedev/core";
import { useMutation } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { StatusBadge, AppStatusBadge } from "@/components/status-badge";
import { TERMINAL_STATUSES } from "@/components/step-status";
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
import { DrawerShell } from "@/components/drawer-shell";
import { toast } from "sonner";
import { Ban, Play, Copy, ChevronDown, GitBranch, Trash2, CheckCircle, XCircle, X } from "lucide-react";
import { useNavigate } from "react-router";
import { pbClient } from "@/providers/pocketbase";
import { timeAgo, formatDurationRange, formatTimestamp } from "@/lib/format";
import { useMediaQuery } from "@/lib/use-media-query";
import { cn } from "@/lib/utils";

function CopyField({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between py-1">
      <span className="text-sm text-muted-foreground">{label}</span>
      <div className="flex items-center gap-1">
        <span className="font-mono text-sm">{value.slice(0, 16)}{value.length > 16 ? "..." : ""}</span>
        <Button
          variant="ghost"
          size="sm"
          aria-label={`Copy ${label}`}
          className="h-7 w-7 p-0 text-muted-foreground hover:text-foreground"
          onClick={() => navigator.clipboard.writeText(value)}
        >
          <Copy className="h-3.5 w-3.5" />
        </Button>
      </div>
    </div>
  );
}

function JsonSection({ label, data }: { label: string; data: unknown }) {
  if (!data) return null;
  return (
    <Collapsible defaultOpen>
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

function CancelResumeActions({
  workflowId,
  canCancel,
  canResume,
}: {
  workflowId: string;
  canCancel: boolean;
  canResume: boolean;
}) {
  const invalidate = useInvalidate();
  const mutation = useMutation({
    mutationFn: async (action: "cancel" | "resume") => {
      await pbClient.send(`/api/pt/workflows/${workflowId}/${action}`, { method: "POST" });
      return action;
    },
    onSuccess: (action) => {
      invalidate({ resource: "pt_workflow_status", invalidates: ["detail", "list"] });
      toast.success(action === "cancel" ? "Workflow cancelled" : "Workflow resumed");
    },
    onError: (_err, action) => {
      toast.error(`Failed to ${action} workflow`);
    },
  });

  return (
    <>
      {canCancel && (
        <Button
          variant="outline"
          size="sm"
          disabled={mutation.isPending && mutation.variables === "cancel"}
          onClick={() => mutation.mutate("cancel")}
        >
          <Ban className="mr-1 h-4 w-4" />
          {mutation.isPending && mutation.variables === "cancel" ? "Cancelling..." : "Cancel"}
        </Button>
      )}
      {canResume && (
        <Button
          variant="outline"
          size="sm"
          disabled={mutation.isPending && mutation.variables === "resume"}
          onClick={() => mutation.mutate("resume")}
        >
          <Play className="mr-1 h-4 w-4" />
          {mutation.isPending && mutation.variables === "resume" ? "Resuming..." : "Resume"}
        </Button>
      )}
    </>
  );
}

function ApprovalActions({ workflowId }: { workflowId: string }) {
  const invalidate = useInvalidate();
  const mutation = useMutation({
    mutationFn: async (approved: boolean) => {
      await pbClient.send(`/api/pt/workflows/${workflowId}/approve`, {
        method: "POST",
        body: { approved, comment: "" },
      });
      return approved;
    },
    onSuccess: (approved) => {
      invalidate({ resource: "pt_workflow_status", invalidates: ["detail", "list"] });
      toast.success(approved ? "Workflow approved" : "Workflow rejected");
    },
    onError: () => {
      toast.error("Failed to send approval decision");
    },
  });

  if (mutation.isPending || mutation.isSuccess) return null;

  return (
    <div className="mt-2 flex items-center gap-2">
      <Button
        variant="outline"
        size="sm"
        className="border-success/40 text-success-foreground hover:bg-success-soft"
        onClick={() => mutation.mutate(true)}
      >
        <CheckCircle className="mr-1 h-4 w-4" />
        Approve
      </Button>
      <Button
        variant="outline"
        size="sm"
        className="border-danger/40 text-danger-foreground hover:bg-danger-soft"
        onClick={() => mutation.mutate(false)}
      >
        <XCircle className="mr-1 h-4 w-4" />
        Reject
      </Button>
    </div>
  );
}

export function WorkflowSidebar({
  workflowId,
  onClose,
  activeTag,
  onTagClick,
}: {
  workflowId: string | null;
  onClose: () => void;
  activeTag?: string;
  onTagClick?: (tag: string) => void;
}) {
  const navigate = useNavigate();

  const { query } = useShow({
    resource: "pt_workflow_status",
    id: workflowId ?? "",
    liveMode: "auto",
    queryOptions: { enabled: !!workflowId, refetchInterval: 5000 },
  });
  const record = query?.data?.data as Record<string, unknown> | undefined;

  const { query: childQuery } = useList({
    resource: "pt_workflow_status",
    filters: [
      { field: "parent_workflow_id", operator: "eq", value: workflowId ?? "" },
    ],
    liveMode: "auto",
    queryOptions: { enabled: !!workflowId, refetchInterval: 5000 },
    pagination: { pageSize: 50 },
  });
  const children = childQuery?.data?.data ?? [];

  const status = (record?.status as string) ?? "";
  const canCancel = status === "PENDING" || status === "ENQUEUED";
  const canResume = status === "CANCELLED";
  const canDelete =
    status === "SUCCESS" ||
    status === "ERROR" ||
    status === "CANCELLED" ||
    status === "MAX_RECOVERY_ATTEMPTS_EXCEEDED";

  const createdMs = record?.created_at_epoch_ms as number | undefined;
  const updatedMs = record?.updated_at_epoch_ms as number | undefined;
  const isTerminal = TERMINAL_STATUSES.has(status);

  const isLg = useMediaQuery("(min-width: 1024px)");

  if (!workflowId) return null;

  const header = (
    <div className="flex items-start justify-between gap-3 border-b border-border-soft px-5 py-3.5">
      <div className="min-w-0 flex-1">
        {record ? (
          <>
            <div className="mb-1 flex flex-wrap items-center gap-2">
              <span className="truncate text-[15px] font-semibold">{record.name as string}</span>
              <StatusBadge status={status} />
              {typeof record.app_status === "string" && record.app_status && (
                <AppStatusBadge
                  label={record.app_status}
                  color={record.app_status_color as string}
                />
              )}
            </div>
            {typeof record.summary === "string" && record.summary && (
              <p className="text-[12.5px] text-muted-foreground">{record.summary}</p>
            )}
          </>
        ) : (
          <div className="text-sm text-muted-foreground">Loading…</div>
        )}
      </div>
      {isLg && (
        <Button
          variant="ghost"
          size="sm"
          onClick={onClose}
          aria-label="Close workflow details"
          className="h-7 w-7 shrink-0 p-0 text-muted-foreground hover:text-foreground"
        >
          <X className="h-3.5 w-3.5" />
        </Button>
      )}
    </div>
  );

  const body = (
    <>
      {header}

      <div className="flex-1 overflow-y-auto px-5 py-3">
          {record && (
            <>
              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  onClick={() => navigate(`/workflows/${workflowId}/steps`)}
                >
                  <GitBranch className="mr-1 h-4 w-4" /> Show Steps
                </Button>
                <CancelResumeActions
                  key={workflowId}
                  workflowId={workflowId}
                  canCancel={canCancel}
                  canResume={canResume}
                />
                <div className="flex-1" />
                <AlertDialog>
                  <AlertDialogTrigger asChild>
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={!canDelete}
                      aria-label="Delete workflow"
                      className="h-7 w-7 p-0 text-muted-foreground hover:bg-destructive/15 hover:text-destructive"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
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

              {typeof record.app_status === "string" && record.app_status === "waiting for approval" && (
                <ApprovalActions key={workflowId} workflowId={workflowId} />
              )}

              {/* Error: shown prominently right after actions */}
              {record.error && (
                <div className="mt-3 rounded-md border border-danger/20 bg-danger-soft px-3 py-2 text-sm text-danger-foreground">
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
                      title={formatTimestamp(createdMs)}
                    >
                      {timeAgo(createdMs)}
                    </span>
                  </div>
                )}
                {createdMs != null && updatedMs != null && isTerminal && (
                  <div className="flex items-center justify-between py-1">
                    <span className="text-sm text-muted-foreground">
                      Duration
                    </span>
                    <span className="font-mono text-sm">
                      {formatDurationRange(createdMs, updatedMs)}
                    </span>
                  </div>
                )}
                {Array.isArray(record.tags) && (record.tags as string[]).length > 0 && (
                  <div className="flex items-center justify-between py-1">
                    <span className="text-sm text-muted-foreground">Tags</span>
                    <div className="flex flex-wrap gap-1">
                      {(record.tags as string[]).map((tag) => {
                        const isActive = activeTag === tag;
                        if (!onTagClick) {
                          return (
                            <Badge key={tag} variant="secondary" className="text-xs">
                              {tag}
                            </Badge>
                          );
                        }
                        return (
                          <Badge
                            key={tag}
                            asChild
                            variant={isActive ? "default" : "secondary"}
                            className={cn(
                              "cursor-pointer text-xs transition-colors",
                              isActive
                                ? "hover:bg-primary/85"
                                : "hover:bg-secondary/70 hover:text-foreground",
                            )}
                          >
                            <button
                              type="button"
                              aria-pressed={isActive}
                              onClick={() => onTagClick(isActive ? "" : tag)}
                            >
                              {tag}
                            </button>
                          </Badge>
                        );
                      })}
                    </div>
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
                        className="flex w-full items-center justify-between rounded-md border p-2 text-sm hover:bg-accent"
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
      </div>
    </>
  );

  return (
    <DrawerShell
      width="w-[380px]"
      sheetClassName="gap-0"
      srLabel="Workflow details"
      onClose={onClose}
    >
      {body}
    </DrawerShell>
  );
}
