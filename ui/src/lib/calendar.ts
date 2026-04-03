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
  if (!stat || stat.total === 0) return "bg-muted";
  const errorRate = stat.error / stat.total;
  const intensity = stat.total / maxTotal;

  if (errorRate > 0.5) {
    if (intensity <= 0.25) return "bg-red-200 dark:bg-red-900/60";
    if (intensity <= 0.5) return "bg-red-300 dark:bg-red-800";
    if (intensity <= 0.75) return "bg-red-400 dark:bg-red-700";
    return "bg-red-500 dark:bg-red-600";
  }
  if (errorRate > 0.2) {
    if (intensity <= 0.25) return "bg-yellow-200 dark:bg-yellow-900/60";
    if (intensity <= 0.5) return "bg-yellow-300 dark:bg-yellow-800";
    if (intensity <= 0.75) return "bg-orange-400 dark:bg-orange-700";
    return "bg-orange-500 dark:bg-orange-600";
  }
  if (intensity <= 0.25) return "bg-green-200 dark:bg-green-900";
  if (intensity <= 0.5) return "bg-green-400 dark:bg-green-700";
  if (intensity <= 0.75) return "bg-green-500 dark:bg-green-500";
  return "bg-green-600 dark:bg-green-400";
}

export const WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
