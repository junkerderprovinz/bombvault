// A refused key deposit call names its reason by a code, and this map turns
// the code into a sentence. The codes live in Go, so the test reads them from
// there: one added on the server and forgotten here fails in this file.
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import type { DepositRefusal } from "./api";
import { DEPOSIT_CODE_KEY, depositSentence } from "./depositCodes";
import { de, en, type TranslationKey } from "./i18n";

const REPO = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..", "..");
const HANDLERS_GO = readFileSync(join(REPO, "internal", "api", "key_deposit.go"), "utf8");

function serverCodes(): string[] {
  const start = HANDLERS_GO.indexOf("var depositCodes = []struct {");
  expect(start, "depositCodes not found in internal/api/key_deposit.go").toBeGreaterThanOrEqual(0);
  const end = HANDLERS_GO.indexOf("\n}\n", HANDLERS_GO.indexOf("}{", start));
  return [...HANDLERS_GO.slice(start, end).matchAll(/"([a-z-]+)"\}/g)].map((m) => m[1]);
}

const t = (key: TranslationKey) => en[key];
const CODES = Object.keys(DEPOSIT_CODE_KEY) as DepositRefusal[];

describe("why a key deposit call was refused", () => {
  it("has a sentence for exactly the codes the server sends", () => {
    expect([...CODES].sort()).toEqual(serverCodes().sort());
  });

  it.each(CODES)("says %s in a sentence of its own, in English and in German", (code) => {
    const key = DEPOSIT_CODE_KEY[code];
    expect(depositSentence(t, { code, error: "the server's own words" })).toBe(en[key]);
    expect(en[key]).toMatch(/\.$/);
    expect(de[key]).toMatch(/\.$/);
    expect(de[key]).not.toBe(en[key]);
  });

  it("gives no two codes the same sentence", () => {
    expect(new Set(CODES.map((code) => en[DEPOSIT_CODE_KEY[code]])).size).toBe(CODES.length);
  });

  it("keeps the server's own words for a code this build does not know", () => {
    expect(depositSentence(t, { code: "deposit-someday", error: "the server's own words" })).toBe("the server's own words");
    expect(depositSentence(t, { error: "the server's own words" })).toBe("the server's own words");
  });

  it("says that it did not work when the server said nothing", () => {
    expect(depositSentence(t, {})).toBe(en["instances.role.saveError"]);
  });
});
