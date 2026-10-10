// Which series a curve draws and where its parts land. The drawing itself is
// checked in SeriesCurve.dom.test.tsx; these are the numbers under it.
import { describe, expect, it } from "vitest";
import { CURVE_BOX, curveLayout, curvePath, curveSource, findingHasCurve } from "./anomalyCurve";
import type { AnomalyQuantity, AnomalySeries, AnomalyView } from "./api";

const DAY = 86400;

function quantity(over: Partial<AnomalyQuantity> = {}, values: number[] = [10, 11, 9, 10, 12]): AnomalyQuantity {
  return {
    quantity: "sourceBytes",
    points: values.map((value, i) => ({ runId: `run-${i}`, at: 1000 + i * DAY, value })),
    learning: false,
    samples: 10,
    needed: 10,
    band: null,
    ...over,
  };
}

function series(over: Partial<AnomalySeries> = {}): AnomalySeries {
  return { scopeKind: "item", part: "", quantities: [quantity()], failed: [], ...over };
}

function finding(over: Partial<AnomalyView>): AnomalyView {
  return { metric: "source_bytes_shrink", scopeKind: "item", part: "", targetId: "tg-1", ...over } as AnomalyView;
}

const limit = (threshold: number) => ({ metric: "", expected: threshold, threshold, samples: 10 });

describe("curveSource", () => {
  const all = [
    series({ quantities: [quantity(), quantity({ quantity: "newDataBytes" }), quantity({ quantity: "resticMs" })] }),
    series({ scopeKind: "dump", quantities: [quantity(), quantity({ quantity: "resticMs" })] }),
    series({ scopeKind: "zfsds", part: "tank/media", quantities: [quantity({ quantity: "sourceFiles" })] }),
  ];

  it("draws the quantity a finding was raised on, in its own series", () => {
    expect(curveSource(all, finding({ metric: "new_data" }))?.quantity.quantity).toBe("newDataBytes");
    const dump = curveSource(all, finding({ metric: "dump_duration_slower", scopeKind: "dump" }));
    expect(dump?.series.scopeKind).toBe("dump");
    expect(dump?.quantity.quantity).toBe("resticMs");
    const dataset = curveSource(all, finding({ metric: "source_files_shrink", scopeKind: "zfsds", part: "tank/media" }));
    expect(dataset?.series.part).toBe("tank/media");
  });

  it("draws a finding about failures over how long the runs took", () => {
    expect(curveSource(all, finding({ metric: "flaky" }))?.quantity.quantity).toBe("resticMs");
  });

  it("draws the size of the item where no finding chooses", () => {
    expect(curveSource(all)?.quantity.quantity).toBe("sourceBytes");
  });

  it("lets a ZFS item's first dataset stand in for the item, which measures nothing", () => {
    const zfs = [series({ quantities: [] }), series({ scopeKind: "zfsds", part: "tank/a" })];
    expect(curveSource(zfs)?.series.part).toBe("tank/a");
  });

  it("draws nothing without a measurement", () => {
    expect(curveSource([series({ quantities: [quantity({}, [])] })])).toBeNull();
    expect(curveSource(all, finding({ metric: "source_files_shrink" }))).toBeNull();
  });
});

describe("findingHasCurve", () => {
  it("is true for a finding of an item and false for a restore check or a disk", () => {
    expect(findingHasCurve(finding({}))).toBe(true);
    expect(findingHasCurve(finding({ metric: "drill_dr", scopeKind: "domain", targetId: "" }))).toBe(false);
    expect(findingHasCurve(finding({ metric: "capacity_low", scopeKind: "volume", targetId: "" }))).toBe(false);
  });
});

