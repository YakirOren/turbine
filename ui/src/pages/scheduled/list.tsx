import { useState, useEffect } from "react";
import { pbClient } from "@/providers/pocketbase";
import { Clock, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

interface RegisteredWorkflow {
  name: string;
  fqn: string;
  triggerable: boolean;
  cronSchedule: string;
}

interface UISchedule {
  id: string;
  workflow_fqn: string;
  input: any;
  type: string;
  cron_expression: string;
  jitter: string;
  scheduled_at: string;
  created: string;
}

export function ScheduledList() {
  const [compileTime, setCompileTime] = useState<RegisteredWorkflow[]>([]);
  const [uiSchedules, setUISchedules] = useState<UISchedule[]>([]);
  const [loading, setLoading] = useState(true);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([
      pbClient.send<RegisteredWorkflow[]>("/api/pf/registered", { method: "GET" }),
      pbClient.send<UISchedule[]>("/api/pf/schedules", { method: "GET" }),
    ])
      .then(([registered, schedules]) => {
        setCompileTime(registered.filter((w) => w.cronSchedule));
        setUISchedules(schedules);
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, []);

  const handleDelete = async (id: string) => {
    if (!window.confirm("Delete this schedule? This cannot be undone.")) return;
    setDeleteError(null);
    const previous = uiSchedules;
    setUISchedules((prev) => prev.filter((s) => s.id !== id));
    try {
      await pbClient.send(`/api/pf/schedules/${id}`, { method: "DELETE" });
    } catch {
      setUISchedules(previous);
      setDeleteError("Failed to delete schedule. Please try again.");
    }
  };

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center text-muted-foreground">
        Loading...
      </div>
    );
  }

  const isEmpty = compileTime.length === 0 && uiSchedules.length === 0;

  if (isEmpty) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 text-muted-foreground">
        <Clock className="h-8 w-8" />
        <div className="text-center">
          <p>No scheduled workflows.</p>
          <p className="mt-1 text-xs">
            Schedule a workflow with{" "}
            <code className="rounded bg-muted px-1.5 py-0.5 font-mono">
              pocketflow.WithSchedule("*/5 * * * *")
            </code>
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">Scheduled Workflows</h1>

      {deleteError && (
        <div role="alert" className="rounded-md border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {deleteError}
        </div>
      )}

      {compileTime.length > 0 && (
        <div>
          <h2 className="mb-2 text-sm font-medium text-muted-foreground">
            Compile-time Schedules
          </h2>
          <div className="rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Schedule</TableHead>
                  <TableHead>Function</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {compileTime.map((s) => (
                  <TableRow key={s.fqn}>
                    <TableCell className="font-medium">{s.name}</TableCell>
                    <TableCell>
                      <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs">
                        {s.cronSchedule}
                      </code>
                    </TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      <span title={s.fqn}>{s.fqn.split(/[./]/).at(-1)}</span>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </div>
      )}

      {uiSchedules.length > 0 && (
        <div>
          <h2 className="mb-2 text-sm font-medium text-muted-foreground">
            UI-Created Schedules
          </h2>
          <div className="rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Workflow</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Schedule</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="w-10" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {uiSchedules.map((s) => (
                  <TableRow key={s.id}>
                    <TableCell className="font-mono text-xs">
                      <span title={s.workflow_fqn}>{s.workflow_fqn.split(/[./]/).at(-1)}</span>
                    </TableCell>
                    <TableCell className="capitalize">{s.type}</TableCell>
                    <TableCell>
                      <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs">
                        {s.type === "cron"
                          ? s.cron_expression
                          : new Date(s.scheduled_at).toLocaleString()}
                      </code>
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {new Date(s.created).toLocaleString()}
                    </TableCell>
                    <TableCell>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                        onClick={() => handleDelete(s.id)}
                        aria-label={`Delete ${s.type} schedule for ${s.workflow_fqn}`}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </div>
      )}
    </div>
  );
}
