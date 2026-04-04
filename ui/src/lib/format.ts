import dayjs from "dayjs";
import relativeTime from "dayjs/plugin/relativeTime";
import duration from "dayjs/plugin/duration";

dayjs.extend(relativeTime);
dayjs.extend(duration);

export function timeAgo(epochMs: number): string {
  if (!epochMs) return "\u2014";
  return dayjs(epochMs).fromNow();
}

export function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  const d = dayjs.duration(ms);
  const hours = Math.floor(d.asHours());
  const mins = d.minutes();
  const secs = d.seconds();
  if (hours > 0) return `${hours}h ${mins}m`;
  if (mins > 0) return `${mins}m ${secs}s`;
  return `${d.asSeconds().toFixed(1)}s`;
}

export function formatDurationRange(startMs: number, endMs: number): string {
  return formatDuration(endMs - startMs);
}

export function formatTimestamp(epochMs: number): string {
  return dayjs(epochMs).format("YYYY/MM/DD HH:mm:ss");
}

export function formatTimestampPrecise(dateStr: string): string {
  return dayjs(dateStr).format("YYYY/MM/DD HH:mm:ss.SSS");
}

export function formatLocalDate(d: dayjs.Dayjs): string {
  return d.format("YYYY-MM-DD");
}

export function formatReadableDate(dateStr: string): string {
  return dayjs(dateStr).format("ddd, MMM D");
}

export function formatMonthShort(dateStr: string): string {
  return dayjs(dateStr).format("MMM");
}
