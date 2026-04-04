import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

const statusConfig: Record<string, { label: string; className: string }> = {
  PENDING: {
    label: "Pending",
    className: "bg-yellow-500/20 text-yellow-400 border-yellow-500/30",
  },
  ENQUEUED: {
    label: "Enqueued",
    className: "bg-blue-500/20 text-blue-400 border-blue-500/30",
  },
  SUCCESS: {
    label: "Success",
    className: "bg-green-500/20 text-green-400 border-green-500/30",
  },
  ERROR: {
    label: "Error",
    className: "bg-red-500/20 text-red-400 border-red-500/30",
  },
  CANCELLED: {
    label: "Cancelled",
    className: "bg-gray-500/20 text-gray-400 border-gray-500/30",
  },
  MAX_RECOVERY_ATTEMPTS_EXCEEDED: {
    label: "Max Retries",
    className: "bg-red-500/20 text-red-400 border-red-500/30",
  },
};

const appStatusColorConfig: Record<string, string> = {
  green: "bg-green-500/20 text-green-400 border-green-500/30",
  red: "bg-red-500/20 text-red-400 border-red-500/30",
  yellow: "bg-yellow-500/20 text-yellow-400 border-yellow-500/30",
  blue: "bg-blue-500/20 text-blue-400 border-blue-500/30",
  gray: "bg-gray-500/20 text-gray-400 border-gray-500/30",
  lime: "bg-lime-500/20 text-lime-400 border-lime-500/30",
  orange: "bg-orange-500/20 text-orange-400 border-orange-500/30",
  purple: "bg-purple-500/20 text-purple-400 border-purple-500/30",
  pink: "bg-pink-500/20 text-pink-400 border-pink-500/30",
  cyan: "bg-cyan-500/20 text-cyan-400 border-cyan-500/30",
};

export function AppStatusBadge({ label, color }: { label?: string; color?: string }) {
  if (!label) return <span className="text-muted-foreground">&mdash;</span>;
  const colorClass = appStatusColorConfig[color ?? ""] ?? appStatusColorConfig.gray;
  return (
    <Badge variant="outline" className={cn("font-medium", colorClass)}>
      {label}
    </Badge>
  );
}

export function StatusBadge({ status }: { status: string }) {
  const config = statusConfig[status] ?? {
    label: status,
    className: "bg-gray-500/20 text-gray-400 border-gray-500/30",
  };

  return (
    <Badge variant="outline" className={cn("font-medium", config.className)}>
      {config.label}
    </Badge>
  );
}

const productStatusConfig: Record<string, { label: string; className: string }> = {
  sent: { label: "Sent", className: "bg-green-500/20 text-green-400 border-green-500/30" },
  failed: { label: "Failed", className: "bg-red-500/20 text-red-400 border-red-500/30" },
};

export function ProductStatusBadge({ status }: { status: string }) {
  const config = productStatusConfig[status] ?? {
    label: status,
    className: "bg-gray-500/20 text-gray-400 border-gray-500/30",
  };
  return (
    <Badge variant="outline" className={cn("font-medium", config.className)}>
      {config.label}
    </Badge>
  );
}
