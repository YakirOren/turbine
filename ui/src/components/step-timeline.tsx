import { useEffect, useMemo, useState } from "react";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { Skeleton } from "@/components/ui/skeleton";
import { StepStatusBadge } from "@/components/status-badge";
import { TERMINAL_STATUSES, fallbackStatusStyle, statusStyles } from "@/components/step-status";
import {
  formatDuration,
  formatDurationRange,
  formatTimestamp,
  formatTimestampPrecise,
} from "@/lib/format";
import { cn } from "@/lib/utils";
import type { StepNodeRecord } from "@/pages/workflows/steps";
import { buildTimelineParts, effectiveEndMs, mapTicksToPiecewise } from "./step-timeline-parts";
import type { BreakReason, PiecewiseTick, TimelinePart } from "./step-timeline-parts";

const ROW_COLS = "grid grid-cols-[110px_1fr_54px] gap-x-3";
const BREAK_PX = 56;
const SEGMENT_MIN_FRACTION = 0.08;
const STRUCTURAL_TICK_MS = 5_000;

interface StepTimelineProps {
  steps: StepNodeRecord[];
  workflowStartMs?: number;
  workflowStatus: string;
  selectedStepId: number | null;
  onSelectStep: (id: number | null) => void;
  onDrillChild: (childWorkflowId: string) => void;
  isLoading?: boolean;
}

function useNow(intervalMs: number, enabled: boolean): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!enabled) return;
    const id = setInterval(() => setNow(Date.now()), intervalMs);
    return () => clearInterval(id);
  }, [intervalMs, enabled]);
  return now;
}

function pickTickStep(rangeMs: number): number {
  if (rangeMs < 2_000) return 200;
  if (rangeMs < 30_000) return 2_000;
  if (rangeMs < 5 * 60_000) return 15_000;
  if (rangeMs < 60 * 60_000) return 60_000;
  return 10 * 60_000;
}

function breakReasonLabel(reason: BreakReason | undefined): string | null {
  if (!reason) return null;
  switch (reason.kind) {
    case "sleep":
      return "sleep";
    case "awaiting-child-workflow":
      return "awaiting child workflow";
    case "waiting-for-approval":
      return "waiting for approval";
    case "enqueued":
      return "enqueued";
  }
}

function formatTickLabel(ms: number): string {
  if (ms === 0) return "0";
  if (ms < 1_000) return `${ms}ms`;
  if (ms < 60_000) {
    const s = ms / 1_000;
    return Number.isInteger(s) ? `${s}s` : `${s.toFixed(1)}s`;
  }
  const m = ms / 60_000;
  return Number.isInteger(m) ? `${m}m` : `${m.toFixed(1)}m`;
}

