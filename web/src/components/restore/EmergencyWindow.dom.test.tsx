// @vitest-environment jsdom
/**
 * The emergency window: access to the backups, BombVault's own settings,
 * the choice of what comes back, and the restore.
 *
 * A repository that holds a database only as dumps holds the only copy of
 * it, so those containers are listed apart and never swept into restoring
 * everything, which would bring them back with an empty database.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";

import type { Container, DBDumpView, FileSetView, Run, VM } from "../../lib/api";
import { I18nProvider, countText, en } from "../../lib/i18n";
import type { EmergencyStep } from "../../lib/restorePlan";
import { RESTORE_NO_RUNTIME } from "../../lib/runReason";
import { ToastProvider } from "../../lib/toast";
import { SETTINGS, containerView, fileSetView, vmView } from "../../pages/backupEntry.testsupport";

class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

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

let containersOnServer: Container[] = [];
let vmsOnServer: VM[] = [];
let setsOnServer: FileSetView[] = [];
let dumpsOnServer: DBDumpView[] = [];
let runsOnServer: Run[] = [];
// Containers whose restore Docker refuses until it is asked without a runtime.
let needsRuntime: string[] = [];
let probe = { ok: true, discovered: 1, repo: "/mnt/user/backups/containers" } as Record<string, unknown>;

function finishedRun(kind: string, domain: string, target: string, error = ""): Run {
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
    domain,
  };
}

// Every start runs detached and is watched through the recorded runs, so each
// one hands its finished run to the next poll.
const restore = vi.fn((name: string, _snapshot: string, _confirm: boolean, _source?: string, _stopped?: boolean, bare?: boolean) => {
  const refused = needsRuntime.includes(name) && !bare;
  runsOnServer = [...runsOnServer, finishedRun("restore", "container", name, refused ? RESTORE_NO_RUNTIME : "")];
  return Promise.resolve({ ok: true, started: true });
});
const restoreVM = vi.fn((name: string) => {
  runsOnServer = [...runsOnServer, finishedRun("restore", "vm", name)];
  return Promise.resolve({ ok: true, started: true });
});
const restoreFileSet = vi.fn((id: string) => {
  const set = setsOnServer.find((s) => s.id === id);
  runsOnServer = [...runsOnServer, finishedRun("restore", "files", set?.name ?? id)];
  return Promise.resolve({ ok: true, started: true });
});
const importDbDump = vi.fn((name: string) => {
  runsOnServer = [...runsOnServer, finishedRun("dbimport", "container", name)];
  return Promise.resolve({ ok: true, started: true });
});
const putSettings = vi.fn((settings: unknown) => {
  void settings;
  return Promise.resolve({ ok: true });
});
const restoreConfig = vi.fn((snapshot: string, source?: string) => {
  void snapshot;
  void source;
  return Promise.resolve({ ok: true, staged: true, autoRestart: false });
});
const discoverAll = vi.fn(() =>
  Promise.resolve({
    containers: containersOnServer.length,
    vms: vmsOnServer.length,
    files: setsOnServer.length,
    zfs: 0,
    skipped: [],
    skippedNeedsAction: false,
    paused: [],
    leftOpen: [],
    directRepos: [],
  })
);

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    getSettings: () =>
      Promise.resolve({
        ok: true,
        settings: {
          ...SETTINGS,
          configPath: "backups/config",
          configOffsite: "",
          containersPath: "backups/containers",
          vmsPath: "backups/vms",
          flashPath: "backups/flash",
          filesPath: "backups/files",
          zfsPath: "backups/zfs",
          containersOffsite: "",
          vmsOffsite: "",
          flashOffsite: "",
          filesOffsite: "",
          zfsOffsite: "",
          encryptionEnabled: true,
        },
        hostMountRoot: "/host/user",
      }),
    putSettings: (s: unknown) => putSettings(s),
    detectEncryption: () => Promise.resolve({ ok: true, verdict: "encrypted", encryptionEnabled: true, repos: [] }),
    discover: () => Promise.resolve(probe),
    discoverVMs: () => Promise.resolve({ ok: true, discovered: 0 }),
    discoverFiles: () => Promise.resolve({ ok: true, discovered: 0 }),
    discoverZFS: () => Promise.resolve({ ok: true, discovered: 0 }),
    discoverAll: () => discoverAll(),
    listContainers: () => Promise.resolve({ ok: true, containers: containersOnServer }),
    listVMs: () => Promise.resolve({ ok: true, vms: vmsOnServer }),
    listFileSets: () => Promise.resolve({ ok: true, fileSets: setsOnServer }),
    listZFSDatasets: () => Promise.resolve({ ok: true, datasets: [] }),
    listRuns: () => Promise.resolve({ ok: true, runs: runsOnServer }),
    getVMSSH: () => Promise.resolve({ ok: true, host: "tower" }),
    getRclone: () => Promise.resolve({ ok: false }),
    listOffsiteTargets: () => Promise.resolve({ ok: true, targets: [] }),
    restore: (...a: unknown[]) => restore(...(a as [string, string, boolean])),
    restoreVM: (...a: unknown[]) => restoreVM(...(a as [string])),
    restoreFileSet: (...a: unknown[]) => restoreFileSet(...(a as [string])),
    restoreConfig: (snapshot: string, source?: string) => restoreConfig(snapshot, source),
    checkRestore: () => Promise.resolve({ ok: true, ready: true, checks: [], plan: null }),
    fileSetSnapshots: () =>
      Promise.resolve({ ok: true, snapshots: [{ id: "f-old", time: "2026-09-01T03:00:00Z" }, { id: "f-new", time: "2026-09-20T03:00:00Z" }] }),
    listDbDumps: () => Promise.resolve({ ok: true, dumps: dumpsOnServer }),
    importDbDump: (...a: unknown[]) => importDbDump(...(a as [string])),
    downloadRecoveryKit: () => Promise.resolve(null),
  };
});

const { EmergencyWindow } = await import("./EmergencyWindow");

function gone(over: Partial<Container> = {}): Container {
  return containerView({ name: "sonarr", state: "not-installed", installed: false, lastBackup: 1_700_000_000, ...over });
}

const onClose = vi.fn();

async function open(start: EmergencyStep = "access") {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <EmergencyWindow start={start} onClose={onClose} />
        </ToastProvider>
      </I18nProvider>
    );
  });
}

async function press(el: HTMLElement) {
  await act(async () => {
    fireEvent.click(el);
  });
}

const button = (name: string) => screen.getByRole("button", { name }) as HTMLButtonElement;
const picks = () => screen.queryAllByTestId("emergency-pick");
const pick = (name: string) => screen.getByRole("switch", { name }) as HTMLElement;
const isOn = (name: string) => pick(name).getAttribute("aria-checked") === "true";

beforeEach(() => {
  containersOnServer = [];
  vmsOnServer = [];
  setsOnServer = [];
  dumpsOnServer = [];
  runsOnServer = [];
  needsRuntime = [];
  probe = { ok: true, discovered: 1, repo: "/mnt/user/backups/containers" };
});

afterEach(() => {
  vi.useRealTimers();
  cleanup();
  vi.clearAllMocks();
});

describe("access", () => {
  it("says that the backups are readable and where it looked", async () => {
    await open();
    await press(button(en["recovery.recheck"]));
    expect(screen.getByText(en["recovery.readable"])).toBeTruthy();
    expect(screen.getByText("/mnt/user/backups/containers")).toBeTruthy();
  });

  it("names the remedy when the key does not fit", async () => {
    probe = { ok: false, error: "wrong password: check APP_KEY" };
    await open();
    await press(button(en["recovery.recheck"]));
    expect(screen.getByText(en["recovery.appKeyRemedy"])).toBeTruthy();
    expect(screen.queryByText(en["recovery.readable"])).toBeNull();
  });

  it("saves the attached folders onto the stored settings and checks again", async () => {
    await open();
    expect(await screen.findByText(en["recovery.encEncrypted"])).toBeTruthy();
    await press(button(en["emergency.connect"]));
    await waitFor(() => expect(putSettings).toHaveBeenCalledTimes(1));
    expect(putSettings.mock.calls[0][0]).toMatchObject({ containersPath: "backups/containers", zfsOffsite: "", restoreFolder: SETTINGS.restoreFolder });
    // The note in the window and the toast both say it.
    expect((await screen.findAllByText(en["recovery.readable"])).length).toBeGreaterThan(0);
  });

  it("offers neither the kit upload nor the way through the group", async () => {
    await open();
    expect(screen.queryByText(/upload/i)).toBeNull();
    expect(screen.queryByText(/twelve words/i)).toBeNull();
  });
});

describe("the settings", () => {
  it("are brought back from the place that was named, and say when the restart is left to the user", async () => {
    await open("settings");
    await press(button(en["emergency.settingsGo"]));
    await waitFor(() => expect(restoreConfig).toHaveBeenCalledWith("latest", undefined));
    expect(putSettings.mock.calls[0][0]).toMatchObject({ configPath: "backups/config" });
    expect(await screen.findByText(en["recovery.configManualRestart"])).toBeTruthy();
  });

  it("can be skipped", async () => {
    await open("settings");
    await press(screen.getByRole("radio", { name: new RegExp(`^${en["recovery.configSkip"]}`) }));
    await press(button(en["zfs.replica.add.next"]));
    expect(restoreConfig).not.toHaveBeenCalled();
    await waitFor(() => expect(discoverAll).toHaveBeenCalled());
  });
});

describe("the selection", () => {
  it("looks through the backups when the step opens", async () => {
    containersOnServer = [gone(), gone({ name: "radarr" })];
    await open("selection");
    expect(discoverAll).toHaveBeenCalledTimes(1);
    expect(screen.getByText(en["recovery.foundCounts"].replace("{c}", "2").replace("{v}", "0"))).toBeTruthy();
    expect(picks()).toHaveLength(2);
  });

  it("looks again after other folders were attached", async () => {
    containersOnServer = [gone()];
    await open("selection");
    expect(discoverAll).toHaveBeenCalledTimes(1);
    await press(screen.getByRole("radio", { name: en["emergency.access"] }));
    await press(button(en["emergency.connect"]));
    await waitFor(() => expect(putSettings).toHaveBeenCalled());
    await screen.findAllByText(en["recovery.readable"]);
    await press(screen.getByRole("radio", { name: en["emergency.selection"] }));
    await waitFor(() => expect(discoverAll).toHaveBeenCalledTimes(2));
  });

  it("starts with what this server lacks switched on and what it has switched off", async () => {
    containersOnServer = [gone(), containerView({ name: "plex" })];
    vmsOnServer = [vmView({ state: "not-installed" })];
    await open("selection");
    expect(isOn("sonarr")).toBe(true);
    expect(isOn("plex")).toBe(false);
    expect(isOn("Windows 11")).toBe(true);
    expect(screen.getByText(new RegExp(countText(en["emergency.count.containers"], "en", 1)))).toBeTruthy();
  });

  it("says so when nothing was found", async () => {
    await open("selection");
    expect(screen.getByText(en["recovery.foundNone"])).toBeTruthy();
    expect(button(en["snapshots.restore"]).disabled).toBe(true);
  });

  it("lists the containers that exist as dumps alone apart from the others", async () => {
    containersOnServer = [gone(), gone({ name: "immich_postgres", dbTier: "curated", dbEngine: "postgres", dumpOnly: true })];
    await open("selection");
    const apart = screen.getByText(en["recovery.dumpOnlyTitle"]).closest("section") as HTMLElement;
    expect(apart.textContent).toContain("immich_postgres");
    expect(apart.textContent).not.toContain("sonarr");
    expect(picks()).toHaveLength(1);
  });

  it("brings a dump-only container back with its data from its own row", async () => {
    containersOnServer = [gone({ name: "immich_postgres", dbTier: "curated", dbEngine: "postgres", dumpOnly: true })];
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
    vi.useFakeTimers();
    await open("selection");
    fireEvent.click(button(en["recovery.restoreAndImport"]));
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    expect(restore.mock.calls[0]).toEqual(["immich_postgres", "latest", true, undefined, undefined, false]);
    expect(importDbDump.mock.calls[0]).toEqual(["immich_postgres", "d1"]);
    expect(screen.getByText(en["dbdump.importDone"])).toBeTruthy();
  });

  it("offers a dump-only container again without GPU and runtime", async () => {
    containersOnServer = [gone({ name: "plex", dbTier: "curated", dbEngine: "postgres", dumpOnly: true })];
    needsRuntime = ["plex"];
    vi.useFakeTimers();
    await open("selection");
    fireEvent.click(button(en["recovery.restoreAndImport"]));
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    expect(document.body.textContent).toContain(en["restore.noRuntime"].replace("{name}", "plex"));
    fireEvent.click(button(en["restore.withoutRuntime"]));
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    expect(restore.mock.calls.at(-1)?.[5]).toBe(true);
  });
});

describe("the restore", () => {
  it("brings back what is switched on, one after the other and left stopped", async () => {
    containersOnServer = [gone(), gone({ name: "immich_postgres", dumpOnly: true })];
    vmsOnServer = [vmView({ state: "not-installed" })];
    setsOnServer = [fileSetView({ path: "" })];
    vi.useFakeTimers();
    await open("selection");
    fireEvent.click(button(en["snapshots.restore"]));
    await act(() => vi.advanceTimersByTimeAsync(30_000));

    expect(restore.mock.calls.map((c) => c[0])).toEqual(["sonarr"]);
    expect(restore.mock.calls[0]).toEqual(["sonarr", "latest", true, undefined, true]);
    expect(restoreVM.mock.calls[0]).toEqual(["win11", "latest", true, undefined, true]);
    expect(restoreFileSet.mock.calls[0]).toEqual([setsOnServer[0].id, "f-new", true, "user/bombvault/restore/Photos"]);
    expect(screen.getAllByTestId("emergency-run")).toHaveLength(3);
    expect(screen.getByText(en["emergency.result"].replace("{ok}", "3").replace("{fail}", "0"))).toBeTruthy();
    expect(button(en["recovery.kitDownload"])).toBeTruthy();
  });

  it("asks first when it would replace something this server still has", async () => {
    containersOnServer = [containerView({ name: "plex" })];
    await open("selection");
    await press(pick("plex"));
    await press(button(en["snapshots.restore"]));
    expect(restore).not.toHaveBeenCalled();
    expect(screen.getByText(new RegExp(en["backups.restore.alsoVMs"]))).toBeTruthy();
    await press(within(screen.getAllByRole("dialog").at(-1)!).getByRole("button", { name: en["common.cancel"] }));
    expect(restore).not.toHaveBeenCalled();
  });

  it("starts what came back when it is not left stopped", async () => {
    containersOnServer = [gone()];
    vi.useFakeTimers();
    await open("selection");
    fireEvent.click(screen.getByRole("switch", { name: en["restoreWindow.leaveStopped"] }));
    fireEvent.click(button(en["snapshots.restore"]));
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    expect(restore.mock.calls[0]).toEqual(["sonarr", "latest", true, undefined, false]);
  });

  it("offers the containers Docker refused again without GPU and runtime", async () => {
    containersOnServer = [gone({ name: "plex" }), gone()];
    needsRuntime = ["plex"];
    vi.useFakeTimers();
    await open("selection");
    fireEvent.click(button(en["snapshots.restore"]));
    await act(() => vi.advanceTimersByTimeAsync(20_000));

    expect(screen.getByText(en["emergency.result"].replace("{ok}", "1").replace("{fail}", "1"))).toBeTruthy();
    expect(document.body.textContent).toContain(en["restore.noRuntimeList"].replace("{names}", "plex"));
    fireEvent.click(button(en["restore.withoutRuntime"]));
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    expect(restore.mock.calls.filter((c) => c[5] === true).map((c) => c[0])).toEqual(["plex"]);
  });
});
