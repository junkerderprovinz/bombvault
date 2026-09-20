// @vitest-environment jsdom
/**
 * A container backup takes the container offline for the run. The first backup
 * a browser starts warns about it; later ones must not, or the warning gets
 * clicked away unread.
 */
import { render, screen, cleanup, waitFor, fireEvent, act, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { en } from "../lib/i18n";

const backupNow = vi.fn(async () => ({ ok: true }));
vi.mock("../lib/api", () => ({ backupNow: (...a: unknown[]) => backupNow(...a) }));

const fire = vi.fn(async () => {});
const watch = { isPending: false };
vi.mock("../lib/backupWatch", () => ({
  useBackupWatch: () => ({ state: { phase: "idle" }, fire, isPending: watch.isPending }),
}));

vi.mock("../lib/toast", () => ({ useToast: () => ({ push: vi.fn() }) }));

import { BackupButton } from "./BackupButton";

const t = ((k: string) => k) as unknown as ReturnType<typeof import("../lib/i18n").useT>["t"];

beforeEach(() => {
  localStorage.clear();
  fire.mockClear();
  watch.isPending = false;
});
afterEach(cleanup);

describe("BackupButton stop warning", () => {
  it("asks before the first backup this browser has ever started", async () => {
    render(<BackupButton name="plex" t={t} onBackedUp={() => {}} />);
    await act(async () => { fireEvent.click(screen.getByRole("button", { name: /backupNow/i })); });
    expect(await screen.findByText("containers.stopWarning")).toBeTruthy();
    expect(fire).not.toHaveBeenCalled();
  });

  it("runs the backup once the warning is accepted", async () => {
    render(<BackupButton name="plex" t={t} onBackedUp={() => {}} />);
    await act(async () => { fireEvent.click(screen.getByRole("button", { name: /backupNow/i })); });
    // The dialog translates its own labels, and its commit repeats the trigger.
    const dialog = await screen.findByRole("dialog");
    const confirm = within(dialog).getByRole("button", { name: en["containers.backupNow"] });
    await act(async () => { fireEvent.click(confirm); });
    await waitFor(() => expect(fire).toHaveBeenCalledTimes(1));
  });

  it("does not run it when the warning is declined", async () => {
    render(<BackupButton name="plex" t={t} onBackedUp={() => {}} />);
    await act(async () => { fireEvent.click(screen.getByRole("button", { name: /backupNow/i })); });
    const dialog = await screen.findByRole("dialog");
    const cancel = within(dialog).getByRole("button", { name: en["common.cancel"] });
    await act(async () => { fireEvent.click(cancel); });
    await waitFor(() => expect(fire).not.toHaveBeenCalled());
  });

  it("never asks again after it has been acknowledged once", async () => {
    localStorage.setItem("bv-container-stop-ack", "1");
    render(<BackupButton name="plex" t={t} onBackedUp={() => {}} />);
    await act(async () => { fireEvent.click(screen.getByRole("button", { name: /backupNow/i })); });
    await waitFor(() => expect(fire).toHaveBeenCalledTimes(1));
    expect(screen.queryByText("containers.stopWarning")).toBeNull();
  });

  it("asks every time when the browser refuses storage, rather than never", async () => {
    const spy = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    render(<BackupButton name="plex" t={t} onBackedUp={() => {}} />);
    await act(async () => { fireEvent.click(screen.getByRole("button", { name: /backupNow/i })); });
    expect(await screen.findByText("containers.stopWarning")).toBeTruthy();
    spy.mockRestore();
  });
});

// A dump of a large database runs for many minutes with no percentage of its
// own, so the button has to say what is happening instead of "backing up".
describe("BackupButton while a database is being dumped", () => {
  const tEn = ((k: string) => en[k as keyof typeof en]) as unknown as ReturnType<
    typeof import("../lib/i18n").useT
  >["t"];

  function bubbleOf(el: HTMLElement): string {
    fireEvent.mouseEnter(el);
    return document.querySelector(".glim-bubble")?.textContent ?? "";
  }

  it("counts the dumped bytes while this container's own backup is dumping", () => {
    watch.isPending = true;
    const { container } = render(
      <BackupButton
        name="immich_postgres"
        t={tEn}
        progress={{ phase: "backup", percent: 0, active: true, lastSeen: 0, stage: "dbdump", bytes: 5_242_880 }}
      />
    );
    expect(bubbleOf(container.querySelector("span, button") as HTMLElement)).toContain("5.0 MB");
  });

  it("says a dump is running when another container's backup blocks this one", () => {
    const { container } = render(
      <BackupButton
        name="plex"
        t={tEn}
        running={{ active: true, phase: "backup", stage: "dbdump" }}
      />
    );
    expect(bubbleOf(container.querySelector("span, button") as HTMLElement)).toBe(en["dbdump.busyDumping"]);
  });

  it("keeps the plain busy hint when no dump is running", () => {
    const { container } = render(
      <BackupButton name="plex" t={tEn} running={{ active: true, phase: "backup" }} />
    );
    expect(bubbleOf(container.querySelector("span, button") as HTMLElement)).toBe(en["common.backupRunning"]);
  });
});
