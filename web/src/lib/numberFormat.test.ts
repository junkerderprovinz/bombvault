// Numbers and sizes follow the language the page is shown in, not the
// browser's own locale.
import { afterEach, describe, expect, it, vi } from "vitest";
import { humanBytes } from "./forecast";
import { countText } from "./i18n";
import { formatNumber, oneDecimal } from "./numberFormat";

function pageIn(lang: string) {
  vi.stubGlobal("document", { documentElement: { lang } });
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("numbers in the page's language", () => {
  it("groups thousands and writes decimals the English way", () => {
    pageIn("en");
    expect(formatNumber(1234)).toBe("1,234");
    expect(oneDecimal(1.25)).toBe("1.3");
    expect(humanBytes(1.2 * 1024 ** 3)).toBe("1.2 GB");
  });

  it("groups thousands and writes decimals the German way", () => {
    pageIn("de");
    expect(formatNumber(1234)).toBe("1.234");
    expect(oneDecimal(2)).toBe("2,0");
    expect(humanBytes(1.2 * 1024 ** 3)).toBe("1,2 GB");
    expect(humanBytes(512)).toBe("512 B");
  });

  it("falls back to English before a language is set", () => {
    pageIn("");
    expect(humanBytes(1536)).toBe("1.5 KB");
  });

  it("puts a count into a sentence in the sentence's language", () => {
    expect(countText("one={n} Datei|other={n} Dateien", "de", 4180)).toBe("4.180 Dateien");
    expect(countText("one={n} file|other={n} files", "en", 4180)).toBe("4,180 files");
  });
});
