import { useMemo } from "react";
import { useNavigate } from "react-router";
import { useCustom } from "@refinedev/core";
import { pbClient } from "@/providers/pocketbase";
import { cn } from "@/lib/utils";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

interface DayStat {
  date: string;
  total: number;
  success: number;
  error: number;
  cancelled: number;
}

function formatLocalDate(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

function getMonthRange(): { from: string; to: string } {
  const now = new Date();
  const to = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const from = new Date(to);
  from.setMonth(from.getMonth() - 5);
  from.setDate(1);
  return {
    from: formatLocalDate(from),
    to: formatLocalDate(to),
  };
}

function generateDayGrid(from: string, to: string): string[] {
  const days: string[] = [];
  const [fy, fm, fd] = from.split("-").map(Number);
  const [ty, tm, td] = to.split("-").map(Number);
  const start = new Date(fy, fm - 1, fd);
  const end = new Date(ty, tm - 1, td);
  for (let d = new Date(start); d <= end; d.setDate(d.getDate() + 1)) {
    days.push(formatLocalDate(d));
  }
  return days;
}

function intensityClass(count: number, max: number): string {
  if (count === 0) return "bg-muted";
  const ratio = count / max;
  if (ratio <= 0.25) return "bg-green-200 dark:bg-green-900";
  if (ratio <= 0.5) return "bg-green-400 dark:bg-green-700";
  if (ratio <= 0.75) return "bg-green-500 dark:bg-green-500";
  return "bg-green-600 dark:bg-green-400";
}

const WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

const WORKFLOW_FILTER_STORAGE_KEY = "pt_workflow_filters";

export function CalendarView() {
  const navigate = useNavigate();
  const { from, to } = useMemo(getMonthRange, []);

  const { query: calendarQuery } = useCustom<DayStat[]>({
    url: "",
    method: "get",
    queryOptions: {
      queryKey: ["calendar", from, to],
      queryFn: () =>
        pbClient
          .send<DayStat[]>(`/api/pt/calendar?from=${from}&to=${to}`, { method: "GET" })
          .then((data) => ({ data })),
    },
  });

  const stats: DayStat[] = (calendarQuery.data?.data as DayStat[] | undefined) ?? [];

  const statMap = useMemo(() => {
    const map = new Map<string, DayStat>();
    for (const s of stats) map.set(s.date, s);
    return map;
  }, [stats]);

  const days = useMemo(() => generateDayGrid(from, to), [from, to]);
  const maxTotal = useMemo(
    () => Math.max(1, ...stats.map((s) => s.total)),
    [stats]
  );

  const weeks = useMemo(() => {
    const result: string[][] = [];
    let currentWeek: string[] = [];
    const firstDayOfWeek = new Date(days[0]).getDay();
    for (let i = 0; i < firstDayOfWeek; i++) currentWeek.push("");
    for (const day of days) {
      const dow = new Date(day).getDay();
      if (dow === 0 && currentWeek.length > 0) {
        result.push(currentWeek);
        currentWeek = [];
      }
      currentWeek.push(day);
    }
    if (currentWeek.length > 0) result.push(currentWeek);
    return result;
  }, [days]);

  const monthLabels = useMemo(() => {
    const labels: { label: string; weekIndex: number }[] = [];
    let lastMonth = "";
    for (let wi = 0; wi < weeks.length; wi++) {
      const firstDay = weeks[wi].find((d) => d !== "");
      if (!firstDay) continue;
      const month = firstDay.slice(0, 7);
      if (month !== lastMonth) {
        labels.push({
          label: new Date(firstDay).toLocaleString(undefined, { month: "short" }),
          weekIndex: wi,
        });
        lastMonth = month;
      }
    }
    return labels;
  }, [weeks]);

  const handleDayClick = (day: string) => {
    // Store the exact day boundaries so WorkflowList can filter precisely
    const dayStart = new Date(day + "T00:00:00").getTime();
    const dayEnd = dayStart + 86400_000;
    localStorage.setItem(
      WORKFLOW_FILTER_STORAGE_KEY,
      JSON.stringify({
        timeRange: "custom",
        customFrom: dayStart,
        customTo: dayEnd,
        name: "",
        status: "all",
      })
    );
    navigate("/workflows");
  };

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">Calendar</h1>
      <p className="text-sm text-muted-foreground">
        Workflow execution activity over the last 6 months. Click a day to view its workflows.
      </p>

      {calendarQuery.isError && (
        <div className="rounded-md border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {(calendarQuery.error as unknown as Error)?.message ?? "Failed to load calendar data"}
        </div>
      )}

      {calendarQuery.isLoading && stats.length === 0 && (
        <div className="text-sm text-muted-foreground">Loading...</div>
      )}

      <TooltipProvider delayDuration={100}>
        <div className="overflow-x-auto">
          <div className="flex mb-1 ml-10">
            {monthLabels.map((m, i) => (
              <span
                key={i}
                className="text-xs text-muted-foreground"
                style={{
                  marginLeft: i === 0 ? m.weekIndex * 14 : undefined,
                  width:
                    i < monthLabels.length - 1
                      ? (monthLabels[i + 1].weekIndex - m.weekIndex) * 14
                      : undefined,
                }}
              >
                {m.label}
              </span>
            ))}
          </div>

          <div className="flex gap-0.5">
            <div className="flex flex-col gap-0.5 mr-1">
              {WEEKDAYS.map((d, i) => (
                <span
                  key={i}
                  className="flex h-[12px] items-center text-[10px] text-muted-foreground"
                >
                  {i % 2 === 1 ? d : ""}
                </span>
              ))}
            </div>

            {weeks.map((week, wi) => (
              <div key={wi} className="flex flex-col gap-0.5">
                {Array.from({ length: 7 }, (_, di) => {
                  const day = week[di] ?? "";
                  if (!day) {
                    return <div key={di} className="h-[12px] w-[12px]" />;
                  }
                  const stat = statMap.get(day);
                  const count = stat?.total ?? 0;

                  return (
                    <Tooltip key={di}>
                      <TooltipTrigger asChild>
                        <button
                          className={cn(
                            "h-[12px] w-[12px] rounded-[2px] transition-colors",
                            intensityClass(count, maxTotal),
                            count > 0
                              ? "cursor-pointer hover:ring-1 hover:ring-foreground"
                              : "cursor-default"
                          )}
                          onClick={() => {
                            if (count > 0) handleDayClick(day);
                          }}
                        />
                      </TooltipTrigger>
                      <TooltipContent side="top" className="text-xs">
                        <div className="font-medium">{day}</div>
                        {count > 0 ? (
                          <div className="text-muted-foreground">
                            {count} workflow{count !== 1 ? "s" : ""}
                            {stat!.error > 0 && (
                              <span className="text-red-400">
                                {" "}({stat!.error} failed)
                              </span>
                            )}
                          </div>
                        ) : (
                          <div className="text-muted-foreground">No workflows</div>
                        )}
                      </TooltipContent>
                    </Tooltip>
                  );
                })}
              </div>
            ))}
          </div>
        </div>
      </TooltipProvider>

      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        <span>Less</span>
        <div className="flex gap-0.5">
          <div className="h-[12px] w-[12px] rounded-[2px] bg-muted" />
          <div className="h-[12px] w-[12px] rounded-[2px] bg-green-200 dark:bg-green-900" />
          <div className="h-[12px] w-[12px] rounded-[2px] bg-green-400 dark:bg-green-700" />
          <div className="h-[12px] w-[12px] rounded-[2px] bg-green-500 dark:bg-green-500" />
          <div className="h-[12px] w-[12px] rounded-[2px] bg-green-600 dark:bg-green-400" />
        </div>
        <span>More</span>
      </div>
    </div>
  );
}
