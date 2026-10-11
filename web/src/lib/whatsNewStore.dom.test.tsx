// @vitest-environment jsdom
// What a browser has seen of the release's changes: kept per release, switched
// on only by coming from an older one, and never lost to a later patch.
import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { CHANGES, WHATS_NEW_RELEASE } from "./whatsNew";
import { beginWhatsNew, markSeen, showDots, useWhatsNew } from "./whatsNewStore";

const KEY = `bv-whats-new@${WHATS_NEW_RELEASE}`;
const [FIRST, SECOND] = CHANGES.map((change) => change.id);
const stored = () => JSON.parse(localStorage.getItem(KEY) ?? "null") as { on: boolean; seen: string[] } | null;

beforeEach(() => localStorage.clear());
afterEach(cleanup);

describe("an update", () => {
  it("switches the dots on when it comes from a release before this one", () => {
    beginWhatsNew("v9.9.1");
    expect(stored()).toEqual({ on: true, seen: [] });
  });

  it("reads the release out of a build's version", () => {
    beginWhatsNew("v9.9.1+main.fcc0544");
    expect(stored()?.on).toBe(true);
  });

  it.each([`v${WHATS_NEW_RELEASE}`, "v99.0.0", "dev"])("stores nothing when it comes from %s", (previous) => {
    beginWhatsNew(previous);
    expect(stored()).toBeNull();
  });

  it("leaves what was seen and hidden when a later update comes from an older release again", () => {
    beginWhatsNew("v9.9.1");
    markSeen([FIRST]);
    showDots(false);
    beginWhatsNew("v9.9.1");
    expect(stored()).toEqual({ on: false, seen: [FIRST] });
  });

  it("drops what an earlier release kept", () => {
    localStorage.setItem("bv-whats-new@9.0.0", JSON.stringify({ on: true, seen: ["old"] }));
    localStorage.setItem("bv-theme", "dark");
    beginWhatsNew("v9.9.1");
    expect(localStorage.getItem("bv-whats-new@9.0.0")).toBeNull();
    expect(localStorage.getItem("bv-theme")).toBe("dark");
  });
});

describe("what was seen", () => {
  it("starts with nothing seen and the dots off in a browser that saw no older release", () => {
    const { result } = renderHook(() => useWhatsNew());
    expect(result.current.on).toBe(false);
    expect(result.current.unseen).toEqual(CHANGES);
  });

  it("counts a change as seen and takes it off the list of what is left", () => {
    beginWhatsNew("v9.9.1");
    const { result } = renderHook(() => useWhatsNew());
    act(() => markSeen([FIRST]));
    expect(result.current.seen.has(FIRST)).toBe(true);
    expect(result.current.unseen.map((change) => change.id)).not.toContain(FIRST);
    expect(result.current.unseen).toHaveLength(CHANGES.length - 1);
    expect(stored()?.seen).toEqual([FIRST]);
  });

  it("does not count a change twice", () => {
    markSeen([FIRST]);
    markSeen([FIRST, SECOND]);
    expect(stored()?.seen).toEqual([FIRST, SECOND]);
  });

  it("makes changes new again", () => {
    const { result } = renderHook(() => useWhatsNew());
    act(() => markSeen([FIRST, SECOND]));
    act(() => markSeen([FIRST], false));
    expect([...result.current.seen]).toEqual([SECOND]);
  });

  it("keeps what was seen when the dots are switched off and on", () => {
    beginWhatsNew("v9.9.1");
    const { result } = renderHook(() => useWhatsNew());
    act(() => markSeen([FIRST]));
    act(() => showDots(false));
    expect(result.current.on).toBe(false);
    act(() => showDots(true));
    expect(result.current).toMatchObject({ on: true });
    expect(result.current.seen.has(FIRST)).toBe(true);
  });

  it("takes anything else in its slot for nothing stored", () => {
    localStorage.setItem(KEY, "{not json");
    const { result } = renderHook(() => useWhatsNew());
    expect(result.current.on).toBe(false);
    expect(result.current.seen.size).toBe(0);
    localStorage.setItem(KEY, JSON.stringify({ on: "yes", seen: [1, FIRST, null] }));
    const again = renderHook(() => useWhatsNew());
    expect(again.result.current.on).toBe(false);
    expect([...again.result.current.seen]).toEqual([FIRST]);
  });

  it("follows another tab", () => {
    const { result } = renderHook(() => useWhatsNew());
    act(() => {
      localStorage.setItem(KEY, JSON.stringify({ on: true, seen: [FIRST] }));
      window.dispatchEvent(new StorageEvent("storage", { key: KEY }));
    });
    expect(result.current.on).toBe(true);
    expect(result.current.seen.has(FIRST)).toBe(true);
  });
});
