import { test } from "node:test";
import assert from "node:assert/strict";
import {
  LANGUAGES,
  DEFAULT_LANGUAGE,
  resources,
  type TranslationKey,
} from "../lib/i18n/locales/index";

const codes = LANGUAGES.map((l) => l.code);
const enKeys = Object.keys(resources.en).sort() as TranslationKey[];

test("English is the default and is registered", () => {
  assert.equal(DEFAULT_LANGUAGE, "en");
  assert.ok(codes.includes("en"));
});

test("every language has a non-empty label", () => {
  for (const lang of LANGUAGES) {
    assert.ok(lang.code.length >= 2, `code for ${lang.code}`);
    assert.ok(lang.label.trim().length > 0, `label for ${lang.code}`);
  }
});

test("every language has a non-empty flag code", () => {
  for (const lang of LANGUAGES) {
    assert.ok(
      typeof lang.flag === "string" && lang.flag.trim().length >= 2,
      `missing or empty flag for ${lang.code}`,
    );
  }
});

test("language codes are unique", () => {
  assert.equal(new Set(codes).size, codes.length);
});

test("RTL flag is only set for known RTL languages", () => {
  const rtl = LANGUAGES.filter((l) => l.rtl).map((l) => l.code).sort();
  for (const code of rtl) {
    assert.ok(["ar", "he", "fa", "ur"].includes(code), `${code} marked rtl`);
  }
});

test("every registered language has a resource bundle", () => {
  for (const code of codes) {
    assert.ok(resources[code], `missing resource bundle for ${code}`);
  }
});

test("every locale has exactly the English key set (no missing, no extra)", () => {
  for (const code of codes) {
    const keys = Object.keys(resources[code]).sort();
    assert.deepEqual(
      keys,
      enKeys,
      `locale "${code}" key set differs from en`,
    );
  }
});

test("no translation value is empty", () => {
  for (const code of codes) {
    for (const key of enKeys) {
      const value = resources[code][key];
      assert.ok(
        typeof value === "string" && value.trim().length > 0,
        `empty translation: ${code}.${key}`,
      );
    }
  }
});
