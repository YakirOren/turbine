import { useEffect, useState } from "react";
import { pbClient } from "@/providers/pocketbase";
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
  tag: string;
}

interface Props {
  filters: WorkflowFilters;
  onChange: (filters: WorkflowFilters) => void;
}

export function WorkflowFilterBar({ filters, onChange }: Props) {
  const [workflowNames, setWorkflowNames] = useState<string[]>([]);

  useEffect(() => {
    pbClient
      .send<{ name: string }[]>("/api/pt/registered", { method: "GET" })
      .then((data) => setWorkflowNames(data.map((w) => w.name)))
      .catch(() => {});
  }, []);

  const [tags, setTags] = useState<string[]>([]);

  useEffect(() => {
    pbClient
      .send<string[]>("/api/pt/tags", { method: "GET" })
      .then(setTags)
      .catch(() => {});
  }, []);

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

      <Select
        value={filters.name || "all"}
        onValueChange={(v) => update("name", v === "all" ? "" : v)}
      >
        <SelectTrigger className="w-52">
          <SelectValue placeholder="All workflows" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">All workflows</SelectItem>
          {workflowNames.map((name) => (
            <SelectItem key={name} value={name}>
              {name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

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

      <Select
        value={filters.tag || "all"}
        onValueChange={(v) => update("tag", v === "all" ? "" : v)}
      >
        <SelectTrigger className="w-36">
          <SelectValue placeholder="All tags" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">All tags</SelectItem>
          {tags.map((t) => (
            <SelectItem key={t} value={t}>
              {t}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
