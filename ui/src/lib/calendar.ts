import dayjs from "dayjs";
import { formatLocalDate } from "@/lib/format";

export interface DayStat {
  date: string;
  total: number;
  success: number;
  error: number;
  cancelled: number;
}

export function getMonthRange(): { from: string; to: string } {
  const to = dayjs().startOf("day");
  const from = to.subtract(5, "month").startOf("month");
  return {
    from: formatLocalDate(from),
    to: formatLocalDate(to),
  };
}

export function generateDayGrid(from: string, to: string): string[] {
  const days: string[] = [];
  let current = dayjs(from);
  const end = dayjs(to);
  while (current.isBefore(end) || current.isSame(end, "day")) {
    days.push(formatLocalDate(current));
    current = current.add(1, "day");
  }
  return days;
}

export function cellColor(stat: DayStat | undefined, maxTotal: number): string {
  if (!stat || stat.total === 0) return "bg-[var(--heat-empty)]";
  const errorRate = stat.error / stat.total;
  const intensity = stat.total / maxTotal;

  if (errorRate > 0.5) {
    if (intensity <= 0.25) return "bg-[var(--heat-danger-1)]";
    if (intensity <= 0.5) return "bg-[var(--heat-danger-2)]";
    if (intensity <= 0.75) return "bg-[var(--heat-danger-3)]";
    return "bg-[var(--heat-danger-4)]";
  }
  if (errorRate > 0.2) {
    if (intensity <= 0.25) return "bg-[var(--heat-warning-1)]";
    if (intensity <= 0.5) return "bg-[var(--heat-warning-2)]";
    if (intensity <= 0.75) return "bg-[var(--heat-warning-3)]";
    return "bg-[var(--heat-warning-4)]";
  }
  if (intensity <= 0.25) return "bg-[var(--heat-success-1)]";
  if (intensity <= 0.5) return "bg-[var(--heat-success-2)]";
  if (intensity <= 0.75) return "bg-[var(--heat-success-3)]";
  return "bg-[var(--heat-success-4)]";
}

export const WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
