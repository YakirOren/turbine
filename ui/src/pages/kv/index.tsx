import { useState } from "react";
import { useCustom } from "@refinedev/core";
import { useQueryClient } from "@tanstack/react-query";
import { pbClient } from "@/providers/pocketbase";
import { Database, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

interface KVRecord {
  id: string;
  key: string;
  value: unknown;
  updated_at_epoch_ms: number;
}

function timeAgo(epochMs: number): string {
  if (!epochMs) return "\u2014";
  const diff = Date.now() - epochMs;
  if (diff < 60000) return `${Math.round(diff / 1000)}s ago`;
  if (diff < 3600000) return `${Math.round(diff / 60000)}m ago`;
  if (diff < 86400000) return `${Math.round(diff / 3600000)}h ago`;
  return `${Math.round(diff / 86400000)}d ago`;
}

function truncateValue(value: unknown, maxLen = 60): string {
  const str = JSON.stringify(value);
  if (str.length <= maxLen) return str;
  return str.slice(0, maxLen) + "\u2026";
}

export function KVList() {
  const queryClient = useQueryClient();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editRecord, setEditRecord] = useState<KVRecord | null>(null);
  const [formKey, setFormKey] = useState("");
  const [formValue, setFormValue] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const { query: kvQuery } = useCustom<KVRecord[]>({
    url: "",
    method: "get",
    queryOptions: {
      queryKey: ["kv"],
      queryFn: () =>
        pbClient
          .send<KVRecord[]>("/api/pt/kv", { method: "GET" })
          .then((data) => ({ data })),
    },
  });

  const records = (kvQuery.data?.data as KVRecord[] | undefined) ?? [];
  const isLoading = kvQuery.isLoading && records.length === 0;

  const openAdd = () => {
    setEditRecord(null);
    setFormKey("");
    setFormValue("");
    setDialogOpen(true);
  };

  const openEdit = (record: KVRecord) => {
    setEditRecord(record);
    setFormKey(record.key);
    setFormValue(JSON.stringify(record.value, null, 2));
    setDialogOpen(true);
  };

  const handleSubmit = async () => {
    if (!formKey.trim()) return;

    let parsed: unknown;
    try {
      parsed = JSON.parse(formValue);
    } catch {
      toast.error("Invalid JSON value");
      return;
    }

    setSubmitting(true);
    try {
      await pbClient.send(`/api/pt/kv/${encodeURIComponent(formKey)}`, {
        method: "PUT",
        body: { value: parsed },
      });
      queryClient.invalidateQueries({ queryKey: ["kv"] });
      toast.success(editRecord ? "Key updated" : "Key created");
      setDialogOpen(false);
    } catch (err: any) {
      toast.error(err?.message || "Failed to save");
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (record: KVRecord) => {
    if (!window.confirm(`Delete key "${record.key}"? This cannot be undone.`))
      return;

    try {
      await pbClient.send(`/api/pt/kv/${encodeURIComponent(record.key)}`, {
        method: "DELETE",
      });
      queryClient.invalidateQueries({ queryKey: ["kv"] });
      toast.success("Key deleted");
    } catch (err: any) {
      toast.error(err?.message || "Failed to delete");
    }
  };

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center text-muted-foreground">
        Loading...
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">KV Store</h1>
        <Button size="sm" onClick={openAdd}>
          <Plus className="mr-1.5 h-3.5 w-3.5" />
          Add Key
        </Button>
      </div>

      {kvQuery.isError && (
        <div className="rounded-md border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {(kvQuery.error as unknown as Error)?.message ??
            "Failed to load KV data"}
        </div>
      )}

      {records.length === 0 && !kvQuery.isError ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 py-20 text-muted-foreground">
          <Database className="h-8 w-8" />
          <div className="text-center">
            <p>No keys stored.</p>
            <p className="mt-1 text-xs">
              Add a key-value pair using the button above or{" "}
              <code className="rounded bg-muted px-1.5 py-0.5 font-mono">
                turbine.KV.Set(ctx, key, value)
              </code>
            </p>
          </div>
        </div>
      ) : (
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Key</TableHead>
                <TableHead>Value</TableHead>
                <TableHead>Updated</TableHead>
                <TableHead className="w-10" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {records.map((record) => (
                <TableRow
                  key={record.id}
                  className="cursor-pointer"
                  onClick={() => openEdit(record)}
                >
                  <TableCell className="font-mono text-sm font-medium">
                    {record.key}
                  </TableCell>
                  <TableCell className="max-w-xs font-mono text-xs text-muted-foreground">
                    {truncateValue(record.value)}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {timeAgo(record.updated_at_epoch_ms)}
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                      onClick={(e) => {
                        e.stopPropagation();
                        handleDelete(record);
                      }}
                      aria-label={`Delete key ${record.key}`}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{editRecord ? "Edit Key" : "Add Key"}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-1.5">
              <label className="text-sm font-medium">Key</label>
              <Input
                value={formKey}
                onChange={(e) => setFormKey(e.target.value)}
                placeholder="my-key"
                className="font-mono"
                readOnly={!!editRecord}
                disabled={!!editRecord}
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-sm font-medium">Value (JSON)</label>
              <textarea
                className="flex min-h-[120px] w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                value={formValue}
                onChange={(e) => setFormValue(e.target.value)}
                placeholder="{}"
              />
            </div>
            <div className="flex justify-end">
              <Button
                onClick={handleSubmit}
                disabled={!formKey.trim() || submitting}
              >
                {submitting ? "Saving..." : "Save"}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
