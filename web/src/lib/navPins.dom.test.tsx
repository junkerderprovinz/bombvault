// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, renderHook } from "@testing-library/react";
import { ADOPTED_EVENT, collect } from "./displayPrefs";
import { readNavPins, setNavPin, subscribeNavPins, useNavPins } from "./navPins";

const KEY = "bombvault.navPins";
const off = { receiverEnabled: false, fleetEnabled: false, pullEnabled: false };

let fetchMock: ReturnType<typeof vi.fn>;

beforeEach(() => {
  localStorage.clear();
  fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ ok: true }) });
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("defaults", () => {
  it("pins Storage locations and leaves Instances out on a fresh install", () => {
    expect(readNavPins(off)).toEqual({ storage: true, instances: false });
  });

  it.each(["receiverEnabled", "fleetEnabled", "pullEnabled"] as const)(
    "pins Instances for an install that has %s on",
    (module) => {
      expect(readNavPins({ ...off, [module]: true })).toEqual({ storage: true, instances: true });
    },
  );

  it("leaves Instances out until the settings are loaded", () => {
    expect(readNavPins(null)).toEqual({ storage: true, instances: false });
  });

  it("stores nothing by reading", () => {
    readNavPins({ ...off, fleetEnabled: true });
    expect(localStorage.getItem(KEY)).toBeNull();
    expect(collect()[KEY]).toBeUndefined();
  });

  it("falls back to the defaults for a stored value it cannot read", () => {
    for (const stored of ["not json", "[true]", '"storage"', '{"storage":"no","instances":1}']) {
      localStorage.setItem(KEY, stored);
      expect(readNavPins({ ...off, pullEnabled: true })).toEqual({ storage: true, instances: true });
    }
  });
});

describe("changing a pin", () => {
  it("stores only the page that was changed", () => {
    setNavPin("storage", false);
    expect(JSON.parse(localStorage.getItem(KEY)!)).toEqual({ storage: false });
    expect(readNavPins(off)).toEqual({ storage: false, instances: false });
    expect(readNavPins({ ...off, receiverEnabled: true })).toEqual({ storage: false, instances: true });
  });

  it("keeps an earlier choice when the other page changes", () => {
    setNavPin("storage", false);
    setNavPin("instances", true);
    expect(JSON.parse(localStorage.getItem(KEY)!)).toEqual({ storage: false, instances: true });
  });

  it("sends the pins to the server with the rest of the look", () => {
    setNavPin("instances", true);
    const put = fetchMock.mock.calls.find((c) => c[1]?.method === "PUT");
    expect(put?.[0]).toBe("/api/display-prefs");
    expect(JSON.parse(put![1].body as string)).toEqual({ [KEY]: '{"instances":true}' });
  });
});

describe("Instances and its modules", () => {
  it("appears when a module is switched on later", () => {
    expect(readNavPins(off).instances).toBe(false);
    expect(readNavPins({ ...off, fleetEnabled: true }).instances).toBe(true);
  });

  it("stays away once it was unpinned, whatever is switched on after that", () => {
    setNavPin("instances", false);
    expect(readNavPins({ ...off, fleetEnabled: true }).instances).toBe(false);
    expect(readNavPins({ receiverEnabled: true, fleetEnabled: true, pullEnabled: true }).instances).toBe(false);
  });

  it("stays pinned by hand with every module off", () => {
    setNavPin("instances", true);
    expect(readNavPins(off).instances).toBe(true);
  });
});

describe("useNavPins", () => {
  it("follows a change made in the same tab", () => {
    const { result } = renderHook(() => useNavPins(off));
    expect(result.current).toEqual({ storage: true, instances: false });

    act(() => setNavPin("instances", true));

    expect(result.current).toEqual({ storage: true, instances: true });
  });

  it("follows the settings while nothing is stored", () => {
    const { result, rerender } = renderHook(({ settings }) => useNavPins(settings), {
      initialProps: { settings: off },
    });
    expect(result.current.instances).toBe(false);

    rerender({ settings: { ...off, pullEnabled: true } });

    expect(result.current.instances).toBe(true);
  });

  it("reads again when the server's look is adopted", () => {
    const { result } = renderHook(() => useNavPins(off));

    localStorage.setItem(KEY, '{"storage":false,"instances":true}');
    expect(result.current).toEqual({ storage: true, instances: false });
    act(() => {
      window.dispatchEvent(new Event(ADOPTED_EVENT));
    });

    expect(result.current).toEqual({ storage: false, instances: true });
    expect(
      fetchMock.mock.calls.filter((c) => c[1]?.method === "PUT"),
      "an adopted value came from the server and is not sent back",
    ).toEqual([]);
  });

  it("reads again when another tab changes the pins", () => {
    const { result } = renderHook(() => useNavPins(off));

    localStorage.setItem(KEY, '{"storage":false}');
    act(() => {
      window.dispatchEvent(new StorageEvent("storage", { key: KEY }));
    });

    expect(result.current).toEqual({ storage: false, instances: false });
  });

});

describe("subscribeNavPins", () => {
  it("reports a change here, in another tab and from the server, until it is cancelled", () => {
    const listener = vi.fn();
    const cancel = subscribeNavPins(listener);

    setNavPin("storage", false);
    window.dispatchEvent(new StorageEvent("storage", { key: KEY }));
    window.dispatchEvent(new Event(ADOPTED_EVENT));
    expect(listener).toHaveBeenCalledTimes(3);

    cancel();
    setNavPin("storage", true);
    window.dispatchEvent(new StorageEvent("storage", { key: KEY }));
    window.dispatchEvent(new Event(ADOPTED_EVENT));
    expect(listener).toHaveBeenCalledTimes(3);
  });
});
