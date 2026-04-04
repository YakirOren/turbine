import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { pbClient } from "@/providers/pocketbase";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from "@/components/ui/combobox";

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
  const { data: registeredWorkflows = [] } = useQuery<{ name: string; tags: string[] }[]>({
    queryKey: ["registered-workflows"],
    queryFn: () =>
      pbClient.collection("pt_workflows").getFullList<{ name: string; tags: string[] }>(),
    staleTime: 10 * 60 * 1000,
  });
  const workflowNames = registeredWorkflows.map((w) => w.name);

  const tags = useMemo(() => {
    const tagSet = new Set<string>();
    for (const w of registeredWorkflows) {
      for (const t of w.tags ?? []) tagSet.add(t);
    }
    return [...tagSet].sort();
  }, [registeredWorkflows]);

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

      <Combobox
        value={filters.name || null}
        onValueChange={(v) => update("name", v ?? "")}
        items={workflowNames}
      >
        <ComboboxInput
          placeholder="All workflows"
          className="w-52"
          showClear={!!filters.name}
        />
        <ComboboxContent>
          <ComboboxEmpty>No workflows found.</ComboboxEmpty>
          <ComboboxList>
            {(item) => (
              <ComboboxItem key={item} value={item}>
                {item}
              </ComboboxItem>
            )}
          </ComboboxList>
        </ComboboxContent>
      </Combobox>

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

      <Combobox
        value={filters.tag || null}
        onValueChange={(v) => update("tag", v ?? "")}
        items={tags}
      >
        <ComboboxInput
          placeholder="All tags"
          className="w-36"
          showClear={!!filters.tag}
        />
        <ComboboxContent>
          <ComboboxEmpty>No tags found.</ComboboxEmpty>
          <ComboboxList>
            {(item) => (
              <ComboboxItem key={item} value={item}>
                {item}
              </ComboboxItem>
            )}
          </ComboboxList>
        </ComboboxContent>
      </Combobox>
    </div>
  );
}
