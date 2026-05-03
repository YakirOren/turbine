export const WORKFLOW_EVENT_OPTIONS = [
  "workflow.SUCCESS",
  "workflow.ERROR",
  "workflow.CANCELLED",
  "workflow.WAITING_FOR_APPROVAL",
  "workflow.MAX_RECOVERY_ATTEMPTS_EXCEEDED",
  "workflow.*",
];

const EVENT_ACTIVE_CLASS: Record<string, string> = {
  "workflow.SUCCESS":
    "bg-success-soft text-success-foreground shadow-sm",
  "workflow.ERROR":
    "bg-danger-soft text-danger-foreground shadow-sm",
  "workflow.CANCELLED":
    "bg-muted-foreground/15 text-foreground shadow-sm",
  "workflow.WAITING_FOR_APPROVAL":
    "bg-warning-soft text-warning-foreground shadow-sm",
  "workflow.MAX_RECOVERY_ATTEMPTS_EXCEEDED":
    "bg-danger-soft text-danger-foreground shadow-sm",
  "workflow.*":
    "bg-info-soft text-info-foreground shadow-sm",
};

type EventChipsProps = {
  value: string[];
  onChange: (next: string[]) => void;
  options?: string[];
  "aria-labelledby"?: string;
};

export function EventChips({
  value,
  onChange,
  options = WORKFLOW_EVENT_OPTIONS,
  "aria-labelledby": ariaLabelledBy,
}: EventChipsProps) {
  const toggle = (opt: string) => {
    onChange(
      value.includes(opt) ? value.filter((v) => v !== opt) : [...value, opt],
    );
  };

  return (
    <div
      role="group"
      aria-labelledby={ariaLabelledBy}
      className="flex flex-wrap gap-1 rounded-md border bg-muted p-1"
    >
      {options.map((opt) => {
        const active = value.includes(opt);
        return (
          <button
            key={opt}
            type="button"
            role="checkbox"
            aria-checked={active}
            onClick={() => toggle(opt)}
            className={`inline-flex items-center justify-center rounded px-2.5 py-1 font-mono text-xs transition-colors ${
              active
                ? `font-medium ${EVENT_ACTIVE_CLASS[opt] ?? "bg-background text-foreground shadow-sm"}`
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            {opt}
          </button>
        );
      })}
    </div>
  );
}
