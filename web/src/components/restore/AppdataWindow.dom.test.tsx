// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { AppdataBackupArchive } from "../../lib/api";
import { I18nProvider, countText, en } from "../../lib/i18n";
import type { ProgressState } from "../../lib/progress";

let archives: AppdataBackupArchive[] = [];
const scan = vi.fn(async (_path: string) => ({ ok: true, archives }));
const importIt = vi.fn(async (_path: string) => ({ ok: true, started: true, archives: 1 }));
vi.mock("../../lib/api", () => ({
  scanAppdataBackup: (p: string) => scan(p),
  importAppdataBackup: (p: string) => importIt(p),
}));
let progressMap: Record<string, ProgressState> = {};
vi.mock("../../lib/progress", () => ({ useProgress: () => progressMap }));
const push = vi.fn();
vi.mock("../../lib/toast", () => ({ useToast: () => ({ push }) }));
vi.mock("../FolderBrowser", () => ({
  FolderBrowser: ({ label, onChange }: { label: string; onChange: (v: string) => void }) => (
    <input aria-label={label} onChange={(e) => onChange(e.target.value)} />
  ),
}));

import { APPDATA_IMPORT_KEY, AppdataWindow } from "./AppdataWindow";

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

const onClose = vi.fn();
const view = () => (
  <I18nProvider>
    <AppdataWindow hostMountRoot="/mnt" onClose={onClose} />
  </I18nProvider>
);
const button = (name: string) => screen.getByRole("button", { name }) as HTMLButtonElement;

beforeEach(() => {
  scan.mockClear();
  importIt.mockClear();
  push.mockClear();
  progressMap = {};
  archives = [
    { folder: "ab_20250101_030000", container: "plex", file: "plex.tar.zst", size: 2048, time: 1_735_700_400, status: "new" },
    { folder: "ab_20250101_030000", container: "gone", file: "gone.tar", size: 10, time: 1_735_700_400, status: "no-container" },
  ];
});
afterEach(cleanup);

async function look(folder = "user/backups") {
  fireEvent.change(screen.getByLabelText(en["recovery.abFolder"]), { target: { value: folder } });
  fireEvent.click(button(en["recovery.abScan"]));
  await screen.findByText("plex");
}

it("shows what the folder holds before anything is imported", async () => {
  render(view());
  await look();
  expect(scan).toHaveBeenCalledWith("user/backups");
  expect(screen.getByText(countText(en["appdata.found"], "en", 2).replace("{fresh}", "1"))).toBeTruthy();
  expect(screen.getByText(en["recovery.abStatusNew"])).toBeTruthy();
  expect(screen.getByText(en["recovery.abStatusNoContainer"])).toBeTruthy();
  expect(importIt).not.toHaveBeenCalled();
  fireEvent.click(button(en["recovery.abImport"]));
  await waitFor(() => expect(importIt).toHaveBeenCalledWith("user/backups"));
});

it("offers no import when nothing is new", async () => {
  archives = archives.map((a) => ({ ...a, status: "imported" }));
  render(view());
  await look();
  expect(button(en["recovery.abImport"]).disabled).toBe(true);
});

it("says so when the folder holds no archive", async () => {
  archives = [];
  render(view());
  fireEvent.change(screen.getByLabelText(en["recovery.abFolder"]), { target: { value: "user/empty" } });
  fireEvent.click(button(en["recovery.abScan"]));
  expect(await screen.findByText(en["recovery.abNothing"])).toBeTruthy();
});

it("goes back to the folder", async () => {
  render(view());
  await look();
  fireEvent.click(button(en["common.back"]));
  expect(screen.queryByText("plex")).toBeNull();
  expect(button(en["recovery.abScan"])).toBeTruthy();
});

// The import runs on the server and reports through the progress stream: an
// entry while it runs, a last frame that lingers with the count of failed
// archives, and then nothing.
async function runImport(page: ReturnType<typeof render>, failed: number | undefined) {
  await look();
  fireEvent.click(button(en["recovery.abImport"]));
  await waitFor(() => expect(importIt).toHaveBeenCalled());
  progressMap = { [APPDATA_IMPORT_KEY]: { phase: "maintenance", percent: 40, active: true, lastSeen: Date.now() } };
  page.rerender(view());
  expect(button(en["recovery.abImport"]).disabled).toBe(true);
  progressMap = { [APPDATA_IMPORT_KEY]: { phase: "maintenance", percent: 100, active: true, finished: true, failed, lastSeen: Date.now() } };
  page.rerender(view());
  progressMap = {};
  page.rerender(view());
}

it("says the import finished when every archive went in", async () => {
  await runImport(render(view()), undefined);
  expect(await screen.findByText(en["recovery.abDone"])).toBeTruthy();
  expect(scan).toHaveBeenCalledTimes(2);
});

it("says no archive went in when every one failed", async () => {
  await runImport(render(view()), 1);
  expect(await screen.findByText(en["recovery.abAllFailed"])).toBeTruthy();
  expect(screen.queryByText(en["recovery.abDone"])).toBeNull();
});

it("warns when only some archives went in", async () => {
  importIt.mockResolvedValueOnce({ ok: true, started: true, archives: 3 });
  await runImport(render(view()), 1);
  expect(await screen.findByText(en["recovery.abSomeFailed"])).toBeTruthy();
});

it("drops a scan that answers after the folder changed", async () => {
  let answer: (v: { ok: boolean; archives: AppdataBackupArchive[] }) => void = () => undefined;
  scan.mockImplementationOnce(() => new Promise((resolve) => (answer = resolve)));
  render(view());
  const field = screen.getByLabelText(en["recovery.abFolder"]);
  fireEvent.change(field, { target: { value: "user/old" } });
  fireEvent.click(button(en["recovery.abScan"]));
  await waitFor(() => expect(scan).toHaveBeenCalledWith("user/old"));
  fireEvent.change(field, { target: { value: "user/new" } });
  answer({ ok: true, archives });
  await new Promise((r) => setTimeout(r, 20));
  expect(screen.queryByText("plex")).toBeNull();
  expect(button(en["recovery.abScan"]).disabled).toBe(false);
});
