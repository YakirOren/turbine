import { useState, useId } from "react";
import { Play } from "lucide-react";
import { toast } from "sonner";
import { useInvalidate } from "@refinedev/core";
import { useQuery, useMutation } from "@tanstack/react-query";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { pbClient } from "@/providers/pocketbase";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
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
import { Field, FieldLabel, FieldDescription, FieldError } from "@/components/ui/field";
import { SchemaFormField, getFieldDefaults, type InputSchema } from "@/components/schema-form";
import type { PtWorkflowsResponse } from "@/types/pocketbase-types";

type RegisteredWorkflow = PtWorkflowsResponse<InputSchema>;

type TimingMode = "now" | "schedule" | "cron";

const TIMING_OPTIONS: { value: TimingMode; label: string }[] = [
  { value: "now", label: "Now" },
  { value: "schedule", label: "Schedule" },
  { value: "cron", label: "Cron" },
];

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
      if (hasValidationError) return;
      parsedInput = values.formValues;
    } else {
      try {
        parsedInput = JSON.parse(values.input);
      } catch {
        setFieldError("input", { message: "Invalid JSON input" });
        return;
      }
    }

    if (values.timing === "schedule" && !values.scheduledAt) {
      setFieldError("scheduledAt", { message: "Scheduled time is required" });
      return;
    }
    if (values.timing === "cron" && !values.cronExpression) {
      setFieldError("cronExpression", { message: "Cron expression is required" });
      return;
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
          </Field>

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
              <FieldLabel htmlFor={`${id}-input`}>Input</FieldLabel>
              <Controller
                control={control}
                name="input"
                render={({ field, fieldState }) => (
                  <>
                    <Textarea
                      {...field}
                      id={`${id}-input`}
                      aria-invalid={!!fieldState.error}
                      className="min-h-[120px] font-mono"
                      placeholder="{}"
                    />
                    <FieldError errors={[fieldState.error]} />
                  </>
                )}
              />
            </Field>
          )}

          <Field>
            <FieldLabel id={`${id}-timing-label`}>Timing</FieldLabel>
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
                  onClick={() => setValue("timing", opt.value)}
                  onKeyDown={(e) => handleTimingKeyDown(e, opt.value)}
                >
                  {opt.label}
                </button>
              ))}
            </div>
          </Field>

          {timing === "schedule" && (
            <Field data-invalid={!!fieldErrors.scheduledAt}>
              <FieldLabel htmlFor={`${id}-scheduled-at`}>Scheduled Time</FieldLabel>
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
            </Field>
          )}

          {timing === "cron" && (
            <>
              <Field data-invalid={!!fieldErrors.cronExpression}>
                <FieldLabel htmlFor={`${id}-cron`}>Cron Expression</FieldLabel>
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
              disabled={!selectedFqn || submitMutation.isPending}
            >
              {submitMutation.isPending ? "Submitting..." : "Submit"}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
