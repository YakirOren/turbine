import { useState, useMemo, useCallback } from "react";
import type { CrudFilters, CrudSort } from "@refinedev/core";
import {
  WorkflowFilterBar,
  type WorkflowFilters,
} from "@/components/workflow-filters";
import { WorkflowTable } from "@/components/workflow-table";
import { WorkflowSidebar } from "@/components/workflow-sidebar";
import { TriggerRunButton } from "@/pages/workflows/trigger-panel";
import { CalendarView } from "@/pages/calendar/index";

const STORAGE_KEY = "pt_workflow_filters";

const defaultFilters: WorkflowFilters = {
  timeRange: "24h",
  name: "",
  status: "all",
  tag: "",
};

function loadFilters(): WorkflowFilters {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return defaultFilters;
    return { ...defaultFilters, ...JSON.parse(raw) };
  } catch {
    return defaultFilters;
  }
}

export function patchWorkflowFilters(partial: Partial<WorkflowFilters>) {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    const parsed = raw ? JSON.parse(raw) : {};
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ ...parsed, ...partial }));
  } catch {
    /* ignore */
  }
}

function timeRangeToEpochMs(range: string): number | null {
  const now = Date.now();
  switch (range) {
    case "1h":
      return now - 3600_000;
    case "6h":
      return now - 21600_000;
    case "24h":
      return now - 86400_000;
    case "7d":
      return now - 604800_000;
    default:
      return null;
  }
}

function loadCustomRange(): { from: number; to: number } | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw);
    if (parsed.timeRange === "custom" && parsed.customFrom && parsed.customTo) {
      return { from: parsed.customFrom, to: parsed.customTo };
    }
    return null;
  } catch {
    return null;
  }
}

export function WorkflowList() {
  const [filters, setFiltersState] = useState<WorkflowFilters>(loadFilters);

  const setFilters = useCallback((f: WorkflowFilters) => {
    setFiltersState(f);
    localStorage.setItem(STORAGE_KEY, JSON.stringify(f));
  }, []);

  const [selectedId, setSelectedId] = useState<string | null>(null);

  const crudFilters = useMemo<CrudFilters>(() => {
    const f: CrudFilters = [];

    const custom = loadCustomRange();
    if (custom) {
      f.push({
        field: "created_at_epoch_ms",
        operator: "gte",
        value: custom.from,
      });
      f.push({
        field: "created_at_epoch_ms",
        operator: "lt",
        value: custom.to,
      });
      // Clear the custom range after applying so it doesn't persist across navigations
      const stored = localStorage.getItem(STORAGE_KEY);
      if (stored) {
        const parsed = JSON.parse(stored);
        if (parsed.timeRange === "custom") {
          localStorage.setItem(STORAGE_KEY, JSON.stringify({ ...defaultFilters }));
        }
      }
    } else {
      const epoch = timeRangeToEpochMs(filters.timeRange);
      f.push({
        field: "created_at_epoch_ms",
        operator: "gte",
        value: epoch ?? 0,
      });
    }

    if (filters.name) {
      f.push({ field: "name", operator: "eq", value: filters.name });
    }
    if (filters.status && filters.status !== "all") {
      f.push({ field: "status", operator: "eq", value: filters.status });
    }
    if (filters.tag) {
      f.push({ field: "tags", operator: "contains", value: `"${filters.tag}"` });
    }

    return f;
  }, [filters]);

  const sorters: CrudSort[] = [
    { field: "created_at_epoch_ms", order: "desc" },
  ];

  return (
    <div className="flex h-full min-h-0 flex-1 bg-card">
      <div className="flex min-w-0 flex-1 flex-col overflow-y-auto p-6">
        <div className="space-y-4">
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">Workflows</h1>
          </div>
          <CalendarView
            timeRange={filters.timeRange}
            name={filters.name}
            status={filters.status}
            tag={filters.tag}
            onDayClick={(day) => {
              const dayStart = new Date(day + "T00:00:00").getTime();
              const dayEnd = dayStart + 86400_000;
              localStorage.setItem(STORAGE_KEY, JSON.stringify({
                timeRange: "custom",
                customFrom: dayStart,
                customTo: dayEnd,
                name: "",
                status: "all",
              }));
              setFilters(loadFilters());
            }}
          />
          <div className="flex items-center gap-3">
            <WorkflowFilterBar filters={filters} onChange={setFilters} />
            <div className="ml-auto">
              <TriggerRunButton />
            </div>
          </div>
          <WorkflowTable
            key={JSON.stringify(crudFilters)}
            filters={crudFilters}
            sorters={sorters}
            selectedId={selectedId}
            onRowClick={(record) => setSelectedId(record.id)}
          />
        </div>
      </div>
      <WorkflowSidebar
        workflowId={selectedId}
        onClose={() => setSelectedId(null)}
        activeTag={filters.tag}
        onTagClick={(tag) => setFilters({ ...filters, tag })}
      />
    </div>
  );
}
