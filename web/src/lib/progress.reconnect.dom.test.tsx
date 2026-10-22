// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, renderHook } from "@testing-library/react";
import { REOPEN_MS, useProgress } from "./progress";

class FakeEventSource {
  static readonly CLOSED = 2;
  readyState = 0;
  onmessage: ((ev: MessageEvent<string>) => void) | null = null;
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(_url: string) {
    streams.push(this);
  }
  close(): void {
    this.readyState = FakeEventSource.CLOSED;
  }
}

const streams: FakeEventSource[] = [];

function open() {
  act(() => streams.at(-1)?.onopen?.());
}

function send(data: unknown) {
  act(() => streams.at(-1)?.onmessage?.({ data: JSON.stringify(data) } as MessageEvent<string>));
}

beforeEach(() => {
  streams.length = 0;
  vi.stubGlobal("EventSource", FakeEventSource);
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("progress after a reconnect", () => {
  it("drops a bar the replay leaves out, whose end came while the stream was down", () => {
    const { result } = renderHook(() => useProgress());
    open();
    send({ key: "container:plex", phase: "backup", percent: 42, active: true });
    send({ key: "vm:win11", phase: "backup", percent: 10, active: true });
    send({ synced: true });

    open();
    send({ key: "vm:win11", phase: "backup", percent: 20, active: true });
    expect(result.current["container:plex"]?.active).toBe(true);
    send({ synced: true });

    expect(result.current["container:plex"]).toBeUndefined();
    expect(result.current["vm:win11"]?.percent).toBe(20);
  });

  it("opens the stream again when the browser gave up on it", () => {
    vi.useFakeTimers();
    try {
      const { result } = renderHook(() => useProgress());
      open();
      send({ key: "container:plex", phase: "backup", percent: 42, active: true });
      send({ synced: true });

      // A proxy answering 502 while the server restarts closes the stream for good.
      const dead = streams.at(-1)!;
      dead.readyState = FakeEventSource.CLOSED;
      act(() => dead.onerror?.());
      expect(streams).toHaveLength(1);
      act(() => vi.advanceTimersByTime(REOPEN_MS));
      expect(streams).toHaveLength(2);

      open();
      send({ synced: true });
      expect(result.current["container:plex"]).toBeUndefined();
    } finally {
      vi.useRealTimers();
    }
  });

  it("leaves a stream the browser retries by itself alone", () => {
    renderHook(() => useProgress());
    open();
    act(() => streams.at(-1)?.onerror?.());
    expect(streams).toHaveLength(1);
  });

  it("keeps a bar that ended just before, until it has lingered", () => {
    vi.useFakeTimers();
    try {
      const { result } = renderHook(() => useProgress());
      open();
      send({ key: "container:plex", phase: "backup", percent: 100, active: false });
      open();
      send({ synced: true });
      expect(result.current["container:plex"]?.finished).toBe(true);
    } finally {
      vi.useRealTimers();
    }
  });
});
