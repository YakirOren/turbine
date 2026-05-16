import { Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";

type Tone = "success" | "info" | "warning" | "danger" | "neutral";

const toneClasses: Record<Tone, { bg: string; fg: string; dot: string }> = {
  success: {
    bg: "bg-success-soft",
    fg: "text-success-foreground",
    dot: "bg-success",
  },
  info: {
    bg: "bg-info-soft",
    fg: "text-info-foreground",
    dot: "bg-info",
  },
  warning: {
    bg: "bg-warning-soft",
    fg: "text-warning-foreground",
    dot: "bg-warning",
  },
  danger: {
    bg: "bg-danger-soft",
    fg: "text-danger-foreground",
    dot: "bg-danger",
  },
  neutral: {
    bg: "bg-secondary",
    fg: "text-secondary-foreground",
    dot: "bg-muted-foreground",
  },
};

const statusConfig: Record<string, { label: string; tone: Tone }> = {
  PENDING: { label: "Pending", tone: "warning" },
  ENQUEUED: { label: "Enqueued", tone: "info" },
  RUNNING: { label: "Running", tone: "info" },
  SUCCESS: { label: "Success", tone: "success" },
  ERROR: { label: "Failed", tone: "danger" },
  CANCELLED: { label: "Cancelled", tone: "neutral" },
  MAX_RECOVERY_ATTEMPTS_EXCEEDED: { label: "Max Retries", tone: "danger" },
};

const stepStatusConfig: Record<string, { label: string; tone: Tone }> = {
  success: { label: "Success", tone: "success" },
  error: { label: "Failed", tone: "danger" },
  running: { label: "Running", tone: "info" },
};

const appStatusColorTone: Record<string, Tone> = {
  green: "success",
  lime: "success",
  red: "danger",
  yellow: "warning",
  orange: "warning",
  blue: "info",
  cyan: "info",
  purple: "info",
  pink: "danger",
  gray: "neutral",
};

export function Pill({
  tone = "neutral",
  children,
  dot = true,
  spinner = false,
  className,
}: {
  tone?: Tone;
  children: React.ReactNode;
  dot?: boolean;
  spinner?: boolean;
  className?: string;
}) {
  const c = toneClasses[tone];
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 whitespace-nowrap rounded-full px-2 py-0.5 text-[11px] font-medium",
        c.bg,
        c.fg,
        className,
      )}
    >
      {spinner ? (
        <Loader2 className={cn("h-3 w-3 animate-spin", c.fg)} />
      ) : (
        dot && <span className={cn("h-1.5 w-1.5 rounded-full", c.dot)} />
      )}
      {children}
    </span>
  );
}

export function StatusBadge({ status }: { status: string }) {
  const config = statusConfig[status] ?? { label: status, tone: "neutral" as Tone };
  return <Pill tone={config.tone} spinner={status === "RUNNING"}>{config.label}</Pill>;
}

export function StepStatusBadge({ status }: { status: string }) {
  const config = stepStatusConfig[status] ?? { label: status, tone: "neutral" as Tone };
  return <Pill tone={config.tone} spinner={status === "running"}>{config.label}</Pill>;
}

export function AppStatusBadge({ label, color }: { label?: string; color?: string }) {
  if (!label) return <span className="text-muted-foreground">&mdash;</span>;
  const tone = appStatusColorTone[color ?? ""] ?? "neutral";
  return <Pill tone={tone}>{label}</Pill>;
}

const productStatusConfig: Record<string, { label: string; tone: Tone }> = {
  sent: { label: "Sent", tone: "success" },
  failed: { label: "Failed", tone: "danger" },
};

export function ProductStatusBadge({ status }: { status: string }) {
  const config = productStatusConfig[status] ?? { label: status, tone: "neutral" as Tone };
  return <Pill tone={config.tone}>{config.label}</Pill>;
}
