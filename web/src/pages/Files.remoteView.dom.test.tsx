// @vitest-environment jsdom
// Files keeps its backup triggers in remote view (POST /api/files/sets/{id}/
// backup and /api/files/backup-all are on the allowlist), but the set's own
// settings (edit, delete, the schedule toggle, the folder selection editor)
// and Discover/Add have no route on it, so they must disappear rather than
// merely disable while a peer's instance is open.
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { InstanceProvider } from "../lib/instanceScope";
import { ToastProvider } from "../lib/toast";
import type { FileSetView } from "../lib/api";

class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

const documents: FileSetView = {
  id: "set1",
  name: "Documents",
  path: "documents",
  excludes: [],
  enabled: true,
  lastBackup: 0,
  pathExists: true,
};

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listFileSets: () => Promise.resolve({ ok: true, fileSets: [documents] }),
    getFileSetPreset: () => Promise.resolve({ ok: true, offered: false, name: "", path: "", excludes: [] }),
    getSettings: () => Promise.resolve({ ok: true, hostMountRoot: "/host/user", settings: { restoreFolder: "" } }),
    listRuns: () => Promise.resolve({ ok: true, runs: [] }),
  };
});

const { Files } = await import("./Files");

afterEach(cleanup);

async function renderFiles(initialEntries: string[]) {
  await act(async () => {
    render(
      <MemoryRouter initialEntries={initialEntries}>
        <InstanceProvider>
          <I18nProvider>
            <ToastProvider>
              <Files />
            </ToastProvider>
          </I18nProvider>
        </InstanceProvider>
      </MemoryRouter>,
    );
  });
}

it("offers the set's settings, schedule toggle and Discover/Add locally", async () => {
  await renderFiles(["/files"]);
  expect(await screen.findByText("Documents")).toBeTruthy();
  expect(screen.getAllByRole("button", { name: en["common.edit"] })[0]).toBeTruthy();
  expect(screen.getAllByRole("button", { name: en["common.delete"] })[0]).toBeTruthy();
  expect(screen.getByRole("switch", { name: en["files.enabled"] })).toBeTruthy();
  expect(screen.getByRole("button", { name: en["containers.discover"] })).toBeTruthy();
  expect(screen.getByRole("button", { name: en["files.addSet"] })).toBeTruthy();
});

it("hides the set's settings, schedule toggle and Discover/Add while a peer's instance is open, and keeps the set listed", async () => {
  await renderFiles(["/files?instance=member-1&instanceName=attic"]);
  expect(await screen.findByText("Documents")).toBeTruthy();
  expect(screen.queryByRole("button", { name: en["common.edit"] })).toBeNull();
  expect(screen.queryByRole("button", { name: en["common.delete"] })).toBeNull();
  expect(screen.queryByRole("switch", { name: en["files.enabled"] })).toBeNull();
  expect(screen.queryByRole("button", { name: en["containers.discover"] })).toBeNull();
  expect(screen.queryByRole("button", { name: en["files.addSet"] })).toBeNull();
});
