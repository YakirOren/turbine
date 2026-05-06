import { useState } from "react";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useList, useInvalidate } from "@refinedev/core";
import { useMutation } from "@tanstack/react-query";
import { pbClient } from "@/providers/pocketbase";
import { Bell, Plus, Trash2, Pencil, Play, Loader2 } from "lucide-react";
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
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { Field, FieldLabel, FieldDescription } from "@/components/ui/field";
import { TableSkeleton } from "@/components/table-skeleton";
import { DocLink } from "@/components/doc-link";
import { EventChips, WORKFLOW_EVENT_OPTIONS } from "@/components/event-chips";
import type { PtAlertChannelsResponse } from "@/types/pocketbase-types";

type AlertChannelRecord = PtAlertChannelsResponse<string[]> & { events: string[] };

const formSchema = z.object({
  name: z.string().min(1, "Name is required"),
  url: z.string(),
  events: z.array(z.string()).min(1, "Select at least one event"),
  enabled: z.boolean(),
});
type FormValues = z.infer<typeof formSchema>;

const SERVICE_NAMES: Record<string, string> = {
  slack: "Slack",
  discord: "Discord",
  telegram: "Telegram",
  smtp: "Email",
  teams: "Teams",
  gotify: "Gotify",
  ntfy: "Ntfy",
  pushover: "Pushover",
  pushbullet: "Pushbullet",
  matrix: "Matrix",
  rocketchat: "Rocket.Chat",
  mattermost: "Mattermost",
  googlechat: "Google Chat",
  opsgenie: "OpsGenie",
  ifttt: "IFTTT",
  join: "Join",
  zulip: "Zulip",
  bark: "Bark",
  generic: "Generic",
  logger: "Logger",
};

function extractServiceName(url: string): string {
  const scheme = url.split("://")[0]?.replace(/\+.*$/, "") ?? "";
  return SERVICE_NAMES[scheme.toLowerCase()] ?? (scheme || "Unknown");
}

function timeAgo(dateStr: string): string {
  if (!dateStr) return "\u2014";
  const diff = Date.now() - new Date(dateStr).getTime();
  if (diff < 60000) return `${Math.round(diff / 1000)}s ago`;
  if (diff < 3600000) return `${Math.round(diff / 60000)}m ago`;
  if (diff < 86400000) return `${Math.round(diff / 3600000)}h ago`;
  return `${Math.round(diff / 86400000)}d ago`;
}

