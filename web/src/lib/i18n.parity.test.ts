// ---------------------------------------------------------------------------
// Locale parity — the permanent regression guard for the 26-locale sweep.
//
// en is the source of truth. Every locale table in the registry must carry
// EXACTLY the en key set (zero missing, zero extra), and every value must use
// the same {placeholder} token set as its en counterpart — a locale that
// drops or renames a placeholder renders a literal "{name}" (or loses the
// value entirely) at runtime.
//
// Pure logic, node environment: importing ./i18n only builds the tables (all
// DOM access in that module lives inside functions).
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { de, en, LANGUAGES, locales } from "./i18n";

const EN_KEYS = Object.keys(en).sort();

/** Unique, sorted {placeholder} tokens in a translation value. */
function placeholderTokens(value: string): string[] {
  return [...new Set(value.match(/\{[a-zA-Z0-9_]+\}/g) ?? [])].sort();
}

describe("locale registry", () => {
  it("is wired to the inline source-of-truth tables", () => {
    expect(locales.en).toBe(en);
    expect(locales.de).toBe(de);
  });

  it("has exactly one table per offered language", () => {
    const offered = LANGUAGES.map((l) => l.code).sort();
    expect(Object.keys(locales).sort()).toEqual(offered);
  });

  it("en carries the full key set (sanity floor)", () => {
    expect(EN_KEYS.length).toBeGreaterThan(700);
  });
});

describe.each(Object.entries(locales))("locale %s", (code, table) => {
  const keys = Object.keys(table).sort();

  it("has exactly the en key set", () => {
    const keySet = new Set(keys);
    const missing = EN_KEYS.filter((k) => !keySet.has(k));
    const enSet = new Set(EN_KEYS);
    const extra = keys.filter((k) => !enSet.has(k));
    expect(
      { missing, extra },
      `locale "${code}" diverges from en: ` +
        `${missing.length} missing (${missing.slice(0, 10).join(", ")}${missing.length > 10 ? ", …" : ""}), ` +
        `${extra.length} extra (${extra.slice(0, 10).join(", ")}${extra.length > 10 ? ", …" : ""})`
    ).toEqual({ missing: [], extra: [] });
  });

  it("keeps every en {placeholder} token set", () => {
    const broken: string[] = [];
    for (const [key, enValue] of Object.entries(en)) {
      const want = placeholderTokens(enValue);
      if (want.length === 0) continue;
      const got = placeholderTokens(table[key as keyof typeof en] ?? "");
      if (got.join("|") !== want.join("|")) {
        broken.push(`${key}: want [${want.join(" ")}], got [${got.join(" ")}]`);
      }
    }
    expect(
      broken,
      `locale "${code}" has ${broken.length} value(s) with divergent placeholders:\n  ` +
        broken.slice(0, 10).join("\n  ")
    ).toEqual([]);
  });
});
