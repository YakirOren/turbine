import { type ColumnDef, flexRender, getCoreRowModel } from "@tanstack/react-table";
import { useTable } from "@refinedev/react-table";
import type { CrudFilters, CrudSort } from "@refinedev/core";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { StatusBadge, AppStatusBadge } from "@/components/status-badge";

export interface WorkflowRecord {
  id: string;
  status: string;
  name: string;
  executor_id: string;
  application_version: string;
  created_at_epoch_ms: number;
  updated_at_epoch_ms: number;
  queue_name: string;
  priority: number;
  app_status: string;
  app_status_color: string;
}

interface Props {
  filters: CrudFilters;
  sorters: CrudSort[];
  onRowClick: (record: WorkflowRecord) => void;
}

export function WorkflowTable({ filters, sorters, onRowClick }: Props) {
  const columns: ColumnDef<WorkflowRecord>[] = [
    {
      accessorKey: "created_at_epoch_ms",
      header: "Created At",
      cell: ({ getValue }) => {
        const ms = getValue<number>();
        return ms ? new Date(ms).toLocaleString() : "\u2014";
      },
    },
    {
      accessorKey: "name",
      header: "Workflow Name",
    },
    {
      accessorKey: "status",
      header: "Status",
      cell: ({ getValue }) => <StatusBadge status={getValue<string>()} />,
    },
    {
      id: "app_status",
      header: "App Status",
      cell: ({ row }) => (
        <AppStatusBadge
          label={row.original.app_status}
          color={row.original.app_status_color}
        />
      ),
    },
  ];

  const { reactTable: table } = useTable<WorkflowRecord>({
    columns,
    refineCoreProps: {
      resource: "pt_workflow_status",
      filters: { permanent: filters },
      sorters: { permanent: sorters },
      pagination: { pageSize: 25 },
      liveMode: "auto",
      queryOptions: { refetchInterval: 5000 },
    },
    getCoreRowModel: getCoreRowModel(),
  });

  return (
    <div>
      <div className="rounded-md border">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((hg) => (
              <TableRow key={hg.id}>
                {hg.headers.map((h) => (
                  <TableHead key={h.id}>
                    {h.isPlaceholder
                      ? null
                      : flexRender(h.column.columnDef.header, h.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow
                  key={row.id}
                  className="cursor-pointer"
                  onClick={() => onRowClick(row.original)}
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>
                      {flexRender(
                        cell.column.columnDef.cell,
                        cell.getContext()
                      )}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell
                  colSpan={columns.length}
                  className="h-24 text-center"
                >
                  No workflows found.
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
      <div className="flex items-center justify-end gap-2 py-4">
        <Button
          variant="outline"
          size="sm"
          onClick={() => table.previousPage()}
          disabled={!table.getCanPreviousPage()}
        >
          Previous
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={() => table.nextPage()}
          disabled={!table.getCanNextPage()}
        >
          Next
        </Button>
      </div>
    </div>
  );
}