export function NotificationList() {
  const invalidate = useInvalidate();
  const [editing, setEditing] = useState<{ id: string | null } | null>(null);
  const [testPopoverId, setTestPopoverId] = useState<string | null>(null);
  const editingId = editing?.id ?? null;

  const { control, handleSubmit: rhfSubmit, reset, watch } = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      name: "",
      url: "",
      events: [],
      enabled: true,
    },
  });

  const events = watch("events");

  const { result, query } = useList<AlertChannelRecord>({
    resource: "pt_alert_channels",
    pagination: { mode: "off" },
  });

  const records = result.data ?? [];
  const isLoading = query.isLoading && records.length === 0;

  const invalidateChannels = () =>
    invalidate({ resource: "pt_alert_channels", invalidates: ["list"] });

  const saveMutation = useMutation({
    mutationFn: async (data: FormValues) => {
      if (!editingId && !data.url) {
        throw new Error("URL is required");
      }
      const body: Record<string, unknown> = {
        name: data.name,
        events: data.events,
        enabled: data.enabled,
      };
      if (data.url) {
        body.url = data.url;
      }
      if (editingId) {
        await pbClient.collection("pt_alert_channels").update(editingId, body);
      } else {
        await pbClient.collection("pt_alert_channels").create(body);
      }
    },
    onSuccess: () => {
      invalidateChannels();
      toast.success(editingId ? "Channel updated" : "Channel created");
      setEditing(null);
    },
    onError: (err: any) => {
      toast.error(err?.message || `Failed to ${editingId ? "update" : "create"} channel`);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) =>
      pbClient.collection("pt_alert_channels").delete(id),
    onSuccess: () => {
      invalidateChannels();
      toast.success("Channel deleted");
    },
    onError: (err: any) => {
      toast.error(err?.message || "Failed to delete channel");
    },
  });

  const toggleMutation = useMutation({
    mutationFn: (record: AlertChannelRecord) =>
      pbClient.collection("pt_alert_channels").update(record.id, { enabled: !record.enabled }),
    onSuccess: () => {
      invalidateChannels();
    },
    onError: (err: any) => {
      toast.error(err?.message || "Failed to toggle channel");
    },
  });

  const testMutation = useMutation({
    mutationFn: (id: string) =>
      pbClient.send(`/api/pt/alert-channels/${id}/test`, { method: "POST" }),
    onSuccess: () => {
      toast.success("Test notification sent");
      setTestPopoverId(null);
    },
    onError: (err: any) => {
      toast.error(err?.message || "Test notification failed");
      setTestPopoverId(null);
    },
  });

  const openAdd = () => {
    reset();
    setEditing({ id: null });
  };

  const openEdit = (record: AlertChannelRecord) => {
    reset({ name: record.name, url: "", events: [...record.events], enabled: record.enabled });
    setEditing({ id: record.id });
  };

  if (isLoading) {
    return (
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-semibold tracking-tight">Notifications</h1>
        </div>
        <TableSkeleton columns={6} headers={["Name", "Service", "Events", "Enabled", "Created", ""]} />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold tracking-tight">Notifications</h1>
        <Button size="sm" onClick={openAdd}>
          <Plus className="mr-1.5 h-3.5 w-3.5" />
          Add Channel
        </Button>
      </div>

      {query.isError && (
        <div className="rounded-md border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {(query.error as unknown as Error)?.message ??
            "Failed to load notification channels"}
        </div>
      )}

      {records.length === 0 && !query.isError ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 py-20 text-muted-foreground">
          <Bell className="h-8 w-8" />
          <div className="text-center">
            <p>No notification channels configured</p>
            <p className="mt-1 text-xs">
              Add a channel to receive alerts when workflows complete.
            </p>
          </div>
          <DocLink path="concepts/notifications" className="text-xs">
            Learn more
          </DocLink>
        </div>
      ) : (
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Service</TableHead>
                <TableHead>Events</TableHead>
                <TableHead>Enabled</TableHead>
                <TableHead>Created</TableHead>
                <TableHead className="w-28" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {records.map((record) => (
                <TableRow key={record.id}>
                  <TableCell className="w-full max-w-0">
                    <div className="truncate font-medium" title={record.name}>
                      {record.name}
                    </div>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {extractServiceName(record.url)}
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-1">
                      {record.events?.map((event) => (
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
                      aria-label={`${record.enabled ? "Disable" : "Enable"} channel`}
                    />
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {timeAgo(record.created)}
                  </TableCell>
                  <TableCell className="w-28">
                    <div className="flex items-center gap-1">
                      {(() => {
                        const isTesting =
                          testMutation.isPending &&
                          testMutation.variables === record.id;
                        const isOpen = testPopoverId === record.id;
                        return (
                          <Popover
                            open={isOpen}
                            onOpenChange={(open) =>
                              setTestPopoverId(open ? record.id : null)
                            }
                          >
                            <TooltipProvider delayDuration={200}>
                              <Tooltip open={isOpen ? false : undefined}>
                                <PopoverTrigger asChild>
                                  <TooltipTrigger asChild>
                                    <Button
                                      variant="ghost"
                                      size="icon"
                                      className="h-8 w-8 text-muted-foreground hover:text-foreground"
                                      disabled={isTesting}
                                      aria-label={`Test channel ${record.name}`}
                                    >
                                      {isTesting ? (
                                        <Loader2 className="h-4 w-4 animate-spin" />
                                      ) : (
                                        <Play className="h-4 w-4" />
                                      )}
                                    </Button>
                                  </TooltipTrigger>
                                </PopoverTrigger>
                                <TooltipContent side="top">
                                  Test channel
                                </TooltipContent>
                              </Tooltip>
                            </TooltipProvider>
                            <PopoverContent align="end" className="w-auto p-3">
                              <div className="flex items-center gap-3">
                                <p className="text-sm">Send test notification?</p>
                                <Button
                                  size="sm"
                                  onClick={() => testMutation.mutate(record.id)}
                                  disabled={isTesting}
                                >
                                  {isTesting ? (
                                    <>
                                      <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                                      Sending
                                    </>
                                  ) : (
                                    "Send"
                                  )}
                                </Button>
                              </div>
                            </PopoverContent>
                          </Popover>
                        );
                      })()}
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-muted-foreground hover:text-foreground"
                        onClick={() => openEdit(record)}
                        aria-label={`Edit channel ${record.name}`}
                      >
                        <Pencil className="h-4 w-4" />
                      </Button>
                      <AlertDialog>
                        <AlertDialogTrigger asChild>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                            aria-label={`Delete channel ${record.name}`}
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </AlertDialogTrigger>
                        <AlertDialogContent>
                          <AlertDialogHeader>
                            <AlertDialogTitle>Delete channel?</AlertDialogTitle>
                            <AlertDialogDescription>
                              This will permanently delete the notification channel{" "}
                              <span className="font-medium">{record.name}</span>.
                              This cannot be undone.
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

      <Dialog open={editing !== null} onOpenChange={(o) => !o && setEditing(null)}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{editingId ? "Edit Notification Channel" : "Add Notification Channel"}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <Field>
              <FieldLabel htmlFor="channel-name">Name</FieldLabel>
              <Controller
                control={control}
                name="name"
                render={({ field }) => (
                  <Input
                    {...field}
                    id="channel-name"
                    placeholder="Slack #alerts"
                  />
                )}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="channel-url">URL</FieldLabel>
              <Controller
                control={control}
                name="url"
                render={({ field }) => (
                  <Input
                    {...field}
                    id="channel-url"
                    placeholder={editingId ? "Enter new URL to update" : "slack://xoxb:token@channel"}
                    className="font-mono"
                  />
                )}
              />
              <FieldDescription>
                <a
                  href="https://shoutrrr.nickfedor.com/v0.14.3/services/overview/"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="underline-offset-2 hover:text-foreground hover:underline"
                >
                  Shoutrrr
                </a>
                {" "}URL — supports Slack, Discord, Telegram, Email, and more
              </FieldDescription>
            </Field>
            <Field>
              <FieldLabel id="channel-events-label">Events</FieldLabel>
              <Controller
                control={control}
                name="events"
                render={({ field }) => (
                  <EventChips
                    value={field.value}
                    onChange={field.onChange}
                    options={WORKFLOW_EVENT_OPTIONS}
                    aria-labelledby="channel-events-label"
                  />
                )}
              />
              {events.length === 0 && (
                <FieldDescription>Select one or more</FieldDescription>
              )}
            </Field>
            <Field orientation="horizontal">
              <FieldLabel htmlFor="channel-enabled">Enabled</FieldLabel>
              <Controller
                control={control}
                name="enabled"
                render={({ field }) => (
                  <Switch
                    id="channel-enabled"
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
              onClick={() => setEditing(null)}
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
