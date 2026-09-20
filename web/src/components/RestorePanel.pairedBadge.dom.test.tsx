// @vitest-environment jsdom
// A snapshot taken together with a database dump says so, on the local
// repository and on an off-site copy, which carries a new id and keeps the
// source id. The tags that make the pairing possible stay out of the row.
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { AdvancedProvider } from "../lib/advanced";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";

class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

const LOCAL = {
  id: "a1b2c3d4e5f6",
  time: "2026-09-01T10:00:00Z",
  paths: [],
  tags: ["container:immich_postgres", "bvrun:run-7", "dbengine:postgres", "dbversion:16.4", "dbname:immich", "nightly"],
  hostname: "tower",
};
const COPY = { ...LOCAL, id: "ffff0000ffff", original: "a1b2c3d4e5f6" };

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listSnapshots: (_name: string, source?: string) =>
      Promise.resolve({ ok: true, snapshots: [source === "local" ? LOCAL : COPY] }),
    listDbDumps: () =>
      Promise.resolve({
        ok: true,
        dumps: [
          {
            id: "9999dddd",
            time: "2026-09-01T09:59:00Z",
            engine: "postgres",
            image: "postgres:16",
            version: "16.4",
            databases: ["immich"],
            bytes: 1024,
            damaged: false,
            pairedSnapshotId: "a1b2c3d4e5f6",
          },
        ],
      }),
    listRuns: () => Promise.resolve({ ok: true, runs: [] }),
    getSettings: () => Promise.resolve({ ok: false }),
    listOffsiteTargets: () => Promise.resolve({ ok: true, targets: [] }),
  };
});

const { RestorePanel } = await import("./RestorePanel");

type PanelProps = Parameters<typeof RestorePanel>[0];
const t = ((key: string) => en[key as keyof typeof en] ?? key) as unknown as PanelProps["t"];

async function renderPanel(over: Partial<PanelProps> = {}) {
  await act(async () => {
    render(
      <I18nProvider>
        <AdvancedProvider>
          <ToastProvider>
            <RestorePanel name="immich_postgres" t={t} open isDatabase dbCoverage="live" containerRunning {...over} />
          </ToastProvider>
        </AdvancedProvider>
      </I18nProvider>
    );
  });
}

beforeEach(() => localStorage.setItem("bombvault.advanced", "1"));
afterEach(() => {
  cleanup();
  localStorage.removeItem("bombvault.advanced");
});

it("marks the snapshot the dump belongs to", async () => {
  await renderPanel();
  expect(await screen.findByText(en["dbdump.pairedBadge"])).toBeTruthy();
});

it("marks an off-site copy of that snapshot too", async () => {
  await renderPanel();
  await act(async () => {
    fireEvent.click(screen.getByRole("tab", { name: en["source.offsite"] }));
  });
  expect(await screen.findByText(en["dbdump.pairedBadge"])).toBeTruthy();
});

it("keeps the pairing tags out of the row", async () => {
  await renderPanel();
  await screen.findByText(en["dbdump.pairedBadge"]);
  expect(screen.getByText("nightly")).toBeTruthy();
  for (const tag of ["bvrun:run-7", "dbengine:postgres", "dbversion:16.4", "dbname:immich"]) {
    expect(screen.queryByText(tag)).toBeNull();
  }
});

it("warns above the restore button that the files were copied while the server ran", async () => {
  await renderPanel();
  fireEvent.click(await screen.findByRole("button", { name: en["restore.open"] }));
  expect(await screen.findByText(en["dbdump.restoreLiveWarn"])).toBeTruthy();
});

it("warns that the data folder is in no backup at all", async () => {
  await renderPanel({ dbCoverage: "none" });
  fireEvent.click(await screen.findByRole("button", { name: en["restore.open"] }));
  expect(await screen.findByText(en["dbdump.restoreNoneWarn"])).toBeTruthy();
});
