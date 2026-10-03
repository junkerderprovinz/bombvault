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

  it("leads the yearly rule to both sections of the Retention card and to each off-site destination", () => {
    const hits = where("keep yearly");
    expect(hits).toContain(`retention: ${en["source.local"]}`);
    expect(hits).toContain(`retention: ${en["source.offsite"]}`);
    expect(hits.filter((h) => h.startsWith("offsite: "))).toHaveLength(6);
    expect(hits.every((h) => h.startsWith("retention: ") || h.startsWith("offsite: "))).toBe(true);
  });

  it("leads compression to the backup paths, the named repositories and each off-site destination", () => {
    const hits = where("compression");
    expect(hits).toContain(`storage: ${en["settings.paths"]}`);
    expect(hits).toContain(`storage: ${en["repos.title"]}`);
    expect(hits.filter((h) => h.startsWith("offsite: "))).toHaveLength(6);
  });

  it("leads each source's own rules to their section of the Retention card", () => {
    expect(where("own keep rules")).toContain(`retention: ${en["source.local"]}`);
    const offsite = where("own off-site keep rules");
    expect(offsite).toContain(`retention: ${en["source.offsite"]}`);
    expect(offsite).not.toContain(`retention: ${en["source.local"]}`);
    expect(where("additional off-site targets keep")).toEqual([`retention: ${en["source.offsite"]}`]);
  });

  it("finds a row by the words behind its (i)", () => {
    const hit = search(items, "calendar years").find((h) => h.item.page === "retention");
    expect(hit?.inHint).toBe(true);
    expect(hit?.item.name).toBe(en["settings.retentionYearly"]);
  });

  it("leads the idle limits to the Schedules page", () => {
    const top = search(items, "idle before backup")[0];
    expect(top.item.page).toBe("schedules");
    expect(top.item.name).toBe(en["idle.title"]);
    expect(where("traffic below")).toEqual([`schedules: ${en["idle.title"]}`]);
    expect(where("for at least")).toContain(`schedules: ${en["idle.title"]}`);
  });

  it("leads Streaming first and its limits to the Off-site page", () => {
    const top = search(items, "streaming first")[0];
    expect(top.item.page).toBe("offsite");
    expect(top.item.name).toBe(en["streaming.title"]);
    expect(where("upload limit while streaming")).toEqual([`offsite: ${en["streaming.title"]}`]);
    expect(where("media servers")).toContain(`offsite: ${en["streaming.title"]}`);
    expect(where("back to normal")).toContain(`offsite: ${en["streaming.title"]}`);
  });

  it("finds the idle and streaming rows by the words behind their (i)", () => {
    const cpu = search(items, "docker stats").find((h) => h.item.page === "schedules");
    expect(cpu?.inHint).toBe(true);
    expect(cpu?.item.name).toBe(en["idle.cpu"]);
    const servers = search(items, "image name").filter((h) => h.item.page === "offsite");
    expect(servers.map((h) => h.item.name)).toContain(en["streaming.servers"]);
    expect(servers.every((h) => h.inHint)).toBe(true);
  });
});
