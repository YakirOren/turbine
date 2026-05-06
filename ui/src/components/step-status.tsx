import { Check, Circle, Loader2, X } from "lucide-react";

export const TERMINAL_STATUSES = new Set([
  "SUCCESS",
  "ERROR",
  "CANCELLED",
  "MAX_RECOVERY_ATTEMPTS_EXCEEDED",
]);

export interface StepStatusStyle {
  border: string;
  ring: string;
  fill: string;
  strong: string;
  icon: React.ReactNode;
}

export const statusStyles: Record<string, StepStatusStyle> = {
  success: {
    border: "border-success",
    ring: "ring-success",
    fill: "bg-success-soft",
    strong: "bg-success",
    icon: <Check className="h-2.5 w-2.5 text-success" strokeWidth={3} />,
  },
  error: {
    border: "border-danger",
    ring: "ring-danger",
    fill: "bg-danger-soft",
    strong: "bg-danger",
    icon: <X className="h-2.5 w-2.5 text-danger" strokeWidth={3} />,
  },
  running: {
    border: "border-info",
    ring: "ring-info",
    fill: "bg-info-soft",
    strong: "bg-info",
    icon: <Loader2 className="h-2.5 w-2.5 animate-spin text-info" />,
  },
};

export const fallbackStatusStyle: StepStatusStyle = {
  border: "border-border",
  ring: "ring-muted-foreground",
  fill: "bg-muted",
  strong: "bg-muted-foreground",
  icon: <Circle className="h-2.5 w-2.5 text-muted-foreground" />,
};
