// @vitest-environment jsdom
// What anomaly detection shows on the ZFS page: the item's findings beside its
// name, each dataset's own beside it in the tree, the item's own settings in
// its editor, and a link from a finding that lands on the dataset's last good
// backup in the restore panel.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { AdvancedProvider } from "../lib/advanced";
import { I18nProvider, en } from "../lib/i18n";
import { isolateLtr } from "../lib/ltrFragments";
import { ToastProvider } from "../lib/toast";
import { AnomalyProvider } from "../lib/useAnomalies";
import type {
  AnomalyItem,
  AnomalySeriesInfo,
  AnomalyView,
  ZFSDatasetView,
  ZFSMemberView,
  ZFSRestorePoint,
} from "../lib/api";

class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

const ROOT = "cache/appdata";
const CHILD = "cache/appdata/plex";

let anomalyItems: AnomalyItem[] = [];
let openFindings: AnomalyView[] = [];
let points: ZFSRestorePoint[] = [];

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listZFSDatasets: () => Promise.resolve({ ok: true, datasets: [zfsItem()] }),
    zfsConnection: () =>
      Promise.resolve({
        ok: true, code: "ok", target: "root@tower", uriTarget: "", version: "2.3.4", detail: "",
        zfsBinary: "/usr/sbin/zfs", propagation: "slave", unpropagated: [],
      }),
    getVMSSH: () => Promise.resolve({ ok: true, host: "tower", publicKey: "ssh-ed25519 AAAA bv" }),
    zfsHostDatasets: () => Promise.resolve({ ok: false, available: false }),
    getSettings: () => Promise.resolve({ ok: true, hostMountRoot: "/host", settings: { perItemSchedules: false } }),
    listRuns: () => Promise.resolve({ ok: true, runs: [] }),
    listRepos: () => Promise.resolve({ ok: true, repos: [] }),
    listContainers: () => Promise.resolve({ ok: true, containers: [] }),
    zfsRestorePoints: () => Promise.resolve({ ok: true, points }),
    getAnomalySummary: () =>
      Promise.resolve({
        ok: true,
        summary: {
          enabled: true, ready: true, generation: 1, open: { critical: 1, warning: 0, info: 0 },
          recoveredCritical: 0, learningItems: 0, retentionHeld: 1, evalErrors: 0, notifyMuted: false,
          backfill: { slots: 0, done: 0, failed: 0, filled: 0, withoutSummary: 0 }, unmeasuredVolumes: [],
        },
      }),
    getAnomalyItems: () => Promise.resolve({ ok: true, items: anomalyItems }),
    getAnomalies: () => Promise.resolve({ ok: true, anomalies: openFindings, nextCursor: "" }),
  };
});

const { ZFS } = await import("./ZFS");

function member(dataset: string, relPath: string): ZFSMemberView {
  return {
    dataset, relPath, hostMountpoint: `/mnt/${dataset}`, outcome: "backed-up",
    isNew: false, usedByDataset: 4096, lastBackupAt: 1_700_000_000,
  };
}

function zfsItem(): ZFSDatasetView {
  return {
    id: "z1", dataset: ROOT, enabled: true, excludes: [], scheduleCadence: "", repo: "",
    repoEffective: "ZFS datasets", stopContainers: [], restartPending: [], excludedChildren: [],
    hookContainer: "", preSnapshot: "", postSnapshot: "", hostMountpoint: `/mnt/${ROOT}`,
    lastBackup: 1_700_000_000, lastRunStatus: "success", lastCheckCode: "ok", lastCheckDetail: "",
    lastCheckAt: 1_700_000_000, leftoverCount: 0, safetyCount: 0, safetyOldestAt: 0,
    members: [member(ROOT, ""), member(CHILD, "plex")],
    effectiveSchedule: { kind: "domain", spec: "0 3 * * *", alsoSpec: "" },
  };
}

function series(part: string, over: Partial<AnomalySeriesInfo> = {}): AnomalySeriesInfo {
  return {
    part,
    learning: { samples: 10, needed: 10 },
    typical: { sourceBytes: 20 << 30, resticMs: 60_000 },
    open: { critical: 0, warning: 0, info: 0 },
    retentionHeld: false,
    ...over,
  };
}

function anomalyItem(): AnomalyItem {
  return {
    targetId: "z1", domain: "zfs", name: ROOT, scheduled: true, sensitivity: "", effective: "balanced",
    notifyMin: "", effectiveNotifyMin: "critical",
    learning: { samples: 10, needed: 10, newData: 10, source: 10, duration: 10, noData: false },
    typical: { sourceBytes: null, newDataBytes: null, resticMs: null },
    dump: null,
    datasets: [series(ROOT), series(CHILD, { open: { critical: 1, warning: 0, info: 0 }, retentionHeld: true })],
    open: { critical: 0, warning: 0, info: 0 },
    retentionHeld: true, selectionSince: 0, expectations: [],
  };
}

function point(stamp: string, time: number, childSnap: string): ZFSRestorePoint {
  return {
    stamp, time,
    members: [
      { dataset: ROOT, relPath: "", snapshotId: `${stamp}-root`, outcome: "backed-up" },
      { dataset: CHILD, relPath: "plex", snapshotId: childSnap, outcome: "backed-up" },
    ],
  };
}

