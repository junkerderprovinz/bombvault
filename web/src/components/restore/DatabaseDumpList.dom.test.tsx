// @vitest-environment jsdom
// The dump list is where a database is taken back by hand, so every row has to
// say which server and which version it came from, and no sentence may carry an
// unfilled placeholder when the snapshot does not record one.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { AdvancedProvider } from "../../lib/advanced";
import { I18nProvider, en } from "../../lib/i18n";
import type { DBDumpView } from "../../lib/api";

const listDbDumps = vi.fn();
const checkDbDumpDownload = vi.fn(() => Promise.resolve({ ok: true }));
const importDbDump = vi.fn(() => Promise.resolve({ ok: true, started: true }));
const saveDbDumpTo = vi.fn(() => Promise.resolve({ ok: true, started: true, target: "/host/user/dumps/pg.sql" }));
const deleteSnapshot = vi.fn(() => Promise.resolve({ ok: true }));
const listRuns = vi.fn(() => Promise.resolve({ ok: true, runs: [] as unknown[] }));

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    listDbDumps: (...a: unknown[]) => listDbDumps(...(a as [])),
    checkDbDumpDownload: (...a: unknown[]) => checkDbDumpDownload(...(a as [])),
    importDbDump: (...a: unknown[]) => importDbDump(...(a as [])),
    saveDbDumpTo: (...a: unknown[]) => saveDbDumpTo(...(a as [])),
    deleteSnapshot: (...a: unknown[]) => deleteSnapshot(...(a as [])),
    listRuns: (...a: unknown[]) => listRuns(...(a as [])),
  };
});

const push = vi.fn();
vi.mock("../../lib/toast", () => ({ useToast: () => ({ push }) }));

const watched: { kind?: string; progressKey?: string }[] = [];
vi.mock("../../lib/backupWatch", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/backupWatch")>();
  return {
    ...actual,
    useBackupWatch: (args: { kind?: string; progressKey?: string }) => {
      watched.push({ kind: args.kind, progressKey: args.progressKey });
      return actual.useBackupWatch(args as Parameters<typeof actual.useBackupWatch>[0]);
    },
  };
});

class StubEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as { EventSource?: unknown }).EventSource = StubEventSource;

const { DatabaseDumpList } = await import("./DatabaseDumpList");

const t = ((k: string) => en[k as keyof typeof en] ?? k) as never;

function makeDump(over: Partial<DBDumpView> = {}): DBDumpView {
  return {
    id: "aaaa1111bbbb2222",
    time: "2026-09-19T10:00:00Z",
    engine: "postgres",
    image: "postgres:16",
    version: "16.4",
    databases: ["immich"],
    bytes: 2048,
    damaged: false,
    pairedSnapshotId: "cccc3333",
    ...over,
  };
}

function renderList(importStops: string[] = []) {
  return render(
    <I18nProvider>
      <AdvancedProvider>
        <DatabaseDumpList
          containerName="immich_postgres"
          source="local"
          recognised
          canImport
          importStops={importStops}
          hostMountRoot="/host/user"
          defaultFolder="user/bombvault/restore"
          reloadTick={0}
          t={t}
        />
      </AdvancedProvider>
    </I18nProvider>
  );
}

/** The import run the backend records once the import has finished. */
function importRun(status: string, error: string) {
  return {
    id: "run-import",
    kind: "dbimport",
    domain: "container",
    target: "immich_postgres",
    status,
    error,
    snapshotId: "",
  };
}

/** Answers the pre-fire lookup with no runs and every later poll with run. */
function recordAfterStart(run: unknown) {
  listRuns.mockReset();
  listRuns.mockImplementationOnce(() => Promise.resolve({ ok: true, runs: [] }));
  listRuns.mockImplementation(() => Promise.resolve({ ok: true, runs: [run] }));
}

async function confirmImport() {
  fireEvent.click(await screen.findByRole("button", { name: en["dbdump.import"] }));
  await waitFor(() => expect(screen.getByRole("dialog")).toBeTruthy());
  fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: en["dbdump.import"] }));
}

function filled(key: keyof typeof en, params: Record<string, string>): string {
  let text: string = en[key];
  for (const [token, value] of Object.entries(params)) text = text.replace(`{${token}}`, value);
  return text;
}

/** Advanced mode holds the format picker, the save action and the tag chips. */
function turnAdvancedOn() {
  localStorage.setItem("bombvault.advanced", "1");
}

beforeEach(() => {
  listDbDumps.mockResolvedValue({ ok: true, dumps: [makeDump()] });
});

afterEach(() => {
  cleanup();
  localStorage.clear();
  watched.length = 0;
  listDbDumps.mockReset();
  checkDbDumpDownload.mockClear();
  importDbDump.mockClear();
  saveDbDumpTo.mockClear();
  deleteSnapshot.mockClear();
  push.mockClear();
  listRuns.mockReset();
  listRuns.mockImplementation(() => Promise.resolve({ ok: true, runs: [] }));
  document.querySelectorAll("a[download]").forEach((a) => a.remove());
});