export function StepTimeline({
  steps,
  workflowStartMs,
  workflowStatus,
  selectedStepId,
  onSelectStep,
  onDrillChild,
  isLoading,
}: StepTimelineProps) {
  const isRunning = !TERMINAL_STATUSES.has(workflowStatus);
  const now = useNow(1_000, isRunning);
  const structuralNowMs = Math.floor(now / STRUCTURAL_TICK_MS) * STRUCTURAL_TICK_MS;

  const parts = useMemo(
    () =>
      buildTimelineParts({
        steps,
        workflowStartMs,
        workflowStatus,
        nowMs: structuralNowMs,
        isRunning,
      }),
    [steps, workflowStartMs, workflowStatus, structuralNowMs, isRunning],
  );

  const ordered = useMemo(
    () => parts.flatMap((p) => (p.kind === "segment" ? p.steps : [])),
    [parts],
  );

  const activityDuration = useMemo(
    () =>
      parts.reduce(
        (acc, p) => (p.kind === "segment" ? acc + Math.max(p.endMs - p.startMs, 0) : acc),
        0,
      ),
    [parts],
  );

  const activityTicks = useMemo(() => {
    if (activityDuration <= 0) return [0];
    const step = pickTickStep(activityDuration);
    const out: number[] = [];
    for (let t = 0; t <= activityDuration; t += step) out.push(t);
    return out;
  }, [activityDuration]);

  const piecewiseTicks = useMemo(
    () => mapTicksToPiecewise(parts, activityTicks),
    [parts, activityTicks],
  );

  const ticksBySegIndex = useMemo(() => {
    const map = new Map<number, PiecewiseTick[]>();
    for (const t of piecewiseTicks) {
      const bucket = map.get(t.segIndex);
      if (bucket) bucket.push(t);
      else map.set(t.segIndex, [t]);
    }
    return map;
  }, [piecewiseTicks]);

  const lastSegmentIndex =
    parts.reduce((acc, p) => (p.kind === "segment" ? acc + 1 : acc), 0) - 1;
  const hasTrailingBreak = parts.length > 0 && parts[parts.length - 1].kind === "break";
  const trailingBreakIndex = hasTrailingBreak && isRunning ? parts.length - 1 : -1;

  if (isLoading) {
    return <TimelineSkeleton />;
  }

  if (ordered.length === 0) {
    return (
      <div className="flex h-96 items-center justify-center rounded-lg border bg-card p-6 text-center text-[12.5px] text-muted-foreground animate-in fade-in slide-in-from-bottom-4 duration-500 ease-out">
        No steps yet.
      </div>
    );
  }

  return (
    <TooltipProvider delayDuration={200}>
      <div className="flex h-96 flex-col rounded-lg border bg-card p-3 animate-in fade-in slide-in-from-bottom-4 duration-500 ease-out">
        <div className="flex items-center justify-between px-1 pb-2.5 pt-0.5">
          <div className="text-[12px] font-semibold tracking-tight text-foreground">Step timeline</div>
          {isRunning && (
            <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
              <span className="inline-block h-1.5 w-1.5 animate-pulse rounded-full bg-info" />
              live
            </div>
          )}
        </div>

        <div className="relative min-h-0 flex-1 overflow-y-auto">
          <div className={cn(ROW_COLS, "items-center")}>
            <div />
            <div className="relative h-4 border-b border-border-soft">
              <AxisRow
                parts={parts}
                breakContent={(brk, i) => {
                  const endMs = i === trailingBreakIndex ? now : brk.endMs;
                  return (
                    <BreakBand
                      durationMs={endMs - brk.startMs}
                      solid
                      showNowPulse={i === trailingBreakIndex}
                    />
                  );
                }}
              >
                {(_segment, segIndex) =>
                  ticksBySegIndex.get(segIndex)?.map((t) => (
                    <div
                      key={t.activityMs}
                      className="pointer-events-none absolute top-0 -translate-x-1/2 font-mono text-[10px] text-muted-foreground"
                      style={{ left: `${t.withinSegmentPct}%` }}
                    >
                      {formatTickLabel(t.activityMs)}
                    </div>
                  ))
                }
              </AxisRow>
            </div>
            <div />
          </div>

          <div className="relative">
            <div aria-hidden className={cn(ROW_COLS, "pointer-events-none absolute inset-0")}>
              <div />
              <AxisRow parts={parts}>
                {(_segment, segIndex) => (
                  <>
                    {ticksBySegIndex.get(segIndex)?.map((t) => (
                      <div
                        key={`tk-${segIndex}-${t.activityMs}`}
                        className="absolute top-0 bottom-0 border-l border-border-soft/40"
                        style={{ left: `${t.withinSegmentPct}%` }}
                      />
                    ))}
                    {isRunning && !hasTrailingBreak && segIndex === lastSegmentIndex && (
                      <div
                        data-now-line=""
                        className="absolute top-0 bottom-0 border-l border-info"
                        style={{ left: "100%" }}
                      />
                    )}
                  </>
                )}
              </AxisRow>
              <div />
            </div>

          {ordered.map((step) => {
            const s = statusStyles[step.status] ?? fallbackStatusStyle;
            const isChild = step.type === "child-workflow";
            const isSelected = step.functionId === selectedStepId;
            const isRunningStep = !step.endedAtMs;
            const effectiveEnd = effectiveEndMs(step, now, isRunning);
            const duration = !isRunningStep ? step.endedAtMs - step.startedAtMs : null;
            const label = `${step.name}, ${step.status}${duration != null ? `, ${formatDuration(duration)}` : ", running"}`;

            return (
              <div
                key={step.id}
                className={cn(
                  "relative h-7 items-center transition-colors",
                  ROW_COLS,
                  isSelected ? "bg-secondary/60" : "hover:bg-muted/50"
                )}
              >
                <div
                  className="truncate pl-1 font-mono text-[12px] text-foreground"
                  title={step.name}
                >
                  {step.name}
                </div>
                <AxisRow parts={parts}>
                  {(segment) => {
                    if (!segment.steps.some((st) => st.functionId === step.functionId)) return null;
                    const segDur = Math.max(segment.endMs - segment.startMs, 0) || 1;
                    const left = ((step.startedAtMs - segment.startMs) / segDur) * 100;
                    // Clamp to the segment boundary so the running bar can't overrun the now-line.
                    const cappedEnd = Math.min(effectiveEnd, segment.endMs);
                    const width = (Math.max(cappedEnd - step.startedAtMs, 0) / segDur) * 100;
                    return (
                      <div className="relative h-[18px]">
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <button
                              type="button"
                              onClick={() => {
                                if (isChild && step.childWorkflowId) {
                                  onDrillChild(step.childWorkflowId);
                                  return;
                                }
                                onSelectStep(isSelected ? null : step.functionId);
                              }}
                              aria-label={label}
                              className={cn(
                                "absolute top-0 h-[18px] cursor-pointer rounded-sm border transition-[box-shadow,filter]",
                                s.fill,
                                s.border,
                                isChild && "border-dashed",
                                isRunningStep && "animate-pulse",
                                isSelected && cn("ring-1 ring-inset", s.ring),
                                "hover:brightness-105"
                              )}
                              style={{
                                left: `${Math.max(left, 0)}%`,
                                width: `${width}%`,
                                minWidth: 4,
                              }}
                            />
                          </TooltipTrigger>
                          <TooltipContent side="top" className="bg-popover text-popover-foreground border">
                            <div className="space-y-1.5">
                              <div className="font-mono text-[12px] font-semibold">{step.name}</div>
                              <div>
                                <StepStatusBadge status={step.status} />
                              </div>
                              <div className="font-mono text-[11px] opacity-80">
                                started {formatTimestampPrecise(new Date(step.startedAtMs).toISOString())}
                              </div>
                              <div className="font-mono text-[11px] opacity-80">
                                {duration != null ? `duration ${formatDuration(duration)}` : "running…"}
                              </div>
                            </div>
                          </TooltipContent>
                        </Tooltip>
                      </div>
                    );
                  }}
                </AxisRow>
                <div className="pr-1 text-right font-mono text-[11px] text-muted-foreground">
                  {duration != null ? formatDuration(duration) : "—"}
                </div>
              </div>
            );
          })}
          </div>

          <div className={cn(ROW_COLS, "pointer-events-none absolute inset-0")}>
            <div />
            <AxisRow
              parts={parts}
              breakContent={(brk, i) => {
                const endMs = i === trailingBreakIndex ? now : brk.endMs;
                const reasonText = breakReasonLabel(brk.reason);
                return (
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <button
                        type="button"
                        aria-label={`paused ${formatDurationRange(brk.startMs, endMs)}`}
                        className="pointer-events-auto block h-full w-full cursor-help bg-transparent p-0"
                      />
                    </TooltipTrigger>
                    <TooltipContent side="top" className="bg-popover text-popover-foreground border">
                      <div className="space-y-1.5">
                        <div className="font-mono text-[12px] font-semibold">
                          paused {formatDurationRange(brk.startMs, endMs)}
                        </div>
                        {reasonText && (
                          <div className="font-mono text-[11px] opacity-80">
                            reason: {reasonText}
                          </div>
                        )}
                        <div className="font-mono text-[11px] opacity-80">
                          from {formatTimestamp(brk.startMs)}
                        </div>
                        <div className="font-mono text-[11px] opacity-80">
                          to&nbsp;&nbsp; {formatTimestamp(endMs)}
                        </div>
                      </div>
                    </TooltipContent>
                  </Tooltip>
                );
              }}
            >
              {() => null}
            </AxisRow>
            <div />
          </div>
        </div>
      </div>
    </TooltipProvider>
  );
}

