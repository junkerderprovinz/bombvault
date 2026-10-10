// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { collect } from "./displayPrefs";
import { readAskKeepLess, setAskKeepLess } from "./keepAsk";

beforeEach(() => {
  localStorage.clear();
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => ({ ok: true }) }));
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("asking before keeping less", () => {
  it("asks until someone switches the question off", () => {
    expect(readAskKeepLess()).toBe(true);
    setAskKeepLess(false);
    expect(readAskKeepLess()).toBe(false);
    setAskKeepLess(true);
    expect(readAskKeepLess()).toBe(true);
  });

  it("travels with the display preferences", () => {
    setAskKeepLess(false);

    expect(collect()["bombvault.askKeepLess"]).toBe("off");
    const put = vi.mocked(fetch).mock.calls.find(([url, init]) => url === "/api/display-prefs" && init?.method === "PUT");
    expect(JSON.parse(String(put?.[1]?.body))).toMatchObject({ "bombvault.askKeepLess": "off" });
  });
});
