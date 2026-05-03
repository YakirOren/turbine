import { useState } from "react";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useList, useInvalidate } from "@refinedev/core";
import { useMutation } from "@tanstack/react-query";
import { pbClient } from "@/providers/pocketbase";
import { Webhook, Plus, Trash2, Pencil } from "lucide-react";
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
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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
import { Field, FieldLabel, FieldDescription } from "@/components/ui/field";
import { TableSkeleton } from "@/components/table-skeleton";
import { DocLink } from "@/components/doc-link";
import { EventChips, WORKFLOW_EVENT_OPTIONS } from "@/components/event-chips";
import type { PtWebhooksResponse } from "@/types/pocketbase-types";

type WebhookRecord = PtWebhooksResponse<string[]> & { events: string[] };

const webhookFormSchema = z.object({
  url: z.string().url("Invalid URL"),
  events: z.array(z.string()).min(1, "Select at least one event"),
  secret: z.string(),
  enabled: z.boolean(),
});
type WebhookFormValues = z.infer<typeof webhookFormSchema>;

function timeAgo(dateStr: string): string {
  if (!dateStr) return "\u2014";
  const diff = Date.now() - new Date(dateStr).getTime();
  if (diff < 60000) return `${Math.round(diff / 1000)}s ago`;
  if (diff < 3600000) return `${Math.round(diff / 60000)}m ago`;
  if (diff < 86400000) return `${Math.round(diff / 3600000)}h ago`;
  return `${Math.round(diff / 86400000)}d ago`;
}

