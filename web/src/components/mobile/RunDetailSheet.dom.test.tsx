// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// RunDetailSheet; jsdom behavior proofs for the phone run-detail sheet.
//
// The sheet is a pure view over a Run record composed entirely from existing
// pieces, so these tests assert the composition contracts that can drift:
//   - the honest stat triad (the frozen-record substitutions;
//     humanBytes / formatDuration / mono slice, and nothing invented),
//   - the activity log rendered through the real buildLogLines pipeline (the
//     same builder ActivityLog.tsx uses; mocked only at the browser boundary,
//     see below),
//   - the failed path surfacing the backend's scrubbed reason verbatim via
//     runReason, with the direction contract (own sentence → page direction,
//     untranslated restic text → dir="ltr"),
//   - the domain honesty matrix (browse/restore only where a file-listing API
//     exists; verify only where checkDomain's union has the domain),
//   - the ≥44px tonal action rows and the restore entry's secondary placement
//     (in the scroll body, before the footer rows; never accent); styling
//     contracts asserted as targeted class tokens, the documented jsdom
//     exception (BottomSheet.dom.test.tsx precedent: jsdom computes no
//     geometry, so the observable form of a styling contract is the token).
//
// EventSource stub: lifted from OffsiteIndicator.dom.test.tsx; jsdom does not
// implement EventSource, and the live-section test drives the real
// lib/progress.ts singleton through `source.onmessage`, exactly the path a
// backend push takes. Nothing about the component under test is stubbed.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { RunDetailSheet } from "./RunDetailSheet";
import { en, I18nProvider } from "../../lib/i18n";
import type { Run } from "../../lib/api";

// The two file-listing endpoints are replaced with manually-resolved
// deferreds so a test can hold a listing in flight and resolve it late;
// the late-response scenario itself. The verify and restore endpoints get
// the same treatment (the verify-write guard and the restore-path tests
// hold requests in flight across close/rerender). Every other export
// (types, ApiError, formatters) stays the real module.
const listingControl = vi.hoisted(() => {
  const pending: { resolve: (value: unknown) => void }[] = [];
  return {
    pending,
    deferred() {
      let resolve!: (value: unknown) => void;
      const promise = new Promise((res) => {
        resolve = res;
      });
      const entry = { resolve, promise };
      pending.push(entry);
      return entry.promise;
    },
  };
});

// Deferreds for the check/restore POSTs; tagged so a test with several in
// flight can pick the right one.
const apiControl = vi.hoisted(() => {
  const pending: { resolve: (value: unknown) => void; kind: string }[] = [];
  return {
    pending,
    deferred(kind: string) {
      let resolve!: (value: unknown) => void;
      const promise = new Promise((res) => {
        resolve = res;
      });
      pending.push({ resolve, kind });
      return promise;
    },
  };
});

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    listSnapshotFiles: () => listingControl.deferred(),
    listSnapshotFilesFileSet: () => listingControl.deferred(),
    checkDomain: () => apiControl.deferred("check"),
    restoreContainerFiles: () => apiControl.deferred("restore-container"),
    restoreFileSetFiles: () => apiControl.deferred("restore-files"),
  };
});

