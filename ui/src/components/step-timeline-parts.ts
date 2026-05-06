import type { StepNodeRecord } from "@/pages/workflows/steps";

export const BREAK_FLOOR_MS = 10_000;
export const BREAK_RELATIVE_THRESHOLD = 0.3;

const STATUS_WAITING_FOR_APPROVAL = "WAITING_FOR_APPROVAL";
const STATUS_ENQUEUED = "ENQUEUED";
const BARRIER_STEP_SLEEP = "pt.sleep";
const BARRIER_STEP_GET_RESULT = "pt.getResult";

export type BreakReason =
  | { kind: "sleep" }
  | { kind: "awaiting-child-workflow" }
  | { kind: "waiting-for-approval" }
  | { kind: "enqueued" };

export type TimelinePart =
  | {
      kind: "segment";
      startMs: number;
      endMs: number;
      steps: StepNodeRecord[];
    }
  | {
      kind: "break";
      startMs: number;
      endMs: number;
      reason?: BreakReason;
    };

export interface BuildTimelinePartsInput {
  steps: StepNodeRecord[];
  workflowStartMs?: number;
  workflowStatus: string;
  nowMs: number;
  isRunning: boolean;
}

export function buildTimelineParts(input: BuildTimelinePartsInput): TimelinePart[] {
  const { steps } = input;
  const withStart = steps.filter((s) => s.startedAtMs > 0);
  if (withStart.length === 0) return [];
  const ordered = [...withStart].sort((a, b) => a.startedAtMs - b.startedAtMs);

  type Gap = { index: number; startMs: number; endMs: number; durMs: number };
  const candidates: Gap[] = [];
  for (let i = 0; i < ordered.length - 1; i++) {
    const prevEnd = ordered[i].endedAtMs || ordered[i].startedAtMs;
    const nextStart = ordered[i + 1].startedAtMs;
    const gap = nextStart - prevEnd;
    if (gap >= BREAK_FLOOR_MS) {
      candidates.push({ index: i, startMs: prevEnd, endMs: nextStart, durMs: gap });
    }
  }

  const stepDurTotal = ordered.reduce(
    (acc, s) => acc + Math.max((s.endedAtMs || s.startedAtMs) - s.startedAtMs, 0),
    0,
  );
  const candidateGapTotal = candidates.reduce((acc, g) => acc + g.durMs, 0);
  const firstStart = ordered[0].startedAtMs;
  const lastEnd = ordered.reduce(
    (max, s) => Math.max(max, s.endedAtMs || s.startedAtMs),
    firstStart,
  );
  const allGapsTotal = lastEnd - firstStart - stepDurTotal;
  const nonCandidateGapTotal = Math.max(allGapsTotal - candidateGapTotal, 0);

  let leadingBreak: { startMs: number; endMs: number } | null = null;
  if (input.workflowStartMs !== undefined && input.workflowStartMs < firstStart) {
    const leadDur = firstStart - input.workflowStartMs;
    if (leadDur >= BREAK_FLOOR_MS) {
      leadingBreak = { startMs: input.workflowStartMs, endMs: firstStart };
    }
  }

  let trailingBreak: { startMs: number; endMs: number } | null = null;
  if (input.isRunning) {
    const tailDur = input.nowMs - lastEnd;
    if (tailDur >= BREAK_FLOOR_MS) {
      trailingBreak = { startMs: lastEnd, endMs: input.nowMs };
    }
  }

  const leadingDur = leadingBreak ? leadingBreak.endMs - leadingBreak.startMs : 0;
  const trailingDur = trailingBreak ? trailingBreak.endMs - trailingBreak.startMs : 0;
  const totalActivity = stepDurTotal + nonCandidateGapTotal + leadingDur + trailingDur;

  const promoted = new Set<number>();
  for (const g of candidates) {
    if (totalActivity > 0 && g.durMs / totalActivity >= BREAK_RELATIVE_THRESHOLD) {
      promoted.add(g.index);
    }
  }

  if (leadingBreak && totalActivity > 0 && leadingDur / totalActivity < BREAK_RELATIVE_THRESHOLD) {
    leadingBreak = null;
  }
  if (trailingBreak && totalActivity > 0 && trailingDur / totalActivity < BREAK_RELATIVE_THRESHOLD) {
    trailingBreak = null;
  }

  const parts: TimelinePart[] = [];
  if (leadingBreak) {
    parts.push({
      kind: "break",
      startMs: leadingBreak.startMs,
      endMs: leadingBreak.endMs,
      reason: inferBreakReason({
        prevStep: null,
        isLeading: true,
        workflowStatus: input.workflowStatus,
      }),
    });
  }
  let segStart = firstStart;
  let segSteps: StepNodeRecord[] = [];

  for (let i = 0; i < ordered.length; i++) {
    segSteps.push(ordered[i]);
    if (promoted.has(i)) {
      const gap = candidates.find((c) => c.index === i)!;
      parts.push({ kind: "segment", startMs: segStart, endMs: gap.startMs, steps: segSteps });
      parts.push({
        kind: "break",
        startMs: gap.startMs,
        endMs: gap.endMs,
        reason: inferBreakReason({
          prevStep: ordered[i],
          workflowStatus: input.workflowStatus,
        }),
      });
      segStart = gap.endMs;
      segSteps = [];
    }
  }
  parts.push({ kind: "segment", startMs: segStart, endMs: lastEnd, steps: segSteps });
  if (trailingBreak) {
    parts.push({
      kind: "break",
      startMs: trailingBreak.startMs,
      endMs: trailingBreak.endMs,
      reason: inferBreakReason({
        prevStep: ordered[ordered.length - 1],
        isTrailing: true,
        workflowStatus: input.workflowStatus,
      }),
    });
  }

  return parts;
}

