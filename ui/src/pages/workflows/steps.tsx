import { Fragment, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { useList, useShow } from "@refinedev/core";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import {
    Background,
    BackgroundVariant,
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

import { useTheme } from "next-themes";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { DrawerShell } from "@/components/drawer-shell";
import { AppStatusBadge, Pill, ProductStatusBadge, StatusBadge, StepStatusBadge } from "@/components/status-badge";
import { nodeTypes } from "@/components/step-node";
import { StepTimeline } from "@/components/step-timeline";
import { TERMINAL_STATUSES } from "@/components/step-status";
import { pbClient } from "@/providers/pocketbase";
import { useMediaQuery } from "@/lib/use-media-query";
import {
    ArrowLeft,
    ChevronRight,
    ChevronUp,
    Cog,
    Download,
    FileText,
    GitFork,
    ListChecks,
    Package,
    RefreshCw,
    Search,
    X,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { formatDuration, formatTimestampPrecise, timeAgo } from "@/lib/format";
import { patchWorkflowFilters } from "@/pages/workflows/list";
import type { PtProductsResponse } from "@/types/pocketbase-types";

type Layout = "split" | "graph-only" | "timeline-only";

const LAYOUT_STORAGE_KEY = "workflows.steps.layout";

function isLayout(v: unknown): v is Layout {
    return v === "split" || v === "graph-only" || v === "timeline-only";
}

function readStoredLayout(): Layout {
    if (typeof window === "undefined") return "split";
    const v = window.localStorage.getItem(LAYOUT_STORAGE_KEY);
    return isLayout(v) ? v : "split";
}

function formatBytes(bytes: number): string {
    if (!bytes) return "—";
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / 1048576).toFixed(1)} MB`;
}

interface LogRecord {
    id: string;
    level: number;
    message: string;
    created: string;
    data: Record<string, unknown>;
}

type ProductRecord = PtProductsResponse<Record<string, unknown>>;

const LOG_LEVELS: Record<number, { label: string; className: string }> = {
    [-4]: { label: "DEBUG", className: "bg-secondary text-muted-foreground" },
    [0]: { label: "INFO", className: "bg-info-soft text-info-foreground" },
    [4]: { label: "WARN", className: "bg-warning-soft text-warning-foreground" },
    [8]: { label: "ERROR", className: "bg-danger-soft text-danger-foreground" },
};

const LEVEL_RAIL: Record<number, string> = {
    [8]: "border-l-danger",
    [4]: "border-l-warning",
    [0]: "border-l-transparent",
    [-4]: "border-l-transparent",
};

function logLevel(level: number) {
    return LOG_LEVELS[level] ?? { label: `LVL${level}`, className: "bg-secondary text-muted-foreground" };
}

const LOG_PAGE_SIZE = 100;

const HIDDEN_DATA_KEYS = new Set(["workflow_id", "source", "step_id"]);
const APP_STATUS_HIDDEN_KEYS = new Set([...HIDDEN_DATA_KEYS, "app_status", "app_status_color"]);

const TABLE_HEADER_CLASS =
    "sticky top-0 z-10 grid gap-x-2 border-l-[3px] border-l-transparent bg-background px-4 py-2 text-[10.5px] font-medium uppercase tracking-[0.06em] text-muted-foreground";
const TABLE_ROW_BASE_CLASS =
    "grid gap-x-2 border-t border-l-[3px] border-border-soft px-4 py-2 text-[12px]";

const MAX_LAYOUT_FRAMES = 120;

export interface StepsTreeResponse {
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

export type StepNodeRecord = StepsTreeResponse["nodes"][number];

function layoutWithDagre(
    nodes: Node[],
    edges: Edge[]
): { nodes: Node[]; edges: Edge[] } {
    const g = new dagre.graphlib.Graph();
    g.setDefaultEdgeLabel(() => ({}));
    g.setGraph({ rankdir: "LR", nodesep: 40, ranksep: 28 });

    for (const node of nodes) {
        const w = node.measured?.width ?? 140;
        const h = node.measured?.height ?? 34;
        g.setNode(node.id, { width: w, height: h });
    }
    for (const edge of edges) {
        g.setEdge(edge.source, edge.target);
    }

    dagre.layout(g);

    const layoutedNodes = nodes.map((node) => {
        const pos = g.node(node.id);
        const w = node.measured?.width ?? 140;
        const h = node.measured?.height ?? 34;
        return {
            ...node,
            position: { x: pos.x - w / 2, y: pos.y - h / 2 },
        };
    });

    return { nodes: layoutedNodes, edges };
}

function StepFlowContent({ workflowId }: { workflowId: string }) {
    const navigate = useNavigate();
    const { resolvedTheme } = useTheme();
    const { getNodes, fitView } = useReactFlow();
    const { query } = useShow({
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
    const [breadcrumbs, setBreadcrumbs] = useState<Array<{ id: string; label: string }>>([
        { id: workflowId, label: "Root" },
    ]);
    type Selection = { kind: "step"; id: number } | { kind: "product"; id: string } | null;
    const [selection, setSelection] = useState<Selection>(null);
    const selectedStepId = selection?.kind === "step" ? selection.id : null;
    const selectedProductId = selection?.kind === "product" ? selection.id : null;
    const setSelectedStepId = useCallback((value: React.SetStateAction<number | null>) => {
        setSelection((prev) => {
            const prevId = prev?.kind === "step" ? prev.id : null;
            const next = typeof value === "function" ? value(prevId) : value;
            return next == null ? null : { kind: "step", id: next };
        });
    }, []);
    const setSelectedProductId = useCallback((value: string | null) => {
        setSelection(value == null ? null : { kind: "product", id: value });
    }, []);
    const [activeTab, setActiveTab] = useState<"logs" | "products">("logs");
    const [layout, setLayout] = useState<Layout>(readStoredLayout);

    useEffect(() => {
        window.localStorage.setItem(LAYOUT_STORAGE_KEY, layout);
    }, [layout]);

    const changeLayout = useCallback(
        (next: Layout) => {
            setLayout(next);
            if (next === "timeline-only") return;
            requestAnimationFrame(() => {
                requestAnimationFrame(() => {
                    fitView({ duration: 200, padding: 0.1 });
                });
            });
        },
        [fitView]
    );

    const currentWorkflowId = breadcrumbs[breadcrumbs.length - 1].id;

    const navigateToChild = useCallback((childId: string) => {
        setBreadcrumbs((prev) => [...prev, { id: childId, label: childId.slice(0, 8) }]);
    }, []);

    const navigateToBreadcrumb = useCallback((index: number) => {
        setBreadcrumbs((prev) => prev.slice(0, index + 1));
    }, []);

    const stepsQuery = useQuery<StepsTreeResponse>({
        queryKey: ["steps-tree", currentWorkflowId],
        queryFn: () =>
            pbClient.send<StepsTreeResponse>(
                `/api/pt/workflows/${currentWorkflowId}/steps-tree`,
                { method: "GET" }
            ),
        refetchInterval: isTerminal ? false : 2000,
    });

    const stepsData = stepsQuery.data;

    const selectedStep: StepNodeRecord | null = useMemo(() => {
        if (selectedStepId == null || !stepsData) return null;
        return stepsData.nodes.find((n) => n.functionId === selectedStepId) ?? null;
    }, [selectedStepId, stepsData]);

    const { query: productsQuery } = useList({
        resource: "pt_products",
        filters: [{ field: "workflow_id", operator: "eq", value: workflowId }],
        sorters: [{ field: "function_id", order: "asc" }],
        pagination: { pageSize: 100 },
        queryOptions: {
            refetchInterval: isTerminal || activeTab !== "products" ? false : 2000,
        },
    });
    const products = useMemo(
        () => (productsQuery.data?.data ?? []) as ProductRecord[],
        [productsQuery.data]
    );
    const productsLoading = productsQuery.isLoading;
    const selectedProduct: ProductRecord | null = useMemo(() => {
        if (selectedProductId == null) return null;
        return products.find((p) => p.id === selectedProductId) ?? null;
    }, [selectedProductId, products]);

    const stepNodes = useMemo<StepNodeRecord[]>(
        () => stepsData?.nodes.filter((n) => n.type !== "workflow-result") ?? [],
        [stepsData]
    );

    useEffect(() => {
        if (!stepsData) return;
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
                    return { ...existing, data: nodeData };
                }
                return {
                    id: n.id,
                    type: n.type,
                    position: existing?.position ?? { x: 0, y: 0 },
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
            style: { stroke: "var(--muted-foreground)", strokeWidth: 1.25, opacity: 0.7 },
        }));
        setEdges(rfEdges);
    }, [stepsData, stepNodes, navigateToChild, selectedStepId, setNodes, setEdges]);

    const showGraph = layout === "split" || layout === "graph-only";

    useEffect(() => {
        if (!showGraph || stepsQuery.isLoading || layoutDone || nodes.length === 0) return;
        let raf = 0;
        let attempts = 0;
        const tryLayout = () => {
            const m = getNodes();
            if (m.length === nodes.length && m.every((n) => n.measured?.width)) {
                const laid = layoutWithDagre(m, edges);
                setNodes(laid.nodes);
                setLayoutDone(true);
                requestAnimationFrame(() => fitView({ padding: 0.2 }));
            } else if (++attempts < MAX_LAYOUT_FRAMES) {
                raf = requestAnimationFrame(tryLayout);
            }
        };
        raf = requestAnimationFrame(tryLayout);
        return () => cancelAnimationFrame(raf);
    }, [showGraph, stepsQuery.isLoading, layoutDone, nodes, edges, getNodes, setNodes, fitView]);

    useEffect(() => {
        if (!showGraph || !layoutDone || nodes.length === 0) return;
        const raf = requestAnimationFrame(() => fitView({ padding: 0.2 }));
        return () => cancelAnimationFrame(raf);
    }, [showGraph, layoutDone, nodes.length, fitView]);

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

    const createdAt = workflow?.created_at_epoch_ms as number | undefined;
    const updatedAt = workflow?.updated_at_epoch_ms as number | undefined;
    const duration = createdAt && updatedAt ? updatedAt - createdAt : null;

    return (
        <div className="flex h-full min-h-0 flex-1 flex-col bg-card">
            <TopBar
                workflowName={workflow?.name as string | undefined}
                workflowId={workflowId}
                breadcrumbs={breadcrumbs}
                onBack={() => navigate("/workflows")}
                onBreadcrumb={navigateToBreadcrumb}
                onWorkflowNameClick={(name) => {
                    patchWorkflowFilters({ name });
                    navigate("/workflows");
                }}
            />

            <div className="flex min-h-0 flex-1">
                {/* Scrollable main column */}
                <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
                    <div className="flex-1 overflow-y-auto">
                        <RunHeader
                            name={(workflow?.name as string) ?? ""}
                            summary={workflow?.summary as string | undefined}
                            workflowId={workflowId}
                            status={workflowStatus}
                            startedAtMs={createdAt}
                            durationMs={duration}
                            stepCount={stepNodes.length}
                        />

                        <div className="px-6 pt-4">
                            <div className="mb-3 flex items-center justify-between">
                                <span className="text-[11px] font-medium uppercase tracking-[0.06em] text-muted-foreground">
                                    Run view
                                </span>
                                <ViewToggle layout={layout} onChange={changeLayout} />
                            </div>

                            <div
                                className={cn(
                                    "grid gap-4",
                                    layout === "split" && "grid-cols-1 xl:[grid-template-columns:1.3fr_1fr]"
                                )}
                            >
                                {(layout === "split" || layout === "graph-only") && (
                                    <div className="relative h-[380px] overflow-hidden rounded-lg border bg-background animate-in fade-in slide-in-from-bottom-4 duration-500 ease-out">
                                        {stepsQuery.isLoading ? (
                                            <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
                                                Loading graph…
                                            </div>
                                        ) : (
                                            <ReactFlow
                                                nodes={nodes}
                                                edges={edges}
                                                onNodesChange={onNodesChange}
                                                onEdgesChange={onEdgesChange}
                                                onNodeClick={(_e, node) => {
                                                    const fid = (node.data as Record<string, unknown>).functionId as
                                                        | number
                                                        | undefined;
                                                    if (fid != null) {
                                                        setSelectedStepId((prev) => (prev === fid ? null : fid));
                                                    }
                                                }}
                                                onPaneClick={() => setSelectedStepId(null)}
                                                nodeTypes={nodeTypes}
                                                nodesDraggable={false}
                                                colorMode={resolvedTheme === "dark" ? "dark" : "light"}
                                                fitView
                                                proOptions={{ hideAttribution: true }}
                                            >
                                                <Background variant={BackgroundVariant.Dots} gap={16} size={1} />
                                                <Controls showInteractive={false} />
                                            </ReactFlow>
                                        )}
                                    </div>
                                )}

                                {(layout === "split" || layout === "timeline-only") && (
                                    <StepTimeline
                                        steps={stepNodes}
                                        workflowStartMs={createdAt}
                                        workflowStatus={workflowStatus}
                                        selectedStepId={selectedStepId}
                                        onSelectStep={setSelectedStepId}
                                        onDrillChild={navigateToChild}
                                        isLoading={stepsQuery.isLoading}
                                    />
                                )}
                            </div>
                        </div>

                        <div className="px-6 pb-6 pt-4">
                            <LogsPanel
                                tab={activeTab}
                                setTab={setActiveTab}
                                workflowId={workflowId}
                                stepId={selectedStepId}
                                setStepId={setSelectedStepId}
                                workflowStatus={workflowStatus}
                                products={products}
                                productsLoading={productsLoading}
                                selectedProductId={selectedProductId}
                                setSelectedProductId={setSelectedProductId}
                            />
                        </div>
                    </div>
                </div>

                {selectedStep && (
                    <StepInspector step={selectedStep} onClose={() => setSelection(null)} />
                )}
                {selectedProduct && (
                    <ProductInspector product={selectedProduct} onClose={() => setSelection(null)} />
                )}
            </div>
        </div>
    );
}

function TopBar({
    workflowName,
    workflowId,
    breadcrumbs,
    onBack,
    onBreadcrumb,
    onWorkflowNameClick,
}: {
    workflowName?: string;
    workflowId: string;
    breadcrumbs: Array<{ id: string; label: string }>;
    onBack: () => void;
    onBreadcrumb: (index: number) => void;
    onWorkflowNameClick: (name: string) => void;
}) {
    const shortId = `${workflowId.slice(0, 16)}…`;
    return (
        <div className="flex h-14 items-center gap-3 border-b bg-card px-5">
            <Button variant="outline" size="sm" className="h-7 px-2.5" onClick={onBack}>
                <ArrowLeft className="mr-1 h-3.5 w-3.5" />
                Back
            </Button>
            <div className="mx-1 h-4 w-px bg-border" />
            <div className="flex items-center gap-2 text-[13px]">
                <span className="text-muted-foreground">Workflows</span>
                <span className="text-muted-foreground/60">/</span>
                {workflowName ? (
                    <button
                        type="button"
                        onClick={() => onWorkflowNameClick(workflowName)}
                        className="text-muted-foreground transition-colors hover:text-foreground"
                    >
                        {workflowName}
                    </button>
                ) : (
                    <span className="text-muted-foreground">—</span>
                )}
                <span className="text-muted-foreground/60">/</span>
                <span className="font-mono text-[12.5px] font-medium text-foreground">{shortId}</span>
            </div>

            {breadcrumbs.length > 1 && (
                <>
                    <div className="mx-1 h-4 w-px bg-border" />
                    <div className="flex items-center gap-1 text-[12.5px]">
                        {breadcrumbs.map((bc, i) => (
                            <span key={bc.id} className="flex items-center gap-1">
                                {i > 0 && <ChevronRight className="h-3 w-3 text-muted-foreground/60" />}
                                <button
                                    className={cn(
                                        "transition-colors",
                                        i === breadcrumbs.length - 1
                                            ? "font-medium text-foreground"
                                            : "text-muted-foreground hover:text-foreground"
                                    )}
                                    onClick={() => onBreadcrumb(i)}
                                >
                                    {bc.label}
                                </button>
                            </span>
                        ))}
                    </div>
                </>
            )}

            <div className="flex-1" />
        </div>
    );
}

function RunHeader({
    name,
    summary,
    workflowId,
    status,
    startedAtMs,
    durationMs,
    stepCount,
}: {
    name: string;
    summary?: string;
    workflowId: string;
    status: string;
    startedAtMs?: number;
    durationMs: number | null;
    stepCount: number;
}) {
    return (
        <div className="min-h-[7rem] border-b border-border-soft bg-card px-6 pb-4 pt-5">
            <div className="flex flex-wrap items-start justify-between gap-6">
                <div className="min-w-0 flex-1" style={{ minWidth: 260 }}>
                    <div className="mb-1.5 flex flex-wrap items-center gap-2.5">
                        <h2 className="m-0 text-[18px] font-semibold tracking-tight text-foreground">
                            {name || "—"}
                        </h2>
                        {status && <StatusBadge status={status} />}
                    </div>
                    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[12.5px] text-muted-foreground">
                        <span className="font-mono text-[12px]">{workflowId}</span>
                        {summary && (
                            <>
                                <span className="text-muted-foreground/60">•</span>
                                <span>{summary}</span>
                            </>
                        )}
                    </div>
                </div>

                <div className="grid auto-cols-[minmax(112px,auto)] grid-flow-col overflow-hidden rounded-lg border">
                    <MetaCell
                        label="Started"
                        value={startedAtMs ? timeAgo(startedAtMs) : "—"}
                        sub={startedAtMs ? formatTimestampPrecise(new Date(startedAtMs).toISOString()).slice(0, 19) : ""}
                    />
                    <MetaCell
                        label="Duration"
                        value={durationMs != null ? formatDuration(durationMs) : "—"}
                        sub={stepCount > 0 ? `${stepCount} step${stepCount === 1 ? "" : "s"}` : ""}
                        divider
                    />
                </div>
            </div>
        </div>
    );
}

function MetaCell({
    label,
    value,
    sub,
    divider,
}: {
    label: string;
    value: React.ReactNode;
    sub?: string;
    divider?: boolean;
}) {
    return (
        <div className={cn("px-4 py-2.5", divider && "border-l border-border-soft")}>
            <div className="mb-0.5 text-[10.5px] font-medium uppercase tracking-[0.06em] text-muted-foreground">
                {label}
            </div>
            <div className="text-[14px] font-semibold leading-tight text-foreground">{value}</div>
            {sub && <div className="mt-0.5 text-[11px] text-muted-foreground">{sub}</div>}
        </div>
    );
}

function ViewToggle({ layout, onChange }: { layout: Layout; onChange: (l: Layout) => void }) {
    const showGraph = layout === "split" || layout === "graph-only";
    const showTimeline = layout === "split" || layout === "timeline-only";
    const toggle = (key: "graph" | "timeline") => {
        let g = showGraph;
        let t = showTimeline;
        if (key === "graph") g = !g;
        if (key === "timeline") t = !t;
        if (!g && !t) {
            if (key === "graph") t = true;
            else g = true;
        }
        onChange(g && t ? "split" : g ? "graph-only" : "timeline-only");
    };
    return (
        <div className="inline-flex">
            <ToggleBtn active={showGraph} onClick={() => toggle("graph")} position="first" icon={<GitFork className="h-3 w-3" />}>
                Graph
            </ToggleBtn>
            <ToggleBtn active={showTimeline} onClick={() => toggle("timeline")} position="last" icon={<ListChecks className="h-3 w-3" />}>
                Timeline
            </ToggleBtn>
        </div>
    );
}

function ToggleBtn({
    active,
    onClick,
    children,
    position,
    icon,
}: {
    active: boolean;
    onClick: () => void;
    children: React.ReactNode;
    position: "first" | "last";
    icon: React.ReactNode;
}) {
    return (
        <button
            type="button"
            onClick={onClick}
            className={cn(
                "inline-flex items-center gap-1.5 border px-2.5 py-1 text-[12px] font-medium transition-colors",
                active
                    ? "z-10 border-brand bg-brand-soft text-brand"
                    : "border-border bg-card text-muted-foreground hover:text-foreground",
                position === "first" ? "rounded-l-md" : "-ml-px rounded-r-md"
            )}
        >
            {icon}
            {children}
        </button>
    );
}

function LogsPanel({
    tab,
    setTab,
    workflowId,
    stepId,
    setStepId,
    workflowStatus,
    products,
    productsLoading,
    selectedProductId,
    setSelectedProductId,
}: {
    tab: "logs" | "products";
    setTab: (t: "logs" | "products") => void;
    workflowId: string;
    stepId: number | null;
    setStepId: (id: number | null) => void;
    workflowStatus: string;
    products: ProductRecord[];
    productsLoading: boolean;
    selectedProductId: string | null;
    setSelectedProductId: (id: string | null) => void;
}) {
    const [showSystem, setShowSystem] = useState(false);
    const [filterText, setFilterText] = useState("");
    const [logsRefresh, setLogsRefresh] = useState<{ refetch: () => Promise<unknown> } | null>(null);
    const [spinning, setSpinning] = useState(false);

    const handleRefresh = useCallback(() => {
        if (!logsRefresh) return;
        setSpinning(true);
        logsRefresh.refetch().finally(() => {
            setTimeout(() => setSpinning(false), 400);
        });
    }, [logsRefresh]);

    return (
        <div className="overflow-hidden rounded-lg border bg-card">
            <div className="flex min-h-11 items-center border-b px-4">
                <TabButton
                    active={tab === "logs"}
                    onClick={() => setTab("logs")}
                    icon={<FileText className="h-3.5 w-3.5" />}
                >
                    Logs
                </TabButton>
                <TabButton
                    active={tab === "products"}
                    onClick={() => setTab("products")}
                    icon={<Package className="h-3.5 w-3.5" />}
                >
                    Products
                </TabButton>
                <div className="flex-1" />
                {tab === "logs" && (
                    <div className="flex items-center gap-3 py-2 text-[12px] text-muted-foreground">
                        <div className="relative flex items-center">
                            <Search className="pointer-events-none absolute left-2.5 h-3 w-3 text-muted-foreground" />
                            <Input
                                placeholder="Filter logs…"
                                value={filterText}
                                onChange={(e) => setFilterText(e.target.value)}
                                className="h-7 w-48 pl-7 text-[12px]"
                            />
                        </div>
                        <label className="flex items-center gap-2">
                            System logs
                            <Switch checked={showSystem} onCheckedChange={setShowSystem} />
                        </label>
                        <Button
                            variant="ghost"
                            size="sm"
                            aria-label="Refresh logs"
                            className="h-7 w-7 p-0 text-muted-foreground"
                            onClick={handleRefresh}
                            disabled={spinning || !logsRefresh}
                        >
                            <RefreshCw className={cn("h-3.5 w-3.5", spinning && "animate-spin")} />
                        </Button>
                    </div>
                )}
            </div>

            {tab === "logs" && (
                <WorkflowLogs
                    key={`${workflowId}|${stepId ?? "all"}|${showSystem ? "sys" : "nosys"}`}
                    workflowId={workflowId}
                    stepId={stepId}
                    setStepId={setStepId}
                    status={workflowStatus}
                    showSystem={showSystem}
                    filterText={filterText}
                    setFilterText={setFilterText}
                    onQueryReady={setLogsRefresh}
                />
            )}
            {tab === "products" && (
                <WorkflowProducts
                    products={products}
                    loading={productsLoading}
                    selectedProductId={selectedProductId}
                    setSelectedProductId={setSelectedProductId}
                />
            )}
        </div>
    );
}

function TabButton({
    active,
    onClick,
    icon,
    children,
}: {
    active: boolean;
    onClick: () => void;
    icon: React.ReactNode;
    children: React.ReactNode;
}) {
    return (
        <button
            type="button"
            onClick={onClick}
            className={cn(
                "-mb-px flex items-center gap-1.5 border-b-2 px-3 py-2.5 text-[12.5px] font-medium transition-colors",
                active
                    ? "border-brand text-foreground"
                    : "border-transparent text-muted-foreground hover:text-foreground"
            )}
        >
            {icon}
            {children}
        </button>
    );
}

function WorkflowLogs({
    workflowId,
    stepId,
    setStepId,
    status,
    showSystem,
    filterText,
    setFilterText,
    onQueryReady,
}: {
    workflowId: string;
    stepId: number | null;
    setStepId: (id: number | null) => void;
    status: string;
    showSystem: boolean;
    filterText: string;
    setFilterText: (t: string) => void;
    onQueryReady?: (handle: { refetch: () => Promise<unknown> }) => void;
}) {
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

    const logsQuery = useInfiniteQuery({
        queryKey: ["workflow-logs", workflowId, stepId, showSystem],
        queryFn: async ({ pageParam }: { pageParam: string | undefined }): Promise<LogRecord[]> => {
            const pageFilter = pageParam
                ? `${filter} && created < "${pageParam}"`
                : filter;
            const result = await pbClient.logs.getList(1, LOG_PAGE_SIZE, {
                filter: pageFilter,
                sort: "-created",
            });
            return result.items.map((item) => ({
                id: item.id,
                level: Number(item.level),
                message: item.message,
                created: item.created,
                data: (item.data ?? {}) as Record<string, unknown>,
            }));
        },
        initialPageParam: undefined,
        getNextPageParam: (lastPage): string | undefined =>
            lastPage.length === LOG_PAGE_SIZE ? lastPage[lastPage.length - 1].created : undefined,
        // Pause polling once the user has loaded older pages — otherwise every tick
        // refetches every loaded page sequentially (TanStack v5 refetches all pages).
        refetchInterval: (query) => {
            if (TERMINAL_STATUSES.has(status)) return false;
            if ((query.state.data?.pages.length ?? 0) > 1) return false;
            return 2000;
        },
    });

    const rawLogs = useMemo(() => {
        const pages = logsQuery.data?.pages ?? [];
        return pages.flatMap((p) => p).slice().reverse();
    }, [logsQuery.data]);
    const logs = useMemo(() => {
        if (!filterText.trim()) return rawLogs;
        const needle = filterText.toLowerCase();
        return rawLogs.filter((l) => {
            if (l.message.toLowerCase().includes(needle)) return true;
            return Object.entries(l.data).some(([k, v]) =>
                `${k}=${String(v)}`.toLowerCase().includes(needle)
            );
        });
    }, [rawLogs, filterText]);
    const {
        isLoading: logsLoading,
        hasNextPage,
        isFetchingNextPage,
        fetchNextPage,
        refetch,
    } = logsQuery;
    useEffect(() => {
        onQueryReady?.({ refetch });
    }, [onQueryReady, refetch]);

    const scrollRef = useRef<HTMLDivElement>(null);
    const anchorLenRef = useRef(0);
    const [follow, setFollow] = useState(true);

    const loadOlder = useCallback(() => {
        const el = scrollRef.current;
        const prevHeight = el?.scrollHeight ?? 0;
        const prevTop = el?.scrollTop ?? 0;
        void fetchNextPage().then(() => {
            // Wait two frames so React has committed prepended rows before reading scrollHeight.
            requestAnimationFrame(() => {
                requestAnimationFrame(() => {
                    const next = scrollRef.current;
                    if (!next) return;
                    const delta = next.scrollHeight - prevHeight;
                    next.scrollTop = prevTop + delta;
                });
            });
        });
    }, [fetchNextPage]);
    const newCount = follow ? 0 : Math.max(0, logs.length - anchorLenRef.current);

    useEffect(() => {
        if (!follow) return;
        const el = scrollRef.current;
        if (el) el.scrollTop = el.scrollHeight;
    }, [logs.length, follow]);

    const handleScroll = () => {
        const el = scrollRef.current;
        if (!el) return;
        const atBottom = el.scrollHeight - el.clientHeight - el.scrollTop < 16;
        if (atBottom && !follow) {
            setFollow(true);
        } else if (!atBottom && follow) {
            anchorLenRef.current = logs.length;
            setFollow(false);
        }
    };

    const jumpToLatest = () => {
        const el = scrollRef.current;
        if (!el) return;
        el.scrollTo({ top: el.scrollHeight });
        setFollow(true);
    };

    if (logsLoading) {
        return <div className="p-4 text-[12.5px] text-muted-foreground">Loading logs…</div>;
    }

    if (logs.length === 0) {
        const hasFilter = filterText.trim().length > 0;
        if (hasFilter && rawLogs.length > 0) {
            return (
                <div className="flex items-center gap-2 p-4 text-[12.5px] text-muted-foreground">
                    <span>
                        No matches for <span className="font-mono text-foreground">{filterText}</span>.
                    </span>
                    <Button
                        variant="ghost"
                        size="sm"
                        className="h-6 px-2 text-[12px]"
                        onClick={() => setFilterText("")}
                    >
                        Clear filter
                    </Button>
                </div>
            );
        }
        if (stepId != null) {
            return (
                <div className="flex items-center gap-2 p-4 text-[12.5px] text-muted-foreground">
                    <span>Step {stepId} produced no logs.</span>
                    <Button
                        variant="ghost"
                        size="sm"
                        className="h-6 px-2 text-[12px]"
                        onClick={() => setStepId(null)}
                    >
                        Show all workflow logs
                    </Button>
                </div>
            );
        }
        if (!showSystem) {
            return (
                <div className="p-4 text-[12.5px] text-muted-foreground">
                    No app logs yet. System logs are hidden.
                </div>
            );
        }
        return <div className="p-4 text-[12.5px] text-muted-foreground">No logs yet.</div>;
    }

    const showStepCol = stepId == null;
    const gridCols = showStepCol
        ? "grid-cols-[95px_56px_1fr_90px]"
        : "grid-cols-[95px_56px_1fr]";

    return (
        <div className="relative">
            <div
                ref={scrollRef}
                onScroll={handleScroll}
                className="max-h-[clamp(240px,50vh,640px)] overflow-auto"
            >
                <div className={cn(TABLE_HEADER_CLASS, gridCols)}>
                    <div>Time</div>
                    <div>Level</div>
                    <div>Message</div>
                    {showStepCol && <div className="text-right">Step</div>}
                </div>
                {hasNextPage && (
                    <div className="flex items-center justify-center border-b border-border-soft bg-muted/30 px-4 py-2">
                        <Button
                            variant="outline"
                            size="xs"
                            onClick={loadOlder}
                            disabled={isFetchingNextPage}
                            className="gap-1.5 font-mono text-[11px] uppercase tracking-[0.06em]"
                        >
                            <ChevronUp className="h-3 w-3" />
                            {isFetchingNextPage ? "Loading older…" : "Load older logs"}
                        </Button>
                    </div>
                )}
                {logs.map((log, i) => {
                    const lvl = logLevel(log.level);
                    const isSystem = log.data.source === "system";
                    const isAppStatus = log.message === "app status changed";
                    const attrs = Object.entries(log.data).filter(
                        ([k]) => !(isAppStatus ? APP_STATUS_HIDDEN_KEYS : HIDDEN_DATA_KEYS).has(k)
                    );
                    const prev = logs[i - 1];
                    const logDay = log.created.slice(0, 10);
                    const dayChanged = !!prev && prev.created.slice(0, 10) !== logDay;
                    return (
                        <Fragment key={log.id}>
                            {dayChanged && (
                                <div className="border-t border-border-soft bg-background px-4 py-1 text-[10.5px] font-medium uppercase tracking-[0.06em] text-muted-foreground">
                                    {new Date(log.created).toUTCString().slice(0, 16)} UTC
                                </div>
                            )}
                            <div
                                className={cn(
                                    TABLE_ROW_BASE_CLASS,
                                    "items-start transition-colors hover:bg-muted/50",
                                    gridCols,
                                    LEVEL_RAIL[log.level] ?? "border-l-transparent",
                                    isSystem && !isAppStatus && "opacity-60"
                                )}
                            >
                                <div
                                    className="font-mono text-[11.5px] text-muted-foreground"
                                    title={formatTimestampPrecise(log.created)}
                                >
                                    {new Date(log.created).toISOString().slice(11, 23)}
                                </div>
                                <div>
                                    <span
                                        className={cn(
                                            "rounded-sm px-1.5 py-0.5 font-mono text-[10px] font-semibold",
                                            lvl.className
                                        )}
                                    >
                                        {lvl.label}
                                    </span>
                                </div>
                                <div>
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
                                                    <Cog className="h-3 w-3 shrink-0 text-muted-foreground" />
                                                )}
                                                <span>{log.message}</span>
                                            </span>
                                            {attrs.length > 0 && (
                                                <span className="mt-1 flex flex-wrap gap-1">
                                                    {attrs.map(([k, v]) => (
                                                        <span
                                                            key={k}
                                                            className="rounded bg-secondary px-1.5 py-0.5 font-mono text-[10.5px]"
                                                        >
                                                            <span className="text-muted-foreground">{k}=</span>
                                                            <span className="text-foreground">{String(v)}</span>
                                                        </span>
                                                    ))}
                                                </span>
                                            )}
                                        </>
                                    )}
                                </div>
                                {showStepCol && (
                                    <div className="text-right">
                                        {log.data.step_id != null ? (
                                            <button
                                                type="button"
                                                onClick={() => setStepId(Number(log.data.step_id))}
                                                className="rounded bg-secondary px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground transition-colors hover:text-foreground"
                                            >
                                                {String(log.data.step_id)}
                                            </button>
                                        ) : (
                                            <span className="font-mono text-[11px] text-muted-foreground">—</span>
                                        )}
                                    </div>
                                )}
                            </div>
                        </Fragment>
                    );
                })}
            </div>
            {newCount > 0 && (
                <Button
                    type="button"
                    size="sm"
                    onClick={jumpToLatest}
                    className="absolute bottom-3 right-3 h-auto rounded-full px-3 py-1 text-[11.5px] shadow-md"
                >
                    {newCount} new ↓
                </Button>
            )}
        </div>
    );
}

function WorkflowProducts({
    products,
    loading,
    selectedProductId,
    setSelectedProductId,
}: {
    products: ProductRecord[];
    loading: boolean;
    selectedProductId: string | null;
    setSelectedProductId: (id: string | null) => void;
}) {
    if (loading) return <div className="p-4 text-[12.5px] text-muted-foreground">Loading products…</div>;
    if (products.length === 0)
        return (
            <div className="p-4 text-[12.5px] text-muted-foreground">
                This workflow hasn't produced any files yet.
            </div>
        );

    const gridCols =
        "grid-cols-[95px_minmax(180px,2fr)_70px_minmax(140px,1fr)_90px_minmax(120px,1.5fr)]";

    return (
        <div className="max-h-[clamp(240px,50vh,640px)] overflow-auto">
            <div className={cn(TABLE_HEADER_CLASS, gridCols)}>
                <div>Time</div>
                <div>File</div>
                <div className="text-right">Size</div>
                <div>Step</div>
                <div>Status</div>
                <div>Error</div>
            </div>
            {products.map((p) => {
                const hasError = !!p.error;
                const isSelected = p.id === selectedProductId;
                return (
                    <button
                        type="button"
                        key={p.id}
                        onClick={() => setSelectedProductId(isSelected ? null : p.id)}
                        className={cn(
                            TABLE_ROW_BASE_CLASS,
                            "w-full items-center text-left transition-colors hover:bg-muted/30 focus:outline-none focus:bg-muted/30",
                            gridCols,
                            hasError ? "border-l-danger" : "border-l-transparent",
                            isSelected && "bg-muted/40"
                        )}
                    >
                        <div
                            className="font-mono text-[11.5px] text-muted-foreground"
                            title={formatTimestampPrecise(p.created)}
                        >
                            {new Date(p.created).toISOString().slice(11, 23)}
                        </div>
                        <div className="truncate font-mono text-[12px] text-foreground" title={p.file_name}>
                            {p.file_name}
                        </div>
                        <div className="text-right font-mono text-[11.5px] text-muted-foreground">
                            {formatBytes(p.size)}
                        </div>
                        <div className="truncate font-mono text-[11.5px] text-muted-foreground">
                            {p.function_name || `Step ${p.function_id}`}
                        </div>
                        <div>
                            <ProductStatusBadge status={p.status} />
                        </div>
                        <div
                            className={cn(
                                "truncate font-mono text-[11.5px]",
                                hasError ? "text-danger-foreground" : "text-muted-foreground"
                            )}
                            title={p.error}
                        >
                            {p.error || "—"}
                        </div>
                    </button>
                );
            })}
        </div>
    );
}

function InspectorShell({
    label,
    title,
    pills,
    srLabel,
    onClose,
    children,
}: {
    label: string;
    title: React.ReactNode;
    pills: React.ReactNode;
    srLabel: string;
    onClose: () => void;
    children: React.ReactNode;
}) {
    const isLg = useMediaQuery("(min-width: 1024px)");
    return (
        <DrawerShell srLabel={srLabel} onClose={onClose}>
            <div className="min-h-[7rem] border-b border-border-soft bg-card px-6 pb-4 pt-5">
                <div className="mb-1.5 flex items-center justify-between">
                    <span className="text-[10.5px] font-medium uppercase tracking-[0.06em] text-muted-foreground">
                        {label}
                    </span>
                    {isLg && (
                        <button
                            type="button"
                            onClick={onClose}
                            aria-label={`Close ${srLabel}`}
                            className="text-muted-foreground transition-colors hover:text-foreground"
                        >
                            <X className="h-4 w-4" />
                        </button>
                    )}
                </div>
                <div className="mb-1.5">{title}</div>
                <div className="flex flex-wrap items-center gap-x-3 gap-y-1">{pills}</div>
            </div>
            <div className="flex-1 overflow-auto px-5 py-3">{children}</div>
        </DrawerShell>
    );
}

function InspectorError({ children }: { children: React.ReactNode }) {
    return (
        <pre className="max-h-64 overflow-auto rounded-md border border-danger/40 bg-danger-soft p-2.5 font-mono text-[11.5px] leading-relaxed text-danger-foreground">
            {children}
        </pre>
    );
}

function ProductInspector({ product, onClose }: { product: ProductRecord; onClose: () => void }) {
    const downloadUrl = product.file ? pbClient.files.getUrl(product, product.file) : null;
    return (
        <InspectorShell
            label="Product"
            srLabel="Product details"
            onClose={onClose}
            title={
                <div className="break-all font-mono text-[14px] font-semibold text-foreground">
                    {product.file_name}
                </div>
            }
            pills={
                <>
                    <ProductStatusBadge status={product.status} />
                    <Pill tone="neutral">{formatBytes(product.size)}</Pill>
                </>
            }
        >
            {downloadUrl && (
                <Button asChild size="sm" className="mb-4 w-full">
                    <a href={downloadUrl} download={product.file_name}>
                        <Download className="mr-1.5 h-3.5 w-3.5" />
                        Download
                    </a>
                </Button>
            )}

            <InspectorRow label="Created" value={formatTimestampPrecise(product.created)} mono />
            <InspectorRow label="Updated" value={formatTimestampPrecise(product.updated)} mono />
            <InspectorRow
                label="Step"
                value={product.function_name || `Step ${product.function_id}`}
                mono
            />
            <InspectorRow label="Function ID" value={String(product.function_id)} mono />
            <InspectorRow label="Size" value={formatBytes(product.size)} mono />

            {product.error && (
                <>
                    <SectionLabel>Error</SectionLabel>
                    <InspectorError>{product.error}</InspectorError>
                </>
            )}
        </InspectorShell>
    );
}

function StepInspector({ step, onClose }: { step: StepNodeRecord; onClose: () => void }) {
    const duration = step.endedAtMs && step.startedAtMs ? step.endedAtMs - step.startedAtMs : null;

    const formattedOutput = useMemo(() => {
        if (!step.output) return null;
        try {
            return JSON.stringify(JSON.parse(step.output), null, 2);
        } catch {
            return step.output;
        }
    }, [step.output]);

    return (
        <InspectorShell
            label="Activity"
            srLabel="Step details"
            onClose={onClose}
            title={
                <div className="font-mono text-[15px] font-semibold text-foreground">{step.name}</div>
            }
            pills={
                <>
                    <StepStatusBadge status={step.status} />
                    {duration != null && <Pill tone="neutral">{formatDuration(duration)}</Pill>}
                </>
            }
        >
            <InspectorRow
                label="Started"
                value={step.startedAtMs ? formatTimestampPrecise(new Date(step.startedAtMs).toISOString()) : "—"}
                mono
            />
            <InspectorRow
                label="Finished"
                value={step.endedAtMs ? formatTimestampPrecise(new Date(step.endedAtMs).toISOString()) : "—"}
                mono
            />
            {step.childWorkflowId && (
                <InspectorRow label="Child workflow" value={step.childWorkflowId} mono />
            )}

            {step.error && (
                <>
                    <SectionLabel>Error</SectionLabel>
                    <InspectorError>{step.error}</InspectorError>
                </>
            )}

            {formattedOutput ? (
                <>
                    <SectionLabel>Result</SectionLabel>
                    <pre className="max-h-[50vh] overflow-auto rounded-md border bg-background p-2.5 font-mono text-[11.5px] leading-relaxed text-foreground">
                        {formattedOutput}
                    </pre>
                </>
            ) : (
                !step.error && (
                    <>
                        <SectionLabel>Result</SectionLabel>
                        <div className="text-[12px] text-muted-foreground">
                            {step.status === "running" ? "Step is still running…" : "No output."}
                        </div>
                    </>
                )
            )}
        </InspectorShell>
    );
}

function InspectorRow({ label, value, mono }: { label: string; value: React.ReactNode; mono?: boolean }) {
    return (
        <div className="flex items-center justify-between border-b border-dashed border-border-soft py-1.5 text-[12px]">
            <span className="text-muted-foreground">{label}</span>
            <span
                className={cn(
                    "truncate pl-2 font-medium text-foreground",
                    mono && "font-mono text-[11.5px]"
                )}
            >
                {value}
            </span>
        </div>
    );
}

function SectionLabel({ children }: { children: React.ReactNode }) {
    return (
        <div className="mb-1.5 mt-4 text-[10.5px] font-medium uppercase tracking-[0.06em] text-muted-foreground">
            {children}
        </div>
    );
}

export function WorkflowSteps() {
    const { id } = useParams();
    if (!id) return null;
    return (
        <ReactFlowProvider>
            <StepFlowContent workflowId={id} />
        </ReactFlowProvider>
    );
}
