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
