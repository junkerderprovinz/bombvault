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

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    restore: (...a: unknown[]) => restore(...(a as [])),
    restoreVM: (...a: unknown[]) => restoreVM(...(a as [])),
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
});

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
