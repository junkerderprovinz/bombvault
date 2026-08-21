// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// OffsiteIndicator — real DOM behaviour for issue #159's live off-site
// progress readout. A first cut of this feature concluded restic copy had no
// honest percentage to show and shipped a duration-only readout; that
// conclusion was wrong (see the component's own doc comment / restic.Copy's
// doc comment for the corrected story) — restic copy DOES print a real,
// parseable per-snapshot percentage once RESTIC_PROGRESS_FPS is wired up.
// This covers the full tiered display end to end: the component consumes the
// SAME lib/progress.ts SSE plumbing production code does (mocking only the
// browser's EventSource, which jsdom does not implement, so `new
// EventSource(...)` inside progress.ts's openSource() doesn't throw) and
// renders through the real useProgress()/offsiteStatusText()/elapsedSince()
// pipeline — nothing about the component under test is stubbed.
//
// See OffsiteIndicator.test.ts for direct pure-function coverage of
// offsiteStatusText's tiering logic (faster, no DOM needed).
//
// `.dom.test.tsx` per Selector.dom.test.tsx's convention for the jsdom-opted-in
// exception (vitest.config.ts stays "node" by default).
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { OffsiteIndicator } from "./OffsiteIndicator";

// A minimal fake EventSource: progress.ts's openSource() does `new
// EventSource("/api/progress")` then assigns `.onmessage`. Capturing the
// latest instance lets a test push a synthetic SSE frame through the EXACT
// same `source.onmessage = handleMessage` path a real backend push would use.
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

describe("OffsiteIndicator — elapsed duration (issue #159)", () => {
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
    // t≈0s: formatDuration(0) => "0s", which IS truthy, so a real elapsed
    // reading of exactly "0s" is still shown rather than being suppressed.
    expect(screen.getByText(/Replicating…\s*\(0s\)/)).toBeTruthy();

    act(() => {
      vi.advanceTimersByTime(3000); // the component's own 1s local tick, no new SSE event needed
    });
    expect(screen.getByText(/Replicating…\s*\(3s\)/)).toBeTruthy();
    expect(document.body.textContent).not.toContain("NaN");
  });

  it("keeps ticking the SAME duration across a backend heartbeat re-publish (same startedAt)", () => {
    const nowSec = Math.floor(Date.now() / 1000);
    render(<OffsiteIndicator domain="containers" />);
    act(() => {
      lastInstance().emit({ key: "offsite:containers", phase: "replicate", percent: 0, active: true, startedAt: nowSec });
    });
    act(() => {
      vi.advanceTimersByTime(5000);
    });
    expect(screen.getByText(/\(5s\)/)).toBeTruthy();
    // Backend heartbeat tick: SAME startedAt, re-published as the real
    // offsiteProgressHeartbeat loop does (see service.go's copyToOffsite).
    act(() => {
      lastInstance().emit({ key: "offsite:containers", phase: "replicate", percent: 0, active: true, startedAt: nowSec });
    });
    expect(screen.getByText(/\(5s\)/)).toBeTruthy(); // unchanged — no reset to 0
  });

  // Review fix: progEnd now publishes the SAME StartedAt on the terminal event
  // (previously it published none at all, which zeroed startedAt and made the
  // duration vanish for the ~0.8-2.5s the terminal event lingers on screen —
  // see progress.ts's COMPLETE_LINGER_MS / this component's MIN_VISIBLE_MS).
  // This asserts the CORRECT behavior now: the duration keeps showing its
  // real value through the terminal event, not just "doesn't say NaN".
  it("keeps showing the correct duration through the terminal event (StartedAt is no longer dropped)", () => {
    const nowSec = Math.floor(Date.now() / 1000);
    render(<OffsiteIndicator domain="containers" />);
    act(() => {
      lastInstance().emit({ key: "offsite:containers", phase: "replicate", percent: 0, active: true, startedAt: nowSec });
    });
    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(screen.getByText(/\(2s\)/)).toBeTruthy();
    // Terminal event now carries the SAME startedAt (the review fix).
    act(() => {
      lastInstance().emit({ key: "offsite:containers", phase: "replicate", percent: 100, active: false, startedAt: nowSec });
    });
    // Still active in the frontend's map during the linger (Active is held
    // true client-side until COMPLETE_LINGER_MS elapses — see progress.ts),
    // so the duration is still rendered, and correctly, not blanked.
    expect(screen.getByText(/\(2s\)/)).toBeTruthy();
    expect(document.body.textContent).not.toContain("NaN");
  });

  // Defensive fallback: even if a future backend regression DID publish a
  // terminal event with no StartedAt (or an old call site missed the fix),
  // the indicator must still degrade to the plain label, never "NaN" or a
  // garbage duration.
  it("degrades to the plain label if a terminal event ever arrives with no StartedAt", () => {
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
    expect(document.body.textContent).not.toMatch(/\(\d/); // no stale/garbage duration survives
  });

  // progress.go's Event doc comment (and reltime.ts's elapsedSince) both
  // document that a client "must treat 0 as unknown, never an actual epoch
  // second" — this is the case that guard exists for: a genuine 0 would
  // otherwise compute an elapsed span back to the Unix epoch.
  it("treats startedAt: 0 as unknown, never rendering the epoch's ~56-year elapsed span", () => {
    render(<OffsiteIndicator domain="containers" />);
    act(() => {
      lastInstance().emit({ key: "offsite:containers", phase: "replicate", percent: 0, active: true, startedAt: 0 });
    });
    expect(screen.getByText(/Replicating…/)).toBeTruthy();
    expect(document.body.textContent).not.toContain("NaN");
    expect(document.body.textContent).not.toMatch(/\(\d/);
  });

  it("treats a negative startedAt as unknown, never rendering a negative or garbage duration", () => {
    render(<OffsiteIndicator domain="containers" />);
    act(() => {
      lastInstance().emit({ key: "offsite:containers", phase: "replicate", percent: 0, active: true, startedAt: -100 });
    });
    expect(screen.getByText(/Replicating…/)).toBeTruthy();
    expect(document.body.textContent).not.toContain("NaN");
    expect(document.body.textContent).not.toMatch(/\(-?\d/);
  });
});

describe("OffsiteIndicator — live snapshot/percent (issue #159's real percentage)", () => {
  it("shows a live snapshot-of-total percentage once the backend reports one", () => {
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
    expect(screen.getByText(/Replicating snapshot 2 of 4 \(63%\)/)).toBeTruthy();
  });

  it("combines the live percentage with the elapsed duration", () => {
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
    expect(screen.getByText(/Replicating snapshot 1 of 1 \(10%\).*0s/)).toBeTruthy();
  });

  it("a single-snapshot run still renders correctly (not misread as 'unknown')", () => {
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
    expect(screen.getByText(/Replicating snapshot 1 of 1 \(80%\)/)).toBeTruthy();
  });
});
