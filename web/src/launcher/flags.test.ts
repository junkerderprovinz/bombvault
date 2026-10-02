import { describe, expect, it } from "vitest";
import { LANGUAGES } from "../lib/i18n";
import { flagUrl } from "./flags";

describe("flagUrl", () => {
  it("has a flag for every language the app speaks", () => {
    for (const l of LANGUAGES) expect(flagUrl(l.flag), l.code).toBeTruthy();
  });
});
