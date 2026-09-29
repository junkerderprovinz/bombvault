// @vitest-environment jsdom
// ContainerRow keeps its backup trigger in remote view, but everything else a
// card can do to itself (export, the schedule and database-dump switches, the
// folders/stop/excludes/hooks editors, and restoring or deleting a snapshot)
// has no route on the allowlist, so those controls must disappear rather than
// merely disable while a peer's instance is open. The snapshot metadata
// (id, date) stays, since decision 1 shows backups as a read.
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { AdvancedProvider } from "../lib/advanced";
import { I18nProvider, countText, en } from "../lib/i18n";
import { InstanceProvider } from "../lib/instanceScope";
import { ToastProvider } from "../lib/toast";
import type { Container, Snapshot } from "../lib/api";
import type { TranslationKey } from "../lib/i18n";

class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

const snap = (id: string): Snapshot => ({ id, time: "2026-09-01T00:00:00Z", paths: [], tags: [], hostname: "tower" });

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listSnapshots: () => Promise.resolve({ ok: true, snapshots: [snap("a1b2c3d4e5f6")] }),
    listRuns: () => Promise.resolve({ ok: true, runs: [] }),
    getSettings: () => Promise.resolve({ ok: true, settings: { restoreFolder: "" }, hostMountRoot: "/host" }),
    listOffsiteTargets: () => Promise.resolve({ ok: true, targets: [] }),
    listDbDumps: () => Promise.resolve({ ok: true, dumps: [] }),
  };
});

const { ContainerRow } = await import("./Containers");
const t = ((key: TranslationKey, n?: number) => countText(en[key], "en", n)) as unknown as Parameters<
  typeof ContainerRow
>[0]["t"];

const radarr: Container = {
  name: "radarr",
  image: "ghcr.io/hotio/radarr:latest",
  state: "running",
  status: "Up 2 hours",
  ip: "",
  installed: true,
  includeInSchedule: true,
  lastBackup: 1_757_000_000,
  lastBackupStarted: null,
  preHook: "",
  postHook: "",
  stopContainers: [],
  excludes: [],
  lastUpdateCheck: 0,
  lastUpdateResult: "",
  stack: "",
};

async function renderRow(initialEntries: string[]) {
  localStorage.setItem("bombvault.advanced", "1");
  await act(async () => {
    render(
      <MemoryRouter initialEntries={initialEntries}>
        <InstanceProvider>
          <I18nProvider>
            <AdvancedProvider>
              <ToastProvider>
                <ContainerRow container={radarr} installedContainers={[radarr]} t={t} onDeleted={() => {}} index={0} />
              </ToastProvider>
            </AdvancedProvider>
          </I18nProvider>
        </InstanceProvider>
      </MemoryRouter>,
    );
  });
}

beforeEach(() => localStorage.clear());
afterEach(cleanup);

it("offers export, the schedule switch, the folders chip and a snapshot's restore/delete locally", async () => {
  await renderRow(["/containers"]);
  expect(screen.getByRole("switch", { name: en["containers.includeInSchedule"] })).toBeTruthy();
  expect(screen.getByRole("button", { name: en["export.button"] })).toBeTruthy();
  expect(screen.getByRole("button", { name: en["folders.title"] })).toBeTruthy();

  fireEvent.click(screen.getByRole("button", { name: en["snapshots.title"] }));
  expect(await screen.findByText("a1b2c3d4")).toBeTruthy();
  expect(screen.getByRole("button", { name: en["restore.open"] })).toBeTruthy();
  expect(screen.getByRole("button", { name: en["snapshots.delete"] })).toBeTruthy();
});

it("hides export, the schedule switch, the folders chip and a snapshot's restore/delete while a peer's instance is open, and keeps the snapshot listed", async () => {
  await renderRow(["/containers?instance=member-1&instanceName=attic"]);
  expect(screen.queryByRole("switch", { name: en["containers.includeInSchedule"] })).toBeNull();
  expect(screen.queryByRole("button", { name: en["export.button"] })).toBeNull();
  expect(screen.queryByRole("button", { name: en["folders.title"] })).toBeNull();

  fireEvent.click(screen.getByRole("button", { name: en["snapshots.title"] }));
  expect(await screen.findByText("a1b2c3d4")).toBeTruthy();
  expect(screen.queryByRole("button", { name: en["restore.open"] })).toBeNull();
  expect(screen.queryByRole("button", { name: en["snapshots.delete"] })).toBeNull();
});
