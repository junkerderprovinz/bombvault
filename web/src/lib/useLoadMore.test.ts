// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// useLoadMore — window math and behavior proofs for the mobile list
// pagination primitive. loadMoreWindow is pure, so its boundary math is asserted directly;
// the hook is exercised through the real React wiring with renderHook (the
// useVisibilityGate.test.ts precedent) — no re-render is faked, so a passing
// reset-on-identity-change test means the consumer filter flow flips the
// slice on the real render path.
//
// jsdom (not node) because renderHook needs a document; the pure-fn asserts
// are environment-independent either way.
// ---------------------------------------------------------------------------
import { afterEach, describe, expect, it } from "vitest";
import { act, cleanup, renderHook } from "@testing-library/react";
import { loadMoreWindow, useLoadMore } from "./useLoadMore";

afterEach(() => {
  cleanup();
});

function makeItems(n: number): number[] {
  return Array.from({ length: n }, (_, i) => i);
}

describe("loadMoreWindow (pure)", () => {
  it("adds exactly one threshold", () => {
    expect(loadMoreWindow(100, 20, 20)).toBe(40);
  });

  it("clamps at total (over-n)", () => {
    expect(loadMoreWindow(45, 40, 20)).toBe(45);
  });

  it("clamps at total when visible already exceeds it", () => {
    expect(loadMoreWindow(10, 20, 20)).toBe(10);
  });

  it("lands exactly on total when total is a threshold multiple (exactly-n)", () => {
    expect(loadMoreWindow(40, 20, 20)).toBe(40);
  });

  it("stays at 0 for an empty list", () => {
    expect(loadMoreWindow(0, 0, 20)).toBe(0);
  });

  it("never returns a negative count", () => {
    expect(loadMoreWindow(5, -25, 20)).toBe(0);
  });

  it("honors a non-default threshold", () => {
    expect(loadMoreWindow(100, 7, 50)).toBe(57);
  });
});

describe("useLoadMore (hook)", () => {
  // The items array must be STABLE across renders (a useState/module const,
  // like every consumer's filtered array) — the hook resets on identity
  // change by contract, so a fresh array per render callback would loop.
  it("shows the initial window and reports more", () => {
    const items = makeItems(45);
    const { result } = renderHook(() => useLoadMore(items));
    expect(result.current.visible).toEqual(items.slice(0, 20));
    expect(result.current.hasMore).toBe(true);
  });

  it("appends exactly one threshold per showMore", () => {
    const items = makeItems(45);
    const { result } = renderHook(() => useLoadMore(items));
    act(() => result.current.showMore());
    expect(result.current.visible).toEqual(items.slice(0, 40));
    expect(result.current.hasMore).toBe(true);
    act(() => result.current.showMore());
    expect(result.current.visible).toEqual(items);
    expect(result.current.hasMore).toBe(false);
  });

  it("reports hasMore false at exactly-n items (button must not render)", () => {
    const items = makeItems(20);
    const { result } = renderHook(() => useLoadMore(items));
    expect(result.current.visible).toEqual(items);
    expect(result.current.hasMore).toBe(false);
  });

  it("reports hasMore false for an empty list", () => {
    const items: number[] = [];
    const { result } = renderHook(() => useLoadMore(items));
    expect(result.current.visible).toEqual([]);
    expect(result.current.hasMore).toBe(false);
  });

  it("reports hasMore false for a one-item list", () => {
    const items = makeItems(1);
    const { result } = renderHook(() => useLoadMore(items));
    expect(result.current.hasMore).toBe(false);
  });

  it("reset returns the slice to the initial window", () => {
    const items = makeItems(45);
    const { result } = renderHook(() => useLoadMore(items));
    act(() => result.current.showMore());
    expect(result.current.visible).toHaveLength(40);
    act(() => result.current.reset());
    expect(result.current.visible).toHaveLength(20);
    expect(result.current.hasMore).toBe(true);
  });

  it("resets the slice when the items array identity changes (new filter result)", () => {
    let items = makeItems(45);
    const { result, rerender } = renderHook(() => useLoadMore(items));
    act(() => result.current.showMore());
    expect(result.current.visible).toHaveLength(40);
    // A NEW array instance is a new filter result even with equal content —
    // consumers pass the filtered array, so identity IS the filter signal.
    items = makeItems(45);
    rerender();
    expect(result.current.visible).toHaveLength(20);
    expect(result.current.hasMore).toBe(true);
  });

  it("keeps the constant threshold across the hook's life", () => {
    const items = makeItems(45);
    const { result } = renderHook(() => useLoadMore(items, 20));
    act(() => result.current.showMore());
    act(() => result.current.showMore());
    // 20 -> 40 -> 45 (clamped): the step never grows.
    expect(result.current.visible).toHaveLength(45);
  });

  // --- live-feed preserveKey (the dashboard activity log's contract) ----

  it("live feed: same preserveKey keeps the window across a data refresh", () => {
    let items = makeItems(45);
    const { result, rerender } = renderHook(() => useLoadMore(items, 20, "filters:f"));
    act(() => result.current.showMore());
    expect(result.current.visible).toHaveLength(40);
    // A poll/tick re-merge: NEW array identity, SAME filter key — a refresh,
    // not a filter result. The reader's page survives it.
    items = makeItems(45);
    rerender();
    expect(result.current.visible).toHaveLength(40);
    expect(result.current.hasMore).toBe(true);
  });

  it("live feed: a preserveKey change rewinds like a filter change", () => {
    let items = makeItems(45);
    let key = "filters:a";
    const { result, rerender } = renderHook(() => useLoadMore(items, 20, key));
    act(() => result.current.showMore());
    expect(result.current.visible).toHaveLength(40);
    key = "filters:b";
    items = makeItems(45);
    rerender();
    expect(result.current.visible).toHaveLength(20);
    expect(result.current.hasMore).toBe(true);
  });

  it("live feed: a refresh that shrinks the list clamps the visible slice", () => {
    let items = makeItems(45);
    const { result, rerender } = renderHook(() => useLoadMore(items, 20, "filters:f"));
    act(() => result.current.showMore());
    act(() => result.current.showMore()); // 45 (clamped at total)
    expect(result.current.visible).toHaveLength(45);
    items = makeItems(30); // prune removed 15 rows server-side
    rerender();
    expect(result.current.visible).toHaveLength(30);
    expect(result.current.hasMore).toBe(false);
  });

  it("live feed: a refresh keeps the window and new rows stay one press away", () => {
    let items = makeItems(25);
    const { result, rerender } = renderHook(() => useLoadMore(items, 20, "filters:f"));
    expect(result.current.visible).toHaveLength(20);
    items = makeItems(25); // five new rows arrived (newest-at-bottom feed)
    rerender();
    expect(result.current.visible).toHaveLength(20);
    act(() => result.current.showMore());
    expect(result.current.visible).toHaveLength(25);
  });
});
