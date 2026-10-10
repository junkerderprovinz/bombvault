// The curve of one measured quantity: which series a finding is about, and
// where its points, its usual range and its marks land in the drawing.

import type { AnomalyQuantity, AnomalySeries, AnomalyView } from "./api";

export type CurveQuantity = AnomalyQuantity["quantity"];

/** The drawing's box, in the units of its viewBox. */
export const CURVE_BOX = { width: 320, height: 150, left: 12, right: 308, top: 14, bottom: 118, axis: 126 };

// A failure leaves no measurement, so a finding about failures is drawn over
// how long the runs around it took.
const QUANTITY_OF_METRIC: Record<string, CurveQuantity> = {
  new_data: "newDataBytes",
  new_data_rewrite: "newDataBytes",
  new_data_full: "newDataBytes",
  source_bytes_shrink: "sourceBytes",
  source_bytes_growth: "sourceBytes",
  dump_bytes_shrink: "sourceBytes",
  dump_bytes_growth: "sourceBytes",
  source_files_shrink: "sourceFiles",
  duration_slower: "resticMs",
  dump_duration_slower: "resticMs",
  failure_streak: "resticMs",
  dump_failure_streak: "resticMs",
  flaky: "resticMs",
  dump_flaky: "resticMs",
};

/** Whether a finding was raised on something an item's series measures. A
 *  restore check and a disk have no such series. */
export function findingHasCurve(a: AnomalyView): boolean {
  return a.targetId !== "" && a.metric in QUANTITY_OF_METRIC;
}

export interface CurveSource {
  series: AnomalySeries;
  quantity: AnomalyQuantity;
}

/**
 * curveSource picks what to draw: the quantity a finding was raised on, in the
 * series it belongs to, or without a finding the size of the first series that
 * measured one. A ZFS item measures nothing itself, so its first dataset
 * stands in.
 */
export function curveSource(all: AnomalySeries[], a?: AnomalyView): CurveSource | null {
  if (a) {
    const series = all.find((s) => s.scopeKind === a.scopeKind && s.part === a.part);
    const quantity = series?.quantities.find((q) => q.quantity === QUANTITY_OF_METRIC[a.metric]);
    return series && quantity && quantity.points.length > 0 ? { series, quantity } : null;
  }
  for (const series of all) {
    const quantity = series.quantities.find((q) => q.quantity === "sourceBytes" && q.points.length > 0);
    if (quantity) return { series, quantity };
  }
  return null;
}

type Point = [x: number, y: number];

export interface CurveLayout {
  /** The measured runs, oldest first. */
  line: Point[];
  /** The run a finding was raised on, as an index into `line`, or -1. */
  marked: number;
  band: { y: number; height: number } | null;
  /** Failed runs, set between the measured runs they fell between. */
  crosses: Point[];
  /** Backups the rules still have to see before they know the usual range. */
  toLearn: Point[];
  ticks: { x: number; anchor: "start" | "middle" | "end"; at: number }[];
}

// How many runs the drawing shows. More than a month of daily backups turns
// the line into a smear at this width.
const SHOWN_RUNS = 30;
const LEAD_IN = 3;
// From this factor over the usual value on, the axis is a square root, so the
// usual range stays visible under a spike.
const ROOT_FROM = 4;

function median(values: number[]): number {
  const sorted = [...values].sort((a, b) => a - b);
  const mid = sorted.length >> 1;
  return sorted.length % 2 ? sorted[mid] : (sorted[mid - 1] + sorted[mid]) / 2;
}

const round = (n: number) => Math.round(n * 10) / 10;

export function curveLayout(
  q: AnomalyQuantity,
  failed: AnomalySeries["failed"],
  markedRun = ""
): CurveLayout {
  const { left, right, top, bottom, axis } = CURVE_BOX;
  const at = markedRun ? q.points.findIndex((p) => p.runId === markedRun) : -1;
  let from = Math.max(0, q.points.length - SHOWN_RUNS);
  if (at >= 0 && at < from) from = Math.max(0, at - LEAD_IN);
  const points = q.points.slice(from);
  const values = points.map((p) => p.value);
  const usual = median(values);

  const high = q.band?.high?.threshold ?? null;
  const low = q.band?.low?.threshold ?? null;
  const edges = [high, low].filter((v): v is number => v !== null);
  // Without a range yet, the one a series this size will get still sets the
  // scale, so its first backups do not look wild.
  const all = values.concat(edges.length > 0 ? edges : [usual * 0.75, usual * 1.25]);
  const root = usual > 0 && Math.max(...all) > ROOT_FROM * usual;
  const scale = (v: number) => (root ? Math.sqrt(Math.max(v, 0)) : v);
  const lo = scale(Math.min(...all));
  const span = scale(Math.max(...all)) - lo || Math.max(lo, 1);
  const pad = span * 0.14;

  const remaining = q.learning ? Math.max(0, q.needed - q.samples) : 0;
  const slots = points.length + remaining;
  const step = slots > 1 ? (right - left) / (slots - 1) : 0;
  const x = (i: number) => round(slots > 1 ? left + step * i : (left + right) / 2);
  const y = (v: number) => round(bottom - ((bottom - top) * (scale(v) - lo + pad)) / (span + 2 * pad));

  const line: Point[] = points.map((p, i) => [x(i), y(p.value)]);
  const last = points.length - 1;

  let band: CurveLayout["band"] = null;
  if (high !== null || low !== null) {
    const upper = high !== null ? y(high) : 4;
    const lower = low !== null ? y(low) : axis;
    band = { y: upper, height: round(lower - upper) };
  }

  const crosses: Point[] = [];
  for (const run of failed) {
    const after = points.findIndex((p) => p.at > run.at);
    if (after === 0) continue;
    if (after < 0) crosses.push([round(Math.min(right, line[last][0] + step / 2)), line[last][1]]);
    else crosses.push([round((line[after - 1][0] + line[after][0]) / 2), round((line[after - 1][1] + line[after][1]) / 2)]);
  }

  const ticks: CurveLayout["ticks"] = [{ x: left, anchor: "start", at: points[0].at }];
  const middle = last >> 1;
  if (!q.learning && middle > 0 && middle < last) ticks.push({ x: line[middle][0], anchor: "middle", at: points[middle].at });
  if (last > 0) ticks.push({ x: line[last][0], anchor: remaining > 0 ? "middle" : "end", at: points[last].at });

  return {
    line,
    marked: at >= 0 ? at - from : -1,
    band,
    crosses,
    toLearn: Array.from({ length: remaining }, (_, k) => [x(points.length + k), y(usual)]),
    ticks,
  };
}

/** The `d` of a polyline through the points. */
export function curvePath(points: Point[]): string {
  return points.map(([px, py], i) => `${i ? "L" : "M"}${px} ${py}`).join("");
}
