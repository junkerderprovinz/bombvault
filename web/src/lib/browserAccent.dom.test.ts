// @vitest-environment jsdom
// Only Firefox and Safari fill AccentColor with the system's colour, so every
// Chromium browser counts as no source.
import { afterEach, describe, expect, it, vi } from "vitest";
import { browserAccentWorks } from "./browserAccent";

const FIREFOX = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:149.0) Gecko/20100101 Firefox/149.0";
const CHROME = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36";
const HEADLESS = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/146.0.0.0 Safari/537.36";

function browser(agent: string, supports: boolean) {
  vi.stubGlobal("CSS", { supports: () => supports });
  vi.spyOn(navigator, "userAgent", "get").mockReturnValue(agent);
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("the browser as a source", () => {
  it("counts in Firefox", () => {
    browser(FIREFOX, true);
    expect(browserAccentWorks()).toBe(true);
  });

  it.each([CHROME, HEADLESS])("does not count in Chromium: %s", (agent) => {
    browser(agent, true);
    expect(browserAccentWorks()).toBe(false);
  });

  it("does not count where AccentColor is unknown", () => {
    browser(FIREFOX, false);
    expect(browserAccentWorks()).toBe(false);
  });
});
