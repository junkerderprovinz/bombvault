// @vitest-environment jsdom
// Restore all and the dump-only row can hit a container whose GPU or runtime
// this host lacks. Both offer the restore again without them.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Container, DBDumpView, ForeignInventory, Run } from "../lib/api";

class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

let containersOnServer: Container[] = [];
let foreignInventory: ForeignInventory = { containers: [], vms: [], fileSets: [], zfs: [], dbDumps: [] };
let dumpsOnServer: DBDumpView[] = [];
let runsOnServer: Run[] = [];

const REFUSED =
  "restore failed: the container used a GPU or runtime this host does not have: Error response from daemon: unknown or invalid runtime name: nvidia";

function finishedRun(kind: string, target: string, error = ""): Run {
  return {
    id: `${kind}-${runsOnServer.length + 1}`,
    targetId: "t1",
    kind,
    status: error ? "failed" : "success",
    startedAt: 1_700_000_000,
    finishedAt: 1_700_000_010,
    snapshotId: "s1",
    bytes: 0,
    error,
    acknowledged: false,
    target,
    domain: "container",
  };
}

// plex is refused for its GPU until it is restored without it.
const restore = vi.fn((name: string, ...rest: unknown[]) => {
  const refused = name === "plex" && rest[4] !== true;
  runsOnServer = [...runsOnServer, finishedRun("restore", name, refused ? REFUSED : "")];
  return Promise.resolve({ ok: true, started: true });
});
const importDbDump = vi.fn((name: string) => {
  runsOnServer = [...runsOnServer, finishedRun("dbimport", name)];
  return Promise.resolve({ ok: true, started: true });
});

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getSettings: () =>
      Promise.resolve({
        ok: true,
        settings: {
          restoreFolder: "restore",
          configPath: "backups/config",
          configOffsite: "",
          containersPath: "backups/containers",
          vmsPath: "backups/vms",
          flashPath: "backups/flash",
          filesPath: "backups/files",
          zfsPath: "backups/zfs",
          encryptionEnabled: true,
        },
        hostMountRoot: "/host/user",
      }),
    detectEncryption: () => Promise.resolve({ ok: false }),
    discover: () => Promise.resolve({ ok: true, discovered: 1 }),
    discoverVMs: () => Promise.resolve({ ok: true, discovered: 0 }),
    discoverFiles: () => Promise.resolve({ ok: true, discovered: 0 }),
    discoverZFS: () => Promise.resolve({ ok: true, discovered: 0 }),
    discoverAll: () =>
      Promise.resolve({ containers: 2, vms: 0, files: 0, zfs: 0, skipped: [], skippedNeedsAction: false }),
    listContainers: () => Promise.resolve({ ok: true, containers: containersOnServer }),
    listVMs: () => Promise.resolve({ ok: true, vms: [] }),
    listFileSets: () => Promise.resolve({ ok: true, fileSets: [] }),
    listZFSDatasets: () => Promise.resolve({ ok: true, datasets: [] }),
    listRuns: () => Promise.resolve({ ok: true, runs: runsOnServer }),
    getVMSSH: () => Promise.resolve({ ok: true, host: "tower" }),
    foreignOpen: () => Promise.resolve({ ok: true, session: "s1", inventory: foreignInventory }),
    foreignClose: () => Promise.resolve({ ok: true }),
    restore: (...a: unknown[]) => restore(...(a as [string, ...unknown[]])),
    listDbDumps: () => Promise.resolve({ ok: true, dumps: dumpsOnServer }),
    importDbDump: (...a: unknown[]) => importDbDump(...(a as [string])),
  };
});

const Recovery = (await import("./Recovery")).default;

function container(over: Partial<Container> = {}): Container {
  return {
    name: "sonarr",
    image: "linuxserver/sonarr",
    state: "not-installed",
    status: "",
    ip: "",
    installed: false,
    includeInSchedule: true,
    lastBackup: 1_700_000_000,
    lastBackupStarted: null,
    preHook: "",
    postHook: "",
    stopContainers: [],
    excludes: [],
    lastUpdateCheck: 0,
    lastUpdateResult: "",
    stack: "",
    dbEngine: "",
    dbSuggestedEngine: "",
    dbTier: "",
    dbDumpOff: false,
    dbDumpEngine: "",
    dbDumpLabelOff: false,
    dbDumpsGlobalOff: false,
    dbDataCoverage: "",
    dbDumpHookOverlap: false,
    dumpOnly: false,
    ...over,
  };
}

async function renderPage() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Recovery />
        </ToastProvider>
      </I18nProvider>
    );
  });
}

beforeEach(() => {
  containersOnServer = [];
  foreignInventory = { containers: [], vms: [], fileSets: [], zfs: [], dbDumps: [] };
  dumpsOnServer = [];
  runsOnServer = [];
  restore.mockClear();
  importDbDump.mockClear();
});

afterEach(() => {
  vi.useRealTimers();
  cleanup();
});

async function discoverAndWait() {
  await renderPage();
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: en["recovery.discover"] }));
  });
}

function retriedWithoutRuntime(): string[] {
  return restore.mock.calls.filter((c) => (c as unknown[])[5] === true).map((c) => (c as unknown[])[0] as string);
}

describe("a container this host has no GPU or runtime for", () => {
  it("is offered again without them after restoring everything", async () => {
    containersOnServer = [container({ name: "plex" }), container({ name: "sonarr" })];
    vi.useFakeTimers();
    await discoverAndWait();
    fireEvent.click(screen.getByRole("button", { name: en["recovery.restoreAll"] }));
    await act(() => vi.advanceTimersByTimeAsync(0));
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["common.confirm"] }));
    });
    await act(() => vi.advanceTimersByTimeAsync(10_000));

    expect(document.body.textContent).toContain(en["restore.noRuntimeList"].replace("{names}", "plex"));
    fireEvent.click(screen.getByRole("button", { name: en["restore.withoutRuntime"] }));
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    expect(retriedWithoutRuntime()).toEqual(["plex"]);
  });

  it("is offered again without them from its dump-only row", async () => {
    containersOnServer = [container({ name: "plex", dbTier: "curated", dbEngine: "postgres", dumpOnly: true })];
    vi.useFakeTimers();
    await discoverAndWait();
    fireEvent.click(screen.getByRole("button", { name: en["recovery.restoreAndImport"] }));
    await act(() => vi.advanceTimersByTimeAsync(10_000));

    expect(document.body.textContent).toContain(en["restore.noRuntime"].replace("{name}", "plex"));
    fireEvent.click(screen.getByRole("button", { name: en["restore.withoutRuntime"] }));
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    expect(retriedWithoutRuntime()).toEqual(["plex"]);
  });
});
