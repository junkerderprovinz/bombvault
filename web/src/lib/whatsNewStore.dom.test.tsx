// @vitest-environment jsdom
// What a browser has seen of the release's changes: kept per release, new only
// to a browser that comes from an older one, and never lost to a later patch.
import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { CHANGES, WHATS_NEW_RELEASE } from "./whatsNew";
import { beginWhatsNew, markAbsent, markSeen, useWhatsNew } from "./whatsNewStore";

const KEY = `bv-whats-new@${WHATS_NEW_RELEASE}`;
const [FIRST, SECOND] = CHANGES.map((change) => change.id);
const stored = () => JSON.parse(localStorage.getItem(KEY) ?? "null") as { seen: string[]; absent: string[] } | null;

beforeEach(() => localStorage.clear());
afterEach(cleanup);

describe("an update", () => {
  it("makes every change new when it comes from a release before this one", () => {
    beginWhatsNew("v9.9.1");
    expect(stored()).toEqual({ seen: [], absent: [] });
  });

  it("reads the release out of a build's version", () => {
    beginWhatsNew("v9.9.1+main.fcc0544");
    expect(stored()).toEqual({ seen: [], absent: [] });
  });

  it.each([`v${WHATS_NEW_RELEASE}`, "v99.0.0", "dev"])("stores nothing when it comes from %s", (previous) => {
    beginWhatsNew(previous);
    expect(stored()).toBeNull();
  });

  it("leaves what was seen when a later update comes from an older release again", () => {
    beginWhatsNew("v9.9.1");
    markSeen([FIRST]);
    beginWhatsNew("v9.9.1");
    expect(stored()).toEqual({ seen: [FIRST], absent: [] });
  });

  it("drops what an earlier release kept", () => {
    localStorage.setItem("bv-whats-new@9.0.0", JSON.stringify({ seen: ["old"] }));
    localStorage.setItem("bv-theme", "dark");
    beginWhatsNew("v9.9.1");
    expect(localStorage.getItem("bv-whats-new@9.0.0")).toBeNull();
    expect(localStorage.getItem("bv-theme")).toBe("dark");
  });
});

describe("what was seen", () => {
  it("counts everything as seen in a browser that saw no older release", () => {
    const { result } = renderHook(() => useWhatsNew());
    expect(result.current.seen.size).toBe(CHANGES.length);
    expect(result.current.unseen).toEqual([]);
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
    beginWhatsNew("v9.9.1");
    markSeen([FIRST]);
    markSeen([FIRST, SECOND]);
    expect(stored()?.seen).toEqual([FIRST, SECOND]);
  });

  it("makes changes new again", () => {
    beginWhatsNew("v9.9.1");
    const { result } = renderHook(() => useWhatsNew());
    act(() => markSeen([FIRST, SECOND]));
    act(() => markSeen([FIRST], false));
    expect([...result.current.seen]).toEqual([SECOND]);
  });

  it("takes anything else in its slot for nothing stored", () => {
    localStorage.setItem(KEY, "{not json");
    const { result } = renderHook(() => useWhatsNew());
    expect(result.current.unseen).toEqual([]);
    localStorage.setItem(KEY, JSON.stringify({ seen: [1, FIRST, null] }));
    const again = renderHook(() => useWhatsNew());
    expect([...again.result.current.seen]).toEqual([FIRST]);
  });

  it("reads a slot that names no absent changes", () => {
    localStorage.setItem(KEY, JSON.stringify({ seen: [FIRST] }));
    const { result } = renderHook(() => useWhatsNew());
    expect(result.current.absent.size).toBe(0);
    expect(result.current.seen.has(FIRST)).toBe(true);
  });

  it("follows another tab", () => {
    const { result } = renderHook(() => useWhatsNew());
    act(() => {
      localStorage.setItem(KEY, JSON.stringify({ seen: [FIRST] }));
      window.dispatchEvent(new StorageEvent("storage", { key: KEY }));
    });
    expect([...result.current.seen]).toEqual([FIRST]);
    expect(result.current.unseen).toHaveLength(CHANGES.length - 1);
  });
});

describe("a change its page has nothing for", () => {
  it("is kept as absent and stays among what is left", () => {
    beginWhatsNew("v9.9.1");
    const { result } = renderHook(() => useWhatsNew());
    act(() => markAbsent([FIRST]));
    expect([...result.current.absent]).toEqual([FIRST]);
    expect(result.current.unseen).toEqual(CHANGES);
    expect(stored()?.absent).toEqual([FIRST]);
  });

  it("is not kept twice, and is there again once its page draws it", () => {
    markAbsent([FIRST]);
    markAbsent([FIRST, SECOND]);
    expect(stored()?.absent).toEqual([FIRST, SECOND]);
    markAbsent([FIRST], false);
    expect(stored()?.absent).toEqual([SECOND]);
  });

  it("is looked for afresh when it is marked as seen or as new", () => {
    beginWhatsNew("v9.9.1");
    markAbsent([FIRST, SECOND]);
    markSeen([FIRST]);
    expect(stored()).toMatchObject({ seen: [FIRST], absent: [SECOND] });
    markSeen([FIRST, SECOND], false);
    expect(stored()).toMatchObject({ seen: [], absent: [] });
  });
});
