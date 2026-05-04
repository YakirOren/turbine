import { useState, useMemo } from "react";
import { useList, useInvalidate } from "@refinedev/core";
import { useMutation } from "@tanstack/react-query";
import { Package, Download, RefreshCw } from "lucide-react";
import { toast } from "sonner";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Input } from "@/components/ui/input";
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from "@/components/ui/combobox";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { CodeMirrorEditor } from "@/components/codemirror";
import { TableSkeleton } from "@/components/table-skeleton";
import { DocLink } from "@/components/doc-link";
import { ProductStatusBadge } from "@/components/status-badge";
import { WorkflowSidebar } from "@/components/workflow-sidebar";
import { timeAgo, formatTimestampPrecise, formatBytes } from "@/lib/format";
import { pbClient } from "@/providers/pocketbase";
import type { PtProductsResponse, PtWorkflowStatusResponse } from "@/types/pocketbase-types";
import type { CrudFilters } from "@refinedev/core";

type Product = PtProductsResponse<Record<string, unknown>>;

export function ProductList() {
  const invalidate = useInvalidate();
  const [selectedWorkflowId, setSelectedWorkflowId] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState<"all" | "sent" | "failed" | "stored">("all");
  const [fileNameFilter, setFileNameFilter] = useState("");
  const [workflowNameFilter, setWorkflowNameFilter] = useState("");
  const [productToResend, setProductToResend] = useState<Product | null>(null);

  const crudFilters = useMemo<CrudFilters>(() => {
    const f: CrudFilters = [];
    if (statusFilter !== "all") f.push({ field: "status", operator: "eq", value: statusFilter });
    if (fileNameFilter) f.push({ field: "file_name", operator: "contains", value: fileNameFilter });
    return f;
  }, [statusFilter, fileNameFilter]);

  const { result, query } = useList<Product>({
    resource: "pt_products",
    filters: crudFilters,
    sorters: [{ field: "created", order: "desc" }],
    pagination: { pageSize: 50 },
  });

  const records = result.data ?? [];

  const workflowIds = useMemo(
    () => Array.from(new Set(records.map((r) => r.workflow_id))),
    [records],
  );

  const { result: wfResult } = useList<PtWorkflowStatusResponse>({
    resource: "pt_workflow_status",
    filters: workflowIds.length > 0 ? [{ field: "id", operator: "in", value: workflowIds }] : [],
    pagination: { mode: "off" },
    queryOptions: { enabled: workflowIds.length > 0 },
  });

  const workflowNameById = useMemo(() => {
    const m = new Map<string, string>();
    for (const w of wfResult.data ?? []) m.set(w.id, w.name);
    return m;
  }, [wfResult.data]);

  const workflowNames = useMemo(
    () => Array.from(new Set(workflowNameById.values())).sort(),
    [workflowNameById],
  );

  const filteredRecords = useMemo(() => {
    if (!workflowNameFilter) return records;
    return records.filter(
      (r) => workflowNameById.get(r.workflow_id) === workflowNameFilter,
    );
  }, [records, workflowNameFilter, workflowNameById]);

  const resendMutation = useMutation({
    mutationFn: async (product: Product) => {
      const res = await pbClient.send(`/api/pt/products/${product.id}/resend`, { method: "POST" });
      if (res?.error) throw new Error(res.error);
    },
    onSuccess: () => {
      invalidate({ resource: "pt_products", invalidates: ["list"] });
      toast.success("Product resent successfully");
      setProductToResend(null);
    },
    onError: (err: unknown) => {
      const msg = err instanceof Error ? err.message : "Failed to resend product";
      toast.error(msg);
      setProductToResend(null);
    },
  });

  const isLoading = query.isLoading && records.length === 0;

  if (isLoading) {
    return (
      <div className="flex h-full min-h-0 flex-1 bg-card">
        <div className="flex min-w-0 flex-1 flex-col overflow-y-auto p-6 space-y-4">
          <h1 className="text-2xl font-semibold tracking-tight">Products</h1>
          <TableSkeleton
            columns={8}
            headers={["Time", "File", "Workflow", "Step", "Status", "Size", "Metadata", ""]}
          />
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-1 bg-card">
      <div className="flex min-w-0 flex-1 flex-col overflow-y-auto p-6 space-y-4">
      <h1 className="text-2xl font-semibold tracking-tight">Products</h1>

      {query.isError && (
        <div className="rounded-md border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {(query.error as unknown as Error)?.message ?? "Failed to load products"}
        </div>
      )}

      <div className="flex flex-wrap gap-2">
        <Select value={statusFilter} onValueChange={(v) => setStatusFilter(v as typeof statusFilter)}>
          <SelectTrigger className="w-36">
            <SelectValue placeholder="Status" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All statuses</SelectItem>
            <SelectItem value="stored">Stored</SelectItem>
            <SelectItem value="sent">Sent</SelectItem>
            <SelectItem value="failed">Failed</SelectItem>
          </SelectContent>
        </Select>
        <Input
          className="w-48"
          placeholder="Filter by file name…"
          value={fileNameFilter}
          onChange={(e) => setFileNameFilter(e.target.value)}
        />
        <Combobox
          value={workflowNameFilter || null}
          onValueChange={(v) => setWorkflowNameFilter(v ?? "")}
          items={workflowNames}
        >
          <ComboboxInput
            placeholder="All workflows"
            className="w-52"
            showClear={!!workflowNameFilter}
          />
          <ComboboxContent>
            <ComboboxEmpty>No workflows found.</ComboboxEmpty>
            <ComboboxList>
              {(item) => (
                <ComboboxItem key={item} value={item}>
                  {item}
                </ComboboxItem>
              )}
            </ComboboxList>
          </ComboboxContent>
        </Combobox>
      </div>

      {filteredRecords.length === 0 && !query.isError ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 py-20 text-muted-foreground">
          <Package className="h-8 w-8" />
          <div className="text-center">
            <p>No products generated.</p>
            <p className="mt-1 text-xs">
              Generate a product from a workflow step using{" "}
              <code className="rounded bg-muted px-1.5 py-0.5 font-mono">
                turbine.SendProduct(ctx, fileName, data, metadata)
              </code>
            </p>
          </div>
          <DocLink path="concepts/products" className="text-xs">
            Learn more
          </DocLink>
        </div>
      ) : (
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Time</TableHead>
                <TableHead>File</TableHead>
                <TableHead>Workflow</TableHead>
                <TableHead>Step</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Size</TableHead>
                <TableHead>Metadata</TableHead>
                <TableHead className="w-20" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {filteredRecords.map((p) => (
                <TableRow key={p.id}>
                  <TableCell
                    className="font-mono text-xs text-muted-foreground"
                    title={formatTimestampPrecise(p.created)}
                  >
                    {timeAgo(new Date(p.created).getTime())}
                  </TableCell>
                  <TableCell className="font-mono text-sm">{p.file_name}</TableCell>
                  <TableCell>
                    <button
                      type="button"
                      className="text-sm underline-offset-2 hover:underline"
                      onClick={() => setSelectedWorkflowId(p.workflow_id)}
                    >
                      {workflowNameById.get(p.workflow_id) ?? p.workflow_id.slice(0, 8)}
                    </button>
                  </TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">
                    {p.function_name || `Step ${p.function_id}`}
                  </TableCell>
                  <TableCell>
                    <ProductStatusBadge status={p.status} />
                  </TableCell>
                  <TableCell className="text-right font-mono text-xs text-muted-foreground">
                    {formatBytes(p.size)}
                  </TableCell>
                  <TableCell className="max-w-48">
                    {p.metadata ? (
                      <Popover>
                        <PopoverTrigger asChild>
                          <button
                            type="button"
                            className="truncate block font-mono text-xs text-muted-foreground hover:text-foreground w-full text-left"
                          >
                            {JSON.stringify(p.metadata)}
                          </button>
                        </PopoverTrigger>
                        <PopoverContent className="w-80 p-0" align="start">
                          <CodeMirrorEditor
                            value={JSON.stringify(p.metadata, null, 2)}
                            onChange={() => {}}
                            readOnly
                            minHeight="60px"
                            maxHeight="300px"
                          />
                        </PopoverContent>
                      </Popover>
                    ) : (
                      <span className="font-mono text-xs text-muted-foreground">—</span>
                    )}
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-muted-foreground hover:text-foreground"
                        onClick={() => setProductToResend(p)}
                        aria-label="Resend product"
                      >
                        <RefreshCw className="h-4 w-4" />
                      </Button>
                      {p.file && (
                        <Button asChild variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-foreground">
                          <a
                            href={pbClient.files.getUrl(p, p.file)}
                            download={p.file_name}
                            aria-label="Download file"
                          >
                            <Download className="h-4 w-4" />
                          </a>
                        </Button>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <AlertDialog open={!!productToResend} onOpenChange={(open) => { if (!open) setProductToResend(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Resend product?</AlertDialogTitle>
            <AlertDialogDescription>
              This will re-send{" "}
              <span className="font-mono">{productToResend?.file_name}</span> to the
              configured product sender. The previous failure status will be replaced.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => productToResend && resendMutation.mutate(productToResend)}
              disabled={resendMutation.isPending}
            >
              {resendMutation.isPending ? "Resending…" : "Resend"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      </div>
      <WorkflowSidebar
        workflowId={selectedWorkflowId}
        onClose={() => setSelectedWorkflowId(null)}
      />
    </div>
  );
}