// A finished, successful container backup; every derived string below is
// computed from these fields by the real formatters.
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
  // Set by close(); the observable form of progress.ts's closeSource
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
  listingControl.pending.length = 0;
  apiControl.pending.length = 0;
  // @ts-expect-error -- test-only global stub; jsdom has no EventSource
  global.EventSource = FakeEventSource;
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  // Restore jsdom's prototype getter (default "visible"); the visibility
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
  it("composes the title from the shared helpers; kind + target, no new title key", () => {
    renderSheet(DONE_RUN);
    // runKindLabel("backup") = "Backup" (run.kindBackup) · runTargetText → the
    // container's human target. The interpunct is the separator (user text is
    // em-dash-free by lint law).
    expect(screen.getByText(`Backup · plex`)).toBeTruthy();
  });

  it("renders the honest stat triad, and nothing invented beyond the frozen record", () => {
    renderSheet(DONE_RUN);
    expect(screen.getByText(en["run.statVolume"])).toBeTruthy();
    expect(screen.getByText(en["dashboard.duration"])).toBeTruthy();
    expect(screen.getByText(en["run.statSnapshot"])).toBeTruthy();
    // humanBytes(5_000_000_000) through the real formatter.
    expect(screen.getByText("4.7 GB")).toBeTruthy();
    // formatDuration(3600) through the real formatter.
    expect(screen.getByText("1h 0m")).toBeTruthy();
    // The snapshot tile: 8-char mono slice with the full id as its title.
    const snap = screen.getByText("0f1e2d3c");
    expect(snap.getAttribute("title")).toBe(DONE_RUN.snapshotId);
  });

  it("carries tabular numerals on the tile values and the mono class on the snapshot", () => {
    renderSheet(DONE_RUN);
    expect(screen.getByText("4.7 GB").className).toContain("tabular-nums");
    expect(screen.getByText("1h 0m").className).toContain("tabular-nums");
    expect(screen.getByText("0f1e2d3c").className).toContain("tabular-nums");
    expect(screen.getByText("0f1e2d3c").className).toContain("font-mono");
    // Stat values take the heading role at weight 600 (text-heading
    // font-semibold; the Fab label's typography), never the old text-sm
    // font-medium pair. Pinned as class tokens, the documented jsdom
    // exception (no computed geometry here).
    for (const value of ["4.7 GB", "1h 0m", "0f1e2d3c"]) {
      expect(screen.getByText(value).className).toContain("text-heading");
      expect(screen.getByText(value).className).toContain("font-semibold");
      expect(screen.getByText(value).className).not.toContain("font-medium");
    }
    // The activity log reuses the ActivityLog mono pattern verbatim. Queried
    // on document.body; the sheet portals there (BottomSheet primitive), so
    // the render container itself stays empty.
    expect(document.body.querySelector("div.font-mono.text-xs")).not.toBeNull();
  });

  it("renders the run's history line through the real buildLogLines pipeline", () => {
    renderSheet(DONE_RUN);
    // buildLogLines' en sentence for a finished backup: "{name} backed up:
    // {bytes} in {duration}"; the same text the dashboard's activity log
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

  it("restore entry sits in the scroll body ahead of the footer rows, never accent", () => {
    const { container } = renderSheet(DONE_RUN);
    const restore = screen.getByRole("button", { name: en["snapshots.restore"] });
    // Secondary/tonal: the neutral surface token, and no accent anywhere.
    expect(restore.className).toContain("bg-carbon-surface3");
    expect(/accent/.test(restore.className)).toBe(false);
    // DOM order: the restore entry precedes both footer rows (it is the last
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
    // SnapshotFileTree's filter input is the tree's observable mount. The
    // listing here is the test file's controlled deferred (left pending), so
    // this also pins the in-flight state: the tree is mounted while the
    // listing has not resolved.
    expect(screen.getByRole("textbox")).toBeTruthy();
  });

  it("a late listing for the previous run never overwrites the current run's tree", async () => {
    const runA = makeRun({ id: "a".repeat(32), target: "plex", targetId: "plex-a" });
    const runB = makeRun({ id: "b".repeat(32), target: "jellyfin", targetId: "jellyfin-b" });
    const { rerender } = renderSheet(runA);
    fireEvent.click(screen.getByRole("button", { name: en["recovery.foreignStepBrowse"] }));
    await act(async () => {});
    expect(listingControl.pending.length).toBe(1);
    const listingA = listingControl.pending.shift();

    // The host swaps the sheet's run while A's listing is still in flight;
    // the Dashboard's single-mounted-sheet contract. The effect re-runs for
    // B and its listing goes out.
    rerender(
      <I18nProvider>
        <RunDetailSheet run={runB} open onClose={() => {}} />
      </I18nProvider>
    );
    await act(async () => {});
    expect(listingControl.pending.length).toBe(1);
    const listingB = listingControl.pending.shift();

    // B resolves first and its files render… (file entries, whose full path
    // the tree renders as text; directory entries are split into segments)
    await act(async () => {
      listingB.resolve({
        ok: true,
        files: [{ path: "/config/jellyfin.ini", type: "file", size: 12 }],
      });
    });
    expect(screen.getByTitle("/config/jellyfin.ini")).toBeTruthy();

    // …then A's slow listing finally lands. It must be dropped, not written
    // into B's tree.
    await act(async () => {
      listingA.resolve({
        ok: true,
        files: [{ path: "/config/plex.ini", type: "file", size: 12 }],
      });
    });
    expect(screen.queryByTitle("/config/plex.ini")).toBeNull();
    expect(screen.getByTitle("/config/jellyfin.ini")).toBeTruthy();
  });

  it("no-snapshot runs render placeholders, never a claimed 0 B or a blank tile", () => {
    // The Backup Everything parent run shape: kind "backup", empty snapshot
    // id, zero bytes; nothing was snapshotted by this run.
    renderSheet(
      makeRun({
        id: "c".repeat(32),
        target: "BombVault",
        targetId: "everything",
        domain: "everything",
        snapshotId: "",
        bytes: 0,
      })
    );
    // Exactly two placeholder marks: volume + snapshot. The duration tile
    // still shows the real duration (finishedAt is present).
    const placeholders = screen.getAllByText("—");
    expect(placeholders.length).toBe(2);
    for (const p of placeholders) {
      expect(p.className).toContain("text-carbon-textMuted");
    }
    // humanBytes(0) would have claimed a measured "0 B"; the mono slice
    // would have rendered a blank tile.
    expect(screen.queryByText("0 B")).toBeNull();
    expect(screen.queryByText("0f1e2d3c")).toBeNull();
  });

  it("a real zero-byte backup that HAS a snapshot keeps its honest 0 B", () => {
    renderSheet(makeRun({ bytes: 0 }));
    expect(screen.getByText("0 B")).toBeTruthy();
    expect(screen.getByText("0f1e2d3c")).toBeTruthy();
  });

  it("vm runs: verify offered, browse/restore honestly absent (no file-listing API)", () => {
    // Backend shape (api.ts Run docs): targetId is the 32-hex vm_targets row
    // id, target is the human name; never set equal (that coincidence once
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
        finishedAt: null, // duration tile falls back to the muted absent-data mark
      })
    );
    // No SSE frame yet: the bar renders nothing (inactive), the live section
    // is mounted (one EventSource for the sheet's subscription).
    expect(screen.queryByRole("progressbar")).toBeNull();
    expect(instances.length).toBe(1);
    // A live frame through the real onmessage path → the bar goes
    // indeterminate-active, and the run's live tail line appears.
    act(() => {
      instances[0].emit({ key: "container:plex", phase: "backup", percent: 0, active: true, startedAt: DONE_RUN.startedAt });
    });
    const bar = screen.getByRole("progressbar");
    expect(bar.getAttribute("aria-valuenow")).toBeNull(); // indeterminate at 0%
    // The duration tile degrades to the muted absent-data mark; never blank.
    // (The completion-time span carries the same mark for a null finishedAt;
    // what matters here is that the duration tile's mark is the muted one.)
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

  // The progress key is the run name (run.target), never the 32-hex row id
  // (run.targetId): the backend records vm/files runs under the row id but
  // publishes SSE under "vm:"+name / "files:"+set.Name (internal/api/service.go).
  // These two pins keep the sheet's lookup on the published side of that
  // split: a lookup by targetId would leave both bars dead, since no frame
  // matches.
  it("vm live run: the channel resolves under the run NAME; a 32-hex targetId keys nothing", () => {
    vi.useFakeTimers();
    renderSheet(
      makeRun({
        status: "running",
        finishedAt: null,
        domain: "vm",
        targetId: "a1b2c3d4e5f60718293a4b5c6d7e8f90", // vm_targets.id; what the backend records
        target: "win11", // the name the SSE publishes under ("vm:"+name)
      })
    );
    expect(instances.length).toBe(1);
    act(() => {
      instances[0].emit({ key: "vm:win11", phase: "backup", percent: 30, active: true, startedAt: DONE_RUN.startedAt });
    });
    expect(screen.getByRole("progressbar").getAttribute("aria-valuenow")).toBe("30");
  });

  it("files live run: same name-keyed channel; the set id never appears in the progress key", () => {
    vi.useFakeTimers();
    renderSheet(
      makeRun({
        status: "running",
        finishedAt: null,
        domain: "files",
        targetId: "f0e1d2c3b4a5968778695a4b3c2d1e0f", // file_sets.id; what the backend records
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

  it("hidden page unmounts the live section, which unsubscribes the shared SSE connection", () => {
    vi.useFakeTimers();
    renderSheet(makeRun({ status: "running", finishedAt: null }));
    act(() => {
      instances[0].emit({ key: "container:plex", phase: "backup", percent: 10, active: true, startedAt: DONE_RUN.startedAt });
    });
    expect(screen.getByRole("progressbar")).toBeTruthy();

    setPageVisibility("hidden");
    // The gated subtree is gone...
    expect(screen.queryByRole("progressbar")).toBeNull();
    // ...and unmounting it was the unsubscribe: progress.ts's ref-count hit
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
    // A second EventSource exists: the remount re-subscribed, and the frozen
    // singleton reconnected (whose server snapshot replay repopulates state).
    expect(instances.length).toBe(2);
    expect(instances[1].closed).toBe(false);
    // Fresh subscription starts from an empty cache (closeSource dropped it);
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
    // `run` record from listRuns (the sheet is a pure view over that record;
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
    // reconnects and lets the server replay; by design, not a leak).
    expect(instances.length).toBe(2);
    rerender(
      <I18nProvider>
        <RunDetailSheet run={DONE_RUN} open onClose={() => {}} />
      </I18nProvider>
    );
    // Terminal content: the history line through the real builder, and the
    // live machinery is gone again; the terminal record unmounted the
    // resubscribed consumer (instances[1] closed; nothing else ever opened).
    expect(screen.getByText(/plex backed up: 4\.7 GB in 1h 0m/)).toBeTruthy();
    expect(screen.queryByRole("progressbar")).toBeNull();
    expect(instances.length).toBe(2);
    expect(instances[1].closed).toBe(true);
  });

  // --- fresh-success CheckDraw gate: only a witnessed transition draws ------
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
    // The transition lands while the sheet is open: the user is watching, the
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
    // Hosts keep the sheet mounted when closed and keep feeding it refreshed
    // records from their poll; modeled exactly: the record flips to success
    // behind the closed sheet...
    const { rerender } = render(sheet(makeRun({ status: "running", finishedAt: null }), false));
    rerender(sheet(makeRun({ status: "success" }), false));
    // ...so the reopen shows the completed run without the animation.
    rerender(sheet(makeRun({ status: "success" }), true));
    expect(draws()).toBe(0);
  });

  // --- the live tail is scoped to THIS run's progress key (bug B2, round 3)
  // buildLiveLines emits one line per active key in the map it is handed, so
  // the whole shared map used to give a single run's sheet every other
  // pass member's lines under its title.

  it("during a multi-run pass the sheet's live tail carries only this run's key", () => {
    vi.useFakeTimers();
    renderSheet(makeRun({ status: "running", finishedAt: null }));
    // A Backup Everything pass: the container child's frames share the SSE
    // stream with the VM and flash children. Only the sheet's own key may
    // reach the tail.
    act(() => {
      instances[0].emit({ key: "container:plex", phase: "backup", percent: 10, active: true, startedAt: DONE_RUN.startedAt });
      instances[0].emit({ key: "vm:win11", phase: "backup", percent: 30, active: true, startedAt: DONE_RUN.startedAt });
      instances[0].emit({ key: "flash", phase: "backup", percent: 40, active: true, startedAt: DONE_RUN.startedAt });
    });
    // This run's own line renders (its key survived the slice)...
    expect(screen.getByRole("progressbar").getAttribute("aria-valuenow")).toBe("10");
    // ...and no other pass member's line does.
    expect(screen.queryByText(/win11/i)).toBeNull();
    expect(screen.queryByText(/flash/i)).toBeNull();
  });

  it("the everything parent keeps the whole map, because those keys are its own children", () => {
    vi.useFakeTimers();
    // The parent publishes no key of its own, so a slice would leave the one
    // sheet the thumb-zone trigger deep-links into blank for the whole pass.
    renderSheet(makeRun({ domain: "everything", target: "", status: "running", finishedAt: null }));
    act(() => {
      instances[0].emit({ key: "container:plex", phase: "backup", percent: 10, active: true, startedAt: DONE_RUN.startedAt });
      instances[0].emit({ key: "vm:win11", phase: "backup", percent: 30, active: true, startedAt: DONE_RUN.startedAt });
    });
    expect(screen.getByText(/win11/i)).toBeTruthy();
    expect(screen.getByText(/plex/i)).toBeTruthy();
  });

  // --- the verify write guard (bug B3, round 3) ------------------------------

  it("verify: the row disables for the flight and re-arms after the outcome", async () => {
    renderSheet(DONE_RUN);
    fireEvent.click(screen.getByRole("button", { name: en["integrity.verify"] }));
    await act(async () => {});
    // Mid-flight: the checking copy, and the row disabled so a second check
    // cannot start beside the first.
    const checking = screen.getByRole("button", { name: en["integrity.checking"] }) as HTMLButtonElement;
    expect(checking.disabled).toBe(true);
    const check = apiControl.pending.find((e) => e.kind === "check");
    expect(check).toBeDefined();
    await act(async () => {
      check!.resolve({ ok: true });
    });
    const reArmed = screen.getByRole("button", { name: en["integrity.verify"] }) as HTMLButtonElement;
    expect(reArmed.disabled).toBe(false);
  });

  it("a check that lands after the sheet closed never writes onto the next run", async () => {
    const containerRun = DONE_RUN;
    const vmRun = makeRun({
      domain: "vm",
      target: "win11",
      targetId: "a1b2c3d4e5f60718293a4b5c6d7e8f90",
    });
    const shape = (run: Run, open: boolean) => (
      <I18nProvider>
        <RunDetailSheet run={run} open={open} onClose={() => {}} />
      </I18nProvider>
    );
    const { rerender } = render(shape(containerRun, true));
    fireEvent.click(screen.getByRole("button", { name: en["integrity.verify"] }));
    await act(async () => {});
    expect(apiControl.pending.find((e) => e.kind === "check")).toBeDefined();

    // The user closes the sheet mid-check; the host then shows a VM run.
    // The pending check's failure must be dropped, not painted there.
    rerender(shape(containerRun, false));
    rerender(shape(vmRun, true));
    const check = apiControl.pending.find((e) => e.kind === "check")!;
    await act(async () => {
      check.resolve({ ok: false, error: "restic: repository is locked" });
    });
    expect(screen.queryByText("restic: repository is locked")).toBeNull();
    expect(screen.queryByText(en["verify.failed"])).toBeNull();
  });

  it("a run swap with the sheet open clears the row and drops the check in flight", async () => {
    const vmRun = makeRun({
      id: "run-vm-swap",
      domain: "vm",
      target: "win11",
      targetId: "a1b2c3d4e5f60718293a4b5c6d7e8f90",
    });
    const shape = (run: Run) => (
      <I18nProvider>
        <RunDetailSheet run={run} open onClose={() => {}} />
      </I18nProvider>
    );
    const { rerender } = render(shape(DONE_RUN));
    fireEvent.click(screen.getByRole("button", { name: en["integrity.verify"] }));
    await act(async () => {});
    expect(screen.getByRole("button", { name: en["integrity.checking"] })).toBeTruthy();

    // The host swaps the run without closing: the sheet stays up, so the row
    // has to come back ready rather than stuck on the previous run's check.
    rerender(shape(vmRun));
    expect(screen.getByRole("button", { name: en["integrity.verify"] })).toBeTruthy();
    const check = apiControl.pending.find((e) => e.kind === "check")!;
    await act(async () => {
      check.resolve({ ok: false, error: "restic: repository is locked" });
    });
    expect(screen.queryByText("restic: repository is locked")).toBeNull();
    expect(screen.getByRole("button", { name: en["integrity.verify"] })).toBeTruthy();
  });

  // --- the restore row is a real restore path (bug B6, round 3) --------------

  it("restore: confirm gate, restore API called, the tree stays open, no disclosure semantics", async () => {
    renderSheet(DONE_RUN);
    const restore = screen.getByRole("button", { name: en["snapshots.restore"] });
    // The footer's Browse & restore row is the section's one disclosure
    // owner; the restore row answers nothing about expansion.
    expect(restore.getAttribute("aria-expanded")).toBeNull();
    expect(restore.getAttribute("aria-controls")).toBeNull();

    // Give the tree a selection.
    fireEvent.click(screen.getByRole("button", { name: en["recovery.foreignStepBrowse"] }));
    await act(async () => {});
    const listing = listingControl.pending.shift()!;
    await act(async () => {
      listing.resolve({ ok: true, files: [{ path: "/config/plex.ini", type: "file", size: 12 }] });
    });
    fireEvent.click(screen.getByRole("checkbox", { name: "/config/plex.ini" }));

    // Press Restore: the confirm gate asks before anything fires.
    fireEvent.click(screen.getByRole("button", { name: en["snapshots.restore"] }));
    await act(async () => {});
    const confirmButton = screen
      .getAllByRole("button", { name: en["common.confirm"] })
      .find((b) => b.closest("[role='dialog']") !== null);
    expect(confirmButton).toBeDefined();

    // The tree is not toggled by the restore press: it
    // used to close on press).
    expect(screen.getByRole("checkbox", { name: "/config/plex.ini" })).toBeTruthy();

    await act(async () => {
      fireEvent.click(confirmButton!);
    });
    // The container restore endpoint got the selection, in place.
    const post = apiControl.pending.find((e) => e.kind === "restore-container");
    expect(post).toBeDefined();
    await act(async () => {
      post!.resolve({ ok: true, target: "/mnt/user/appdata/plex" });
    });
    // The ack says the restore started, and the line says exactly that: the
    // server copies detached, so a green "restored" here would be a claim the
    // sheet cannot back. It renders muted, not in the success colour.
    const started = screen.getByText(en["restore.started"]);
    expect(started).toBeTruthy();
    expect(started.className).toContain("text-carbon-textSub");
    expect(started.className).not.toContain("text-statusOk");
    expect(screen.getByRole("checkbox", { name: "/config/plex.ini" })).toBeTruthy();
  });

  it("restore with nothing selected only opens the tree, never fires the API and never closes it", async () => {
    renderSheet(DONE_RUN);
    fireEvent.click(screen.getByRole("button", { name: en["snapshots.restore"] }));
    // The tree opened (the selection is the path's input)...
    expect(screen.getByRole("textbox")).toBeTruthy();
    expect(apiControl.pending.length).toBe(0);
    // ...and pressing again with still nothing selected leaves it open,
    // with no confirm and no request.
    fireEvent.click(screen.getByRole("button", { name: en["snapshots.restore"] }));
    expect(screen.getByRole("textbox")).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["common.confirm"] })).toBeNull();
    expect(apiControl.pending.length).toBe(0);
  });

  // --- the slot wrappers own no horizontal padding (bug B11, round 3) --------

  it("body and footer slots carry no px of their own; the BottomSheet clamp owns the insets", () => {
    renderSheet(DONE_RUN);
    // This sheet's own wrappers, not the primitive's: the primitive never had
    // a px-4, so asserting on its scroll container and footer slot would hold
    // whatever this file does. The body wrapper is the scroll container's one
    // child; the footer wrapper is the one this sheet puts its rows in.
    const bodySlot = (document.body.querySelector("div.overflow-y-auto") as HTMLElement)
      .firstElementChild as HTMLElement;
    expect(bodySlot.className).not.toContain("px-4");
    const footerSlot = screen
      .getByRole("button", { name: en["integrity.verify"] })
      .closest("div.py-4") as HTMLElement;
    expect(footerSlot).not.toBeNull();
    expect(footerSlot.className).not.toContain("px-4");
    // The inner rounded cards keep their own px-4: they inset from the slot,
    // not from the screen.
    const logCard = document.body.querySelector("div.font-mono.text-xs") as HTMLElement;
    expect(logCard.className).toContain("px-4");
  });
});
