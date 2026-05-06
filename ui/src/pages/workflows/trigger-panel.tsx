import { useState, useId, useMemo, useRef, useCallback } from "react";
import { Play, Loader2, Clock, Repeat } from "lucide-react";
import { toast } from "sonner";
import { useInvalidate } from "@refinedev/core";
import { useQuery, useMutation } from "@tanstack/react-query";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import cronstrue from "cronstrue";
import { pbClient } from "@/providers/pocketbase";
import { formatScheduledAt } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
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
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Field, FieldLabel, FieldDescription, FieldError } from "@/components/ui/field";
import { SchemaFormField, getFieldDefaults, type InputSchema } from "@/components/schema-form";
import { CodeMirrorEditor } from "@/components/codemirror";
import type { PtWorkflowsResponse } from "@/types/pocketbase-types";

type RegisteredWorkflow = PtWorkflowsResponse<InputSchema>;

type TimingMode = "now" | "schedule" | "cron";

const TIMING_OPTIONS: {
  value: TimingMode;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  activeClass: string;
}[] = [
  {
    value: "now",
    label: "Run now",
    icon: Play,
    activeClass: "bg-info-soft text-info-foreground shadow-sm",
  },
  {
    value: "schedule",
    label: "Run once later",
    icon: Clock,
    activeClass: "bg-warning-soft text-warning-foreground shadow-sm",
  },
  {
    value: "cron",
    label: "Run on schedule",
    icon: Repeat,
    activeClass: "bg-success-soft text-success-foreground shadow-sm",
  },
];

const SUBMIT_LABELS: Record<TimingMode, { idle: string; pending: string }> = {
  now: { idle: "Run now", pending: "Starting…" },
  schedule: { idle: "Schedule run", pending: "Scheduling…" },
  cron: { idle: "Create cron schedule", pending: "Creating…" },
};

const triggerFormSchema = z.object({
  selectedFqn: z.string().min(1),
  input: z.string(),
  timing: z.enum(["now", "schedule", "cron"]),
  scheduledAt: z.string(),
  cronExpression: z.string(),
  jitter: z.string(),
  formValues: z.record(z.string(), z.unknown()),
});
type TriggerFormValues = z.infer<typeof triggerFormSchema>;