export interface PiecewiseTick {
  activityMs: number;
  segIndex: number;
  withinSegmentPct: number;
}

export interface InferBreakReasonInput {
  prevStep: StepNodeRecord | null;
  isLeading?: boolean;
  isTrailing?: boolean;
  workflowStatus: string;
}

export function inferBreakReason(input: InferBreakReasonInput): BreakReason | undefined {
  const { prevStep, isLeading, isTrailing, workflowStatus } = input;
  if (prevStep?.name === BARRIER_STEP_SLEEP) return { kind: "sleep" };
  if (prevStep?.name === BARRIER_STEP_GET_RESULT) return { kind: "awaiting-child-workflow" };
  if (isTrailing && workflowStatus === STATUS_WAITING_FOR_APPROVAL) {
    return { kind: "waiting-for-approval" };
  }
  if (isLeading && workflowStatus === STATUS_ENQUEUED) return { kind: "enqueued" };
  return undefined;
}

export function mapTicksToPiecewise(
  parts: TimelinePart[],
  activityTicksMs: number[],
): PiecewiseTick[] {
  interface SegInfo {
    segIndex: number;
    activityStart: number;
    activityEnd: number;
  }
  const segInfos: SegInfo[] = [];
  let activityCursor = 0;
  let segCounter = 0;
  for (const p of parts) {
    if (p.kind !== "segment") continue;
    const dur = Math.max(p.endMs - p.startMs, 0);
    segInfos.push({
      segIndex: segCounter++,
      activityStart: activityCursor,
      activityEnd: activityCursor + dur,
    });
    activityCursor += dur;
  }

  const lastSeg = segInfos[segInfos.length - 1];

  return activityTicksMs.map((t) => {
    for (const info of segInfos) {
      if (t >= info.activityStart && t <= info.activityEnd) {
        const dur = info.activityEnd - info.activityStart || 1;
        return {
          activityMs: t,
          segIndex: info.segIndex,
          withinSegmentPct: ((t - info.activityStart) / dur) * 100,
        };
      }
    }
    return {
      activityMs: t,
      segIndex: lastSeg?.segIndex ?? 0,
      withinSegmentPct: lastSeg ? 100 : 0,
    };
  });
}
