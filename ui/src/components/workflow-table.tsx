import {useState} from "react";
import type {CrudFilters, CrudSort} from "@refinedev/core";
import {useList} from "@refinedev/core";
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow,} from "@/components/ui/table";
import {Button} from "@/components/ui/button";
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue,} from "@/components/ui/select";
import {AppStatusBadge, StatusBadge} from "@/components/status-badge";
import {TableSkeleton} from "@/components/table-skeleton";
import {formatDuration, formatTimestamp} from "@/lib/format";
import {cn} from "@/lib/utils";
import type {PtWorkflowStatusResponse} from "@/types/pocketbase-types";

export type WorkflowRecord = PtWorkflowStatusResponse;

interface Props {
    filters: CrudFilters;
    sorters: CrudSort[];
    onRowClick: (record: WorkflowRecord) => void;
    selectedId?: string | null;
}

const tableColumns = [
    {key: "created_at_epoch_ms", header: "Created At"},
    {key: "name", header: "Workflow Name"},
    {key: "summary", header: "Summary"},
    {key: "status", header: "Status"},
    {key: "duration", header: "Duration"},
    {key: "app_status", header: "App Status"},
];

export function WorkflowTable({filters, sorters, onRowClick, selectedId}: Props) {
    const [currentPage, setCurrentPage] = useState(1);
    const [pageSize, setPageSize] = useState(10);

    const {result, query: listQuery} = useList<WorkflowRecord>({
        resource: "pt_workflow_status",
        filters,
        sorters,
        pagination: {currentPage, pageSize},
        liveMode: "auto",
        queryOptions: {refetchInterval: 5000},
    });

    const rows = result.data ?? [];
    const total = result.total ?? 0;
    const pageCount = Math.max(1, Math.ceil(total / pageSize));

    if (listQuery.isLoading) {
        return <TableSkeleton columns={tableColumns.length} headers={tableColumns.map((c) => c.header)}/>;
    }

    return (
        <div>
            <div className="rounded-md border">
                <Table>
                    <TableHeader>
                        <TableRow>
                            {tableColumns.map((col) => (
                                <TableHead key={col.key}>{col.header}</TableHead>
                            ))}
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {rows.length ? (
                            rows.map((row) => (
                                <TableRow
                                    key={row.id}
                                    role="button"
                                    tabIndex={0}
                                    aria-pressed={selectedId === row.id}
                                    aria-label={`Open workflow ${row.name}`}
                                    data-state={selectedId === row.id ? "selected" : undefined}
                                    className={cn(
                                        "cursor-pointer focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring",
                                        selectedId === row.id && "bg-accent/60 hover:bg-accent/60"
                                    )}
                                    onClick={() => onRowClick(row)}
                                    onKeyDown={(e) => {
                                        if (e.key === "Enter" || e.key === " ") {
                                            e.preventDefault();
                                            onRowClick(row);
                                        }
                                    }}
                                >
                                    <TableCell>
                                        {row.created_at_epoch_ms
                                            ? formatTimestamp(row.created_at_epoch_ms)
                                            : "\u2014"}
                                    </TableCell>
                                    <TableCell>{row.name}</TableCell>
                                    <TableCell>
                    <span className="block font-mono text-xs text-muted-foreground max-w-[300px] truncate">
                      {row.summary || "\u2014"}
                    </span>
                                    </TableCell>
                                    <TableCell>
                                        <StatusBadge status={row.status}/>
                                    </TableCell>
                                    <TableCell className="font-mono text-xs text-muted-foreground">
                                        {row.created_at_epoch_ms && row.updated_at_epoch_ms && row.updated_at_epoch_ms > row.created_at_epoch_ms
                                            ? formatDuration(row.updated_at_epoch_ms - row.created_at_epoch_ms)
                                            : "\u2014"}
                                    </TableCell>
                                    <TableCell>
                                        <AppStatusBadge
                                            label={row.app_status}
                                            color={row.app_status_color}
                                        />
                                    </TableCell>
                                </TableRow>
                            ))
                        ) : (
                            <TableRow>
                                <TableCell
                                    colSpan={tableColumns.length}
                                    className="h-32 text-center"
                                >
                                    <div className="flex flex-col items-center gap-1.5 text-sm">
                                        <span className="font-medium text-foreground">No workflows in this range</span>
                                        <span className="text-muted-foreground">
                                            Trigger a run or widen the time range.
                                        </span>
                                    </div>
                                </TableCell>
                            </TableRow>
                        )}
                    </TableBody>
                </Table>
            </div>
            <div className="flex items-center justify-between py-4">
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    <span>Rows per page</span>
                    <Select
                        value={String(pageSize)}
                        onValueChange={(value) => {
                            setPageSize(Number(value));
                            setCurrentPage(1);
                        }}
                    >
                        <SelectTrigger className="h-8 w-[70px]">
                            <SelectValue/>
                        </SelectTrigger>
                        <SelectContent>
                            {[10, 25, 50, 100].map((size) => (
                                <SelectItem key={size} value={String(size)}>
                                    {size}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </div>
                <div className="flex items-center gap-4">
          <span className="text-sm text-muted-foreground">
            Page {currentPage} of {pageCount}
          </span>
                    <div className="flex items-center gap-2">
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={() => setCurrentPage(currentPage - 1)}
                            disabled={currentPage <= 1}
                        >
                            Previous
                        </Button>
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={() => setCurrentPage(currentPage + 1)}
                            disabled={currentPage >= pageCount}
                        >
                            Next
                        </Button>
                    </div>
                </div>
            </div>
        </div>
    );
}
