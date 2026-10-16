// @vitest-environment jsdom
// What a browser has seen of the changes: new to a browser that comes from a
// release before theirs, and never lost to a later update.
import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { CHANGES } from "./whatsNew";
import { beginWhatsNew, markAbsent, markSeen, newer, useWhatsNew } from "./whatsNewStore";

const KEY = "bv-whats-new";
const [FIRST, SECOND] = CHANGES.map((change) => change.id);
const OLDEST = CHANGES.reduce((min, change) => (newer(min, change.since) ? change.since : min), CHANGES[0].since);
const NEWEST = CHANGES.reduce((max, change) => (newer(change.since, max) ? change.since : max), CHANGES[0].since);
const stored = () => JSON.parse(localStorage.getItem(KEY) ?? "null") as { seen: string[]; absent: string[]; from?: string } | null;

beforeEach(() => localStorage.clear());
afterEach(cleanup);

describe("an update", () => {
  it("makes every change new when it comes from a release before all of them", () => {
    beginWhatsNew("v9.9.1");
    expect(stored()).toEqual({ seen: [], absent: [], from: "9.9.1" });
  });

  it("reads the release out of a build's version", () => {
    beginWhatsNew("v9.9.1+main.fcc0544");
    expect(stored()?.from).toBe("9.9.1");
  });

  it("makes only the changes of later releases new", () => {
    beginWhatsNew(`v${OLDEST}`);
    const { result } = renderHook(() => useWhatsNew());
    expect(result.current.unseen.length).toBeGreaterThan(0);
    expect(result.current.unseen.every((change) => newer(change.since, OLDEST))).toBe(true);
  });

  it("makes nothing new when it comes from the newest release or a later one", () => {
    beginWhatsNew(`v${NEWEST}`);
    expect(renderHook(() => useWhatsNew()).result.current.unseen).toEqual([]);
    beginWhatsNew("v99.0.0");
    expect(renderHook(() => useWhatsNew()).result.current.unseen).toEqual([]);
  });

  it("stores nothing for a build without a release", () => {
    beginWhatsNew("dev");
    expect(stored()).toBeNull();
  });

  it("keeps what an earlier release left unseen", () => {
    beginWhatsNew("v9.9.1");
    markSeen([FIRST]);
    beginWhatsNew(`v${NEWEST}`);
    expect(stored()?.seen).toEqual([FIRST]);
    expect(renderHook(() => useWhatsNew()).result.current.unseen).toHaveLength(CHANGES.length - 1);
  });

  it("takes over the slot of an older release and drops it", () => {
    localStorage.setItem("bv-whats-new@10.0.0", JSON.stringify({ seen: [FIRST] }));
    localStorage.setItem("bv-theme", "dark");
    expect([...renderHook(() => useWhatsNew()).result.current.seen]).toEqual([FIRST]);
    beginWhatsNew(`v${NEWEST}`);
    expect(stored()?.seen).toEqual([FIRST]);
    expect(localStorage.getItem("bv-whats-new@10.0.0")).toBeNull();
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
    expect(stored()).toMatchObject({ seen: [FIRST], absent: [SECOND], from: "9.9.1" });
    markSeen([FIRST, SECOND], false);
    expect(stored()).toMatchObject({ seen: [], absent: [] });
  });
});