export function TriggerRunButton() {
  const id = useId();
  const invalidate = useInvalidate();
  const [open, setOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { control, reset, watch, setValue, getValues, setError: setFieldError, formState: { errors: fieldErrors }, clearErrors } = useForm<TriggerFormValues>({
    resolver: zodResolver(triggerFormSchema),
    defaultValues: {
      selectedFqn: "",
      input: "{}",
      timing: "now",
      scheduledAt: "",
      cronExpression: "",
      jitter: "",
      formValues: {},
    },
  });

  const timing = watch("timing");
  const selectedFqn = watch("selectedFqn");
  const formValues = watch("formValues");
  const cronExpression = watch("cronExpression");
  const scheduledAt = watch("scheduledAt");

  const scrollRef = useRef<HTMLDivElement | null>(null);
  const [edges, setEdges] = useState({ top: false, bottom: false });

  const scrollRefCallback = useCallback((node: HTMLDivElement | null) => {
    scrollRef.current = node;
    if (!node) return;

    const update = () => {
      const { scrollTop, scrollHeight, clientHeight } = node;
      const top = scrollTop > 2;
      const bottom = scrollHeight - scrollTop - clientHeight > 2;
      setEdges((prev) => (prev.top === top && prev.bottom === bottom ? prev : { top, bottom }));
    };

    update();
    node.addEventListener("scroll", update, { passive: true });
    const ro = new ResizeObserver(update);
    ro.observe(node);
    if (node.firstElementChild) ro.observe(node.firstElementChild);

    return () => {
      node.removeEventListener("scroll", update);
      ro.disconnect();
      scrollRef.current = null;
    };
  }, []);

  const scrollFirstErrorIntoView = () => {
    requestAnimationFrame(() => {
      const container = scrollRef.current;
      if (!container) return;
      const target = container.querySelector<HTMLElement>(
        '[data-invalid="true"]',
      );
      if (!target) return;
      target.scrollIntoView({ block: "center", behavior: "smooth" });
      const focusable = target.querySelector<HTMLElement>(
        'input, textarea, select, button, [tabindex]:not([tabindex="-1"])',
      );
      focusable?.focus();
    });
  };

  const { data: workflows = [], isLoading } = useQuery<RegisteredWorkflow[]>({
    queryKey: ["registered-workflows-triggerable"],
    queryFn: () =>
      pbClient
        .collection("pt_workflows")
        .getFullList<RegisteredWorkflow>({ filter: "triggerable = true" }),
    staleTime: 10 * 60 * 1000,
    enabled: open,
  });
  const loadingWorkflows = isLoading && open;

  const selectedWorkflow = workflows.find((w) => w.fqn === selectedFqn);
  const schema = selectedWorkflow?.input_schema;
  const hasSchema = schema?.fields && schema.fields.length > 0;
  const workflowTags = (selectedWorkflow?.tags ?? []) as string[];

  const cronPreview = useMemo(() => {
    if (!cronExpression.trim()) return null;
    try {
      return { ok: true, text: cronstrue.toString(cronExpression, { verbose: false }) };
    } catch (e) {
      return { ok: false, text: (e as Error).message || "Invalid cron expression" };
    }
  }, [cronExpression]);

  const schedulePreview = useMemo(
    () => (scheduledAt ? formatScheduledAt(scheduledAt) : null),
    [scheduledAt],
  );

  const handleWorkflowChange = (fqn: string) => {
    const wf = workflows.find((w) => w.fqn === fqn);
    if (wf?.input_schema?.fields) {
      const defaults = getFieldDefaults(wf.input_schema.fields);
      setValue("selectedFqn", fqn);
      setValue("formValues", defaults);
    } else {
      setValue("selectedFqn", fqn);
      setValue("formValues", {});
    }
  };

  const handleOpenChange = (next: boolean) => {
    setOpen(next);
    if (!next) {
      reset();
      setError(null);
    }
  };

  const submitMutation = useMutation({
    mutationFn: async (parsedInput: unknown) => {
      const values = getValues();
      if (values.timing === "now") {
        const res = await pbClient.send<{ workflow_id: string }>(
          "/api/pt/trigger",
          {
            method: "POST",
            body: {
              workflow_fqn: values.selectedFqn,
              input: parsedInput,
            },
          }
        );
        return { timing: values.timing as TimingMode, id: res.workflow_id };
      } else if (values.timing === "schedule") {
        const res = await pbClient
          .collection("pt_schedules")
          .create<{ id: string }>({
            workflow_fqn: values.selectedFqn,
            input: parsedInput,
            type: "once",
            enabled: true,
            scheduled_at: new Date(values.scheduledAt).toISOString(),
          });
        return { timing: values.timing as TimingMode, id: res.id };
      } else {
        const res = await pbClient
          .collection("pt_schedules")
          .create<{ id: string }>({
            workflow_fqn: values.selectedFqn,
            input: parsedInput,
            type: "cron",
            enabled: true,
            cron_expression: values.cronExpression,
            ...(values.jitter ? { jitter: values.jitter } : {}),
          });
        return { timing: values.timing as TimingMode, id: res.id };
      }
    },
    onSuccess: (data) => {
      if (data.timing === "now") {
        toast.success(`Workflow triggered: ${data.id}`);
        invalidate({ resource: "pt_workflow_status", invalidates: ["list"] });
      } else if (data.timing === "schedule") {
        toast.success(`Schedule created: ${data.id}`);
      } else {
        toast.success(`Cron schedule created: ${data.id}`);
      }
      setOpen(false);
    },
    onError: (err: any) => {
      setError(err?.message || "Failed to submit");
    },
  });

  const handleSubmit = () => {
    setError(null);
    clearErrors();
    const values = getValues();

    let hasValidationError = false;
    let parsedInput: unknown;
    if (hasSchema) {
      for (const field of schema!.fields) {
        if (field.required && (values.formValues[field.name] === "" || values.formValues[field.name] === undefined)) {
          setFieldError(`formValues.${field.name}` as any, {
            message: `${field.label ?? field.name} is required`,
          });
          hasValidationError = true;
        }
      }
      if (hasValidationError) {
        scrollFirstErrorIntoView();
        return;
      }
      parsedInput = values.formValues;
    } else {
      try {
        parsedInput = JSON.parse(values.input);
      } catch (e) {
        setFieldError("input", {
          message: (e as Error).message || "Invalid JSON",
        });
        scrollFirstErrorIntoView();
        return;
      }
    }

    if (values.timing === "schedule" && !values.scheduledAt) {
      setFieldError("scheduledAt", { message: "Scheduled time is required" });
      scrollFirstErrorIntoView();
      return;
    }
    if (values.timing === "cron") {
      if (!values.cronExpression) {
        setFieldError("cronExpression", { message: "Cron expression is required" });
        scrollFirstErrorIntoView();
        return;
      }
      if (cronPreview && !cronPreview.ok) {
        setFieldError("cronExpression", { message: cronPreview.text });
        scrollFirstErrorIntoView();
        return;
      }
    }

    submitMutation.mutate(parsedInput);
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
      setValue("timing", TIMING_OPTIONS[next].value);
    }
  };

  const submitLabels = SUBMIT_LABELS[timing];

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button size="sm">
          <Play className="mr-1.5 h-3.5 w-3.5" />
          Trigger Run
        </Button>
      </DialogTrigger>
      <DialogContent className="flex max-h-[85vh] flex-col sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Trigger Run</DialogTitle>
        </DialogHeader>

        <div
          ref={scrollRefCallback}
          className="-mx-6 min-h-0 flex-1 overflow-y-auto px-6"
        >
          {edges.top && (
            <div
              aria-hidden
              className="pointer-events-none sticky top-0 z-10 -mx-6 -mb-4 h-4 bg-gradient-to-b from-background to-transparent"
            />
          )}
          <div className="space-y-4 pb-4">
          <Field>
            <FieldLabel id={`${id}-timing-label`}>When</FieldLabel>
            <div
              role="radiogroup"
              aria-labelledby={`${id}-timing-label`}
              className="grid grid-cols-3 gap-1 rounded-md border bg-muted p-1"
            >
              {TIMING_OPTIONS.map((opt) => {
                const active = timing === opt.value;
                const Icon = opt.icon;
                return (
                  <button
                    key={opt.value}
                    type="button"
                    role="radio"
                    aria-checked={active}
                    tabIndex={active ? 0 : -1}
                    className={`inline-flex items-center justify-center gap-1.5 rounded px-3 py-1.5 text-sm transition-colors ${
                      active
                        ? `font-medium ${opt.activeClass}`
                        : "text-muted-foreground hover:text-foreground"
                    }`}
                    onClick={() => setValue("timing", opt.value)}
                    onKeyDown={(e) => handleTimingKeyDown(e, opt.value)}
                  >
                    <Icon className="h-3.5 w-3.5" />
                    {opt.label}
                  </button>
                );
              })}
            </div>
          </Field>

          <Field>
            <FieldLabel htmlFor={`${id}-workflow`}>Workflow</FieldLabel>
            <Select
              value={selectedFqn}
              onValueChange={handleWorkflowChange}
              disabled={loadingWorkflows}
            >
              <SelectTrigger id={`${id}-workflow`}>
                <SelectValue
                  placeholder={
                    loadingWorkflows ? "Loading workflows..." : "Select a workflow"
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
            {selectedWorkflow && workflowTags.length > 0 && (
              <div className="mt-1.5 flex flex-wrap gap-1">
                {workflowTags.map((tag) => (
                  <Badge key={tag} variant="secondary" className="text-xs">
                    {tag}
                  </Badge>
                ))}
              </div>
            )}
          </Field>

          {timing === "schedule" && (
            <Field data-invalid={!!fieldErrors.scheduledAt}>
              <FieldLabel htmlFor={`${id}-scheduled-at`}>Scheduled time</FieldLabel>
              <Controller
                control={control}
                name="scheduledAt"
                render={({ field, fieldState }) => (
                  <>
                    <Input
                      {...field}
                      id={`${id}-scheduled-at`}
                      type="datetime-local"
                      aria-invalid={!!fieldState.error}
                    />
                    <FieldError errors={[fieldState.error]} />
                  </>
                )}
              />
              {schedulePreview && !fieldErrors.scheduledAt && (
                <FieldDescription>{schedulePreview}</FieldDescription>
              )}
            </Field>
          )}

          {timing === "cron" && (
            <>
              <Field data-invalid={!!fieldErrors.cronExpression}>
                <FieldLabel htmlFor={`${id}-cron`}>Cron expression</FieldLabel>
                <Controller
                  control={control}
                  name="cronExpression"
                  render={({ field, fieldState }) => (
                    <>
                      <Input
                        {...field}
                        id={`${id}-cron`}
                        className="font-mono"
                        placeholder="0 * * * *"
                        aria-invalid={!!fieldState.error}
                      />
                      <FieldError errors={[fieldState.error]} />
                    </>
                  )}
                />
                {cronPreview && !fieldErrors.cronExpression && (
                  <FieldDescription
                    className={
                      cronPreview.ok
                        ? "text-success-foreground"
                        : "text-danger-foreground"
                    }
                  >
                    {cronPreview.text}
                  </FieldDescription>
                )}
              </Field>
              <Field>
                <FieldLabel htmlFor={`${id}-jitter`}>Jitter (optional)</FieldLabel>
                <FieldDescription>
                  Random delay before each execution (e.g. 30s, 2m)
                </FieldDescription>
                <Controller
                  control={control}
                  name="jitter"
                  render={({ field }) => (
                    <Input
                      {...field}
                      id={`${id}-jitter`}
                      className="font-mono"
                      placeholder="30s"
                    />
                  )}
                />
              </Field>
            </>
          )}

          {hasSchema ? (
            <div className="space-y-3">
              <FieldLabel>Input</FieldLabel>
              {schema!.fields.map((field) => (
                <SchemaFormField
                  key={field.name}
                  field={field}
                  value={formValues[field.name]}
                  onChange={(v) => {
                    const current = getValues("formValues");
                    setValue("formValues", { ...current, [field.name]: v });
                  }}
                  id={id}
                  error={(fieldErrors.formValues as any)?.[field.name]}
                />
              ))}
            </div>
          ) : (
            <Field data-invalid={!!fieldErrors.input}>
              <FieldLabel htmlFor={`${id}-input`}>Input (JSON)</FieldLabel>
              <Controller
                control={control}
                name="input"
                render={({ field, fieldState }) => (
                  <>
                    <CodeMirrorEditor
                      value={field.value}
                      onChange={field.onChange}
                      placeholder="{}"
                      minHeight="120px"
                      maxHeight="240px"
                    />
                    <FieldError errors={[fieldState.error]} />
                  </>
                )}
              />
            </Field>
          )}

          {error && (
            <div
              role="alert"
              className="rounded-md border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive"
            >
              {error}
            </div>
          )}
          </div>
          {edges.bottom && (
            <div
              aria-hidden
              className="pointer-events-none sticky bottom-0 z-10 -mx-6 -mt-4 h-4 bg-gradient-to-t from-foreground/15 to-transparent"
            />
          )}
        </div>

        <DialogFooter className="-mx-6 -mb-6 -mt-4 rounded-b-lg border-t bg-muted/40 px-6 py-4">
          <Button
            variant="ghost"
            onClick={() => handleOpenChange(false)}
            disabled={submitMutation.isPending}
            className="hover:bg-foreground/10 dark:hover:bg-foreground/10"
          >
            Cancel
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={
              !selectedFqn ||
              submitMutation.isPending ||
              (timing === "cron" && cronPreview?.ok === false)
            }
          >
            {submitMutation.isPending && (
              <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
            )}
            {submitMutation.isPending ? submitLabels.pending : submitLabels.idle}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
