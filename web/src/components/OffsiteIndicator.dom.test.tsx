// @vitest-environment jsdom
// Runs the component through the real lib/progress.ts SSE handling. Only
// EventSource is faked, because jsdom has none.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { OffsiteIndicator } from "./OffsiteIndicator";

// progress.ts's openSource() constructs an EventSource and assigns
// `.onmessage`; emit() pushes a frame through that same handler.
const instances: FakeEventSource[] = [];

class FakeEventSource {
  onmessage: ((ev: MessageEvent<string>) => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(_url: string) {
    instances.push(this);
  }
  close(): void {}
  emit(payload: unknown): void {
    this.onmessage?.({ data: JSON.stringify(payload) } as MessageEvent<string>);
  }
}

function lastInstance(): FakeEventSource {
  const inst = instances.at(-1);
  if (!inst) throw new Error("no FakeEventSource constructed yet");
  return inst;
}

beforeEach(() => {
  vi.useFakeTimers();
  instances.length = 0;
  // @ts-expect-error -- test-only global stub; jsdom has no EventSource
  global.EventSource = FakeEventSource;
});

afterEach(() => {
  cleanup(); // unmount first: its effect cleanup closes the shared EventSource singleton
  vi.useRealTimers();
});

describe("OffsiteIndicator, elapsed duration", () => {
  it("shows the plain replicating label with no duration before startedAt is known", () => {
    render(<OffsiteIndicator domain="containers" />);
    act(() => {
      lastInstance().emit({ key: "offsite:containers", phase: "replicate", percent: 0, active: true });
    });
    expect(screen.getByText(/Replicating…/)).toBeTruthy();
    expect(document.body.textContent).not.toContain("NaN");
    expect(document.body.textContent).not.toMatch(/\(\d/); // no "(Ns)"-style duration yet
  });

  it("ticks a live elapsed duration once startedAt is known, advancing every second", () => {
    const nowSec = Math.floor(Date.now() / 1000);
    render(<OffsiteIndicator domain="containers" />);
    act(() => {
      lastInstance().emit({ key: "offsite:containers", phase: "replicate", percent: 0, active: true, startedAt: nowSec });
    });
    // "0s" is a non-empty string, so a zero elapsed time is still shown.
    expect(screen.getByText(/Replicating…\s*\(0s\)/)).toBeTruthy();

    act(() => {
      vi.advanceTimersByTime(3000); // the local tick, no SSE event
    });
    expect(screen.getByText(/Replicating…\s*\(3s\)/)).toBeTruthy();
    expect(document.body.textContent).not.toContain("NaN");
  });

  it("keeps the duration across a heartbeat that re-publishes the same startedAt", () => {
    const nowSec = Math.floor(Date.now() / 1000);
    render(<OffsiteIndicator domain="containers" />);
    act(() => {
      lastInstance().emit({ key: "offsite:containers", phase: "replicate", percent: 0, active: true, startedAt: nowSec });
    });
    act(() => {
      vi.advanceTimersByTime(5000);
    });
    expect(screen.getByText(/\(5s\)/)).toBeTruthy();
    // offsiteProgressHeartbeat re-publishes the event with the same startedAt.
    act(() => {
      lastInstance().emit({ key: "offsite:containers", phase: "replicate", percent: 0, active: true, startedAt: nowSec });
    });
    expect(screen.getByText(/\(5s\)/)).toBeTruthy(); // not reset to 0
  });

  it("keeps showing the duration through the terminal event", () => {
    const nowSec = Math.floor(Date.now() / 1000);
    render(<OffsiteIndicator domain="containers" />);
    act(() => {
      lastInstance().emit({ key: "offsite:containers", phase: "replicate", percent: 0, active: true, startedAt: nowSec });
    });
    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(screen.getByText(/\(2s\)/)).toBeTruthy();
    act(() => {
      lastInstance().emit({ key: "offsite:containers", phase: "replicate", percent: 100, active: false, startedAt: nowSec });
    });
    // progress.ts keeps the entry active until COMPLETE_LINGER_MS has passed.
    expect(screen.getByText(/\(2s\)/)).toBeTruthy();
    expect(document.body.textContent).not.toContain("NaN");
  });

  it("falls back to the plain label when a terminal event has no startedAt", () => {
    const nowSec = Math.floor(Date.now() / 1000);
    render(<OffsiteIndicator domain="containers" />);
    act(() => {
      lastInstance().emit({ key: "offsite:containers", phase: "replicate", percent: 0, active: true, startedAt: nowSec });
    });
    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(screen.getByText(/\(2s\)/)).toBeTruthy();
    act(() => {
      lastInstance().emit({ key: "offsite:containers", phase: "replicate", percent: 100, active: false });
    });
    expect(screen.getByText(/Replicating…/)).toBeTruthy();
    expect(document.body.textContent).not.toContain("NaN");
    expect(document.body.textContent).not.toMatch(/\(\d/); // no stale duration
  });

  // 0 means unknown (see progress.go's Event), not the Unix epoch.
  it("treats startedAt 0 as unknown", () => {
    render(<OffsiteIndicator domain="containers" />);
    act(() => {
      lastInstance().emit({ key: "offsite:containers", phase: "replicate", percent: 0, active: true, startedAt: 0 });
    });
    expect(screen.getByText(/Replicating…/)).toBeTruthy();
    expect(document.body.textContent).not.toContain("NaN");
    expect(document.body.textContent).not.toMatch(/\(\d/);
  });

  it("treats a negative startedAt as unknown", () => {
    render(<OffsiteIndicator domain="containers" />);
    act(() => {
      lastInstance().emit({ key: "offsite:containers", phase: "replicate", percent: 0, active: true, startedAt: -100 });
    });
    expect(screen.getByText(/Replicating…/)).toBeTruthy();
    expect(document.body.textContent).not.toContain("NaN");
    expect(document.body.textContent).not.toMatch(/\(-?\d/);
  });
});

describe("OffsiteIndicator, run-level percentage", () => {
  it("shows a run-level percentage once the backend reports a snapshot index/total", () => {
    render(<OffsiteIndicator domain="containers" />);
    act(() => {
      lastInstance().emit({
        key: "offsite:containers",
        phase: "replicate",
        percent: 62.6,
        active: true,
        snapshotIndex: 2,
        snapshotTotal: 4,
      });
    });
    expect(screen.getByText(/Replicating… 41% overall \(snapshot 2 of 4\)/)).toBeTruthy();
  });

  it("shows snapshot 15 of 126 at 55% as 12% overall, not 55%", () => {
    render(<OffsiteIndicator domain="containers" />);
    act(() => {
      lastInstance().emit({
        key: "offsite:containers",
        phase: "replicate",
        percent: 55,
        active: true,
        snapshotIndex: 15,
        snapshotTotal: 126,
      });
    });
    expect(screen.getByText(/Replicating… 12% overall \(snapshot 15 of 126\)/)).toBeTruthy();
    expect(document.body.textContent).not.toContain("55%");
  });

  // No total means the backend could not estimate one (api.progBeginCopySink).
  it("falls back to the duration readout when no snapshot total was reported", () => {
    const nowSec = Math.floor(Date.now() / 1000);
    render(<OffsiteIndicator domain="containers" />);
    act(() => {
      lastInstance().emit({
        key: "offsite:containers",
        phase: "replicate",
        percent: 55,
        active: true,
        startedAt: nowSec,
        snapshotIndex: 15,
      });
    });
    expect(screen.getByText(/Replicating…\s*\(0s\)/)).toBeTruthy();
    expect(document.body.textContent).not.toContain("overall");
  });

  it("combines the run-level percentage with the elapsed duration", () => {
    const nowSec = Math.floor(Date.now() / 1000);
    render(<OffsiteIndicator domain="containers" />);
    act(() => {
      lastInstance().emit({
        key: "offsite:containers",
        phase: "replicate",
        percent: 10,
        active: true,
        startedAt: nowSec,
        snapshotIndex: 1,
        snapshotTotal: 1,
      });
    });
    expect(screen.getByText(/Replicating… 10% overall \(snapshot 1 of 1\).*0s/)).toBeTruthy();
  });

  it("shows a single-snapshot run rather than treating it as unknown", () => {
    render(<OffsiteIndicator domain="files" />);
    act(() => {
      lastInstance().emit({
        key: "offsite:files",
        phase: "replicate",
        percent: 80,
        active: true,
        snapshotIndex: 1,
        snapshotTotal: 1,
      });
    });
    expect(screen.getByText(/Replicating… 80% overall \(snapshot 1 of 1\)/)).toBeTruthy();
  });

  it("attaches the estimate info bubble only while a run-level percentage is shown", () => {
    render(<OffsiteIndicator domain="containers" />);
    act(() => {
      lastInstance().emit({ key: "offsite:containers", phase: "replicate", percent: 0, active: true });
    });
    expect(document.querySelector('[aria-label^="Overall progress"]')).toBeNull();

    act(() => {
      lastInstance().emit({
        key: "offsite:containers",
        phase: "replicate",
        percent: 30,
        active: true,
        snapshotIndex: 2,
        snapshotTotal: 4,
      });
    });
    expect(document.querySelector('[aria-label^="Overall progress"]')).not.toBeNull();
  });
});
