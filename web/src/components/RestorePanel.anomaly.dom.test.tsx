// @vitest-environment jsdom
// After a source collapsed or was encrypted in place, the newest snapshot is
// the one a hurried restore picks, and it is the damaged one. The panel marks
// it, and the link from the finding opens the last good snapshot ready to go.
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, within } from "@testing-library/react";
import { AdvancedProvider } from "../lib/advanced";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { AnomalySummary, AnomalyView } from "../lib/api";

class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

const snap = (id: string, time: string) => ({ id, time, paths: [], tags: ["container:plex"], hostname: "tower" });

const SUMMARY: AnomalySummary = {
  enabled: true,
  ready: true,
  generation: 1,
  open: { critical: 1, warning: 0, info: 0 },
  recoveredCritical: 0,
  learningItems: 0,
  retentionHeld: 1,
  evalErrors: 0,
  notifyMuted: false,
  backfill: { slots: 1, done: 1, failed: 0, filled: 0, withoutSummary: 0 },
  unmeasuredVolumes: [],
};

const COLLAPSE = {
  id: "an-1",
  metric: "source_bytes_shrink",
  severity: "critical",
  state: "open",
  runId: "run-9",
  lastRunId: "run-9",
  lastGood: { runId: "run-8", snapshotId: "good000000aa", at: 1700000000 },
  flaggedSnapshots: ["bad0000000bb"],
} as unknown as AnomalyView;

const DUMP_COLLAPSE = {
  ...COLLAPSE,
  id: "an-2",
  metric: "dump_bytes_shrink",
  scopeKind: "dump",
  lastGood: { runId: "run-6", snapshotId: "dumpgood0001", at: 1700000000 },
  flaggedSnapshots: ["dumpbad00002"],
} as unknown as AnomalyView;

const dump = (id: string, time: string) => ({
  id,
  time,
  engine: "postgres",
  image: "postgres:16",
  version: "16.4",
  databases: ["plex"],
  bytes: 1024,
  damaged: false,
});

let dumpsOnServer: ReturnType<typeof dump>[] = [];

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listSnapshots: () =>
      Promise.resolve({
        ok: true,
        snapshots: [snap("bad0000000bb", "2026-09-02T02:00:00Z"), snap("good000000aa", "2026-09-01T02:00:00Z")],
      }),
    listDbDumps: () => Promise.resolve({ ok: true, dumps: dumpsOnServer }),
    listRuns: () => Promise.resolve({ ok: true, runs: [] }),
    getSettings: () => Promise.resolve({ ok: false }),
    listOffsiteTargets: () => Promise.resolve({ ok: true, targets: [] }),
    getAnomalySummary: () => Promise.resolve({ ok: true, summary: SUMMARY }),
    getAnomalies: () => Promise.resolve({ ok: true, anomalies: [COLLAPSE, DUMP_COLLAPSE], nextCursor: "" }),
  };
});

const { RestorePanel } = await import("./RestorePanel");
const { AnomalyProvider } = await import("../lib/useAnomalies");

type PanelProps = Parameters<typeof RestorePanel>[0];
const t = ((key: string) => en[key as keyof typeof en] ?? key) as unknown as PanelProps["t"];

async function renderPanel(over: Partial<PanelProps> = {}) {
  await act(async () => {
    render(
      <I18nProvider>
        <AdvancedProvider>
          <ToastProvider>
            <AnomalyProvider>
              <RestorePanel name="plex" t={t} open {...over} />
            </AnomalyProvider>
          </ToastProvider>
        </AdvancedProvider>
      </I18nProvider>
    );
  });
}

function rowOf(shortId: string): HTMLElement {
  return screen.getByText(shortId).closest("div.border-b") as HTMLElement;
}

afterEach(() => {
  cleanup();
  dumpsOnServer = [];
});

it("marks the flagged snapshot and preselects the last good one", async () => {
  await renderPanel({ preselect: "good000000aa" });

  const bad = await screen.findByText("bad00000");
  expect(within(rowOf("bad00000")).getByText(en["anomaly.snapshotFlagged"])).toBeTruthy();
  expect(within(rowOf("good0000")).queryByText(en["anomaly.snapshotFlagged"])).toBeNull();
  expect(bad).toBeTruthy();

  // Only the preselected row opens its restore choices.
  expect(within(rowOf("good0000")).getByText(en["restore.inPlaceHint"])).toBeTruthy();
  expect(within(rowOf("bad00000")).queryByText(en["restore.inPlaceHint"])).toBeNull();
});

it("opens nothing on its own without a request", async () => {
  await renderPanel();
  await screen.findByText("bad00000");
  expect(screen.queryByText(en["restore.inPlaceHint"])).toBeNull();
});

it("marks the flagged dump and singles out the one a dump finding links to", async () => {
  dumpsOnServer = [dump("dumpbad00002", "2026-09-02T02:00:00Z"), dump("dumpgood0001", "2026-09-01T02:00:00Z")];
  await renderPanel({ preselectDump: "dumpgood0001" });

  await screen.findByText("dumpbad0");
  expect(within(rowOf("dumpbad0")).getByText(en["anomaly.snapshotFlagged"])).toBeTruthy();
  expect(within(rowOf("dumpgood")).queryByText(en["anomaly.snapshotFlagged"])).toBeNull();
  expect(rowOf("dumpgood").className).toContain("bg-carbon-surface2");
  expect(rowOf("dumpbad0").className).not.toContain("bg-carbon-surface2");
  // A dump link opens no snapshot's restore choices.
  expect(screen.queryByText(en["restore.inPlaceHint"])).toBeNull();
});

// 2026-09-01T12:00:00Z, closer to the good snapshot than to the bad one.
const GONE_AT = 1788264000;

it("says that a linked backup is gone and names the nearest one", async () => {
  await renderPanel({ preselect: "gone0000cc", preselectAt: GONE_AT });
  await screen.findByText("bad00000");
  const notice = screen.getByRole("status");
  expect(notice.textContent).toContain(
    en["restore.missingPoint"].replace("{date}", new Date(GONE_AT * 1000).toLocaleString())
  );
  expect(notice.textContent).toContain(
    en["restore.nearestPoint"].replace("{date}", new Date("2026-09-01T02:00:00Z").toLocaleString())
  );
  expect(screen.queryByText(en["restore.inPlaceHint"])).toBeNull();
});

it("says nothing about a linked backup that is still there", async () => {
  await renderPanel({ preselect: "good000000aa", preselectAt: GONE_AT });
  await screen.findByText("bad00000");
  expect(screen.queryByRole("status")).toBeNull();
});

it("says that a linked dump is gone and names the nearest one", async () => {
  dumpsOnServer = [dump("dumpbad00002", "2026-09-02T02:00:00Z"), dump("dumpgood0001", "2026-09-01T02:00:00Z")];
  await renderPanel({ preselectDump: "dumpgone0003", preselectAt: GONE_AT });
  await screen.findByText("dumpbad0");
  const notice = screen.getByRole("status");
  expect(notice.textContent).toContain(
    en["restore.missingPoint"].replace("{date}", new Date(GONE_AT * 1000).toLocaleString())
  );
  expect(notice.textContent).toContain(
    en["restore.nearestPoint"].replace("{date}", new Date("2026-09-01T02:00:00Z").toLocaleString())
  );
});
