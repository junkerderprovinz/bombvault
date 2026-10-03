// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { en } from "../../lib/i18n";
import type { AppdataBackupArchive } from "../../lib/api";
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

import { AppdataBackupImport, APPDATA_IMPORT_KEY } from "./AppdataBackupImport";

const t = ((k: keyof typeof en) => en[k]) as unknown as ReturnType<typeof import("../../lib/i18n").useT>["t"];

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

function renderIt() {
  let hue = 0;
  render(<AppdataBackupImport hostMountRoot="/mnt" nextHue={() => hue++} t={t} />);
}

it("shows what the folder holds before anything is imported", async () => {
  renderIt();
  fireEvent.change(screen.getByLabelText(en["recovery.abFolder"]), { target: { value: "user/backups" } });
  fireEvent.click(screen.getByRole("button", { name: en["recovery.abScan"] }));
  expect(await screen.findByText("plex")).toBeTruthy();
  expect(scan).toHaveBeenCalledWith("user/backups");
  expect(screen.getByText(en["recovery.abStatusNew"])).toBeTruthy();
  expect(screen.getByText(en["recovery.abStatusNoContainer"])).toBeTruthy();
  expect(importIt).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: en["recovery.abImport"] }));
  await waitFor(() => expect(importIt).toHaveBeenCalledWith("user/backups"));
});

it("offers no import when nothing is new", async () => {
  archives = archives.map((a) => ({ ...a, status: "imported" }));
  renderIt();
  fireEvent.change(screen.getByLabelText(en["recovery.abFolder"]), { target: { value: "user/backups" } });
  fireEvent.click(screen.getByRole("button", { name: en["recovery.abScan"] }));
  await screen.findByText("plex");
  expect(screen.getByRole("button", { name: en["recovery.abImport"] }).hasAttribute("disabled")).toBe(true);
});

// The import runs on the server and reports through the progress stream: an
// entry while it runs, a last frame that lingers with the count of failed
// archives, and then nothing.
async function runImport(view: ReturnType<typeof render>, failed: number | undefined) {
  fireEvent.change(screen.getByLabelText(en["recovery.abFolder"]), { target: { value: "user/backups" } });
  fireEvent.click(screen.getByRole("button", { name: en["recovery.abScan"] }));
  await screen.findByText("plex");
  fireEvent.click(screen.getByRole("button", { name: en["recovery.abImport"] }));
  await waitFor(() => expect(importIt).toHaveBeenCalled());
  const rerender = () => view.rerender(<AppdataBackupImport hostMountRoot="/mnt" nextHue={() => 0} t={t} />);
  progressMap = { [APPDATA_IMPORT_KEY]: { phase: "maintenance", percent: 40, active: true, lastSeen: Date.now() } };
  rerender();
  progressMap = { [APPDATA_IMPORT_KEY]: { phase: "maintenance", percent: 100, active: true, finished: true, failed, lastSeen: Date.now() } };
  rerender();
  progressMap = {};
  rerender();
}

it("says the import finished when every archive went in", async () => {
  const view = render(<AppdataBackupImport hostMountRoot="/mnt" nextHue={() => 0} t={t} />);
  await runImport(view, undefined);
  await waitFor(() => expect(push).toHaveBeenCalledWith(en["recovery.abDone"], "success"));
});

it("says no archive went in when every one failed", async () => {
  const view = render(<AppdataBackupImport hostMountRoot="/mnt" nextHue={() => 0} t={t} />);
  await runImport(view, 1);
  await waitFor(() => expect(push).toHaveBeenCalledWith(en["recovery.abAllFailed"], "fail"));
  expect(push).not.toHaveBeenCalledWith(en["recovery.abDone"], "success");
});

it("warns when only some archives went in", async () => {
  importIt.mockResolvedValueOnce({ ok: true, started: true, archives: 3 });
  const view = render(<AppdataBackupImport hostMountRoot="/mnt" nextHue={() => 0} t={t} />);
  await runImport(view, 1);
  await waitFor(() => expect(push).toHaveBeenCalledWith(en["recovery.abSomeFailed"], "warn"));
});

it("drops a scan that answers after the folder changed", async () => {
  let answer: (v: { ok: boolean; archives: AppdataBackupArchive[] }) => void = () => undefined;
  scan.mockImplementationOnce(() => new Promise((resolve) => (answer = resolve)));
  renderIt();
  const field = screen.getByLabelText(en["recovery.abFolder"]);
  fireEvent.change(field, { target: { value: "user/old" } });
  fireEvent.click(screen.getByRole("button", { name: en["recovery.abScan"] }));
  await waitFor(() => expect(scan).toHaveBeenCalledWith("user/old"));
  fireEvent.change(field, { target: { value: "user/new" } });
  answer({ ok: true, archives });
  await new Promise((r) => setTimeout(r, 20));
  expect(screen.queryByText("plex")).toBeNull();
  expect(screen.queryByRole("button", { name: en["recovery.abImport"] })).toBeNull();
  expect(screen.getByRole("button", { name: en["recovery.abScan"] }).hasAttribute("disabled")).toBe(false);
});
