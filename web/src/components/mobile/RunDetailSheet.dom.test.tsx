// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// RunDetailSheet — jsdom behavior proofs for the phone run-detail sheet.
//
// The sheet is a pure view over a Run record composed ENTIRELY from existing
// pieces, so these tests assert the composition contracts that can drift:
//   - the honest stat triad (the frozen-record substitutions —
//     humanBytes / formatDuration / mono slice, and NOTHING invented),
//   - the activity log rendered through the real buildLogLines pipeline (the
//     same builder ActivityLog.tsx uses — mocked only at the browser boundary,
//     see below),
//   - the failed path surfacing the backend's scrubbed reason verbatim via
//     runReason, with the direction contract (own sentence → page direction,
//     untranslated restic text → dir="ltr"),
//   - the domain honesty matrix (browse/restore only where a file-listing API
//     exists; verify only where checkDomain's union has the domain),
//   - the ≥44px tonal action rows and the restore entry's secondary placement
//     (in the scroll body, BEFORE the footer rows; never accent) — styling
//     contracts asserted as targeted class tokens, the documented jsdom
//     exception (BottomSheet.dom.test.tsx precedent: jsdom computes no
//     geometry, so the observable form of a styling contract IS the token).
//
// EventSource stub: lifted from OffsiteIndicator.dom.test.tsx — jsdom does not
// implement EventSource, and the live-section test drives the REAL
// lib/progress.ts singleton through `source.onmessage`, exactly the path a
// backend push takes. Nothing about the component under test is stubbed.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { RunDetailSheet } from "./RunDetailSheet";
import { en, I18nProvider } from "../../lib/i18n";
import type { Run } from "../../lib/api";

// A finished, successful container backup — every derived string below is
// computed from these fields by the REAL formatters.
const DONE_RUN: Run = {
  id: "0f1e2d3c4b5a69788796a5b4c3d2e1f0",
  targetId: "plex",
  kind: "backup",
  status: "success",
  startedAt: 1757600000,
  finishedAt: 1757603600, // +3600s → formatDuration: "1h 0m"
  snapshotId: "0f1e2d3c4b5a69788796a5b4c3d2e1f0",
  bytes: 5_000_000_000, // humanBytes: "4.7 GB"
  error: "",
  acknowledged: true,
  target: "plex",
  domain: "container",
};

function makeRun(overrides: Partial<Run>): Run {
  return { ...DONE_RUN, ...overrides };
}

function renderSheet(run: Run) {
  return render(
    <I18nProvider>
      <RunDetailSheet run={run} open onClose={() => {}} />
    </I18nProvider>
  );
}

// --- EventSource fake (OffsiteIndicator.dom.test.tsx pattern) ---------------

const instances: FakeEventSource[] = [];

class FakeEventSource {
  onmessage: ((ev: MessageEvent<string>) => void) | null = null;
  onerror: (() => void) | null = null;
  // Set by close() — the observable form of progress.ts's closeSource
  // contract (last unsubscribe closes the shared connection and drops its
  // cached state).
  closed = false;
  constructor(_url: string) {
    instances.push(this);
  }
  close(): void {
    this.closed = true;
  }
  emit(payload: unknown): void {
    this.onmessage?.({ data: JSON.stringify(payload) } as MessageEvent<string>);
  }
}

beforeEach(() => {
  instances.length = 0;
  // @ts-expect-error -- test-only global stub; jsdom has no EventSource
  global.EventSource = FakeEventSource;
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  // Restore jsdom's prototype getter (default "visible") — the visibility
  // tests below shadow it with an own property.
  delete (document as { visibilityState?: unknown }).visibilityState;
});

// Shadow document.visibilityState (jsdom never flips it by itself) and flip
// the gate the way a real browser does: a visibilitychange event. act() so
// React flushes the store subscription before the assertion reads the DOM.
function setPageVisibility(state: "visible" | "hidden"): void {
  Object.defineProperty(document, "visibilityState", {
    configurable: true,
    get: () => state,
  });
  act(() => {
    document.dispatchEvent(new Event("visibilitychange"));
  });
}

