import { describe, expect, it } from "vitest";
import {
  buildTimelineParts,
  inferBreakReason,
  mapTicksToPiecewise,
  type TimelinePart,
} from "./step-timeline-parts";
import type { StepNodeRecord } from "@/pages/workflows/steps";

function step(overrides: Partial<StepNodeRecord>): StepNodeRecord {
  return {
    id: "1",
    name: "step",
    type: "step",
    status: "success",
    functionId: 1,
    startedAtMs: 0,
    endedAtMs: 0,
    output: null,
    error: null,
    childWorkflowId: null,
    ...overrides,
  } as StepNodeRecord;
}

describe("buildTimelineParts", () => {
  it("returns a single segment when two steps are adjacent in time", () => {
    const steps = [
      step({ id: "a", functionId: 1, startedAtMs: 1000, endedAtMs: 2000 }),
      step({ id: "b", functionId: 2, startedAtMs: 2100, endedAtMs: 3000 }),
    ];
    const parts = buildTimelineParts({
      steps,
      workflowStatus: "SUCCESS",
      nowMs: 3000,
      isRunning: false,
    });
    expect(parts).toHaveLength(1);
    expect(parts[0].kind).toBe("segment");
    if (parts[0].kind === "segment") {
      expect(parts[0].startMs).toBe(1000);
      expect(parts[0].endMs).toBe(3000);
      expect(parts[0].steps.map((s) => s.id)).toEqual(["a", "b"]);
    }
  });

  it("promotes a break when the gap exceeds 10s floor and 30% of activity", () => {
    const steps = [
      step({ id: "a", functionId: 1, startedAtMs: 1_000, endedAtMs: 2_000 }),
      step({ id: "b", functionId: 2, startedAtMs: 122_000, endedAtMs: 123_000 }),
    ];
    const parts = buildTimelineParts({
      steps,
      workflowStatus: "SUCCESS",
      nowMs: 123_000,
      isRunning: false,
    });
    expect(parts.map((p) => p.kind)).toEqual(["segment", "break", "segment"]);
    const [seg1, brk, seg2] = parts;
    if (seg1.kind !== "segment" || brk.kind !== "break" || seg2.kind !== "segment") {
      throw new Error("unexpected part shape");
    }
    expect(seg1.steps.map((s) => s.id)).toEqual(["a"]);
    expect(seg2.steps.map((s) => s.id)).toEqual(["b"]);
    expect(brk.startMs).toBe(2_000);
    expect(brk.endMs).toBe(122_000);
  });

  it("does not promote a gap below the 10s floor even if it dominates ratio", () => {
    const steps = [
      step({ id: "a", functionId: 1, startedAtMs: 1_000, endedAtMs: 1_100 }),
      step({ id: "b", functionId: 2, startedAtMs: 6_100, endedAtMs: 6_200 }),
    ];
    const parts = buildTimelineParts({
      steps,
      workflowStatus: "SUCCESS",
      nowMs: 6_200,
      isRunning: false,
    });
    expect(parts).toHaveLength(1);
    expect(parts[0].kind).toBe("segment");
  });

  it("does not promote a gap that clears the floor but not the ratio", () => {
    const steps = [
      step({ id: "a", functionId: 1, startedAtMs: 0, endedAtMs: 60_000 }),
      step({ id: "b", functionId: 2, startedAtMs: 72_000, endedAtMs: 132_000 }),
    ];
    const parts = buildTimelineParts({
      steps,
      workflowStatus: "SUCCESS",
      nowMs: 132_000,
      isRunning: false,
    });
    expect(parts).toHaveLength(1);
    expect(parts[0].kind).toBe("segment");
  });

  it("promotes a leading break when workflowStartMs precedes the first step", () => {
    const steps = [
      step({ id: "a", functionId: 1, startedAtMs: 120_000, endedAtMs: 122_000 }),
    ];
    const parts = buildTimelineParts({
      steps,
      workflowStartMs: 0,
      workflowStatus: "SUCCESS",
      nowMs: 122_000,
      isRunning: false,
    });
    expect(parts.map((p) => p.kind)).toEqual(["break", "segment"]);
    const [brk, seg] = parts;
    if (brk.kind !== "break" || seg.kind !== "segment") throw new Error();
    expect(brk.startMs).toBe(0);
    expect(brk.endMs).toBe(120_000);
    expect(seg.steps.map((s) => s.id)).toEqual(["a"]);
  });

  it("promotes a trailing break for a still-running workflow with a long idle tail", () => {
    const steps = [
      step({ id: "a", functionId: 1, startedAtMs: 1_000, endedAtMs: 2_000 }),
    ];
    const parts = buildTimelineParts({
      steps,
      workflowStatus: "WAITING_FOR_APPROVAL",
      nowMs: 200_000,
      isRunning: true,
    });
    expect(parts.map((p) => p.kind)).toEqual(["segment", "break"]);
    const [seg, brk] = parts;
    if (seg.kind !== "segment" || brk.kind !== "break") throw new Error();
    expect(seg.endMs).toBe(2_000);
    expect(brk.startMs).toBe(2_000);
    expect(brk.endMs).toBe(200_000);
  });

  it("does not emit a trailing break when the workflow is not running", () => {
    const steps = [
      step({ id: "a", functionId: 1, startedAtMs: 1_000, endedAtMs: 2_000 }),
    ];
    const parts = buildTimelineParts({
      steps,
      workflowStatus: "SUCCESS",
      nowMs: 200_000,
      isRunning: false,
    });
    expect(parts).toHaveLength(1);
    expect(parts[0].kind).toBe("segment");
  });
});

