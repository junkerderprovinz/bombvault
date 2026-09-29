// @vitest-environment jsdom
// VMRow keeps its backup trigger in remote view, but export, the schedule
// switch, and restoring or deleting a snapshot have no route on the
// allowlist, so those controls must disappear rather than merely disable
// while a peer's instance is open. The snapshot metadata (id, date) stays,
// since decision 1 shows backups as a read.
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { AdvancedProvider } from "../lib/advanced";
import { I18nProvider, countText, en } from "../lib/i18n";
import { InstanceProvider } from "../lib/instanceScope";
import { ToastProvider } from "../lib/toast";
import type { Snapshot, VM } from "../lib/api";
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
    listVMSnapshots: () => Promise.resolve({ ok: true, snapshots: [snap("a1b2c3d4e5f6")] }),
    listRuns: () => Promise.resolve({ ok: true, runs: [] }),
    getSettings: () => Promise.resolve({ ok: true, settings: { restoreFolder: "" }, hostMountRoot: "/host" }),
  };
});

const { VMRow } = await import("./VMs");
const t = ((key: TranslationKey, n?: number) => countText(en[key], "en", n)) as unknown as Parameters<
  typeof VMRow
>[0]["t"];

const win11: VM = {
  name: "win11",
  libvirtName: "1_win11",
  state: "running",
  method: "graceful",
  includeInSchedule: true,
  lastBackup: 1_757_000_000,
  lastBackupStarted: null,
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
                <VMRow vm={win11} t={t} onRefresh={() => {}} index={0} />
              </ToastProvider>
            </AdvancedProvider>
          </I18nProvider>
        </InstanceProvider>
      </MemoryRouter>,
    );
  });
}

afterEach(() => {
  cleanup();
  localStorage.clear();
});

it("offers export, the schedule switch and a snapshot's restore/delete locally", async () => {
  await renderRow(["/vms"]);
  expect(screen.getByRole("switch", { name: en["containers.includeInSchedule"] })).toBeTruthy();
  expect(screen.getByRole("button", { name: en["export.button"] })).toBeTruthy();

  fireEvent.click(screen.getByRole("button", { name: en["snapshots.title"] }));
  expect(await screen.findByText("a1b2c3d4")).toBeTruthy();
  expect(screen.getByRole("button", { name: en["restore.open"] })).toBeTruthy();
  expect(screen.getByRole("button", { name: en["snapshots.delete"] })).toBeTruthy();
});

it("hides export, the schedule switch and a snapshot's restore/delete while a peer's instance is open, and keeps the snapshot listed", async () => {
  await renderRow(["/vms?instance=member-1&instanceName=attic"]);
  expect(screen.queryByRole("switch", { name: en["containers.includeInSchedule"] })).toBeNull();
  expect(screen.queryByRole("button", { name: en["export.button"] })).toBeNull();

  fireEvent.click(screen.getByRole("button", { name: en["snapshots.title"] }));
  expect(await screen.findByText("a1b2c3d4")).toBeTruthy();
  expect(screen.queryByRole("button", { name: en["restore.open"] })).toBeNull();
  expect(screen.queryByRole("button", { name: en["snapshots.delete"] })).toBeNull();
});
