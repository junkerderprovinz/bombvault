import { describe, expect, it } from "vitest";
import type { OffsiteTarget } from "./api";
import { retentionLowered } from "./directRepo";

function target(over: Partial<OffsiteTarget>): OffsiteTarget {
  return {
    id: "t-b2", domain: "containers", name: "B2", repo: "b2:bkt:containers", credsRef: "", storageClass: "",
    immutable: false, schedule: "", retentionKeepLast: 0, retentionKeepDaily: 0, retentionKeepWeekly: 0,
    retentionKeepMonthly: 0, limitUpload: 0, limitDownload: 0, growthBudgetGb: 0, enabled: true, createdAt: 1,
    sortOrder: 1, ...over,
  };
}

describe("retentionLowered", () => {
  const r = (last: number, daily: number) => target({ retentionKeepLast: last, retentionKeepDaily: daily });
  it.each([
    ["everything to a count", r(0, 0), r(5, 0), true],
    ["a count to everything", r(5, 0), r(0, 0), false],
    ["a smaller count", r(7, 0), r(3, 0), true],
    ["a larger count", r(7, 0), r(9, 0), false],
    ["a second dimension added", r(7, 0), r(7, 3), false],
    ["a dimension dropped", r(7, 3), r(7, 0), true],
  ])("%s", (_name, before, after, want) => {
    expect(retentionLowered(before, after)).toBe(want);
  });
});
