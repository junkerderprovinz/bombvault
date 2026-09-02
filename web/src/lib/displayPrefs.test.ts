// @vitest-environment jsdom
// The look of the interface lives on the server so it survives a browser
// (issue #191). Two of the three paths here are the ones that can go wrong in a
// way the user notices: seeding on upgrade, and the reload that adopts a stored
// look without turning into a loop.

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { collect, save, sync } from "./displayPrefs";

const RELOAD_GUARD = "bv-display-prefs-reloaded";

function antwort(body: unknown, ok = true): Response {
  return { ok, json: async () => body } as unknown as Response;
}

let reloads = 0;

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  reloads = 0;
  // jsdom refuses to navigate; replace the whole location so a reload is
  // countable instead of throwing.
  Object.defineProperty(window, "location", {
    configurable: true,
    value: { reload: () => { reloads += 1; } },
  });
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("collect", () => {
  it("takes the look and leaves the workspace alone", () => {
    localStorage.setItem("bv-theme", "light");
    localStorage.setItem("bombvault.advanced", "1");
    // Not a look: which filter was open on the Containers page is about what
    // you were doing, and syncing it would move someone else's filter under
    // your cursor on another machine.
    localStorage.setItem("bv-containers-sort", "name");
    localStorage.setItem("bv-password", "shown");

    const got = collect();
    expect(got["bv-theme"]).toBe("light");
    expect(got["bombvault.advanced"]).toBe("1");
    expect(got["bv-containers-sort"]).toBeUndefined();
    expect(got["bv-password"]).toBeUndefined();
  });

  it("omits keys this browser never set", () => {
    localStorage.setItem("bv-theme", "dark");
    const got = collect();
    expect(Object.keys(got)).toEqual(["bv-theme"]);
  });
});

describe("sync", () => {
  it("seeds the server from this browser when the server has nothing", async () => {
    // The upgrade path. Anyone who already had a look must keep it, rather than
    // being reset to factory settings by the release that moved storage.
    localStorage.setItem("bv-theme", "light");
    localStorage.setItem("bv-accent", "#1D99F3");
    const fetchMock = vi.fn().mockResolvedValue(antwort({ ok: true, prefs: {}, stored: false }));
    vi.stubGlobal("fetch", fetchMock);

    await sync();

    const put = fetchMock.mock.calls.find((c) => c[1]?.method === "PUT");
    expect(put, "a server with nothing stored must be seeded").toBeTruthy();
    expect(JSON.parse(put![1].body as string)).toMatchObject({
      "bv-theme": "light",
      "bv-accent": "#1D99F3",
    });
    expect(reloads).toBe(0);
  });

  it("adopts a stored look and reloads once", async () => {
    localStorage.setItem("bv-theme", "dark");
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(
      antwort({ ok: true, stored: true, prefs: { "bv-theme": "light", "bv-lang": "fr" } })
    ));

    await sync();

    expect(localStorage.getItem("bv-theme")).toBe("light");
    expect(localStorage.getItem("bv-lang")).toBe("fr");
    expect(reloads).toBe(1);
  });

  it("never reloads twice, even if the server keeps disagreeing", async () => {
    // The failure this guard exists for: a value the server returns and the
    // browser cannot keep would otherwise reload the page forever.
    localStorage.setItem("bv-theme", "dark");
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(
      antwort({ ok: true, stored: true, prefs: { "bv-theme": "light" } })
    ));

    await sync();
    localStorage.setItem("bv-theme", "dark"); // pretend the write did not stick
    await sync();

    expect(reloads).toBe(1);
    expect(sessionStorage.getItem(RELOAD_GUARD)).toBe("1");
  });

  it("does nothing when the browser and the server already agree", async () => {
    localStorage.setItem("bv-theme", "light");
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(
      antwort({ ok: true, stored: true, prefs: { "bv-theme": "light" } })
    ));

    await sync();

    expect(reloads).toBe(0);
  });

  it("leaves the cached look alone when the server cannot be reached", async () => {
    // Offline is the old behaviour, not a reset: the browser cache IS the look
    // until the server can be asked again.
    localStorage.setItem("bv-theme", "light");
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("offline")));

    await expect(sync()).resolves.toBeUndefined();
    expect(localStorage.getItem("bv-theme")).toBe("light");
    expect(reloads).toBe(0);
  });

  it("ignores keys it does not own", async () => {
    // The column is written by this client, but a hand-edited value must not be
    // able to put arbitrary keys into a visitor's browser storage.
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(
      antwort({ ok: true, stored: true, prefs: { "bv-theme": "light", "evil-key": "x" } })
    ));

    await sync();

    expect(localStorage.getItem("bv-theme")).toBe("light");
    expect(localStorage.getItem("evil-key")).toBeNull();
  });
});

describe("save", () => {
  it("sends the whole look as one object", () => {
    localStorage.setItem("bv-theme", "light");
    localStorage.setItem("bv-motion", "subtle");
    const fetchMock = vi.fn().mockResolvedValue(antwort({ ok: true }));
    vi.stubGlobal("fetch", fetchMock);

    save();

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/display-prefs");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body as string)).toEqual({
      "bv-theme": "light",
      "bv-motion": "subtle",
    });
  });

  it("does not throw when the server is unreachable", () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("offline")));
    expect(() => save()).not.toThrow();
  });
});
