// @vitest-environment jsdom
/**
 * The Verify tab of an entry's page.
 *
 * The tab answers whether the entry's backups are any good. Each card has to
 * say so for the entry on the page and for no other, and has to admit when
 * there is nothing to say yet: no backup, no check, nothing learned.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import type { ItemChecks, Run } from "../lib/api";
import { ENTRY_ROUTE } from "../lib/backupEntry";
import { humanBytes } from "../lib/forecast";
import { I18nProvider, en } from "../lib/i18n";
import { AnomalyProvider } from "../lib/useAnomalies";
import { NOW, ZFS_ID, backupRun, serveEntries, timelineOf, watched } from "./backupEntry.testsupport";

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getBackupItem: vi.fn(),
    listContainers: vi.fn(),
    listVMs: vi.fn(),
    listFileSets: vi.fn(),
    listZFSDatasets: vi.fn(),
    listItemRuns: vi.fn(),
    listRuns: vi.fn(),
    getSettings: vi.fn(),
    getAnomalies: vi.fn(),
    getAnomalySummary: vi.fn(),
    getAnomalyItems: vi.fn(),
    getAnomalyItemSeries: vi.fn(),
    getTimeline: vi.fn(),
    getItemChecks: vi.fn(),
    probeItem: vi.fn(),
    getSizeBreakdown: vi.fn(),
    zfsRunMembers: vi.fn(),
    getZFSReplica: vi.fn(),
  };
});

vi.mock("../lib/toast", () => ({ useToast: () => ({ push: () => {} }) }));

const api = await import("../lib/api");
const mocks = {
  getBackupItem: vi.mocked(api.getBackupItem),
  listContainers: vi.mocked(api.listContainers),
  listVMs: vi.mocked(api.listVMs),
  listFileSets: vi.mocked(api.listFileSets),
  listZFSDatasets: vi.mocked(api.listZFSDatasets),
  listItemRuns: vi.mocked(api.listItemRuns),
  getSettings: vi.mocked(api.getSettings),
};
const getAnomalySummary = vi.mocked(api.getAnomalySummary);
const getAnomalyItems = vi.mocked(api.getAnomalyItems);
const getItemChecks = vi.mocked(api.getItemChecks);
const probeItem = vi.mocked(api.probeItem);
const getSizeBreakdown = vi.mocked(api.getSizeBreakdown);
const zfsRunMembers = vi.mocked(api.zfsRunMembers);

const { BackupEntryPage } = await import("./BackupEntry");

window.matchMedia = ((query: string) => ({
  matches: true,
  media: query,
  onchange: null,
  addListener: () => {},
  removeListener: () => {},
  addEventListener: () => {},
  removeEventListener: () => {},
  dispatchEvent: () => false,
})) as unknown as typeof window.matchMedia;

class NoopEventSource {
  onmessage: unknown = null;
  close() {}
}

async function renderVerify(entry: string) {
  await act(async () => {
    render(
      <I18nProvider>
        <MemoryRouter initialEntries={[`/backups/${entry}?tab=verify`]}>
          <AnomalyProvider>
            <Routes>
              <Route path={ENTRY_ROUTE} element={<BackupEntryPage />} />
            </Routes>
          </AnomalyProvider>
        </MemoryRouter>
      </I18nProvider>
    );
  });
}

async function press(el: HTMLElement) {
  await act(async () => {
    fireEvent.click(el);
  });
}

const card = (title: string) => screen.getByRole("heading", { level: 2, name: new RegExp(`^${title}`) }).parentElement!;
const runRows = () => screen.queryAllByTestId("entry-run");

const MB = 1024 ** 2;
const PROBE = { targetId: "tg-immich", domain: "container", at: NOW - 3600, ok: true, detail: "", snapshotId: "s", files: 12, bytes: 1 };

function checksOf(over: Partial<ItemChecks> = {}): ItemChecks {
  return { targetId: "tg-immich", domain: "container", name: "immich", ...over } as ItemChecks;
}

function watching(enabled = true) {
  getAnomalySummary.mockResolvedValue({ ok: true, summary: { enabled, generation: 1 } } as never);
}

beforeEach(() => {
  localStorage.clear();
  vi.stubGlobal("EventSource", NoopEventSource);
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => ({ ok: true, json: async () => ({ ok: false }) }))
  );
  serveEntries(mocks, { runs: [backupRun("r0", 0, { bytes: 7 * MB, sourceBytes: 65 * MB })] });
  vi.mocked(api.listRuns).mockResolvedValue({ ok: true, runs: [] });
  vi.mocked(api.getAnomalies).mockResolvedValue({ ok: true, anomalies: [], nextCursor: "" });
  getAnomalySummary.mockResolvedValue({ ok: false });
  getAnomalyItems.mockResolvedValue({ ok: true, items: [] });
  vi.mocked(api.getAnomalyItemSeries).mockResolvedValue({ ok: true, series: [] });
  vi.mocked(api.getTimeline).mockResolvedValue({ ok: true, places: [], rows: [] });
  getItemChecks.mockResolvedValue({ ok: true, items: [checksOf({ probe: PROBE })] });
  probeItem.mockResolvedValue({ ok: true, probe: PROBE });
  getSizeBreakdown.mockResolvedValue({ ok: true, breakdown: { state: "none", path: "", size: 0, files: 0, added: 0, children: [] } });
  zfsRunMembers.mockResolvedValue({ ok: true, members: [], windowSeconds: 4 } as never);
  vi.mocked(api.getZFSReplica).mockRejectedValue(new Error("no replica"));
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe("the checks", () => {
  it("show the last restore check of the entry and run a new one", async () => {
    await renderVerify("container/immich");
    const checks = within(card(en["entry.verify.checks"]));
    expect(await checks.findByText(/Restore checked/)).toBeTruthy();
    await press(checks.getByRole("button", { name: en["checks.probeNow"] }));
    expect(probeItem).toHaveBeenCalledWith("tg-immich");
  });

  it("stand as rows that name the check, say what it found and explain it in a bubble", async () => {
    await renderVerify("container/immich");
    const checks = within(card(en["entry.verify.checks"]));
    const probe = (await checks.findByText(/Restore checked/)).closest("[data-search-row]") as HTMLElement;
    expect(probe.getAttribute("data-search-row")).toBe(en["overview.check.drill"]);
    expect(within(probe).getByLabelText(en["checks.probeHint"])).toBeTruthy();
    expect(within(probe).getByRole("button", { name: en["checks.probeNow"] })).toBeTruthy();
  });

  it("offer the start test for a container and for nothing else", async () => {
    getItemChecks.mockResolvedValue({
      ok: true,
      items: [checksOf({ probe: PROBE }), checksOf({ targetId: "tg-win11", domain: "vm", name: "win11", probe: PROBE })],
    });
    await renderVerify("container/immich");
    expect(await screen.findByRole("button", { name: en["checks.startNow"] })).toBeTruthy();
    cleanup();

    await renderVerify("vm/win11");
    await within(card(en["entry.verify.checks"])).findByRole("button", { name: en["checks.probeNow"] });
    expect(screen.queryByRole("button", { name: en["checks.startNow"] })).toBeNull();
  });

  it("say that there is nothing to check before the first backup", async () => {
    serveEntries(mocks, { runs: [] });
    await renderVerify("container/gitea");
    expect(within(card(en["entry.verify.checks"])).getByText(en["breakdown.none"])).toBeTruthy();
    expect(within(card(en["breakdown.title"])).getByText(en["breakdown.none"])).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["checks.probeNow"] })).toBeNull();
  });
});

describe("the anomalies", () => {
  it("show how closely the entry is watched and that its old backups are held", async () => {
    watching();
    getAnomalyItems.mockResolvedValue({ ok: true, items: [watched({ retentionHeld: true })] });
    await renderVerify("container/immich");
    const anomalies = within(card(en["anomaly.title"]));
    expect(await anomalies.findByText(en["anomaly.items.sensitivity"])).toBeTruthy();
    expect(anomalies.getByText(en["anomaly.retentionPaused"])).toBeTruthy();
  });

  it("say so when detection has not seen the entry yet", async () => {
    watching();
    await renderVerify("container/immich");
    expect(await within(card(en["anomaly.title"])).findByText(en["entry.verify.notWatched"])).toBeTruthy();
  });

  it("say so when detection is switched off", async () => {
    watching(false);
    await renderVerify("container/immich");
    expect(await within(card(en["anomaly.title"])).findByText(en["anomaly.off"])).toBeTruthy();
  });
});

describe("the recent runs", () => {
  const RUNS: Run[] = [
    backupRun("ok", 0),
    backupRun("failed", 1, { status: "failed", snapshotId: "", error: "boom" }),
    backupRun("cancelled", 2, { status: "cancelled", snapshotId: "" }),
    backupRun("slow", 3, { bottleneck: { kind: "cpu", share: 0.97 } }),
    backupRun("dump", 3, { kind: "dbdump" }),
    backupRun("restore", 4, { kind: "restore" }),
    backupRun("import", 5, { kind: "import" }),
  ];

  it("list backups and imports with how each ended", async () => {
    serveEntries(mocks, { runs: RUNS });
    await renderVerify("container/immich");
    expect(runRows()).toHaveLength(5);
    expect(within(runRows()[1]).getByText(en["run.statusFailed"])).toBeTruthy();
    expect(within(runRows()[2]).getByText(en["run.statusCancelled"])).toBeTruthy();
    expect(within(runRows()[3]).getByText(new RegExp(en["bottleneck.slow"]))).toBeTruthy();
    expect(within(runRows()[4]).getByText(en["run.kindImport"])).toBeTruthy();
  });

  it("show no more than the eight newest", async () => {
    serveEntries(mocks, { runs: Array.from({ length: 12 }, (_, i) => backupRun(`r${i}`, i)) });
    await renderVerify("container/immich");
    expect(runRows()).toHaveLength(8);
  });

  it("open a run in a window with what was recorded about it", async () => {
    serveEntries(mocks, { runs: RUNS });
    await renderVerify("container/immich");
    await press(runRows()[0]);
    const window = screen.getByRole("dialog");
    expect(within(window).getByText(en["run.statSnapshot"])).toBeTruthy();
    expect(within(window).getByText(RUNS[0].snapshotId.slice(0, 8))).toBeTruthy();
    await press(within(window).getByRole("button", { name: en["common.close"] }));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("lead from a run to the restore of the backup it wrote", async () => {
    serveEntries(mocks, { runs: RUNS });
    vi.mocked(api.getTimeline).mockResolvedValue(timelineOf(RUNS));
    await renderVerify("container/immich");
    await press(runRows()[0]);
    await press(within(screen.getByRole("dialog")).getByRole("button", { name: en["restore.open"] }));
    expect(await screen.findByRole("dialog", { name: en["entry.restore.title"].replace("{name}", "immich") })).toBeTruthy();
  });

  it("open a ZFS run on what happened to its datasets", async () => {
    const run = backupRun("z0", 0, { domain: "zfs", target: "tank/appdata" });
    serveEntries(mocks, { runs: [run] });
    await renderVerify(`zfs/${ZFS_ID}`);
    await press(runRows()[0]);
    expect(zfsRunMembers).toHaveBeenCalledWith("z0");
    expect(await within(screen.getByRole("dialog")).findByText(en["zfs.window"].replace("{seconds}", "4"))).toBeTruthy();
  });

  it("say so when the entry never ran", async () => {
    serveEntries(mocks, { runs: [] });
    await renderVerify("container/gitea");
    expect(within(card(en["run.recentTitle"])).getByText(en["dashboard.noRuns"])).toBeTruthy();
  });
});

describe("the size", () => {
  it("names what the newest backup read and what it stored", async () => {
    await renderVerify("container/immich");
    const size = within(card(en["breakdown.title"]));
    expect(
      size.getByText(
        en["entry.dedup.figures"].replace("{read}", humanBytes(65 * MB)).replace("{added}", humanBytes(7 * MB))
      )
    ).toBeTruthy();
    expect(size.getByRole("img", { name: en["entry.dedup.label"] })).toBeTruthy();
  });

  it("does not deny the backup it has just named when the server has no folders to list", async () => {
    await renderVerify("container/immich");
    await waitFor(() => expect(getSizeBreakdown).toHaveBeenCalled());
    const size = within(card(en["breakdown.title"]));
    expect(size.getByRole("img", { name: en["entry.dedup.label"] })).toBeTruthy();
    expect(size.queryByText(en["breakdown.none"])).toBeNull();
    expect(size.queryByText(en["breakdown.running"])).toBeNull();
  });

  it("asks at once for the folders of a kind that has them", async () => {
    await renderVerify("container/immich");
    await waitFor(() => expect(getSizeBreakdown).toHaveBeenCalledWith("containers", "immich", "", false));
    expect(within(card(en["breakdown.title"])).queryByRole("button", { name: en["breakdown.title"] })).toBeNull();
  });

  it("leaves the folders out for a kind the server cannot break down", async () => {
    await renderVerify("flash/flash");
    expect(within(card(en["breakdown.title"])).getByRole("img", { name: en["entry.dedup.label"] })).toBeTruthy();
    expect(getSizeBreakdown).not.toHaveBeenCalled();
  });
});
