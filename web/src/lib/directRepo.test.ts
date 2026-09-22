import { describe, expect, it } from "vitest";
import type { NamedRepo, OffsiteTarget } from "./api";
import { directAsk, directUse, itemsText, primaryDirects, retentionLowered } from "./directRepo";
import { en, type TranslationKey } from "./i18n";

const t = (key: TranslationKey) => en[key];

function target(over: Partial<OffsiteTarget>): OffsiteTarget {
  return {
    id: "t-b2", domain: "containers", name: "B2", repo: "b2:bkt:containers", credsRef: "", storageClass: "",
    immutable: false, schedule: "", retentionKeepLast: 0, retentionKeepDaily: 0, retentionKeepWeekly: 0,
    retentionKeepMonthly: 0, limitUpload: 0, limitDownload: 0, growthBudgetGb: 0, enabled: true, createdAt: 1,
    sortOrder: 1, ...over,
  };
}

function repo(over: Partial<NamedRepo>): NamedRepo {
  return {
    id: "d1", name: "B2 direct", repo: "b2:bkt:containers-direct", credsRef: "", storageClass: "", limitUpload: 0,
    limitDownload: 0, immutable: false, enabled: true, offPremises: false, inUse: 0, companionOf: "t-b2", companionLost: false, ...over,
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

describe("direct repositories in use", () => {
  it("count only while an item backs up there, an unknown count included", () => {
    expect(directUse(target({}), [repo({ inUse: 0 })])).toBeUndefined();
    expect(directUse(target({}), [repo({ inUse: -1 })])?.repo.id).toBe("d1");
    expect(directUse(target({}), [repo({ inUse: 2, companionOf: "other" })])).toBeUndefined();
  });

  it("add up their items and say when a count is unknown", () => {
    expect(itemsText([repo({ inUse: 2 }), repo({ inUse: 3 })])).toBe("5");
    expect(itemsText([repo({ inUse: 2 }), repo({ inUse: -1 })])).toBe("?");
  });

  it("name every target in one question", () => {
    const uses = [
      { target: target({}), repo: repo({ inUse: 2 }) },
      { target: target({ id: "t-h", name: "Hetzner" }), repo: repo({ id: "d2", inUse: 1, companionOf: "t-h" }) },
    ];
    expect(directAsk(t, "en", "offsite.directAppendOnlyAsk", uses)).toBe(
      "Items whose only copy is in B2 and Hetzner direct: 3. Without append-only this box may delete from it. Save anyway?"
    );
  });

  it("find the field targets whose direct repository is used", () => {
    const field = target({ id: "t-f", sortOrder: 0 });
    const extra = target({ id: "t-x", sortOrder: 1 });
    const repos = [repo({ companionOf: "t-f", inUse: 1 }), repo({ id: "d2", companionOf: "t-x", inUse: 4 })];
    expect(primaryDirects([field, extra], repos).map((u) => u.target.id)).toEqual(["t-f"]);
  });
});