interface BreakBandProps {
  durationMs: number;
  solid?: boolean;
  showNowPulse?: boolean;
}

function BreakBand({ durationMs, solid, showNowPulse }: BreakBandProps) {
  return (
    <div
      className={cn(
        "absolute inset-0 flex items-center justify-center border-l border-r border-dashed border-border-soft",
        solid ? "bg-muted/40" : "bg-transparent",
      )}
    >
      {solid ? (
        <>
          <div className="absolute left-0 right-0 top-1/2 -translate-y-1/2 border-t border-dashed border-border-soft/80" />
          <div className="relative z-[1] whitespace-nowrap rounded bg-card/80 px-1.5 font-mono text-[10px] leading-none text-muted-foreground">
            ⋯ {formatDuration(durationMs)} ⋯
          </div>
          {showNowPulse ? (
            <span className="absolute right-1 top-1/2 -translate-y-1/2 inline-block h-1.5 w-1.5 animate-pulse rounded-full bg-info" />
          ) : null}
        </>
      ) : null}
    </div>
  );
}

interface AxisRowProps {
  parts: TimelinePart[];
  children: (
    segment: Extract<TimelinePart, { kind: "segment" }>,
    index: number,
  ) => React.ReactNode;
  breakContent?: (
    brk: Extract<TimelinePart, { kind: "break" }>,
    index: number,
  ) => React.ReactNode;
}

