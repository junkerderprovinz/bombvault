// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, renderHook } from "@testing-library/react";
import { useProgress } from "./progress";

class FakeEventSource {
  onmessage: ((ev: MessageEvent<string>) => void) | null = null;
  onopen: (() => void) | null = null;
  constructor(_url: string) {
    streams.push(this);
  }
  close(): void {}
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
