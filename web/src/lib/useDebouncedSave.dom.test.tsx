// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useDebouncedSave } from "./useDebouncedSave";

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe("useDebouncedSave", () => {
  it("runs only the last call once the delay has passed", () => {
    const runs: string[] = [];
    const { result } = renderHook(() => useDebouncedSave());
    result.current.debouncedSave(() => runs.push("a"));
    result.current.debouncedSave(() => runs.push("b"));
    act(() => vi.advanceTimersByTime(1000));
    expect(runs).toEqual(["b"]);
  });

  it("saves a call still waiting when the component goes away", () => {
    const runs: string[] = [];
    const { result, unmount } = renderHook(() => useDebouncedSave());
    result.current.debouncedSave(() => runs.push("typed"));
    unmount();
    expect(runs).toEqual(["typed"]);
    act(() => vi.advanceTimersByTime(1000));
    expect(runs).toEqual(["typed"]);
  });

  it("runs nothing more on unmount once the call has run or was cancelled", () => {
    const runs: string[] = [];
    const { result, unmount } = renderHook(() => useDebouncedSave());
    result.current.debouncedSave(() => runs.push("ran"));
    act(() => vi.advanceTimersByTime(1000));
    result.current.debouncedSave(() => runs.push("cancelled"));
    result.current.cancel();
    unmount();
    expect(runs).toEqual(["ran"]);
  });
});
