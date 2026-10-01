// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { timelineMark, timelinePlace, timelineRow } from "./placement.testsupport";
import { autoMark, mergePlaceRows, newestId, sourceOfPlace } from "./timeline";

const home = timelinePlace();
const nas = timelinePlace({ place: "offsite:t-nas", label: "NAS copy", kind: "target" });
const b2 = timelinePlace({ place: "offsite:t-b2", label: "B2", kind: "target", remote: true });
const hz = timelinePlace({ place: "offsite:t-hz", label: "Hetzner", kind: "target", remote: true, state: "unchecked" });

describe("mergePlaceRows", () => {
  it("joins a place's marks to the row with the same key whatever its time says", () => {
    const rows = [timelineRow("a1", "2026-09-18T03:00:00Z", timelineMark("local", "a1"))];
    const merged = mergePlaceRows(rows, "offsite:t-b2", [
      timelineRow("a1", "2026-09-18T05:00:00+02:00", timelineMark("offsite:t-b2", "b9")),
    ]);
    expect(merged).toHaveLength(1);
    expect(merged[0].places.map((m) => m.place)).toEqual(["local", "offsite:t-b2"]);
  });

  it("leaves one mark when the same place is loaded twice", () => {
    let rows = [timelineRow("a1", "2026-09-18T03:00:00Z", timelineMark("local", "a1"))];
    rows = mergePlaceRows(rows, "offsite:t-b2", [timelineRow("a1", "2026-09-18T03:00:00Z", timelineMark("offsite:t-b2", "b8"))]);
    rows = mergePlaceRows(rows, "offsite:t-b2", [timelineRow("a1", "2026-09-18T03:00:00Z", timelineMark("offsite:t-b2", "b9"))]);
    expect(rows[0].places).toEqual([timelineMark("local", "a1"), timelineMark("offsite:t-b2", "b9")]);
  });

  it("gives a backup found only at the place a row of its own, newest first", () => {
    const rows = [timelineRow("a1", "2026-09-18T03:00:00Z", timelineMark("local", "a1"))];
    const merged = mergePlaceRows(rows, "offsite:t-b2", [
      timelineRow("a7", "2026-09-10T03:00:00Z", timelineMark("offsite:t-b2", "b7")),
      timelineRow("a9", "2026-09-20T03:00:00Z", timelineMark("offsite:t-b2", "b9")),
    ]);
    expect(merged.map((r) => r.key)).toEqual(["a9", "a1", "a7"]);
  });

  it("drops a row whose only mark was at the place and is gone there", () => {
    const rows = [timelineRow("a7", "2026-09-10T03:00:00Z", timelineMark("offsite:t-b2", "b7"))];
    expect(mergePlaceRows(rows, "offsite:t-b2", [])).toEqual([]);
  });
});

describe("autoMark", () => {
  const row = timelineRow(
    "a1",
    "2026-09-18T03:00:00Z",
    timelineMark("offsite:t-b2", "b1"),
    timelineMark("offsite:t-nas", "n1"),
    timelineMark("local", "a1")
  );
  const places = [home, b2, nas, hz];

  it("takes the item's location first, then local targets, then remote ones", () => {
    expect(autoMark(row, places)?.place).toBe("local");
    expect(autoMark({ ...row, places: row.places.slice(0, 2) }, places)?.place).toBe("offsite:t-nas");
    expect(autoMark({ ...row, places: row.places.slice(0, 1) }, places)?.place).toBe("offsite:t-b2");
  });

  it("passes over an incomplete mark and a place that was not read", () => {
    const partial = timelineRow(
      "a1",
      "2026-09-18T03:00:00Z",
      { ...timelineMark("local", "a1"), incomplete: true },
      timelineMark("offsite:t-hz", "h1"),
      timelineMark("offsite:t-b2", "b1")
    );
    expect(autoMark(partial, places)?.place).toBe("offsite:t-b2");
  });

  it("finds nothing when no read place holds the whole backup", () => {
    const partial = timelineRow("a1", "2026-09-18T03:00:00Z", { ...timelineMark("local", "a1"), incomplete: true });
    expect(autoMark(partial, places)).toBeNull();
  });
});

describe("newestId and sourceOfPlace", () => {
  it("takes the first id of a mark and keeps offsite ids as sources", () => {
    expect(newestId(timelineMark("offsite:t-b2", "b9", "b8"))).toBe("b9");
    expect(sourceOfPlace("local")).toBe("local");
    expect(sourceOfPlace("offsite:t-b2")).toBe("offsite:t-b2");
  });
});
