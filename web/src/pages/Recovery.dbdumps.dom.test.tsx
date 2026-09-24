// @vitest-environment jsdom
// A repository that holds nothing but database dumps still holds the only copy
// of those databases. The wizard must not call it empty, and it must not sweep
// the containers it rebuilt from dumps alone into "restore everything", which
// would bring them back with an empty database and say nothing about it.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { countText, I18nProvider, en } from "../lib/i18n";
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

function finishedRun(kind: string, target: string): Run {
  return {
    id: `${kind}-${runsOnServer.length + 1}`,
    targetId: "t1",
    kind,
    status: "success",
    startedAt: 1_700_000_000,
    finishedAt: 1_700_000_010,
    snapshotId: "s1",
    bytes: 0,
    error: "",
    acknowledged: false,
    target,
    domain: "container",
  };
}

// Both actions of the dump-only row run detached and are watched through the
// recorded runs, so each start hands its finished run to the next poll.
const restore = vi.fn((name: string) => {
  runsOnServer = [...runsOnServer, finishedRun("restore", name)];
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
    restore: (...a: unknown[]) => restore(...(a as [string])),
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

/** The connect form's inputs carry no htmlFor, so each is found under its own
 *  label. */
function inputUnder(labelText: string): HTMLInputElement {
  const label = screen.getByText(labelText);
  const field = label.closest("div") as HTMLElement;
  return field.querySelector("input") as HTMLInputElement;
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

describe("a foreign repository holding only dumps", () => {
  it("is not reported as empty", async () => {
    foreignInventory = {
      containers: [],
      vms: [],
      fileSets: [],
      zfs: [],
      dbDumps: [{ name: "immich_postgres", snapshots: [] }],
    };
    await renderPage();

    fireEvent.change(inputUnder(en["recovery.foreignKey"]), { target: { value: "a".repeat(64) } });
    fireEvent.change(inputUnder(en["recovery.foreignLocation"]), { target: { value: "backups/other" } });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["recovery.foreignConnect"] }));
    });

    expect(screen.queryByText(en["recovery.foreignEmpty"])).toBeNull();
    expect(screen.getByText(countText(en["recovery.foreignDbDumps"], "en", 1))).toBeTruthy();
  });
});

describe("containers that exist as dumps alone", () => {
  beforeEach(() => {
    containersOnServer = [
      container({ name: "sonarr" }),
      container({ name: "immich_postgres", dbTier: "curated", dbEngine: "postgres", dumpOnly: true }),
    ];
  });

  it("are listed apart from the containers with a files backup", async () => {
    await renderPage();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["recovery.discover"] }));
    });

    const apart = await screen.findByText(en["recovery.dumpOnlyTitle"]);
    const list = apart.parentElement as HTMLElement;
    expect(list.textContent).toContain("immich_postgres");
    expect(list.textContent).not.toContain("sonarr");
    expect(screen.getByRole("button", { name: en["recovery.restoreAndImport"] })).toBeTruthy();
  });

  it("come back with their data when the row's one action is used", async () => {
    dumpsOnServer = [
      {
        id: "d1",
        time: "2026-09-23T19:10:19Z",
        engine: "postgres",
        image: "postgres:16",
        version: "16.4",
        databases: ["immich"],
        bytes: 5_000,
        damaged: false,
      },
    ];
    // Both runs are watched by polling the run list, so the clock has to move.
    vi.useFakeTimers();
    await renderPage();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["recovery.discover"] }));
    });

    fireEvent.click(screen.getByRole("button", { name: en["recovery.restoreAndImport"] }));
    await act(() => vi.advanceTimersByTimeAsync(10_000));

    expect(restore.mock.calls[0]).toEqual(["immich_postgres", "latest", true]);
    expect(importDbDump.mock.calls[0]).toEqual(["immich_postgres", "d1"]);
    expect(screen.getByText(en["dbdump.importDone"])).toBeTruthy();
  });

  it("are left out of restoring everything, and said so", async () => {
    await renderPage();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["recovery.discover"] }));
    });

    fireEvent.click(screen.getByRole("button", { name: en["recovery.restoreAll"] }));
    const question = await screen.findByText(new RegExp(countText(en["recovery.dumpOnlySkipped"], "en", 1)));
    expect(question).toBeTruthy();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["common.confirm"] }));
    });
    await waitFor(() => expect(restore).toHaveBeenCalled());
    expect(restore.mock.calls.map((c) => (c as unknown[])[0])).toEqual(["sonarr"]);
  });
});
