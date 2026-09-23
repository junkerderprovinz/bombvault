import { describe, expect, it } from "vitest";
import { countText, en } from "./i18n";
import { allLocales as locales } from "./localesForTests";

const CATEGORIES = ["zero", "one", "two", "few", "many", "other"];

/** The labelled forms of a value, or null when it offers only one. */
function forms(value: string): Map<string, string> | null {
  const out = new Map<string, string>();
  for (const part of value.split("|")) {
    const at = part.indexOf("=");
    if (at <= 0 || !CATEGORIES.includes(part.slice(0, at))) return null;
    out.set(part.slice(0, at), part.slice(at + 1));
  }
  return out.size > 0 ? out : null;
}

describe("countText", () => {
  const backups = "one={n} backup|other={n} backups";

  it("takes the form the count needs", () => {
    expect(countText(backups, "en", 1)).toBe("1 backup");
    expect(countText(backups, "en", 2)).toBe("2 backups");
  });

  it("follows the language's own rule, not English's", () => {
    const ru = "one={n} копия|few={n} копии|many={n} копий";
    expect(countText(ru, "ru", 1)).toBe("1 копия");
    expect(countText(ru, "ru", 2)).toBe("2 копии");
    expect(countText(ru, "ru", 5)).toBe("5 копий");
    expect(countText(ru, "ru", 21)).toBe("21 копия");
    expect(countText(ru, "ru", 11)).toBe("11 копий");
  });

  it("falls back to the plain form for a category the value leaves out", () => {
    expect(countText(backups, "ru", 3)).toBe("3 backups");
  });

  it("puts the count into a value that offers one form", () => {
    expect(countText("{n} 個のバックアップ", "ja", 3)).toBe("3 個のバックアップ");
  });

  it("leaves a sentence that merely contains = or | alone", () => {
    const sentence = "Set PUID=99 | PGID=100 for {n} containers";
    expect(countText(sentence, "en", 2)).toBe("Set PUID=99 | PGID=100 for 2 containers");
  });

  it("answers without a count with the plain form and an untouched placeholder", () => {
    expect(countText(backups, "en")).toBe("{n} backups");
  });
});

describe("plural forms in the tables", () => {
  const labelled = Object.keys(en).filter((k) => forms(en[k as keyof typeof en]) !== null);

  it("are declared on the strings that carry a count", () => {
    expect(labelled).toContain("takeover.backups");
    expect(labelled).toContain("time.minutesAgo");
  });

  it.each(Object.entries(locales))("locale %s offers only categories it uses", (code, table) => {
    const allowed = new Set(new Intl.PluralRules(code).resolvedOptions().pluralCategories);
    for (const [key, value] of Object.entries(table)) {
      const f = forms(value as string);
      if (!f) continue;
      const stray = [...f.keys()].filter((c) => !allowed.has(c as Intl.LDMLPluralRule));
      expect(stray, `${code} "${key}" labels ${stray.join(", ")}, which ${code} never selects`).toEqual([]);
      expect(f.has("other"), `${code} "${key}" has no other form to fall back on`).toBe(true);
    }
  });
});
