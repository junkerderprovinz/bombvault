// @vitest-environment jsdom
// Runs the button through the real lib/progress.ts stream, so the frames the
// backend sends decide whether a cancel is still offered. Only EventSource is
// faked, because jsdom has none.
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const cancelBackup = vi.fn(async () => ({ ok: true, cancelled: true }));
vi.mock("../lib/api", async (orig) => ({
  ...(await orig<Record<string, unknown>>()),
  cancelBackup: () => cancelBackup(),
}));

const pushed: { message: string; severity?: string }[] = [];
vi.mock("../lib/toast", () => ({
  useToast: () => ({ push: (message: string, severity?: string) => pushed.push({ message, severity }) }),
}));

import { BackupCancelButton } from "./BackupCancelButton";

const instances: FakeEventSource[] = [];

class FakeEventSource {
  onmessage: ((ev: MessageEvent<string>) => void) | null = null;
  constructor(_url: string) {
    instances.push(this);
  }
  close(): void {}
}

function send(frame: Record<string, unknown>) {
  act(() => {
    instances.at(-1)?.onmessage?.({ data: JSON.stringify(frame) } as MessageEvent<string>);
  });
}

const t = ((key: string) => key) as never;

function cancelButton() {
  return screen.queryByRole("button", { name: /backup.cancel/i });
}

beforeEach(() => {
  vi.useFakeTimers();
  instances.length = 0;
  pushed.length = 0;
  cancelBackup.mockClear();
  globalThis.EventSource = FakeEventSource as unknown as typeof EventSource;
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("BackupCancelButton on the live progress stream", () => {
  it("stays hidden while a committed backup's bar lingers after the run", () => {
    render(<BackupCancelButton cancelKey="container:plex" name="plex" t={t} />);
    send({ key: "container:plex", phase: "backup", percent: 40, active: true });
    expect(cancelButton()).not.toBeNull();
    send({ key: "container:plex", phase: "backup", percent: 100, active: true, committed: true });
    expect(cancelButton()).toBeNull();
    send({ key: "container:plex", phase: "backup", percent: 100, active: false });
    expect(cancelButton()).toBeNull();
  });

  it("offers no cancel for a run that has ended", () => {
    render(<BackupCancelButton cancelKey="files:docs" name="docs" t={t} />);
    send({ key: "files:docs", phase: "backup", percent: 70, active: true });
    expect(cancelButton()).not.toBeNull();
    send({ key: "files:docs", phase: "backup", percent: 100, active: false });
    expect(cancelButton()).toBeNull();
  });

  it.each([
    ["the restore point is written", { percent: 100, active: true, committed: true }, "backup.cancelTooLate"],
    ["the run ends", { percent: 100, active: false }, "backup.cancelNotRunning"],
  ])("closes an open confirmation once %s and says why", async (_when, frame, message) => {
    render(<BackupCancelButton cancelKey="container:plex" name="plex" t={t} />);
    send({ key: "container:plex", phase: "backup", percent: 40, active: true });
    await act(async () => {
      fireEvent.click(cancelButton()!);
    });
    expect(screen.getByRole("dialog").textContent).toContain("backup.cancelConfirm");

    send({ key: "container:plex", phase: "backup", ...frame });
    await act(async () => {});
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(pushed).toEqual([{ message, severity: "warn" }]);
    expect(cancelBackup).not.toHaveBeenCalled();
  });

  it("keeps keyboard focus in place when the cancel goes away under an open confirmation", async () => {
    const { container } = render(<BackupCancelButton cancelKey="container:plex" name="plex" t={t} />);
    send({ key: "container:plex", phase: "backup", percent: 40, active: true });
    cancelButton()!.focus();
    await act(async () => {
      fireEvent.click(cancelButton()!);
    });

    send({ key: "container:plex", phase: "backup", percent: 100, active: true, committed: true });
    await act(async () => {});
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).not.toBe(document.body);
    expect(container.contains(document.activeElement)).toBe(true);
  });

  it("leaves the cancel alone when no confirmation is open", () => {
    render(<BackupCancelButton cancelKey="container:plex" name="plex" t={t} />);
    send({ key: "container:plex", phase: "backup", percent: 40, active: true });
    send({ key: "container:plex", phase: "backup", percent: 100, active: true, committed: true });
    expect(pushed).toEqual([]);
  });
});
