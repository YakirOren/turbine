import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { pbClient } from "@/providers/pocketbase";
import {
  type ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  ChartLegend,
  ChartLegendContent,
} from "@/components/ui/chart";
import { Skeleton } from "@/components/ui/skeleton";

interface BucketStat {
  time: number;
  success: number;
  error: number;
  cancelled: number;
}

interface CalendarViewProps {
  timeRange?: string;
  name?: string;
  status?: string;
  tag?: string;
  onDayClick?: (day: string) => void;
}

function getRangeParams(timeRange: string): { fromMs: number; toMs: number; bucketMins: number } {
  const now = Date.now();
  switch (timeRange) {
    case "1h":
      return { fromMs: now - 3600_000, toMs: now, bucketMins: 5 };
    case "6h":
      return { fromMs: now - 21600_000, toMs: now, bucketMins: 15 };
    case "24h":
      return { fromMs: now - 86400_000, toMs: now, bucketMins: 60 };
    case "7d":
      return { fromMs: now - 604800_000, toMs: now, bucketMins: 1440 };
    default: {
      // "all" — last 6 months
      const to = new Date();
      const from = new Date(to);
      from.setMonth(from.getMonth() - 5);
      from.setDate(1);
      return { fromMs: from.getTime(), toMs: to.getTime(), bucketMins: 1440 };
    }
  }
}

function formatTick(timeRange: string): (v: number) => string {
  switch (timeRange) {
    case "1h":
    case "6h":
      return (v: number) => new Date(v).toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
    case "24h":
      return (v: number) => new Date(v).toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
    case "7d":
      return (v: number) => new Date(v).toLocaleDateString(undefined, { weekday: "short" });
    default:
      return (v: number) => new Date(v).toLocaleDateString(undefined, { month: "short", day: "numeric" });
  }
}

function formatTooltipLabel(timeRange: string): (v: number) => string {
  switch (timeRange) {
    case "1h":
    case "6h":
    case "24h":
      return (v: number) => new Date(v).toLocaleString(undefined, {
        month: "short", day: "numeric", hour: "2-digit", minute: "2-digit",
      });
    default:
      return (v: number) => new Date(v).toLocaleDateString(undefined, {
        weekday: "short", month: "short", day: "numeric",
      });
  }
}

const chartConfig = {
  success: { label: "Success", color: "var(--color-green-500)" },
  error: { label: "Error", color: "var(--color-red-500)" },
  cancelled: { label: "Cancelled", color: "var(--color-gray-500)" },
} satisfies ChartConfig;

export function CalendarView({ timeRange = "all", name, status, tag, onDayClick }: CalendarViewProps) {
  const { fromMs, toMs, bucketMins } = useMemo(() => getRangeParams(timeRange), [timeRange]);

  const queryParams = useMemo(() => {
    const p = new URLSearchParams({
      from_ms: String(fromMs),
      to_ms: String(toMs),
      bucket_mins: String(bucketMins),
    });
    if (name) p.set("name", name);
    if (status && status !== "all") p.set("status", status);
    if (tag) p.set("tag", tag);
    return p.toString();
  }, [fromMs, toMs, bucketMins, name, status, tag]);

  const calendarQuery = useQuery<BucketStat[]>({
    queryKey: ["activity", queryParams],
    queryFn: () =>
      pbClient
        .send<BucketStat[]>(`/api/pt/calendar?${queryParams}`, { method: "GET" }),
  });

  const stats: BucketStat[] = calendarQuery.data ?? [];

  const hasSuccess = stats.some((s) => s.success > 0);
  const hasError = stats.some((s) => s.error > 0);
  const hasCancelled = stats.some((s) => s.cancelled > 0);

  const tickFormatter = useMemo(() => formatTick(timeRange), [timeRange]);
  const tooltipFormatter = useMemo(() => formatTooltipLabel(timeRange), [timeRange]);

  if (calendarQuery.isLoading && stats.length === 0) {
    return <Skeleton className="h-[120px] w-full" />;
  }

  if (calendarQuery.isError) {
    return (
      <div className="rounded-md border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive">
        {(calendarQuery.error as unknown as Error)?.message ?? "Failed to load activity data"}
      </div>
    );
  }

  if (stats.length === 0) {
    return (
      <div className="flex h-[120px] items-center justify-center text-sm text-muted-foreground">
        No activity in this period.
      </div>
    );
  }

  return (
    <ChartContainer config={chartConfig} className="h-[120px] w-full">
      <AreaChart
        data={stats}
        margin={{ top: 4, right: 4, bottom: 0, left: 0 }}
        onClick={(state) => {
          if (state?.activePayload?.[0]?.payload && onDayClick) {
            const t = state.activePayload[0].payload.time as number;
            const d = new Date(t);
            const day = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
            onDayClick(day);
          }
        }}
        className="cursor-pointer"
      >
        <CartesianGrid vertical={false} strokeDasharray="3 3" />
        <XAxis
          dataKey="time"
          type="number"
          domain={[fromMs, toMs]}
          tickLine={false}
          axisLine={false}
          tickMargin={4}
          tick={{ fontSize: 10 }}
          tickFormatter={tickFormatter}
          minTickGap={40}
        />
        <YAxis
          tickLine={false}
          axisLine={false}
          tick={{ fontSize: 10 }}
          width={28}
          allowDecimals={false}
        />
        <ChartTooltip
          content={
            <ChartTooltipContent
              labelFormatter={(_, payload) => {
                const t = payload?.[0]?.payload?.time as number | undefined;
                return t ? tooltipFormatter(t) : "";
              }}
            />
          }
        />
        <ChartLegend content={<ChartLegendContent />} />
        {hasSuccess && (
          <Area
            dataKey="success"
            type="monotone"
            fill="var(--color-success)"
            stroke="var(--color-success)"
            fillOpacity={0.3}
            stackId="a"
          />
        )}
        {hasError && (
          <Area
            dataKey="error"
            type="monotone"
            fill="var(--color-error)"
            stroke="var(--color-error)"
            fillOpacity={0.3}
            stackId="a"
          />
        )}
        {hasCancelled && (
          <Area
            dataKey="cancelled"
            type="monotone"
            fill="var(--color-cancelled)"
            stroke="var(--color-cancelled)"
            fillOpacity={0.3}
            stackId="a"
          />
        )}
      </AreaChart>
    </ChartContainer>
  );
}