describe("inferBreakReason", () => {
  it("returns sleep when the preceding step is pt.sleep", () => {
    const prev = step({ id: "a", name: "pt.sleep" });
    expect(
      inferBreakReason({ prevStep: prev, isTrailing: false, workflowStatus: "SUCCESS" }),
    ).toEqual({ kind: "sleep" });
  });

  it("returns awaiting-child-workflow when preceding step is pt.getResult", () => {
    const prev = step({ id: "a", name: "pt.getResult" });
    expect(
      inferBreakReason({ prevStep: prev, isTrailing: false, workflowStatus: "SUCCESS" }),
    ).toEqual({ kind: "awaiting-child-workflow" });
  });

  it("returns waiting-for-approval for a trailing break in WAITING_FOR_APPROVAL", () => {
    expect(
      inferBreakReason({
        prevStep: step({ id: "a", name: "prepare" }),
        isTrailing: true,
        workflowStatus: "WAITING_FOR_APPROVAL",
      }),
    ).toEqual({ kind: "waiting-for-approval" });
  });

  it("returns enqueued for a leading break when status is ENQUEUED", () => {
    expect(
      inferBreakReason({ prevStep: null, isLeading: true, workflowStatus: "ENQUEUED" }),
    ).toEqual({ kind: "enqueued" });
  });

  it("returns undefined when nothing matches", () => {
    expect(
      inferBreakReason({
        prevStep: step({ id: "a", name: "normal-step" }),
        isTrailing: false,
        workflowStatus: "SUCCESS",
      }),
    ).toBeUndefined();
  });
});

describe("mapTicksToPiecewise", () => {
  function seg(startMs: number, endMs: number): TimelinePart {
    return { kind: "segment", startMs, endMs, steps: [] };
  }
  function brk(startMs: number, endMs: number): TimelinePart {
    return { kind: "break", startMs, endMs };
  }

  it("maps activity ticks linearly when there are no breaks", () => {
    const parts: TimelinePart[] = [seg(0, 100)];
    const ticks = mapTicksToPiecewise(parts, [0, 50, 100]);
    expect(ticks.map((t) => t.withinSegmentPct)).toEqual([0, 50, 100]);
    expect(ticks.every((t) => t.segIndex === 0)).toBe(true);
  });

  it("places activity ticks across segments regardless of breaks", () => {
    // Two 10ms segments plus a break in the middle. Activity total = 20ms.
    const parts: TimelinePart[] = [seg(0, 10), brk(10, 110), seg(110, 120)];
    const ticks = mapTicksToPiecewise(parts, [0, 5, 10, 15, 20]);

    expect(ticks[0]).toEqual({ activityMs: 0, segIndex: 0, withinSegmentPct: 0 });
    expect(ticks[1]).toEqual({ activityMs: 5, segIndex: 0, withinSegmentPct: 50 });
    // Boundary at activity 10 falls in seg0 (first match) at 100% within.
    expect(ticks[2]).toEqual({ activityMs: 10, segIndex: 0, withinSegmentPct: 100 });
    expect(ticks[3]).toEqual({ activityMs: 15, segIndex: 1, withinSegmentPct: 50 });
    expect(ticks[4]).toEqual({ activityMs: 20, segIndex: 1, withinSegmentPct: 100 });
  });

  it("clamps overshooting ticks to the end of the last segment", () => {
    const parts: TimelinePart[] = [seg(0, 10), brk(10, 110), seg(110, 120)];
    const ticks = mapTicksToPiecewise(parts, [30]);
    expect(ticks).toEqual([{ activityMs: 30, segIndex: 1, withinSegmentPct: 100 }]);
  });
});
