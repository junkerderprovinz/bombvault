// The search index is written by hand, so this holds it to the source: every
// card title a page draws, itself or through a card file in this folder, has
// to be in that page's entry. It reads source; it does not render.
import { existsSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { pageSource } from "./pageSources.testsupport";
import { SETTINGS_INDEX } from "./searchIndex";
import { SETTINGS_PAGES } from "./settingsPages";

const HERE = dirname(fileURLToPath(import.meta.url));
/** The keys passed as `title={t("…")}` to a Card in some source. */
function cardTitles(source: string): string[] {
  return [...source.matchAll(/<Card\b[^>]*?\btitle=\{t\("([^"]+)"\)/g)].map((m) => m[1]);
}

/** Card titles drawn by the card files a page mounts, such as <NotifyCard>. */
function mountedTitles(source: string): string[] {
  const out: string[] = [];
  for (const [, name] of source.matchAll(/<([A-Z][A-Za-z]*Card)\b/g)) {
    const file = join(HERE, `${name}.tsx`);
    if (existsSync(file)) out.push(...cardTitles(readFileSync(file, "utf8")));
  }
  return out;
}

describe("the settings search index", () => {
  it("has an entry for every page and none for a page that does not exist", () => {
    expect(Object.keys(SETTINGS_INDEX).sort()).toEqual(SETTINGS_PAGES.map((p) => p.id).sort());
  });

  it.each(SETTINGS_PAGES.map((p) => p.id))("knows every card the %s page draws", (page) => {
    const drawn = pageSource(page);
    expect(drawn, `nothing draws the ${page} page`).not.toBe("");
    const indexed = new Set(SETTINGS_INDEX[page].map((c) => c.title));
    const missing = [...new Set([...cardTitles(drawn), ...mountedTitles(drawn)])].filter((k) => !indexed.has(k as never));
    expect(missing).toEqual([]);
  });

  it("knows every keep rule of the retention grids with its (i)", () => {
    const grid = /\["\w+", "(settings\.retention\w+)", "(settings\.retention\w+Info)"\]/g;
    const targetSource = readFileSync(join(HERE, "..", "..", "components", "OffsiteTargetsSection.tsx"), "utf8");
    const targetRules = [...targetSource.matchAll(grid)].map(([, key, hint]) => ({ key, hint }));
    expect(targetRules.map((r) => r.key)).toContain("settings.retentionYearly");

    for (const card of SETTINGS_INDEX.storage.filter((c) => c.title === "offsite.copies.title")) {
      for (const rule of targetRules) expect(card.rows).toContainEqual(rule);
    }
  });

  it("has the compression choice on the off-site copies card and not on the repositories card", () => {
    const row = { key: "settings.compression", hint: "settings.compressionInfo" };
    expect(pageSource("storage")).toContain("<ReposCard");
    const copies = SETTINGS_INDEX.storage.find((c) => c.title === "offsite.copies.title");
    expect(copies?.rows).toContainEqual(row);
    expect(SETTINGS_INDEX.storage.find((c) => c.title === "repos.title")?.rows).not.toContainEqual(row);
  });

  it.each([["schedules", "IdleCard"]] as const)("has every caption and (i) of the card the %s page mounts as %s", (page, file) => {
    expect(pageSource(page)).toContain(`<${file}`);
    const source = readFileSync(join(HERE, `${file}.tsx`), "utf8");
    const [title] = cardTitles(source);
    const card = SETTINGS_INDEX[page].find((c) => c.title === title);
    expect(card, `${title} on ${page}`).toBeDefined();
    const rows = [
      ...source.matchAll(/key: "\w+", label: "([\w.]+)"(?:, hint: "([\w.]+)")?/g),
      ...source.matchAll(/label=\{t\("([\w.]+)"\)\}\s*hint=\{t\("([\w.]+)"\)\}/g),
      ...source.matchAll(/\{t\("([\w.]+)"\)\}\s*<InfoBubble tip=\{t\("([\w.]+)"\)\}/g),
    ].map(([, key, hint]) => (hint ? { key, hint } : { key }));
    expect(rows.length).toBeGreaterThanOrEqual(3);
    for (const row of rows) expect(card?.rows).toContainEqual(row);
  });

  it.each([
    ["ApiTokensCard", "ApiTokensCard"],
    ["ServicesCard", "HomeAssistantRows"],
    ["ServicesCard", "MdnsRow"],
  ])(
    "has every caption and (i) of the card the integrations page mounts as %s, from %s",
    (card, file) => {
      expect(pageSource("integrations")).toContain(`<${card}`);
      const holder = readFileSync(join(HERE, `${card}.tsx`), "utf8");
      if (file !== card) expect(holder).toContain(`<${file}`);
      const source = readFileSync(join(HERE, `${file}.tsx`), "utf8");
      const [title] = cardTitles(holder);
      const indexed = SETTINGS_INDEX.integrations.find((c) => c.title === title);
      expect(indexed, `${title} on integrations`).toBeDefined();
      // A bubble whose text is built in a variable leaves only its caption to compare.
      const rows = [
        ...source.matchAll(/<(?:ToggleRow|Toggle|SettingRow)\s+label=\{t\("([\w.]+)"\)\}(?:\s*hint=\{t\("([\w.]+)"\))?/g),
        ...source.matchAll(/\{t\("([\w.]+)"\)\}\s*(?:<\/label>\s*)?<InfoBubble tip=\{(?:t\(|tLtr\(t, )"([\w.]+)"\)/g),
        ...source.matchAll(/<label\b[^>]*>\s*\{t\("([\w.]+)"\)\}/g),
      ].map(([, key, hint]) => ({ key, hint }));
      expect(rows.length).toBeGreaterThanOrEqual(1);
      for (const { key, hint } of rows) {
        const row = indexed?.rows.find((r) => r.key === key);
        expect(row, `${key} on ${title}`).toBeDefined();
        if (hint) expect(row?.hint, `the (i) of ${key}`).toBe(hint);
      }
    }
  );

  it("indexes the retention card's pointer to the storage locations and its link", () => {
    const source = readFileSync(join(HERE, "sections", "RetentionCards.tsx"), "utf8");
    const rows = SETTINGS_INDEX.storage.find((c) => c.title === "storage.keep.title")?.rows ?? [];
    for (const key of ["settings.retentionWhere", "storage.rules.open"] as const) {
      expect(source).toContain(`"${key}"`);
      expect(rows).toContainEqual({ key });
    }
  });

  it("lists no card twice on one page", () => {
    for (const [page, cards] of Object.entries(SETTINGS_INDEX)) {
      const names = cards.map((c) => `${c.title}${JSON.stringify(c.vars ?? {})}`);
      expect(names.length, page).toBe(new Set(names).size);
    }
  });
});
