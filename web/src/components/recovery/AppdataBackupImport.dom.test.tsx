// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { en } from "../../lib/i18n";
import type { AppdataBackupArchive } from "../../lib/api";

let archives: AppdataBackupArchive[] = [];
const scan = vi.fn(async (_path: string) => ({ ok: true, archives }));
const importIt = vi.fn(async (_path: string) => ({ ok: true, started: true, archives: 1 }));
vi.mock("../../lib/api", () => ({
  scanAppdataBackup: (p: string) => scan(p),
  importAppdataBackup: (p: string) => importIt(p),
}));
vi.mock("../../lib/progress", () => ({ useProgress: () => ({}) }));
vi.mock("../../lib/toast", () => ({ useToast: () => ({ push: vi.fn() }) }));
vi.mock("../FolderBrowser", () => ({
  FolderBrowser: ({ label, onChange }: { label: string; onChange: (v: string) => void }) => (
    <input aria-label={label} onChange={(e) => onChange(e.target.value)} />
  ),
}));

import { AppdataBackupImport } from "./AppdataBackupImport";

const t = ((k: keyof typeof en) => en[k]) as unknown as ReturnType<typeof import("../../lib/i18n").useT>["t"];

beforeEach(() => {
  scan.mockClear();
  importIt.mockClear();
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
