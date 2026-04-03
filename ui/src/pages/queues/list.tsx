import { useState, useMemo } from "react";
import type { CrudFilters, CrudSort } from "@refinedev/core";
import { useQuery } from "@tanstack/react-query";
import { LayoutList } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Input } from "@/components/ui/input";
import { WorkflowTable } from "@/components/workflow-table";
import { WorkflowSidebar } from "@/components/workflow-sidebar";
import { pbClient } from "@/providers/pocketbase";

interface QueueInfo {
  name: string;
  workerConcurrency?: number;
  globalConcurrency?: number;
  priorityEnabled: boolean;
  partitioned: boolean;
  rateLimit?: { limit: number; period: string };
}

interface QueueStats {
  enqueued: number;
  running: number;
  completed: number;
  failed: number;
}

export function QueueList() {
  const [selectedQueue, setSelectedQueue] = useState<string>("");
  const [nameFilter, setNameFilter] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const { data: queues = [], isLoading: queuesLoading } = useQuery<QueueInfo[]>({
    queryKey: ["queues"],
    queryFn: () =>
      pbClient
        .send<QueueInfo[]>("/api/pt/queues", { method: "GET" })
        .then((data) => {
          if (data.length > 0 && !selectedQueue) {
            setSelectedQueue(data[0].name);
          }
          return data;
        }),
  });

  const loading = queuesLoading && queues.length === 0;

  const { data: stats = null } = useQuery<QueueStats>({
    queryKey: ["queue-stats", selectedQueue],
    queryFn: () =>
      pbClient
        .send<QueueStats>(
          `/api/pt/queues/${encodeURIComponent(selectedQueue)}/stats`,
          { method: "GET" }
        ),
    enabled: !!selectedQueue,
    refetchInterval: 5000,
  });
  const selectedQueueInfo = queues.find((q) => q.name === selectedQueue);

  const crudFilters = useMemo<CrudFilters>(() => {
    const f: CrudFilters = [];
    if (selectedQueue) {
      f.push({ field: "queue_name", operator: "eq", value: selectedQueue });
    }
    if (nameFilter) {
      f.push({ field: "name", operator: "contains", value: nameFilter });
    }
    return f;
  }, [selectedQueue, nameFilter]);

  const sorters: CrudSort[] = [
    { field: "created_at_epoch_ms", order: "desc" },
  ];

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center text-muted-foreground">
        Loading...
      </div>
    );
  }

  if (queues.length === 0) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 text-muted-foreground">
        <LayoutList className="h-8 w-8" />
        <div className="text-center">
          <p>No queues registered.</p>
          <p className="mt-1 text-xs">
            Register a queue with{" "}
            <code className="rounded bg-muted px-1.5 py-0.5 font-mono">
              rt.Queue("name")
            </code>
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">Queues</h1>

      <div className="flex items-center gap-3">
        <Select value={selectedQueue} onValueChange={setSelectedQueue}>
          <SelectTrigger className="w-52">
            <SelectValue placeholder="Select queue" />
          </SelectTrigger>
          <SelectContent>
            {queues.map((q) => (
              <SelectItem key={q.name} value={q.name}>
                {q.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Input
          placeholder="Workflow name"
          value={nameFilter}
          onChange={(e) => setNameFilter(e.target.value)}
          className="w-44"
        />

        {stats && (
          <div className="flex items-center gap-2">
            <Badge variant="secondary" className="font-mono text-xs">
              {stats.enqueued} enqueued
            </Badge>
            <Badge variant="secondary" className="font-mono text-xs text-blue-400">
              {stats.running} running
            </Badge>
            <Badge variant="secondary" className="font-mono text-xs text-green-400">
              {stats.completed} completed
            </Badge>
            {stats.failed > 0 && (
              <Badge variant="secondary" className="font-mono text-xs text-red-400">
                {stats.failed} failed
              </Badge>
            )}
          </div>
        )}
      </div>

      {selectedQueueInfo && (
        <div className="flex flex-wrap gap-3 text-sm text-muted-foreground">
          {selectedQueueInfo.workerConcurrency != null && (
            <span>Worker Concurrency: {selectedQueueInfo.workerConcurrency}</span>
          )}
          {selectedQueueInfo.globalConcurrency != null && (
            <span>Global Concurrency: {selectedQueueInfo.globalConcurrency}</span>
          )}
          {selectedQueueInfo.priorityEnabled && <span>Priority: enabled</span>}
          {selectedQueueInfo.partitioned && <span>Partitioned</span>}
          {selectedQueueInfo.rateLimit && (
            <span>
              Rate Limit: {selectedQueueInfo.rateLimit.limit}/
              {selectedQueueInfo.rateLimit.period}
            </span>
          )}
        </div>
      )}

      {selectedQueue && (
        <WorkflowTable
          filters={crudFilters}
          sorters={sorters}
          onRowClick={(record) => setSelectedId(record.id)}
        />
      )}

      <WorkflowSidebar
        workflowId={selectedId}
        onClose={() => setSelectedId(null)}
      />
    </div>
  );
}