// ---------------------------------------------------------------------------

describe("RunDetailSheet", () => {
  it("composes the title from the shared helpers — kind + target, no new title key", () => {
    renderSheet(DONE_RUN);
    // runKindLabel("backup") = "Backup" (run.kindBackup) · runTargetText → the
    // container's human target. The interpunct is the separator (user text is
    // em-dash-free by lint law).
    expect(screen.getByText(`Backup · plex`)).toBeTruthy();
  });

  it("renders the honest stat triad — and nothing invented beyond the frozen record", () => {
    renderSheet(DONE_RUN);
    expect(screen.getByText(en["run.statVolume"])).toBeTruthy();
    expect(screen.getByText(en["dashboard.duration"])).toBeTruthy();
    expect(screen.getByText(en["run.statSnapshot"])).toBeTruthy();
    // humanBytes(5_000_000_000) through the real formatter.
    expect(screen.getByText("4.7 GB")).toBeTruthy();
    // formatDuration(3600) through the real formatter.
    expect(screen.getByText("1h 0m")).toBeTruthy();
    // The snapshot tile: 8-char mono slice with the FULL id as its title.
    const snap = screen.getByText("0f1e2d3c");
    expect(snap.getAttribute("title")).toBe(DONE_RUN.snapshotId);
  });

  it("carries tabular numerals on the tile values and the mono class on the snapshot", () => {
    renderSheet(DONE_RUN);
    expect(screen.getByText("4.7 GB").className).toContain("tabular-nums");
    expect(screen.getByText("1h 0m").className).toContain("tabular-nums");
    expect(screen.getByText("0f1e2d3c").className).toContain("tabular-nums");
    expect(screen.getByText("0f1e2d3c").className).toContain("font-mono");
    // Stat VALUES take the heading role at weight 600 (text-heading
    // font-semibold — the Fab label's typography), never the old text-sm
    // font-medium pair. Pinned as class tokens, the documented jsdom
    // exception (no computed geometry here).
    for (const value of ["4.7 GB", "1h 0m", "0f1e2d3c"]) {
      expect(screen.getByText(value).className).toContain("text-heading");
      expect(screen.getByText(value).className).toContain("font-semibold");
      expect(screen.getByText(value).className).not.toContain("font-medium");
    }
    // The activity log reuses the ActivityLog mono pattern verbatim. Queried
    // on document.body — the sheet portals there (BottomSheet primitive), so
    // the render container itself stays empty.
    expect(document.body.querySelector("div.font-mono.text-xs")).not.toBeNull();
  });

  it("renders the run's history line through the real buildLogLines pipeline", () => {
    renderSheet(DONE_RUN);
    // buildLogLines' en sentence for a finished backup: "{name} backed up:
    // {bytes} in {duration}" — the SAME text the dashboard's activity log
    // renders for this run.
    expect(screen.getByText(/plex backed up: 4\.7 GB in 1h 0m/)).toBeTruthy();
    // The glyph carries the shared aria-label vocabulary.
    expect(screen.getByLabelText(en["activityLog.glyphSuccess"])).toBeTruthy();
  });

  it("shows the status chip from the shared statusLabel/statusTone pair", () => {
    renderSheet(DONE_RUN);
    // statusLabel("success") → spike.ok.
    expect(screen.getByText(en["spike.ok"])).toBeTruthy();
  });

  it("failed run: backend reason verbatim, untranslated text pinned dir=ltr, fail chip", () => {
    renderSheet(makeRun({ status: "failed", error: "restic: repository is locked" }));
    expect(screen.getByText("restic: repository is locked")).toBeTruthy();
    const reason = screen.getByText("restic: repository is locked");
    expect(reason.getAttribute("dir")).toBe("ltr");
    expect(screen.getByText(en["spike.fail"])).toBeTruthy();
  });

  it("own-reason failed run: no dir override (the sentence follows the page)", () => {
    renderSheet(makeRun({ status: "failed", error: "interrupted (BombVault restarted mid-run)" }));
    const reason = screen.getByText("interrupted (BombVault restarted mid-run)");
    expect(reason.getAttribute("dir")).toBeNull();
  });

  it("footer rows exist with the >=44px min-height token", () => {
    renderSheet(DONE_RUN);
    const browse = screen.getByRole("button", { name: en["recovery.foreignStepBrowse"] });
    const verify = screen.getByRole("button", { name: en["integrity.verify"] });
    expect(browse.className).toContain("min-h-[2.75rem]");
    expect(verify.className).toContain("min-h-[2.75rem]");
  });

  it("restore entry sits in the scroll body BEFORE the footer rows, never accent", () => {
    const { container } = renderSheet(DONE_RUN);
    const restore = screen.getByRole("button", { name: en["snapshots.restore"] });
    // Secondary/tonal: the neutral surface token, and no accent anywhere.
    expect(restore.className).toContain("bg-carbon-surface3");
    expect(/accent/.test(restore.className)).toBe(false);
    // DOM order: the restore entry precedes BOTH footer rows (it is the last
    // element of the scroll body, above the chrome footer).
    const verify = screen.getByRole("button", { name: en["integrity.verify"] });
    expect(
      restore.compareDocumentPosition(verify) & Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy();
    expect(container).toBeTruthy();
  });

  it("browse row is a disclosure: aria-expanded flips and the tree mounts", () => {
    renderSheet(DONE_RUN);
    const browse = screen.getByRole("button", { name: en["recovery.foreignStepBrowse"] });
    expect(browse.getAttribute("aria-expanded")).toBe("false");
    fireEvent.click(browse);
    expect(browse.getAttribute("aria-expanded")).toBe("true");
    // SnapshotFileTree's filter input is the tree's observable mount (the
    // fetch itself rejects in jsdom and renders the tree's inline error —
    // also a real state).
    expect(screen.getByRole("textbox")).toBeTruthy();
  });

  it("vm runs: verify offered, browse/restore honestly absent (no file-listing API)", () => {
    // Backend shape (api.ts Run docs): targetId is the 32-hex vm_targets row
    // id, target is the human name — never set equal (that coincidence once
    // hid the progress-key bug the live tests below pin).
    renderSheet(makeRun({ targetId: "a1b2c3d4e5f60718293a4b5c6d7e8f90", target: "win11", domain: "vm" }));
    expect(screen.getByRole("button", { name: en["integrity.verify"] })).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["recovery.foreignStepBrowse"] })).toBeNull();
    expect(screen.queryByRole("button", { name: en["snapshots.restore"] })).toBeNull();
  });

  it("config runs: no verify (not in checkDomain's union), no browse, no footer rows", () => {
    renderSheet(makeRun({ targetId: "config", target: "Unraid config", domain: "config" }));
    expect(screen.queryByRole("button", { name: en["integrity.verify"] })).toBeNull();
    expect(screen.queryByRole("button", { name: en["recovery.foreignStepBrowse"] })).toBeNull();
    expect(screen.queryByRole("button", { name: en["snapshots.restore"] })).toBeNull();
  });

  it("running run: live section subscribes to the real progress pipeline and shows the bar", () => {
    vi.useFakeTimers();
    renderSheet(
      makeRun({
        status: "running",
        finishedAt: null, // duration tile falls back to the muted "—" mark
      })
    );
    // No SSE frame yet: the bar renders nothing (inactive), the live section
    // IS mounted (one EventSource for the sheet's subscription).
    expect(screen.queryByRole("progressbar")).toBeNull();
    expect(instances.length).toBe(1);
    // A live frame through the real onmessage path → the bar goes
    // indeterminate-active, and the run's live tail line appears.
    act(() => {
      instances[0].emit({ key: "container:plex", phase: "backup", percent: 0, active: true, startedAt: DONE_RUN.startedAt });
    });
    const bar = screen.getByRole("progressbar");
    expect(bar.getAttribute("aria-valuenow")).toBeNull(); // indeterminate at 0%
    // The duration tile degrades to the "—" meta mark, muted — never blank.
    // (The completion-time span carries the same mark for a null finishedAt;
    // what matters here is that the DURATION tile's mark is the muted one.)
    const marks = screen.getAllByText("—");
    expect(marks.length).toBeGreaterThanOrEqual(2);
    expect(marks.every((m) => m.className.includes("text-carbon-textMuted"))).toBe(true);
  });

  it("live frame percent drives the bar's aria-valuenow", () => {
    vi.useFakeTimers();
    renderSheet(makeRun({ status: "running", finishedAt: null }));
    act(() => {
      instances[0].emit({ key: "container:plex", phase: "backup", percent: 42.5, active: true, startedAt: DONE_RUN.startedAt });
    });
    expect(screen.getByRole("progressbar").getAttribute("aria-valuenow")).toBe("43");
  });

  // The progress key is the run NAME (run.target), never the 32-hex row id
  // (run.targetId): the backend records vm/files runs under the row id but
  // publishes SSE under "vm:"+name / "files:"+set.Name (internal/api/service.go).
  // These two pins keep the sheet's lookup on the published side of that split
  // — a regression to targetId would leave both bars dead (no frame matches).
  it("vm live run: the channel resolves under the run NAME — a 32-hex targetId keys nothing", () => {
    vi.useFakeTimers();
    renderSheet(
      makeRun({
        status: "running",
        finishedAt: null,
        domain: "vm",
        targetId: "a1b2c3d4e5f60718293a4b5c6d7e8f90", // vm_targets.id — what the backend records
        target: "win11", // the name the SSE publishes under ("vm:"+name)
      })
    );
    expect(instances.length).toBe(1);
    act(() => {
      instances[0].emit({ key: "vm:win11", phase: "backup", percent: 30, active: true, startedAt: DONE_RUN.startedAt });
    });
    expect(screen.getByRole("progressbar").getAttribute("aria-valuenow")).toBe("30");
  });

  it("files live run: same name-keyed channel — the set id never appears in the progress key", () => {
    vi.useFakeTimers();
    renderSheet(
      makeRun({
        status: "running",
        finishedAt: null,
        domain: "files",
        targetId: "f0e1d2c3b4a5968778695a4b3c2d1e0f", // file_sets.id — what the backend records
        target: "docs", // the name the SSE publishes under ("files:"+set.Name)
      })
    );
    expect(instances.length).toBe(1);
    act(() => {
      instances[0].emit({ key: "files:docs", phase: "backup", percent: 12, active: true, startedAt: DONE_RUN.startedAt });
    });
    expect(screen.getByRole("progressbar").getAttribute("aria-valuenow")).toBe("12");
  });

  // --- The live section is gated on page visibility --------------------------

  it("hidden page unmounts the live section — which unsubscribes the shared SSE connection", () => {
    vi.useFakeTimers();
    renderSheet(makeRun({ status: "running", finishedAt: null }));
    act(() => {
      instances[0].emit({ key: "container:plex", phase: "backup", percent: 10, active: true, startedAt: DONE_RUN.startedAt });
    });
    expect(screen.getByRole("progressbar")).toBeTruthy();

    setPageVisibility("hidden");
    // The gated subtree is gone...
    expect(screen.queryByRole("progressbar")).toBeNull();
    // ...and unmounting it WAS the unsubscribe: progress.ts's ref-count hit
    // zero and closeSource() closed the shared EventSource.
    expect(instances[0].closed).toBe(true);
  });

  it("visible again remounts the live section and resubscribes into a fresh connection", () => {
    vi.useFakeTimers();
    renderSheet(makeRun({ status: "running", finishedAt: null }));
    act(() => {
      instances[0].emit({ key: "container:plex", phase: "backup", percent: 10, active: true, startedAt: DONE_RUN.startedAt });
    });
    setPageVisibility("hidden");
    expect(screen.queryByRole("progressbar")).toBeNull();

    setPageVisibility("visible");
    // A SECOND EventSource exists: the remount re-subscribed, and the frozen
    // singleton reconnected (whose server snapshot replay repopulates state).
    expect(instances.length).toBe(2);
    expect(instances[1].closed).toBe(false);
    // Fresh subscription starts from an empty cache (closeSource dropped it) —
    // the bar is back only once the replayed stream delivers a frame again.
    expect(screen.queryByRole("progressbar")).toBeNull();
    act(() => {
      instances[1].emit({ key: "container:plex", phase: "backup", percent: 55, active: true, startedAt: DONE_RUN.startedAt });
    });
    expect(screen.getByRole("progressbar").getAttribute("aria-valuenow")).toBe("55");
  });

  it("a run that finished while hidden reconciles from the refetched server record on return", () => {
    vi.useFakeTimers();
    // The consumer's refetch is modeled the only way it can be here: a new
    // `run` record from listRuns (the sheet is a pure view over that record —
    // completion is never extrapolated from clocks).
    const { rerender } = renderSheet(makeRun({ status: "running", finishedAt: null }));
    act(() => {
      instances[0].emit({ key: "container:plex", phase: "backup", percent: 10, active: true, startedAt: DONE_RUN.startedAt });
    });
    setPageVisibility("hidden");
    expect(screen.queryByRole("progressbar")).toBeNull();

    // The run finishes in the background; the page returns and the consumer
    // refetches, handing the sheet the completed record.
    setPageVisibility("visible");
    // Between the flip and the refetched record there is one transient
    // resubscribe (the sheet still believes the run is in flight, so it
    // reconnects and lets the server replay — by design, not a leak).
    expect(instances.length).toBe(2);
    rerender(
      <I18nProvider>
        <RunDetailSheet run={DONE_RUN} open onClose={() => {}} />
      </I18nProvider>
    );
    // Terminal content: the history line through the real builder, and the
    // live machinery is gone again — the terminal record unmounted the
    // resubscribed consumer (instances[1] closed; nothing else ever opened).
    expect(screen.getByText(/plex backed up: 4\.7 GB in 1h 0m/)).toBeTruthy();
    expect(screen.queryByRole("progressbar")).toBeNull();
    expect(instances.length).toBe(2);
    expect(instances[1].closed).toBe(true);
  });

  // --- fresh-success CheckDraw gate: only a WITNESSED transition draws ------
  // (.glim-check-draw is CheckDraw's stroke path; the sheet's only other
  // CheckDraw sits behind a verify press these tests never make.)

  it("witnessed running to success while open draws the check; the next open starts clean", () => {
    const draws = () => document.body.querySelectorAll(".glim-check-draw").length;
    const sheet = (run: Run, open: boolean) => (
      <I18nProvider>
        <RunDetailSheet run={run} open={open} onClose={() => {}} />
      </I18nProvider>
    );
    const { rerender } = render(sheet(makeRun({ status: "running", finishedAt: null }), true));
    expect(draws()).toBe(0);
    // The transition lands while the sheet is OPEN: the user is watching, the
    // check draws.
    rerender(sheet(makeRun({ status: "success" }), true));
    expect(draws()).toBe(1);
    // Close + reopen consumes it: the animation belongs to the session that
    // witnessed the completion, never to the next one.
    rerender(sheet(makeRun({ status: "success" }), false));
    rerender(sheet(makeRun({ status: "success" }), true));
    expect(draws()).toBe(0);
  });

  it("a running to success flip that lands while closed never animates on reopen", () => {
    const draws = () => document.body.querySelectorAll(".glim-check-draw").length;
    const sheet = (run: Run, open: boolean) => (
      <I18nProvider>
        <RunDetailSheet run={run} open={open} onClose={() => {}} />
      </I18nProvider>
    );
    // Hosts keep the sheet MOUNTED when closed and keep feeding it refreshed
    // records from their poll — modeled exactly: the record flips to success
    // behind the closed sheet...
    const { rerender } = render(sheet(makeRun({ status: "running", finishedAt: null }), false));
    rerender(sheet(makeRun({ status: "success" }), false));
    // ...so the reopen shows the completed run WITHOUT the animation.
    rerender(sheet(makeRun({ status: "success" }), true));
    expect(draws()).toBe(0);
  });
});
