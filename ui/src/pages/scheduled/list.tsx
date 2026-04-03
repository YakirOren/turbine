import { useQuery, useQueryClient } from "@tanstack/react-query";
import { pbClient } from "@/providers/pocketbase";
import dayjs from "dayjs";
import relativeTime from "dayjs/plugin/relativeTime";
import { formatTimestampPrecise } from "@/lib/format";

dayjs.extend(relativeTime);
import cronstrue from "cronstrue";
import { Clock, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
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
import { TableSkeleton } from "@/components/table-skeleton";

interface Schedule {
  id: string;
  workflow_fqn: string;
  input: any;
  type: string;
  cron_expression: string;
  jitter: string;
  scheduled_at: string;
  enabled: boolean;
  created: string;
}

export function ScheduledList() {
  const queryClient = useQueryClient();

  const { data: schedules = [], isLoading: schedulesLoading } = useQuery<Schedule[]>({
    queryKey: ["schedules"],
    queryFn: () =>
      pbClient
        .collection("pt_schedules")
        .getFullList<Schedule>(),
  });

  const loading = schedulesLoading && schedules.length === 0;

  const handleToggle = async (schedule: Schedule) => {
    try {
      await pbClient
        .collection("pt_schedules")
        .update(schedule.id, { enabled: !schedule.enabled });
      queryClient.invalidateQueries({ queryKey: ["schedules"] });
    } catch {
      toast.error("Failed to toggle schedule");
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await pbClient.collection("pt_schedules").delete(id);
      queryClient.invalidateQueries({ queryKey: ["schedules"] });
      toast.success("Schedule deleted");
    } catch {
      toast.error("Failed to delete schedule. Please try again.");
    }
  };

  if (loading) {
    return (
      <div className="space-y-4">
        <h1 className="text-2xl font-bold">Scheduled Workflows</h1>
        <TableSkeleton columns={5} headers={["Workflow", "Schedule", "Enabled", "Created", ""]} />
      </div>
    );
  }

  if (schedules.length === 0) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 text-muted-foreground">
        <Clock className="h-8 w-8" />
        <div className="text-center">
          <p>No scheduled workflows.</p>
          <p className="mt-1 text-xs">
            Schedule a workflow with{" "}
            <code className="rounded bg-muted px-1.5 py-0.5 font-mono">
              turbine.WithSchedule("*/5 * * * *")
            </code>
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">Scheduled Workflows</h1>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Workflow</TableHead>
              <TableHead>Schedule</TableHead>
              <TableHead>Enabled</TableHead>
              <TableHead>Created</TableHead>
              <TableHead className="w-10" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {schedules.map((s) => (
              <TableRow key={s.id}>
                <TableCell className="font-mono text-xs">
                  <span title={s.workflow_fqn}>
                    {s.workflow_fqn.split(/[./]/).at(-1)}
                  </span>
                </TableCell>
                <TableCell>
                  {s.cron_expression ? (
                    <TooltipProvider delayDuration={100}>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <code className="cursor-default rounded bg-muted px-1.5 py-0.5 font-mono text-xs">
                            {s.cron_expression}
                          </code>
                        </TooltipTrigger>
                        <TooltipContent side="top">
                          {cronstrue.toString(s.cron_expression)}
                        </TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                  ) : (
                    <TooltipProvider delayDuration={100}>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <code className="cursor-default rounded bg-muted px-1.5 py-0.5 font-mono text-xs">
                            {formatTimestampPrecise(s.scheduled_at)}
                          </code>
                        </TooltipTrigger>
                        <TooltipContent side="top">
                          {dayjs(s.scheduled_at).fromNow()}
                        </TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                  )}
                </TableCell>
                <TableCell>
                  <Switch
                    checked={s.enabled}
                    onCheckedChange={() => handleToggle(s)}
                    aria-label={`${s.enabled ? "Disable" : "Enable"} schedule`}
                  />
                </TableCell>
                <TableCell className="text-muted-foreground">
                  <TooltipProvider delayDuration={100}>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <span className="cursor-default">
                          {formatTimestampPrecise(s.created)}
                        </span>
                      </TooltipTrigger>
                      <TooltipContent side="top">
                        {dayjs(s.created).fromNow()}
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                </TableCell>
                <TableCell>
                  {s.type !== "compile" && (
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                          aria-label={`Delete schedule for ${s.workflow_fqn}`}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>Delete schedule?</AlertDialogTitle>
                          <AlertDialogDescription>
                            This will permanently delete the schedule for{" "}
                            <span className="font-mono">{s.workflow_fqn.split(/[./]/).at(-1)}</span>.
                            This cannot be undone.
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>Cancel</AlertDialogCancel>
                          <AlertDialogAction
                            variant="destructive"
                            onClick={() => handleDelete(s.id)}
                          >
                            Delete
                          </AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
