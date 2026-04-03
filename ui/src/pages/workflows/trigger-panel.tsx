import { useState, useEffect, useRef, useId } from "react";
import { Play } from "lucide-react";
import { pbClient } from "@/providers/pocketbase";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";

interface RegisteredWorkflow {
  name: string;
  fqn: string;
  triggerable: boolean;
  cronSchedule: string;
}

type TimingMode = "now" | "schedule" | "cron";

const TIMING_OPTIONS: { value: TimingMode; label: string }[] = [
  { value: "now", label: "Now" },
  { value: "schedule", label: "Schedule" },
  { value: "cron", label: "Cron" },
];

export function TriggerRunButton() {
  const id = useId();
  const [open, setOpen] = useState(false);
  const [workflows, setWorkflows] = useState<RegisteredWorkflow[]>([]);
  const [loadingWorkflows, setLoadingWorkflows] = useState(false);
  const [selectedFqn, setSelectedFqn] = useState("");
  const [input, setInput] = useState("{}");
  const [timing, setTiming] = useState<TimingMode>("now");
  const [scheduledAt, setScheduledAt] = useState("");
  const [cronExpression, setCronExpression] = useState("");
  const [jitter, setJitter] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const resultTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (!open) return;
    setLoadingWorkflows(true);
    pbClient
      .send<RegisteredWorkflow[]>("/api/pt/registered", { method: "GET" })
      .then((data) => {
        setWorkflows(data.filter((w) => w.triggerable));
      })
      .catch(() => {})
      .finally(() => setLoadingWorkflows(false));
  }, [open]);

  // Clear auto-dismiss timer on unmount
  useEffect(() => {
    return () => {
      if (resultTimerRef.current !== null) {
        clearTimeout(resultTimerRef.current);
      }
    };
  }, []);

  const scheduleResultDismiss = () => {
    if (resultTimerRef.current !== null) {
      clearTimeout(resultTimerRef.current);
    }
    resultTimerRef.current = setTimeout(() => {
      setResult(null);
      resultTimerRef.current = null;
    }, 4000);
  };

  const reset = () => {
    if (resultTimerRef.current !== null) {
      clearTimeout(resultTimerRef.current);
      resultTimerRef.current = null;
    }
    setSelectedFqn("");
    setInput("{}");
    setTiming("now");
    setScheduledAt("");
    setCronExpression("");
    setJitter("");
    setResult(null);
    setError(null);
  };

  const handleOpenChange = (next: boolean) => {
    setOpen(next);
    if (!next) reset();
  };

  const handleSubmit = async () => {
    setSubmitting(true);
    setResult(null);
    setError(null);

    try {
      let parsedInput: unknown;
      try {
        parsedInput = JSON.parse(input);
      } catch {
        setError("Invalid JSON input");
        setSubmitting(false);
        return;
      }

      if (timing === "now") {
        const res = await pbClient.send<{ workflow_id: string }>(
          "/api/pt/trigger",
          {
            method: "POST",
            body: {
              workflow_fqn: selectedFqn,
              input: parsedInput,
            },
          }
        );
        setResult(`Workflow triggered: ${res.workflow_id}`);
        scheduleResultDismiss();
      } else if (timing === "schedule") {
        if (!scheduledAt) {
          setError("Scheduled time is required");
          setSubmitting(false);
          return;
        }
        const res = await pbClient.send<{ id: string }>(
          "/api/pt/schedules",
          {
            method: "POST",
            body: {
              workflow_fqn: selectedFqn,
              input: parsedInput,
              type: "once",
              scheduled_at: new Date(scheduledAt).toISOString(),
            },
          }
        );
        setResult(`Schedule created: ${res.id}`);
        scheduleResultDismiss();
      } else if (timing === "cron") {
        if (!cronExpression) {
          setError("Cron expression is required");
          setSubmitting(false);
          return;
        }
        const res = await pbClient.send<{ id: string }>(
          "/api/pt/schedules",
          {
            method: "POST",
            body: {
              workflow_fqn: selectedFqn,
              input: parsedInput,
              type: "cron",
              cron_expression: cronExpression,
              ...(jitter ? { jitter } : {}),
            },
          }
        );
        setResult(`Cron schedule created: ${res.id}`);
        scheduleResultDismiss();
      }
    } catch (err: any) {
      setError(err?.message || "Failed to submit");
    } finally {
      setSubmitting(false);
    }
  };

  const handleTimingKeyDown = (e: React.KeyboardEvent, current: TimingMode) => {
    const idx = TIMING_OPTIONS.findIndex((o) => o.value === current);
    let next: number | null = null;
    if (e.key === "ArrowRight" || e.key === "ArrowDown") {
      next = (idx + 1) % TIMING_OPTIONS.length;
    } else if (e.key === "ArrowLeft" || e.key === "ArrowUp") {
      next = (idx - 1 + TIMING_OPTIONS.length) % TIMING_OPTIONS.length;
    }
    if (next !== null) {
      e.preventDefault();
      setTiming(TIMING_OPTIONS[next].value);
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button size="sm">
          <Play className="mr-1.5 h-3.5 w-3.5" />
          Trigger Run
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Trigger Run</DialogTitle>
          <DialogDescription>
            Trigger a workflow to run now, at a scheduled time, or on a cron
            schedule.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="space-y-1.5">
            <label htmlFor={`${id}-workflow`} className="text-sm font-medium">
              Workflow
            </label>
            <Select
              value={selectedFqn}
              onValueChange={setSelectedFqn}
              disabled={loadingWorkflows}
            >
              <SelectTrigger id={`${id}-workflow`}>
                <SelectValue
                  placeholder={
                    loadingWorkflows ? "Loading workflows..." : "Select Workflow"
                  }
                />
              </SelectTrigger>
              <SelectContent>
                {workflows.length === 0 ? (
                  <div className="px-2 py-4 text-center text-sm text-muted-foreground">
                    No triggerable workflows registered
                  </div>
                ) : (
                  workflows.map((w) => (
                    <SelectItem key={w.fqn} value={w.fqn}>
                      {w.name}
                    </SelectItem>
                  ))
                )}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1.5">
            <label htmlFor={`${id}-input`} className="text-sm font-medium">
              Input
            </label>
            <textarea
              id={`${id}-input`}
              className="flex min-h-[120px] w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
              value={input}
              onChange={(e) => setInput(e.target.value)}
              placeholder="{}"
            />
          </div>

          <div className="space-y-1.5">
            <span id={`${id}-timing-label`} className="text-sm font-medium">
              Timing
            </span>
            <div
              role="radiogroup"
              aria-labelledby={`${id}-timing-label`}
              className="flex gap-1 rounded-md border bg-muted p-1 w-fit"
            >
              {TIMING_OPTIONS.map((opt) => (
                <button
                  key={opt.value}
                  role="radio"
                  aria-checked={timing === opt.value}
                  tabIndex={timing === opt.value ? 0 : -1}
                  className={`rounded px-3 py-1 text-sm ${
                    timing === opt.value
                      ? "bg-background font-medium shadow-sm"
                      : "text-muted-foreground hover:text-foreground"
                  }`}
                  onClick={() => setTiming(opt.value)}
                  onKeyDown={(e) => handleTimingKeyDown(e, opt.value)}
                >
                  {opt.label}
                </button>
              ))}
            </div>
          </div>

          {timing === "schedule" && (
            <div className="space-y-1.5">
              <label
                htmlFor={`${id}-scheduled-at`}
                className="text-sm font-medium"
              >
                Scheduled Time
              </label>
              <Input
                id={`${id}-scheduled-at`}
                type="datetime-local"
                value={scheduledAt}
                onChange={(e) => setScheduledAt(e.target.value)}
              />
            </div>
          )}

          {timing === "cron" && (
            <>
              <div className="space-y-1.5">
                <label
                  htmlFor={`${id}-cron`}
                  className="text-sm font-medium"
                >
                  Cron Expression
                </label>
                <Input
                  id={`${id}-cron`}
                  className="font-mono"
                  value={cronExpression}
                  onChange={(e) => setCronExpression(e.target.value)}
                  placeholder="0 * * * *"
                />
              </div>
              <div className="space-y-1.5">
                <label
                  htmlFor={`${id}-jitter`}
                  className="text-sm font-medium"
                >
                  Jitter (optional)
                </label>
                <Input
                  id={`${id}-jitter`}
                  className="font-mono"
                  value={jitter}
                  onChange={(e) => setJitter(e.target.value)}
                  placeholder="30s"
                  aria-describedby={`${id}-jitter-hint`}
                />
                <p
                  id={`${id}-jitter-hint`}
                  className="text-xs text-muted-foreground"
                >
                  Random delay before each execution (e.g. 30s, 2m)
                </p>
              </div>
            </>
          )}

          {result && (
            <div
              role="status"
              className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-400"
            >
              {result}
            </div>
          )}
          {error && (
            <div
              role="alert"
              className="rounded-md border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive"
            >
              {error}
            </div>
          )}

          <div className="flex justify-end">
            <Button
              onClick={handleSubmit}
              disabled={!selectedFqn || submitting}
            >
              {submitting ? "Submitting..." : "Submit"}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
