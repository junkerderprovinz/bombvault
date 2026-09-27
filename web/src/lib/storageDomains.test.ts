import { describe, expect, it } from "vitest";
import type { DefaultImpact } from "./api";
import { countText, en, type TranslationKey } from "./i18n";
import type { CopiesPreview, HomePreview, PlaceRefusal } from "./places";
import { copiesExpect, homeExpect, impactLines } from "./storageDomains";

const t = (key: TranslationKey, n?: number) => countText(en[key], "en", n);

const impact: DefaultImpact = {
  dropped: [{ targetId: "t1", name: "B2", items: 3, snapshots: 40, unknown: false, uncheckable: [] }],
  added: [{ targetId: "t2", name: "NAS", items: 2, snapshots: 10, unknown: false, uncheckable: ["plex"] }],
  openTakeHome: 4,
  home: "",
  skip: ["t1"],
};

describe("the preview a domain-row write sends back", () => {
  it("is a Stored in preview without the answer's envelope", () => {
    const answer: PlaceRefusal & HomePreview = {
      ok: true,
      error: "",
      code: "",
      mode: "default",
      placeId: "p1",
      homePlace: "p0",
      homeHasBackups: true,
      repoId: "r1",
      creates: "repository",
      impact,
      backups: 0,
    };
    expect(homeExpect(answer)).toEqual({
      mode: "default",
      placeId: "p1",
      homePlace: "p0",
      homeHasBackups: true,
      repoId: "r1",
      creates: "repository",
      impact,
      backups: 0,
    });
  });

  it("is a chip's preview without the answer's envelope", () => {
    const answer: PlaceRefusal & CopiesPreview = {
      ok: true,
      placeId: "p2",
      on: false,
      targetId: "t1",
      skip: ["t1"],
      enabled: true,
      impact,
    };
    expect(JSON.parse(JSON.stringify(copiesExpect(answer)))).toEqual({
      placeId: "p2",
      on: false,
      targetId: "t1",
      skip: ["t1"],
      enabled: true,
      impact,
    });
  });
});

describe("impactLines", () => {
  it("names what each target stops and starts receiving, and who takes the new home", () => {
    expect(impactLines(t, "en", "containers", impact, "NAS Keller")).toEqual([
      "Items and project folders that B2 no longer gets: 3. Copies that stay there: 40.",
      "Items and project folders that NAS gets from now on: 2. Snapshots uploaded at the next run: at most 10.",
      "Could not be checked: plex",
      "Items without a location that take NAS Keller at their first backup: 4.",
    ]);
  });

  it("names project folders only for containers", () => {
    expect(impactLines(t, "en", "vms", impact, "NAS Keller").slice(0, 2)).toEqual([
      "Items that B2 no longer gets: 3. Copies that stay there: 40.",
      "Items that NAS gets from now on: 2. Snapshots uploaded at the next run: at most 10.",
    ]);
  });
});