async function renderPage(search = "") {
  window.history.pushState({}, "", `/zfs${search}`);
  await act(async () => {
    render(
      <I18nProvider>
        <AdvancedProvider>
          <ToastProvider>
            <MemoryRouter>
              <AnomalyProvider>
                <ZFS />
              </AnomalyProvider>
            </MemoryRouter>
          </ToastProvider>
        </AdvancedProvider>
      </I18nProvider>,
    );
  });
  await screen.findByText(ROOT, { selector: "span" });
}

function badgeLabel(name: string, n: number): string {
  return en["anomaly.itemBadgeAria"].replace("{name}", name).replace("{n}", String(n));
}

beforeEach(() => {
  anomalyItems = [anomalyItem()];
  openFindings = [];
  points = [
    point("bombvault-20260921030000", 1_789_086_400, "child-emptied"),
    point("bombvault-20260920030000", 1_789_000_000, "child-good"),
  ];
  localStorage.clear();
});

afterEach(() => {
  cleanup();
  window.history.pushState({}, "", "/");
});

describe("ZFS page anomalies", () => {
  it("counts a dataset's findings beside the item and marks the dataset in the tree", async () => {
    await renderPage();
    const itemBadge = await screen.findByRole("link", { name: badgeLabel(ROOT, 1) });
    expect(itemBadge.getAttribute("href")).toBe("/anomalies?scope=item:z1#findings");

    fireEvent.click(screen.getByRole("button", { name: en["zfs.membersSummary"].replace("{n}", "2") }));
    const childLine = screen.getByRole("link", { name: badgeLabel(CHILD, 1) }).closest("li") as HTMLElement;
    expect(within(childLine).getByText(en["anomaly.retentionPaused"])).toBeTruthy();
    expect(screen.queryByRole("link", { name: badgeLabel(ROOT, 0) })).toBeNull();
  });

  it("offers the item's own sensitivity in its editor", async () => {
    await renderPage();
    await screen.findByRole("link", { name: badgeLabel(ROOT, 1) });
    fireEvent.click(screen.getByRole("button", { name: en["common.edit"] }));
    expect(screen.getByRole("combobox", { name: en["anomaly.items.sensitivity"] })).toBeTruthy();
  });

  it("opens the restore panel on the dataset's last good backup and marks the emptied one", async () => {
    openFindings = [
      {
        id: "an-1", detector: "source", metric: "source_bytes_shrink", severity: "critical", state: "open",
        scopeKind: "zfsds", scopeId: CHILD, targetId: "z1", domain: "zfs", name: ROOT, part: CHILD,
        targetName: "", runId: "r2", lastRunId: "r2", lastRunAt: 1_789_086_400,
        lastGood: { runId: "r1", snapshotId: "child-good", at: 1_789_000_000 },
        flaggedSnapshots: ["child-emptied"],
        observed: 0, expected: 20 << 30, threshold: 2 << 30, samples: 5, sensitivity: "balanced",
        details: { collapse: true }, occurrences: 1, firstSeenAt: 1, lastSeenAt: 1, recoveredAt: 0,
        resolvedAt: 0, ackedAt: 0, clearedAt: 0, ackNote: "", notifiedAt: 0, expectable: true,
        retentionHeld: true, stillPresent: false,
      },
    ];
    await renderPage(`?restore=child-good&item=${encodeURIComponent(ROOT)}&dataset=${encodeURIComponent(CHILD)}`);

    const panel = await screen.findByRole("group", { name: en["zfs.restore.title"].replace("{dataset}", ROOT) });
    const pointField = await within(panel).findByRole("combobox", { name: en["zfs.restore.point"] });
    expect(pointField.textContent).toContain(new Date(1_789_000_000 * 1000).toLocaleString());
    expect(within(panel).getByRole("combobox", { name: en["zfs.restore.dataset"] }).textContent).toContain(CHILD);
    expect(within(panel).queryByText(en["anomaly.snapshotFlagged"])).toBeNull();

    fireEvent.click(pointField);
    fireEvent.click(await screen.findByRole("option", { name: new RegExp(new Date(1_789_086_400 * 1000).toLocaleString().replace(/[.*+?^${}()|[\]\\]/g, "\\$&")) }));
    expect(await within(panel).findByText(en["anomaly.snapshotFlagged"])).toBeTruthy();
  });
  it("says that a dataset's linked backup is gone and opens on the nearest one", async () => {
    await renderPage(
      `?restore=child-pruned&at=1788990000&item=${encodeURIComponent(ROOT)}&dataset=${encodeURIComponent(CHILD)}`
    );

    const panel = await screen.findByRole("group", { name: en["zfs.restore.title"].replace("{dataset}", ROOT) });
    const pointField = await within(panel).findByRole("combobox", { name: en["zfs.restore.point"] });
    expect(pointField.textContent).toContain(new Date(1_789_000_000 * 1000).toLocaleString());
    expect(within(panel).getByRole("combobox", { name: en["zfs.restore.dataset"] }).textContent).toContain(CHILD);
    const notice = within(panel).getByRole("status");
    expect(notice.textContent).toContain(
      en["restore.missingPoint"].replace("{date}", isolateLtr(new Date(1_788_990_000 * 1000).toLocaleString()))
    );
    expect(notice.textContent).toContain(
      en["restore.nearestPoint"].replace("{date}", isolateLtr(new Date(1_789_000_000 * 1000).toLocaleString()))
    );
  });
});
