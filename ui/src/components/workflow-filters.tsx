import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const TIME_RANGES = [
  { label: "Last 1h", value: "1h" },
  { label: "Last 6h", value: "6h" },
  { label: "Last 24h", value: "24h" },
  { label: "Last 7d", value: "7d" },
  { label: "All time", value: "all" },
];

const STATUSES = [
  { value: "PENDING", label: "Pending", dot: "bg-yellow-400" },
  { value: "ENQUEUED", label: "Enqueued", dot: "bg-blue-400" },
  { value: "SUCCESS", label: "Success", dot: "bg-green-400" },
  { value: "ERROR", label: "Error", dot: "bg-red-400" },
  { value: "CANCELLED", label: "Cancelled", dot: "bg-gray-400" },
  { value: "MAX_RECOVERY_ATTEMPTS_EXCEEDED", label: "Max Retries", dot: "bg-red-400" },
];

export interface WorkflowFilters {
  timeRange: string;
  name: string;
  status: string;
}

interface Props {
  filters: WorkflowFilters;
  onChange: (filters: WorkflowFilters) => void;
}

export function WorkflowFilterBar({ filters, onChange }: Props) {
  const update = (key: keyof WorkflowFilters, value: string) => {
    onChange({ ...filters, [key]: value });
  };

  return (
    <div className="flex flex-wrap items-center gap-3">
      <Select
        value={filters.timeRange}
        onValueChange={(v) => update("timeRange", v)}
      >
        <SelectTrigger className="w-36">
          <SelectValue placeholder="Time range" />
        </SelectTrigger>
        <SelectContent>
          {TIME_RANGES.map((r) => (
            <SelectItem key={r.value} value={r.value}>
              {r.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <Input
        placeholder="Search by name..."
        value={filters.name}
        onChange={(e) => update("name", e.target.value)}
        className="w-52"
      />

      <Select
        value={filters.status}
        onValueChange={(v) => update("status", v)}
      >
        <SelectTrigger className="w-44">
          <SelectValue placeholder="All statuses" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">All statuses</SelectItem>
          {STATUSES.map((s) => (
            <SelectItem key={s.value} value={s.value}>
              <span className="flex items-center gap-2">
                <span className={`h-2 w-2 rounded-full ${s.dot}`} />
                {s.label}
              </span>
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
