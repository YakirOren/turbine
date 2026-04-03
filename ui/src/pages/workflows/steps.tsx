import {useCallback, useEffect, useMemo, useState} from "react";
import {useNavigate, useParams} from "react-router";
import {useCustom, useList, useShow} from "@refinedev/core";
import {
    Background,
    Controls,
    type Edge,
    type Node,
    ReactFlow,
    ReactFlowProvider,
    useEdgesState,
    useNodesState,
    useReactFlow,
} from "@xyflow/react";
import dagre from "@dagrejs/dagre";
import "@xyflow/react/dist/style.css";

import {useTheme} from "next-themes";
import {Button} from "@/components/ui/button";
import {Switch} from "@/components/ui/switch";
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow,} from "@/components/ui/table";
import {Tabs, TabsContent, TabsList, TabsTrigger} from "@/components/ui/tabs";
import {AppStatusBadge, ProductStatusBadge, StatusBadge} from "@/components/status-badge";
import {nodeTypes} from "@/components/step-node";
import {pbClient} from "@/providers/pocketbase";
import {ArrowLeft, ChevronRight, Cog, FileText, Package, RefreshCw, X as XIcon} from "lucide-react";

function timeAgo(epochMs: number): string {
    const diff = Date.now() - epochMs;
    if (diff < 60000) return `${Math.round(diff / 1000)}s ago`;
    if (diff < 3600000) return `${Math.round(diff / 60000)}m ago`;
    if (diff < 86400000) return `${Math.round(diff / 3600000)}h ago`;
    return `${Math.round(diff / 86400000)}d ago`;
}

function formatDuration(ms: number): string {
    if (ms < 1000) return `${ms}ms`;
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
    const mins = Math.floor(ms / 60000);
    const secs = Math.round((ms % 60000) / 1000);
    return `${mins}min ${secs}s`;
}

