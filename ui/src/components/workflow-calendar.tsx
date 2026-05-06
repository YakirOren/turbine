import { useMemo, useState } from "react";
import dayjs from "dayjs";
import { useQuery } from "@tanstack/react-query";
import { pbClient } from "@/providers/pocketbase";
import { cn } from "@/lib/utils";
import { formatReadableDate, formatMonthShort } from "@/lib/format";
import { type DayStat, getMonthRange, generateDayGrid, cellColor, WEEKDAYS } from "@/lib/calendar";
import { Calendar, ChevronRight } from "lucide-react";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

const COLLAPSED_STORAGE_KEY = "pt_calendar_collapsed";

interface WorkflowCalendarProps {
  onDayClick: (dayStart: number, dayEnd: number) => void;
}

export function WorkflowCalendar({ onDayClick }: WorkflowCalendarProps) {
  const [collapsed, setCollapsed] = useState(() => {
    try {
      const stored = localStorage.getItem(COLLAPSED_STORAGE_KEY);
      return stored === null ? true : stored === "true";
    } catch {
      return true;
    }
  });

  const toggleCollapsed = () => {
    const next = !collapsed;
    setCollapsed(next);
    localStorage.setItem(COLLAPSED_STORAGE_KEY, String(next));
  };

  const { from, to } = useMemo(getMonthRange, []);

  const calendarQuery = useQuery<DayStat[]>({
    queryKey: ["calendar", from, to],
    queryFn: () =>
      pbClient
        .send<DayStat[]>(`/api/pt/calendar?from=${from}&to=${to}`, { method: "GET" }),
  });

  const stats: DayStat[] = calendarQuery.data ?? [];

  const summary = useMemo(() => {
    let total = 0;
    let success = 0;
    let errors = 0;
    for (const s of stats) {
      total += s.total;
      success += s.success;
      errors += s.error;
    }
    const successRate = total > 0 ? Math.round((success / total) * 100) : 0;
    return { total, errors, successRate };
  }, [stats]);

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
    const firstDayOfWeek = dayjs(days[0]).day();
    for (let i = 0; i < firstDayOfWeek; i++) currentWeek.push("");
    for (const day of days) {
      const dow = dayjs(day).day();
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
          label: formatMonthShort(firstDay),
          weekIndex: wi,
        });
        lastMonth = month;
      }
    }
    return labels;
  }, [weeks]);

  const handleDayClick = (day: string) => {
    const dayStart = dayjs(day).startOf("day").valueOf();
    const dayEnd = dayjs(day).endOf("day").valueOf() + 1;
    onDayClick(dayStart, dayEnd);
  };

  const isLoading = calendarQuery.isLoading && stats.length === 0;

  return (
    <div className="space-y-3">
      <button
        onClick={toggleCollapsed}
        aria-expanded={!collapsed}
        aria-controls="activity-calendar-grid"
        className="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
      >
        <ChevronRight
          className={cn(
            "h-3.5 w-3.5 transition-transform",
            !collapsed && "rotate-90"
          )}
        />
        <Calendar className="h-3.5 w-3.5" />
        <span>Activity</span>
        {stats.length > 0 && (
          <span className="flex items-center gap-3 ml-1 text-xs">
            <span className="font-mono">{summary.total.toLocaleString()} runs</span>
            <span className={cn(
              "font-mono",
              summary.successRate >= 90
                ? "text-success-foreground"
                : summary.successRate >= 70
                  ? "text-warning-foreground"
                  : "text-danger-foreground"
            )}>
              {summary.successRate}%
            </span>
            {summary.errors > 0 && (
              <span className="font-mono text-danger-foreground">{summary.errors} failed</span>
            )}
          </span>
        )}
      </button>

      <div
        id="activity-calendar-grid"
        className="grid transition-[grid-template-rows] duration-200 ease-out"
        style={{ gridTemplateRows: collapsed ? "0fr" : "1fr" }}
      >
        <div className="overflow-hidden">
          <TooltipProvider delayDuration={100}>
            <div className="overflow-x-auto pb-1">
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

                {isLoading
                  ? Array.from({ length: 26 }, (_, wi) => (
                      <div key={wi} className="flex flex-col gap-0.5">
                        {Array.from({ length: 7 }, (_, di) => (
                          <div key={di} className="h-[12px] w-[12px] rounded-[2px] bg-muted-foreground/15 animate-pulse" />
                        ))}
                      </div>
                    ))
                  : weeks.map((week, wi) => (
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
                                    cellColor(stat, maxTotal),
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
                                <div className="font-medium">{formatReadableDate(day)}</div>
                                {count > 0 ? (
                                  <div className="space-y-0.5 text-muted-foreground">
                                    <div>{count} workflow{count !== 1 ? "s" : ""}</div>
                                    {stat!.success > 0 && <div className="text-success-foreground">{stat!.success} passed</div>}
                                    {stat!.error > 0 && <div className="text-danger-foreground">{stat!.error} failed</div>}
                                    {stat!.cancelled > 0 && <div className="text-muted-foreground">{stat!.cancelled} cancelled</div>}
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

          <div className="flex items-center gap-4 text-xs text-muted-foreground mt-2">
            <div className="flex items-center gap-1.5">
              <span>Less</span>
              <div className="flex gap-0.5">
                <div className="h-[12px] w-[12px] rounded-[2px] bg-[var(--heat-empty)]" />
                <div className="h-[12px] w-[12px] rounded-[2px] bg-[var(--heat-success-1)]" />
                <div className="h-[12px] w-[12px] rounded-[2px] bg-[var(--heat-success-2)]" />
                <div className="h-[12px] w-[12px] rounded-[2px] bg-[var(--heat-success-3)]" />
                <div className="h-[12px] w-[12px] rounded-[2px] bg-[var(--heat-success-4)]" />
              </div>
              <span>More</span>
            </div>
            <div className="flex items-center gap-1.5">
              <div className="h-[12px] w-[12px] rounded-[2px] bg-[var(--heat-warning-3)]" />
              <span>Mixed</span>
            </div>
            <div className="flex items-center gap-1.5">
              <div className="h-[12px] w-[12px] rounded-[2px] bg-[var(--heat-danger-3)]" />
              <span>Errors</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
