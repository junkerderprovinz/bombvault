// The search index is written by hand, so this holds it to the source: every
// card title a page draws in Settings.tsx, directly or through a card file in
// this folder, has to be in that page's entry. It reads source; it does not
// render.
import { existsSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { SETTINGS_INDEX } from "./searchIndex";
import { SETTINGS_PAGES } from "./settingsPages";

const HERE = dirname(fileURLToPath(import.meta.url));
const PAGE_SOURCE = readFileSync(join(HERE, "..", "Settings.tsx"), "utf8");

/** Every `{page === "X" && …}` block of Settings.tsx, brace-matched to its close. */
function region(page: string): string {
  const needle = `{page === "${page}" &&`;
  let out = "";
  for (let i = PAGE_SOURCE.indexOf(needle); i !== -1; i = PAGE_SOURCE.indexOf(needle, i + 1)) {
    let depth = 0;
    for (let j = i; j < PAGE_SOURCE.length; j++) {
      if (PAGE_SOURCE[j] === "{") depth++;
      else if (PAGE_SOURCE[j] === "}" && --depth === 0) {
        out += PAGE_SOURCE.slice(i, j + 1);
        break;
      }
    }
  }
  return out;
}

/** The keys passed as `title={t("…")}` to a Card in some source. */
function cardTitles(source: string): string[] {
  return [...source.matchAll(/<Card\b[^>]*?\btitle=\{t\("([^"]+)"\)/g)].map((m) => m[1]);
}

/** Card titles drawn by the card files a region mounts, such as <NotifyCard>. */
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
    const drawn = region(page);
    expect(drawn, `no {page === "${page}"} gate in Settings.tsx`).not.toBe("");
    const indexed = new Set(SETTINGS_INDEX[page].map((c) => c.title));
    const missing = [...new Set([...cardTitles(drawn), ...mountedTitles(drawn)])].filter((k) => !indexed.has(k as never));
    expect(missing).toEqual([]);
  });

  it("knows every keep rule of the retention grids with its (i)", () => {
    const grid = /\["\w+", "(settings\.retention\w+)", "(settings\.retention\w+Info)"\]/g;
    expect(region("retention")).toContain("<RetentionRulesCard");
    const sectionSource = readFileSync(join(HERE, "OwnRetentionCard.tsx"), "utf8");
    const pageRules = [...sectionSource.matchAll(grid)].map(([, key, hint]) => ({ key, hint }));
    const targetSource = readFileSync(join(HERE, "..", "..", "components", "OffsiteTargetsSection.tsx"), "utf8");
    const targetRules = [...targetSource.matchAll(grid)].map(([, key, hint]) => ({ key, hint }));
    expect(pageRules.map((r) => r.key)).toContain("settings.retentionYearly");
    expect(targetRules.map((r) => r.key)).toContain("settings.retentionYearly");

    const sections = SETTINGS_INDEX.retention.filter(
      (c) => c.title === "settings.retentionLocalTitle" || c.title === "settings.retentionOffsiteTitle"
    );
    expect(sections).toHaveLength(2);
    for (const section of sections) {
      for (const rule of pageRules) expect(section.rows).toContainEqual(rule);
    }
    for (const card of SETTINGS_INDEX.offsite.filter((c) => c.title === "offsite.copyDomainTitle")) {
      for (const rule of targetRules) expect(card.rows).toContainEqual(rule);
    }
  });

  it("has the compression choice on every card that draws one", () => {
    const row = { key: "settings.compression", hint: "settings.compressionInfo" };
    const storage = region("storage");
    expect(storage).toContain("<PathModeSwitch");
    expect(storage).toContain("<ReposCard");
    const cards = [
      ...SETTINGS_INDEX.storage.filter((c) => c.title === "settings.paths" || c.title === "repos.title"),
      ...SETTINGS_INDEX.offsite.filter((c) => c.title === "offsite.copyDomainTitle"),
    ];
    expect(cards).toHaveLength(8);
    for (const card of cards) expect(card.rows).toContainEqual(row);
  });

  it.each([
    ["schedules", "IdleCard"],
    ["offsite", "StreamingCard"],
  ] as const)("has every caption and (i) of the card the %s page mounts as %s", (page, file) => {
    expect(region(page)).toContain(`<${file}`);
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

  it.each(["ApiTokensCard", "HomeAssistantCard", "NetworkCard"])(
    "has every caption and (i) of the card the integrations page mounts as %s",
    (file) => {
      expect(region("integrations")).toContain(`<${file}`);
      const source = readFileSync(join(HERE, `${file}.tsx`), "utf8");
      const [title] = cardTitles(source);
      const card = SETTINGS_INDEX.integrations.find((c) => c.title === title);
      expect(card, `${title} on integrations`).toBeDefined();
      // A bubble whose text is built in a variable leaves only its caption to compare.
      const rows = [
        ...source.matchAll(/<(?:ToggleRow|Toggle)\s+label=\{t\("([\w.]+)"\)\}(?:\s*hint=\{t\("([\w.]+)"\))?/g),
        ...source.matchAll(/\{t\("([\w.]+)"\)\}\s*(?:<\/label>\s*)?<InfoBubble tip=\{(?:t\(|tLtr\(t, )"([\w.]+)"\)/g),
        ...source.matchAll(/<label\b[^>]*>\s*\{t\("([\w.]+)"\)\}/g),
      ].map(([, key, hint]) => ({ key, hint }));
      expect(rows.length).toBeGreaterThanOrEqual(1);
      for (const { key, hint } of rows) {
        const row = card?.rows.find((r) => r.key === key);
        expect(row, `${key} on ${title}`).toBeDefined();
        if (hint) expect(row?.hint, `the (i) of ${key}`).toBe(hint);
      }
    }
  );

  it("has every caption of the placement defaults card, its row of buttons and its fields", () => {
    expect(region("storage")).toContain("<PlacementDefaultsCard");
    const card = SETTINGS_INDEX.storage.find((c) => c.title === "placementDefaults.title");
    expect(card, "placementDefaults.title on storage").toBeDefined();
    expect(card?.hint).toBe("placementDefaults.hint");
    const sources = [
      readFileSync(join(HERE, "PlacementDefaultsCard.tsx"), "utf8"),
      readFileSync(join(HERE, "..", "..", "components", "placement", "PlacementBar.tsx"), "utf8"),
    ];
    const keys = sources.flatMap((source) => [
      ...[...source.matchAll(/label=\{t\("([\w.]+)"\)\}/g)].map(([, key]) => key),
      ...[...source.matchAll(/\blabel: t\("([\w.]+)"\)/g)].map(([, key]) => key),
    ]);
    expect(keys).toContain("placement.segLocal");
    expect(keys.length).toBeGreaterThanOrEqual(5);
    for (const key of keys) expect(card?.rows.map((r) => r.key), key).toContain(key);
  });

  it("has the off-premises switch of the repositories card with its (i)", () => {
    const source = readFileSync(join(HERE, "ReposCard.tsx"), "utf8");
    const rows = [...source.matchAll(/\{t\("([\w.]+)"\)\}\s*<InfoBubble tip=\{t\("([\w.]+)"\)\}/g)].map(([, key, hint]) => ({ key, hint }));
    expect(rows).toContainEqual({ key: "repos.offPremises", hint: "repos.offPremisesHint" });
    const card = SETTINGS_INDEX.storage.find((c) => c.title === "repos.title");
    for (const row of rows) expect(card?.rows).toContainEqual(row);
  });

  it("indexes each switch and the extra-targets line of the retention cards with its (i)", () => {
    const source = readFileSync(join(HERE, "OwnRetentionCard.tsx"), "utf8");
    const rows = (title: string) => SETTINGS_INDEX.retention.find((c) => c.title === title)?.rows ?? [];
    for (const [section, key, hint] of [
      ["settings.retentionLocalTitle", "settings.ownRetention", "settings.ownRetentionToggleHint"],
      ["settings.retentionOffsiteTitle", "settings.ownOffsiteRetention", "settings.ownOffsiteRetentionToggleHint"],
    ] as const) {
      expect(source).toContain(`"${hint}"`);
      expect(rows(section)).toContainEqual({ key, hint });
    }
    expect(source).toContain('"settings.retentionExtraTargets"');
    expect(rows("settings.retentionOffsiteTitle")).toContainEqual({ key: "settings.retentionExtraTargets" });
    for (const [card, hint] of [
      ["settings.retentionLocalTitle", "settings.ownRetentionToggleHint"],
      ["settings.retentionOffsiteTitle", "settings.ownOffsiteRetentionToggleHint"],
    ] as const) {
      for (const key of ["nav.containers", "nav.vms", "nav.flash", "nav.files", "nav.zfs", "nav.config"] as const) {
        expect(source).toContain(`"${key}"`);
        expect(rows(card)).toContainEqual({ key, hint });
      }
    }
  });

  it("lists no card twice on one page", () => {
    for (const [page, cards] of Object.entries(SETTINGS_INDEX)) {
      const names = cards.map((c) => `${c.title}${JSON.stringify(c.vars ?? {})}`);
      expect(names.length, page).toBe(new Set(names).size);
    }
  });
});