function defaultBreakContent(brk: Extract<TimelinePart, { kind: "break" }>) {
  return <BreakBand durationMs={brk.endMs - brk.startMs} />;
}

function AxisRow({ parts, children, breakContent = defaultBreakContent }: AxisRowProps) {
  const breakCount = parts.filter((q) => q.kind === "break").length;
  return (
    <div className="relative flex h-full w-full">
      {parts.map((p, i) => {
        if (p.kind === "break") {
          return (
            <div
              key={`b-${i}`}
              data-break
              className="relative flex-none"
              style={{ flex: `0 0 ${BREAK_PX}px` }}
            >
              {breakContent(p, i)}
            </div>
          );
        }
        const segDur = Math.max(p.endMs - p.startMs, 0);
        return (
          <div
            key={`s-${i}`}
            className="relative"
            style={{
              flex: `${segDur} 1 auto`,
              minWidth: `calc((100% - ${breakCount * BREAK_PX}px) * ${SEGMENT_MIN_FRACTION})`,
            }}
          >
            {children(p, i)}
          </div>
        );
      })}
    </div>
  );
}

export function TimelineSkeleton() {
  const ticks = [0, 2, 4, 6, 8, 10, 12, 14];
  const rows = 5;
  return (
    <div className="h-96 rounded-lg border bg-card p-3">
      <div className="flex items-center justify-between px-1 pb-2.5 pt-0.5">
        <div className="text-[12px] font-semibold tracking-tight text-foreground">Step timeline</div>
        <div className="text-[11px] text-muted-foreground">Loading…</div>
      </div>
      <div className={cn(ROW_COLS, "items-center")}>
        <div />
        <div className="relative h-4 border-b border-border-soft">
          {ticks.map((t) => (
            <div
              key={t}
              className="absolute top-0 -translate-x-1/2 font-mono text-[10px] text-muted-foreground"
              style={{ left: `${(t / 14) * 100}%` }}
            >
              {t}s
            </div>
          ))}
        </div>
        <div />
      </div>
      {Array.from({ length: rows }).map((_, i) => {
        const left = (i * 2) % 8;
        const width = 3 + ((i * 7) % 5);
        return (
          <div
            key={i}
            className={cn(ROW_COLS, "items-center py-1")}
          >
            <Skeleton className="h-3 w-20 rounded-sm" />
            <div className="relative h-4">
              <Skeleton
                className="absolute inset-y-0.5 rounded-sm"
                style={{ left: `${(left / 14) * 100}%`, width: `${(width / 14) * 100}%` }}
              />
            </div>
            <Skeleton className="h-3 w-10 justify-self-end rounded-sm" />
          </div>
        );
      })}
    </div>
  );
}
