import { describe, expect, it } from "vitest";
import { fold, search } from "./SettingsSearch";

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
