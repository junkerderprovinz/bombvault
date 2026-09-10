// @vitest-environment jsdom
/**
 * The backup cancel button (#200).
 *
 * Two things are worth pinning. The key, because it is the one thing that can
 * be wrong without anything saying so: the backend registers a running backup
 * under the key the progress stream publishes, and a button that sends anything
 * else gets a cheerful "cancelled: false" and does nothing forever. And the
 * confirmation, because a long backup stopped by an accidental click is an
 * evening of somebody's time.
 */
import { render, screen, cleanup, fireEvent, act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const cancelBackup = vi.fn(async () => ({ ok: true, cancelled: true }));
vi.mock("../lib/api", async (orig) => ({
  ...(await orig<Record<string, unknown>>()),
  cancelBackup: (key: string) => cancelBackup(key),
}));

// useConfirm renders a dialog; drive it by answering yes or no.
let answer = true;
vi.mock("../lib/useConfirm", () => ({
  useConfirm: () => ({
    confirm: async () => answer,
    confirmDialog: null,
  }),
}));

import { BackupCancelButton } from "./BackupCancelButton";

const t = ((key: string) => key) as never;

function draw(key = "files:My_Backups", name = "My_Backups") {
  return render(<BackupCancelButton cancelKey={key} name={name} t={t} />);
}

beforeEach(() => {
  cancelBackup.mockClear();
  answer = true;
});
afterEach(cleanup);

describe("BackupCancelButton", () => {
  it("sends the exact progress key it was given", async () => {
    draw("files:My_Backups");
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /backup.cancel/i }));
    });
    expect(cancelBackup).toHaveBeenCalledTimes(1);
    // Not the set id, not a prefix of its own invention: the key the card got
    // from the progress stream, verbatim.
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
  });

  it("does not report success when the POST fails, so the button stays usable", async () => {
    const onCancelled = vi.fn();
    cancelBackup.mockRejectedValueOnce(new Error("network"));
    render(<BackupCancelButton cancelKey="files:x" name="x" t={t} onCancelled={onCancelled} />);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /backup.cancel/i }));
    });
    expect(onCancelled).not.toHaveBeenCalled();
    // A failed cancel leaves the backup running, which is the safe outcome.
    expect(screen.getByRole("button", { name: /backup.cancel/i })).toHaveProperty("disabled", false);
  });
});
