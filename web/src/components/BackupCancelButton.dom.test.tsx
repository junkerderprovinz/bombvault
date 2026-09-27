// @vitest-environment jsdom
/**
 * The backend registers a running backup under its progress key and answers
 * any other key with "cancelled: false", so a wrong key fails silently. The
 * confirmation keeps a stray click from stopping a long backup.
 */
import { render, screen, cleanup, fireEvent, act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const cancelBackup = vi.fn(
  async (): Promise<{ ok: boolean; cancelled: boolean; reason?: string }> => ({ ok: true, cancelled: true })
);
vi.mock("../lib/api", async (orig) => ({
  ...(await orig<Record<string, unknown>>()),
  cancelBackup: (key: string) => cancelBackup(key),
}));

const pushed: { message: string; severity?: string }[] = [];
vi.mock("../lib/toast", () => ({
  useToast: () => ({ push: (message: string, severity?: string) => pushed.push({ message, severity }) }),
}));

// Answers the confirmation without rendering the dialog.
let answer = true;
vi.mock("../lib/useConfirm", () => ({
  useConfirm: () => ({
    confirm: async () => answer,
    confirmDialog: null,
    dismiss: () => {},
  }),
}));

type Entry = { phase: string; percent: number; active: boolean; lastSeen: number; committed?: boolean };
let progress: Record<string, Entry> = {};
vi.mock("../lib/progress", () => ({
  useProgress: () => progress,
}));

import { BackupCancelButton } from "./BackupCancelButton";

const t = ((key: string) => key) as never;

function draw(key = "files:My_Backups", name = "My_Backups") {
  return render(<BackupCancelButton cancelKey={key} name={name} t={t} />);
}

beforeEach(() => {
  cancelBackup.mockClear();
  pushed.length = 0;
  answer = true;
  progress = {};
});
afterEach(cleanup);

describe("BackupCancelButton", () => {
  it("sends the exact progress key it was given", async () => {
    draw("files:My_Backups");
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /backup.cancel/i }));
    });
    expect(cancelBackup).toHaveBeenCalledTimes(1);
    expect(cancelBackup).toHaveBeenCalledWith("files:My_Backups");
  });

  it("asks first, and sends nothing when the answer is no", async () => {
    answer = false;
    draw();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /backup.cancel/i }));
    });
    expect(cancelBackup).not.toHaveBeenCalled();
  });

  it("tells the card once the server accepted it", async () => {
    const onCancelled = vi.fn();
    render(<BackupCancelButton cancelKey="files:x" name="x" t={t} onCancelled={onCancelled} />);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /backup.cancel/i }));
    });
    expect(onCancelled).toHaveBeenCalledTimes(1);
    expect(pushed).toEqual([]);
  });

  it.each([
    ["committed", "backup.cancelTooLate"],
    ["not_running", "backup.cancelNotRunning"],
  ])("says why nothing was cancelled when the server answers %s", async (reason, message) => {
    const onCancelled = vi.fn();
    cancelBackup.mockResolvedValueOnce({ ok: true, cancelled: false, reason });
    render(<BackupCancelButton cancelKey="container:plex" name="plex" t={t} onCancelled={onCancelled} />);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /backup.cancel/i }));
    });
    expect(onCancelled).not.toHaveBeenCalled();
    expect(pushed).toEqual([{ message, severity: "warn" }]);
  });

  it("stops offering a cancel once the backup has written its restore point", () => {
    // The confirmation promises a run without a snapshot, which is no longer
    // true while the containers start again.
    progress = { "container:plex": { phase: "backup", percent: 100, active: true, lastSeen: 0, committed: true } };
    render(<BackupCancelButton cancelKey="container:plex" name="plex" t={t} />);
    expect(screen.queryByRole("button", { name: /backup.cancel/i })).toBeNull();
  });

  it("keeps the cancel for another item's committed backup", () => {
    progress = { "container:db": { phase: "backup", percent: 100, active: true, lastSeen: 0, committed: true } };
    render(<BackupCancelButton cancelKey="container:plex" name="plex" t={t} />);
    expect(screen.getByRole("button", { name: /backup.cancel/i })).toBeTruthy();
  });

  it("does not report success when the POST fails, so the button stays usable", async () => {
    const onCancelled = vi.fn();
    cancelBackup.mockRejectedValueOnce(new Error("network"));
    render(<BackupCancelButton cancelKey="files:x" name="x" t={t} onCancelled={onCancelled} />);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /backup.cancel/i }));
    });
    expect(onCancelled).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: /backup.cancel/i })).toHaveProperty("disabled", false);
  });
});
