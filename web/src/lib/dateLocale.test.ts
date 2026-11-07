// Dates take their words from the page's language and their order and clock
// from the reader's region, which the browser's formatting locale carries.
import { describe, expect, it } from "vitest";
import { dateLocale } from "./dateLocale";

const NOV_7 = new Date(2026, 10, 7, 17, 5);
const shortStamp = (locale: string) =>
  NOV_7.toLocaleString(locale, { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });

describe("dateLocale", () => {
  it("writes an English page read in Portugal day first, on a 24-hour clock", () => {
    expect(dateLocale("en", "pt-PT")).toBe("en-PT");
    expect(shortStamp(dateLocale("en", "pt-PT"))).toBe("07/11, 17:05");
  });

  it("follows a British English browser", () => {
    expect(dateLocale("en", "en-GB")).toBe("en-GB");
  });

  it("keeps the American order for a reader in the United States", () => {
    expect(shortStamp(dateLocale("en", "en-US"))).toBe("11/07, 05:05 PM");
  });

  it("keeps the language alone where Intl has no data for the pairing", () => {
    expect(dateLocale("de", "en-US")).toBe("de");
  });

  it("keeps the language alone when the browser names no region", () => {
    expect(dateLocale("en", "en")).toBe("en");
  });

  it("keeps the 24-hour clock a reader chose over the region's own", () => {
    expect(shortStamp(dateLocale("en", "en-US-u-hc-h23"))).toBe("11/07, 17:05");
  });
});
