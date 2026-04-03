import { useState, useMemo } from "react";
import type { CrudFilters, CrudSort } from "@refinedev/core";
import {
  WorkflowFilterBar,
  type WorkflowFilters,
} from "@/components/workflow-filters";
import { WorkflowTable } from "@/components/workflow-table";
import { WorkflowSidebar } from "@/components/workflow-sidebar";
import { TriggerRunButton } from "@/pages/workflows/trigger-panel";

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

export function WorkflowList() {
  const [filters, setFilters] = useState<WorkflowFilters>({
    timeRange: "24h",
    name: "",
    status: "all",
  });

  const [selectedId, setSelectedId] = useState<string | null>(null);

  const crudFilters = useMemo<CrudFilters>(() => {
    const f: CrudFilters = [];

    const epoch = timeRangeToEpochMs(filters.timeRange);
    if (epoch) {
      f.push({
        field: "created_at_epoch_ms",
        operator: "gte",
        value: epoch,
      });
    }

    if (filters.name) {
      f.push({ field: "name", operator: "contains", value: filters.name });
    }
    if (filters.status && filters.status !== "all") {
      f.push({ field: "status", operator: "eq", value: filters.status });
    }

    return f;
  }, [filters]);

  const sorters: CrudSort[] = [
    { field: "created_at_epoch_ms", order: "desc" },
  ];

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Workflows</h1>
        <TriggerRunButton />
      </div>
      <WorkflowFilterBar filters={filters} onChange={setFilters} />
      <WorkflowTable
        filters={crudFilters}
        sorters={sorters}
        onRowClick={(record) => setSelectedId(record.id)}
      />
      <WorkflowSidebar
        workflowId={selectedId}
        onClose={() => setSelectedId(null)}
      />
    </div>
  );
}