export function WebhookList() {
  const invalidate = useInvalidate();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);

  const { control, handleSubmit: rhfSubmit, reset, watch } = useForm<WebhookFormValues>({
    resolver: zodResolver(webhookFormSchema),
    defaultValues: {
      url: "",
      events: [],
      secret: "",
      enabled: true,
    },
  });

  const events = watch("events");

  const { result, query: webhooksQuery } = useList<WebhookRecord>({
    resource: "pt_webhooks",
    pagination: { mode: "off" },
  });

  const records = result.data ?? [];
  const isLoading = webhooksQuery.isLoading && records.length === 0;

  const invalidateWebhooks = () =>
    invalidate({ resource: "pt_webhooks", invalidates: ["list"] });

  const saveMutation = useMutation({
    mutationFn: async (data: WebhookFormValues) => {
      const body: Record<string, unknown> = {
        url: data.url,
        events: data.events,
        enabled: data.enabled,
      };
      if (data.secret) {
        body.secret = data.secret;
      }
      if (editingId) {
        await pbClient.collection("pt_webhooks").update(editingId, body);
      } else {
        await pbClient.collection("pt_webhooks").create(body);
      }
    },
    onSuccess: () => {
      invalidateWebhooks();
      toast.success(editingId ? "Webhook updated" : "Webhook created");
      setDialogOpen(false);
    },
    onError: (err: any) => {
      toast.error(err?.message || `Failed to ${editingId ? "update" : "create"} webhook`);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) =>
      pbClient.collection("pt_webhooks").delete(id),
    onSuccess: () => {
      invalidateWebhooks();
      toast.success("Webhook deleted");
    },
    onError: (err: any) => {
      toast.error(err?.message || "Failed to delete webhook");
    },
  });

  const toggleMutation = useMutation({
    mutationFn: (record: WebhookRecord) =>
      pbClient.collection("pt_webhooks").update(record.id, { enabled: !record.enabled }),
    onSuccess: () => {
      invalidateWebhooks();
    },
    onError: (err: any) => {
      toast.error(err?.message || "Failed to toggle webhook");
    },
  });

  const openAdd = () => {
    reset();
    setEditingId(null);
    setDialogOpen(true);
  };

  const openEdit = (record: WebhookRecord) => {
    reset({ url: record.url, events: [...record.events], secret: "", enabled: record.enabled });
    setEditingId(record.id);
    setDialogOpen(true);
  };

  if (isLoading) {
    return (
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-semibold tracking-tight">Webhooks</h1>
        </div>
        <TableSkeleton columns={6} headers={["URL", "Events", "Enabled", "Secret", "Created", ""]} />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold tracking-tight">Webhooks</h1>
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
          <DocLink path="concepts/webhooks" className="text-xs">
            Learn more
          </DocLink>
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
                <TableHead className="w-20" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {records.map((record) => (
                <TableRow key={record.id}>
                  <TableCell className="w-full max-w-0">
                    <div className="truncate font-mono text-sm" title={record.url}>
                      {record.url}
                    </div>
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
                      onCheckedChange={() => toggleMutation.mutate(record)}
                      aria-label={`${record.enabled ? "Disable" : "Enable"} webhook`}
                    />
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {record.secret ? "(set)" : "(none)"}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {timeAgo(record.created)}
                  </TableCell>
                  <TableCell className="w-20">
                    <div className="flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-muted-foreground hover:text-foreground"
                        onClick={() => openEdit(record)}
                        aria-label={`Edit webhook ${record.url}`}
                      >
                        <Pencil className="h-4 w-4" />
                      </Button>
                      <AlertDialog>
                        <AlertDialogTrigger asChild>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                            aria-label={`Delete webhook ${record.url}`}
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </AlertDialogTrigger>
                        <AlertDialogContent>
                          <AlertDialogHeader>
                            <AlertDialogTitle>Delete webhook?</AlertDialogTitle>
                            <AlertDialogDescription>
                              This will permanently delete the webhook. This cannot be undone.
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel>Cancel</AlertDialogCancel>
                            <AlertDialogAction
                              variant="destructive"
                              onClick={() => deleteMutation.mutate(record.id)}
                            >
                              Delete
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                    </div>
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
            <DialogTitle>{editingId ? "Edit Webhook" : "Add Webhook"}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <Field>
              <FieldLabel htmlFor="webhook-url">URL</FieldLabel>
              <Controller
                control={control}
                name="url"
                render={({ field }) => (
                  <Input
                    {...field}
                    id="webhook-url"
                    placeholder="https://example.com/webhook"
                    className="font-mono"
                  />
                )}
              />
            </Field>
            <Field>
              <FieldLabel id="webhook-events-label">Events</FieldLabel>
              <Controller
                control={control}
                name="events"
                render={({ field }) => (
                  <EventChips
                    value={field.value}
                    onChange={field.onChange}
                    options={WORKFLOW_EVENT_OPTIONS}
                    aria-labelledby="webhook-events-label"
                  />
                )}
              />
              {events.length === 0 && (
                <FieldDescription>Select one or more</FieldDescription>
              )}
            </Field>
            <Field>
              <FieldLabel htmlFor="webhook-secret">Secret</FieldLabel>
              <Controller
                control={control}
                name="secret"
                render={({ field }) => (
                  <Input
                    {...field}
                    id="webhook-secret"
                    placeholder={editingId ? "Leave blank to keep current" : "Optional signing secret"}
                  />
                )}
              />
            </Field>
            <Field orientation="horizontal">
              <FieldLabel htmlFor="webhook-enabled">Enabled</FieldLabel>
              <Controller
                control={control}
                name="enabled"
                render={({ field }) => (
                  <Switch
                    id="webhook-enabled"
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                )}
              />
            </Field>
          </div>
          <DialogFooter>
            <Button
              variant="ghost"
              onClick={() => setDialogOpen(false)}
              disabled={saveMutation.isPending}
            >
              Cancel
            </Button>
            <Button
              onClick={rhfSubmit((data) => saveMutation.mutate(data))}
              disabled={saveMutation.isPending || events.length === 0}
            >
              {saveMutation.isPending ? (editingId ? "Saving..." : "Creating...") : (editingId ? "Save" : "Create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
