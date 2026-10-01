import { describe, expect, it } from "vitest";
import { en } from "../../lib/i18n";
import { buildItems, fold, search } from "./SettingsSearch";
import { SETTINGS_PAGES } from "./settingsPages";

const item = (name: string, prose = "", tier = 2) => ({ foldedName: fold(name), foldedProse: fold(prose), tier, name });

describe("fold", () => {
  it("drops case and accents, so a plain keyboard finds umlauts", () => {
    expect(fold("Zeitpläne")).toBe("zeitplane");
    expect(fold("Größe")).toBe("grosse");
  });
});

describe("search", () => {
  const items = [
    item("Backup CPU"),
    item("Snapshot retention", "How long backups are kept"),
    item("Retention", "", 0),
    item("Off-site retention"),
  ];

  it("finds nothing for an empty query", () => {
    expect(search(items, "  ")).toEqual([]);
  });

  it("wants every word of the query", () => {
    expect(search(items, "snapshot keep").map((h) => h.item.name)).toEqual([]);
    expect(search(items, "snapshot kept").map((h) => h.item.name)).toEqual(["Snapshot retention"]);
  });

  it("ranks a name that starts with the word above one that only contains it", () => {
    expect(search(items, "reten")[0].item.name).toBe("Retention");
  });

  it("ranks every name match above a match in an explanation", () => {
    const prose = search([item("Snapshot retention", "backups are kept"), item("Kept copies")], "kept");
    expect(prose.map((h) => [h.item.name, h.inHint])).toEqual([
      ["Kept copies", false],
      ["Snapshot retention", true],
    ]);
  });
});

describe("the settings index in English", () => {
  const items = buildItems((k) => en[k], SETTINGS_PAGES);
  const where = (query: string) =>
    search(items, query)
      .filter((h) => !h.inHint)
      .map((h) => `${h.item.page}: ${h.item.cardName ?? ""}`);

  it("leads the yearly rule to both policies on the Retention page and to each off-site destination", () => {
    const hits = where("keep yearly");
    expect(hits).toContain(`retention: ${en["settings.retentionTitle"]}`);
    expect(hits).toContain(`retention: ${en["settings.retentionOffsiteTitle"]}`);
    expect(hits.filter((h) => h.startsWith("offsite: "))).toHaveLength(6);
    expect(hits.every((h) => h.startsWith("retention: ") || h.startsWith("offsite: "))).toBe(true);
  });

  it("leads compression to the backup paths, the named repositories and each off-site destination", () => {
    const hits = where("compression");
    expect(hits).toContain(`storage: ${en["settings.paths"]}`);
    expect(hits).toContain(`storage: ${en["repos.title"]}`);
    expect(hits.filter((h) => h.startsWith("offsite: "))).toHaveLength(6);
  });

  it("finds a row by the words behind its (i)", () => {
    const hit = search(items, "calendar years").find((h) => h.item.page === "retention");
    expect(hit?.inHint).toBe(true);
    expect(hit?.item.name).toBe(en["settings.retentionYearly"]);
  });
});
