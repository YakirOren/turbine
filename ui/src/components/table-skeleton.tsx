import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

interface Props {
  columns: number;
  rows?: number;
  headers?: string[];
}

export function TableSkeleton({ columns, rows = 5, headers }: Props) {
  return (
    <div className="rounded-md border">
      <Table>
        {headers && (
          <TableHeader>
            <TableRow>
              {headers.map((h, i) => (
                <TableHead key={i}>{h}</TableHead>
              ))}
            </TableRow>
          </TableHeader>
        )}
        <TableBody>
          {Array.from({ length: rows }, (_, i) => (
            <TableRow key={i}>
              {Array.from({ length: columns }, (_, j) => (
                <TableCell key={j}>
                  <Skeleton className="h-4 w-full" />
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
