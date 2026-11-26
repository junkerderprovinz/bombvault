// @vitest-environment jsdom
import { render, screen, cleanup, waitFor, fireEvent, act, within } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { en } from "../lib/i18n";

const listFlashPlugins = vi.fn(async () => ({
  ok: true,
  plugins: [{ name: "unassigned.devices", version: "2026.09.01", size: 2048, packages: [] }],
}));
const session = { phone: false };
vi.mock("../lib/api", () => ({
  listFlashPlugins: (...a: unknown[]) => listFlashPlugins(...(a as [])),
  restoreFlashPlugin: vi.fn(),
  openedAsPairedPhone: () => session.phone,
}));

const fire = vi.fn(async () => {});
vi.mock("../lib/backupWatch", () => ({
  useBackupWatch: () => ({ state: { phase: "idle" }, fire, isPending: false }),
}));
vi.mock("../lib/toast", () => ({ useToast: () => ({ push: vi.fn() }) }));

import { FlashPluginList } from "./FlashPluginList";

const t = ((k: keyof typeof en) => en[k]) as unknown as ReturnType<typeof import("../lib/i18n").useT>["t"];

beforeEach(() => fire.mockClear());
afterEach(() => {
  cleanup();
  session.phone = false;
});

it("lists each plugin with its version", async () => {
  render(<FlashPluginList snapshotId="abc" source="local" t={t} />);
  expect(await screen.findByText("unassigned.devices")).toBeTruthy();
  expect(screen.getByText("2026.09.01")).toBeTruthy();
  expect(listFlashPlugins).toHaveBeenCalledWith("abc", "local");
});

it("writes into the flash only after the confirmation", async () => {
  render(<FlashPluginList snapshotId="abc" source="local" t={t} />);
  await screen.findByText("unassigned.devices");
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: en["flash.pluginRestore"] }));
  });
  expect(fire).not.toHaveBeenCalled();
  const dialog = await screen.findByRole("dialog");
  expect(dialog.textContent).toContain("unassigned.devices");
  await act(async () => {
    fireEvent.click(within(dialog).getByRole("button", { name: en["flash.pluginRestore"] }));
  });
  await waitFor(() => expect(fire).toHaveBeenCalledTimes(1));
});

it("says on a paired phone that the flash is backed up first", async () => {
  session.phone = true;
  render(<FlashPluginList snapshotId="abc" source="local" t={t} />);
  await screen.findByText("unassigned.devices");
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: en["flash.pluginRestore"] }));
  });
  expect((await screen.findByRole("dialog")).textContent).toContain(en["flash.pluginRestoreBackupFirst"]);
});