function formatBytes(bytes: number): string {
    if (!bytes) return "—";
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / 1048576).toFixed(1)} MB`;
}

function InfoItem({label, value}: { label: string; value: React.ReactNode }) {
    return (
        <div className="flex flex-col gap-0.5">
            <span className="text-xs text-muted-foreground">{label}</span>
            <span className="text-sm">{value}</span>
        </div>
    );
}

interface LogRecord {
    id: string;
    level: number;
    message: string;
    created: string;
    data: Record<string, unknown>;
}

interface ProductRecord {
    id: string;
    workflow_id: string;
    function_id: number;
    function_name: string;
    file_name: string;
    size: number;
    metadata: Record<string, unknown>;
    status: string;
    error: string;
    created: string;
}

const LOG_LEVELS: Record<number, { label: string; className: string }> = {
    [-4]: {label: "DEBUG", className: "text-muted-foreground"},
    [0]: {label: "INFO", className: "text-blue-400"},
    [4]: {label: "WARN", className: "text-yellow-400"},
    [8]: {label: "ERROR", className: "text-red-400"},
};

function logLevel(level: number) {
    return LOG_LEVELS[level] ?? {label: `LVL${level}`, className: "text-muted-foreground"};
}

const HIDDEN_DATA_KEYS = new Set(["workflow_id", "source", "step_id"]);
const APP_STATUS_HIDDEN_KEYS = new Set([...HIDDEN_DATA_KEYS, "app_status", "app_status_color"]);

interface StepsTreeResponse {
    nodes: Array<{
        id: string;
        name: string;
        type: string;
        status: string;
        functionId: number;
        startedAtMs: number;
        endedAtMs: number;
        output: string | null;
        error: string | null;
        childWorkflowId: string | null;
    }>;
    edges: Array<{ source: string; target: string }>;
    workflowStatus: string;
}

function layoutWithDagre(
    nodes: Node[],
    edges: Edge[]
): { nodes: Node[]; edges: Edge[] } {
    const g = new dagre.graphlib.Graph();
    g.setDefaultEdgeLabel(() => ({}));
    g.setGraph({rankdir: "LR", nodesep: 40, ranksep: 20});

    for (const node of nodes) {
        const w = node.measured?.width ?? 80;
        const h = node.measured?.height ?? 28;
        g.setNode(node.id, {width: w, height: h});
    }
    for (const edge of edges) {
        g.setEdge(edge.source, edge.target);
    }

    dagre.layout(g);

    const layoutedNodes = nodes.map((node) => {
        const pos = g.node(node.id);
        const w = node.measured?.width ?? 80;
        const h = node.measured?.height ?? 28;
        return {
            ...node,
            position: {x: pos.x - w / 2, y: pos.y - h / 2},
        };
    });

    return {nodes: layoutedNodes, edges};
}

function StepFlowContent({workflowId}: { workflowId: string }) {
    const navigate = useNavigate();
    const {resolvedTheme} = useTheme();
    const {getNodes, fitView} = useReactFlow();
    const {query} = useShow({
        resource: "pt_workflow_status",
        liveMode: "auto",
        id: workflowId,
        queryOptions: {
          refetchInterval: (q) => {
            const s = (q.state.data?.data as Record<string, unknown> | undefined)?.status as string | undefined;
            return s && TERMINAL_STATUSES.has(s) ? false : 2000;
          },
        },
    });
    const workflow = query?.data?.data as Record<string, unknown> | undefined;
    const workflowStatus = (workflow?.status as string) ?? "";
    const isTerminal = TERMINAL_STATUSES.has(workflowStatus);
    const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
    const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);
    const [layoutDone, setLayoutDone] = useState(false);
    const [breadcrumbs, setBreadcrumbs] = useState<
        Array<{ id: string; label: string }>
    >([{id: workflowId, label: "Root"}]);
    const [selectedStepId, setSelectedStepId] = useState<number | null>(null);

    const currentWorkflowId = breadcrumbs[breadcrumbs.length - 1].id;

    const navigateToChild = useCallback(
        (childId: string) => {
            setBreadcrumbs((prev) => [...prev, {id: childId, label: childId.slice(0, 8)}]);
        },
        []
    );

    const navigateToBreadcrumb = useCallback((index: number) => {
        setBreadcrumbs((prev) => prev.slice(0, index + 1));
    }, []);

    const {query: stepsQuery} = useCustom<StepsTreeResponse>({
        url: "",
        method: "get",
        queryOptions: {
            queryKey: ["steps-tree", currentWorkflowId],
            queryFn: () =>
                pbClient
                    .send<StepsTreeResponse>(
                        `/api/pt/workflows/${currentWorkflowId}/steps-tree`,
                        {method: "GET"}
                    )
                    .then((data) => ({data})),
            refetchInterval: isTerminal ? false : 2000,
        },
    });

    const stepsData = stepsQuery.data?.data as StepsTreeResponse | undefined;
    const stepsDataUpdatedAt = stepsQuery.dataUpdatedAt;

    const selectedStep = useMemo(() => {
        if (selectedStepId == null || !stepsData) return null;
        return stepsData.nodes.find((n) => n.functionId === selectedStepId) ?? null;
    }, [selectedStepId, stepsData]);

    // Build/update React Flow nodes from steps data
    useEffect(() => {
        if (!stepsData) return;

        const stepNodes = stepsData.nodes.filter((n) => n.type !== "workflow-result");
        const stepNodeIds = new Set(stepNodes.map((n) => n.id));
        const stepEdges = stepsData.edges.filter((e) => stepNodeIds.has(e.source) && stepNodeIds.has(e.target));
        const targetIds = new Set(stepEdges.map((e) => e.target));
        const sourceIds = new Set(stepEdges.map((e) => e.source));

        setNodes((prev) => {
            const existingById = new Map(prev.map((n) => [n.id, n]));
            const needsLayout = stepNodes.length !== prev.length || stepNodes.some((n) => !existingById.has(n.id));

            const updated = stepNodes.map((n) => {
                const existing = existingById.get(n.id);
                const nodeData = {
                    label: n.name,
                    status: n.status,
                    functionId: n.functionId,
                    childWorkflowId: n.childWorkflowId,
                    onChildClick: navigateToChild,
                    hasInput: targetIds.has(n.id),
                    hasOutput: sourceIds.has(n.id),
                    durationMs: n.endedAtMs && n.startedAtMs ? n.endedAtMs - n.startedAtMs : null,
                    selected: n.functionId === selectedStepId,
                };

                if (existing && !needsLayout) {
                    return {...existing, data: nodeData};
                }
                return {
                    id: n.id,
                    type: n.type,
                    position: existing?.position ?? {x: 0, y: 0},
                    data: nodeData,
                };
            });

            if (needsLayout) {
                setLayoutDone(false);
            }
            return updated;
        });

        const rfEdges: Edge[] = stepEdges.map((e, i) => ({
            id: `e-${i}`,
            source: e.source,
            target: e.target,
            type: "bezier",
            animated: false,
            style: {stroke: "var(--color-muted-foreground)", strokeWidth: 1.5},
        }));
        setEdges(rfEdges);
    }, [stepsData, navigateToChild, selectedStepId, setNodes, setEdges]);

    // Apply dagre layout when new nodes need positioning
    useEffect(() => {
        if (stepsQuery.isLoading || layoutDone || nodes.length === 0) return;
        const measured = getNodes();
        const allMeasured = measured.every((n) => n.measured?.width);
        if (!allMeasured) {
            const raf = requestAnimationFrame(() => {
                const m = getNodes();
                if (m.every((n) => n.measured?.width)) {
                    const laid = layoutWithDagre(m, edges);
                    setNodes(laid.nodes);
                    setLayoutDone(true);
                    requestAnimationFrame(() => fitView({padding: 0.2}));
                }
            });
            return () => cancelAnimationFrame(raf);
        }
        const laid = layoutWithDagre(measured, edges);
        setNodes(laid.nodes);
        setLayoutDone(true);
        requestAnimationFrame(() => fitView({padding: 0.2}));
    }, [stepsQuery.isLoading, layoutDone, nodes, edges, getNodes, setNodes, fitView]);

    // Update node selection visual
    useEffect(() => {
        setNodes((nds) =>
            nds.map((n) => ({
                ...n,
                data: {
                    ...n.data,
                    selected: (n.data as Record<string, unknown>).functionId === selectedStepId,
                },
            }))
        );
    }, [selectedStepId, setNodes]);

    return (
        <div className="flex h-[calc(100vh-3rem)] flex-col">
            {/* Header */}
            <div className="flex items-center gap-3 border-b px-4 py-2 text-sm">
                <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => navigate("/workflows")}
                >
                    <ArrowLeft className="mr-1 h-4 w-4"/> Back
                </Button>
                {breadcrumbs.length > 1 && (
                    <div className="flex items-center gap-1">
                        {breadcrumbs.map((bc, i) => (
                            <span key={bc.id} className="flex items-center gap-1">
                {i > 0 && (
                    <ChevronRight className="h-3 w-3 text-muted-foreground"/>
                )}
                                <button
                                    className={
                                        i === breadcrumbs.length - 1
                                            ? "font-medium"
                                            : "text-muted-foreground hover:text-foreground"
                                    }
                                    onClick={() => navigateToBreadcrumb(i)}
                                >
                  {bc.label}
                </button>
              </span>
                        ))}
                    </div>
                )}
            </div>

            {/* Workflow info bar */}
            {workflow && (() => {
                const createdAt = workflow.created_at_epoch_ms as number;
                const updatedAt = workflow.updated_at_epoch_ms as number;
                const duration = createdAt && updatedAt ? updatedAt - createdAt : null;
                return (
                    <div className="flex items-center border-b px-4 py-2">
                        <div className="flex flex-col gap-0.5">
                            <span className="font-medium">{workflow.name as string}</span>
                            <span className="font-mono text-xs text-muted-foreground">
                {workflow.id as string}
              </span>
                        </div>
                        <div className="ml-auto flex items-center gap-6">
                            <InfoItem label="Status" value={<StatusBadge status={(workflow.status as string) ?? ""}/>}/>
                            <InfoItem label="Started" value={createdAt ? timeAgo(createdAt) : "—"}/>
                            <InfoItem label="Duration" value={duration ? formatDuration(duration) : "—"}/>
                        </div>
                    </div>
                );
            })()}

            {/* React Flow canvas */}
            <div className="mx-4 mt-4 h-[250px] rounded-lg border">
                {stepsQuery.isLoading ? (
                    <div className="flex h-full items-center justify-center text-muted-foreground">
                        Loading steps...
                    </div>
                ) : (
                    <ReactFlow
                        nodes={nodes}
                        edges={edges}
                        onNodesChange={onNodesChange}
                        onEdgesChange={onEdgesChange}
                        onNodeClick={(_event, node) => {
                            const fid = (node.data as Record<string, unknown>).functionId as number | undefined;
                            if (fid != null) {
                                setSelectedStepId((prev) => (prev === fid ? null : fid));
                            }
                        }}
                        onPaneClick={() => setSelectedStepId(null)}
                        nodeTypes={nodeTypes}
                        nodesDraggable={false}
                        colorMode={resolvedTheme === "dark" ? "dark" : "light"}
                        fitView
                        proOptions={{hideAttribution: true}}
                    >
                        <Background/>
                        <Controls/>
                    </ReactFlow>
                )}
            </div>

            {/* Step detail */}
            {selectedStep && (
                <div className="mx-4 mt-2 rounded-md border p-3">
                    <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2 text-sm font-medium">
                            <span>{selectedStep.name}</span>
                            <StatusBadge status={selectedStep.status === "success" ? "SUCCESS" : selectedStep.status === "error" ? "ERROR" : "PENDING"} />
                            {selectedStep.startedAtMs > 0 && selectedStep.endedAtMs > 0 && (
                                <span className="font-mono text-xs text-muted-foreground">
                                    {formatDuration(selectedStep.endedAtMs - selectedStep.startedAtMs)}
                                </span>
                            )}
                        </div>
                        <Button variant="ghost" size="sm" className="h-6 w-6 p-0" onClick={() => setSelectedStepId(null)}>
                            <XIcon className="h-3.5 w-3.5" />
                        </Button>
                    </div>
                    {selectedStep.error && (
                        <pre className="mt-2 max-h-32 overflow-auto rounded bg-red-500/10 p-2 text-xs text-red-400">
                            {selectedStep.error}
                        </pre>
                    )}
                    {selectedStep.output && (
                        <pre className="mt-2 max-h-48 overflow-auto rounded bg-muted p-2 font-mono text-xs">
                            {(() => {
                                try { return JSON.stringify(JSON.parse(selectedStep.output), null, 2); }
                                catch { return selectedStep.output; }
                            })()}
                        </pre>
                    )}
                    {!selectedStep.output && !selectedStep.error && selectedStep.status === "running" && (
                        <p className="mt-2 text-xs text-muted-foreground">Step is still running...</p>
                    )}
                </div>
            )}

            {/* Logs & Products */}
            <Tabs defaultValue="logs" className="flex-1 flex flex-col overflow-hidden">
                <div className="px-4 pt-2">
                    <TabsList>
                        <TabsTrigger value="logs"><FileText className="h-3.5 w-3.5 mr-1.5"/>Logs</TabsTrigger>
                        <TabsTrigger value="products"><Package className="h-3.5 w-3.5 mr-1.5"/>Products</TabsTrigger>
                    </TabsList>
                </div>
                <TabsContent value="logs" className="flex-1 overflow-y-auto mt-0">
                    <WorkflowLogs
                        workflowId={workflowId}
                        stepId={selectedStepId}
                        stepsDataUpdatedAt={stepsDataUpdatedAt}
                        status={(workflow?.status as string) ?? ""}
                    />
                </TabsContent>
                <TabsContent value="products" className="flex-1 overflow-y-auto mt-0">
                    <WorkflowProducts
                        workflowId={workflowId}
                        status={(workflow?.status as string) ?? ""}
                    />
                </TabsContent>
            </Tabs>
        </div>
    );
}

const TERMINAL_STATUSES = new Set(["SUCCESS", "ERROR", "CANCELLED", "MAX_RECOVERY_ATTEMPTS_EXCEEDED"]);

function WorkflowLogs({workflowId, stepId, stepsDataUpdatedAt, status}: {
    workflowId: string;
    stepId: number | null;
    stepsDataUpdatedAt: number;
    status: string
}) {
    const [showSystem, setShowSystem] = useState(false);

    const filter = useMemo(() => {
        let f = `data.workflow_id = "${workflowId}"`;
        if (stepId != null) {
            f += ` && data.step_id = ${stepId}`;
        }
        if (!showSystem) {
            f += ` && (data.source != "system" || message = "app status changed")`;
        }
        return f;
    }, [workflowId, stepId, showSystem]);

    const {query: logsQuery} = useCustom<LogRecord[]>({
        url: "",
        method: "get",
        queryOptions: {
            queryKey: ["workflow-logs", workflowId, stepId, showSystem, stepsDataUpdatedAt],
            queryFn: () =>
                pbClient.logs
                    .getList(1, 100, {filter, sort: "created"})
                    .then((result) => ({
                        data: result.items.map((item) => ({
                            id: item.id,
                            level: Number(item.level),
                            message: item.message,
                            created: item.created,
                            data: (item.data ?? {}) as Record<string, unknown>,
                        })),
                    })),
            refetchInterval: TERMINAL_STATUSES.has(status) ? 5000 : 2000,
        },
    });

    const logs = (logsQuery.data?.data ?? []) as LogRecord[];
    const logsLoading = logsQuery.isLoading;
    const [spinning, setSpinning] = useState(false);

    const handleRefresh = useCallback(() => {
        setSpinning(true);
        logsQuery.refetch().finally(() => {
            setTimeout(() => setSpinning(false), 400);
        });
    }, [logsQuery]);

    return (
        <div className="flex-1 overflow-y-auto p-4">
            <div className="mb-2 flex items-center justify-between">
                <h3 className="text-sm font-semibold">Logs</h3>
                <div className="flex items-center gap-3">
                    <Button
                        variant="ghost"
                        size="sm"
                        className="h-7 w-7 p-0"
                        disabled={spinning}
                        onClick={handleRefresh}
                    >
                        <RefreshCw className={`h-3.5 w-3.5 ${spinning ? "animate-spin" : ""}`}/>
                    </Button>
                    <label className="flex items-center gap-2 text-sm text-muted-foreground">
                        System logs
                        <Switch checked={showSystem} onCheckedChange={setShowSystem}/>
                    </label>
                </div>
            </div>
            {logsLoading ? (
                <div className="text-sm text-muted-foreground">Loading logs...</div>
            ) : logs.length === 0 ? (
                <div className="text-sm text-muted-foreground">No logs found.</div>
            ) : (
                <div className="rounded-md border">
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead className="w-36">Time</TableHead>
                                <TableHead className="w-20">Level</TableHead>
                                <TableHead>Message</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {logs.map((log) => {
                                const lvl = logLevel(log.level);
                                const isSystem = log.data.source === "system";
                                const isAppStatus = log.message === "app status changed";
                                const attrs = Object.entries(log.data).filter(
                                    ([k]) => !(isAppStatus ? APP_STATUS_HIDDEN_KEYS : HIDDEN_DATA_KEYS).has(k)
                                );
                                return (
                                    <TableRow key={log.id} className={isSystem && !isAppStatus ? "opacity-60" : ""}>
                                        <TableCell className="font-mono text-xs text-muted-foreground">
                                            {new Date(log.created).toLocaleString(undefined, {
                                                year: "numeric",
                                                month: "2-digit",
                                                day: "2-digit",
                                                hour: "2-digit",
                                                minute: "2-digit",
                                                second: "2-digit",
                                                fractionalSecondDigits: 3,
                                            })}
                                        </TableCell>
                                        <TableCell>
                      <span className={`font-mono text-xs font-medium ${lvl.className}`}>
                        {lvl.label}
                      </span>
                                        </TableCell>
                                        <TableCell className="font-mono text-xs">
                                            {isAppStatus ? (
                                                <span className="flex items-center gap-1.5">
                          App status changed to{" "}
                                                    <AppStatusBadge
                                                        label={String(log.data.app_status ?? "")}
                                                        color={String(log.data.app_status_color ?? "")}
                                                    />
                        </span>
                                            ) : (
                                                <>
                          <span className="flex items-center gap-1.5">
                            {showSystem && isSystem && (
                                <Cog className="h-3 w-3 shrink-0 text-muted-foreground"/>
                            )}
                              {log.message}
                          </span>
                                                    {attrs.length > 0 && (
                                                        <span className="mt-0.5 flex flex-wrap gap-1">
                              {attrs.map(([k, v]) => (
                                  <span
                                      key={k}
                                      className="rounded bg-muted px-1 py-0.5 text-[10px] text-muted-foreground"
                                  >
                                  {k}={String(v)}
                                </span>
                              ))}
                            </span>
                                                    )}
                                                </>
                                            )}
                                        </TableCell>
                                    </TableRow>
                                );
                            })}
                        </TableBody>
                    </Table>
                </div>
            )}
        </div>
    );
}

function WorkflowProducts({workflowId, status}: { workflowId: string; status: string }) {
    const {query: productsQuery} = useList({
        resource: "pt_products",
        filters: [
            {field: "workflow_id", operator: "eq", value: workflowId},
        ],
        sorters: [{field: "function_id", order: "asc"}],
        pagination: {pageSize: 100},
        queryOptions: {
            refetchInterval: TERMINAL_STATUSES.has(status) ? 5000 : 2000,
        },
    });

    const products = (productsQuery.data?.data ?? []) as ProductRecord[];
    const loading = productsQuery.isLoading;
    const [spinning, setSpinning] = useState(false);

    const handleRefresh = useCallback(() => {
        setSpinning(true);
        productsQuery.refetch().finally(() => {
            setTimeout(() => setSpinning(false), 400);
        });
    }, [productsQuery]);

    return (
        <div className="flex-1 overflow-y-auto p-4">
            <div className="mb-2 flex items-center justify-between">
                <h3 className="text-sm font-semibold">Products</h3>
                <div className="flex items-center gap-3">
                    <Button
                        variant="ghost"
                        size="sm"
                        className="h-7 w-7 p-0"
                        disabled={spinning}
                        onClick={handleRefresh}
                    >
                        <RefreshCw className={`h-3.5 w-3.5 ${spinning ? "animate-spin" : ""}`}/>
                    </Button>
                </div>
            </div>
            {loading ? (
                <div className="text-sm text-muted-foreground">Loading products...</div>
            ) : products.length === 0 ? (
                <div className="text-sm text-muted-foreground">This workflow hasn't produced any files yet.</div>
            ) : (
                <div className="rounded-md border">
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead className="w-36">Time</TableHead>
                                <TableHead>File</TableHead>
                                <TableHead className="w-20">Size</TableHead>
                                <TableHead>Step</TableHead>
                                <TableHead className="w-24">Status</TableHead>
                                <TableHead>Error</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {products.map((p) => (
                                <TableRow key={p.id}>
                                    <TableCell className="font-mono text-xs text-muted-foreground">
                                        {new Date(p.created).toLocaleString(undefined, {
                                            year: "numeric",
                                            month: "2-digit",
                                            day: "2-digit",
                                            hour: "2-digit",
                                            minute: "2-digit",
                                            second: "2-digit",
                                        })}
                                    </TableCell>
                                    <TableCell className="font-mono text-xs">{p.file_name}</TableCell>
                                    <TableCell className="font-mono text-xs text-muted-foreground">
                                        {formatBytes(p.size)}
                                    </TableCell>
                                    <TableCell className="font-mono text-xs text-muted-foreground">
                                        {p.function_name || `Step ${p.function_id}`}
                                    </TableCell>
                                    <TableCell>
                                        <ProductStatusBadge status={p.status}/>
                                    </TableCell>
                                    <TableCell className="font-mono text-xs text-red-400 max-w-[200px] truncate" title={p.error}>
                                        {p.error || "\u2014"}
                                    </TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>
                </div>
            )}
        </div>
    );
}

export function WorkflowSteps() {
    const {id} = useParams();
    if (!id) return null;
    return (
        <ReactFlowProvider>
            <StepFlowContent workflowId={id}/>
        </ReactFlowProvider>
    );
}
