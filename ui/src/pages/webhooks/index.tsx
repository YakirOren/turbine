import { useState } from "react";
import { useCustom } from "@refinedev/core";
import { useQueryClient } from "@tanstack/react-query";
import { pbClient } from "@/providers/pocketbase";
import { Webhook, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
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

interface WebhookRecord {
  id: string;
  url: string;
  events: string[];
  enabled: boolean;
  secret: boolean;
  created: string;
}

const EVENT_OPTIONS = [
  "workflow.SUCCESS",
  "workflow.ERROR",
  "workflow.CANCELLED",
  "workflow.*",
];

function timeAgo(dateStr: string): string {
  if (!dateStr) return "\u2014";
  const diff = Date.now() - new Date(dateStr).getTime();
  if (diff < 60000) return `${Math.round(diff / 1000)}s ago`;
  if (diff < 3600000) return `${Math.round(diff / 60000)}m ago`;
  if (diff < 86400000) return `${Math.round(diff / 3600000)}h ago`;
  return `${Math.round(diff / 86400000)}d ago`;
}

function truncateUrl(url: string, maxLen = 40): string {
  if (url.length <= maxLen) return url;
  return url.slice(0, maxLen) + "\u2026";
}

export function WebhookList() {
  const queryClient = useQueryClient();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [formUrl, setFormUrl] = useState("");
  const [formEvents, setFormEvents] = useState<string[]>([]);
  const [formSecret, setFormSecret] = useState("");
  const [formEnabled, setFormEnabled] = useState(true);
  const [submitting, setSubmitting] = useState(false);

  const { query: webhooksQuery } = useCustom<WebhookRecord[]>({
    url: "",
    method: "get",
    queryOptions: {
      queryKey: ["webhooks"],
      queryFn: () =>
        pbClient
          .send<WebhookRecord[]>("/api/pt/webhooks", { method: "GET" })
          .then((data) => ({ data })),
    },
  });

  const records =
    (webhooksQuery.data?.data as WebhookRecord[] | undefined) ?? [];
  const isLoading = webhooksQuery.isLoading && records.length === 0;

  const openAdd = () => {
    setFormUrl("");
    setFormEvents([]);
    setFormSecret("");
    setFormEnabled(true);
    setDialogOpen(true);
  };

  const handleCreate = async () => {
    if (!formUrl.trim()) return;

    setSubmitting(true);
    try {
      await pbClient.send("/api/pt/webhooks", {
        method: "POST",
        body: {
          url: formUrl,
          events: formEvents,
          secret: formSecret || undefined,
          enabled: formEnabled,
        },
      });
      queryClient.invalidateQueries({ queryKey: ["webhooks"] });
      toast.success("Webhook created");
      setDialogOpen(false);
    } catch (err: any) {
      toast.error(err?.message || "Failed to create webhook");
    } finally {
      setSubmitting(false);
    }
  };

  const handleToggle = async (record: WebhookRecord) => {
    try {
      await pbClient.send(`/api/pt/webhooks/${record.id}/toggle`, {
        method: "POST",
      });
      queryClient.invalidateQueries({ queryKey: ["webhooks"] });
    } catch (err: any) {
      toast.error(err?.message || "Failed to toggle webhook");
    }
  };

  const handleDelete = async (record: WebhookRecord) => {
    if (
      !window.confirm(
        `Delete webhook for "${record.url}"? This cannot be undone.`
      )
    )
      return;

    try {
      await pbClient.send(`/api/pt/webhooks/${record.id}`, {
        method: "DELETE",
      });
      queryClient.invalidateQueries({ queryKey: ["webhooks"] });
      toast.success("Webhook deleted");
    } catch (err: any) {
      toast.error(err?.message || "Failed to delete webhook");
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
        <h1 className="text-2xl font-bold">Webhooks</h1>
        <Button size="sm" onClick={openAdd}>
          <Plus className="mr-1.5 h-3.5 w-3.5" />
          Add Webhook
        </Button>
      </div>

      {webhooksQuery.isError && (
        <div className="rounded-md border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {(webhooksQuery.error as unknown as Error)?.message ??
            "Failed to load webhooks"}
        </div>
      )}

      {records.length === 0 && !webhooksQuery.isError ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 py-20 text-muted-foreground">
          <Webhook className="h-8 w-8" />
          <div className="text-center">
            <p>No webhooks configured</p>
            <p className="mt-1 text-xs">
              Add a webhook to receive notifications when workflows complete.
            </p>
          </div>
        </div>
      ) : (
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>URL</TableHead>
                <TableHead>Events</TableHead>
                <TableHead>Enabled</TableHead>
                <TableHead>Secret</TableHead>
                <TableHead>Created</TableHead>
                <TableHead className="w-10" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {records.map((record) => (
                <TableRow key={record.id}>
                  <TableCell className="font-mono text-sm">
                    {truncateUrl(record.url)}
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-1">
                      {record.events.map((event) => (
                        <Badge
                          key={event}
                          variant="secondary"
                          className="text-xs"
                        >
                          {event}
                        </Badge>
                      ))}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Switch
                      checked={record.enabled}
                      onCheckedChange={() => handleToggle(record)}
                    />
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {record.secret ? "(set)" : "(none)"}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {timeAgo(record.created)}
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                      onClick={() => handleDelete(record)}
                      aria-label={`Delete webhook ${record.url}`}
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
            <DialogTitle>Add Webhook</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-1.5">
              <label className="text-sm font-medium">URL</label>
              <Input
                value={formUrl}
                onChange={(e) => setFormUrl(e.target.value)}
                placeholder="https://example.com/webhook"
                className="font-mono"
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-sm font-medium">Events</label>
              <div className="flex flex-wrap gap-1.5">
                {EVENT_OPTIONS.map((opt) => {
                  const active = formEvents.includes(opt);
                  return (
                    <button
                      key={opt}
                      type="button"
                      className={`rounded-md border px-2.5 py-1 text-xs transition-colors ${
                        active
                          ? "border-primary bg-primary text-primary-foreground"
                          : "border-input bg-background text-muted-foreground hover:bg-accent hover:text-accent-foreground"
                      }`}
                      onClick={() =>
                        setFormEvents(
                          active
                            ? formEvents.filter((e) => e !== opt)
                            : [...formEvents, opt]
                        )
                      }
                    >
                      {opt}
                    </button>
                  );
                })}
              </div>
              {formEvents.length === 0 && (
                <p className="text-xs text-muted-foreground">
                  Select one or more
                </p>
              )}
            </div>
            <div className="space-y-1.5">
              <label className="text-sm font-medium">Secret</label>
              <Input
                value={formSecret}
                onChange={(e) => setFormSecret(e.target.value)}
                placeholder="Optional signing secret"
              />
            </div>
            <div className="flex items-center justify-between py-1">
              <label className="text-sm font-medium">Enabled</label>
              <Switch
                checked={formEnabled}
                onCheckedChange={setFormEnabled}
              />
            </div>
            <div className="flex justify-end">
              <Button
                onClick={handleCreate}
                disabled={!formUrl.trim() || submitting}
              >
                {submitting ? "Creating..." : "Create"}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
