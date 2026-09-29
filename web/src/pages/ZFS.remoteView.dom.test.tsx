// @vitest-environment jsdom
// ZFS keeps its per-dataset backup trigger in remote view (POST
// /api/zfs/datasets/{id}/backup is on the allowlist), but every settings edit
// (edit, delete, the enabled switch, removing a missing dataset, sweeping
// leftovers), the safety-snapshot section and the restore panel have no route
// on it, so they must disappear rather than merely disable while a peer's
// instance is open.
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { AdvancedProvider } from "../lib/advanced";
import { I18nProvider, en } from "../lib/i18n";
import { InstanceProvider } from "../lib/instanceScope";
import { ToastProvider } from "../lib/toast";
import type { ZFSDatasetView } from "../lib/api";

class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

function item(overrides: Partial<ZFSDatasetView> = {}): ZFSDatasetView {
  return {
    id: "z1",
    dataset: "cache/appdata",
    enabled: true,
    excludes: [],
    scheduleCadence: "",
    repo: "",
    repoEffective: "ZFS datasets",
    stopContainers: [],
    restartPending: [],
    excludedChildren: [],
    hookContainer: "",
    preSnapshot: "",
    postSnapshot: "",
    hostMountpoint: "/mnt/cache/appdata",
    lastBackup: 1_700_000_000,
    lastRunStatus: "success",
    lastCheckCode: "ok",
    lastCheckDetail: "",
    lastCheckAt: 1_700_000_000,
    leftoverCount: 2,
    safetyCount: 1,
    safetyOldestAt: 0,
    members: [
      {
        dataset: "cache/appdata",
        relPath: "",
        hostMountpoint: "/mnt/cache/appdata",
        outcome: "backed-up",
        isNew: false,
        usedByDataset: 4096,
        lastBackupAt: 1_700_000_000,
      },
    ],
    effectiveSchedule: { kind: "domain", spec: "0 3 * * *", alsoSpec: "" },
    ...overrides,
  };
}

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listZFSDatasets: () => Promise.resolve({ ok: true, datasets: [item()] }),
    zfsHostDatasets: () => Promise.resolve({ ok: true, available: true, notInItem: 0, unusedZvols: 0, datasets: [] }),
    getSettings: () => Promise.resolve({ ok: true, hostMountRoot: "/host", settings: { perItemSchedules: false } }),
    listRuns: () => Promise.resolve({ ok: true, runs: [] }),
    listRepos: () => Promise.resolve({ ok: true, repos: [] }),
    listContainers: () => Promise.resolve({ ok: true, containers: [] }),
    zfsConnection: () =>
      Promise.resolve({ ok: true, code: "ok", target: "root@tower", uriTarget: "", version: "", detail: "" }),
  };
});

const { ZFS } = await import("./ZFS");

afterEach(cleanup);

async function renderZFS(initialEntries: string[]) {
  await act(async () => {
    render(
      <MemoryRouter initialEntries={initialEntries}>
        <InstanceProvider>
          <I18nProvider>
            <AdvancedProvider>
              <ToastProvider>
                <ZFS />
              </ToastProvider>
            </AdvancedProvider>
          </I18nProvider>
        </InstanceProvider>
      </MemoryRouter>,
    );
  });
}

it("offers the dataset's settings and restore controls locally", async () => {
  await renderZFS(["/zfs"]);
  expect(await screen.findByText("cache/appdata")).toBeTruthy();
  expect(screen.getByRole("button", { name: en["containers.backupNow"] })).toBeTruthy();
  expect(screen.getByRole("button", { name: en["common.edit"] })).toBeTruthy();
  expect(screen.getByRole("button", { name: en["common.delete"] })).toBeTruthy();
  expect(screen.getByRole("switch", { name: en["zfs.enabled"] })).toBeTruthy();
  expect(screen.getByRole("button", { name: en["zfs.removeLeftovers"] })).toBeTruthy();
  expect(screen.getByRole("button", { name: en["snapshots.title"] })).toBeTruthy();
});

it("hides the dataset's settings and restore controls while a peer's instance is open, and keeps the backup trigger", async () => {
  await renderZFS(["/zfs?instance=member-1&instanceName=attic"]);
  expect(await screen.findByText("cache/appdata")).toBeTruthy();
  expect(screen.getByRole("button", { name: en["containers.backupNow"] })).toBeTruthy();
  expect(screen.queryByRole("button", { name: en["common.edit"] })).toBeNull();
  expect(screen.queryByRole("button", { name: en["common.delete"] })).toBeNull();
  expect(screen.queryByRole("switch", { name: en["zfs.enabled"] })).toBeNull();
  expect(screen.queryByRole("button", { name: en["zfs.removeLeftovers"] })).toBeNull();
  expect(screen.queryByRole("button", { name: en["snapshots.title"] })).toBeNull();
});

it("hides the SSH connection card and the add-dataset controls remotely", async () => {
  await renderZFS(["/zfs?instance=member-1&instanceName=attic"]);
  await screen.findByText("cache/appdata");
  expect(screen.queryByText(en["zfs.connection.title"])).toBeNull();
  expect(screen.queryByRole("button", { name: en["zfs.addDatasets"] })).toBeNull();
  expect(screen.queryByRole("button", { name: en["containers.discover"] })).toBeNull();
});
