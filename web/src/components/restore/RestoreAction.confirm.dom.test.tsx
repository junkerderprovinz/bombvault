// @vitest-environment jsdom
// A row-action restore overwrites live appdata or VM disks, so it asks in a
// modal first. These tests check that the restore call waits for the answer,
// not only that a dialog appears.
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider, en } from "../../lib/i18n";
import { stubEventSource } from "../../lib/placement.testsupport";

const restore = vi.fn(() => Promise.resolve({ ok: true, started: true }));
const restoreVM = vi.fn(() => Promise.resolve({ ok: true, started: true }));
const passing = [
  { id: "repository", status: "ok" },
  { id: "key", status: "ok" },
  { id: "snapshot", status: "ok", detail: "aaaa1111" },
  { id: "space", status: "ok", need: 100, free: 1000 },
];
const checkRestore = vi.fn((): Promise<unknown> => Promise.resolve({ ok: true, ready: true, checks: passing, plan: null }));

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    restore: (...a: unknown[]) => restore(...(a as [])),
    restoreVM: (...a: unknown[]) => restoreVM(...(a as [])),
    checkRestore: (...a: unknown[]) => checkRestore(...(a as [])),
    listRuns: () => Promise.resolve({ ok: true, runs: [] }),
  };
});

// The progress store opens an EventSource on mount and jsdom has none. What it
// carries is irrelevant to these tests, which assert whether restore() is
// called at all.
stubEventSource();

const { RestoreAction } = await import("./RestoreAction");

function renderAction(props: Record<string, unknown> = {}) {
  const Harness = () => {
    const t = ((k: string) => en[k as keyof typeof en] ?? k) as never;
    return (
      <RestoreAction
        domain="container"
        name="plex"
        snapshotId="latest"
        otherActive={{ active: false }}
        successMessage="done"
        requireConfirm={false}
        showLeaveStopped={false}
        forceLeaveStopped
        showBusyHint={false}
        showStartedHint={false}
        iconBadge
        t={t}
        {...props}
      />
    );
  };
  return render(
    <I18nProvider>
      <Harness />
    </I18nProvider>
  );
}

function trigger() {
  return screen.getByRole("button", { name: en["snapshots.restore"] });
}

afterEach(() => {
  cleanup();
  restore.mockClear();
  restoreVM.mockClear();
  checkRestore.mockClear();
});

const shortOfSpace = {
  ok: true,
  ready: false,
  checks: [...passing.slice(0, 3), { id: "space", status: "fail", reason: "short", need: 1000, free: 100 }],
  plan: null,
};

describe("row-action restore confirmation", () => {
  it("does not restore while the question is still open", async () => {
    renderAction({ confirmMessage: "Really restore plex?" });

    fireEvent.click(trigger());
    await waitFor(() => expect(screen.getByText("Really restore plex?")).toBeTruthy());
    expect(restore).not.toHaveBeenCalled();
  });

  it("does not restore when the answer is no", async () => {
    renderAction({ confirmMessage: "Really restore plex?" });

    fireEvent.click(trigger());
    await waitFor(() => expect(screen.getByText("Really restore plex?")).toBeTruthy());
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["common.cancel"] }));
    });
    expect(restore).not.toHaveBeenCalled();
  });

  it("restores once the answer is yes", async () => {
    renderAction({ confirmMessage: "Really restore plex?" });

    fireEvent.click(trigger());
    await waitFor(() => expect(screen.getByText("Really restore plex?")).toBeTruthy());
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["common.confirm"] }));
    });
    await waitFor(() => expect(restore).toHaveBeenCalledTimes(1));
  });

  it("restores without a question when there is no confirmMessage", async () => {
    // The form call sites gate on the confirm toggle and must not ask twice.
    renderAction();
    await act(async () => {
      fireEvent.click(trigger());
    });
    await waitFor(() => expect(restore).toHaveBeenCalledTimes(1));
  });
});

describe("a place that lost the snapshot", () => {
  it("is reported so the timeline can offer the next place", async () => {
    restore.mockImplementationOnce((() =>
      Promise.resolve({ ok: false, code: "snapshot-missing", error: "gone" })) as never);
    const onMissing = vi.fn();
    renderAction({ onMissing });
    await act(async () => {
      fireEvent.click(trigger());
    });
    await waitFor(() => expect(onMissing).toHaveBeenCalledTimes(1));
  });
});

describe("the form restore button", () => {
  it("reads as the action it starts until a restore runs", () => {
    renderAction({ iconBadge: false, requireConfirm: true });
    const button = screen.getByRole("button", { name: en["snapshots.restore"] });
    expect(button.hasAttribute("disabled")).toBe(true);
    expect(screen.queryByText(en["common.restoring"])).toBeNull();
  });
  it("waits for the pre-flight check before Start opens", async () => {
    renderAction({ iconBadge: false, requireConfirm: false });
    const button = screen.getByRole("button", { name: en["snapshots.restore"] });
    expect(button.hasAttribute("disabled")).toBe(true);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: en["snapshots.restore"] }).hasAttribute("disabled")).toBe(false)
    );
    expect(checkRestore).toHaveBeenCalledWith(
      expect.objectContaining({ kind: "container", name: "plex", snapshotId: "latest" }),
      expect.anything()
    );
    expect(screen.getByText(en["restoreCheck.line.space"])).toBeTruthy();
  });

  it("stays locked while a check is red and says which one", async () => {
    checkRestore.mockImplementation(() => Promise.resolve(shortOfSpace));
    renderAction({ iconBadge: false, requireConfirm: false });
    await waitFor(() => expect(screen.getByText(en["restoreCheck.status.fail"])).toBeTruthy());
    const button = screen.getByRole("button", { name: en["snapshots.restore"] });
    expect(button.hasAttribute("disabled")).toBe(true);
    expect(screen.getByText("Needs 1000 B, 100 B free.")).toBeTruthy();
    checkRestore.mockImplementation(() => Promise.resolve({ ok: true, ready: true, checks: passing, plan: null }));
  });
});

describe("the row restore check", () => {
  it("locks the confirm button when a check fails", async () => {
    checkRestore.mockImplementation(() => Promise.resolve(shortOfSpace));
    renderAction({ confirmMessage: "Really restore plex?" });
    fireEvent.click(trigger());
    await waitFor(() => expect(screen.getByText("Really restore plex?")).toBeTruthy());
    const confirmButton = screen.getByRole("button", { name: en["common.confirm"] });
    expect(confirmButton.hasAttribute("disabled")).toBe(true);
    expect(restore).not.toHaveBeenCalled();
    checkRestore.mockImplementation(() => Promise.resolve({ ok: true, ready: true, checks: passing, plan: null }));
  });
});
