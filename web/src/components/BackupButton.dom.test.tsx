// @vitest-environment jsdom
/**
 * The stop warning (#197), and the three things about it that matter.
 *
 * A container backup takes the container offline for the duration of the run.
 * That is the right default, because it is what makes the appdata consistent,
 * and it was stated nowhere: somebody pressing this on Plex in the afternoon
 * loses the service for as long as the first full backup takes and reads the
 * result as the tool misbehaving.
 *
 * Said once is the whole design. A warning on every press is one people click
 * away without reading, which lands back where we started, so "the second press
 * does not ask" is asserted here as firmly as "the first press does".
 */
import { render, screen, cleanup, waitFor, fireEvent, act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const backupNow = vi.fn(async () => ({ ok: true }));
vi.mock("../lib/api", () => ({ backupNow: (...a: unknown[]) => backupNow(...a) }));

const fire = vi.fn(async () => {});
vi.mock("../lib/backupWatch", () => ({
  useBackupWatch: () => ({ state: { phase: "idle" }, fire, isPending: false }),
}));

vi.mock("../lib/toast", () => ({ useToast: () => ({ push: vi.fn() }) }));

import { BackupButton } from "./BackupButton";

const t = ((k: string) => k) as unknown as ReturnType<typeof import("../lib/i18n").useT>["t"];

beforeEach(() => {
  localStorage.clear();
  fire.mockClear();
});
afterEach(cleanup);

describe("BackupButton, the stop warning", () => {
  it("asks before the first backup this browser has ever started", async () => {
    render(<BackupButton name="plex" t={t} onBackedUp={() => {}} />);
    await act(async () => { fireEvent.click(screen.getByRole("button", { name: /backupNow/i })); });
    expect(await screen.findByText("containers.stopWarning")).toBeTruthy();
    // Nothing has run yet: the dialog is a question, not a notice after the fact.
    expect(fire).not.toHaveBeenCalled();
  });

  it("runs the backup once the warning is accepted", async () => {
    render(<BackupButton name="plex" t={t} onBackedUp={() => {}} />);
    await act(async () => { fireEvent.click(screen.getByRole("button", { name: /backupNow/i })); });
    await screen.findByText("containers.stopWarning");
    const confirm = screen
      .getAllByRole("button")
      .find((b) => b.textContent?.includes("containers.backupNow") && b.closest("[role=dialog]"));
    await act(async () => { fireEvent.click(confirm!); });
    await waitFor(() => expect(fire).toHaveBeenCalledTimes(1));
  });

  it("does NOT run it when the warning is declined", async () => {
    render(<BackupButton name="plex" t={t} onBackedUp={() => {}} />);
    await act(async () => { fireEvent.click(screen.getByRole("button", { name: /backupNow/i })); });
    await screen.findByText("containers.stopWarning");
    // The dialog fetches its own cancel/close wording, so it carries the REAL
    // translation rather than this file's key-echoing t. Picked by exclusion:
    // whatever is not the confirm button, whose label this component passes in.
    const cancel = screen
      .getAllByRole("button")
      .filter((b) => b.closest("[role=dialog]"))
      .find((b) => !b.textContent?.includes("containers.backupNow"));
    await act(async () => { fireEvent.click(cancel!); });
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
    // A private window or blocked site data: annoying beats silent downtime.
    const spy = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    render(<BackupButton name="plex" t={t} onBackedUp={() => {}} />);
    await act(async () => { fireEvent.click(screen.getByRole("button", { name: /backupNow/i })); });
    expect(await screen.findByText("containers.stopWarning")).toBeTruthy();
    spy.mockRestore();
  });
});
