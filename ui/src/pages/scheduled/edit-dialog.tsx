import { useId, useMemo, useState, useEffect } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import cronstrue from "cronstrue";
import { pbClient } from "@/providers/pocketbase";
import { formatScheduledAt } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Field, FieldLabel, FieldDescription, FieldError } from "@/components/ui/field";
import {
  SchemaFormField,
  getFieldDefaults,
  type InputSchema,
} from "@/components/schema-form";
import { CodeMirrorEditor } from "@/components/codemirror";
import type { PtSchedulesResponse, PtWorkflowsResponse } from "@/types/pocketbase-types";

type Schedule = PtSchedulesResponse;
type RegisteredWorkflow = PtWorkflowsResponse<InputSchema>;

const editFormSchema = z.object({
  scheduledAt: z.string(),
  cronExpression: z.string(),
  jitter: z.string(),
  input: z.string(),
  formValues: z.record(z.string(), z.unknown()),
});
type EditFormValues = z.infer<typeof editFormSchema>;

function toLocalDateTimeInput(iso: string | undefined): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const pad = (n: number) => n.toString().padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function parseInputAsObject(input: unknown): unknown {
  if (input == null || input === "") return undefined;
  if (typeof input === "string") {
    try {
      return JSON.parse(input);
    } catch {
      return input;
    }
  }
  return input;
}

function stringifyInput(input: unknown): string {
  const parsed = parseInputAsObject(input);
  if (parsed === undefined) return "{}";
  if (typeof parsed === "string") return parsed;
  try {
    return JSON.stringify(parsed, null, 2);
  } catch {
    return "{}";
  }
}

type EditScheduleDialogProps = {
  schedule: Schedule;
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

export function EditScheduleDialog({
  schedule,
  open,
  onOpenChange,
}: EditScheduleDialogProps) {
  const id = useId();
  const queryClient = useQueryClient();
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { data: workflow } = useQuery<RegisteredWorkflow | null>({
    queryKey: ["registered-workflow", schedule.workflow_fqn],
    queryFn: async () => {
      try {
        return await pbClient
          .collection("pt_workflows")
          .getFirstListItem<RegisteredWorkflow>(
            pbClient.filter("fqn = {:fqn}", { fqn: schedule.workflow_fqn }),
          );
      } catch {
        return null;
      }
    },
    enabled: open,
    staleTime: 10 * 60 * 1000,
  });

  const schema = workflow?.input_schema;
  const hasSchema = !!schema?.fields && schema.fields.length > 0;

  const {
    control,
    reset,
    watch,
    getValues,
    setValue,
    setError: setFieldError,
    formState: { errors: fieldErrors },
    clearErrors,
  } = useForm<EditFormValues>({
    resolver: zodResolver(editFormSchema),
    defaultValues: {
      scheduledAt: "",
      cronExpression: "",
      jitter: "",
      input: "{}",
      formValues: {},
    },
  });

  useEffect(() => {
    if (!open) return;
    const parsedInput = parseInputAsObject(schedule.input);
    const formValues = hasSchema
      ? getFieldDefaults(schema!.fields, parsedInput)
      : {};
    reset({
      scheduledAt: toLocalDateTimeInput(schedule.scheduled_at),
      cronExpression: schedule.cron_expression ?? "",
      jitter: schedule.jitter ?? "",
      input: stringifyInput(schedule.input),
      formValues,
    });
    setError(null);
  }, [open, schedule, reset, hasSchema, schema]);

  const cronExpression = watch("cronExpression");
  const scheduledAt = watch("scheduledAt");
  const formValues = watch("formValues");

  const cronPreview = useMemo(() => {
    if (schedule.type !== "cron") return null;
    if (!cronExpression.trim()) return null;
    try {
      return { ok: true, text: cronstrue.toString(cronExpression, { verbose: false }) };
    } catch (e) {
      return { ok: false, text: (e as Error).message || "Invalid cron expression" };
    }
  }, [cronExpression, schedule.type]);

  const schedulePreview = useMemo(
    () => (schedule.type === "once" && scheduledAt ? formatScheduledAt(scheduledAt) : null),
    [scheduledAt, schedule.type],
  );

  const handleSave = async () => {
    setError(null);
    clearErrors();
    const values = getValues();

    let parsedInput: unknown;
    if (hasSchema) {
      for (const field of schema!.fields) {
        const v = values.formValues[field.name];
        if (field.required && (v === "" || v === undefined)) {
          setFieldError(`formValues.${field.name}` as any, {
            message: `${field.label ?? field.name} is required`,
          });
          return;
        }
      }
      parsedInput = values.formValues;
    } else {
      parsedInput = values.input;
    }

    const patch: Record<string, unknown> = { input: parsedInput };

    if (schedule.type === "cron") {
      if (!values.cronExpression.trim()) {
        setFieldError("cronExpression", { message: "Cron expression is required" });
        return;
      }
      if (cronPreview && !cronPreview.ok) {
        setFieldError("cronExpression", { message: cronPreview.text });
        return;
      }
      patch.cron_expression = values.cronExpression;
      patch.jitter = values.jitter;
    } else if (schedule.type === "once") {
      if (!values.scheduledAt) {
        setFieldError("scheduledAt", { message: "Scheduled time is required" });
        return;
      }
      patch.scheduled_at = new Date(values.scheduledAt).toISOString();
    }

    setSubmitting(true);
    try {
      await pbClient.collection("pt_schedules").update(schedule.id, patch);
      queryClient.invalidateQueries({ queryKey: ["schedules"] });
      toast.success("Schedule updated");
      onOpenChange(false);
    } catch (err: any) {
      setError(err?.message || "Failed to update schedule");
    } finally {
      setSubmitting(false);
    }
  };

  const workflowLabel = schedule.workflow_fqn.split(/[./]/).at(-1);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            Edit schedule
            <span className="ml-2 font-mono text-sm font-normal text-muted-foreground">
              {workflowLabel}
            </span>
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          {schedule.type === "once" && (
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

          {schedule.type === "cron" && (
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
              {schema!.fields.map((f) => (
                <SchemaFormField
                  key={f.name}
                  field={f}
                  value={formValues[f.name]}
                  onChange={(v) => {
                    const current = getValues("formValues");
                    setValue("formValues", { ...current, [f.name]: v });
                  }}
                  id={id}
                  error={(fieldErrors.formValues as any)?.[f.name]}
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

        <DialogFooter>
          <Button
            variant="ghost"
            onClick={() => onOpenChange(false)}
            disabled={submitting}
          >
            Cancel
          </Button>
          <Button
            onClick={handleSave}
            disabled={submitting || (schedule.type === "cron" && cronPreview?.ok === false)}
          >
            {submitting && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
            {submitting ? "Saving…" : "Save changes"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
