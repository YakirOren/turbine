import { useShow, useList } from "@refinedev/core";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Button } from "@/components/ui/button";
import { StatusBadge } from "@/components/status-badge";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { Separator } from "@/components/ui/separator";
import { Ban, Play, Copy, ChevronDown, GitBranch } from "lucide-react";
import { useState } from "react";
import { pbClient } from "@/providers/pocketbase";
import { StepFlowDialog } from "@/components/step-flow";

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
      <CollapsibleTrigger className="flex w-full items-center justify-between py-1 text-sm text-muted-foreground hover:text-foreground">
        {label}
        <ChevronDown className="h-4 w-4" />
      </CollapsibleTrigger>
      <CollapsibleContent>
        <pre className="mt-1 max-h-48 overflow-auto rounded bg-muted p-2 text-xs">
          {typeof data === "string" ? data : JSON.stringify(data, null, 2)}
        </pre>
      </CollapsibleContent>
    </Collapsible>
  );
}

export function WorkflowSidebar({
  workflowId,
  onClose,
}: {
  workflowId: string | null;
  onClose: () => void;
}) {
  const [showSteps, setShowSteps] = useState(false);

  const { query } = useShow({
    resource: "pf_workflow_status",
    id: workflowId ?? "",
    queryOptions: { enabled: !!workflowId },
  });
  const record = query?.data?.data as Record<string, unknown> | undefined;

  const { query: childQuery } = useList({
    resource: "pf_workflow_status",
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

  return (
    <>
      <Sheet open={!!workflowId} onOpenChange={() => onClose()}>
        <SheetContent className="w-[480px] overflow-y-auto sm:max-w-lg">
          {record && (
            <>
              <SheetHeader>
                <SheetTitle className="flex items-center gap-2">
                  {record.name as string}
                  <StatusBadge status={status} />
                </SheetTitle>
              </SheetHeader>

              <div className="mt-4 flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={!canCancel}
                  onClick={() =>
                    pbClient.send(
                      `/api/pf/workflows/${workflowId}/cancel`,
                      { method: "POST" }
                    )
                  }
                >
                  <Ban className="mr-1 h-4 w-4" /> Cancel
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={!canResume}
                  onClick={() =>
                    pbClient.send(
                      `/api/pf/workflows/${workflowId}/resume`,
                      { method: "POST" }
                    )
                  }
                >
                  <Play className="mr-1 h-4 w-4" /> Resume
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setShowSteps(true)}
                >
                  <GitBranch className="mr-1 h-4 w-4" /> Show Steps
                </Button>
              </div>

              <Separator className="my-4" />

              <div className="space-y-1">
                <CopyField
                  label="Workflow ID"
                  value={record.id as string}
                />
                <CopyField
                  label="Executor ID"
                  value={record.executor_id as string}
                />
                <div className="flex items-center justify-between py-1">
                  <span className="text-sm text-muted-foreground">
                    App Version
                  </span>
                  <span className="font-mono text-sm">
                    {((record.application_version as string) ?? "").slice(0, 12)}
                  </span>
                </div>
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
                <div className="flex items-center justify-between py-1">
                  <span className="text-sm text-muted-foreground">
                    Created At
                  </span>
                  <span className="text-sm">
                    {new Date(
                      record.created_at_epoch_ms as number
                    ).toLocaleString()}
                  </span>
                </div>
                <div className="flex items-center justify-between py-1">
                  <span className="text-sm text-muted-foreground">
                    Updated At
                  </span>
                  <span className="text-sm">
                    {new Date(
                      record.updated_at_epoch_ms as number
                    ).toLocaleString()}
                  </span>
                </div>
              </div>

              <Separator className="my-4" />

              <JsonSection label="Input" data={record.inputs} />
              <JsonSection label="Output" data={record.output} />
              {record.error && (
                <div className="mt-2 rounded bg-red-500/10 p-2 text-sm text-red-400">
                  {record.error as string}
                </div>
              )}

              {children.length > 0 && (
                <>
                  <Separator className="my-4" />
                  <h3 className="mb-2 text-sm font-semibold">
                    Child Workflows
                  </h3>
                  <div className="space-y-2">
                    {children.map((child: Record<string, unknown>) => (
                      <div
                        key={child.id as string}
                        className="flex items-center justify-between rounded border p-2 text-sm"
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
                      </div>
                    ))}
                  </div>
                </>
              )}
            </>
          )}
        </SheetContent>
      </Sheet>

      {showSteps && workflowId && (
        <StepFlowDialog
          workflowId={workflowId}
          onClose={() => setShowSteps(false)}
        />
      )}
    </>
  );
}