describe("the database dump list", () => {
  it("names the engine, the version, the databases and the size", async () => {
    renderList();

    await screen.findByText(en["dbdump.listTitle"]);
    expect(screen.getByText("PostgreSQL")).toBeTruthy();
    expect(screen.getByText(filled("dbdump.versionLabel", { engine: "PostgreSQL", version: "16.4" }))).toBeTruthy();
    expect(document.body.textContent).toContain(filled("dbdump.databasesLine", { names: "immich" }));
    expect(screen.getByText("2.0 KB")).toBeTruthy();
  });

  it("offers nothing but delete on a damaged dump", async () => {
    listDbDumps.mockResolvedValue({ ok: true, dumps: [makeDump({ damaged: true })] });
    turnAdvancedOn();
    renderList();

    await screen.findByText(en["dbdump.damagedBadge"]);
    expect(screen.getByRole("button", { name: en["snapshots.delete"] })).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["dbdump.download"] })).toBeNull();
    expect(screen.queryByRole("button", { name: en["dbdump.import"] })).toBeNull();
    expect(screen.queryByRole("button", { name: en["dbdump.saveToFolder"] })).toBeNull();
  });

  it("saves nothing when the download check refuses", async () => {
    checkDbDumpDownload.mockResolvedValueOnce({ ok: false, error: "the repository is locked" });
    renderList();

    fireEvent.click(await screen.findByRole("button", { name: en["dbdump.download"] }));
    await waitFor(() => expect(push).toHaveBeenCalled());
    expect(push.mock.calls[0][0]).toContain(en["dbdump.downloadRefused"]);
    expect(document.querySelector("a[download]")).toBeNull();
  });

  it("downloads through an anchor that carries the chosen format", async () => {
    turnAdvancedOn();
    const clicks: string[] = [];
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(function (this: HTMLAnchorElement) {
        clicks.push(this.getAttribute("href") ?? "");
      });
    renderList();

    fireEvent.click(await screen.findByRole("combobox", { name: en["dbdump.format"] }));
    fireEvent.click(screen.getByRole("option", { name: en["dbdump.formatGz"] }));
    fireEvent.click(screen.getByRole("button", { name: en["dbdump.download"] }));

    await waitFor(() => expect(clicks.length).toBe(1));
    expect(clicks[0]).toContain("/dbdumps/aaaa1111bbbb2222/download");
    expect(clicks[0]).toContain("gz=1");
    click.mockRestore();
  });

  it("watches the save as its own kind of run", async () => {
    turnAdvancedOn();
    renderList();

    fireEvent.click(await screen.findByRole("button", { name: en["dbdump.saveToFolder"] }));
    expect(screen.getByText(en["dbdump.saveHint"])).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: en["common.confirm"] }));

    await waitFor(() => expect(saveDbDumpTo).toHaveBeenCalled());
    expect(watched.some((w) => w.kind === "dbdumpsave")).toBe(true);
    expect(await screen.findByRole("button", { name: en["restore.cancel"] })).toBeTruthy();
    expect(screen.queryByText(en["restore.started"])).toBeNull();
  });

  it("asks with the engine, the version and the container before importing", async () => {
    renderList();

    fireEvent.click(await screen.findByRole("button", { name: en["dbdump.import"] }));
    const question = filled("dbdump.importConfirm", {
      engine: "PostgreSQL",
      version: "16.4",
      container: "immich_postgres",
    });
    await waitFor(() => expect(screen.getByText(question)).toBeTruthy());
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: en["dbdump.import"] }));
    await waitFor(() => expect(importDbDump).toHaveBeenCalledWith("immich_postgres", "aaaa1111bbbb2222", "local"));
  });

  it("names the one app the import stops while it runs", async () => {
    renderList(["immich_server"]);

    fireEvent.click(await screen.findByRole("button", { name: en["dbdump.import"] }));
    await waitFor(() => expect(screen.getByRole("dialog")).toBeTruthy());
    expect(screen.getByRole("dialog").textContent).toContain(filled("dbdump.importStopsOne", { app: "immich_server" }));
  });

  it("names every app the import stops while it runs", async () => {
    renderList(["immich_server", "immich_machine_learning"]);

    fireEvent.click(await screen.findByRole("button", { name: en["dbdump.import"] }));
    await waitFor(() => expect(screen.getByRole("dialog")).toBeTruthy());
    expect(screen.getByRole("dialog").textContent).toContain(
      filled("dbdump.importStopsMany", { apps: "immich_server, immich_machine_learning" })
    );
  });

  it("turns a refused import into its own sentence, said once", async () => {
    importDbDump.mockResolvedValueOnce({ ok: false, error: "not running", code: "notRunning" });
    renderList();
    await confirmImport();

    expect(await screen.findByText(en["dbdump.importRefused.notRunning"])).toBeTruthy();
    expect(document.body.textContent).not.toContain("not running");
    expect(push).not.toHaveBeenCalled();
  });

  it("names the two versions a version mismatch refuses on", async () => {
    importDbDump.mockResolvedValueOnce({ ok: false, error: "too old", code: "version", server: "14", dump: "16" });
    renderList();
    await confirmImport();

    expect(await screen.findByText(filled("dbdump.importRefused.version", { server: "14", dump: "16" }))).toBeTruthy();
  });

  it("asks a MariaDB dump for the major version it came from", async () => {
    listDbDumps.mockResolvedValue({ ok: true, dumps: [makeDump({ engine: "mariadb", version: "10.11.9" })] });
    importDbDump.mockResolvedValueOnce({ ok: false, error: "too new", code: "version", server: "11", dump: "10" });
    renderList();
    await confirmImport();

    expect(
      await screen.findByText(filled("dbdump.importRefused.versionSameMajor", { server: "11", dump: "10" }))
    ).toBeTruthy();
  });

  it("offers no cancel for an import, which cannot be stopped once it runs", async () => {
    renderList();
    await confirmImport();

    await waitFor(() => expect(importDbDump).toHaveBeenCalled());
    expect(screen.queryByRole("button", { name: en["restore.cancel"] })).toBeNull();
    expect(screen.queryByText(en["restore.started"])).toBeNull();
  });

  it("says where the import kept the previous data folder", async () => {
    const kept = "/mnt/user/appdata/immich/pg.bombvault-before-import-20260921-031500";
    recordAfterStart(importRun("success", `database imported; the previous data folder was kept: ${kept}`));
    renderList();
    await confirmImport();

    const line = await screen.findByText(en["runReason.dbimportKeptOld"], { exact: false }, { timeout: 4000 });
    expect(line.className).toContain("statusOk");
    expect(line.querySelector('bdi[dir="ltr"]')?.textContent).toBe(kept);
  });

  it("reports an import that finished with errors as a warning", async () => {
    recordAfterStart(
      importRun("success", "database imported with errors: 3 errors, the previous data folder is kept at /mnt/x")
    );
    renderList();
    await confirmImport();

    const line = await screen.findByText(en["runReason.dbimportErrors"], { exact: false }, { timeout: 4000 });
    expect(line.className).toContain("statusWarn");
    expect(document.body.textContent).not.toContain(en["dbdump.importDone"]);
  });

  it("reports a failed import in the reader's language", async () => {
    recordAfterStart(importRun("failed", "database import failed: the import tool reported an error: ERROR: boom"));
    renderList();
    await confirmImport();

    const line = await screen.findByText(en["runReason.dbimportFailed"], { exact: false }, { timeout: 4000 });
    expect(line.className).toContain("statusFail");
    expect(line.querySelector("bdi")?.textContent).toBe("ERROR: boom");
  });

  it("leaves out the version text and the import when the snapshot names no engine", async () => {
    listDbDumps.mockResolvedValue({ ok: true, dumps: [makeDump({ engine: "", version: "16.4" })] });
    renderList();

    await screen.findByText(en["dbdump.engineUnknown"]);
    expect(screen.queryByRole("button", { name: en["dbdump.import"] })).toBeNull();
    expect(screen.queryByText(/16\.4/)).toBeNull();
    expect(screen.getByRole("button", { name: en["dbdump.download"] })).toBeTruthy();
  });

  it("asks without a version when the dump records none", async () => {
    listDbDumps.mockResolvedValue({ ok: true, dumps: [makeDump({ version: "" })] });
    renderList();

    fireEvent.click(await screen.findByRole("button", { name: en["dbdump.import"] }));
    const question = filled("dbdump.importConfirmNoVersion", {
      engine: "PostgreSQL",
      container: "immich_postgres",
    });
    await waitFor(() => expect(screen.getByRole("dialog")).toBeTruthy());
    const asked = screen.getByRole("dialog").textContent ?? "";
    expect(asked).toContain(question);
    expect(asked).toContain(en["dbdump.importNoVersion"]);
    expect(document.body.textContent).not.toContain("{version}");
  });

  it("deletes the snapshot the row stands for", async () => {
    renderList();

    fireEvent.click(await screen.findByRole("button", { name: en["snapshots.delete"] }));
    await waitFor(() => expect(screen.getByText(en["dbdump.deleteConfirm"])).toBeTruthy());
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: en["snapshots.delete"] }));

    await waitFor(() => expect(deleteSnapshot).toHaveBeenCalledWith("containers", "aaaa1111bbbb2222", "local"));
  });

  it("says so when a recognised database has no dump, and stays away otherwise", async () => {
    listDbDumps.mockResolvedValue({ ok: true, dumps: [] });
    const { container, rerender } = renderList();
    await screen.findByText(en["dbdump.none"]);

    rerender(
      <I18nProvider>
        <AdvancedProvider>
          <DatabaseDumpList
            containerName="plex"
            source="local"
            recognised={false}
            canImport={false}
            hostMountRoot="/host/user"
            defaultFolder="user/bombvault/restore"
            reloadTick={0}
            t={t}
          />
        </AdvancedProvider>
      </I18nProvider>
    );
    await waitFor(() => expect(container.textContent).toBe(""));
  });

  it("reports a failed load inline", async () => {
    listDbDumps.mockResolvedValue({ ok: false, error: "repository unreachable" });
    renderList();

    expect(await screen.findByText(en["dbdump.loadFailed"])).toBeTruthy();
    expect(push).not.toHaveBeenCalled();
  });
});