describe("curveLayout", () => {
  const { left, right, top, bottom, axis } = CURVE_BOX;

  it("spreads the runs over the width, oldest on the left", () => {
    const { line } = curveLayout(quantity(), []);
    expect(line).toHaveLength(5);
    expect(line[0][0]).toBe(left);
    expect(line[4][0]).toBe(right);
    for (const [, y] of line) {
      expect(y).toBeGreaterThanOrEqual(top);
      expect(y).toBeLessThanOrEqual(bottom);
    }
  });

  it("has no band while the rules are learning and leaves room for the backups still to come", () => {
    const layout = curveLayout(quantity({ learning: true, samples: 5 }), []);
    expect(layout.band).toBeNull();
    expect(layout.toLearn).toHaveLength(5);
    expect(layout.line[4][0]).toBeLessThan(right);
    expect(layout.toLearn[4][0]).toBe(right);
  });

  it("draws the band between the two thresholds", () => {
    const { band, line } = curveLayout(quantity({ band: { low: limit(5), high: limit(20) } }), []);
    expect(band!.y).toBeGreaterThan(top - 10);
    expect(band!.y + band!.height).toBeLessThan(axis);
    for (const [, y] of line) {
      expect(y).toBeGreaterThan(band!.y);
      expect(y).toBeLessThan(band!.y + band!.height);
    }
  });

  it("opens the band to the edge on the side no rule watches", () => {
    const upTo = curveLayout(quantity({ band: { low: null, high: limit(20) } }), []).band!;
    expect(upTo.y + upTo.height).toBe(axis);
    const above = curveLayout(quantity({ band: { low: limit(5), high: null } }), []).band!;
    expect(above.y).toBeLessThan(top);
  });

  it("marks the run a finding was raised on", () => {
    expect(curveLayout(quantity(), [], "run-3").marked).toBe(3);
    expect(curveLayout(quantity(), [], "run-gone").marked).toBe(-1);
    expect(curveLayout(quantity(), []).marked).toBe(-1);
  });

  it("shows the newest thirty runs, and reaches back for a marked run older than that", () => {
    const many = quantity({}, Array.from({ length: 50 }, () => 10));
    expect(curveLayout(many, []).line).toHaveLength(30);
    const back = curveLayout(many, [], "run-10");
    expect(back.line).toHaveLength(43);
    expect(back.marked).toBe(3);
  });

  it("keeps the usual range visible under a spike many times its size", () => {
    const spiked = quantity({ band: { low: null, high: limit(20) } }, [10, 11, 9, 10, 1000]);
    const { band, line } = curveLayout(spiked, []);
    expect(line[4][1]).toBeLessThan(top + 20);
    // On a linear axis the usual runs sit within a pixel of the band's edge.
    expect(line[0][1] - band!.y).toBeGreaterThan(2);
  });

  it("sets a failed run between the runs it fell between and skips one before the window", () => {
    const failed = [
      { runId: "early", at: 500 },
      { runId: "between", at: 1000 + 1.5 * DAY },
      { runId: "latest", at: 1000 + 9 * DAY },
    ];
    const { crosses, line } = curveLayout(quantity(), failed);
    expect(crosses).toHaveLength(2);
    expect(crosses[0][0]).toBeGreaterThan(line[1][0]);
    expect(crosses[0][0]).toBeLessThan(line[2][0]);
    expect(crosses[1][0]).toBe(right);
  });

  it("dates the ends and the middle of the line", () => {
    const { ticks } = curveLayout(quantity(), []);
    expect(ticks.map((tick) => tick.at)).toEqual([1000, 1000 + 2 * DAY, 1000 + 4 * DAY]);
    expect(ticks.map((tick) => tick.anchor)).toEqual(["start", "middle", "end"]);
  });

  it("draws a single run as a point with one date", () => {
    const { line, ticks } = curveLayout(quantity({ learning: true, samples: 1 }, [10]), []);
    expect(line).toHaveLength(1);
    expect(ticks).toHaveLength(1);
  });
});

describe("curvePath", () => {
  it("joins the points into one line", () => {
    expect(
      curvePath([
        [1, 2],
        [3, 4],
      ])
    ).toBe("M1 2L3 4");
  });
});
