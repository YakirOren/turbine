import { useState, useEffect, useMemo } from "react";
import type { CrudFilters, CrudSort } from "@refinedev/core";
import { LayoutList } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
  const [queues, setQueues] = useState<QueueInfo[]>([]);
  const [selectedQueue, setSelectedQueue] = useState<string>("");
  const [stats, setStats] = useState<QueueStats | null>(null);
  const [nameFilter, setNameFilter] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    pbClient
      .send<QueueInfo[]>("/api/pt/queues", { method: "GET" })
      .then((data) => {
        setQueues(data);
        if (data.length > 0) {
          setSelectedQueue((prev) => prev || data[0].name);
        }
      })
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    if (!selectedQueue) return;
    pbClient
      .send<QueueStats>(`/api/pt/queues/${encodeURIComponent(selectedQueue)}/stats`, {
        method: "GET",
      })
      .then(setStats);
  }, [selectedQueue]);

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
      </div>

      {stats && (
        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm text-muted-foreground">
                Enqueued
              </CardTitle>
            </CardHeader>
            <CardContent>
              <span className="text-2xl font-bold">{stats.enqueued}</span>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm text-muted-foreground">
                Running
              </CardTitle>
            </CardHeader>
            <CardContent>
              <span className="text-2xl font-bold text-blue-400">
                {stats.running}
              </span>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm text-muted-foreground">
                Completed
              </CardTitle>
            </CardHeader>
            <CardContent>
              <span className="text-2xl font-bold text-green-400">
                {stats.completed}
              </span>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm text-muted-foreground">
                Failed
              </CardTitle>
            </CardHeader>
            <CardContent>
              <span className="text-2xl font-bold text-red-400">
                {stats.failed}
              </span>
            </CardContent>
          </Card>
        </div>
      )}

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
